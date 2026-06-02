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
        ("linode_instances_list", Capability.Read, [Scope.LinodesReadOnly]),
        ("linode_instance_delete", Capability.Destroy, [Scope.LinodesReadWrite]),
        ("linode_volume_clone", Capability.Write, [Scope.VolumesReadWrite]),
        ("linode_volume_create", Capability.Write, [Scope.VolumesReadWrite]),
        ("linode_volumes_list", Capability.Read, [Scope.VolumesReadOnly]),
        ("linode_volume_types_list", Capability.Read, [Scope.VolumesReadOnly]),
        ("linode_domain_delete", Capability.Destroy, [Scope.DomainsReadWrite]),
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
        ("linode_vpcs_list", Capability.Read, [Scope.VPCReadOnly]),
        (
            "linode_nodebalancer_vpc_configs_list",
            Capability.Read,
            [Scope.NodeBalancersReadOnly],
        ),
        (
            "linode_nodebalancer_update",
            Capability.Write,
            [Scope.NodeBalancersReadWrite],
        ),
        (
            "linode_nodebalancer_firewalls_update",
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
        ("linode_firewall_settings_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_firewall_settings_update", Capability.Write, [Scope.AccountReadWrite]),
        ("linode_account", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_invoice_items_list",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        ("linode_account_beta_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_account_child_account_get", Capability.Read, [Scope.AccountReadOnly]),
        ("linode_account_oauth_client_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_oauth_client_thumbnail_get",
            Capability.Read,
            [Scope.AccountReadOnly],
        ),
        ("linode_account_invoice_get", Capability.Read, [Scope.AccountReadOnly]),
        (
            "linode_account_agreements_list",
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
            "linode_account_agreements_acknowledge",
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
            "linode_account_oauth_client_delete",
            Capability.Destroy,
            [Scope.AccountReadWrite],
        ),
        (
            "linode_account_payment_method_create",
            Capability.Write,
            [Scope.AccountReadWrite],
        ),
        ("linode_account_cancel", Capability.Destroy, [Scope.AccountReadWrite]),
        ("linode_profile", Capability.Read, [Scope.AccountReadOnly]),
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


def test_instance_create_needs_linodes_write_and_images_read() -> None:
    """Provisioning a Linode from an image requires images:read_only too.

    Locks in the cross-category mapping. If a refactor drops the extras
    table, this test catches it before token validation silently lets
    under-scoped tokens through.
    """
    got = required_scopes("linode_instance_create", Capability.Write)
    assert set(got) == {Scope.LinodesReadWrite, Scope.ImagesReadOnly}


def test_instance_clone_needs_linodes_write_and_images_read() -> None:
    """Cloning carries the same image dependency as creation."""
    got = required_scopes("linode_instance_clone", Capability.Write)
    assert set(got) == {Scope.LinodesReadWrite, Scope.ImagesReadOnly}


def test_lke_cluster_create_needs_lke_write_and_linodes_write() -> None:
    """LKE clusters provision Linodes under the hood."""
    got = required_scopes("linode_lke_cluster_create", Capability.Write)
    assert set(got) == {Scope.LKEReadWrite, Scope.LinodesReadWrite}


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
        "linode_instance_backups_list",
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
        ("linode_sshkeys_list", Capability.Read, Scope.AccountReadOnly),
        ("linode_sshkey_get", Capability.Read, Scope.AccountReadOnly),
        ("linode_sshkey_create", Capability.Write, Scope.AccountReadWrite),
        (
            "linode_monitor_dashboards_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_service_dashboards_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_service_get",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_services_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_alert_channels_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_alert_definitions_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_service_alert_definitions_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_service_metric_definitions_list",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_service_metrics_read",
            Capability.Read,
            Scope.AccountReadOnly,
        ),
        (
            "linode_monitor_alert_definition_update",
            Capability.Write,
            Scope.AccountReadWrite,
        ),
        (
            "linode_monitor_service_token_create",
            Capability.Write,
            Scope.AccountReadWrite,
        ),
        (
            "linode_monitor_service_alert_definition_delete",
            Capability.Destroy,
            Scope.AccountReadWrite,
        ),
    ],
)
def test_ssh_and_monitor_are_account_scoped(
    tool_name: str, capability: Capability, expected: Scope
) -> None:
    """SSH-key and monitor tools live under /profile and /monitor, both
    of which are gated by account-level access in the Linode API.
    """
    assert required_scopes(tool_name, capability) == [expected]


def test_scope_string_values_match_linode_api() -> None:
    """Scope enum string values must match the Linode API verbatim.

    Locks in the values so a refactor that renames or restructures
    can't silently change the wire format. Cross-language parity with
    Go's scope catalog depends on these exact strings.
    """
    assert Scope.LinodesReadOnly.value == "linodes:read_only"
    assert Scope.LinodesReadWrite.value == "linodes:read_write"
    assert Scope.TokensReadOnly.value == "tokens:read_only"
    assert Scope.TokensReadWrite.value == "tokens:read_write"
    assert Scope.ObjectStorageReadWrite.value == "object_storage:read_write"
    assert Scope.Wildcard.value == "*"
