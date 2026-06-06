package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

// configWithEnv returns a base test config pointing the default
// environment at the given API URL/token. Used by the validation
// tests to swap in an httptest server URL without dragging in the
// production config-file path.
func configWithEnv(apiURL, token string) *config.Config {
	cfg := baseTestConfig()
	cfg.ActiveProfile = profiles.BuiltinFullAccess
	cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		profiles.BuiltinFullAccess: {Disabled: false},
	}

	env := cfg.Environments[envKeyDefault]
	env.Linode = config.LinodeConfig{APIURL: apiURL, Token: token}
	cfg.Environments[envKeyDefault] = env

	return cfg
}

// TestValidateScopesNoTokenReturnsSentinel covers the missing-token
// path: an environment with an empty token never makes an API call and
// returns ErrTokenNotConfigured so main can pick a policy based on
// profile elevation.
func TestValidateScopesNoTokenReturnsSentinel(t *testing.T) {
	t.Parallel()

	cfg := configWithEnv("https://example.invalid", "")

	srv, err := server.New(cfg)
	requireNoError(t, err)

	got, err := srv.ValidateScopes(t.Context())
	requireError(t, err)
	assertNil(t, got, "result must be nil when no token configured")
	requireErrorIs(t, err, profiles.ErrTokenNotConfigured,
		"missing token must surface as ErrTokenNotConfigured so callers can match it")
}

// TestValidateScopesPATPathSucceeds covers the happy path: a PAT
// response carrying every required scope produces a comparison with
// no missing entries. Uses httptest so the test exercises the real
// Linode client code path.
func TestValidateScopesPATPathSucceeds(t *testing.T) {
	t.Parallel()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, "/profile", r.URL.Path,
			"PAT path must only hit /profile (no /profile/grants)")
		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Profile{
			Username: "u",
			Scopes:   "*",
		}))
	}))
	defer httpSrv.Close()

	cfg := configWithEnv(httpSrv.URL, "pat-token")

	srv, err := server.New(cfg)
	requireNoError(t, err)

	got, err := srv.ValidateScopes(t.Context())
	requireNoError(t, err)
	requireNotNil(t, got)

	assertEqual(t, profiles.TokenKindPAT, got.Kind)
	assertFalse(t, got.Comparison.HasMissing(),
		"wildcard PAT must satisfy every required scope")
}

// TestValidateScopesMissingScopesReportedInComparison verifies that
// under-scoped tokens do NOT produce an error from Server.ValidateScopes;
// the missing entries appear in the Comparison so main can format them
// into a user-facing error message.
func TestValidateScopesMissingScopesReportedInComparison(t *testing.T) {
	t.Parallel()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Profile{
			Username: "u",
			Scopes:   "linodes:read_only",
		}))
	}))
	defer httpSrv.Close()

	cfg := configWithEnv(httpSrv.URL, "pat-token")

	srv, err := server.New(cfg)
	requireNoError(t, err)

	got, err := srv.ValidateScopes(t.Context())
	requireNoError(t, err, "missing scopes must not surface as an error")
	requireNotNil(t, got)
	assertTrue(t, got.Comparison.HasMissing(),
		"full-access profile needs more scopes than linodes:read_only")
}

// TestProfileIsElevated covers the missing-token policy helper. Default
// and readonly-full carry only :read_only scopes and must not be
// flagged elevated; full-access and compute-admin must be.
func TestProfileIsElevated(t *testing.T) {
	t.Parallel()

	defaultCfg := baseTestConfig()
	srv, err := server.New(defaultCfg)
	requireNoError(t, err)

	defaultProfile := srv.ActiveProfile()
	assertFalse(t, profiles.ProfileIsElevated(&defaultProfile),
		"default profile is read-only and must not be classified elevated")

	fullCfg := configWithEnv("https://example.invalid", "tok")
	fullSrv, err := server.New(fullCfg)
	requireNoError(t, err)

	fullProfile := fullSrv.ActiveProfile()
	assertTrue(t, profiles.ProfileIsElevated(&fullProfile),
		"full-access carries :read_write scopes and must be classified elevated")
}
