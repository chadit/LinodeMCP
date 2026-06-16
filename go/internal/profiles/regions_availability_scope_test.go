package profiles_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestRequiredScopesRegionAvailabilityList(t *testing.T) {
	t.Parallel()

	if !reflect.DeepEqual(profiles.RequiredScopes("linode_region_availability_list", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly}) {
		t.Errorf("got %v, want %v", profiles.RequiredScopes("linode_region_availability_list", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly})
	}

	if !reflect.DeepEqual(profiles.RequiredScopes("linode_region_availability_get", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly}) {
		t.Errorf("got %v, want %v", profiles.RequiredScopes("linode_region_availability_get", profiles.CapRead), []profiles.Scope{profiles.ScopeLinodesReadOnly})
	}
}
