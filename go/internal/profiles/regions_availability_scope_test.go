package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// Region availability routes are public: the OpenAPI spec declares no
// security requirement for them, so no token scope is required.
func TestRequiredScopesRegionAvailabilityList(t *testing.T) {
	t.Parallel()

	if got := profiles.RequiredScopes("linode_region_availability_list", profiles.CapRead); got != nil {
		t.Errorf("got %v, want nil", got)
	}

	if got := profiles.RequiredScopes("linode_region_availability_get", profiles.CapRead); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
