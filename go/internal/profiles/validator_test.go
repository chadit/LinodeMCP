package profiles_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, profiles.TokenKindPAT, got.Kind,
		"non-empty Profile.Scopes must be classified as PAT")
	assert.False(t, got.Comparison.HasMissing())
	assert.False(t, got.Comparison.HasExcess())
	assert.False(t, inspector.grantsCalled,
		"GetProfileGrants must not be called on the PAT path")
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
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, profiles.TokenKindOAuth, got.Kind,
		"empty Profile.Scopes must be classified as OAuth")
	assert.True(t, inspector.grantsCalled,
		"OAuth path must call GetProfileGrants")
	assert.False(t, got.Comparison.HasMissing(),
		"add_linodes implies linodes:read_write, so nothing is missing")
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
	require.NoError(t, err, "missing scopes must not surface as an error")
	require.NotNil(t, got)

	assert.True(t, got.Comparison.HasMissing())
	assert.Equal(t, []profiles.Scope{
		profiles.ScopeLinodesReadWrite,
		profiles.ScopeVolumesReadOnly,
	}, got.Comparison.Missing)
}

// TestValidateScopesProfileErrorWrapped confirms that GetProfile
// failures bubble up wrapped in ErrProfileFetchFailed so callers can
// pattern-match on it.
func TestValidateScopesProfileErrorWrapped(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("network down")
	inspector := &fakeInspector{profileErr: apiErr}

	got, err := profiles.ValidateScopes(t.Context(), inspector, nil)
	require.Error(t, err)
	assert.Nil(t, got, "result must be nil on profile fetch failure")
	require.ErrorIs(t, err, profiles.ErrProfileFetchFailed,
		"error must wrap profiles.ErrProfileFetchFailed for callers to pattern-match")
	require.ErrorIs(t, err, apiErr,
		"wrapper must preserve the underlying API error in the chain")
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
	require.Error(t, err)
	assert.Nil(t, got)
	require.ErrorIs(t, err, profiles.ErrGrantsFetchFailed,
		"OAuth-path failure must wrap profiles.ErrGrantsFetchFailed")
	require.ErrorIs(t, err, apiErr)
}

// TestTokenKindString locks in the user-facing names so log messages
// and audit fields stay stable when the constants get reordered.
func TestTokenKindString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Unknown", profiles.TokenKindUnknown.String())
	assert.Equal(t, "PAT", profiles.TokenKindPAT.String())
	assert.Equal(t, "OAuth", profiles.TokenKindOAuth.String())
}
