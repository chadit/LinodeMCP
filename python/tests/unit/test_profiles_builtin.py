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
    Scope,
    ToolDescriptor,
    builtin_catalog_json,
    builtin_profiles,
    required_scopes,
)
from linodemcp.profiles.builtin import categories


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
        ToolDescriptor("linode_profile_security_questions_list", Capability.Read),
        ToolDescriptor("linode_account", Capability.Read),
        ToolDescriptor("linode_account_betas_list", Capability.Read),
        ToolDescriptor("linode_betas_list", Capability.Read),
        ToolDescriptor("linode_database_instances_list", Capability.Read),
        ToolDescriptor("linode_account_child_accounts_list", Capability.Read),
        ToolDescriptor("linode_account_user_create", Capability.Write),
        ToolDescriptor("linode_account_service_transfers_list", Capability.Read),
        ToolDescriptor("linode_account_service_transfer_accept", Capability.Write),
        ToolDescriptor("linode_account_events_list", Capability.Read),
        ToolDescriptor("linode_account_event_seen", Capability.Write),
        ToolDescriptor("linode_account_invoice_get", Capability.Read),
        ToolDescriptor("linode_account_invoice_items_list", Capability.Read),
        ToolDescriptor("linode_account_invoices_list", Capability.Read),
        ToolDescriptor("linode_account_payment_get", Capability.Read),
        ToolDescriptor("linode_account_payment_methods_list", Capability.Read),
        ToolDescriptor("linode_account_payment_create", Capability.Write),
        ToolDescriptor("linode_account_payment_method_delete", Capability.Destroy),
        ToolDescriptor("linode_account_payments_list", Capability.Read),
        ToolDescriptor("linode_account_service_transfer_create", Capability.Write),
        ToolDescriptor("linode_account_login_get", Capability.Read),
        ToolDescriptor("linode_account_user_delete", Capability.Destroy),
        ToolDescriptor("linode_account_user_get", Capability.Read),
        ToolDescriptor("linode_account_user_grants_get", Capability.Read),
        ToolDescriptor("linode_account_logins_list", Capability.Read),
        ToolDescriptor("linode_account_maintenance_list", Capability.Read),
        ToolDescriptor("linode_account_users_list", Capability.Read),
        ToolDescriptor("linode_account_settings_get", Capability.Read),
        ToolDescriptor("linode_account_settings_managed_enable", Capability.Write),
        ToolDescriptor("linode_account_transfer_get", Capability.Read),
        ToolDescriptor("linode_account_notifications_list", Capability.Read),
        ToolDescriptor("linode_account_oauth_client_get", Capability.Read),
        ToolDescriptor("linode_account_payment_method_get", Capability.Read),
        ToolDescriptor("linode_account_payment_method_make_default", Capability.Write),
        ToolDescriptor("linode_account_oauth_client_thumbnail_get", Capability.Read),
        ToolDescriptor(
            "linode_account_oauth_client_thumbnail_update", Capability.Write
        ),
        ToolDescriptor("linode_account_oauth_client_update", Capability.Write),
        ToolDescriptor("linode_account_oauth_clients_list", Capability.Read),
        ToolDescriptor("linode_account_oauth_client_reset_secret", Capability.Write),
        ToolDescriptor("linode_account_child_account_token_create", Capability.Write),
        ToolDescriptor("linode_account_oauth_client_create", Capability.Write),
        ToolDescriptor("linode_account_oauth_client_delete", Capability.Destroy),
        ToolDescriptor("linode_account_payment_method_create", Capability.Write),
        ToolDescriptor("linode_account_promo_credit_add", Capability.Write),
        ToolDescriptor("linode_account_service_transfer_delete", Capability.Destroy),
        ToolDescriptor("linode_account_service_transfer_get", Capability.Read),
        ToolDescriptor("linode_account_user_update", Capability.Write),
        # Databases.
        ToolDescriptor("linode_database_engine_get", Capability.Read),
        ToolDescriptor("linode_database_type_get", Capability.Read),
        ToolDescriptor("linode_database_cluster_create", Capability.Write),
        ToolDescriptor("linode_database_mysql_credentials_reset", Capability.Write),
        ToolDescriptor(
            "linode_database_postgresql_credentials_reset", Capability.Write
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_credentials_get", Capability.Write
        ),
        ToolDescriptor("linode_database_mysql_instance_delete", Capability.Destroy),
        ToolDescriptor("linode_database_mysql_instance_patch", Capability.Write),
        ToolDescriptor("linode_database_mysql_instance_resume", Capability.Write),
        ToolDescriptor("linode_database_mysql_instance_suspend", Capability.Write),
        ToolDescriptor("linode_database_mysql_instance_update", Capability.Write),
        ToolDescriptor("linode_database_mysql_instances_list", Capability.Read),
        ToolDescriptor(
            "linode_database_postgresql_instance_delete", Capability.Destroy
        ),
        ToolDescriptor("linode_database_postgresql_instance_patch", Capability.Write),
        ToolDescriptor("linode_database_postgresql_instance_update", Capability.Write),
        ToolDescriptor(
            "linode_database_postgresql_instance_credentials_get", Capability.Write
        ),
        ToolDescriptor("linode_database_postgresql_instance_resume", Capability.Write),
        ToolDescriptor("linode_database_postgresql_instance_suspend", Capability.Write),
        ToolDescriptor("linode_database_postgresql_instances_list", Capability.Read),
        # Compute reads + mutations.
        ToolDescriptor("linode_instances_list", Capability.Read),
        ToolDescriptor("linode_instance_get", Capability.Read),
        ToolDescriptor("linode_instance_create", Capability.Write),
        ToolDescriptor("linode_instance_delete", Capability.Destroy),
        ToolDescriptor("linode_beta_get", Capability.Read),
        ToolDescriptor("linode_regions_list", Capability.Read),
        ToolDescriptor("linode_regions_availability_list", Capability.Read),
        ToolDescriptor("linode_regions_availability_get", Capability.Read),
        ToolDescriptor("linode_kernels_list", Capability.Read),
        ToolDescriptor("linode_types_list", Capability.Read),
        ToolDescriptor("linode_type_get", Capability.Read),
        ToolDescriptor("linode_database_mysql_config_get", Capability.Read),
        ToolDescriptor("linode_database_postgresql_config_get", Capability.Read),
        ToolDescriptor("linode_database_postgresql_instance_create", Capability.Write),
        ToolDescriptor("linode_database_postgresql_instance_ssl_get", Capability.Read),
        ToolDescriptor("linode_database_mysql_instance_get", Capability.Read),
        ToolDescriptor("linode_database_mysql_instance_ssl_get", Capability.Read),
        ToolDescriptor("linode_image_delete", Capability.Destroy),
        ToolDescriptor("linode_image_sharegroup_create", Capability.Write),
        ToolDescriptor("linode_image_sharegroups_by_image_list", Capability.Read),
        ToolDescriptor("linode_images_list", Capability.Read),
        ToolDescriptor("linode_images_sharegroup_delete", Capability.Destroy),
        ToolDescriptor("linode_images_sharegroup_image_delete", Capability.Destroy),
        ToolDescriptor("linode_images_sharegroup_images_add", Capability.Write),
        ToolDescriptor("linode_images_sharegroup_images_list", Capability.Read),
        ToolDescriptor("linode_images_sharegroup_members_add", Capability.Write),
        ToolDescriptor("linode_images_sharegroup_members_list", Capability.Read),
        ToolDescriptor("linode_images_sharegroups_list", Capability.Read),
        ToolDescriptor("linode_images_sharegroup_update", Capability.Write),
        ToolDescriptor("linode_images_sharegroups_token_create", Capability.Write),
        ToolDescriptor("linode_images_sharegroups_token_delete", Capability.Destroy),
        ToolDescriptor("linode_images_sharegroups_token_get", Capability.Read),
        ToolDescriptor(
            "linode_images_sharegroups_token_sharegroup_get", Capability.Read
        ),
        ToolDescriptor(
            "linode_images_sharegroups_token_sharegroup_images_list", Capability.Read
        ),
        ToolDescriptor("linode_images_sharegroups_token_update", Capability.Write),
        ToolDescriptor("linode_images_sharegroups_tokens_list", Capability.Read),
        ToolDescriptor("linode_stackscript_get", Capability.Read),
        ToolDescriptor("linode_stackscripts_list", Capability.Read),
        # Compute deep (backups, disks, ips).
        ToolDescriptor("linode_instance_backups_list", Capability.Read),
        ToolDescriptor("linode_instance_backup_create", Capability.Write),
        ToolDescriptor("linode_instance_config_create", Capability.Write),
        ToolDescriptor("linode_instance_disk_create", Capability.Write),
        ToolDescriptor("linode_instance_ip_allocate", Capability.Write),
        # Block storage.
        ToolDescriptor("linode_volumes_list", Capability.Read),
        ToolDescriptor("linode_volume_types_list", Capability.Read),
        ToolDescriptor("linode_volume_clone", Capability.Write),
        ToolDescriptor("linode_volume_create", Capability.Write),
        ToolDescriptor("linode_volume_delete", Capability.Destroy),
        # Object storage.
        ToolDescriptor("linode_object_storage_buckets_list", Capability.Read),
        ToolDescriptor("linode_object_storage_bucket_create", Capability.Write),
        # Databases.
        ToolDescriptor("linode_databases_engines_list", Capability.Read),
        ToolDescriptor("linode_databases_types_list", Capability.Read),
        # DNS.
        ToolDescriptor("linode_domains_list", Capability.Read),
        ToolDescriptor("linode_domain_clone", Capability.Write),
        ToolDescriptor("linode_domain_create", Capability.Write),
        ToolDescriptor("linode_domain_zone_file_get", Capability.Read),
        ToolDescriptor("linode_domain_import", Capability.Write),
        ToolDescriptor("linode_domain_record_create", Capability.Write),
        # Networking.
        ToolDescriptor("linode_firewalls_list", Capability.Read),
        ToolDescriptor("linode_firewall_create", Capability.Write),
        ToolDescriptor("linode_instance_firewalls_apply", Capability.Write),
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
        ToolDescriptor("linode_sshkey_get", Capability.Read),
        ToolDescriptor("linode_sshkey_create", Capability.Write),
        # Monitor.
        ToolDescriptor("linode_monitor_dashboard_get", Capability.Read),
        ToolDescriptor("linode_monitor_alert_channels_list", Capability.Read),
        ToolDescriptor("linode_monitor_alert_definitions_list", Capability.Read),
        ToolDescriptor("linode_monitor_dashboards_list", Capability.Read),
        ToolDescriptor("linode_monitor_service_dashboards_list", Capability.Read),
        ToolDescriptor("linode_monitor_service_get", Capability.Read),
        ToolDescriptor("linode_monitor_services_list", Capability.Read),
        ToolDescriptor(
            "linode_monitor_service_alert_definitions_list", Capability.Read
        ),
        ToolDescriptor(
            "linode_monitor_service_metric_definitions_list", Capability.Read
        ),
        ToolDescriptor("linode_monitor_service_metrics_read", Capability.Read),
        ToolDescriptor("linode_monitor_alert_definition_update", Capability.Write),
        ToolDescriptor("linode_monitor_service_token_create", Capability.Write),
        ToolDescriptor(
            "linode_monitor_service_alert_definition_delete", Capability.Destroy
        ),
        ToolDescriptor("linode_profile_phone_number_delete", Capability.Write),
        ToolDescriptor("linode_profile_phone_number_verify", Capability.Write),
        ToolDescriptor("linode_profile_security_questions_answer", Capability.Write),
        ToolDescriptor("linode_profile_app_revoke", Capability.Destroy),
        ToolDescriptor("linode_profile_tfa_disable", Capability.Write),
        ToolDescriptor("linode_profile_tfa_enable", Capability.Write),
        ToolDescriptor("linode_profile_tfa_enable_confirm", Capability.Write),
        # Admin tool (synthetic). Never selected by any built-in.
        ToolDescriptor("linode_admin_synthetic", Capability.Admin),
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
    assert "linode_volume_clone" in compute_admin_tools
    assert "linode_volume_create" in compute_admin_tools
    assert "linode_sshkey_create" in compute_admin_tools


def test_network_admin_excludes_compute_writes() -> None:
    """Compute mutators do NOT leak into the network admin profile."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    network_admin_tools = set(profiles["network-admin"].allowed_tools)
    assert "linode_instance_create" not in network_admin_tools
    assert "linode_volume_clone" not in network_admin_tools
    assert "linode_volume_create" not in network_admin_tools
    # But network mutators are present.
    assert "linode_firewall_create" in network_admin_tools
    assert "linode_domain_clone" in network_admin_tools
    assert "linode_domain_create" in network_admin_tools
    assert "linode_domain_import" in network_admin_tools
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
    assert "linode_domain_import" not in k8s_tools
    assert "linode_firewall_create" not in k8s_tools


def test_storage_admin_includes_backups_but_not_other_compute() -> None:
    """Storage admin elevates compute_deep (backups) without all of compute."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    storage_tools = set(profiles["storage-admin"].allowed_tools)
    assert "linode_instance_backup_create" in storage_tools
    assert "linode_volume_clone" in storage_tools
    assert "linode_volume_create" in storage_tools
    assert "linode_object_storage_bucket_create" in storage_tools
    # No general compute write access.
    assert "linode_instance_create" not in storage_tools


def test_linode_kernels_list_requires_linodes_read_scope() -> None:
    """Kernels list requires the Linodes read token scope."""
    assert required_scopes("linode_kernels_list", Capability.Read) == [
        Scope.LinodesReadOnly
    ]
    assert categories("linode_kernels_list") == ["compute"]


def test_database_tools_require_database_read_scope() -> None:
    """Managed Database tools require the database token scope."""
    assert required_scopes("linode_database_instances_list", Capability.Read) == [
        Scope.DatabasesReadOnly
    ]
    assert categories("linode_database_instances_list") == ["databases"]
    assert required_scopes("linode_database_mysql_config_get", Capability.Read) == [
        Scope.DatabasesReadOnly
    ]
    assert categories("linode_database_mysql_config_get") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_config_get", Capability.Read
    ) == [Scope.DatabasesReadOnly]
    assert categories("linode_database_postgresql_config_get") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_instance_create", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_postgresql_instance_create") == ["databases"]
    assert categories("linode_databases_types_list") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_instance_ssl_get", Capability.Read
    ) == [Scope.DatabasesReadOnly]
    assert categories("linode_database_postgresql_instance_ssl_get") == ["databases"]
    assert required_scopes("linode_database_cluster_create", Capability.Write) == [
        Scope.DatabasesReadWrite
    ]
    assert categories("linode_database_cluster_create") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_credentials_reset", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_mysql_credentials_reset") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_credentials_reset", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_postgresql_credentials_reset") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_delete", Capability.Destroy
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_mysql_instance_delete") == ["databases"]
    assert required_scopes("linode_database_mysql_instance_get", Capability.Read) == [
        Scope.DatabasesReadOnly
    ]
    assert categories("linode_database_mysql_instance_get") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_ssl_get", Capability.Read
    ) == [Scope.DatabasesReadOnly]
    assert categories("linode_database_mysql_instance_ssl_get") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_credentials_get", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_mysql_instance_credentials_get") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_resume", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_mysql_instance_resume") == ["databases"]
    assert categories("linode_database_postgresql_instance_resume") == ["databases"]
    assert required_scopes("linode_database_mysql_instances_list", Capability.Read) == [
        Scope.DatabasesReadOnly
    ]
    assert categories("linode_database_mysql_instances_list") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_instance_patch", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_postgresql_instance_patch") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_instance_update", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_postgresql_instance_update") == ["databases"]
    assert required_scopes(
        "linode_database_postgresql_instance_credentials_get", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_postgresql_instance_credentials_get") == [
        "databases"
    ]
    assert required_scopes(
        "linode_database_postgresql_instances_list", Capability.Read
    ) == [Scope.DatabasesReadOnly]
    assert categories("linode_database_postgresql_instances_list") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_patch", Capability.Write
    ) == [Scope.DatabasesReadWrite]
    assert categories("linode_database_mysql_instance_patch") == ["databases"]
    assert categories("linode_database_mysql_instance_update") == ["databases"]
    assert required_scopes(
        "linode_database_mysql_instance_update", Capability.Write
    ) == [Scope.DatabasesReadWrite]


def test_database_credentials_tool_is_not_readonly_profile_tool() -> None:
    """Credential retrieval is not granted to generic read-only profiles."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    for tool_name in (
        "linode_database_mysql_instance_credentials_get",
        "linode_database_postgresql_instance_credentials_get",
    ):
        assert tool_name not in profiles["default"].allowed_tools
        assert tool_name not in profiles["readonly-full"].allowed_tools
        assert tool_name in profiles["full-access"].allowed_tools


def test_account_payment_method_delete_is_account_category() -> None:
    """Payment-method deletion is an account destroy tool."""
    assert categories("linode_account_payment_method_delete") == ["account"]


def test_account_service_transfer_delete_is_account_category() -> None:
    """Service-transfer deletion is an account destroy tool."""
    assert categories("linode_account_service_transfer_delete") == ["account"]


def test_profile_app_revoke_is_account_category() -> None:
    """OAuth app revoke is an account-profile destroy tool."""
    assert categories("linode_profile_app_revoke") == ["account"]


def test_profile_phone_number_delete_is_account_category() -> None:
    """Phone-number deletion is an account-profile write tool."""
    assert categories("linode_profile_phone_number_delete") == ["account"]


def test_profile_phone_number_verify_is_account_category() -> None:
    """Phone-number verification is an account-profile write tool."""
    assert categories("linode_profile_phone_number_verify") == ["account"]


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
    assert "linode_admin_synthetic" not in full_tools
    assert "linode_undiscovered_thing" not in full_tools


def test_allowed_tools_are_sorted_for_determinism() -> None:
    """Sorted output keeps the cross-language JSON parity check stable."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    for name, profile in profiles.items():
        tools = list(profile.allowed_tools)
        assert tools == sorted(tools), f"{name} allowed_tools not sorted"


def test_required_token_scopes_derived_from_tools() -> None:
    """Phase 6.3 contract: each profile's required_token_scopes equals
    the deduplicated, sorted union of required_scopes() over its
    allowed_tools.

    Pins the derivation against drift between the scope catalog and the
    blueprint, and catches anyone trying to restore hardcoded scope
    tuples by mistake.
    """
    catalog = _synthetic_catalog()
    built = builtin_profiles(catalog)
    cap_by_name = {d.name: d.capability for d in catalog}

    for name, prof in built.items():
        expected: set[str] = set()
        for tool_name in prof.allowed_tools:
            capability = cap_by_name.get(tool_name)
            if capability is None:
                continue
            for scope in required_scopes(tool_name, capability):
                expected.add(scope.value)
        assert set(prof.required_token_scopes) == expected, (
            f"profile {name} scope union mismatch: "
            f"got {prof.required_token_scopes}, expected {sorted(expected)}"
        )
        assert list(prof.required_token_scopes) == sorted(prof.required_token_scopes), (
            f"profile {name} required_token_scopes must be sorted "
            "ascending for cross-language parity"
        )


def test_read_only_profiles_have_no_write_scopes() -> None:
    """Default and readonly-full carry only :read_only scopes.

    A regression that lets a write tool slip into a read-only built-in
    would surface here as a :read_write scope appearing on the profile.
    """
    catalog = _synthetic_catalog()
    built = builtin_profiles(catalog)

    for name in ("default", "readonly-full"):
        for scope in built[name].required_token_scopes:
            assert ":read_write" not in scope, (
                f"profile {name} is read-only but lists write scope {scope!r}"
            )


def test_database_tool_category() -> None:
    """Database tools map to the databases profile category."""
    assert categories("linode_database_engine_get") == ["databases"]
    assert categories("linode_database_type_get") == ["databases"]
    assert categories("linode_database_cluster_create") == ["databases"]
    assert categories("linode_database_postgresql_instance_create") == ["databases"]


def test_full_access_scopes_match_expected_categories() -> None:
    """Full-access aggregates every write scope the catalog can produce.

    The synthetic catalog includes write tools for compute, volumes,
    domains, firewall, nodebalancers, LKE, object storage, and VPC.
    StackScripts is intentionally absent (only a list-read tool exists
    in the fixture), so stackscripts:read_write should NOT appear.
    Images:read_only is pulled in by instance_create's cross-category
    extras table.
    """
    built = builtin_profiles(_synthetic_catalog())
    full = built["full-access"].required_token_scopes

    want_present = {
        Scope.LinodesReadWrite.value,
        Scope.VolumesReadWrite.value,
        Scope.DatabasesReadWrite.value,
        Scope.DomainsReadWrite.value,
        Scope.FirewallReadWrite.value,
        Scope.NodeBalancersReadWrite.value,
        Scope.LKEReadWrite.value,
        Scope.ObjectStorageReadWrite.value,
        Scope.VPCReadWrite.value,
        Scope.ImagesReadOnly.value,
    }
    for scope in want_present:
        assert scope in full, f"full-access should include {scope}"

    assert Scope.StackScriptsReadWrite.value not in full, (
        "fixture has no stackscripts write tool; scope should not be in the derived set"
    )


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
