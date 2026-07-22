package profiles_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// TestRequiredScopesForTagCreate pins POST /tags to account:read_write
// alone, matching the operation's documented OAuth scope. Access to the
// entities being tagged is enforced by the API through grants at request
// time, not through extra token scopes, so the old cross-category union
// overstated what the token must carry.
func TestRequiredScopesForTagCreate(t *testing.T) {
	t.Parallel()

	want := []profiles.Scope{profiles.ScopeAccountReadWrite}
	if !reflect.DeepEqual(profiles.RequiredScopes("linode_tag_create", profiles.CapWrite), want) {
		t.Errorf("got %v, want %v", profiles.RequiredScopes("linode_tag_create", profiles.CapWrite), want)
	}
}

// TestRequiredScopesMetaReturnsNil locks in the spec contract that
// CapMeta tools (hello, version, profile builder) need no Linode scope
// because they never touch the Linode API.
func TestRequiredScopesMetaReturnsNil(t *testing.T) {
	t.Parallel()

	if profiles.RequiredScopes("hello", profiles.CapMeta) != nil {
		t.Errorf("value = %v, want nil", profiles.RequiredScopes("hello", profiles.CapMeta))
	}

	if profiles.RequiredScopes("version", profiles.CapMeta) != nil {
		t.Errorf("value = %v, want nil", profiles.RequiredScopes("version", profiles.CapMeta))
	}
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
			toolName:   "linode_tag_list",
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
			toolName:   toolVolumesList,
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
			toolName:   "linode_longview_plan_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLongviewReadOnly},
		},
		{
			name:       "longview clients list",
			toolName:   "linode_longview_client_list",
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
			name:       "vpc create",
			toolName:   "linode_vpc_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeVPCReadWrite},
		},
		{
			name:       "nodebalancer update",
			toolName:   "linode_nodebalancer_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeNodeBalancersReadWrite},
		},
		{
			name:       "nodebalancer node update",
			toolName:   "linode_nodebalancer_config_node_update",
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
			name:       "profile preferences read",
			toolName:   "linode_profile_preferences_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile security questions read",
			toolName:   "linode_profile_security_question_list",
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
			// The API gates /profile/tokens with account scopes, not a
			// dedicated tokens scope.
			name:       "profile tokens read",
			toolName:   "linode_profile_token_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile token update",
			toolName:   "linode_profile_token_update",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile devices read",
			toolName:   "linode_profile_device_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile security questions answer",
			toolName:   "linode_profile_security_question_answer",
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tt.toolName, tt.capability)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRequiredScopesReconciledFamilies pins the families the scope
// parity gate found diverging between Go and Python: databases, managed,
// support tickets, placement groups, tags, the profile subtree, and the
// firewall-settings split. Each expectation comes from the security
// block of the underlying operation in the Linode OpenAPI spec.
func TestRequiredScopesReconciledFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		toolName   string
		capability profiles.Capability
		want       []profiles.Scope
	}{
		{
			name:       "profile token create",
			toolName:   "linode_profile_token_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "profile app list",
			toolName:   "linode_profile_app_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "profile login get",
			toolName:   "linode_profile_login_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "managed service list",
			toolName:   "linode_managed_service_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "managed service create",
			toolName:   "linode_managed_service_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "managed credential get is admin gated",
			toolName:   "linode_managed_credential_get",
			capability: profiles.CapAdmin,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			// Event routes live under /account but the API gates them
			// with events:* scopes.
			name:       "account event list",
			toolName:   "linode_account_event_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeEventsReadOnly},
		},
		{
			name:       "account event get",
			toolName:   "linode_account_event_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeEventsReadOnly},
		},
		{
			// POST .../seen is documented with only events:read_only;
			// scopeOverrides mirrors the spec.
			name:       "account event seen",
			toolName:   "linode_account_event_seen",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeEventsReadOnly},
		},
		{
			name:       "support ticket list",
			toolName:   "linode_support_ticket_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "support ticket create",
			toolName:   "linode_support_ticket_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "support ticket reply list",
			toolName:   "linode_support_ticket_reply_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "tag object list",
			toolName:   "linode_tag_object_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadOnly},
		},
		{
			name:       "database mysql instance list",
			toolName:   toolDatabaseMySQLInstanceList,
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadOnly},
		},
		{
			name:       "database postgresql instance create",
			toolName:   "linode_database_postgresql_instance_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadWrite},
		},
		{
			name:       "database mysql instance delete",
			toolName:   toolDatabaseMySQLInstanceDelete,
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadWrite},
		},
		{
			name:       "database mysql config read",
			toolName:   toolDatabaseMySQLConfigGet,
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadOnly},
		},
		{
			name:       "placement group get",
			toolName:   "linode_placement_group_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLinodesReadOnly},
		},
		{
			name:       "placement group create",
			toolName:   "linode_placement_group_create",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeLinodesReadWrite},
		},
		{
			name:       "placement group assign",
			toolName:   "linode_placement_group_assign",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeLinodesReadWrite},
		},
		{
			// The API documents placement:read_only for this one route,
			// but its own OAuth catalog never defines that scope, so the
			// tool stays on the family's linodes derivation. See
			// scopeOverrides for the full rationale.
			name:       "placement group list stays on linodes",
			toolName:   "linode_placement_group_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLinodesReadOnly},
		},
		{
			// GET /managed/contacts/{id} is documented account:read_write
			// despite being a read (contacts hold PII); scopeOverrides
			// mirrors the spec, and the elevation policy derives from
			// capabilities so this write scope does not flip it.
			name:       "managed contact get needs account write",
			toolName:   "linode_managed_contact_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "firewall settings read stays in firewall",
			toolName:   "linode_firewall_settings_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeFirewallReadOnly},
		},
		{
			name:       "networking ip list",
			toolName:   "linode_networking_ip_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeIPsReadOnly},
		},
		{
			name:       "networking ip update",
			toolName:   "linode_networking_ip_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeIPsReadWrite},
		},
		{
			// The docs list this route as "ips:read", an upstream typo:
			// the scope catalog only defines read_only and read_write,
			// so the family's read_only applies.
			name:       "ipv6 range get",
			toolName:   "linode_ipv6_range_get",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeIPsReadOnly},
		},
		{
			name:       "ipv6 pool list",
			toolName:   "linode_ipv6_pool_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeIPsReadOnly},
		},
		{
			name:       "vlan list",
			toolName:   "linode_vlan_list",
			capability: profiles.CapRead,
			want:       []profiles.Scope{profiles.ScopeLinodesReadOnly},
		},
		{
			name:       "vlan delete",
			toolName:   "linode_vlan_delete",
			capability: profiles.CapDestroy,
			want:       []profiles.Scope{profiles.ScopeLinodesReadWrite},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tt.toolName, tt.capability)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRequiredScopesIPAssignmentNeedsLinodesWrite locks in the dual
// scope on address assignment and sharing: those operations target
// Linodes, so the API documents linodes:read_write alongside
// ips:read_write.
func TestRequiredScopesIPAssignmentNeedsLinodesWrite(t *testing.T) {
	t.Parallel()

	cases := []string{
		"linode_networking_ip_assign",
		"linode_networking_ip_share",
		"linode_networking_ipv4_assign",
		"linode_networking_ipv4_share",
		"linode_ipv6_range_create",
	}

	for _, tool := range cases {
		got := profiles.RequiredScopes(tool, profiles.CapWrite)

		gotEls := slices.Clone(got)
		wantEls := []profiles.Scope{profiles.ScopeIPsReadWrite, profiles.ScopeLinodesReadWrite}

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("RequiredScopes(%s) = %v, want %v (any order)", tool, got, wantEls)
		}
	}
}

// TestRequiredScopesMultiCategory locks in the cross-category mappings
// for tools that hit more than one Linode resource. Provisioning a
// Linode from an image requires both linodes:read_write and
// images:read_only; cloning and rebuilding take the same dependency.
func TestRequiredScopesMultiCategory(t *testing.T) {
	t.Parallel()

	t.Run("volume_attach needs volumes write plus linodes write", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_volume_attach", profiles.CapWrite)
		{
			gotEls := slices.Clone(got)
			wantEls := slices.Clone([]profiles.Scope{
				profiles.ScopeVolumesReadWrite,
				profiles.ScopeLinodesReadWrite,
			})

			slices.Sort(gotEls)
			slices.Sort(wantEls)

			if !slices.Equal(gotEls, wantEls) {
				t.Errorf("got %v, want %v (any order)", got, wantEls)
			}
		}
	})

	t.Run("image_create needs images write plus linodes read", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_image_create", profiles.CapWrite)
		{
			gotEls := slices.Clone(got)
			wantEls := slices.Clone([]profiles.Scope{
				profiles.ScopeImagesReadWrite,
				profiles.ScopeLinodesReadOnly,
			})

			slices.Sort(gotEls)
			slices.Sort(wantEls)

			if !slices.Equal(gotEls, wantEls) {
				t.Errorf("got %v, want %v (any order)", got, wantEls)
			}
		}
	})

	// Instance provisioning and LKE cluster creation document a single
	// scope: image and Linode access is grant-enforced by the API at
	// request time, so the old cross-category extras overstated what
	// the token must carry.
	t.Run("instance_create needs only linodes write", func(t *testing.T) {
		t.Parallel()

		for _, tool := range []string{
			toolInstanceCreate, "linode_instance_clone", "linode_instance_rebuild",
		} {
			got := profiles.RequiredScopes(tool, profiles.CapWrite)
			if !reflect.DeepEqual(got, []profiles.Scope{profiles.ScopeLinodesReadWrite}) {
				t.Errorf("RequiredScopes(%s) = %v, want [linodes:read_write]", tool, got)
			}
		}
	})

	t.Run("lke_cluster_create needs only lke write", func(t *testing.T) {
		t.Parallel()

		got := profiles.RequiredScopes("linode_lke_cluster_create", profiles.CapWrite)
		if !reflect.DeepEqual(got, []profiles.Scope{profiles.ScopeLKEReadWrite}) {
			t.Errorf("got %v, want [lke:read_write]", got)
		}
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
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
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
		"linode_instance_config_interface_list",
		"linode_instance_config_interface_get",
		"linode_instance_config_interface_delete",
		"linode_instance_config_create",
		"linode_instance_config_interface_reorder",
		"linode_instance_config_delete",
		"linode_instance_disk_create",
		"linode_instance_ip_allocate",
		"linode_instance_ip_update",
	}

	for _, tool := range cases {
		got := profiles.RequiredScopes(tool, profiles.CapWrite)
		if len(got) == 0 {
			t.Fatal("got is empty")
		}

		if !slices.Contains(got, profiles.ScopeLinodesReadWrite) {
			t.Errorf("got does not contain %v", profiles.ScopeLinodesReadWrite)
		}
	}
}

// TestRequiredScopesSSHAndMonitorScopes verifies the two subtrees whose
// names don't spell their category: SSH keys live under /profile, which
// is account-gated, while the /monitor routes carry their own monitor:*
// scopes per the API docs.
func TestRequiredScopesSSHAndMonitorScopes(t *testing.T) {
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
			name: "monitor dashboard list",
			tool: "linode_monitor_dashboard_list",
			cap:  profiles.CapRead,
			want: profiles.ScopeMonitorReadOnly,
		},
		{
			name: "monitor alert definition update",
			tool: "linode_monitor_service_alert_definition_update",
			cap:  profiles.CapWrite,
			want: profiles.ScopeMonitorReadWrite,
		},
		{
			// POST /monitor/services/{type}/token is documented with
			// only monitor:read_only; scopeOverrides mirrors the spec.
			name: "monitor service token write",
			tool: "linode_monitor_service_token_create",
			cap:  profiles.CapWrite,
			want: profiles.ScopeMonitorReadOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tt.tool, tt.cap)
			if !reflect.DeepEqual(got, []profiles.Scope{tt.want}) {
				t.Errorf("got = %v, want %v", got, []profiles.Scope{tt.want})
			}
		})
	}
}

// TestRequiredScopesScopelessRoutes pins the tools whose documented
// routes carry no OAuth scope requirement. Catalog, pricing, and region
// routes are public (no authentication at all); betas, maintenance, the
// caller's own profile, Longview subscription plans, VPC reads, the
// OAuth-client thumbnail, and the metrics query accept any
// authenticated token per the spec. An empty return here is deliberate,
// and the scope completeness test in internal/server keeps this list as
// the only sanctioned source of empty scopes for non-meta tools.
func TestRequiredScopesScopelessRoutes(t *testing.T) {
	t.Parallel()

	cases := []string{
		"linode_kernel_get",
		"linode_kernel_list",
		"linode_region_get",
		"linode_region_list",
		"linode_region_availability_get",
		"linode_region_availability_list",
		"linode_type_get",
		"linode_type_list",
		toolDatabaseEngineGet,
		toolDatabaseEngineList,
		toolDatabaseTypeGet,
		toolDatabaseTypeList,
		"linode_lke_type_list",
		"linode_longview_type_list",
		"linode_nodebalancer_type_list",
		"linode_object_storage_type_list",
		"linode_volume_type_list",
		"linode_network_transfer_price_list",
		"linode_beta_get",
		"linode_beta_list",
		"linode_maintenance_policy_list",
		"linode_account_maintenance_list",
		"linode_profile_get",
		"linode_longview_subscription_get",
		"linode_longview_subscription_list",
		"linode_vpc_get",
		"linode_vpc_list",
		"linode_vpc_subnet_get",
		"linode_vpc_subnet_list",
		"linode_account_oauth_client_thumbnail_get",
		"linode_monitor_service_metric_query",
	}

	for _, tool := range cases {
		if got := profiles.RequiredScopes(tool, profiles.CapRead); got != nil {
			t.Errorf("RequiredScopes(%s) = %v, want nil", tool, got)
		}
	}
}

// TestRequiredScopesDocumentedQuirks pins the per-tool overrides where
// the API documents a scope the category matrix cannot derive: the
// firewall-settings update that hops to account, and database
// credential reads that only need read_only despite registering as
// mutators.
func TestRequiredScopesDocumentedQuirks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		toolName   string
		capability profiles.Capability
		want       []profiles.Scope
	}{
		{
			name:       "firewall settings update is account gated",
			toolName:   "linode_firewall_settings_update",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeAccountReadWrite},
		},
		{
			name:       "mysql credentials get only needs databases read",
			toolName:   toolDatabaseMySQLCredentialsGet,
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadOnly},
		},
		{
			name:       "postgresql credentials get only needs databases read",
			toolName:   "linode_database_postgresql_instance_credentials_get",
			capability: profiles.CapWrite,
			want:       []profiles.Scope{profiles.ScopeDatabasesReadOnly},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := profiles.RequiredScopes(tt.toolName, tt.capability)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequiredScopesForTagDelete(t *testing.T) {
	t.Parallel()

	if !reflect.DeepEqual(profiles.RequiredScopes("linode_tag_delete", profiles.CapDestroy), []profiles.Scope{profiles.ScopeAccountReadWrite}) {
		t.Errorf("got %v, want %v", profiles.RequiredScopes("linode_tag_delete", profiles.CapDestroy), []profiles.Scope{profiles.ScopeAccountReadWrite})
	}

	if !slices.Contains(profiles.Categories("linode_tag_delete"), "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}
