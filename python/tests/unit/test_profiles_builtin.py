"""Phase 2 tests for the built-in profile catalog.

Synthetic ``ToolDescriptor`` lists drive every test so failures point at
the resolver, not at whatever happens to be in the live tool registry on
the day the test runs. The cross-language parity check (Go side) reads
the JSON exported via ``builtin_catalog_json`` against a matching catalog.
"""

from __future__ import annotations

import dataclasses
import json

from linodemcp.gentools import categories_for, scopes_for
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
    catalog = [
        # Core: always included via Meta/Read.
        ToolDescriptor("hello", Capability.Meta),
        ToolDescriptor("version", Capability.Meta),
        ToolDescriptor("linode_profile_get", Capability.Read),
        ToolDescriptor(
            "linode_profile_security_question_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_account_get", Capability.Read),
        ToolDescriptor("linode_account_beta_list", Capability.Read),
        ToolDescriptor("linode_beta_list", Capability.Read),
        ToolDescriptor("linode_database_instance_list", Capability.Read),
        ToolDescriptor(
            "linode_account_child_account_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_account_user_create", Capability.Write),
        ToolDescriptor(
            "linode_account_service_transfer_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_account_service_transfer_accept",
            Capability.Write,
        ),
        ToolDescriptor("linode_account_event_list", Capability.Read),
        ToolDescriptor("linode_account_event_seen", Capability.Write),
        ToolDescriptor("linode_account_invoice_get", Capability.Read),
        ToolDescriptor("linode_account_invoice_item_list", Capability.Read),
        ToolDescriptor("linode_account_invoice_list", Capability.Read),
        ToolDescriptor("linode_account_payment_get", Capability.Read),
        ToolDescriptor(
            "linode_account_payment_method_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_account_payment_create", Capability.Write),
        ToolDescriptor(
            "linode_account_payment_method_delete",
            Capability.Destroy,
        ),
        ToolDescriptor("linode_account_payment_list", Capability.Read),
        ToolDescriptor(
            "linode_account_service_transfer_create",
            Capability.Write,
        ),
        ToolDescriptor("linode_account_login_get", Capability.Read),
        ToolDescriptor("linode_account_user_delete", Capability.Destroy),
        ToolDescriptor("linode_account_user_get", Capability.Read),
        ToolDescriptor("linode_account_user_grants_get", Capability.Read),
        ToolDescriptor("linode_account_login_list", Capability.Read),
        ToolDescriptor("linode_account_maintenance_list", Capability.Read),
        ToolDescriptor("linode_maintenance_policy_list", Capability.Read),
        ToolDescriptor("linode_account_user_list", Capability.Read),
        ToolDescriptor("linode_account_settings_get", Capability.Read),
        ToolDescriptor(
            "linode_account_settings_managed_enable",
            Capability.Write,
        ),
        ToolDescriptor("linode_managed_contact_create", Capability.Write),
        ToolDescriptor("linode_account_transfer_get", Capability.Read),
        ToolDescriptor("linode_account_notification_list", Capability.Read),
        ToolDescriptor("linode_account_oauth_client_get", Capability.Read),
        ToolDescriptor(
            "linode_account_payment_method_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_account_payment_method_make_default",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_account_oauth_client_thumbnail_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_account_oauth_client_thumbnail_update", Capability.Write
        ),
        ToolDescriptor(
            "linode_account_oauth_client_update",
            Capability.Write,
        ),
        ToolDescriptor("linode_account_oauth_client_list", Capability.Read),
        ToolDescriptor(
            "linode_account_oauth_client_secret_reset",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_account_child_account_token_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_account_oauth_client_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_account_oauth_client_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_account_payment_method_create",
            Capability.Write,
        ),
        ToolDescriptor("linode_account_promo_credit_add", Capability.Write),
        ToolDescriptor(
            "linode_account_service_transfer_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_account_service_transfer_get",
            Capability.Read,
        ),
        ToolDescriptor("linode_account_user_update", Capability.Write),
        # Databases.
        ToolDescriptor("linode_database_engine_get", Capability.Read),
        ToolDescriptor("linode_database_type_get", Capability.Read),
        ToolDescriptor(
            "linode_database_mysql_instance_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_credentials_reset", Capability.Write
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_credentials_reset", Capability.Write
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_credentials_get", Capability.Write
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_patch",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_resume",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_suspend",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_update",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_delete", Capability.Destroy
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_patch",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_update",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_credentials_get", Capability.Write
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_resume",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_suspend",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_list",
            Capability.Read,
        ),
        # Compute reads + mutations.
        ToolDescriptor("linode_instance_list", Capability.Read),
        ToolDescriptor("linode_instance_get", Capability.Read),
        ToolDescriptor("linode_instance_create", Capability.Write),
        ToolDescriptor("linode_instance_delete", Capability.Destroy),
        ToolDescriptor("linode_beta_get", Capability.Read),
        ToolDescriptor("linode_region_list", Capability.Read),
        ToolDescriptor("linode_region_availability_list", Capability.Read),
        ToolDescriptor("linode_region_availability_get", Capability.Read),
        ToolDescriptor("linode_kernel_list", Capability.Read),
        ToolDescriptor("linode_type_list", Capability.Read),
        ToolDescriptor("linode_type_get", Capability.Read),
        ToolDescriptor(
            "linode_database_mysql_config_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_database_postgresql_config_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_database_postgresql_instance_ssl_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_database_mysql_instance_ssl_get",
            Capability.Read,
        ),
        ToolDescriptor("linode_image_delete", Capability.Destroy),
        ToolDescriptor("linode_image_sharegroup_create", Capability.Write),
        ToolDescriptor(
            "linode_image_sharegroup_by_image_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_image_list", Capability.Read),
        ToolDescriptor(
            "linode_image_sharegroup_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_image_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_image_add",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_image_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_member_add",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_member_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_image_sharegroup_list", Capability.Read),
        ToolDescriptor("linode_image_sharegroup_update", Capability.Write),
        ToolDescriptor(
            "linode_image_sharegroup_token_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_token_delete",
            Capability.Destroy,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_token_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_by_token_get",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_token_image_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_token_update",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_image_sharegroup_token_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_stackscript_get", Capability.Read),
        ToolDescriptor("linode_stackscript_list", Capability.Read),
        # Compute deep (backups, disks, ips).
        ToolDescriptor(
            "linode_instance_backup_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_instance_backup_create",
            Capability.Write,
        ),
        ToolDescriptor("linode_instance_config_create", Capability.Write),
        ToolDescriptor(
            "linode_instance_disk_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_instance_ip_allocate",
            Capability.Write,
        ),
        # Block storage.
        ToolDescriptor("linode_volume_list", Capability.Read),
        ToolDescriptor("linode_volume_type_list", Capability.Read),
        ToolDescriptor("linode_volume_clone", Capability.Write),
        ToolDescriptor("linode_volume_create", Capability.Write),
        ToolDescriptor("linode_volume_delete", Capability.Destroy),
        # Object storage.
        ToolDescriptor(
            "linode_object_storage_bucket_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_object_storage_bucket_create",
            Capability.Write,
        ),
        # Databases.
        ToolDescriptor("linode_database_engine_list", Capability.Read),
        ToolDescriptor("linode_database_type_list", Capability.Read),
        # DNS.
        ToolDescriptor("linode_domain_list", Capability.Read),
        ToolDescriptor("linode_domain_clone", Capability.Write),
        ToolDescriptor("linode_domain_create", Capability.Write),
        ToolDescriptor("linode_domain_zone_file_get", Capability.Read),
        ToolDescriptor("linode_domain_import", Capability.Write),
        ToolDescriptor("linode_domain_record_create", Capability.Write),
        # Networking.
        ToolDescriptor("linode_firewall_list", Capability.Read),
        ToolDescriptor("linode_firewall_create", Capability.Write),
        ToolDescriptor("linode_instance_firewall_apply", Capability.Write),
        ToolDescriptor("linode_nodebalancer_create", Capability.Write),
        ToolDescriptor("linode_vlan_delete", Capability.Destroy),
        ToolDescriptor("linode_ipv6_range_create", Capability.Write),
        # LKE.
        ToolDescriptor("linode_lke_cluster_list", Capability.Read),
        ToolDescriptor("linode_lke_cluster_create", Capability.Write),
        ToolDescriptor("linode_lke_cluster_delete", Capability.Destroy),
        # VPCs.
        ToolDescriptor("linode_vpc_list", Capability.Read),
        ToolDescriptor("linode_vpc_create", Capability.Write),
        ToolDescriptor("linode_vpc_subnet_create", Capability.Write),
        # Security (SSH keys).
        ToolDescriptor("linode_sshkey_list", Capability.Read),
        ToolDescriptor("linode_sshkey_get", Capability.Read),
        ToolDescriptor("linode_sshkey_create", Capability.Write),
        # Monitor.
        ToolDescriptor("linode_monitor_dashboard_get", Capability.Read),
        ToolDescriptor(
            "linode_monitor_alert_channel_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_monitor_alert_definition_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_monitor_dashboard_list", Capability.Read),
        ToolDescriptor(
            "linode_monitor_service_dashboard_list",
            Capability.Read,
        ),
        ToolDescriptor("linode_monitor_service_get", Capability.Read),
        ToolDescriptor("linode_monitor_service_list", Capability.Read),
        ToolDescriptor(
            "linode_monitor_service_alert_definition_list",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_monitor_service_metric_definition_list", Capability.Read
        ),
        ToolDescriptor(
            "linode_monitor_service_metric_query",
            Capability.Read,
        ),
        ToolDescriptor(
            "linode_monitor_service_alert_definition_update", Capability.Write
        ),
        ToolDescriptor(
            "linode_monitor_service_token_create",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_monitor_service_alert_definition_delete", Capability.Destroy
        ),
        ToolDescriptor(
            "linode_profile_phone_number_delete",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_profile_phone_number_verify",
            Capability.Write,
        ),
        ToolDescriptor(
            "linode_profile_security_question_answer",
            Capability.Write,
        ),
        ToolDescriptor("linode_profile_app_delete", Capability.Destroy),
        ToolDescriptor("linode_profile_tfa_disable", Capability.Write),
        ToolDescriptor("linode_profile_tfa_enable", Capability.Write),
        ToolDescriptor(
            "linode_profile_tfa_enable_confirm",
            Capability.Write,
        ),
        # Admin tool (synthetic). Never selected by any built-in.
        ToolDescriptor("linode_admin_synthetic", Capability.Admin),
        # Unknown tool (synthetic). Never selected; resolver ignores it.
        ToolDescriptor("linode_undiscovered_thing", Capability.Unknown),
    ]

    return _with_fixture_declarations(catalog)


# Each synthetic tool's declared scope answer, frozen from the retired hand
# resolver when the contract took over, so no production source can quietly
# rewrite the expectation. Tools resolving no scope are absent.
_FIXTURE_SCOPES: dict[str, tuple[str, ...]] = {
    "linode_profile_security_question_list": ("account:read_only",),
    "linode_account_get": ("account:read_only",),
    "linode_account_beta_list": ("account:read_only",),
    "linode_database_instance_list": ("databases:read_only",),
    "linode_account_child_account_list": ("account:read_only",),
    "linode_account_user_create": ("account:read_write",),
    "linode_account_service_transfer_list": ("account:read_only",),
    "linode_account_service_transfer_accept": ("account:read_write",),
    "linode_account_event_list": ("events:read_only",),
    "linode_account_event_seen": ("events:read_only",),
    "linode_account_invoice_get": ("account:read_only",),
    "linode_account_invoice_item_list": ("account:read_only",),
    "linode_account_invoice_list": ("account:read_only",),
    "linode_account_payment_get": ("account:read_only",),
    "linode_account_payment_method_list": ("account:read_only",),
    "linode_account_payment_create": ("account:read_write",),
    "linode_account_payment_method_delete": ("account:read_only",),
    "linode_account_payment_list": ("account:read_only",),
    "linode_account_service_transfer_create": ("account:read_write",),
    "linode_account_login_get": ("account:read_only",),
    "linode_account_user_delete": ("account:read_write",),
    "linode_account_user_get": ("account:read_only",),
    "linode_account_user_grants_get": ("account:read_only",),
    "linode_account_login_list": ("account:read_only",),
    "linode_account_user_list": ("account:read_only",),
    "linode_account_settings_get": ("account:read_only",),
    "linode_account_settings_managed_enable": ("account:read_write",),
    "linode_managed_contact_create": ("account:read_write",),
    "linode_account_transfer_get": ("account:read_only",),
    "linode_account_notification_list": ("account:read_only",),
    "linode_account_oauth_client_get": ("account:read_only",),
    "linode_account_payment_method_get": ("account:read_only",),
    "linode_account_payment_method_make_default": ("account:read_write",),
    "linode_account_oauth_client_thumbnail_update": ("account:read_write",),
    "linode_account_oauth_client_update": ("account:read_write",),
    "linode_account_oauth_client_list": ("account:read_only",),
    "linode_account_oauth_client_secret_reset": ("account:read_write",),
    "linode_account_child_account_token_create": ("account:read_write",),
    "linode_account_oauth_client_create": ("account:read_write",),
    "linode_account_oauth_client_delete": ("account:read_write",),
    "linode_account_payment_method_create": ("account:read_write",),
    "linode_account_promo_credit_add": ("account:read_only",),
    "linode_account_service_transfer_delete": ("account:read_write",),
    "linode_account_service_transfer_get": ("account:read_only",),
    "linode_account_user_update": ("account:read_write",),
    "linode_database_mysql_instance_create": ("databases:read_write",),
    "linode_database_mysql_instance_credentials_reset": ("databases:read_write",),
    "linode_database_postgresql_instance_credentials_reset": ("databases:read_write",),
    "linode_database_mysql_instance_credentials_get": ("databases:read_only",),
    "linode_database_mysql_instance_delete": ("databases:read_write",),
    "linode_database_mysql_instance_patch": ("databases:read_write",),
    "linode_database_mysql_instance_resume": ("databases:read_write",),
    "linode_database_mysql_instance_suspend": ("databases:read_write",),
    "linode_database_mysql_instance_update": ("databases:read_write",),
    "linode_database_mysql_instance_list": ("databases:read_only",),
    "linode_database_postgresql_instance_delete": ("databases:read_write",),
    "linode_database_postgresql_instance_patch": ("databases:read_write",),
    "linode_database_postgresql_instance_update": ("databases:read_write",),
    "linode_database_postgresql_instance_credentials_get": ("databases:read_only",),
    "linode_database_postgresql_instance_resume": ("databases:read_write",),
    "linode_database_postgresql_instance_suspend": ("databases:read_write",),
    "linode_database_postgresql_instance_list": ("databases:read_only",),
    "linode_instance_list": ("linodes:read_only",),
    "linode_instance_get": ("linodes:read_only",),
    "linode_instance_create": ("linodes:read_write",),
    "linode_instance_delete": ("linodes:read_write",),
    "linode_database_mysql_config_get": ("databases:read_only",),
    "linode_database_postgresql_config_get": ("databases:read_only",),
    "linode_database_postgresql_instance_create": ("databases:read_write",),
    "linode_database_postgresql_instance_ssl_get": ("databases:read_only",),
    "linode_database_mysql_instance_get": ("databases:read_only",),
    "linode_database_mysql_instance_ssl_get": ("databases:read_only",),
    "linode_image_delete": ("images:read_write",),
    "linode_image_sharegroup_create": ("images:read_write",),
    "linode_image_sharegroup_by_image_list": ("images:read_only",),
    "linode_image_list": ("images:read_only",),
    "linode_image_sharegroup_delete": ("images:read_write",),
    "linode_image_sharegroup_image_delete": ("images:read_write",),
    "linode_image_sharegroup_image_add": ("images:read_write",),
    "linode_image_sharegroup_image_list": ("images:read_only",),
    "linode_image_sharegroup_member_add": ("images:read_write",),
    "linode_image_sharegroup_member_list": ("images:read_only",),
    "linode_image_sharegroup_list": ("images:read_only",),
    "linode_image_sharegroup_update": ("images:read_write",),
    "linode_image_sharegroup_token_create": ("images:read_write",),
    "linode_image_sharegroup_token_delete": ("images:read_write",),
    "linode_image_sharegroup_token_get": ("images:read_only",),
    "linode_image_sharegroup_by_token_get": ("images:read_only",),
    "linode_image_sharegroup_token_image_list": ("images:read_only",),
    "linode_image_sharegroup_token_update": ("images:read_write",),
    "linode_image_sharegroup_token_list": ("images:read_only",),
    "linode_stackscript_get": ("stackscripts:read_only",),
    "linode_stackscript_list": ("stackscripts:read_only",),
    "linode_instance_backup_list": ("linodes:read_only",),
    "linode_instance_backup_create": ("linodes:read_write",),
    "linode_instance_config_create": ("linodes:read_write",),
    "linode_instance_disk_create": ("linodes:read_write",),
    "linode_instance_ip_allocate": ("linodes:read_write",),
    "linode_volume_list": ("volumes:read_only",),
    "linode_volume_clone": ("volumes:read_write",),
    "linode_volume_create": ("volumes:read_write",),
    "linode_volume_delete": ("volumes:read_write",),
    "linode_object_storage_bucket_list": ("object_storage:read_only",),
    "linode_object_storage_bucket_create": ("object_storage:read_write",),
    "linode_domain_list": ("domains:read_only",),
    "linode_domain_clone": ("domains:read_write",),
    "linode_domain_create": ("domains:read_write",),
    "linode_domain_zone_file_get": ("domains:read_only",),
    "linode_domain_import": ("domains:read_write",),
    "linode_domain_record_create": ("domains:read_write",),
    "linode_firewall_list": ("firewall:read_only",),
    "linode_firewall_create": ("firewall:read_write",),
    "linode_instance_firewall_apply": ("linodes:read_write",),
    "linode_nodebalancer_create": ("nodebalancers:read_write",),
    "linode_vlan_delete": ("linodes:read_write",),
    "linode_ipv6_range_create": (
        "ips:read_write",
        "linodes:read_write",
    ),
    "linode_lke_cluster_list": ("lke:read_only",),
    "linode_lke_cluster_create": ("lke:read_write",),
    "linode_lke_cluster_delete": ("lke:read_write",),
    "linode_vpc_create": ("vpc:read_write",),
    "linode_vpc_subnet_create": ("vpc:read_write",),
    "linode_sshkey_list": ("account:read_only",),
    "linode_sshkey_get": ("account:read_only",),
    "linode_sshkey_create": ("account:read_write",),
    "linode_monitor_dashboard_get": ("monitor:read_only",),
    "linode_monitor_alert_channel_list": ("monitor:read_only",),
    "linode_monitor_alert_definition_list": ("monitor:read_only",),
    "linode_monitor_dashboard_list": ("monitor:read_only",),
    "linode_monitor_service_dashboard_list": ("monitor:read_only",),
    "linode_monitor_service_get": ("monitor:read_only",),
    "linode_monitor_service_list": ("monitor:read_only",),
    "linode_monitor_service_alert_definition_list": ("monitor:read_only",),
    "linode_monitor_service_metric_definition_list": ("monitor:read_only",),
    "linode_monitor_service_alert_definition_update": ("monitor:read_write",),
    "linode_monitor_service_token_create": ("monitor:read_only",),
    "linode_monitor_service_alert_definition_delete": ("monitor:read_write",),
    "linode_profile_phone_number_delete": ("account:read_write",),
    "linode_profile_phone_number_verify": ("account:read_write",),
    "linode_profile_security_question_answer": ("account:read_write",),
    "linode_profile_app_delete": ("account:read_write",),
    "linode_profile_tfa_disable": ("account:read_write",),
    "linode_profile_tfa_enable": ("account:read_write",),
    "linode_profile_tfa_enable_confirm": ("account:read_write",),
}


# Each synthetic tool's declared categories, frozen from the retired hand
# resolver when the contract took over, so no production source can quietly
# rewrite the expectation. Tools resolving no category are absent.
_FIXTURE_CATEGORIES: dict[str, tuple[str, ...]] = {
    "hello": ("core",),
    "version": ("core",),
    "linode_profile_get": ("core",),
    "linode_profile_security_question_list": ("account",),
    "linode_account_get": ("core",),
    "linode_account_beta_list": ("account",),
    "linode_beta_list": ("account",),
    "linode_database_instance_list": ("databases",),
    "linode_account_child_account_list": ("account",),
    "linode_account_user_create": ("account",),
    "linode_account_service_transfer_list": ("account",),
    "linode_account_service_transfer_accept": ("account",),
    "linode_account_event_list": ("account",),
    "linode_account_event_seen": ("account",),
    "linode_account_invoice_get": ("account",),
    "linode_account_invoice_item_list": ("account",),
    "linode_account_invoice_list": ("account",),
    "linode_account_payment_get": ("account",),
    "linode_account_payment_method_list": ("account",),
    "linode_account_payment_create": ("account",),
    "linode_account_payment_method_delete": ("account",),
    "linode_account_payment_list": ("account",),
    "linode_account_service_transfer_create": ("account",),
    "linode_account_login_get": ("account",),
    "linode_account_user_delete": ("account",),
    "linode_account_user_get": ("account",),
    "linode_account_user_grants_get": ("account",),
    "linode_account_login_list": ("account",),
    "linode_account_maintenance_list": ("account",),
    "linode_maintenance_policy_list": ("account",),
    "linode_account_user_list": ("account",),
    "linode_account_settings_get": ("account",),
    "linode_account_settings_managed_enable": ("account",),
    "linode_managed_contact_create": ("account",),
    "linode_account_transfer_get": ("account",),
    "linode_account_notification_list": ("account",),
    "linode_account_oauth_client_get": ("account",),
    "linode_account_payment_method_get": ("account",),
    "linode_account_payment_method_make_default": ("account",),
    "linode_account_oauth_client_thumbnail_get": ("account",),
    "linode_account_oauth_client_thumbnail_update": ("account",),
    "linode_account_oauth_client_update": ("account",),
    "linode_account_oauth_client_list": ("account",),
    "linode_account_oauth_client_secret_reset": ("account",),
    "linode_account_child_account_token_create": ("account",),
    "linode_account_oauth_client_create": ("account",),
    "linode_account_oauth_client_delete": ("account",),
    "linode_account_payment_method_create": ("account",),
    "linode_account_promo_credit_add": ("account",),
    "linode_account_service_transfer_delete": ("account",),
    "linode_account_service_transfer_get": ("account",),
    "linode_account_user_update": ("account",),
    "linode_database_engine_get": ("databases",),
    "linode_database_type_get": ("databases",),
    "linode_database_mysql_instance_create": ("databases",),
    "linode_database_mysql_instance_credentials_reset": ("databases",),
    "linode_database_postgresql_instance_credentials_reset": ("databases",),
    "linode_database_mysql_instance_credentials_get": ("databases",),
    "linode_database_mysql_instance_delete": ("databases",),
    "linode_database_mysql_instance_patch": ("databases",),
    "linode_database_mysql_instance_resume": ("databases",),
    "linode_database_mysql_instance_suspend": ("databases",),
    "linode_database_mysql_instance_update": ("databases",),
    "linode_database_mysql_instance_list": ("databases",),
    "linode_database_postgresql_instance_delete": ("databases",),
    "linode_database_postgresql_instance_patch": ("databases",),
    "linode_database_postgresql_instance_update": ("databases",),
    "linode_database_postgresql_instance_credentials_get": ("databases",),
    "linode_database_postgresql_instance_resume": ("databases",),
    "linode_database_postgresql_instance_suspend": ("databases",),
    "linode_database_postgresql_instance_list": ("databases",),
    "linode_instance_list": ("compute",),
    "linode_instance_get": ("compute",),
    "linode_instance_create": ("compute",),
    "linode_instance_delete": ("compute",),
    "linode_beta_get": ("account",),
    "linode_region_list": ("compute",),
    "linode_region_availability_list": ("compute",),
    "linode_region_availability_get": ("compute",),
    "linode_kernel_list": ("compute",),
    "linode_type_list": ("compute",),
    "linode_type_get": ("compute",),
    "linode_database_mysql_config_get": ("databases",),
    "linode_database_postgresql_config_get": ("databases",),
    "linode_database_postgresql_instance_create": ("databases",),
    "linode_database_postgresql_instance_ssl_get": ("databases",),
    "linode_database_mysql_instance_get": ("databases",),
    "linode_database_mysql_instance_ssl_get": ("databases",),
    "linode_image_delete": ("compute",),
    "linode_image_sharegroup_create": ("compute",),
    "linode_image_sharegroup_by_image_list": ("compute",),
    "linode_image_list": ("compute",),
    "linode_image_sharegroup_delete": ("compute",),
    "linode_image_sharegroup_image_delete": ("compute",),
    "linode_image_sharegroup_image_add": ("compute",),
    "linode_image_sharegroup_image_list": ("compute",),
    "linode_image_sharegroup_member_add": ("compute",),
    "linode_image_sharegroup_member_list": ("compute",),
    "linode_image_sharegroup_list": ("compute",),
    "linode_image_sharegroup_update": ("compute",),
    "linode_image_sharegroup_token_create": ("compute",),
    "linode_image_sharegroup_token_delete": ("compute",),
    "linode_image_sharegroup_token_get": ("compute",),
    "linode_image_sharegroup_by_token_get": ("compute",),
    "linode_image_sharegroup_token_image_list": ("compute",),
    "linode_image_sharegroup_token_update": ("compute",),
    "linode_image_sharegroup_token_list": ("compute",),
    "linode_stackscript_get": ("compute",),
    "linode_stackscript_list": ("compute",),
    "linode_instance_backup_list": (
        "compute_deep",
        "compute",
    ),
    "linode_instance_backup_create": (
        "compute_deep",
        "compute",
    ),
    "linode_instance_config_create": ("compute",),
    "linode_instance_disk_create": (
        "compute_deep",
        "compute",
    ),
    "linode_instance_ip_allocate": (
        "compute_deep",
        "compute",
    ),
    "linode_volume_list": ("block_storage",),
    "linode_volume_type_list": ("block_storage",),
    "linode_volume_clone": ("block_storage",),
    "linode_volume_create": ("block_storage",),
    "linode_volume_delete": ("block_storage",),
    "linode_object_storage_bucket_list": ("object_storage",),
    "linode_object_storage_bucket_create": ("object_storage",),
    "linode_database_engine_list": ("databases",),
    "linode_database_type_list": ("databases",),
    "linode_domain_list": ("dns",),
    "linode_domain_clone": ("dns",),
    "linode_domain_create": ("dns",),
    "linode_domain_zone_file_get": ("dns",),
    "linode_domain_import": ("dns",),
    "linode_domain_record_create": ("dns",),
    "linode_firewall_list": ("networking",),
    "linode_firewall_create": ("networking",),
    "linode_instance_firewall_apply": ("compute",),
    "linode_nodebalancer_create": ("networking",),
    "linode_vlan_delete": ("networking",),
    "linode_ipv6_range_create": ("networking",),
    "linode_lke_cluster_list": ("lke",),
    "linode_lke_cluster_create": ("lke",),
    "linode_lke_cluster_delete": ("lke",),
    "linode_vpc_list": ("vpcs",),
    "linode_vpc_create": ("vpcs",),
    "linode_vpc_subnet_create": ("vpcs",),
    "linode_sshkey_list": ("security",),
    "linode_sshkey_get": ("security",),
    "linode_sshkey_create": ("security",),
    "linode_monitor_dashboard_get": ("monitor",),
    "linode_monitor_alert_channel_list": ("monitor",),
    "linode_monitor_alert_definition_list": ("monitor",),
    "linode_monitor_dashboard_list": ("monitor",),
    "linode_monitor_service_dashboard_list": ("monitor",),
    "linode_monitor_service_get": ("monitor",),
    "linode_monitor_service_list": ("monitor",),
    "linode_monitor_service_alert_definition_list": ("monitor",),
    "linode_monitor_service_metric_definition_list": ("monitor",),
    "linode_monitor_service_metric_query": ("monitor",),
    "linode_monitor_service_alert_definition_update": ("monitor",),
    "linode_monitor_service_token_create": ("monitor",),
    "linode_monitor_service_alert_definition_delete": ("monitor",),
    "linode_profile_phone_number_delete": ("account",),
    "linode_profile_phone_number_verify": ("account",),
    "linode_profile_security_question_answer": ("account",),
    "linode_profile_app_delete": ("account",),
    "linode_profile_tfa_disable": ("account",),
    "linode_profile_tfa_enable": ("account",),
    "linode_profile_tfa_enable_confirm": ("account",),
}


def _with_fixture_declarations(
    catalog: list[ToolDescriptor],
) -> list[ToolDescriptor]:
    """Fill each descriptor's scopes and categories the way production fills
    them from the generated registry tables, sourced here from the frozen
    fixtures so the derivation tests keep answers independent of those
    tables.
    """
    return [
        dataclasses.replace(
            d,
            scopes=_FIXTURE_SCOPES.get(d.name, ()),
            categories=_FIXTURE_CATEGORIES.get(d.name, ()),
        )
        for d in catalog
    ]


# Tool names the IAM cases below spell more than once.
_IAM_IDP_CONFIG_DELETE = "linode_iam_idp_config_delete"
_IAM_ROLE_PERMISSION_UPDATE = "linode_iam_user_role_permission_update"
_ACCOUNT_USER_GRANTS_GET = "linode_account_user_grants_get"
_ACCOUNT_USER_GRANTS_UPDATE = "linode_account_user_grants_update"
_INSTANCE_CREATE = "linode_instance_create"

_EXPECTED_NAMES = (
    "default",
    "readonly-full",
    "compute-admin",
    "network-admin",
    "kubernetes-admin",
    "storage-admin",
    "iam-admin",
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


def test_linode_kernels_list_requires_no_scope() -> None:
    """Kernels list is a public catalog route and needs no token scope.

    The OpenAPI spec declares no security requirement for GET
    /linode/kernels, so the mapping deliberately returns nothing while
    the tool stays in the compute profile category.
    """
    assert scopes_for("linode_kernel_list") == []
    assert categories_for("linode_kernel_list") == ["compute"]


def test_longview_tools_map_to_one_longview_category() -> None:
    """Longview tools report one Longview category and matching scopes."""
    assert categories_for("linode_longview_client_create") == ["longview"]
    assert categories_for("linode_longview_client_delete") == ["longview"]
    assert scopes_for("linode_longview_client_create") == ["longview:read_write"]


def test_managed_tools_are_account_scoped() -> None:
    """Managed tools belong to the account category and scopes."""
    assert categories_for("linode_managed_contact_create") == ["account"]
    assert scopes_for("linode_managed_contact_create") == ["account:read_write"]
    assert categories_for("linode_managed_credential_create") == ["account"]
    assert scopes_for("linode_managed_credential_create") == ["account:read_write"]
    assert categories_for("linode_managed_credential_revoke") == ["account"]
    assert scopes_for("linode_managed_credential_revoke") == ["account:read_write"]
    assert categories_for("linode_managed_credential_list") == ["account"]
    assert scopes_for("linode_managed_credential_list") == ["account:read_only"]
    assert categories_for("linode_managed_credential_update") == ["account"]
    assert scopes_for("linode_managed_credential_update") == ["account:read_write"]
    assert categories_for("linode_managed_service_update") == ["account"]
    assert scopes_for("linode_managed_service_update") == ["account:read_write"]
    assert categories_for("linode_managed_service_enable") == ["account"]
    assert scopes_for("linode_managed_service_enable") == ["account:read_write"]
    assert categories_for("linode_managed_issue_get") == ["account"]
    assert scopes_for("linode_managed_issue_get") == ["account:read_only"]
    assert categories_for("linode_managed_service_get") == ["account"]
    assert scopes_for("linode_managed_service_get") == ["account:read_only"]
    assert categories_for("linode_managed_credential_username_password_update") == [
        "account"
    ]
    assert scopes_for("linode_managed_credential_username_password_update") == [
        "account:read_write"
    ]
    assert categories_for("linode_managed_service_delete") == ["account"]
    assert scopes_for("linode_managed_service_delete") == ["account:read_write"]


def test_managed_service_disable_is_account_scoped() -> None:
    """Managed service disable belongs to the account category and scope."""
    assert categories_for("linode_managed_service_disable") == ["account"]
    assert scopes_for("linode_managed_service_disable") == ["account:read_write"]


def test_managed_contact_delete_is_account_scoped() -> None:
    """Managed contact delete belongs to the account category and scope."""
    assert categories_for("linode_managed_contact_delete") == ["account"]
    assert scopes_for("linode_managed_contact_delete") == ["account:read_write"]


def test_database_tools_require_database_read_scope() -> None:
    """Managed Database tools require the database token scope."""
    assert scopes_for("linode_database_instance_list") == ["databases:read_only"]
    assert categories_for("linode_database_instance_list") == ["databases"]
    assert scopes_for("linode_database_mysql_config_get") == ["databases:read_only"]
    assert categories_for("linode_database_mysql_config_get") == ["databases"]
    assert scopes_for("linode_database_postgresql_config_get") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_postgresql_config_get") == ["databases"]
    assert scopes_for("linode_database_postgresql_instance_create") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_postgresql_instance_create") == ["databases"]
    assert categories_for("linode_database_type_list") == ["databases"]
    assert scopes_for("linode_database_postgresql_instance_ssl_get") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_postgresql_instance_ssl_get") == [
        "databases"
    ]
    assert scopes_for("linode_database_mysql_instance_create") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_mysql_instance_create") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_credentials_reset") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_mysql_instance_credentials_reset") == [
        "databases"
    ]
    assert scopes_for("linode_database_postgresql_instance_credentials_reset") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_postgresql_instance_credentials_reset") == [
        "databases"
    ]
    assert scopes_for("linode_database_mysql_instance_delete") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_mysql_instance_delete") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_get") == ["databases:read_only"]
    assert categories_for("linode_database_mysql_instance_get") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_ssl_get") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_mysql_instance_ssl_get") == ["databases"]
    # Credential reads are documented as databases:read_only even though
    # the tools register as mutators; the declared scopes mirror the spec.
    assert scopes_for("linode_database_mysql_instance_credentials_get") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_mysql_instance_credentials_get") == [
        "databases"
    ]
    assert scopes_for("linode_database_mysql_instance_resume") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_mysql_instance_resume") == ["databases"]
    assert categories_for("linode_database_postgresql_instance_resume") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_list") == ["databases:read_only"]
    assert categories_for("linode_database_mysql_instance_list") == ["databases"]
    assert scopes_for("linode_database_postgresql_instance_patch") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_postgresql_instance_patch") == ["databases"]
    assert scopes_for("linode_database_postgresql_instance_update") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_postgresql_instance_update") == ["databases"]
    assert scopes_for("linode_database_postgresql_instance_credentials_get") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_postgresql_instance_credentials_get") == [
        "databases"
    ]
    assert scopes_for("linode_database_postgresql_instance_list") == [
        "databases:read_only"
    ]
    assert categories_for("linode_database_postgresql_instance_list") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_patch") == [
        "databases:read_write"
    ]
    assert categories_for("linode_database_mysql_instance_patch") == ["databases"]
    assert categories_for("linode_database_mysql_instance_update") == ["databases"]
    assert scopes_for("linode_database_mysql_instance_update") == [
        "databases:read_write"
    ]


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
    assert categories_for("linode_account_payment_method_delete") == ["account"]


def test_account_service_transfer_delete_is_account_category() -> None:
    """Service-transfer deletion is an account destroy tool."""
    assert categories_for("linode_account_service_transfer_delete") == ["account"]


def test_profile_app_revoke_is_account_category() -> None:
    """OAuth app revoke is an account-profile destroy tool."""
    assert categories_for("linode_profile_app_delete") == ["account"]


def test_profile_phone_number_delete_is_account_category() -> None:
    """Phone-number deletion is an account-profile write tool."""
    assert categories_for("linode_profile_phone_number_delete") == ["account"]


def test_profile_phone_number_verify_is_account_category() -> None:
    """Phone-number verification is an account-profile write tool."""
    assert categories_for("linode_profile_phone_number_verify") == ["account"]


def test_full_access_includes_every_mutator_and_admin() -> None:
    """Full access spans every category and grants Admin (account/child_account
    scope) tools. Only Unknown-capability tools stay excluded."""
    catalog = _synthetic_catalog()
    profiles = builtin_profiles(catalog)

    full_tools = set(profiles["full-access"].allowed_tools)
    mutators = {
        tool.name
        for tool in catalog
        if tool.capability in (Capability.Write, Capability.Destroy, Capability.Admin)
    }
    assert mutators.issubset(full_tools)
    assert "linode_admin_synthetic" in full_tools
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
    the deduplicated, sorted union of the catalog's declared per-tool
    scopes over its allowed_tools.

    Pins the derivation against drift between the scope catalog and the
    blueprint, and catches anyone trying to restore hardcoded scope
    tuples by mistake.
    """
    catalog = _synthetic_catalog()
    built = builtin_profiles(catalog)
    scopes_by_name = {d.name: d.scopes for d in catalog}

    for name, prof in built.items():
        expected: set[str] = set()
        for tool_name in prof.allowed_tools:
            expected.update(scopes_by_name.get(tool_name, ()))
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
    assert categories_for("linode_database_engine_get") == ["databases"]
    assert categories_for("linode_database_type_get") == ["databases"]
    assert categories_for("linode_database_mysql_instance_create") == ["databases"]
    assert categories_for("linode_database_postgresql_instance_create") == ["databases"]


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
        "linodes:read_write",
        "volumes:read_write",
        "databases:read_write",
        "domains:read_write",
        "firewall:read_write",
        "nodebalancers:read_write",
        "lke:read_write",
        "object_storage:read_write",
        "vpc:read_write",
        "images:read_only",
    }
    for scope in want_present:
        assert scope in full, f"full-access should include {scope}"

    assert "stackscripts:read_write" not in full, (
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


def test_iam_tools_map_to_the_iam_category() -> None:
    """IAM tools carry the iam category, mirroring Go's Categories()."""
    assert categories_for(_IAM_IDP_CONFIG_DELETE) == ["iam"]
    assert categories_for("linode_iam_delegation_child_account_list") == ["iam"]
    assert categories_for("linode_iam_role_permission_list") == ["iam"]


def test_iam_removal_resolves_in_the_wildcard_profiles_and_iam_admin() -> None:
    """An IAM destroy is served by the wildcards and iam-admin, nowhere else.

    The category is what lets iam-admin reach it: the wildcards short-circuit
    whatever a tool's categories are, so dropping the category would leave this
    destroy wildcard-only while every gate stayed green.
    """
    tool_name = _IAM_IDP_CONFIG_DELETE
    profiles = builtin_profiles(
        [ToolDescriptor(tool_name, Capability.Destroy, categories=("iam",))]
    )

    serving = {name for name, p in profiles.items() if tool_name in p.allowed_tools}
    assert serving == {"full-access", "emergency", "iam-admin"}


def _iam_admin_catalog() -> list[ToolDescriptor]:
    """The fixture the iam-admin cases resolve against.

    Holds the nine IAM mutators and the two IAM removals beside the entity
    list, the legacy account grants pair, and one compute write, so a profile
    reaching past its own category fails by name rather than by a count.
    """
    iam_writes = (
        _IAM_ROLE_PERMISSION_UPDATE,
        "linode_iam_delegation_child_account_user_update",
        "linode_iam_delegation_default_role_permission_update",
        "linode_iam_delegation_profile_child_account_token_create",
        "linode_iam_idp_config_create",
        "linode_iam_idp_config_update",
        "linode_iam_idp_config_certificate_create",
        "linode_iam_idp_config_excluded_user_update",
        "linode_iam_idp_config_included_user_update",
    )
    catalog = [
        ToolDescriptor(name, Capability.Write, categories=("iam",))
        for name in iam_writes
    ]
    catalog.extend(
        [
            ToolDescriptor(
                _IAM_IDP_CONFIG_DELETE, Capability.Destroy, categories=("iam",)
            ),
            ToolDescriptor(
                "linode_iam_idp_config_certificate_delete",
                Capability.Destroy,
                categories=("iam",),
            ),
            ToolDescriptor("linode_entity_list", Capability.Read, categories=("iam",)),
            ToolDescriptor(
                _ACCOUNT_USER_GRANTS_GET, Capability.Read, categories=("account",)
            ),
            ToolDescriptor(
                _ACCOUNT_USER_GRANTS_UPDATE, Capability.Admin, categories=("account",)
            ),
            ToolDescriptor(_INSTANCE_CREATE, Capability.Write, categories=("compute",)),
        ]
    )
    return catalog


def test_iam_admin_serves_the_whole_iam_surface() -> None:
    """iam-admin runs Linode IAM: every IAM mutator, removal, and the entity list."""
    catalog = _iam_admin_catalog()
    allowed = set(builtin_profiles(catalog)["iam-admin"].allowed_tools)

    iam_tools = {tool.name for tool in catalog if "iam" in tool.categories}
    assert iam_tools <= allowed
    assert "linode_entity_list" in allowed


def test_iam_admin_excludes_the_legacy_grants_mutator() -> None:
    """The legacy account grants mutator stays out of iam-admin.

    Linode documents mixing grant-based and role-based access control on one
    account as a security risk, and the grants routes are deprecated, so the
    profile that manages IAM must not hand over the older system too. The tier
    is not doing the work alone: the tool is Admin AND filed under account, so
    a retag to Write would still leave it out of an iam-only profile.

    Its paired read is a Read, which every built-in serves down to the
    read-only default. Pinned here so retagging it into a mutating tier fails
    rather than quietly widening what iam-admin can do.
    """
    profiles = builtin_profiles(_iam_admin_catalog())
    allowed = set(profiles["iam-admin"].allowed_tools)

    assert _ACCOUNT_USER_GRANTS_UPDATE not in allowed
    assert _ACCOUNT_USER_GRANTS_GET in allowed
    assert _ACCOUNT_USER_GRANTS_GET in set(profiles["default"].allowed_tools)


def test_iam_admin_is_scoped_to_the_iam_category() -> None:
    """No write outside iam reaches iam-admin, and no IAM write reaches a sibling."""
    profiles = builtin_profiles(_iam_admin_catalog())

    assert _INSTANCE_CREATE not in set(profiles["iam-admin"].allowed_tools)

    siblings = (
        "default",
        "readonly-full",
        "compute-admin",
        "network-admin",
        "kubernetes-admin",
        "storage-admin",
    )
    for name in siblings:
        allowed = set(profiles[name].allowed_tools)
        assert _IAM_ROLE_PERMISSION_UPDATE not in allowed, name


def test_entity_list_read_served_by_every_builtin() -> None:
    """A Read in the iam category is answered by every built-in, default included."""
    catalog = [
        ToolDescriptor("linode_entity_list", Capability.Read, categories=("iam",))
    ]

    for name, profile in builtin_profiles(catalog).items():
        assert "linode_entity_list" in profile.allowed_tools, name


def test_entity_list_maps_to_the_iam_category() -> None:
    """GET /entities is documented under Linode IAM, so it files with its siblings."""
    assert categories_for("linode_entity_list") == ["iam"]


def test_network_admin_serves_reserved_ip_writes() -> None:
    """Reserved IPs are networking surface, so network-admin serves them.

    Go filed them under no category at all, which made network-admin two
    different profiles depending on which client the caller ran.
    """
    catalog = [
        ToolDescriptor(
            "linode_networking_reserved_ip_create",
            Capability.Write,
            categories=("networking",),
        ),
        ToolDescriptor(
            "linode_networking_reserved_ip_update",
            Capability.Write,
            categories=("networking",),
        ),
        ToolDescriptor(
            "linode_networking_reserved_ip_delete",
            Capability.Destroy,
            categories=("networking",),
        ),
    ]
    allowed = set(builtin_profiles(catalog)["network-admin"].allowed_tools)

    assert allowed == {tool.name for tool in catalog}


def test_storage_admin_serves_instance_backup_switches() -> None:
    """Enabling and canceling backups is the surface storage-admin exists for."""
    catalog = [
        ToolDescriptor(
            "linode_instance_backups_enable",
            Capability.Write,
            categories=("compute_deep", "compute"),
        ),
        ToolDescriptor(
            "linode_instance_backups_cancel",
            Capability.Destroy,
            categories=("compute_deep", "compute"),
        ),
    ]
    allowed = set(builtin_profiles(catalog)["storage-admin"].allowed_tools)

    assert allowed == {tool.name for tool in catalog}


def test_category_less_mutator_resolves_only_in_wildcard_profiles() -> None:
    """A mutator in no category reaches only the profiles elevating every one.

    Before the wildcard marker landed here it reached nothing in Python while
    Go's short-circuit served it, so the same call succeeded in one language
    and failed as an unknown tool in the other.
    """
    tool_name = "linode_unmapped_thing_create"
    profiles = builtin_profiles([ToolDescriptor(tool_name, Capability.Write)])

    serving = {name for name, p in profiles.items() if tool_name in p.allowed_tools}
    assert serving == {"full-access", "emergency"}


def test_multi_category_tool_is_served_from_either_of_its_categories() -> None:
    """Every category a tool falls in counts, not just the first one listed."""
    tool_name = "linode_instance_backup_create"
    assert categories_for(tool_name) == ["compute_deep", "compute"]

    profiles = builtin_profiles(
        [
            ToolDescriptor(
                tool_name,
                Capability.Write,
                categories=("compute_deep", "compute"),
            )
        ]
    )
    for name in ("storage-admin", "compute-admin"):
        assert tool_name in profiles[name].allowed_tools


def test_account_gated_tools_are_in_the_account_category() -> None:
    """What the API gates on account:* is what an account-admin would elevate.

    Core is the four tools a session starts from; naming the rest core left
    them in a bucket no profile can lift.
    """
    for tool_name in (
        "linode_account_payment_create",
        "linode_beta_list",
        "linode_lock_create",
        "linode_maintenance_policy_list",
        "linode_managed_service_create",
        "linode_profile_grants_get",
        "linode_profile_login_get",
        "linode_profile_update",
        "linode_support_ticket_get",
        "linode_tag_object_list",
    ):
        assert categories_for(tool_name) == ["account"], tool_name

    for tool_name in ("hello", "version", "linode_profile_get", "linode_account_get"):
        assert categories_for(tool_name) == ["core"], tool_name
