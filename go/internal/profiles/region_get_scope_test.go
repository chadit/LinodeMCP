package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRequiredScopesRegionGet(t *testing.T) {
	t.Parallel()

	assertEqual(
		t,
		[]profiles.Scope{profiles.ScopeLinodesReadOnly},
		profiles.RequiredScopes("linode_region_get", profiles.CapRead),
		"region get should use read-only linodes scope",
	)
}
