package profiles_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestRequiredScopesReservedIPList(t *testing.T) {
	t.Parallel()

	want := []profiles.Scope{profiles.ScopeIPsReadOnly}

	got := profiles.RequiredScopes("linode_networking_reserved_ip_list", profiles.CapRead)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RequiredScopes() = %v, want %v", got, want)
	}
}
