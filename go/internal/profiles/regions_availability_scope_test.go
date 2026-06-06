package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRequiredScopesRegionAvailabilityList(t *testing.T) {
	t.Parallel()

	assertEqual(
		t,
		[]profiles.Scope{profiles.ScopeLinodesReadOnly},
		profiles.RequiredScopes("linode_region_availability_list", profiles.CapRead),
		"region availability list should use read-only linodes scope",
	)
	assertEqual(
		t,
		[]profiles.Scope{profiles.ScopeLinodesReadOnly},
		profiles.RequiredScopes("linode_region_availability_get", profiles.CapRead),
		"region availability get should use read-only linodes scope",
	)
}
