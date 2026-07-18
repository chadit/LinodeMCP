"""Tests for the write-proto success-path classifier.

The classifier (``linodemcp.tools._write_proto_classifier``) statically decides,
per mutating tool, whether its Python handler routes success output through the
proto path (``serialize_api_response`` / ``serialize_list_response``) or builds a
plain dict. The write-proto gate diffs this against the Go classifier, so these
tests pin the classifier's own behavior: every mutating tool resolves to a
handler, and a spread of known proto and known legacy handlers land correctly.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from linodemcp.tools._write_proto_classifier import classify

_CAPABILITIES_PATH = (
    Path(__file__).resolve().parents[3]
    / "docs"
    / "contracts"
    / "tools-capabilities.txt"
)

# Handlers known to serialize their success output through a proto message.
_KNOWN_PROTO = (
    "linode_nodebalancer_config_create",
    "linode_lke_pool_create",
    "linode_firewall_create",
    "linode_database_mysql_instance_create",
    "linode_instance_disk_create",
    "linode_account_user_create",
    "linode_support_ticket_create",
    "linode_sshkey_create",
    "linode_instance_create",
    "linode_instance_clone",
    "linode_image_create",
    "linode_lke_cluster_create",
    "linode_nodebalancer_create",
    "linode_object_storage_bucket_create",
    "linode_object_storage_key_create",
    "linode_account_settings_update",
    "linode_account_cancel",
    "linode_instance_interface_add",
    "linode_instance_interface_update",
    "linode_instance_interface_settings_update",
    "linode_instance_interface_upgrade",
    "linode_stackscript_create",
    "linode_stackscript_update",
    "linode_ipv6_range_create",
    "linode_firewall_device_create",
    "linode_firewall_settings_update",
    "linode_nodebalancer_firewall_update",
    "linode_lke_acl_update",
    "linode_instance_boot",
    "linode_instance_reboot",
    "linode_instance_shutdown",
    "linode_instance_migrate",
    "linode_instance_rescue",
    "linode_instance_resize",
    "linode_instance_backups_enable",
    "linode_instance_backups_cancel",
    "linode_instance_password_reset",
    "linode_instance_disk_password_reset",
    "linode_volume_delete",
    "linode_domain_delete",
    "linode_firewall_delete",
    "linode_nodebalancer_delete",
    "linode_vpc_delete",
    "linode_lke_cluster_delete",
    "linode_stackscript_delete",
    "linode_object_storage_key_delete",
    "linode_sshkey_delete",
    "linode_instance_delete",
    "linode_instance_disk_delete",
    "linode_firewall_device_delete",
    "linode_domain_record_delete",
    "linode_vpc_subnet_delete",
    "linode_lke_pool_delete",
    "linode_lke_kubeconfig_delete",
    "linode_lke_service_token_delete",
    "linode_instance_config_delete",
    "linode_instance_config_interface_delete",
    "linode_instance_interface_delete",
    "linode_nodebalancer_config_delete",
    "linode_nodebalancer_config_node_delete",
    "linode_image_delete",
    "linode_image_sharegroup_delete",
    "linode_image_sharegroup_image_delete",
    "linode_image_sharegroup_token_delete",
    "linode_image_sharegroup_member_token_delete",
    "linode_instance_ip_delete",
    "linode_ipv6_range_delete",
    "linode_lke_cluster_recycle",
    "linode_lke_cluster_regenerate",
    "linode_lke_pool_recycle",
    "linode_lke_node_recycle",
    "linode_lke_node_delete",
    "linode_placement_group_delete",
    "linode_vlan_delete",
    "linode_volume_detach",
    "linode_object_storage_bucket_delete",
    "linode_database_mysql_instance_delete",
    "linode_database_postgresql_instance_delete",
    "linode_lke_acl_delete",
    "linode_object_storage_ssl_delete",
    "linode_object_storage_cancel",
    "linode_instance_config_interface_reorder",
    "linode_instance_firewall_update",
    "linode_database_mysql_instance_credentials_get",
    "linode_database_postgresql_instance_credentials_get",
    "linode_monitor_service_token_create",
)

# Every mutating handler now routes success output through a proto message, so
# no known-legacy handlers remain. The tuple stays for the ratchet: a handler
# that regresses to a hand-built dict gets pinned here.
_KNOWN_LEGACY: tuple[str, ...] = ()


def _mutating_tool_count() -> int:
    """Count Write/Destroy/Admin tools in the canonical capabilities file."""
    mutating = {"Write", "Destroy", "Admin"}
    count = 0

    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        parts = stripped.split("\t")
        expected_fields = 2
        if len(parts) == expected_fields and parts[1].strip() in mutating:
            count += 1

    return count


def test_classify_covers_every_mutating_tool() -> None:
    """Every mutating tool resolves to a handler; none fall through to review."""
    result = classify()

    assert len(result) == _mutating_tool_count()
    review = sorted(name for name, status in result.items() if status == "review")
    assert review == [], f"handlers not found for: {review}"
    assert set(result.values()) <= {"proto", "legacy"}


def test_known_proto_handlers_classify_proto() -> None:
    """Handlers that serialize a proto message classify as proto."""
    result = classify()

    for tool in _KNOWN_PROTO:
        assert result.get(tool) == "proto", (
            f"{tool} expected proto, got {result.get(tool)}"
        )


def test_known_legacy_handlers_classify_legacy() -> None:
    """Handlers that build a curated dict classify as legacy."""
    result = classify()

    for tool in _KNOWN_LEGACY:
        assert result.get(tool) == "legacy", (
            f"{tool} expected legacy, got {result.get(tool)}"
        )


def _read_tool_count() -> int:
    """Count Read tools in the canonical capabilities file."""
    count = 0

    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split("\t")
        if len(parts) == 2 and parts[1].strip() == "Read":
            count += 1

    return count


def test_classify_read_surface_covers_every_read_tool() -> None:
    """Read mode selects exactly the Read-capability tools, none unresolved."""
    result = classify("read")

    assert len(result) == _read_tool_count()
    review = sorted(name for name, status in result.items() if status == "review")
    assert review == [], f"handlers not found for: {review}"
    assert set(result.values()) <= {"proto", "legacy"}


def test_classify_read_surface_known_proto() -> None:
    """Long-converted read handlers classify as proto in read mode."""
    result = classify("read")

    for tool in ("linode_instance_get", "linode_volume_list", "linode_region_list"):
        assert result.get(tool) == "proto", (
            f"{tool} expected proto, got {result.get(tool)}"
        )


def _meta_tool_count() -> int:
    """Count Meta tools in the canonical capabilities file."""
    count = 0

    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split("\t")
        if len(parts) == 2 and parts[1].strip() == "Meta":
            count += 1

    return count


def test_classify_meta_surface_covers_every_meta_tool() -> None:
    """Meta mode selects exactly the Meta-capability tools, none unresolved."""
    result = classify("meta")

    assert len(result) == _meta_tool_count()
    review = sorted(name for name, status in result.items() if status == "review")
    assert review == [], f"handlers not found for: {review}"
    assert set(result.values()) <= {"proto", "legacy"}


def test_classify_meta_surface_known_proto() -> None:
    """Converted meta handlers classify as proto in meta mode."""
    result = classify("meta")

    for tool in ("hello", "version", "linode_audit_health", "linode_profile_draft_new"):
        assert result.get(tool) == "proto", (
            f"{tool} expected proto, got {result.get(tool)}"
        )


# Factories known to build their MCP input schema from the proto contract
# (inputSchema=schema(...)). These are long-converted read tools, stable across
# the input-surface conversion waves.
_KNOWN_INPUT_GENERATED = (
    "linode_volume_get",
    "linode_region_get",
    "linode_instance_get",
    "linode_instance_list",
)


def _all_tool_count() -> int:
    """Count every tool in the canonical capabilities file, all capabilities."""
    count = 0

    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split("\t")
        if len(parts) == 2:
            count += 1

    return count


def test_classify_input_surface_covers_every_tool() -> None:
    """Input mode selects every tool, all resolving to a factory."""
    result = classify("input")

    assert len(result) == _all_tool_count()
    review = sorted(name for name, status in result.items() if status == "review")
    assert review == [], f"factories not found for: {review}"
    assert set(result.values()) <= {"generated", "hand"}


def test_classify_input_surface_known_generated() -> None:
    """Factories loading a proto schema classify as generated in input mode."""
    result = classify("input")

    for tool in _KNOWN_INPUT_GENERATED:
        assert result.get(tool) == "generated", (
            f"{tool} expected generated, got {result.get(tool)}"
        )


def test_classify_rejects_unknown_surface() -> None:
    """An unknown surface raises instead of silently classifying nothing."""
    with pytest.raises(ValueError, match="bogus"):
        classify("bogus")
