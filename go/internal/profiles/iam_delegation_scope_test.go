package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// The account-delegation routes are gated on the token alone: the OpenAPI spec
// lists personalAccessToken and no OAuth scope for any of them, including the
// two replacements and the delegate-token mint. A scope invented for them here
// would refuse a token the API accepts.
func TestRequiredScopesIamDelegationSurface(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool       string
		capability profiles.Capability
	}{
		{"linode_iam_delegation_child_account_list", profiles.CapRead},
		{"linode_iam_delegation_child_account_user_list", profiles.CapRead},
		{"linode_iam_delegation_child_account_user_update", profiles.CapAdmin},
		{"linode_iam_delegation_default_role_permission_get", profiles.CapRead},
		{"linode_iam_delegation_default_role_permission_update", profiles.CapAdmin},
		{"linode_iam_delegation_profile_child_account_list", profiles.CapRead},
		{"linode_iam_delegation_profile_child_account_get", profiles.CapRead},
		{"linode_iam_delegation_profile_child_account_token_create", profiles.CapAdmin},
		{"linode_iam_delegation_user_child_account_list", profiles.CapRead},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			if got := profiles.RequiredScopes(testCase.tool, testCase.capability); got != nil {
				t.Errorf("got %v, want nil", got)
			}
		})
	}
}
