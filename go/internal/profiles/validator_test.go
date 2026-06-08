package profiles_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
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

func (f *fakeInspector) GetProfile(_ context.Context) (*linode.Profile, error) {
	return f.profile, f.profileErr
}

func (f *fakeInspector) GetProfileGrants(_ context.Context) (*linode.Grants, error) {
	f.grantsCalled = true

	return f.grants, f.grantsErr
}

// TestValidateScopesPATPath verifies the personal-access-token path:
// Profile.Scopes is non-empty, so ParsePATScopes drives the actual
// scope set and GetProfileGrants is never called.
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
			Username: "oauthuser",
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

// TestValidateScopesProfileErrorWrapped confirms that GetProfile
// failures bubble up wrapped in ErrProfileFetchFailed so callers can
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
