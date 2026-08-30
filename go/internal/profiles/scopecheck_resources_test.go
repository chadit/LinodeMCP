package profiles_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// TestFlattenGrantsAddDatabasesAndLongviewImplyWritePairs covers the two
// global add flags the wider add-flag test leaves out. A token that may add
// databases or Longview clients must land on the matching write pair and
// nothing else, or a profile requiring those scopes would fail against a
// token that can already create the resource.
func TestFlattenGrantsAddDatabasesAndLongviewImplyWritePairs(t *testing.T) {
	t.Parallel()

	got := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{AddDatabases: true, AddLongview: true},
	})

	want := []profiles.Scope{
		profiles.ScopeDatabasesReadOnly,
		profiles.ScopeDatabasesReadWrite,
		profiles.ScopeLongviewReadOnly,
		profiles.ScopeLongviewReadWrite,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FlattenGrants(add databases, add longview) = %v, want %v", got, want)
	}
}

// TestFlattenGrantsMapsEachResourceListToItsCategory walks every per-resource
// grant list the flattener reads. Each row grants one resource read_write and
// expects exactly that category's pair, so a list wired to the wrong scope
// constants, or dropped from the walk, fails on its own row.
func TestFlattenGrantsMapsEachResourceListToItsCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		grants   func(perm linode.GrantPermission) *linode.Grants
		name     string
		wantPair []profiles.Scope
	}{
		{
			name: "nodebalancer",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{NodeBalancer: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeNodeBalancersReadOnly, profiles.ScopeNodeBalancersReadWrite},
		},
		{
			name: "image",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{Image: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeImagesReadOnly, profiles.ScopeImagesReadWrite},
		},
		{
			name: "longview",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{Longview: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeLongviewReadOnly, profiles.ScopeLongviewReadWrite},
		},
		{
			name: "stackscript",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{StackScript: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeStackScriptsReadOnly, profiles.ScopeStackScriptsReadWrite},
		},
		{
			name: "database",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{Database: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeDatabasesReadOnly, profiles.ScopeDatabasesReadWrite},
		},
		{
			name: "firewall",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{Firewall: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeFirewallReadOnly, profiles.ScopeFirewallReadWrite},
		},
		{
			name: "vpc",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{VPC: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeVPCReadOnly, profiles.ScopeVPCReadWrite},
		},
		{
			name: "lke cluster",
			grants: func(perm linode.GrantPermission) *linode.Grants {
				return &linode.Grants{LKECluster: []linode.Grant{{ID: 1, Permissions: perm}}}
			},
			wantPair: []profiles.Scope{profiles.ScopeLKEReadOnly, profiles.ScopeLKEReadWrite},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := profiles.FlattenGrants(tt.grants(permReadWrite)); !reflect.DeepEqual(got, tt.wantPair) {
				t.Errorf("read_write grant flattened to %v, want %v", got, tt.wantPair)
			}

			wantReadOnly := tt.wantPair[:1]
			if got := profiles.FlattenGrants(tt.grants(permReadOnly)); !reflect.DeepEqual(got, wantReadOnly) {
				t.Errorf("read_only grant flattened to %v, want %v", got, wantReadOnly)
			}

			if got := profiles.FlattenGrants(tt.grants("")); len(got) != 0 {
				t.Errorf("empty permission flattened to %v, want nothing granted", got)
			}
		})
	}
}
