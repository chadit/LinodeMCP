package server_test

import (
	"encoding/json"
	"errors"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := srv.ValidateScopes(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if !errors.Is(err, profiles.ErrTokenNotConfigured) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrTokenNotConfigured)
	}
}

// TestValidateScopesPATPathSucceeds covers the happy path: a PAT
// response carrying every required scope produces a comparison with
// no missing entries. Uses httptest so the test exercises the real
// Linode client code path.
func TestValidateScopesPATPathSucceeds(t *testing.T) {
	t.Parallel()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Profile{
			Username: "u",
			Scopes:   "*",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer httpSrv.Close()

	cfg := configWithEnv(httpSrv.URL, "pat-token")

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := srv.ValidateScopes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Kind != profiles.TokenKindPAT {
		t.Errorf("got.Kind = %v, want %v", got.Kind, profiles.TokenKindPAT)
	}

	if got.Comparison.HasMissing() {
		t.Error("got.Comparison.HasMissing() = true, want false")
	}
}

// TestValidateScopesMissingScopesReportedInComparison verifies that
// under-scoped tokens do NOT produce an error from Server.ValidateScopes;
// the missing entries appear in the Comparison so main can format them
// into a user-facing error message.
func TestValidateScopesMissingScopesReportedInComparison(t *testing.T) {
	t.Parallel()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Profile{
			Username: "u",
			Scopes:   "linodes:read_only",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer httpSrv.Close()

	cfg := configWithEnv(httpSrv.URL, "pat-token")

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := srv.ValidateScopes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !got.Comparison.HasMissing() {
		t.Error("got.Comparison.HasMissing() = false, want true")
	}
}

// TestProfileIsElevated covers the missing-token policy helper. Default
// and readonly-full carry only :read_only scopes and must not be
// flagged elevated; full-access and compute-admin must be.
func TestProfileIsElevated(t *testing.T) {
	t.Parallel()

	defaultCfg := baseTestConfig()

	srv, err := server.New(defaultCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaultProfile := srv.ActiveProfile()
	if profiles.ProfileIsElevated(&defaultProfile) {
		t.Error("expected condition to be false")
	}

	fullCfg := configWithEnv("https://example.invalid", "tok")

	fullSrv, err := server.New(fullCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fullProfile := fullSrv.ActiveProfile()
	if !profiles.ProfileIsElevated(&fullProfile) {
		t.Error("expected condition to be true")
	}
}
