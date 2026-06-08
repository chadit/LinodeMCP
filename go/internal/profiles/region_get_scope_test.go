package profiles_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRequiredScopesRegionGet(t *testing.T) {
	t.Parallel()

	if !reflect.DeepEqual(profiles.RequiredScopes("linode_region_get", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly}) {
		t.Errorf("got %v, want %v", profiles.RequiredScopes("linode_region_get", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly})
	}
}
