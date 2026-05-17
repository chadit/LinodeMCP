"""Phase 2 tests for the built-in profile catalog.

Synthetic ``ToolDescriptor`` lists drive every test so failures point at
the resolver, not at whatever happens to be in the live tool registry on
the day the test runs. The cross-language parity check (Go side) reads
the JSON exported via ``builtin_catalog_json`` against a matching catalog.
"""

from __future__ import annotations

import json

from linodemcp.profiles import (
    Capability,
    Profile,
    ToolDescriptor,
    builtin_catalog_json,
    builtin_profiles,
)


def _synthetic_catalog() -> list[ToolDescriptor]:
    """Return a fixed catalog covering every category and capability.

    Tools are named to match the prefix categorization in
    ``linodemcp.profiles.builtin``. Both read and mutating variants live in
    most categories so the elevation rules can be exercised end-to-end.
    """
    return [
        # Core: always included via Meta/Read.
        ToolDescriptor("hello", Capability.Meta),
        ToolDescriptor("version", Capability.Meta),
        ToolDescriptor("linode_profile", Capability.Read),
        ToolDescriptor("linode_account", Capability.Read),
        # Compute reads + mutations.
        ToolDescriptor("linode_instances_list", Capability.Read),
        ToolDescriptor("linode_instance_get", Capability.Read),
        ToolDescriptor("linode_instance_create", Capability.Write),
        ToolDescriptor("linode_instance_delete", Capability.Destroy),
        ToolDescriptor("linode_regions_list", Capability.Read),
        ToolDescriptor("linode_types_list", Capability.Read),
        ToolDescriptor("linode_images_list", Capability.Read),
        ToolDescriptor("linode_stackscripts_list", Capability.Read),
        # Compute deep (backups, disks, ips).
        ToolDescriptor("linode_instance_backups_list", Capability.Read),
        ToolDescriptor("linode_instance_backup_create", Capability.Write),
        ToolDescriptor("linode_instance_disk_create", Capability.Write),
        ToolDescriptor("linode_instance_ip_allocate", Capability.Write),
        # Block storage.
        ToolDescriptor("linode_volumes_list", Capability.Read),
        ToolDescriptor("linode_volume_create", Capability.Write),
        ToolDescriptor("linode_volume_delete", Capability.Destroy),
        # Object storage.
        ToolDescriptor("linode_object_storage_buckets_list", Capability.Read),
        ToolDescriptor("linode_object_storage_bucket_create", Capability.Write),
        # DNS.
        ToolDescriptor("linode_domains_list", Capability.Read),
        ToolDescriptor("linode_domain_create", Capability.Write),
        ToolDescriptor("linode_domain_record_create", Capability.Write),
        # Networking.
        ToolDescriptor("linode_firewalls_list", Capability.Read),
        ToolDescriptor("linode_firewall_create", Capability.Write),
        ToolDescriptor("linode_nodebalancer_create", Capability.Write),
        ToolDescriptor("linode_vlan_delete", Capability.Destroy),
        ToolDescriptor("linode_ipv6_range_create", Capability.Write),
        # LKE.
        ToolDescriptor("linode_lke_clusters_list", Capability.Read),
        ToolDescriptor("linode_lke_cluster_create", Capability.Write),
        ToolDescriptor("linode_lke_cluster_delete", Capability.Destroy),
        # VPCs.
        ToolDescriptor("linode_vpcs_list", Capability.Read),
        ToolDescriptor("linode_vpc_create", Capability.Write),
        ToolDescriptor("linode_vpc_subnet_create", Capability.Write),
        # Security (SSH keys).
        ToolDescriptor("linode_sshkeys_list", Capability.Read),
        ToolDescriptor("linode_sshkey_create", Capability.Write),
        # Monitor (no built-in elevates it currently except full-access).
        ToolDescriptor("linode_monitor_service_token_create", Capability.Write),
        # Admin tool (synthetic). Never selected by any built-in.
        ToolDescriptor("linode_account_settings_update", Capability.Admin),
        # Unknown tool (synthetic). Never selected; resolver ignores it.
        ToolDescriptor("linode_undiscovered_thing", Capability.Unknown),
    ]


_EXPECTED_NAMES = (
    "default",
    "readonly-full",
    "compute-admin",
    "network-admin",
    "kubernetes-admin",
    "storage-admin",
    "full-access",
    "emergency",
)


def test_builtin_profiles_are_non_empty() -> None:
    """Every built-in resolves at least one tool against a realistic catalog."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    assert set(profiles.keys()) == set(_EXPECTED_NAMES)
    for name in _EXPECTED_NAMES:
        profile = profiles[name]
        assert isinstance(profile, Profile)
        assert profile.allowed_tools, (
            f"profile {name!r} resolved to zero tools; "
            "the catalog or category rules are wrong"
        )


def test_default_profile_contains_only_read_and_meta() -> None:
    """The default profile must not pick up any Write/Destroy/Admin tool."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)
    default = profiles["default"]

    readable_meta = {
        tool.name
        for tool in catalog
        if tool.capability in (Capability.Read, Capability.Meta)
    }
    assert set(default.allowed_tools) == readable_meta


def test_readonly_full_matches_default_tool_set() -> None:
    """``readonly-full`` and ``default`` resolve to the same tool list."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    assert profiles["readonly-full"].allowed_tools == profiles["default"].allowed_tools


def test_emergency_allows_yolo_and_default_does_not() -> None:
    """``emergency`` is the only built-in that opts into yolo by default."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    assert profiles["emergency"].allow_yolo is True
    assert profiles["default"].allow_yolo is False
    other_names = [n for n in _EXPECTED_NAMES if n != "emergency"]
    for name in other_names:
        assert profiles[name].allow_yolo is False, name


def test_full_access_and_emergency_disabled() -> None:
    """Power-user profiles ship disabled; users opt in via config."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    assert profiles["full-access"].disabled is True
    assert profiles["emergency"].disabled is True
    enabled_names = [
        n for n in _EXPECTED_NAMES if n not in ("full-access", "emergency")
    ]
    for name in enabled_names:
        assert profiles[name].disabled is False, name


def test_compute_admin_includes_instance_writes() -> None:
    """Compute mutators land in ``compute-admin`` and not in narrower roles."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    compute_admin_tools = set(profiles["compute-admin"].allowed_tools)
    assert "linode_instance_create" in compute_admin_tools
    assert "linode_instance_delete" in compute_admin_tools
    # Block storage and SSH keys are in compute-admin's elevated categories
    # per spec.
    assert "linode_volume_create" in compute_admin_tools
    assert "linode_sshkey_create" in compute_admin_tools


def test_network_admin_excludes_compute_writes() -> None:
    """Compute mutators do NOT leak into the network admin profile."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    network_admin_tools = set(profiles["network-admin"].allowed_tools)
    assert "linode_instance_create" not in network_admin_tools
    assert "linode_volume_create" not in network_admin_tools
    # But network mutators are present.
    assert "linode_firewall_create" in network_admin_tools
    assert "linode_domain_create" in network_admin_tools
    assert "linode_vpc_create" in network_admin_tools


def test_kubernetes_admin_includes_lke_and_compute() -> None:
    """K8s admin needs compute (for node mgmt) plus LKE-specific writes."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    k8s_tools = set(profiles["kubernetes-admin"].allowed_tools)
    assert "linode_lke_cluster_create" in k8s_tools
    assert "linode_instance_create" in k8s_tools
    assert "linode_vpc_create" in k8s_tools
    # Outside its categories.
    assert "linode_domain_create" not in k8s_tools
    assert "linode_firewall_create" not in k8s_tools


def test_storage_admin_includes_backups_but_not_other_compute() -> None:
    """Storage admin elevates compute_deep (backups) without all of compute."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    storage_tools = set(profiles["storage-admin"].allowed_tools)
    assert "linode_instance_backup_create" in storage_tools
    assert "linode_volume_create" in storage_tools
    assert "linode_object_storage_bucket_create" in storage_tools
    # No general compute write access.
    assert "linode_instance_create" not in storage_tools


def test_full_access_includes_every_mutator_except_admin() -> None:
    """Full access spans every category but skips ``Capability.Admin``."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    full_tools = set(profiles["full-access"].allowed_tools)
    mutators = {
        tool.name
        for tool in catalog
        if tool.capability in (Capability.Write, Capability.Destroy)
    }
    assert mutators.issubset(full_tools)
    assert "linode_account_settings_update" not in full_tools
    assert "linode_undiscovered_thing" not in full_tools


def test_allowed_tools_are_sorted_for_determinism() -> None:
    """Sorted output keeps the cross-language JSON parity check stable."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    for name, profile in profiles.items():
        tools = list(profile.allowed_tools)
        assert tools == sorted(tools), f"{name} allowed_tools not sorted"


def test_json_roundtrip() -> None:
    """``builtin_catalog_json`` round-trips through ``json.loads`` cleanly."""
    catalog = _synthetic_catalog()
    raw = builtin_catalog_json(catalog)

    parsed = json.loads(raw)
    assert set(parsed.keys()) == set(_EXPECTED_NAMES)

    profiles = builtin_profiles(catalog)
    for name, profile in profiles.items():
        entry = parsed[name]
        assert entry["name"] == profile.name
        assert entry["description"] == profile.description
        assert entry["allowed_tools"] == list(profile.allowed_tools)
        assert entry["allowed_environments"] == list(profile.allowed_environments)
        assert entry["required_token_scopes"] == list(profile.required_token_scopes)
        assert entry["allow_yolo"] == profile.allow_yolo
        assert entry["disabled"] == profile.disabled
