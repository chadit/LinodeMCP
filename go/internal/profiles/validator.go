package profiles

import (
	"context"
	"fmt"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// TokenKind identifies whether the active token is a personal access
// token (scopes embedded in /profile) or an OAuth token (grants on
// /profile/grants). The validator picks the right code path based on
// what /profile returns; consumers use the kind for logging.
type TokenKind int

const (
	// TokenKindUnknown is the zero value. Set when validation has not
	// run yet, or when the /profile response was malformed.
	TokenKindUnknown TokenKind = iota
	// TokenKindPAT marks a personal access token. PATs carry their
	// scope string directly on the /profile response.
	TokenKindPAT
	// TokenKindOAuth marks an OAuth token. OAuth tokens have an empty
	// Scopes field; the validator falls back to /profile/grants.
	TokenKindOAuth
)

// String returns the token kind name for log lines and error messages.
func (k TokenKind) String() string {
	switch k {
	case TokenKindUnknown:
		return "Unknown"
	case TokenKindPAT:
		return "PAT"
	case TokenKindOAuth:
		return "OAuth"
	default:
		return "TokenKind(invalid)"
	}
}

// TokenInspector is the minimal Linode client surface ValidateScopes
// needs. The real *linode.RetryableClient satisfies this interface
// without changes; tests inject a stub so scope validation logic stays
// network-free.
type TokenInspector interface {
	GetProfile(ctx context.Context) (*linode.Profile, error)
	GetProfileGrants(ctx context.Context) (*linode.Grants, error)
}

// ScopeValidationResult is what ValidateScopes returns to its caller.
// The caller (main.go in production, tests in unit) decides what to do
// with the comparison: a Missing set is always a hard fail, an Excess
// set is a warn by default, fail under strict mode.
type ScopeValidationResult struct {
	// Kind tells whether the active token is a PAT or OAuth.
	Kind TokenKind

	// ActualScopes is the deduplicated, sorted scope set the token
	// actually carries. For PATs this comes from Profile.Scopes; for
	// OAuth it comes from FlattenGrants.
	ActualScopes []Scope

	// Comparison holds the Missing/Excess diff against the required
	// scope list ValidateScopes was given.
	Comparison ScopeComparison

	// Profile is the /profile response, for callers that want the
	// username, restricted flag, or UID for logging. Never nil on a
	// successful return.
	Profile *linode.Profile
}

// ValidateScopes inspects the token's actual scopes and returns the
// diff against required. The PAT path uses Profile.Scopes directly;
// OAuth tokens (empty Profile.Scopes) trigger a second call to
// /profile/grants and the result is flattened via FlattenGrants.
//
// Policy decisions live in the caller: this function reports facts
// only. A Comparison with Missing entries is a load-time failure under
// the spec, but ValidateScopes itself returns nil error for that case
// so callers can inspect both Missing and Excess in one place.
//
// Errors returned from this function are network/API failures only.
// The PAT path returns ErrProfileFetchFailed wrapping the underlying
// error; the OAuth path returns ErrGrantsFetchFailed.
func ValidateScopes(
	ctx context.Context,
	inspector TokenInspector,
	required []Scope,
) (*ScopeValidationResult, error) {
	profile, err := inspector.GetProfile(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProfileFetchFailed, err)
	}

	if profile.Scopes != "" {
		actual := ParsePATScopes(profile.Scopes)

		return &ScopeValidationResult{
			Kind:         TokenKindPAT,
			ActualScopes: actual,
			Comparison:   CompareScopes(required, actual),
			Profile:      profile,
		}, nil
	}

	grants, err := inspector.GetProfileGrants(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGrantsFetchFailed, err)
	}

	actual := FlattenGrants(grants)

	return &ScopeValidationResult{
		Kind:         TokenKindOAuth,
		ActualScopes: actual,
		Comparison:   CompareScopes(required, actual),
		Profile:      profile,
	}, nil
}
