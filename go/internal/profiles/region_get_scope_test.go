package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// Region routes are public: the OpenAPI spec declares no security
// requirement for them, so no token scope is required.
func TestRequiredScopesRegionGet(t *testing.T) {
	t.Parallel()

	if got := profiles.RequiredScopes("linode_region_get", profiles.CapRead); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
