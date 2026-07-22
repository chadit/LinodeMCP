package profiles_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestRequiredScopesInstanceConfigInterfacesList(t *testing.T) {
	t.Parallel()

	got := profiles.RequiredScopes("linode_instance_config_interface_list", profiles.CapRead)
	want := []profiles.Scope{profiles.ScopeLinodesReadOnly}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v, want %v", got, want)
	}
}
