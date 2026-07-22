"""Unit tests for the Phase 6 scope catalog and prefix mapping.

Mirrors ``go/internal/profiles/scope_test.go``. Both sides MUST agree on
the resolved scope list for the same (tool_name, capability) input; a
parity test in a later phase will lock in that contract end-to-end. For
now these tests pin the Python side's behavior so the Go and Python
files cannot drift silently.
"""

from __future__ import annotations

import pytest

from linodemcp.profiles import Capability, Scope, required_scopes


def test_meta_returns_empty() -> None:
    """Meta tools (hello, version) need no Linode scope.

    Phase 6.4 short-circuits the scope check when capability is Meta;
    the empty list here is the signal it relies on.
    """
    assert required_scopes("hello", Capability.Meta) == []
    assert required_scopes("version", Capability.Meta) == []


@pytest.mark.parametrize(
    ("tool_name", "capability", "expected"),
    [
        ("linode_instance_list", Capability.Read, [Scope.LinodesReadOnly]),
        ("linode_instance_delete", Capability.Destroy, [Scope.LinodesReadWrite]),
        (
            "linode_instance_firewall_update",
            Capability.Write,
            [Scope.LinodesReadWrite],
        ),
        ("linode_volume_clone", Capability.Write, [Scope.VolumesReadWrite]),
        ("linode_volume_create", Capability.Write, [Scope.VolumesReadWrite]),
        ("linode_volume_list", Capability.Read, [Scope.VolumesReadOnly]),
        ("linode_volume_type_list", Capability.Read, []),
        # Database engines and types are public catalog routes: the
        # OpenAPI spec declares no security requirement for them, so no
        # scope is required.
        ("linode_database_engine_list", Capability.Read, []),
        ("linode_database_type_list", Capability.Read, []),
        ("linode_domain_delete", Capability.Destroy, [Scope.DomainsReadWrite]),
        ("linode_domain_import", Capability.Write, [Scope.DomainsReadWrite]),
        ("linode_lke_cluster_regenerate", Capability.Write, [Scope.LKEReadWrite]),
        (
            "linode_object_storage_bucket_create",
            Capability.Write,
            [Scope.ObjectStorageReadWrite],
        ),
        (
            "linode_stackscript_create",
            Capability.Write,
            [Scope.StackScriptsReadWrite],
        ),
        ("linode_vpc_list", Capability.Read, []),
        ("linode_instance_config_get", Capability.Read, [Scope.LinodesReadOnly]),
        (
            "linode_instance_config_interface_get",
            Capability.Read,
            [Scope.LinodesReadOnly],
        ),
        (
            "linode_instance_config_interface_update",
            Capability.Write,
            [Scope.LinodesReadWrite],
        ),
        ("linode_instance_config_delete", Capability.Destroy, [Scope.LinodesReadWrite]),
        (
            "linode_instance_config_interface_list",
            Capability.Read,
            [Scope.LinodesReadOnly],
        ),
        (
            "linode_instance_interface_history_list",
            Capability.Read,
            [Scope.LinodesReadOnly],
        ),
        (
            "linode_nodebalancer_vpc_config_list",
            Capability.Read,
            [Scope.NodeBalancersReadOnly],
        ),
        (
            "linode_nodebalancer_update",
            Capability.Write,
            [Scope.NodeBalancersReadWrite],
        ),
        (
            "linode_nodebalancer_firewall_update",
            Capability.Write,
            [Scope.NodeBalancersReadWrite],
        ),
        (
            "linode_nodebalancer_config_node_update",
            Capability.Write,
            [Scope.NodeBalancersReadWrite],
        ),
        (
            "linode_nodebalancer_config_delete",
            Capability.Destroy,
            [Scope.NodeBalancersReadWrite],
        ),
        ("linode_firewall_create", Capability.Write, [Scope.FirewallReadWrite]),
        ("linode_instance_firewall_apply", Capability.Write, [Scope.LinodesReadWrite]),
        # The API splits this pair: GET stays firewall:read_only while
        # PUT hops to account:read_write.
        ("linode_firewall_settings_get", Capability.Read, [Scope.FirewallReadOnly]),
        ("linode_firewall_settings_update", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_account_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_invoice_item_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        ("linode_account_beta_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_account_child_account_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_account_oauth_client_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_oauth_client_thumbnail_get",
            Capability.Read,
            [],
        ),
        ("linode_account_invoice_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_account_payment_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_agreement_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_account_availability_get",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_account_availability_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_account_agreement_acknowledge",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_beta_enroll",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_oauth_client_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_user_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_oauth_client_delete",
            Capability.Destroy,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_payment_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_payment_method_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        # POST /account/promo-codes is documented with only
        # account:read_only; _scope_overrides mirrors the spec.
        (
            "linode_account_promo_credit_add",
            Capability.Write,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_account_service_transfer_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        ("linode_account_cancel", Capability.Destroy, [Scope.AccountReadWrite]),
        ("linode_account_user_update", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_managed_contact_update", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_managed_service_update", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_managed_service_enable", Capability.Write, [Scope.AccountReadWrite]),
        (
            "linode_managed_credential_update",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_managed_credential_username_password_update",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        # GET /profile is documented with an empty scope list: any
        # authenticated token may read its own profile.
        ("linode_profile_get", Capability.Read, []),
        ("linode_database_engine_get", Capability.Read, []),
        ("linode_database_type_get", Capability.Read, []),
        (
            "linode_database_mysql_instance_create",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_postgresql_instance_create",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_mysql_instance_resume",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_mysql_instance_suspend",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_postgresql_instance_resume",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_mysql_instance_update",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_mysql_instance_list",
            Capability.Read,
            [Scope.DatabasesReadOnly],
        ),
        (
            "linode_database_postgresql_instance_delete",
            Capability.Destroy,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_postgresql_instance_patch",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_postgresql_instance_update",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        (
            "linode_database_postgresql_instance_suspend",
            Capability.Write,
            [Scope.DatabasesReadWrite],
        ),
        # Credential reads are documented as databases:read_only even
        # though the tools register as mutators; the override in
        # _scope_overrides mirrors the spec.
        (
            "linode_database_postgresql_instance_credentials_get",
            Capability.Write,
            [Scope.DatabasesReadOnly],
        ),
        (
            "linode_database_mysql_instance_credentials_get",
            Capability.Write,
            [Scope.DatabasesReadOnly],
        ),
        (
            "linode_database_postgresql_instance_list",
            Capability.Read,
            [Scope.DatabasesReadOnly],
        ),
    ],
)
def test_read_vs_write_per_category(
    tool_name: str, capability: Capability, expected: list[Scope]
) -> None:
    """Read capability maps to :read_only, mutators map to :read_write.

    The Linode API doesn't distinguish Destroy from Write at the scope
    level, so both resolve to :read_write. This parametrized table
    locks in the mapping per category so a future refactor that splits
    a category would have to update the test deliberately.
    """
    assert required_scopes(tool_name, capability) == expected


def test_image_upload_needs_images_write() -> None:
    """Creating an image upload needs image write scope."""
    assert required_scopes("linode_image_upload", Capability.Write) == [
        Scope.ImagesReadWrite
    ]


def test_image_sharegroup_token_create_needs_images_write() -> None:
    """Creating an image share group token needs image write scope."""
    assert required_scopes(
        "linode_image_sharegroup_token_create", Capability.Write
    ) == [Scope.ImagesReadWrite]


def test_image_sharegroup_token_delete_needs_images_write() -> None:
    """Deleting an image share group token needs image write scope."""
    assert required_scopes(
        "linode_image_sharegroup_token_delete", Capability.Destroy
    ) == [Scope.ImagesReadWrite]


def test_image_sharegroup_token_update_needs_images_write() -> None:
    """Updating an image share group token needs image write scope."""
    assert required_scopes(
        "linode_image_sharegroup_token_update", Capability.Write
    ) == [Scope.ImagesReadWrite]


def test_image_delete_needs_images_write() -> None:
    """Deleting a private image needs image write scope."""
    assert required_scopes("linode_image_delete", Capability.Destroy) == [
        Scope.ImagesReadWrite
    ]


def test_image_sharegroup_delete_needs_images_write() -> None:
    """Deleting an image share group needs image write scope."""
    assert required_scopes("linode_image_sharegroup_delete", Capability.Destroy) == [
        Scope.ImagesReadWrite
    ]


def test_image_sharegroup_update_needs_images_write() -> None:
    """Updating an image share group needs image write scope."""
    assert required_scopes("linode_image_sharegroup_update", Capability.Write) == [
        Scope.ImagesReadWrite
    ]


def test_image_sharegroup_image_delete_needs_images_write() -> None:
    """Revoking shared image access needs image write scope."""
    assert required_scopes(
        "linode_image_sharegroup_image_delete", Capability.Destroy
    ) == [Scope.ImagesReadWrite]


def test_image_sharegroup_images_add_needs_images_write() -> None:
    """Adding images to a share group needs image write scope."""
    assert required_scopes("linode_image_sharegroup_image_add", Capability.Write) == [
        Scope.ImagesReadWrite
    ]


def test_image_sharegroup_members_add_needs_images_write() -> None:
    """Adding members to a share group needs image write scope."""
    assert required_scopes("linode_image_sharegroup_member_add", Capability.Write) == [
        Scope.ImagesReadWrite
    ]


def test_instance_create_needs_only_linodes_write() -> None:
    """Provisioning documents linodes:read_write alone.

    The API grants image access at request time; requiring
    images:read_only here would deny tokens the API itself accepts.
    Clone and rebuild carry the same single-scope contract.
    """
    for tool in (
        "linode_instance_create",
        "linode_instance_clone",
        "linode_instance_rebuild",
    ):
        assert required_scopes(tool, Capability.Write) == [Scope.LinodesReadWrite]


def test_lke_cluster_create_needs_only_lke_write() -> None:
    """LKE cluster creation documents lke:read_write alone."""
    got = required_scopes("linode_lke_cluster_create", Capability.Write)
    assert got == [Scope.LKEReadWrite]


def test_image_create_needs_images_write_and_linodes_read() -> None:
    """Capturing an image reads the source disk, per the documented pair."""
    got = required_scopes("linode_image_create", Capability.Write)
    assert set(got) == {Scope.ImagesReadWrite, Scope.LinodesReadOnly}


def test_volume_attach_detach_need_linodes_write() -> None:
    """Attach and detach touch the target Linode, per the documented pair."""
    for tool in ("linode_volume_attach", "linode_volume_detach"):
        got = required_scopes(tool, Capability.Write)
        assert set(got) == {Scope.VolumesReadWrite, Scope.LinodesReadWrite}


@pytest.mark.parametrize(
    "tool_name",
    [
        "linode_networking_ip_assign",
        "linode_networking_ip_share",
        "linode_networking_ipv4_assign",
        "linode_networking_ipv4_share",
        "linode_ipv6_range_create",
    ],
)
def test_ip_assignment_needs_linodes_write(tool_name: str) -> None:
    """Address assignment and sharing carry a dual scope.

    Those operations target Linodes, so the API documents
    linodes:read_write alongside ips:read_write.
    """
    got = required_scopes(tool_name, Capability.Write)
    assert set(got) == {Scope.IPsReadWrite, Scope.LinodesReadWrite}


def test_unknown_tool_returns_empty() -> None:
    """Unknown tool names degrade into the warn path, not a hard fail.

    Phase 6.4 logs a warning when a tool's scope can't be determined
    rather than refusing to start the server. The empty return here is
    the signal that triggers that warning.
    """
    assert required_scopes("not_a_real_tool", Capability.Write) == []


@pytest.mark.parametrize(
    "tool_name",
    [
        "linode_instance_backup_list",
        "linode_instance_config_create",
        "linode_instance_disk_create",
        "linode_instance_ip_allocate",
    ],
)
def test_instance_subtools_route_to_linodes(tool_name: str) -> None:
    """Backups, disks, and IPs under linode_instance_* stay in the linodes scope.

    Confirms the prefix dispatcher doesn't get fooled by the shared
    ``linode_instance_`` root into routing somewhere else.
    """
    got = required_scopes(tool_name, Capability.Write)
    assert Scope.LinodesReadWrite in got


@pytest.mark.parametrize(
    ("tool_name", "capability", "expected"),
    [
        ("linode_sshkey_list", Capability.Read, Scope.AccountReadOnly),
        ("linode_sshkey_get", Capability.Read, Scope.AccountReadOnly),
        ("linode_sshkey_create", Capability.Write, Scope.AccountReadWrite),
        (
            "linode_monitor_dashboard_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_dashboard_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_get",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_alert_channel_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_alert_definition_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_alert_definition_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_metric_definition_list",
            Capability.Read,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_alert_definition_update",
            Capability.Write,
            Scope.MonitorReadWrite,
        ),
        # POST /monitor/services/{type}/token is documented with only
        # monitor:read_only; _scope_overrides mirrors the spec.
        (
            "linode_monitor_service_token_create",
            Capability.Write,
            Scope.MonitorReadOnly,
        ),
        (
            "linode_monitor_service_alert_definition_delete",
            Capability.Destroy,
            Scope.MonitorReadWrite,
        ),
    ],
)
def test_ssh_and_monitor_scopes(
    tool_name: str, capability: Capability, expected: Scope
) -> None:
    """SSH keys are account-gated; /monitor carries its own monitor:* scopes.

    The metric-query tool is absent here on purpose: its route is
    documented scopeless and lives in the scopeless-route test instead.
    """
    assert required_scopes(tool_name, capability) == [expected]


@pytest.mark.parametrize(
    "tool_name",
    [
        "linode_kernel_get",
        "linode_kernel_list",
        "linode_database_engine_get",
        "linode_database_engine_list",
        "linode_database_type_get",
        "linode_database_type_list",
        "linode_network_transfer_price_list",
        "linode_beta_get",
        "linode_beta_list",
        "linode_maintenance_policy_list",
        "linode_region_get",
        "linode_region_list",
        "linode_region_availability_get",
        "linode_region_availability_list",
        "linode_type_get",
        "linode_type_list",
        "linode_lke_type_list",
        "linode_longview_type_list",
        "linode_nodebalancer_type_list",
        "linode_object_storage_type_list",
        "linode_volume_type_list",
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
    ],
)
def test_scopeless_routes_return_empty(tool_name: str) -> None:
    """Documented scopeless routes require no token scope.

    Catalog, pricing, and region routes are public (no authentication at
    all); betas, maintenance, the caller's own profile, Longview
    subscription plans, VPC reads, the OAuth-client thumbnail, and the
    metrics query accept any authenticated token per the spec. The empty
    return is deliberate, and the scope completeness test keeps this
    list as the only sanctioned source of empty scopes for non-meta
    tools.
    """
    assert required_scopes(tool_name, Capability.Read) == []


@pytest.mark.parametrize(
    ("tool_name", "capability", "expected"),
    [
        # The API gates /profile/tokens with account scopes, not a
        # dedicated tokens scope.
        ("linode_profile_token_list", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_profile_token_update",
            Capability.Admin,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_profile_token_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        ("linode_profile_app_list", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_profile_login_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_profile_preferences_get",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_profile_preferences_update",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_profile_tfa_enable",
            Capability.Admin,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_profile_phone_number_delete",
            Capability.Destroy,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_profile_security_question_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        ("linode_profile_device_list", Capability.Read, [Scope.AccountReadOnly]),
        # Event routes live under /account but the API gates them with
        # events:* scopes; the seen-marker POST is documented with only
        # events:read_only and _scope_overrides mirrors that.
        ("linode_account_event_list", Capability.Read, [Scope.EventsReadOnly]),
        ("linode_account_event_get", Capability.Read, [Scope.EventsReadOnly]),
        ("linode_account_event_seen", Capability.Write, [Scope.EventsReadOnly]),
        ("linode_support_ticket_list", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_support_ticket_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_support_ticket_reply_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        # POST /tags documents account:read_write alone; entity access
        # is enforced by the API through grants at request time, not
        # extra token scopes.
        ("linode_tag_create", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_tag_object_list", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_managed_service_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        (
            "linode_managed_credential_get",
            Capability.Admin,
            [Scope.AccountReadWrite],
        ),
        # GET /managed/contacts/{id} is documented account:read_write
        # despite being a read (contacts hold PII); _scope_overrides
        # mirrors the spec, and the elevation policy derives from
        # capabilities so this write scope does not flip it.
        (
            "linode_managed_contact_get",
            Capability.Read,
            [Scope.AccountReadWrite],
        ),
        ("linode_placement_group_get", Capability.Read, [Scope.LinodesReadOnly]),
        (
            "linode_placement_group_create",
            Capability.Write,
            [Scope.LinodesReadWrite],
        ),
        # The API documents placement:read_only for this one route, but
        # its own OAuth catalog never defines that scope, so the tool
        # stays on the family's linodes derivation. See _scope_overrides.
        (
            "linode_placement_group_list",
            Capability.Read,
            [Scope.LinodesReadOnly],
        ),
        ("linode_networking_ip_list", Capability.Read, [Scope.IPsReadOnly]),
        ("linode_networking_ip_update", Capability.Write, [Scope.IPsReadWrite]),
        # The docs list this route as "ips:read", an upstream typo: the
        # scope catalog only defines read_only and read_write, so the
        # family's read_only applies.
        ("linode_ipv6_range_get", Capability.Read, [Scope.IPsReadOnly]),
        ("linode_ipv6_pool_list", Capability.Read, [Scope.IPsReadOnly]),
        ("linode_vlan_list", Capability.Read, [Scope.LinodesReadOnly]),
        ("linode_vlan_delete", Capability.Destroy, [Scope.LinodesReadWrite]),
    ],
)
def test_reconciled_families(
    tool_name: str, capability: Capability, expected: list[Scope]
) -> None:
    """Families the scope parity gate found diverging between Go and Python.

    Databases, managed, support tickets, placement groups, tags, the
    profile subtree, and the firewall-settings split. Each expectation
    comes from the security block of the underlying operation in the
    Linode OpenAPI spec.
    """
    assert required_scopes(tool_name, capability) == expected


def test_scope_string_values_match_linode_api() -> None:
    """Scope enum string values must match the Linode API verbatim.

    Locks in the values so a refactor that renames or restructures
    can't silently change the wire format. Cross-language parity with
    Go's scope catalog depends on these exact strings.
    """
    assert Scope.LinodesReadOnly.value == "linodes:read_only"
    assert Scope.LinodesReadWrite.value == "linodes:read_write"
    assert Scope.ObjectStorageReadWrite.value == "object_storage:read_write"
    assert Scope.Wildcard.value == "*"
