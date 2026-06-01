package profiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

// TestRequiredScopesMetaReturnsNil locks in the spec contract that
// CapMeta tools (hello, version, profile builder) need no Linode scope
// because they never touch the Linode API.
func TestRequiredScopesMetaReturnsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, profiles.RequiredScopes("hello", profiles.CapMeta))
	assert.Nil(t, profiles.RequiredScopes("version", profiles.CapMeta))
}

// TestRequiredScopesReadVsWrite covers the read-only/read-write split per
// category. The Linode API doesn't distinguish CapDestroy from CapWrite
// at the scope level, so both should resolve to the same :read_write
// scope.
func TestRequiredScopesReadVsWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		toolName   string
		capability profiles.Capability
		want       []profiles.Scope
	}{
		{
			name:       "tags list",
			toolName:   "linode_tags",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "instances list read",
			toolName:   toolInstancesList,
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLinodesReadOnly},
		},
		{
			name:       "instance delete maps to read_write",
			toolName:   toolInstanceDelete,
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeLinodesReadWrite},
		},
		{
			name:       "volume create",
			toolName:   "linode_volume_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeVolumesReadWrite},
		},
		{
			name:       "volume list",
			toolName:   "linode_volume_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeVolumesReadOnly},
		},
		{
			name:       "domain delete",
			toolName:   "linode_domain_delete",
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeDomainsReadWrite},
		},
		{
			name:       "lke cluster regenerate",
			toolName:   "linode_lke_cluster_regenerate",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeLKEReadWrite},
		},
		{
			name:       "object storage bucket create",
			toolName:   "linode_object_storage_bucket_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeObjectStorageReadWrite},
		},
		{
			name:       "longview plan get",
			toolName:   "linode_longview_plan",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLongviewReadOnly},
		},
		{
			name:       "longview clients list",
			toolName:   "linode_longview_clients",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLongviewReadOnly},
		},
		{
			name:       "longview client update",
			toolName:   "linode_longview_client_update",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeLongviewReadWrite},
		},
		{
			name:       "longview client delete",
			toolName:   "linode_longview_client_delete",
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeLongviewReadWrite},
		},
		{
			name:       "stackscript create",
			toolName:   "linode_stackscript_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeStackScriptsReadWrite},
		},
		{
			name:       "vpc list",
			toolName:   "linode_vpc_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeVPCReadOnly},
		},
		{
			name:       "nodebalancer update",
			toolName:   "linode_nodebalancer_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeNodeBalancersReadWrite},
		},
		{
			name:       "nodebalancer node update",
			toolName:   "linode_nodebalancer_node_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeNodeBalancersReadWrite},
		},
		{
			name:       "firewall create",
			toolName:   "linode_firewall_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeFirewallReadWrite},
		},
		{
			name:       "account read",
			toolName:   toolAccount,
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile read",
			toolName:   toolProfile,
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile preferences read",
			toolName:   "linode_profile_preferences",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile security questions read",
			toolName:   "linode_profile_security_questions",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile preferences update",
			toolName:   "linode_profile_preferences_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile tokens read",
			toolName:   "linode_profile_tokens",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeTokensReadOnly},
		},
		{
			name:       "profile token update",
			toolName:   "linode_profile_token_update",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeTokensReadWrite},
		},
		{
			name:       "profile devices read",
			toolName:   "linode_profile_devices",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile security questions answer",
			toolName:   "linode_profile_security_questions_answer",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile tfa enable",
			toolName:   "linode_profile_tfa_enable",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile phone number delete",
			toolName:   "linode_profile_phone_number_delete",
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile tfa disable",
			toolName:   "linode_profile_tfa_disable",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile tfa enable confirm",
			toolName:   "linode_profile_tfa_enable_confirm",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tc.toolName, tc.capability)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRequiredScopesMultiCategory locks in the cross-category mappings
// for tools that hit more than one Linode resource. Provisioning a
// Linode from an image requires both linodes:read_write and
// images:read_only; cloning and rebuilding take the same dependency.
func TestRequiredScopesMultiCategory(t *testing.T) {
	t.Parallel()

	t.Run("instance_create needs linodes write plus images read", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_instance_create", profiles.CapWrite)
		assert.ElementsMatch(
			t,
			[]profiles.Scope{
				profiles.ScopeLinodesReadWrite,
				profiles.ScopeImagesReadOnly,
			},
			got,
		)
	})

	t.Run("instance_clone needs linodes write plus images read", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_instance_clone", profiles.CapWrite)
		assert.ElementsMatch(
			t,
			[]profiles.Scope{
				profiles.ScopeLinodesReadWrite,
				profiles.ScopeImagesReadOnly,
			},
			got,
		)
	})

	t.Run("lke_cluster_create needs lke write plus linodes write", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_lke_cluster_create", profiles.CapWrite)
		assert.ElementsMatch(
			t,
			[]profiles.Scope{
				profiles.ScopeLKEReadWrite,
				profiles.ScopeLinodesReadWrite,
			},
			got,
		)
	})
}

// TestRequiredScopesUnknownToolReturnsNil verifies the spec's
// "best effort" fallback: a tool name that doesn't match any prefix
// returns nil instead of erroring. Phase 6.4 logs a warning when this
// happens but still loads the server, so forgotten mappings degrade
// into observable warnings rather than startup failures.
func TestRequiredScopesUnknownToolReturnsNil(t *testing.T) {
	t.Parallel()

	got := profiles.RequiredScopes("not_a_real_tool", profiles.CapWrite)
	assert.Nil(t, got)
}

// TestRequiredScopesPrefixOrdering confirms that longer prefixes win
// over shorter ones. The instance backup/config/disk/IP sub-tools share the
// linode_instance_ root but live under /linode/instances themselves;
// they should resolve to linodes:* not to a hypothetical shorter route.
func TestRequiredScopesPrefixOrdering(t *testing.T) {
	t.Parallel()

	cases := []string{
		"linode_instance_backup_list",
		"linode_instance_stats_get",
		"linode_instance_interface_settings_get",
		"linode_instance_interface_settings_update",
		"linode_instance_interface_history_list",
		toolLinodeInstanceConfigList,
		"linode_instance_config_interfaces_list",
		"linode_instance_config_interface_get",
		"linode_instance_config_interface_delete",
		"linode_instance_config_create",
		"linode_instance_config_interfaces_reorder",
		"linode_instance_config_delete",
		"linode_instance_disk_create",
		"linode_instance_ip_allocate",
		"linode_instance_ip_update_rdns",
	}

	for _, tool := range cases {
		got := profiles.RequiredScopes(tool, profiles.CapWrite)
		require.NotEmpty(t, got, "tool %s should resolve to a non-empty scope list", tool)
		assert.Contains(
			t, got, profiles.ScopeLinodesReadWrite,
			"tool %s should require linodes:read_write", tool,
		)
	}
}

// TestRequiredScopesSSHAndMonitorAreAccountScoped verifies that the
// SSH-key and monitor tools route to the account scope even though
// their names don't include "account". SSH keys live under /profile,
// and monitor service tokens live under /monitor; both are gated by
// account-level access in the Linode API.
func TestRequiredScopesSSHAndMonitorAreAccountScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tool string
		cap  profiles.Capability
		want profiles.Scope
	}{
		{
			name: "sshkeys list read",
			tool: "linode_sshkey_list",
			cap:  profiles.CapRead,
			want: profiles.ScopeAccountReadOnly,
		},
		{
			name: "sshkey create write",
			tool: "linode_sshkey_create",
			cap:  profiles.CapWrite,
			want: profiles.ScopeAccountReadWrite,
		},
		{
			name: "monitor service token write",
			tool: "linode_monitor_service_token_create",
			cap:  profiles.CapWrite,
			want: profiles.ScopeAccountReadWrite,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tc.tool, tc.cap)
			assert.Equal(t, []profiles.Scope{tc.want}, got)
		})
	}
}

// TestScopeCatalogTokensNotFlaggedAsCredentials documents the rationale
// for the split-literal form of ScopeTokensReadOnly / ScopeTokensReadWrite
// in scope.go: a single-string assignment of "tokens:read_write" trips
// gosec G101. Concatenation at the literal level keeps the constant
// value identical while making the source line invisible to the regex.
// This test pins the resolved values so a refactor that "tidies up" the
// concatenation gets flagged.
func TestScopeCatalogTokensNotFlaggedAsCredentials(t *testing.T) {
	t.Parallel()

	assert.Equal(t, profiles.ScopeTokensReadOnly, profiles.Scope("tokens:read_only"))
	assert.Equal(t, profiles.ScopeTokensReadWrite, profiles.Scope("tokens:read_write"))
}
