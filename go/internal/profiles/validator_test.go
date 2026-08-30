package profiles_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// The two tools the validator reads through, spelled here because a stub has
// to dispatch on what the real client would be asked for.
const (
	profileTool = "linode_profile_get"
	grantsTool  = "linode_profile_grants_get"
)

// oauthUsername is the username the OAuth fixtures answer with.
const oauthUsername = "oauthuser"

var (
	errUnexpectedTool   = errors.New("unexpected tool")
	errUnexpectedTarget = errors.New("unexpected decode target")
)

// fakeInspector is a stub TokenInspector for the validator tests. It
// keeps a programmable profile/grants response plus optional errors so
// each test case can dial in PAT vs OAuth and the success/failure path
// without spinning up an httptest server.
type fakeInspector struct {
	profile      *linode.Profile
	profileErr   error
	grants       *linode.Grants
	grantsErr    error
	grantsCalled bool
}

// CallRouteJSON answers the two reads the validator makes and refuses any
// other tool, so a validator that reached for a third route fails by name.
func (f *fakeInspector) CallRouteJSON(_ context.Context, tool string, _ []any, out any) error {
	switch tool {
	case profileTool:
		return fill(out, f.profile, f.profileErr)
	case grantsTool:
		f.grantsCalled = true

		return fill(out, f.grants, f.grantsErr)
	}

	return fmt.Errorf("%w: %s", errUnexpectedTool, tool)
}

// fill copies a fixture into the validator's decode target the way the real
// client's JSON decode would, so the stub proves the same wiring.
func fill[T any](out any, fixture *T, err error) error {
	if err != nil {
		return err
	}

	target, ok := out.(*T)
	if !ok {
		return fmt.Errorf("%w: %T", errUnexpectedTarget, out)
	}

	*target = *fixture

	return nil
}

// TestValidateScopesPATPath verifies the personal-access-token path:
// Profile.Scopes is non-empty, so ParsePATScopes drives the actual
// scope set and the grants route is never read.
func TestValidateScopesPATPath(t *testing.T) {
	t.Parallel()

	inspector := &fakeInspector{
		profile: &linode.Profile{
			Username: "patuser",
			Scopes:   "linodes:read_write volumes:read_only",
		},
	}

	required := []profiles.Scope{
		profiles.ScopeLinodesReadWrite,
		profiles.ScopeVolumesReadOnly,
	}

	got, err := profiles.ValidateScopes(t.Context(), inspector, required)
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

	if got.Comparison.HasExcess() {
		t.Error("got.Comparison.HasExcess() = true, want false")
	}

	if inspector.grantsCalled {
		t.Error("inspector.grantsCalled = true, want false")
	}
}

// TestValidateScopesOAuthPath verifies the OAuth path: empty
// Profile.Scopes triggers a /profile/grants fetch, and FlattenGrants
// drives the actual scope set.
func TestValidateScopesOAuthPath(t *testing.T) {
	t.Parallel()

	inspector := &fakeInspector{
		profile: &linode.Profile{
			Username: oauthUsername,
			Scopes:   "",
		},
		grants: &linode.Grants{
			Global: linode.GlobalGrants{
				AddLinodes:    true,
				AccountAccess: "read_only",
			},
		},
	}

	required := []profiles.Scope{profiles.ScopeLinodesReadWrite}

	got, err := profiles.ValidateScopes(t.Context(), inspector, required)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Kind != profiles.TokenKindOAuth {
		t.Errorf("got.Kind = %v, want %v", got.Kind, profiles.TokenKindOAuth)
	}

	if !inspector.grantsCalled {
		t.Error("inspector.grantsCalled = false, want true")
	}

	if got.Comparison.HasMissing() {
		t.Error("got.Comparison.HasMissing() = true, want false")
	}
}

// TestValidateScopesReportsMissing verifies that under-scoped tokens
// surface as Missing rather than an error. The loader's policy
// distinguishes missing (fail) from API failures (also fail, different
// error class).
func TestValidateScopesReportsMissing(t *testing.T) {
	t.Parallel()

	inspector := &fakeInspector{
		profile: &linode.Profile{
			Scopes: "linodes:read_only",
		},
	}

	required := []profiles.Scope{
		profiles.ScopeLinodesReadWrite,
		profiles.ScopeVolumesReadOnly,
	}

	got, err := profiles.ValidateScopes(t.Context(), inspector, required)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !got.Comparison.HasMissing() {
		t.Error("got.Comparison.HasMissing() = false, want true")
	}

	if !reflect.DeepEqual(got.Comparison.Missing, []profiles.Scope{
		profiles.ScopeLinodesReadWrite,
		profiles.ScopeVolumesReadOnly,
	}) {
		t.Errorf("got.Comparison.Missing = %v, want %v", got.Comparison.Missing, []profiles.Scope{
			profiles.ScopeLinodesReadWrite,
			profiles.ScopeVolumesReadOnly,
		})
	}
}

// TestValidateScopesProfileErrorWrapped confirms that a failed profile
// read bubbles up wrapped in ErrProfileFetchFailed so callers can
// pattern-match on it.
func TestValidateScopesProfileErrorWrapped(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("network down")
	inspector := &fakeInspector{profileErr: apiErr}

	got, err := profiles.ValidateScopes(t.Context(), inspector, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if !errors.Is(err, profiles.ErrProfileFetchFailed) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrProfileFetchFailed)
	}

	if !errors.Is(err, apiErr) {
		t.Fatalf("error = %v, want %v", err, apiErr)
	}
}

// TestValidateScopesGrantsErrorWrapped covers the OAuth-path failure:
// /profile succeeded (empty Scopes signals OAuth) but /profile/grants
// failed. The error must wrap ErrGrantsFetchFailed.
func TestValidateScopesGrantsErrorWrapped(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("rate limited")
	inspector := &fakeInspector{
		profile:   &linode.Profile{Scopes: ""},
		grantsErr: apiErr,
	}

	got, err := profiles.ValidateScopes(t.Context(), inspector, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if !errors.Is(err, profiles.ErrGrantsFetchFailed) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrGrantsFetchFailed)
	}

	if !errors.Is(err, apiErr) {
		t.Fatalf("error = %v, want %v", err, apiErr)
	}
}

// TestValidateScopesReadsTheDeclaredProfileRoutes runs the real client
// against a stub API, so the two typed reads are proven to reach GET
// /profile and GET /profile/grants. An empty scope string sends the
// validator down the OAuth path, the one that makes both reads.
func TestValidateScopesReadsTheDeclaredProfileRoutes(t *testing.T) {
	t.Parallel()

	bodies := map[string]any{
		"/profile":        linode.Profile{Username: oauthUsername},
		"/profile/grants": linode.Grants{Global: linode.GlobalGrants{AddLinodes: true}},
	}

	var (
		requestsMu sync.Mutex
		requests   []string
	)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()

		requests = append(requests, r.Method+" "+r.URL.Path)

		requestsMu.Unlock()

		body, known := bodies[r.URL.Path]
		if !known {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode %s: %v", r.URL.Path, err)
		}
	}))
	defer httpSrv.Close()

	client := linode.NewClient(httpSrv.URL, "oauth-token", nil, linode.WithMaxRetries(0))

	got, err := profiles.ValidateScopes(t.Context(), client, []profiles.Scope{profiles.ScopeLinodesReadWrite})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestsMu.Lock()
	defer requestsMu.Unlock()

	if want := []string{"GET /profile", "GET /profile/grants"}; !slices.Equal(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}

	if got.Kind != profiles.TokenKindOAuth {
		t.Errorf("got.Kind = %v, want %v", got.Kind, profiles.TokenKindOAuth)
	}

	if got.Profile.Username != oauthUsername {
		t.Errorf("got.Profile.Username = %q, want %q", got.Profile.Username, oauthUsername)
	}

	if got.Comparison.HasMissing() {
		t.Error("got.Comparison.HasMissing() = true, want false: add_linodes grants linodes:read_write")
	}
}

// TestTokenKindString locks in the user-facing names so log messages
// and audit fields stay stable when the constants get reordered.
func TestTokenKindString(t *testing.T) {
	t.Parallel()

	if profiles.TokenKindUnknown.String() != "Unknown" {
		t.Errorf("profiles.TokenKindUnknown.String() = %v, want %v", profiles.TokenKindUnknown.String(), "Unknown")
	}

	if profiles.TokenKindPAT.String() != "PAT" {
		t.Errorf("profiles.TokenKindPAT.String() = %v, want %v", profiles.TokenKindPAT.String(), "PAT")
	}

	if profiles.TokenKindOAuth.String() != "OAuth" {
		t.Errorf("profiles.TokenKindOAuth.String() = %v, want %v", profiles.TokenKindOAuth.String(), "OAuth")
	}
}

// TestProfileIsElevatedReadsCapabilityFlag pins the elevation policy to
// the capability-derived Elevated field. Scope suffixes must not drive
// it: the API documents :read_write scopes on several read-only routes
// (kubeconfig, managed contacts, instance interfaces), so a read-only
// profile whose scope union contains write scopes still starts
// tokenless with a warning.
func TestProfileIsElevatedReadsCapabilityFlag(t *testing.T) {
	t.Parallel()

	if profiles.ProfileIsElevated(nil) {
		t.Error("nil profile must not be elevated")
	}

	readOnly := profiles.Profile{
		Name:                "readonly",
		RequiredTokenScopes: []string{"account:read_write", "lke:read_write"},
		Elevated:            false,
	}
	if profiles.ProfileIsElevated(&readOnly) {
		t.Error("write scopes alone must not elevate a read-only profile")
	}

	mutating := profiles.Profile{
		Name:                "writer",
		RequiredTokenScopes: []string{"linodes:read_only"},
		Elevated:            true,
	}
	if !profiles.ProfileIsElevated(&mutating) {
		t.Error("a profile with mutating tools must be elevated")
	}
}
