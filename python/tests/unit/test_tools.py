"""Unit tests for MCP tools."""

import json
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from mcp.types import TextContent

from linodemcp.config import Config
from linodemcp.linode import (
    UDF,
    Account,
    Addons,
    Alerts,
    Backups,
    BackupsAddon,
    Domain,
    DomainRecord,
    Firewall,
    FirewallAddresses,
    FirewallRule,
    FirewallRules,
    FirewallTemplate,
    Image,
    Instance,
    InstanceType,
    NodeBalancer,
    Price,
    Profile,
    Region,
    Resolver,
    Schedule,
    Specs,
    SSHKey,
    StackScript,
    Transfer,
    Volume,
)
from linodemcp.profiles import Capability
from linodemcp.tools import (
    create_linode_account_agreements_acknowledge_tool,
    create_linode_account_availability_list_tool,
    create_linode_account_beta_enroll_tool,
    create_linode_account_beta_get_tool,
    create_linode_account_event_get_tool,
    create_linode_account_invoice_items_list_tool,
    create_linode_account_maintenance_list_tool,
    create_linode_account_oauth_client_get_tool,
    create_linode_account_oauth_client_thumbnail_get_tool,
    create_linode_account_payment_method_delete_tool,
    create_linode_account_payment_method_get_tool,
    create_linode_account_settings_get_tool,
    create_linode_account_support_ticket_attachment_create_tool,
    create_linode_account_support_ticket_close_tool,
    create_linode_account_support_ticket_create_tool,
    create_linode_account_support_ticket_get_tool,
    create_linode_account_support_ticket_replies_list_tool,
    create_linode_account_support_ticket_reply_create_tool,
    create_linode_account_support_tickets_list_tool,
    create_linode_account_tag_create_tool,
    create_linode_account_tag_delete_tool,
    create_linode_account_tag_objects_list_tool,
    create_linode_account_tags_list_tool,
    create_linode_account_update_tool,
    create_linode_firewall_get_tool,
    create_linode_firewall_rules_get_tool,
    create_linode_firewall_settings_update_tool,
    create_linode_firewall_template_get_tool,
    create_linode_image_create_tool,
    create_linode_image_get_tool,
    create_linode_image_update_tool,
    create_linode_image_upload_tool,
    create_linode_images_sharegroups_token_create_tool,
    create_linode_images_sharegroups_token_update_tool,
    create_linode_instance_backup_create_tool,
    create_linode_instance_backup_get_tool,
    create_linode_instance_backup_restore_tool,
    create_linode_instance_backups_cancel_tool,
    create_linode_instance_backups_enable_tool,
    create_linode_instance_backups_list_tool,
    create_linode_instance_clone_tool,
    create_linode_instance_config_create_tool,
    create_linode_instance_config_delete_tool,
    create_linode_instance_config_get_tool,
    create_linode_instance_config_interface_get_tool,
    create_linode_instance_config_interfaces_list_tool,
    create_linode_instance_configs_list_tool,
    create_linode_instance_disk_clone_tool,
    create_linode_instance_disk_create_tool,
    create_linode_instance_disk_delete_tool,
    create_linode_instance_disk_get_tool,
    create_linode_instance_disk_password_reset_tool,
    create_linode_instance_disk_resize_tool,
    create_linode_instance_disk_update_tool,
    create_linode_instance_disks_list_tool,
    create_linode_instance_firewalls_apply_tool,
    create_linode_instance_firewalls_list_tool,
    create_linode_instance_firewalls_update_tool,
    create_linode_instance_interface_firewalls_list_tool,
    create_linode_instance_ip_allocate_tool,
    create_linode_instance_ip_delete_tool,
    create_linode_instance_ip_get_tool,
    create_linode_instance_ip_update_tool,
    create_linode_instance_ips_list_tool,
    create_linode_instance_migrate_tool,
    create_linode_instance_mutate_tool,
    create_linode_instance_password_reset_tool,
    create_linode_instance_rebuild_tool,
    create_linode_instance_rescue_tool,
    create_linode_instance_stats_tool,
    create_linode_instance_update_tool,
    create_linode_instance_upgrade_interfaces_tool,
    create_linode_instance_volumes_list_tool,
    create_linode_ipv6_range_create_tool,
    create_linode_ipv6_range_delete_tool,
    create_linode_ipv6_range_get_tool,
    create_linode_kernel_get_tool,
    create_linode_kernels_list_tool,
    create_linode_lke_cluster_create_tool,
    create_linode_lke_cluster_delete_tool,
    create_linode_lke_cluster_get_tool,
    create_linode_lke_clusters_list_tool,
    create_linode_maintenance_policies_list_tool,
    create_linode_managed_contact_delete_tool,
    create_linode_managed_contact_get_tool,
    create_linode_managed_contacts_list_tool,
    create_linode_managed_credential_get_tool,
    create_linode_managed_credential_revoke_tool,
    create_linode_managed_credential_update_tool,
    create_linode_managed_credential_username_password_update_tool,
    create_linode_managed_credentials_list_tool,
    create_linode_managed_issue_get_tool,
    create_linode_managed_issues_list_tool,
    create_linode_managed_linode_settings_list_tool,
    create_linode_managed_service_disable_tool,
    create_linode_managed_service_get_tool,
    create_linode_managed_ssh_key_get_tool,
    create_linode_managed_stats_tool,
    create_linode_monitor_service_alert_definition_get_tool,
    create_linode_monitor_service_get_tool,
    create_linode_monitor_service_token_create_tool,
    create_linode_monitor_services_list_tool,
    create_linode_nodebalancer_config_create_tool,
    create_linode_nodebalancer_config_delete_tool,
    create_linode_nodebalancer_config_get_tool,
    create_linode_nodebalancer_config_node_create_tool,
    create_linode_nodebalancer_config_node_delete_tool,
    create_linode_nodebalancer_config_node_get_tool,
    create_linode_nodebalancer_config_node_update_tool,
    create_linode_nodebalancer_config_rebuild_tool,
    create_linode_nodebalancer_config_update_tool,
    create_linode_nodebalancer_configs_list_tool,
    create_linode_nodebalancer_firewalls_list_tool,
    create_linode_nodebalancer_firewalls_update_tool,
    create_linode_nodebalancer_stats_tool,
    create_linode_nodebalancer_vpc_config_get_tool,
    create_linode_nodebalancer_vpc_configs_list_tool,
    create_linode_object_storage_cancel_tool,
    create_linode_object_storage_endpoints_list_tool,
    create_linode_object_storage_quota_get_tool,
    create_linode_object_storage_quota_usage_tool,
    create_linode_object_storage_quotas_list_tool,
    create_linode_placement_group_assign_tool,
    create_linode_placement_group_create_tool,
    create_linode_placement_group_delete_tool,
    create_linode_placement_group_get_tool,
    create_linode_placement_group_unassign_tool,
    create_linode_placement_group_update_tool,
    create_linode_placement_groups_list_tool,
    create_linode_profile_app_get_tool,
    create_linode_profile_app_revoke_tool,
    create_linode_profile_apps_list_tool,
    create_linode_profile_device_get_tool,
    create_linode_profile_device_revoke_tool,
    create_linode_profile_devices_list_tool,
    create_linode_profile_login_get_tool,
    create_linode_profile_logins_list_tool,
    create_linode_profile_phone_number_delete_tool,
    create_linode_profile_phone_number_send_tool,
    create_linode_profile_phone_number_verify_tool,
    create_linode_profile_preferences_get_tool,
    create_linode_profile_preferences_update_tool,
    create_linode_profile_security_questions_answer_tool,
    create_linode_profile_security_questions_list_tool,
    create_linode_profile_tfa_disable_tool,
    create_linode_profile_tfa_enable_confirm_tool,
    create_linode_profile_tfa_enable_tool,
    create_linode_profile_token_create_tool,
    create_linode_profile_token_get_tool,
    create_linode_profile_token_revoke_tool,
    create_linode_profile_token_update_tool,
    create_linode_profile_tokens_list_tool,
    create_linode_regions_availability_get_tool,
    create_linode_regions_availability_list_tool,
    create_linode_regions_get_tool,
    create_linode_stackscript_create_tool,
    create_linode_stackscript_delete_tool,
    create_linode_vlan_delete_tool,
    create_linode_vlans_list_tool,
    create_linode_vpc_create_tool,
    create_linode_vpc_delete_tool,
    create_linode_vpc_get_tool,
    create_linode_vpc_subnet_create_tool,
    create_linode_vpc_subnet_delete_tool,
    create_linode_vpcs_list_tool,
    handle_hello,
    handle_linode_account,
    handle_linode_account_agreements_acknowledge,
    handle_linode_account_availability_list,
    handle_linode_account_beta_enroll,
    handle_linode_account_beta_get,
    handle_linode_account_event_get,
    handle_linode_account_invoice_items_list,
    handle_linode_account_maintenance_list,
    handle_linode_account_oauth_client_get,
    handle_linode_account_oauth_client_thumbnail_get,
    handle_linode_account_payment_method_delete,
    handle_linode_account_payment_method_get,
    handle_linode_account_settings_get,
    handle_linode_account_support_ticket_attachment_create,
    handle_linode_account_support_ticket_close,
    handle_linode_account_support_ticket_create,
    handle_linode_account_support_ticket_get,
    handle_linode_account_support_ticket_replies_list,
    handle_linode_account_support_ticket_reply_create,
    handle_linode_account_support_tickets_list,
    handle_linode_account_tag_create,
    handle_linode_account_tag_delete,
    handle_linode_account_tag_objects_list,
    handle_linode_account_tags_list,
    handle_linode_account_update,
    handle_linode_domain_clone,
    handle_linode_domain_create,
    handle_linode_domain_delete,
    handle_linode_domain_get,
    handle_linode_domain_record_create,
    handle_linode_domain_record_delete,
    handle_linode_domain_record_get,
    handle_linode_domain_record_update,
    handle_linode_domain_records_list,
    handle_linode_domain_update,
    handle_linode_domains_list,
    handle_linode_firewall_create,
    handle_linode_firewall_delete,
    handle_linode_firewall_get,
    handle_linode_firewall_rules_get,
    handle_linode_firewall_rules_update,
    handle_linode_firewall_settings_update,
    handle_linode_firewall_template_get,
    handle_linode_firewall_update,
    handle_linode_firewalls_list,
    handle_linode_image_create,
    handle_linode_image_get,
    handle_linode_image_update,
    handle_linode_image_upload,
    handle_linode_images_list,
    handle_linode_images_sharegroups_token_create,
    handle_linode_images_sharegroups_token_update,
    handle_linode_instance_backup_create,
    handle_linode_instance_backup_get,
    handle_linode_instance_backup_restore,
    handle_linode_instance_backups_cancel,
    handle_linode_instance_backups_enable,
    handle_linode_instance_backups_list,
    handle_linode_instance_boot,
    handle_linode_instance_clone,
    handle_linode_instance_config_create,
    handle_linode_instance_config_delete,
    handle_linode_instance_config_get,
    handle_linode_instance_config_interface_get,
    handle_linode_instance_config_interfaces_list,
    handle_linode_instance_configs_list,
    handle_linode_instance_create,
    handle_linode_instance_delete,
    handle_linode_instance_disk_clone,
    handle_linode_instance_disk_create,
    handle_linode_instance_disk_delete,
    handle_linode_instance_disk_get,
    handle_linode_instance_disk_password_reset,
    handle_linode_instance_disk_resize,
    handle_linode_instance_disk_update,
    handle_linode_instance_disks_list,
    handle_linode_instance_firewalls_apply,
    handle_linode_instance_firewalls_list,
    handle_linode_instance_firewalls_update,
    handle_linode_instance_get,
    handle_linode_instance_interface_firewalls_list,
    handle_linode_instance_ip_allocate,
    handle_linode_instance_ip_delete,
    handle_linode_instance_ip_get,
    handle_linode_instance_ip_update,
    handle_linode_instance_ips_list,
    handle_linode_instance_migrate,
    handle_linode_instance_mutate,
    handle_linode_instance_password_reset,
    handle_linode_instance_reboot,
    handle_linode_instance_rebuild,
    handle_linode_instance_rescue,
    handle_linode_instance_resize,
    handle_linode_instance_shutdown,
    handle_linode_instance_stats,
    handle_linode_instance_update,
    handle_linode_instance_upgrade_interfaces,
    handle_linode_instance_volumes_list,
    handle_linode_instances_list,
    handle_linode_ipv6_range_create,
    handle_linode_ipv6_range_delete,
    handle_linode_ipv6_range_get,
    handle_linode_kernel_get,
    handle_linode_kernels_list,
    handle_linode_lke_acl_delete,
    handle_linode_lke_acl_get,
    handle_linode_lke_acl_update,
    handle_linode_lke_api_endpoints_list,
    handle_linode_lke_cluster_create,
    handle_linode_lke_cluster_delete,
    handle_linode_lke_cluster_get,
    handle_linode_lke_cluster_recycle,
    handle_linode_lke_cluster_regenerate,
    handle_linode_lke_cluster_update,
    handle_linode_lke_clusters_list,
    handle_linode_lke_dashboard_get,
    handle_linode_lke_kubeconfig_delete,
    handle_linode_lke_kubeconfig_get,
    handle_linode_lke_node_delete,
    handle_linode_lke_node_get,
    handle_linode_lke_node_recycle,
    handle_linode_lke_pool_create,
    handle_linode_lke_pool_delete,
    handle_linode_lke_pool_get,
    handle_linode_lke_pool_recycle,
    handle_linode_lke_pool_update,
    handle_linode_lke_pools_list,
    handle_linode_lke_service_token_delete,
    handle_linode_lke_tier_versions_list,
    handle_linode_lke_types_list,
    handle_linode_lke_version_get,
    handle_linode_lke_versions_list,
    handle_linode_maintenance_policies_list,
    handle_linode_managed_contact_delete,
    handle_linode_managed_contact_get,
    handle_linode_managed_contacts_list,
    handle_linode_managed_credential_get,
    handle_linode_managed_credential_revoke,
    handle_linode_managed_credential_update,
    handle_linode_managed_credential_username_password_update,
    handle_linode_managed_credentials_list,
    handle_linode_managed_issue_get,
    handle_linode_managed_issues_list,
    handle_linode_managed_linode_settings_list,
    handle_linode_managed_service_disable,
    handle_linode_managed_service_get,
    handle_linode_managed_ssh_key_get,
    handle_linode_managed_stats,
    handle_linode_monitor_service_alert_definition_get,
    handle_linode_monitor_service_get,
    handle_linode_monitor_service_token_create,
    handle_linode_monitor_services_list,
    handle_linode_nodebalancer_config_create,
    handle_linode_nodebalancer_config_delete,
    handle_linode_nodebalancer_config_get,
    handle_linode_nodebalancer_config_node_create,
    handle_linode_nodebalancer_config_node_delete,
    handle_linode_nodebalancer_config_node_get,
    handle_linode_nodebalancer_config_node_update,
    handle_linode_nodebalancer_config_nodes_list,
    handle_linode_nodebalancer_config_rebuild,
    handle_linode_nodebalancer_config_update,
    handle_linode_nodebalancer_configs_list,
    handle_linode_nodebalancer_create,
    handle_linode_nodebalancer_delete,
    handle_linode_nodebalancer_firewalls_list,
    handle_linode_nodebalancer_firewalls_update,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancer_stats,
    handle_linode_nodebalancer_update,
    handle_linode_nodebalancer_vpc_config_get,
    handle_linode_nodebalancer_vpc_configs_list,
    handle_linode_nodebalancers_list,
    handle_linode_object_storage_bucket_access_allow,
    handle_linode_object_storage_bucket_access_get,
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_bucket_contents,
    handle_linode_object_storage_bucket_create,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_bucket_get,
    handle_linode_object_storage_buckets_list,
    handle_linode_object_storage_buckets_region_list,
    handle_linode_object_storage_cancel,
    handle_linode_object_storage_endpoints_list,
    handle_linode_object_storage_key_create,
    handle_linode_object_storage_key_delete,
    handle_linode_object_storage_key_get,
    handle_linode_object_storage_key_update,
    handle_linode_object_storage_keys_list,
    handle_linode_object_storage_object_acl_get,
    handle_linode_object_storage_object_acl_update,
    handle_linode_object_storage_presigned_url,
    handle_linode_object_storage_quota_get,
    handle_linode_object_storage_quota_usage,
    handle_linode_object_storage_quotas_list,
    handle_linode_object_storage_ssl_delete,
    handle_linode_object_storage_ssl_get,
    handle_linode_object_storage_ssl_upload,
    handle_linode_object_storage_transfer,
    handle_linode_object_storage_types_list,
    handle_linode_placement_group_assign,
    handle_linode_placement_group_create,
    handle_linode_placement_group_delete,
    handle_linode_placement_group_get,
    handle_linode_placement_group_unassign,
    handle_linode_placement_group_update,
    handle_linode_placement_groups_list,
    handle_linode_profile,
    handle_linode_profile_app_get,
    handle_linode_profile_app_revoke,
    handle_linode_profile_apps_list,
    handle_linode_profile_device_get,
    handle_linode_profile_device_revoke,
    handle_linode_profile_devices_list,
    handle_linode_profile_login_get,
    handle_linode_profile_logins_list,
    handle_linode_profile_phone_number_delete,
    handle_linode_profile_phone_number_send,
    handle_linode_profile_phone_number_verify,
    handle_linode_profile_preferences_get,
    handle_linode_profile_preferences_update,
    handle_linode_profile_security_questions_answer,
    handle_linode_profile_security_questions_list,
    handle_linode_profile_tfa_disable,
    handle_linode_profile_tfa_enable,
    handle_linode_profile_tfa_enable_confirm,
    handle_linode_profile_token_create,
    handle_linode_profile_token_get,
    handle_linode_profile_token_revoke,
    handle_linode_profile_token_update,
    handle_linode_profile_tokens_list,
    handle_linode_regions_availability_get,
    handle_linode_regions_availability_list,
    handle_linode_regions_get,
    handle_linode_regions_list,
    handle_linode_sshkey_create,
    handle_linode_sshkey_delete,
    handle_linode_sshkey_get,
    handle_linode_sshkey_update,
    handle_linode_sshkeys_list,
    handle_linode_stackscript_create,
    handle_linode_stackscript_delete,
    handle_linode_stackscripts_list,
    handle_linode_type_get,
    handle_linode_types_list,
    handle_linode_vlan_delete,
    handle_linode_vlans_list,
    handle_linode_volume_attach,
    handle_linode_volume_clone,
    handle_linode_volume_create,
    handle_linode_volume_delete,
    handle_linode_volume_detach,
    handle_linode_volume_get,
    handle_linode_volume_resize,
    handle_linode_volume_types_list,
    handle_linode_volume_update,
    handle_linode_volumes_list,
    handle_linode_vpc_create,
    handle_linode_vpc_delete,
    handle_linode_vpc_get,
    handle_linode_vpc_ip_list,
    handle_linode_vpc_ips_list,
    handle_linode_vpc_subnet_create,
    handle_linode_vpc_subnet_delete,
    handle_linode_vpc_subnet_get,
    handle_linode_vpc_subnet_update,
    handle_linode_vpc_subnets_list,
    handle_linode_vpc_update,
    handle_linode_vpcs_list,
    handle_version,
)


async def test_handle_hello_with_name() -> None:
    """Test hello tool with name parameter."""
    result = await handle_hello({"name": "Alice"})
    assert len(result) == 1
    assert "Hello, Alice!" in result[0].text
    assert "LinodeMCP server is running" in result[0].text


async def test_handle_hello_without_name() -> None:
    """Test hello tool without name parameter."""
    result = await handle_hello({})
    assert len(result) == 1
    assert "Hello, World!" in result[0].text


async def test_handle_version() -> None:
    """Test version tool."""
    result = await handle_version({})
    assert len(result) == 1
    assert "version" in result[0].text.lower()
    assert "0.1.0" in result[0].text


async def test_handle_linode_profile(
    sample_config: Config, sample_profile_data: dict[str, Any]
) -> None:
    """Test linode_profile tool."""
    mock_profile = Profile(
        username=sample_profile_data["username"],
        email=sample_profile_data["email"],
        timezone=sample_profile_data["timezone"],
        email_notifications=sample_profile_data["email_notifications"],
        restricted=sample_profile_data["restricted"],
        two_factor_auth=sample_profile_data["two_factor_auth"],
        uid=sample_profile_data["uid"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile({}, sample_config)

        assert len(result) == 1
        assert "testuser" in result[0].text
        assert "test@example.com" in result[0].text


async def test_handle_linode_profile_with_environment(sample_config: Config) -> None:
    """Test linode_profile tool with environment parameter."""
    mock_profile = Profile(
        username="envuser",
        email="env@example.com",
        timezone="UTC",
        email_notifications=True,
        restricted=False,
        two_factor_auth=False,
        uid=99999,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile({"environment": "default"}, sample_config)

        assert len(result) == 1
        assert "envuser" in result[0].text


async def test_handle_linode_profile_missing_environment(sample_config: Config) -> None:
    """Test linode_profile tool with missing environment."""
    result = await handle_linode_profile({"environment": "nonexistent"}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "error" in result[0].text


def test_create_linode_profile_preferences_get_tool() -> None:
    """Profile preferences get tool exposes read-only schema."""
    tool, capability = create_linode_profile_preferences_get_tool()

    assert tool.name == "linode_profile_preferences_get"
    assert capability == Capability.Read
    assert "required" not in tool.inputSchema


def test_create_linode_account_payment_method_delete_tool() -> None:
    """Account payment method delete tool exposes confirm-gated schema."""
    tool, capability = create_linode_account_payment_method_delete_tool()

    assert tool.name == "linode_account_payment_method_delete"
    assert capability == Capability.Destroy
    assert tool.inputSchema["required"] == ["payment_method_id", "confirm"]
    assert tool.inputSchema["properties"]["payment_method_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_account_payment_method_delete_success(
    sample_config: Config,
) -> None:
    """Handler deletes a payment method with confirm=true."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_account_payment_method.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_delete(
            {"payment_method_id": 123, "confirm": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Payment method deleted successfully"
    assert payload["result"] == {}
    mock_client.delete_account_payment_method.assert_awaited_once_with(123)


async def test_handle_linode_account_payment_method_delete_dry_run(
    sample_config: Config,
) -> None:
    """Dry-run previews the DELETE route without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_payment_method_delete(
            {"payment_method_id": 456, "confirm": False, "dry_run": True},
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"]["method"] == "DELETE"
    assert payload["would_execute"]["path"] == "/account/payment-methods/456"
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_account_payment_method_delete_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Missing or non-true confirm values are rejected before client calls."""
    arguments: dict[str, object] = {"payment_method_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_payment_method_delete(
            arguments, sample_config
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "payment_method_id", [None, 0, -1, True, "1", "1/2", "1?x", ".."]
)
async def test_handle_linode_account_payment_method_delete_validates_id(
    sample_config: Config, payment_method_id: object
) -> None:
    """Malformed payment method IDs are rejected before client calls."""
    arguments: dict[str, object] = {"confirm": True}
    if payment_method_id is not None:
        arguments["payment_method_id"] = payment_method_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_payment_method_delete(
            arguments, sample_config
        )

    assert "payment_method_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_preferences_get_success(
    sample_config: Config,
) -> None:
    """Handler gets profile preferences."""
    preferences = {"dashboard": {"theme": "dark"}, "dismissed": ["welcome"]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_preferences.return_value = preferences
        mock_client.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_preferences_get({}, sample_config)

    assert len(result) == 1
    assert "dashboard" in result[0].text
    mock_client.get_profile_preferences.assert_awaited_once_with()


async def test_handle_linode_profile_preferences_get_error(
    sample_config: Config,
) -> None:
    """Handler surfaces client errors for profile preferences reads."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_preferences.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_preferences_get({}, sample_config)

    assert len(result) == 1
    assert "Failed to retrieve Linode profile preferences" in result[0].text


def test_create_linode_profile_preferences_update_tool() -> None:
    """Profile preferences update tool exposes confirm-gated schema."""
    tool, capability = create_linode_profile_preferences_update_tool()

    assert tool.name == "linode_profile_preferences_update"
    assert capability == Capability.Write
    assert tool.inputSchema["required"] == ["preferences", "confirm"]
    assert tool.inputSchema["properties"]["preferences"]["type"] == "object"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_preferences_update_success(
    sample_config: Config,
) -> None:
    """Handler updates profile preferences with confirm=true."""
    preferences = {"dashboard": {"theme": "dark"}, "dismissed": ["welcome"]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_profile_preferences.return_value = preferences
        mock_client.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_preferences_update(
            {"preferences": preferences, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "dashboard" in result[0].text
    mock_client.update_profile_preferences.assert_awaited_once_with(preferences)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_profile_preferences_update_requires_boolean_confirm(
    sample_config: Config, confirm: Any
) -> None:
    """Profile preferences update rejects missing or non-true confirm."""
    arguments: dict[str, Any] = {"preferences": {"theme": "dark"}}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_preferences_update(
            arguments, sample_config
        )

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("preferences", [None, [], "theme", 1, True])
async def test_handle_linode_profile_preferences_update_requires_object(
    sample_config: Config, preferences: Any
) -> None:
    """Profile preferences update validates preferences before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_preferences_update(
            {"preferences": preferences, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "preferences must be an object" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_preferences_update_error(
    sample_config: Config,
) -> None:
    """Handler surfaces client errors for profile preferences updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_profile_preferences.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_preferences_update(
            {"preferences": {}, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to update Linode profile preferences" in result[0].text


async def test_linode_instance_config_delete_tool_definition() -> None:
    """Test linode_instance_config_delete tool definition."""
    tool, capability = create_linode_instance_config_delete_tool()

    assert tool.name == "linode_instance_config_delete"
    assert capability == Capability.Destroy
    assert tool.inputSchema["required"] == ["linode_id", "config_id", "confirm"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_instance_config_delete(sample_config: Config) -> None:
    """Test linode_instance_config_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_instance_config.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_delete(
            {"linode_id": 123, "config_id": 6, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "deleted" in result[0].text
    assert "123" in result[0].text
    assert "6" in result[0].text
    mock_client.delete_instance_config.assert_called_once_with(123, 6)


@pytest.mark.parametrize(
    "confirm_value",
    [None, False, "true", 1, 0],
)
async def test_handle_linode_instance_config_delete_requires_boolean_confirm(
    confirm_value: Any, sample_config: Config
) -> None:
    """linode_instance_config_delete rejects missing or non-true confirm."""
    arguments: dict[str, Any] = {"linode_id": 123, "config_id": 6}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_config_delete(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_config_delete_dry_run(
    sample_config: Config,
) -> None:
    """dry_run previews config deletion without calling delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_config.return_value = {"id": 6, "label": "boot"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_delete(
            {
                "linode_id": 123,
                "config_id": 6,
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_config_delete"
    assert body["would_execute"] == {
        "method": "DELETE",
        "path": "/linode/instances/123/configs/6",
    }
    assert body["current_state"] == {"id": 6, "label": "boot"}
    mock_client.get_instance_config.assert_called_once_with(123, 6)
    mock_client.delete_instance_config.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0, "config_id": 6, "confirm": True},
        {"linode_id": -1, "config_id": 6, "confirm": True},
        {"linode_id": True, "config_id": 6, "confirm": True},
        {"linode_id": "123", "config_id": 6, "confirm": True},
        {"linode_id": "1/2", "config_id": 6, "confirm": True},
        {"linode_id": "1?x", "config_id": 6, "confirm": True},
        {"linode_id": "..", "config_id": 6, "confirm": True},
        {"linode_id": 123, "confirm": True},
        {"linode_id": 123, "config_id": 0, "confirm": True},
        {"linode_id": 123, "config_id": -1, "confirm": True},
        {"linode_id": 123, "config_id": True, "confirm": True},
        {"linode_id": 123, "config_id": "6", "confirm": True},
        {"linode_id": 123, "config_id": "1/2", "confirm": True},
        {"linode_id": 123, "config_id": "1?x", "confirm": True},
        {"linode_id": 123, "config_id": "..", "confirm": True},
    ],
)
async def test_handle_linode_instance_config_delete_invalid_ids(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_config_delete rejects malformed path parameters."""
    result = await handle_linode_instance_config_delete(arguments, sample_config)

    assert len(result) == 1
    assert "positive integer" in result[0].text or "confirm=true" in result[0].text


async def test_handle_linode_instance_config_delete_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_delete error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_instance_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_delete(
            {"linode_id": 123, "config_id": 6, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to delete" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_config_get_tool_definition() -> None:
    """Test linode_instance_config_get tool definition."""
    tool, capability = create_linode_instance_config_get_tool()

    assert tool.name == "linode_instance_config_get"
    assert capability == Capability.Read
    assert tool.inputSchema["required"] == ["linode_id", "config_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1


async def test_handle_linode_instance_config_get(sample_config: Config) -> None:
    """Test linode_instance_config_get tool."""
    mock_config = {"id": 6, "label": "boot-config"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_config.return_value = mock_config
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_get(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "boot-config" in result[0].text
    mock_client.get_instance_config.assert_called_once_with(123, 6)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0, "config_id": 6},
        {"linode_id": -1, "config_id": 6},
        {"linode_id": True, "config_id": 6},
        {"linode_id": "123", "config_id": 6},
        {"linode_id": "1/2", "config_id": 6},
        {"linode_id": "1?x", "config_id": 6},
        {"linode_id": "..", "config_id": 6},
        {"linode_id": 123},
        {"linode_id": 123, "config_id": 0},
        {"linode_id": 123, "config_id": -1},
        {"linode_id": 123, "config_id": True},
        {"linode_id": 123, "config_id": "6"},
        {"linode_id": 123, "config_id": "1/2"},
        {"linode_id": 123, "config_id": "1?x"},
        {"linode_id": 123, "config_id": ".."},
    ],
)
async def test_handle_linode_instance_config_get_invalid_ids(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_config_get rejects malformed path parameters."""
    result = await handle_linode_instance_config_get(arguments, sample_config)

    assert len(result) == 1
    assert "positive integer" in result[0].text


async def test_handle_linode_instance_config_get_error(sample_config: Config) -> None:
    """Test linode_instance_config_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_get(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_config_interface_get_tool_definition() -> None:
    """Test linode_instance_config_interface_get tool definition."""
    tool, capability = create_linode_instance_config_interface_get_tool()

    assert tool.name == "linode_instance_config_interface_get"
    assert capability == Capability.Read
    assert tool.inputSchema["required"] == [
        "linode_id",
        "config_id",
        "interface_id",
    ]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["interface_id"]["minimum"] == 1


async def test_handle_linode_instance_config_interface_get(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interface_get tool."""
    mock_interface = {"id": 9, "purpose": "vlan"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_config_interface.return_value = mock_interface
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_get(
            {"linode_id": 123, "config_id": 6, "interface_id": 9}, sample_config
        )

    assert len(result) == 1
    assert "vlan" in result[0].text
    mock_client.get_instance_config_interface.assert_called_once_with(123, 6, 9)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0, "config_id": 6, "interface_id": 9},
        {"linode_id": -1, "config_id": 6, "interface_id": 9},
        {"linode_id": True, "config_id": 6, "interface_id": 9},
        {"linode_id": "123", "config_id": 6, "interface_id": 9},
        {"linode_id": "1/2", "config_id": 6, "interface_id": 9},
        {"linode_id": "1?x", "config_id": 6, "interface_id": 9},
        {"linode_id": "..", "config_id": 6, "interface_id": 9},
        {"linode_id": 123},
        {"linode_id": 123, "config_id": 0, "interface_id": 9},
        {"linode_id": 123, "config_id": -1, "interface_id": 9},
        {"linode_id": 123, "config_id": True, "interface_id": 9},
        {"linode_id": 123, "config_id": "6", "interface_id": 9},
        {"linode_id": 123, "config_id": "1/2", "interface_id": 9},
        {"linode_id": 123, "config_id": "1?x", "interface_id": 9},
        {"linode_id": 123, "config_id": "..", "interface_id": 9},
        {"linode_id": 123, "config_id": 6},
        {"linode_id": 123, "config_id": 6, "interface_id": 0},
        {"linode_id": 123, "config_id": 6, "interface_id": -1},
        {"linode_id": 123, "config_id": 6, "interface_id": True},
        {"linode_id": 123, "config_id": 6, "interface_id": "9"},
        {"linode_id": 123, "config_id": 6, "interface_id": "1/2"},
        {"linode_id": 123, "config_id": 6, "interface_id": "1?x"},
        {"linode_id": 123, "config_id": 6, "interface_id": ".."},
    ],
)
async def test_handle_linode_instance_config_interface_get_invalid_ids(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_config_interface_get rejects malformed path parameters."""
    result = await handle_linode_instance_config_interface_get(arguments, sample_config)

    assert len(result) == 1
    assert "positive integer" in result[0].text


async def test_handle_linode_instance_config_interface_get_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interface_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_config_interface.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_get(
            {"linode_id": 123, "config_id": 6, "interface_id": 9}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_config_interfaces_list_tool_definition() -> None:
    """Test linode_instance_config_interfaces_list tool definition."""
    tool, capability = create_linode_instance_config_interfaces_list_tool()

    assert tool.name == "linode_instance_config_interfaces_list"
    assert capability == Capability.Read
    assert tool.inputSchema["required"] == ["linode_id", "config_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1


async def test_handle_linode_instance_config_interfaces_list(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interfaces_list tool."""
    mock_interfaces = {
        "data": [{"id": 9, "purpose": "vlan"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instance_config_interfaces.return_value = mock_interfaces
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interfaces_list(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "vlan" in result[0].text
    mock_client.list_instance_config_interfaces.assert_called_once_with(123, 6)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0, "config_id": 6},
        {"linode_id": -1, "config_id": 6},
        {"linode_id": True, "config_id": 6},
        {"linode_id": "123", "config_id": 6},
        {"linode_id": "1/2", "config_id": 6},
        {"linode_id": "1?x", "config_id": 6},
        {"linode_id": "..", "config_id": 6},
        {"linode_id": 123},
        {"linode_id": 123, "config_id": 0},
        {"linode_id": 123, "config_id": -1},
        {"linode_id": 123, "config_id": True},
        {"linode_id": 123, "config_id": "6"},
        {"linode_id": 123, "config_id": "1/2"},
        {"linode_id": 123, "config_id": "1?x"},
        {"linode_id": 123, "config_id": ".."},
    ],
)
async def test_handle_linode_instance_config_interfaces_list_invalid_ids(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_config_interfaces_list rejects malformed path parameters."""
    result = await handle_linode_instance_config_interfaces_list(
        arguments, sample_config
    )

    assert len(result) == 1
    assert "positive integer" in result[0].text


async def test_handle_linode_instance_config_interfaces_list_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interfaces_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instance_config_interfaces.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interfaces_list(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_configs_list_tool_definition() -> None:
    """Test linode_instance_configs_list tool definition."""
    tool, capability = create_linode_instance_configs_list_tool()

    assert tool.name == "linode_instance_configs_list"
    assert capability == Capability.Read
    assert tool.inputSchema["required"] == ["linode_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


def test_create_linode_instance_stats_tool_schema() -> None:
    """Linode instance stats tool requires a positive Linode ID."""
    tool, capability = create_linode_instance_stats_tool()

    assert tool.name == "linode_instance_stats"
    assert capability == Capability.Read
    assert tool.inputSchema["required"] == ["linode_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1


async def test_handle_linode_instance_stats(sample_config: Config) -> None:
    """Test linode_instance_stats tool."""
    stats_payload = {
        "data": {
            "cpu": [[1715731200000, 1.5]],
            "io": {"io": [[1715731200000, 8.0]], "swap": [[1715731200000, 0]]},
            "netv4": {"in": [[1715731200000, 100.0]], "out": [[1715731200000, 42.0]]},
            "netv6": {"in": [[1715731200000, 10.0]], "out": [[1715731200000, 4.0]]},
        },
        "title": "linode123 - day (5 min avg)",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance_stats.return_value = stats_payload
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_stats(
            {"linode_id": 123456}, sample_config
        )

    assert len(result) == 1
    assert "linode123" in result[0].text
    assert "1715731200000" in result[0].text
    mock_client.get_instance_stats.assert_awaited_once_with(123456)


@pytest.mark.parametrize("linode_id", [None, 0, -1, True, "1", "1/2", "1?x", ".."])
async def test_handle_linode_instance_stats_rejects_invalid_linode_id(
    sample_config: Config, linode_id: object
) -> None:
    """Malformed Linode IDs are rejected before the client call."""
    arguments = {} if linode_id is None else {"linode_id": linode_id}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_stats(arguments, sample_config)

    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_configs_list(sample_config: Config) -> None:
    """Test linode_instance_configs_list tool."""
    mock_configs = {
        "data": [{"id": 6, "label": "boot-config"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instance_configs.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_configs_list(
            {"linode_id": 123, "page": 2, "page_size": 50}, sample_config
        )

    assert len(result) == 1
    assert "boot-config" in result[0].text
    mock_client.list_instance_configs.assert_called_once_with(123, page=2, page_size=50)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0},
        {"linode_id": -1},
        {"linode_id": True},
        {"linode_id": "123"},
        {"linode_id": "1/2"},
        {"linode_id": "1?x"},
        {"linode_id": ".."},
    ],
)
async def test_handle_linode_instance_configs_list_invalid_linode_id(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_configs_list rejects malformed path parameters."""
    result = await handle_linode_instance_configs_list(arguments, sample_config)

    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text


@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": 123, "page": 0},
        {"linode_id": 123, "page": "1"},
        {"linode_id": 123, "page_size": 24},
        {"linode_id": 123, "page_size": 501},
        {"linode_id": 123, "page_size": True},
    ],
)
async def test_handle_linode_instance_configs_list_invalid_pagination(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """linode_instance_configs_list rejects invalid pagination."""
    result = await handle_linode_instance_configs_list(arguments, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_instance_configs_list_error(sample_config: Config) -> None:
    """Test linode_instance_configs_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instance_configs.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_configs_list(
            {"linode_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_instances_list(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instances_list tool."""
    mock_instance = Instance(
        id=sample_instance_data["id"],
        label=sample_instance_data["label"],
        status=sample_instance_data["status"],
        type=sample_instance_data["type"],
        region=sample_instance_data["region"],
        image=sample_instance_data["image"],
        ipv4=sample_instance_data["ipv4"],
        ipv6=sample_instance_data["ipv6"],
        hypervisor=sample_instance_data["hypervisor"],
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(**sample_instance_data["backups"]["schedule"]),
            last_successful=None,
        ),
        created=sample_instance_data["created"],
        updated=sample_instance_data["updated"],
        group=sample_instance_data["group"],
        tags=sample_instance_data["tags"],
        watchdog_enabled=sample_instance_data["watchdog_enabled"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = [mock_instance]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list({}, sample_config)

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "123456" in result[0].text
        assert "running" in result[0].text


async def test_handle_linode_instances_list_with_status_filter(
    sample_config: Config,
    sample_instance_data: dict[str, Any],
) -> None:
    """Test linode_instances_list tool with status filter."""
    running_instance = Instance(
        id=123456,
        label="running-instance",
        status="running",
        type="g6-standard-1",
        region="us-east",
        image="linode/ubuntu22.04",
        ipv4=["192.0.2.1"],
        ipv6="2001:db8::1/64",
        hypervisor="kvm",
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=True,
            available=True,
            schedule=Schedule(day="Saturday", window="W22"),
            last_successful=None,
        ),
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
        group="production",
        tags=["web"],
        watchdog_enabled=True,
    )

    stopped_instance = Instance(
        id=789012,
        label="stopped-instance",
        status="stopped",
        type="g6-standard-1",
        region="us-east",
        image="linode/ubuntu22.04",
        ipv4=["192.0.2.2"],
        ipv6="2001:db8::2/64",
        hypervisor="kvm",
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=True,
            available=True,
            schedule=Schedule(day="Saturday", window="W22"),
            last_successful=None,
        ),
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
        group="staging",
        tags=["test"],
        watchdog_enabled=False,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = [running_instance, stopped_instance]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list(
            {"status": "running"}, sample_config
        )

        assert len(result) == 1
        assert "running-instance" in result[0].text
        assert "stopped-instance" not in result[0].text
        assert '"count": 1' in result[0].text
        assert "status=running" in result[0].text


async def test_handle_linode_instances_list_error(sample_config: Config) -> None:
    """Test linode_instances_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list({}, sample_config)

        assert len(result) == 1
        assert (
            "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()
        )


# Stage 2 Tool Tests


async def test_handle_linode_instance_get(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_get tool."""
    mock_instance = Instance(
        id=sample_instance_data["id"],
        label=sample_instance_data["label"],
        status=sample_instance_data["status"],
        type=sample_instance_data["type"],
        region=sample_instance_data["region"],
        image=sample_instance_data["image"],
        ipv4=sample_instance_data["ipv4"],
        ipv6=sample_instance_data["ipv6"],
        hypervisor=sample_instance_data["hypervisor"],
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(**sample_instance_data["backups"]["schedule"]),
            last_successful=None,
        ),
        created=sample_instance_data["created"],
        updated=sample_instance_data["updated"],
        group=sample_instance_data["group"],
        tags=sample_instance_data["tags"],
        watchdog_enabled=sample_instance_data["watchdog_enabled"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = mock_instance
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": "123456"}, sample_config
        )

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "running" in result[0].text
        mock_client.get_instance.assert_called_once_with(123456)


async def test_handle_linode_instance_get_missing_id(sample_config: Config) -> None:
    """Test linode_instance_get tool with missing ID."""
    result = await handle_linode_instance_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_instance_get_invalid_id(sample_config: Config) -> None:
    """Test linode_instance_get tool with invalid ID."""
    result = await handle_linode_instance_get(
        {"instance_id": "not-a-number"}, sample_config
    )

    assert len(result) == 1
    assert "Error" in result[0].text or "integer" in result[0].text.lower()


async def test_handle_linode_account(sample_config: Config) -> None:
    """Test linode_account tool."""
    mock_account = Account(
        first_name="Test",
        last_name="User",
        email="test@example.com",
        company="TestCo",
        address_1="123 Test St",
        address_2="",
        city="Test City",
        state="TS",
        zip="12345",
        country="US",
        phone="555-1234",
        balance=100.50,
        balance_uninvoiced=50.25,
        capabilities=["Linodes", "Block Storage"],
        active_since="2020-01-01T00:00:00",
        euuid="abcd-1234",
        billing_source="linode",
        active_promotions=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account.return_value = mock_account
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account({}, sample_config)

        assert len(result) == 1
        assert "Test" in result[0].text
        assert "test@example.com" in result[0].text
        mock_client.get_account.assert_called_once()


async def test_create_linode_account_beta_enroll_tool() -> None:
    """Test linode_account_beta_enroll tool schema."""
    tool, capability = create_linode_account_beta_enroll_tool()

    assert tool.name == "linode_account_beta_enroll"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["id"]["type"] == "string"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "id" in tool.inputSchema.get("required", [])
    assert "confirm" in tool.inputSchema.get("required", [])


async def test_handle_linode_account_beta_enroll_dry_run(
    sample_config: Config,
) -> None:
    """dry_run=true previews beta enrollment without a client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_enroll(
            {"id": "distributed-beta", "dry_run": True, "confirm": True}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_account_beta_enroll"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/account/betas"
    assert body["would_execute"]["body"] == {"id": "distributed-beta"}
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert "distributed-beta" in body["side_effects"][0]
    mock_client_class.assert_not_called()


async def test_handle_linode_account_beta_enroll_dry_run_previews_without_confirm(
    sample_config: Config,
) -> None:
    """Dry-run previews without requiring the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_enroll(
            {"id": "distributed-beta", "dry_run": True}, sample_config
        )

    assert '"dry_run": true' in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_beta_enroll(
    sample_config: Config,
) -> None:
    """Test linode_account_beta_enroll tool."""
    response_data = {"id": "distributed-beta", "label": "Distributed Beta"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.enroll_account_beta.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_beta_enroll(
            {"id": "distributed-beta", "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == response_data
    mock_client.enroll_account_beta.assert_awaited_once_with("distributed-beta")


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_account_beta_enroll_requires_boolean_confirm(
    sample_config: Config, bad_confirm: object
) -> None:
    """Beta enrollment rejects non-true confirm before client call."""
    arguments: dict[str, object] = {"id": "distributed-beta"}
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_enroll(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "id is required"),
        ({"id": 123, "confirm": True}, "id must be a string"),
        ({"id": "   ", "confirm": True}, "id is required"),
    ],
)
async def test_handle_linode_account_beta_enroll_rejects_invalid_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Beta enrollment validates the required beta id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_enroll(arguments, sample_config)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_account_agreements_acknowledge_tool() -> None:
    """Test linode_account_agreements_acknowledge tool schema."""
    tool, capability = create_linode_account_agreements_acknowledge_tool()

    assert tool.name == "linode_account_agreements_acknowledge"
    assert capability.name == "Write"
    assert "eu_model" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema.get("required", [])


async def test_account_agreements_ack_schema_requires_confirm() -> None:
    """The schema requires confirm for mutating acknowledgement calls."""
    tool, _capability = create_linode_account_agreements_acknowledge_tool()

    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema.get("required", [])


async def test_handle_linode_account_agreements_acknowledge_dry_run(
    sample_config: Config,
) -> None:
    """dry_run=true previews acknowledgement without confirm or client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreements_acknowledge(
            {"eu_model": True, "dry_run": True}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_account_agreements_acknowledge"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/account/agreements"
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert "acknowledged" in body["side_effects"][0]
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge(
    sample_config: Config,
) -> None:
    """Test linode_account_agreements_acknowledge tool."""
    response_data = {"accepted": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.acknowledge_account_agreements.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_agreements_acknowledge(
            {"eu_model": True, "privacy_policy": False, "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.acknowledge_account_agreements.assert_awaited_once_with(
        {"eu_model": True, "privacy_policy": False}
    )


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_account_agreements_acknowledge_requires_boolean_confirm(
    sample_config: Config, bad_confirm: object
) -> None:
    """Agreement acknowledgement rejects non-true confirm before client call."""
    arguments: dict[str, object] = {"eu_model": True}
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreements_acknowledge(
            arguments, sample_config
        )

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge_requires_field(
    sample_config: Config,
) -> None:
    """Agreement acknowledgement requires at least one agreement field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreements_acknowledge(
            {"confirm": True}, sample_config
        )

    assert "At least one account agreement field" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge_requires_boolean_field(
    sample_config: Config,
) -> None:
    """Agreement acknowledgement rejects non-boolean agreement values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreements_acknowledge(
            {"confirm": True, "eu_model": "true"}, sample_config
        )

    assert "eu_model must be a boolean" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_account_update_tool() -> None:
    """Test linode_account_update tool schema."""
    tool, capability = create_linode_account_update_tool()

    assert tool.name == "linode_account_update"
    assert capability.name == "Write"
    assert "email" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["required"]


async def test_handle_linode_account_update(sample_config: Config) -> None:
    """Test linode_account_update tool."""
    mock_account = Account(
        first_name="Test",
        last_name="User",
        email="updated@example.com",
        company="TestCo",
        address_1="123 Test St",
        address_2="Suite 1",
        city="Test City",
        state="TS",
        zip="12345",
        country="US",
        phone="555-1234",
        balance=100.50,
        balance_uninvoiced=50.25,
        capabilities=["Linodes", "Block Storage"],
        active_since="2020-01-01T00:00:00",
        euuid="abcd-1234",
        billing_source="linode",
        active_promotions=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account.return_value = mock_account
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_update(
            {"email": "updated@example.com", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "updated@example.com" in result[0].text
        mock_client.update_account.assert_called_once_with(email="updated@example.com")


async def test_handle_linode_account_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Test linode_account_update requires confirmation."""
    result = await handle_linode_account_update(
        {"email": "updated@example.com"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_update_requires_field(
    sample_config: Config,
) -> None:
    """Test linode_account_update requires an account field."""
    result = await handle_linode_account_update({"confirm": True}, sample_config)

    assert len(result) == 1
    assert "At least one account field" in result[0].text


async def test_create_linode_managed_contacts_list_tool() -> None:
    """Test linode_managed_contacts_list tool schema."""
    tool, capability = create_linode_managed_contacts_list_tool()

    assert tool.name == "linode_managed_contacts_list"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "required" not in tool.inputSchema
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


async def test_handle_linode_managed_contacts_list(sample_config: Config) -> None:
    """Test linode_managed_contacts_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "name": "Primary", "email": "ops@example.com"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_managed_contacts.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contacts_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_managed_contacts.assert_awaited_once_with(page=1, page_size=25)


async def test_handle_linode_managed_contacts_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_contacts_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contacts_list({"page": 0}, sample_config)

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("page_size", "expected"),
    [
        (24, "page_size must be at least 25"),
        (501, "page_size must be at most 500"),
    ],
)
async def test_handle_linode_managed_contacts_list_rejects_invalid_page_size(
    sample_config: Config, page_size: int, expected: str
) -> None:
    """Test linode_managed_contacts_list rejects out-of-range page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contacts_list(
            {"page_size": page_size}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_issues_list_tool() -> None:
    """Test linode_managed_issues_list tool schema."""
    tool, capability = create_linode_managed_issues_list_tool()

    assert tool.name == "linode_managed_issues_list"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "required" not in tool.inputSchema
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


async def test_handle_linode_managed_issues_list(sample_config: Config) -> None:
    """Test linode_managed_issues_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "entity": {"label": "web-1"}}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_managed_issues.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issues_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_managed_issues.assert_awaited_once_with(page=1, page_size=25)


async def test_handle_linode_managed_issues_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_issues_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_issues_list({"page": 0}, sample_config)

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("page_size", "expected"),
    [
        (24, "page_size must be at least 25"),
        (501, "page_size must be at most 500"),
    ],
)
async def test_handle_linode_managed_issues_list_rejects_invalid_page_size(
    sample_config: Config, page_size: int, expected: str
) -> None:
    """Test linode_managed_issues_list rejects out-of-range page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_issues_list(
            {"page_size": page_size}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_linode_settings_list_tool() -> None:
    """Test linode_managed_linode_settings_list tool schema."""
    tool, capability = create_linode_managed_linode_settings_list_tool()

    assert tool.name == "linode_managed_linode_settings_list"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "required" not in tool.inputSchema
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


async def test_handle_linode_managed_linode_settings_list(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 123, "label": "web-1", "group": "prod"}],
        "page": 2,
        "pages": 4,
        "results": 76,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_managed_linode_settings.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_linode_settings_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.list_managed_linode_settings.assert_awaited_once_with(
        page=2, page_size=25
    )


async def test_handle_linode_managed_linode_settings_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects invalid page."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page": 0}, sample_config
        )

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_linode_settings_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects bad page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page_size": 501}, sample_config
        )

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_linode_settings_list_rejects_low_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects low page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page_size": 24}, sample_config
        )

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_service_disable_tool() -> None:
    """Test linode_managed_service_disable tool schema."""
    tool, capability = create_linode_managed_service_disable_tool()

    assert tool.name == "linode_managed_service_disable"
    assert capability is Capability.Write
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["service_id", "confirm"]
    assert tool.inputSchema["properties"]["service_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_managed_service_disable(sample_config: Config) -> None:
    """Test linode_managed_service_disable tool."""
    response_data: dict[str, Any] = {"id": 9944, "status": "disabled"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.disable_managed_service.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_service_disable(
            {"service_id": 9944, "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "message": "Managed service disabled successfully",
        "result": response_data,
    }
    mock_client.disable_managed_service.assert_awaited_once_with(9944)


async def test_handle_linode_managed_service_disable_requires_confirm(
    sample_config: Config,
) -> None:
    """Test linode_managed_service_disable requires confirm."""
    arguments: dict[str, Any] = {"service_id": 9944}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_disable(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_disable_validates_service_id(
    sample_config: Config,
) -> None:
    """Test linode_managed_service_disable validates service_id."""
    arguments: dict[str, Any] = {"service_id": "1/2", "confirm": True}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_disable(arguments, sample_config)

    assert "service_id" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_disable_dry_run(
    sample_config: Config,
) -> None:
    """Test linode_managed_service_disable dry run response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_disable(
            {"service_id": 9944, "confirm": True, "dry_run": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_managed_service_disable"
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == "/managed/services/9944/disable"
    mock_client_class.assert_not_called()


async def test_create_linode_managed_contact_delete_tool() -> None:
    """Test linode_managed_contact_delete tool schema."""
    tool, capability = create_linode_managed_contact_delete_tool()

    assert tool.name == "linode_managed_contact_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["contact_id", "confirm"]
    assert tool.inputSchema["properties"]["contact_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_managed_contact_delete(sample_config: Config) -> None:
    """Test linode_managed_contact_delete tool."""
    response_data: dict[str, Any] = {"id": 123}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_managed_contact.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_delete(
            {"contact_id": 123, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Managed contact deleted successfully",
            "result": response_data,
        }
        mock_client.delete_managed_contact.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_managed_contact_delete_requires_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Managed contact delete requires literal confirm=true before client calls."""
    arguments: dict[str, object] = {"contact_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_delete(arguments, sample_config)

    assert "confirm" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("contact_id", [None, 0, "123", "1/2", "1?x", "..", True])
async def test_handle_linode_managed_contact_delete_validates_contact_id(
    sample_config: Config, contact_id: object
) -> None:
    """Managed contact delete validates contact ID before client calls."""
    arguments: dict[str, object] = {"confirm": True}
    if contact_id is not None:
        arguments["contact_id"] = contact_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_delete(arguments, sample_config)

    assert "contact_id" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contact_delete_dry_run(
    sample_config: Config,
) -> None:
    """Managed contact delete dry run previews DELETE without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_delete(
            {"contact_id": 123, "confirm": True, "dry_run": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_managed_contact_delete"
    assert payload["would_execute"]["method"] == "DELETE"
    assert payload["would_execute"]["path"] == "/managed/contacts/123"
    mock_client_class.assert_not_called()


async def test_create_linode_managed_credential_get_tool() -> None:
    """Test linode_managed_credential_get tool schema."""
    tool, capability = create_linode_managed_credential_get_tool()

    assert tool.name == "linode_managed_credential_get"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["credential_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["credential_id"]["minimum"] == 1
    assert tool.inputSchema["required"] == ["credential_id"]


async def test_handle_linode_managed_credential_get(sample_config: Config) -> None:
    """Test linode_managed_credential_get tool."""
    response_data: dict[str, Any] = {"id": 123, "label": "db-root"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_credential.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_credential_get(
            {"credential_id": 123}, sample_config
        )

        assert json.loads(result[0].text) == response_data
        mock_client.get_managed_credential.assert_awaited_once_with(123)


@pytest.mark.parametrize("credential_id", [None, 0, -1, True, "/", "1?", ".."])
async def test_handle_linode_managed_credential_get_rejects_invalid_id(
    sample_config: Config, credential_id: object
) -> None:
    """Managed credential get rejects invalid IDs before client construction."""
    arguments = {} if credential_id is None else {"credential_id": credential_id}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_get(arguments, sample_config)

    assert "credential_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_credential_username_password_update_tool() -> None:
    """Test username/password credential update tool schema."""
    tool, capability = create_linode_managed_credential_username_password_update_tool()
    assert tool.name == "linode_managed_credential_username_password_update"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {
        "credential_id",
        "password",
        "confirm",
    }
    assert tool.inputSchema["properties"]["credential_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["password"]["type"] == "string"
    assert tool.inputSchema["properties"]["username"]["type"] == "string"


async def test_handle_linode_managed_credential_username_password_update(
    sample_config: Config,
) -> None:
    """Test username/password credential update handler."""
    response_data: dict[str, Any] = {"id": 91, "username": "root"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_update = mock_client.update_managed_credential_username_password
        mock_update.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        result = await handle_linode_managed_credential_username_password_update(
            {
                "credential_id": 91,
                "password": "s3cret",
                "username": "root",
                "confirm": True,
            },
            sample_config,
        )
    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_update.assert_awaited_once_with(91, password="s3cret", username="root")


async def test_create_linode_managed_credential_revoke_tool() -> None:
    """Test linode_managed_credential_revoke tool schema."""
    tool, capability = create_linode_managed_credential_revoke_tool()

    assert tool.name == "linode_managed_credential_revoke"
    assert capability is Capability.Destroy
    assert set(tool.inputSchema["required"]) == {"credential_id", "confirm"}
    assert tool.inputSchema["properties"]["credential_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_managed_credential_revoke(sample_config: Config) -> None:
    """Test linode_managed_credential_revoke handler."""
    response_data = {"message": "Credential revoked"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.revoke_managed_credential.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_credential_revoke(
            {"credential_id": 91, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.revoke_managed_credential.assert_awaited_once_with(91)


async def test_handle_linode_managed_credential_revoke_rejects_bad_id(
    sample_config: Config,
) -> None:
    """Test linode_managed_credential_revoke rejects invalid IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_revoke(
            {"credential_id": "91/../x", "confirm": True}, sample_config
        )

    assert "credential_id must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_credentials_list_tool() -> None:
    """Test linode_managed_credentials_list tool schema."""
    tool, capability = create_linode_managed_credentials_list_tool()

    assert tool.name == "linode_managed_credentials_list"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


async def test_handle_linode_managed_credentials_list(sample_config: Config) -> None:
    """Test linode_managed_credentials_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "label": "credential"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_managed_credentials.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_credentials_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_managed_credentials.assert_awaited_once_with(
            page=1, page_size=25
        )


async def test_handle_linode_managed_credentials_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_credentials_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credentials_list(
            {"page": 0}, sample_config
        )

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_credentials_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_credentials_list rejects invalid page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credentials_list(
            {"page_size": 501}, sample_config
        )

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_ssh_key_get_tool() -> None:
    """Test linode_managed_ssh_key_get tool schema."""
    tool, capability = create_linode_managed_ssh_key_get_tool()

    assert tool.name == "linode_managed_ssh_key_get"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "required" not in tool.inputSchema


async def test_handle_linode_managed_ssh_key_get(sample_config: Config) -> None:
    """Test linode_managed_ssh_key_get tool."""
    response_data: dict[str, Any] = {"ssh_key": "ssh-rsa AAAAmanagedkey linode-managed"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_ssh_key.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_ssh_key_get({}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.get_managed_ssh_key.assert_awaited_once_with()


async def test_handle_linode_managed_ssh_key_get_propagates_errors(
    sample_config: Config,
) -> None:
    """Test linode_managed_ssh_key_get reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_ssh_key.side_effect = Exception("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_ssh_key_get({}, sample_config)

    assert "Failed to get Linode Managed SSH key" in result[0].text
    assert "boom" in result[0].text
    mock_client.get_managed_ssh_key.assert_awaited_once_with()


async def test_create_linode_managed_credential_update_tool() -> None:
    """Test linode_managed_credential_update tool schema."""
    tool, capability = create_linode_managed_credential_update_tool()

    assert tool.name == "linode_managed_credential_update"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {"credential_id", "label", "confirm"}
    assert tool.inputSchema["properties"]["credential_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["label"]["type"] == "string"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_managed_credential_update(sample_config: Config) -> None:
    """Test linode_managed_credential_update tool."""
    response_data = {"id": 42, "label": "prod-root"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_managed_credential.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_credential_update(
            {"credential_id": 42, "label": "prod-root", "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.update_managed_credential.assert_awaited_once_with(
        42, label="prod-root"
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_managed_credential_update_requires_confirm(
    sample_config: Config,
    confirm: object,
) -> None:
    """Test linode_managed_credential_update requires literal confirm=true."""
    arguments: dict[str, object] = {"credential_id": 42, "label": "prod-root"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_update(arguments, sample_config)

    assert "confirm" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_credential_id", [0, -1, True, "1/2", "1?x", ".."])
async def test_handle_linode_managed_credential_update_rejects_bad_credential_id(
    sample_config: Config,
    bad_credential_id: object,
) -> None:
    """Test linode_managed_credential_update validates credential_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_update(
            {"credential_id": bad_credential_id, "label": "prod-root", "confirm": True},
            sample_config,
        )

    assert "credential_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"credential_id": 42, "confirm": True}, "label is required"),
        (
            {"credential_id": 42, "label": "", "confirm": True},
            "label must be a non-empty string",
        ),
        (
            {"credential_id": 42, "label": 123, "confirm": True},
            "label must be a non-empty string",
        ),
        (
            {"credential_id": 42, "label": "prod-root", "id": 99, "confirm": True},
            "Read-only fields are not accepted: id",
        ),
        (
            {
                "credential_id": 42,
                "label": "prod-root",
                "last_decrypted": "2024-01-01T00:00:00",
                "confirm": True,
            },
            "Read-only fields are not accepted: last_decrypted",
        ),
    ],
)
async def test_handle_linode_managed_credential_update_rejects_invalid_body(
    sample_config: Config,
    arguments: dict[str, object],
    expected: str,
) -> None:
    """Test linode_managed_credential_update validates body fields."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_update(arguments, sample_config)

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_credential_update_dry_run(
    sample_config: Config,
) -> None:
    """Test linode_managed_credential_update dry run."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_update(
            {
                "credential_id": 42,
                "label": "prod-root",
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_managed_credential_update"
    assert payload["would_execute"]["method"] == "PUT"
    assert payload["would_execute"]["path"] == "/managed/credentials/42"
    assert payload["would_execute"]["body"] == {"label": "prod-root"}
    mock_client_class.assert_not_called()


async def test_create_linode_managed_stats_tool() -> None:
    """Test linode_managed_stats tool schema."""
    tool, capability = create_linode_managed_stats_tool()

    assert tool.name == "linode_managed_stats"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "required" not in tool.inputSchema


async def test_handle_linode_managed_stats(sample_config: Config) -> None:
    """Test linode_managed_stats tool."""
    response_data: dict[str, Any] = {"data": {"cpu": [{"x": 1, "y": 2.0}]}}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_stats.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_stats({}, sample_config)

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_managed_stats.assert_awaited_once_with()


async def test_create_linode_managed_issue_get_tool() -> None:
    """Test linode_managed_issue_get tool schema."""
    tool, capability = create_linode_managed_issue_get_tool()

    assert tool.name == "linode_managed_issue_get"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["issue_id"]
    assert tool.inputSchema["properties"]["issue_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["issue_id"]["minimum"] == 1


async def test_handle_linode_managed_issue_get(sample_config: Config) -> None:
    """Test linode_managed_issue_get tool."""
    response_data: dict[str, Any] = {"id": 77, "entity": {"label": "web-1"}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_issue.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issue_get({"issue_id": 77}, sample_config)

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_managed_issue.assert_awaited_once_with(77)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"issue_id": 0},
        {"issue_id": False},
        {"issue_id": "77"},
        {"issue_id": "1/2"},
        {"issue_id": "1?x"},
        {"issue_id": ".."},
    ],
)
async def test_handle_linode_managed_issue_get_rejects_bad_issue_id(
    arguments: dict[str, object], sample_config: Config
) -> None:
    """Test Managed issue handler rejects missing or unsafe issue IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_issue_get(arguments, sample_config)

    assert len(result) == 1
    assert "issue_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_issue_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed issue handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_issue.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issue_get({"issue_id": 77}, sample_config)

    assert len(result) == 1
    assert "Failed to get Linode Managed issue: boom" in result[0].text
    mock_client.get_managed_issue.assert_awaited_once_with(77)


async def test_create_linode_managed_contact_get_tool() -> None:
    """Test linode_managed_contact_get tool schema."""
    tool, capability = create_linode_managed_contact_get_tool()

    assert tool.name == "linode_managed_contact_get"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["contact_id"]
    assert tool.inputSchema["properties"]["contact_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["contact_id"]["minimum"] == 1


async def test_handle_linode_managed_contact_get(sample_config: Config) -> None:
    """Test linode_managed_contact_get tool."""
    response_data: dict[str, Any] = {"id": 42, "name": "Primary on-call"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_contact.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_get(
            {"contact_id": 42}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_managed_contact.assert_awaited_once_with(42)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"contact_id": 0},
        {"contact_id": False},
        {"contact_id": "42"},
        {"contact_id": "1/2"},
        {"contact_id": "1?x"},
        {"contact_id": ".."},
    ],
)
async def test_handle_linode_managed_contact_get_rejects_bad_contact_id(
    arguments: dict[str, object], sample_config: Config
) -> None:
    """Test Managed contact handler rejects missing or unsafe contact IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_get(arguments, sample_config)

    assert len(result) == 1
    assert "contact_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contact_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed contact handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_contact.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_get(
            {"contact_id": 42}, sample_config
        )

    assert len(result) == 1
    assert "Failed to get Linode Managed contact: boom" in result[0].text
    mock_client.get_managed_contact.assert_awaited_once_with(42)


async def test_create_linode_managed_service_get_tool() -> None:
    """Test linode_managed_service_get tool schema."""
    tool, capability = create_linode_managed_service_get_tool()

    assert tool.name == "linode_managed_service_get"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["service_id"]
    assert tool.inputSchema["properties"]["service_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["service_id"]["minimum"] == 1
    assert "confirm" not in tool.inputSchema["properties"]


async def test_handle_linode_managed_service_get(sample_config: Config) -> None:
    """Test linode_managed_service_get tool."""
    response_data: dict[str, Any] = {"id": 314, "label": "web monitor"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_service.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_service_get(
            {"service_id": 314}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_managed_service.assert_awaited_once_with(314)


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"service_id": 0},
        {"service_id": -1},
        {"service_id": False},
        {"service_id": "314"},
        {"service_id": "1/2"},
        {"service_id": "1?x"},
        {"service_id": ".."},
    ],
)
async def test_handle_linode_managed_service_get_rejects_bad_service_id(
    arguments: dict[str, object], sample_config: Config
) -> None:
    """Test Managed service handler rejects missing or unsafe service IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_get(arguments, sample_config)

    assert len(result) == 1
    assert "service_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed service handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_service.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_service_get(
            {"service_id": 314}, sample_config
        )

    assert len(result) == 1
    assert "Failed to get Linode Managed service monitor: boom" in result[0].text
    mock_client.get_managed_service.assert_awaited_once_with(314)


async def test_create_linode_account_beta_get_tool() -> None:
    """Test linode_account_beta_get tool schema."""
    tool, capability = create_linode_account_beta_get_tool()

    assert tool.name == "linode_account_beta_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["beta_id"]
    assert tool.inputSchema["properties"]["beta_id"]["type"] == "string"


async def test_handle_linode_account_beta_get(sample_config: Config) -> None:
    """Test linode_account_beta_get tool."""
    response_data = {"id": "example-open", "label": "Example Open Beta"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_beta.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_beta_get(
            {"beta_id": "example-open"}, sample_config
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_beta.assert_awaited_once_with("example-open")


@pytest.mark.parametrize("arguments", [{}, {"beta_id": ""}, {"beta_id": "   "}])
async def test_handle_linode_account_beta_get_requires_beta_id(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """Account beta get requires a non-empty beta_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get(arguments, sample_config)

    assert "beta_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_beta_get_rejects_non_string_beta_id(
    sample_config: Config,
) -> None:
    """Account beta get rejects non-string beta_id values before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get({"beta_id": 123}, sample_config)

    assert "beta_id must be a string" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("beta_id", ["example/open", "example?open", ".."])
async def test_handle_linode_account_beta_get_rejects_malformed_beta_id(
    beta_id: str, sample_config: Config
) -> None:
    """Account beta get rejects malformed path separator/traversal values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get(
            {"beta_id": beta_id}, sample_config
        )

    assert "beta_id must not contain" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_account_settings_get_tool() -> None:
    """Test linode_account_settings_get tool schema."""
    tool, capability = create_linode_account_settings_get_tool()

    assert tool.name == "linode_account_settings_get"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment"}
    assert "required" not in tool.inputSchema


async def test_handle_linode_account_settings_get(sample_config: Config) -> None:
    """Test linode_account_settings_get tool."""
    response_data: dict[str, Any] = {
        "backups_enabled": True,
        "managed": False,
        "network_helper": True,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_settings.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_get({}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.get_account_settings.assert_awaited_once_with()


async def test_create_linode_account_maintenance_list_tool() -> None:
    """Test linode_account_maintenance_list tool schema."""
    tool, capability = create_linode_account_maintenance_list_tool()

    assert tool.name == "linode_account_maintenance_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment"}
    assert "required" not in tool.inputSchema


async def test_handle_linode_account_maintenance_list(sample_config: Config) -> None:
    """Test linode_account_maintenance_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"entity": {"id": 123, "type": "linode"}, "status": "pending"}],
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_maintenance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_maintenance_list({}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.list_account_maintenance.assert_awaited_once_with()


async def test_create_linode_maintenance_policies_list_tool() -> None:
    """Test linode_maintenance_policies_list tool schema."""
    tool, capability = create_linode_maintenance_policies_list_tool()

    assert tool.name == "linode_maintenance_policies_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment"}
    assert "required" not in tool.inputSchema


async def test_handle_linode_maintenance_policies_list(sample_config: Config) -> None:
    """Test linode_maintenance_policies_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"slug": "linode/migrate", "label": "Migrate"}],
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_maintenance_policies.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_maintenance_policies_list({}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.list_maintenance_policies.assert_awaited_once_with()


async def test_create_linode_account_availability_list_tool() -> None:
    """Test linode_account_availability_list tool schema."""
    tool, capability = create_linode_account_availability_list_tool()

    assert tool.name == "linode_account_availability_list"
    assert capability is Capability.Read
    assert "page" not in tool.inputSchema.get("required", [])
    assert "page_size" not in tool.inputSchema.get("required", [])


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": "2"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
        ({"page": 0}, "page must be at least 1"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": False}, "page_size must be an integer"),
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
    ],
)
async def test_handle_linode_account_availability_list_rejects_invalid_pagination(
    arguments: dict[str, Any], expected_error: str, sample_config: Config
) -> None:
    """Account availability listing validates pagination arguments."""
    result = await handle_linode_account_availability_list(arguments, sample_config)

    assert len(result) == 1
    assert expected_error in result[0].text


async def test_handle_linode_account_availability_list(sample_config: Config) -> None:
    """Test linode_account_availability_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"service": "Linodes", "available": True}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_availability.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_availability_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.list_account_availability.assert_awaited_once_with(page=2, page_size=25)


async def test_create_linode_account_tags_list_tool() -> None:
    """Test linode_account_tags_list tool schema."""
    tool, capability = create_linode_account_tags_list_tool()

    assert tool.name == "linode_account_tags_list"
    assert capability is Capability.Read
    assert "page" not in tool.inputSchema.get("required", [])
    assert "page_size" not in tool.inputSchema.get("required", [])


async def test_handle_linode_account_tags_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account tag listing validates page."""
    result = await handle_linode_account_tags_list({"page": 0}, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_account_tags_list(sample_config: Config) -> None:
    """Test linode_account_tags_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"label": "production"}, {"label": "web"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_tags.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_tags_list(
            {"page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_tags.assert_awaited_once_with(page=2, page_size=25)


async def test_create_linode_account_tag_objects_list_tool() -> None:
    """Test linode_account_tag_objects_list tool schema."""
    tool, capability = create_linode_account_tag_objects_list_tool()

    assert tool.name == "linode_account_tag_objects_list"
    assert capability is Capability.Read
    assert "tag_label" in tool.inputSchema["required"]
    assert "page" not in tool.inputSchema["required"]


async def test_handle_linode_account_tag_objects_list_requires_label(
    sample_config: Config,
) -> None:
    """Tagged object listing requires a non-empty tag label."""
    result = await handle_linode_account_tag_objects_list({}, sample_config)

    assert len(result) == 1
    assert "tag_label" in result[0].text


async def test_handle_linode_account_tag_objects_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Tagged object listing validates page."""
    result = await handle_linode_account_tag_objects_list(
        {"tag_label": "production", "page": 0}, sample_config
    )

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_account_tag_objects_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Tagged object listing validates page_size."""
    result = await handle_linode_account_tag_objects_list(
        {"tag_label": "production", "page_size": 10}, sample_config
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_tag_objects_list(sample_config: Config) -> None:
    """Test linode_account_tag_objects_list tool."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "type": "linode",
                "data": {"id": 123, "label": "web-1"},
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_tagged_objects.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_tag_objects_list(
            {"tag_label": "production", "page": 2, "page_size": 25},
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_tagged_objects.assert_awaited_once_with(
            "production", page=2, page_size=25
        )


async def test_create_linode_account_tag_create_tool() -> None:
    """Test linode_account_tag_create tool schema."""
    tool, capability = create_linode_account_tag_create_tool()

    assert tool.name == "linode_account_tag_create"
    assert capability is Capability.Write
    assert "label" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]
    assert "linodes" not in tool.inputSchema["required"]


async def test_handle_linode_account_tag_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Tag creation requires confirmation."""
    result = await handle_linode_account_tag_create(
        {"label": "production", "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_tag_create_rejects_non_boolean_confirm(
    sample_config: Config,
) -> None:
    """Tag creation requires confirm to be true boolean."""
    result = await handle_linode_account_tag_create(
        {"confirm": "yes", "label": "production"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_tag_create_requires_label(
    sample_config: Config,
) -> None:
    """Tag creation requires a non-empty label."""
    result = await handle_linode_account_tag_create(
        {"confirm": True, "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "label" in result[0].text


async def test_handle_linode_account_tag_create_rejects_blank_label(
    sample_config: Config,
) -> None:
    """Tag creation rejects a blank label."""
    result = await handle_linode_account_tag_create(
        {"confirm": True, "label": "   ", "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "label" in result[0].text


async def test_handle_linode_account_tag_create_rejects_invalid_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation validates resource ID lists."""
    result = await handle_linode_account_tag_create(
        {"confirm": True, "label": "production", "linodes": [123, "bad"]}, sample_config
    )

    assert len(result) == 1
    assert "linodes" in result[0].text


async def test_handle_linode_account_tag_create_rejects_non_positive_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation rejects non-positive resource IDs."""
    result = await handle_linode_account_tag_create(
        {"confirm": True, "label": "production", "linodes": [0]}, sample_config
    )

    assert len(result) == 1
    assert "positive integers" in result[0].text


async def test_handle_linode_account_tag_create_rejects_boolean_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation rejects boolean resource IDs."""
    result = await handle_linode_account_tag_create(
        {"confirm": True, "label": "production", "volumes": [True]}, sample_config
    )

    assert len(result) == 1
    assert "volumes" in result[0].text


async def test_handle_linode_account_tag_create(sample_config: Config) -> None:
    """Test linode_account_tag_create tool."""
    response_data: dict[str, Any] = {"label": "production"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_tag.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_tag_create(
            {
                "confirm": True,
                "label": "production",
                "domains": [1],
                "linodes": [2],
                "nodebalancers": [3],
                "volumes": [4],
            },
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Tag 'production' created successfully",
            "tag": response_data,
        }
        mock_client.create_tag.assert_awaited_once_with(
            "production",
            domains=[1],
            linodes=[2],
            nodebalancers=[3],
            volumes=[4],
        )


async def test_handle_linode_account_tag_create_omits_empty_resource_lists(
    sample_config: Config,
) -> None:
    """Tag creation omits empty resource lists."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_tag.return_value = {"label": "production"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        await handle_linode_account_tag_create(
            {"confirm": True, "label": "production", "linodes": []}, sample_config
        )

        mock_client.create_tag.assert_awaited_once_with(
            "production",
            domains=None,
            linodes=None,
            nodebalancers=None,
            volumes=None,
        )


async def test_handle_linode_account_tag_create_reports_client_errors(
    sample_config: Config,
) -> None:
    """Tag creation reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_tag.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_tag_create(
            {"confirm": True, "label": "production"}, sample_config
        )

        assert len(result) == 1
        assert "Failed to create Linode tag" in result[0].text


async def test_account_tag_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account tag create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_tag_create_tool" in tools_mod.__all__
    assert "handle_linode_account_tag_create" in tools_mod.__all__

    from linodemcp.server import get_tool_registry

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_account_tag_create"].capability is Capability.Write


async def test_create_linode_account_support_ticket_create_tool() -> None:
    """Test support ticket create tool schema."""
    tool, capability = create_linode_account_support_ticket_create_tool()

    assert tool.name == "linode_account_support_ticket_create"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "summary",
        "description",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["severity"]["maximum"] == 3


async def test_handle_linode_account_support_ticket_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Support ticket creation requires confirmation."""
    result = await handle_linode_account_support_ticket_create(
        {"summary": "Need help", "description": "Details"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_support_ticket_create_requires_summary(
    sample_config: Config,
) -> None:
    """Support ticket creation requires a non-empty summary."""
    result = await handle_linode_account_support_ticket_create(
        {"confirm": True, "description": "Details", "summary": "   "},
        sample_config,
    )

    assert len(result) == 1
    assert "summary" in result[0].text


async def test_handle_linode_account_support_ticket_create_requires_description(
    sample_config: Config,
) -> None:
    """Support ticket creation requires a non-empty description."""
    result = await handle_linode_account_support_ticket_create(
        {"confirm": True, "summary": "Need help", "description": "   "},
        sample_config,
    )

    assert len(result) == 1
    assert "description" in result[0].text


async def test_handle_linode_account_support_ticket_create_rejects_bad_managed_issue(
    sample_config: Config,
) -> None:
    """Support ticket creation validates managed_issue."""
    result = await handle_linode_account_support_ticket_create(
        {
            "confirm": True,
            "summary": "Need help",
            "description": "Details",
            "managed_issue": "yes",
        },
        sample_config,
    )

    assert len(result) == 1
    assert "managed_issue" in result[0].text


async def test_handle_linode_account_support_ticket_create_rejects_bad_severity(
    sample_config: Config,
) -> None:
    """Support ticket creation validates severity."""
    result = await handle_linode_account_support_ticket_create(
        {
            "confirm": True,
            "summary": "Need help",
            "description": "Details",
            "severity": 4,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "severity" in result[0].text


async def test_handle_linode_account_support_ticket_create(
    sample_config: Config,
) -> None:
    """Test support ticket create handler."""
    response_data: dict[str, Any] = {"id": 789, "summary": "Need help"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_create(
            {
                "confirm": True,
                "summary": " Need help ",
                "description": " Details ",
                "linode_id": 123,
                "severity": 2,
            },
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Support ticket opened successfully",
            "ticket": response_data,
        }
        mock_client.create_support_ticket.assert_awaited_once_with(
            "Need help",
            "Details",
            bucket=None,
            database_id=None,
            domain_id=None,
            firewall_id=None,
            linode_id=123,
            lkecluster_id=None,
            longviewclient_id=None,
            managed_issue=None,
            nodebalancer_id=None,
            region=None,
            severity=2,
            vlan=None,
            volume_id=None,
            vpc_id=None,
        )


async def test_handle_linode_account_support_ticket_create_reports_client_errors(
    sample_config: Config,
) -> None:
    """Support ticket creation reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_create(
            {
                "confirm": True,
                "summary": "Need help",
                "description": "Details",
            },
            sample_config,
        )

    assert len(result) == 1
    assert "Failed to open Linode support ticket" in result[0].text
    assert "boom" in result[0].text


async def test_account_support_ticket_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_create_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_create" in tools_mod.__all__

    from linodemcp.server import get_tool_registry

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_account_support_ticket_create"].capability is Capability.Write
    )


async def test_create_linode_account_support_ticket_get_tool() -> None:
    """Test linode_account_support_ticket_get tool schema."""
    tool, capability = create_linode_account_support_ticket_get_tool()

    assert tool.name == "linode_account_support_ticket_get"
    assert capability is Capability.Read
    assert "ticket_id" in tool.inputSchema["required"]


async def test_create_linode_account_support_tickets_list_tool() -> None:
    """Test linode_account_support_tickets_list tool schema."""
    tool, capability = create_linode_account_support_tickets_list_tool()

    assert tool.name == "linode_account_support_tickets_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema
    assert "page" in tool.inputSchema["properties"]
    assert "page_size" in tool.inputSchema["properties"]


async def test_handle_linode_account_support_tickets_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Support ticket listing validates page_size."""
    result = await handle_linode_account_support_tickets_list(
        {"page_size": 10}, sample_config
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_support_tickets_list(
    sample_config: Config,
) -> None:
    """Test linode_account_support_tickets_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 789, "summary": "Need help"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_tickets.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_tickets_list(
            {"page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_support_tickets.assert_awaited_once_with(page=2, page_size=25)


async def test_handle_linode_account_support_tickets_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support tickets list handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_tickets.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_tickets_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to list Linode support tickets" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_support_ticket_get_requires_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval requires a positive ticket_id."""
    result = await handle_linode_account_support_ticket_get({}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get_rejects_bad_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval rejects invalid ticket IDs."""
    result = await handle_linode_account_support_ticket_get(
        {"ticket_id": 0}, sample_config
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get_rejects_bool_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval rejects bool ticket IDs."""
    result = await handle_linode_account_support_ticket_get(
        {"ticket_id": True}, sample_config
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get(
    sample_config: Config,
) -> None:
    """Test linode_account_support_ticket_get tool."""
    response_data: dict[str, Any] = {"id": 123, "summary": "Need help"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_get(
            {"ticket_id": 123}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_support_ticket.assert_awaited_once_with(123)


async def test_handle_linode_account_support_ticket_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support ticket get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_get(
            {"ticket_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to get Linode support ticket" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_oauth_client_get_tool() -> None:
    """Test linode_account_oauth_client_get tool schema."""
    tool, capability = create_linode_account_oauth_client_get_tool()

    assert tool.name == "linode_account_oauth_client_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["client_id"]
    assert tool.inputSchema["properties"]["client_id"]["type"] == "string"


async def test_handle_linode_account_oauth_client_get_requires_client_id(
    sample_config: Config,
) -> None:
    """OAuth client retrieval requires client_id."""
    result = await handle_linode_account_oauth_client_get({}, sample_config)

    assert len(result) == 1
    assert "client_id is required" in result[0].text


async def test_handle_linode_account_oauth_client_get_rejects_bad_client_id(
    sample_config: Config,
) -> None:
    """OAuth client retrieval rejects malformed client IDs."""
    for bad_client_id in (123, "   ", "client/id", "client?id", ".."):
        result = await handle_linode_account_oauth_client_get(
            {"client_id": bad_client_id}, sample_config
        )

        assert len(result) == 1
        assert "client_id" in result[0].text


async def test_handle_linode_account_oauth_client_get(
    sample_config: Config,
) -> None:
    """Test linode_account_oauth_client_get tool."""
    response_data: dict[str, Any] = {
        "id": "client-123",
        "label": "Example OAuth Client",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_get(
            {"client_id": "client-123"}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_account_oauth_client.assert_awaited_once_with("client-123")


async def test_create_linode_account_payment_method_get_tool() -> None:
    """Test linode_account_payment_method_get tool schema."""
    tool, capability = create_linode_account_payment_method_get_tool()

    assert tool.name == "linode_account_payment_method_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["payment_method_id"]
    assert tool.inputSchema["properties"]["payment_method_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["payment_method_id"]["minimum"] == 1


async def test_handle_linode_account_payment_method_get_requires_payment_method_id(
    sample_config: Config,
) -> None:
    """Payment method retrieval requires payment_method_id."""
    result = await handle_linode_account_payment_method_get({}, sample_config)

    assert len(result) == 1
    assert "payment_method_id is required" in result[0].text


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"payment_method_id": "123"}, "payment_method_id must be an integer"),
        ({"payment_method_id": True}, "payment_method_id must be an integer"),
        ({"payment_method_id": 0}, "payment_method_id must be at least 1"),
        ({"payment_method_id": "12/3"}, "payment_method_id must be an integer"),
        ({"payment_method_id": "12?3"}, "payment_method_id must be an integer"),
        ({"payment_method_id": ".."}, "payment_method_id must be an integer"),
    ],
)
async def test_handle_linode_account_payment_method_get_rejects_bad_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Payment method retrieval rejects malformed payment_method_id values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_payment_method_get(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_payment_method_get(
    sample_config: Config,
) -> None:
    """Test linode_account_payment_method_get tool."""
    response_data: dict[str, Any] = {"id": 123, "type": "credit_card"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_payment_method.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_get(
            {"payment_method_id": 123}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_account_payment_method.assert_awaited_once_with(123)


async def test_handle_linode_account_payment_method_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test payment method get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_payment_method.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_get(
            {"payment_method_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve Linode account payment method" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_oauth_client_thumbnail_get_tool() -> None:
    """Test linode_account_oauth_client_thumbnail_get tool schema."""
    tool, capability = create_linode_account_oauth_client_thumbnail_get_tool()

    assert tool.name == "linode_account_oauth_client_thumbnail_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["client_id"]
    assert tool.inputSchema["properties"]["client_id"]["type"] == "string"


async def test_handle_linode_account_oauth_client_thumbnail_get_requires_client_id(
    sample_config: Config,
) -> None:
    """OAuth client thumbnail retrieval requires client_id."""
    result = await handle_linode_account_oauth_client_thumbnail_get({}, sample_config)

    assert len(result) == 1
    assert "client_id" in result[0].text


async def test_handle_linode_account_oauth_client_thumbnail_get_rejects_bad_client_id(
    sample_config: Config,
) -> None:
    """OAuth client thumbnail retrieval rejects malformed client IDs."""
    for bad_client_id in (123, "   ", "client/id", "client?id", ".."):
        result = await handle_linode_account_oauth_client_thumbnail_get(
            {"client_id": bad_client_id}, sample_config
        )

        assert len(result) == 1
        assert "client_id" in result[0].text


async def test_handle_linode_account_oauth_client_thumbnail_get(
    sample_config: Config,
) -> None:
    """Test linode_account_oauth_client_thumbnail_get tool."""
    response_data: dict[str, Any] = {
        "content_type": "image/png",
        "encoding": "base64",
        "data": "iVBORw0KGgo=",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client_thumbnail.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_thumbnail_get(
            {"client_id": "client-123"}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.get_account_oauth_client_thumbnail.assert_awaited_once_with(
            "client-123"
        )


async def test_handle_linode_account_oauth_client_thumbnail_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test OAuth client thumbnail get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client_thumbnail.side_effect = RuntimeError(
            "boom"
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_thumbnail_get(
            {"client_id": "client-123"}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve Linode account OAuth client thumbnail" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_oauth_client_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test OAuth client get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_get(
            {"client_id": "client-123"}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve Linode account OAuth client" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_support_ticket_replies_list_tool() -> None:
    """Test linode_account_support_ticket_replies_list tool schema."""
    tool, capability = create_linode_account_support_ticket_replies_list_tool()

    assert tool.name == "linode_account_support_ticket_replies_list"
    assert capability is Capability.Read
    assert "ticket_id" in tool.inputSchema["required"]
    assert "page" not in tool.inputSchema["required"]
    assert "page_size" not in tool.inputSchema["required"]


async def test_handle_linode_account_support_ticket_replies_list_requires_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket reply listing requires a positive ticket_id."""
    result = await handle_linode_account_support_ticket_replies_list({}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_rejects_bad_id(
    sample_config: Config,
) -> None:
    """Support ticket reply listing rejects invalid ticket IDs."""
    result = await handle_linode_account_support_ticket_replies_list(
        {"ticket_id": 0}, sample_config
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Support ticket reply listing validates page_size."""
    result = await handle_linode_account_support_ticket_replies_list(
        {"ticket_id": 123, "page_size": 10}, sample_config
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list(
    sample_config: Config,
) -> None:
    """Test linode_account_support_ticket_replies_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 456, "description": "Thanks"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_ticket_replies.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_replies_list(
            {"ticket_id": 123, "page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == response_data
        mock_client.list_support_ticket_replies.assert_awaited_once_with(
            123, page=2, page_size=25
        )


async def test_create_linode_account_event_get_tool() -> None:
    """Test account event get tool schema."""
    tool, capability = create_linode_account_event_get_tool()

    assert tool.name == "linode_account_event_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["event_id"]


async def test_create_linode_account_invoice_items_list_tool() -> None:
    """Test linode_account_invoice_items_list tool schema."""
    tool, capability = create_linode_account_invoice_items_list_tool()

    assert tool.name == "linode_account_invoice_items_list"
    assert capability is Capability.Read
    assert tool.inputSchema.get("required") == ["invoice_id"]
    properties = tool.inputSchema.get("properties", {})
    assert properties["invoice_id"]["minimum"] == 1
    assert properties["page_size"]["maximum"] == 500


async def test_handle_linode_account_invoice_items_list(sample_config: Config) -> None:
    """Test linode_account_invoice_items_list tool."""
    response_data = {
        "data": [{"label": "Compute Instance", "amount": 12.34}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_invoice_items.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_invoice_items_list(
            {"invoice_id": 123, "page": 2, "page_size": 25}, sample_config
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_invoice_items.assert_awaited_once_with(
        123, page=2, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "invoice_id must be a positive integer"),
        ({"invoice_id": 0}, "invoice_id must be a positive integer"),
        ({"invoice_id": False}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123/456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123?456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": ".."}, "invoice_id must be a positive integer"),
        ({"invoice_id": 123, "page": True}, "page must be an integer"),
        ({"invoice_id": 123, "page_size": 501}, "page_size must be at most 500"),
    ],
)
async def test_handle_linode_account_invoice_items_list_rejects_invalid_arguments(
    arguments: dict[str, Any], expected_error: str, sample_config: Config
) -> None:
    """Account invoice items list validates arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_invoice_items_list(
            arguments, sample_config
        )

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_invoice_items_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Account invoice items list reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_invoice_items.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_invoice_items_list(
            {"invoice_id": 123}, sample_config
        )

    assert "boom" in result[0].text


async def test_handle_linode_account_event_get(sample_config: Config) -> None:
    """Test account event get handler."""
    response_data: dict[str, Any] = {
        "id": 123,
        "action": "linode_create",
        "status": "finished",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_event.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_event_get({"event_id": 123}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.get_account_event.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    "event_id", [None, 0, -1, "123", "1/2", "1?x", "..", True, 1.5]
)
async def test_handle_linode_account_event_get_validates_event_id(
    sample_config: Config, event_id: Any
) -> None:
    """Account event get validates event_id before client calls."""
    arguments: dict[str, Any] = {}
    if event_id is not None:
        arguments["event_id"] = event_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_event_get(arguments, sample_config)

    assert len(result) == 1
    assert "event_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_event_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Account event get reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_event.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_event_get({"event_id": 123}, sample_config)

    assert len(result) == 1
    assert "Failed to get Linode account event" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support ticket replies list handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_ticket_replies.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_replies_list(
            {"ticket_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to list Linode support ticket replies" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_support_ticket_close_tool() -> None:
    """Test support ticket close tool schema."""
    tool, capability = create_linode_account_support_ticket_close_tool()

    assert tool.name == "linode_account_support_ticket_close"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["ticket_id", "confirm"]


async def test_handle_linode_account_support_ticket_close_requires_confirm(
    sample_config: Config,
) -> None:
    """Support ticket close requires confirmation."""
    result = await handle_linode_account_support_ticket_close(
        {"ticket_id": 123}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_support_ticket_close_validates_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket close validates ticket_id."""
    result = await handle_linode_account_support_ticket_close(
        {"confirm": True, "ticket_id": 0},
        sample_config,
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_close(
    sample_config: Config,
) -> None:
    """Test support ticket close handler."""
    response_data: dict[str, Any] = {"id": 123, "status": "closed"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.close_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_close(
            {"confirm": True, "ticket_id": 123},
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Support ticket closed successfully",
            "ticket": response_data,
        }
        mock_client.close_support_ticket.assert_awaited_once_with(123)


async def test_handle_linode_account_support_ticket_close_reports_client_errors(
    sample_config: Config,
) -> None:
    """Support ticket close reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.close_support_ticket.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_close(
            {"confirm": True, "ticket_id": 123},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed to close Linode support ticket" in result[0].text
        assert "boom" in result[0].text


async def test_account_support_ticket_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_get_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_get" in tools_mod.__all__

    from linodemcp.server import get_tool_registry

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_account_support_ticket_get"].capability is Capability.Read


async def test_account_support_ticket_close_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket close tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_account_support_ticket_close_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_close" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_account_support_ticket_close"].capability is Capability.Write
    )


async def test_create_linode_account_support_ticket_reply_create_tool() -> None:
    """Test support ticket reply create tool schema."""
    tool, capability = create_linode_account_support_ticket_reply_create_tool()

    assert tool.name == "linode_account_support_ticket_reply_create"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "ticket_id",
        "description",
        "confirm",
    ]


async def test_handle_linode_account_support_ticket_reply_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Support ticket reply creation requires confirmation."""
    result = await handle_linode_account_support_ticket_reply_create(
        {"ticket_id": 123, "description": "Thanks"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_support_ticket_reply_create_validates_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket reply creation validates ticket_id."""
    result = await handle_linode_account_support_ticket_reply_create(
        {"confirm": True, "ticket_id": 0, "description": "Thanks"},
        sample_config,
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_reply_create_requires_description(
    sample_config: Config,
) -> None:
    """Support ticket reply creation requires a non-empty description."""
    result = await handle_linode_account_support_ticket_reply_create(
        {"confirm": True, "ticket_id": 123, "description": "   "},
        sample_config,
    )

    assert len(result) == 1
    assert "description" in result[0].text


async def test_handle_linode_account_support_ticket_reply_create(
    sample_config: Config,
) -> None:
    """Test support ticket reply create handler."""
    response_data: dict[str, Any] = {"id": 456, "description": "Thanks"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket_reply.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_reply_create(
            {"confirm": True, "ticket_id": 123, "description": " Thanks "},
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Support ticket reply created successfully",
            "reply": response_data,
        }
        mock_client.create_support_ticket_reply.assert_awaited_once_with(123, "Thanks")


async def test_handle_linode_account_support_ticket_reply_create_reports_client_errors(
    sample_config: Config,
) -> None:
    """Support ticket reply creation reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket_reply.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_reply_create(
            {"confirm": True, "ticket_id": 123, "description": "Thanks"},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed to create Linode support ticket reply" in result[0].text


async def test_account_support_ticket_reply_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket reply create tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_account_support_ticket_reply_create_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_reply_create" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_account_support_ticket_reply_create"].capability
        is Capability.Write
    )


async def test_create_linode_account_support_ticket_attachment_create_tool() -> None:
    """Test support ticket attachment create tool schema."""
    tool, capability = create_linode_account_support_ticket_attachment_create_tool()

    assert tool.name == "linode_account_support_ticket_attachment_create"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["ticket_id", "file", "confirm"]


async def test_handle_linode_account_support_ticket_attachment_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation requires confirmation."""
    result = await handle_linode_account_support_ticket_attachment_create(
        {"ticket_id": 123, "file": "/Users/e/a.txt"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_support_ticket_attachment_validates_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation validates ticket_id."""
    result = await handle_linode_account_support_ticket_attachment_create(
        {"confirm": True, "ticket_id": 0, "file": "/Users/e/a.txt"},
        sample_config,
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_attachment_create_requires_file(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation requires a non-empty file."""
    result = await handle_linode_account_support_ticket_attachment_create(
        {"confirm": True, "ticket_id": 123, "file": "   "},
        sample_config,
    )

    assert len(result) == 1
    assert "file" in result[0].text


async def test_handle_support_ticket_attachment_requires_absolute_file(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation requires an absolute file path."""
    result = await handle_linode_account_support_ticket_attachment_create(
        {"confirm": True, "ticket_id": 123, "file": "attachment.txt"},
        sample_config,
    )

    assert len(result) == 1
    assert "absolute path" in result[0].text


async def test_handle_support_ticket_attachment_requires_file_key(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation requires the file key."""
    result = await handle_linode_account_support_ticket_attachment_create(
        {"confirm": True, "ticket_id": 123},
        sample_config,
    )

    assert len(result) == 1
    assert "file" in result[0].text


async def test_handle_linode_account_support_ticket_attachment_create(
    sample_config: Config,
) -> None:
    """Test support ticket attachment create handler."""
    response_data: dict[str, Any] = {"id": 789, "file": "attachment.txt"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket_attachment.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_attachment_create(
            {"confirm": True, "ticket_id": 123, "file": " /Users/e/a.txt "},
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "message": "Support ticket attachment created successfully",
            "attachment": response_data,
        }
        mock_client.create_support_ticket_attachment.assert_awaited_once_with(
            123, "/Users/e/a.txt"
        )


async def test_handle_support_ticket_attachment_reports_client_errors(
    sample_config: Config,
) -> None:
    """Support ticket attachment creation reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket_attachment.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_support_ticket_attachment_create(
            {"confirm": True, "ticket_id": 123, "file": "/Users/e/a.txt"},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed to create Linode support ticket attachment" in result[0].text


async def test_account_support_ticket_attachment_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket attachment create tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert (
        "create_linode_account_support_ticket_attachment_create_tool"
        in tools_mod.__all__
    )
    assert "handle_linode_account_support_ticket_attachment_create" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_account_support_ticket_attachment_create"].capability
        is Capability.Write
    )


async def test_create_linode_account_tag_delete_tool() -> None:
    """Test linode_account_tag_delete tool schema."""
    tool, capability = create_linode_account_tag_delete_tool()

    assert tool.name == "linode_account_tag_delete"
    assert capability is Capability.Destroy
    assert "tag_label" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]


async def test_handle_linode_account_tag_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Tag delete requires confirmation."""
    result = await handle_linode_account_tag_delete(
        {"tag_label": "obsolete"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_tag_delete_requires_label(
    sample_config: Config,
) -> None:
    """Tag delete requires a non-empty tag label."""
    result = await handle_linode_account_tag_delete({"confirm": True}, sample_config)

    assert len(result) == 1
    assert "tag_label" in result[0].text


async def test_handle_linode_account_tag_delete_rejects_blank_label(
    sample_config: Config,
) -> None:
    """Tag delete rejects a blank tag label."""
    result = await handle_linode_account_tag_delete(
        {"tag_label": "   ", "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "tag_label" in result[0].text


async def test_handle_linode_account_tag_delete(sample_config: Config) -> None:
    """Test linode_account_tag_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_tag.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_tag_delete(
            {"tag_label": "obsolete", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted successfully" in result[0].text
        mock_client.delete_tag.assert_awaited_once_with("obsolete")


async def test_create_linode_regions_get_tool() -> None:
    """Region get tool is read-only and requires region_id."""
    tool, capability = create_linode_regions_get_tool()

    assert tool.name == "linode_regions_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["region_id"]


async def test_linode_regions_get_tool_is_exported_and_registered() -> None:
    """Region get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_regions_get_tool" in tools_mod.__all__
    assert "handle_linode_regions_get" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_regions_get"].capability is Capability.Read


async def test_handle_linode_regions_get(sample_config: Config) -> None:
    """Test linode_regions_get tool."""
    region = Region(
        id="us-east",
        label="Newark, NJ",
        country="us",
        capabilities=["Linodes", "Block Storage"],
        status="ok",
        resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
        site_type="core",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_region.return_value = region
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == "us-east"
        assert data["label"] == "Newark, NJ"
        assert data["resolvers"] == {
            "ipv4": "192.0.2.1",
            "ipv6": "2001:db8::1",
        }
        mock_client.get_region.assert_awaited_once_with("us-east")


async def test_handle_linode_regions_get_rejects_malformed_region_id(
    sample_config: Config,
) -> None:
    """Region get rejects separators in region_id."""
    for region_id in ("us/east", "us-east?x=1", "../us-east"):
        result = await handle_linode_regions_get(
            {"region_id": region_id}, sample_config
        )

        assert len(result) == 1
        assert "letters, numbers, and hyphens" in result[0].text


async def test_handle_linode_regions_get_requires_region_id(
    sample_config: Config,
) -> None:
    """Region get requires region_id."""
    result = await handle_linode_regions_get({}, sample_config)

    assert len(result) == 1
    assert "region_id is required" in result[0].text


async def test_handle_linode_regions_get_error(sample_config: Config) -> None:
    """Test linode_regions_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_region.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_create_linode_regions_availability_list_tool() -> None:
    """Regions availability list tool is read-only and has no route inputs."""
    tool, capability = create_linode_regions_availability_list_tool()

    assert tool.name == "linode_regions_availability_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema


async def test_handle_linode_regions_availability_list(sample_config: Config) -> None:
    """Test linode_regions_availability_list tool."""
    availability = [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"},
        {"available": False, "plan": "g6-standard-2", "region": "us-west"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions_availability.return_value = availability
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_availability_list({}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["count"] == 2
        assert data["availability"] == availability
        mock_client.list_regions_availability.assert_awaited_once_with()


async def test_handle_linode_regions_availability_list_error(
    sample_config: Config,
) -> None:
    """Test linode_regions_availability_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions_availability.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_availability_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_create_linode_regions_availability_get_tool() -> None:
    """Region availability tool is read-only and requires region_id."""
    tool, capability = create_linode_regions_availability_get_tool()

    assert tool.name == "linode_regions_availability_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["region_id"]


async def test_handle_linode_regions_availability_get(sample_config: Config) -> None:
    """Test linode_regions_availability_get tool."""
    availability = [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"},
        {"available": False, "plan": "g6-standard-2", "region": "us-east"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_region_availability.return_value = availability
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_availability_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["region_id"] == "us-east"
        assert data["count"] == 2
        assert data["availability"] == availability
        mock_client.get_region_availability.assert_awaited_once_with("us-east")


async def test_handle_linode_regions_availability_get_rejects_malformed_region_id(
    sample_config: Config,
) -> None:
    """Region availability rejects separators in region_id."""
    for region_id in ("us/east", "us-east?x=1", "../us-east"):
        result = await handle_linode_regions_availability_get(
            {"region_id": region_id}, sample_config
        )

        assert len(result) == 1
        assert "letters, numbers, and hyphens" in result[0].text


async def test_handle_linode_regions_availability_get_requires_region_id(
    sample_config: Config,
) -> None:
    """Region availability requires region_id."""
    result = await handle_linode_regions_availability_get({}, sample_config)

    assert len(result) == 1
    assert "region_id is required" in result[0].text


async def test_handle_linode_regions_availability_get_error(
    sample_config: Config,
) -> None:
    """Test linode_regions_availability_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_region_availability.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_availability_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_regions_list(sample_config: Config) -> None:
    """Test linode_regions_list tool."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes", "Block Storage"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({}, sample_config)

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "eu-west" in result[0].text
        mock_client.list_regions.assert_called_once()


async def test_handle_linode_regions_list_filter_country(sample_config: Config) -> None:
    """Test linode_regions_list tool with country filter."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="us-west",
            label="Fremont, CA",
            country="us",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.3", ipv6="2001:db8::3"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({"country": "us"}, sample_config)

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "us-west" in result[0].text
        assert "eu-west" not in result[0].text
        assert '"count": 2' in result[0].text


def test_linode_kernels_list_tool_schema() -> None:
    """The kernels list tool exposes pagination fields."""
    tool, capability = create_linode_kernels_list_tool()
    assert tool.name == "linode_kernels_list"
    assert capability is Capability.Read
    props: dict[str, Any] = tool.inputSchema["properties"]
    assert props["page"]["minimum"] == 1
    assert props["page_size"]["minimum"] == 25
    assert props["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema


async def test_handle_linode_kernels_list(sample_config: Config) -> None:
    """Test linode_kernels_list tool."""
    response = {
        "data": [
            {
                "id": "linode/latest-64bit",
                "label": "Latest 64 bit",
                "version": "6.8.0",
                "architecture": "x86_64",
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_kernels.return_value = response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernels_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["data"][0]["id"] == "linode/latest-64bit"
    assert body["page"] == 2
    mock_client.list_kernels.assert_awaited_once_with(page=2, page_size=25)


@pytest.mark.parametrize(
    "arguments",
    [
        {"page": 0},
        {"page": True},
        {"page_size": 24},
        {"page_size": 501},
        {"page_size": "25"},
    ],
)
async def test_handle_linode_kernels_list_rejects_invalid_pagination(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """Invalid pagination arguments are rejected before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_kernels_list(arguments, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_types_list(sample_config: Config) -> None:
    """Test linode_types_list tool."""
    mock_types = [
        InstanceType(
            id="g6-nanode-1",
            label="Nanode 1GB",
            class_="nanode",
            disk=25600,
            memory=1024,
            vcpus=1,
            gpus=0,
            network_out=1000,
            transfer=1000,
            price=Price(hourly=0.0075, monthly=5.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
            successor=None,
        ),
        InstanceType(
            id="g6-standard-2",
            label="Linode 4GB",
            class_="standard",
            disk=81920,
            memory=4096,
            vcpus=2,
            gpus=0,
            network_out=4000,
            transfer=4000,
            price=Price(hourly=0.03, monthly=20.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.008, monthly=5.0))),
            successor=None,
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({}, sample_config)

        assert len(result) == 1
        assert "g6-nanode-1" in result[0].text
        assert "g6-standard-2" in result[0].text
        mock_client.list_types.assert_called_once()


async def test_handle_linode_types_list_filter_class(sample_config: Config) -> None:
    """Test linode_types_list tool with class filter."""
    mock_types = [
        InstanceType(
            id="g6-nanode-1",
            label="Nanode 1GB",
            class_="nanode",
            disk=25600,
            memory=1024,
            vcpus=1,
            gpus=0,
            network_out=1000,
            transfer=1000,
            price=Price(hourly=0.0075, monthly=5.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
            successor=None,
        ),
        InstanceType(
            id="g6-standard-2",
            label="Linode 4GB",
            class_="standard",
            disk=81920,
            memory=4096,
            vcpus=2,
            gpus=0,
            network_out=4000,
            transfer=4000,
            price=Price(hourly=0.03, monthly=20.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.008, monthly=5.0))),
            successor=None,
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({"class": "standard"}, sample_config)

        assert len(result) == 1
        assert "g6-standard-2" in result[0].text
        assert "g6-nanode-1" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_type_get(sample_config: Config) -> None:
    """Test linode_type_get tool."""
    mock_type = InstanceType(
        id="g6-nanode-1",
        label="Nanode 1GB",
        class_="nanode",
        disk=25600,
        memory=1024,
        vcpus=1,
        gpus=0,
        network_out=1000,
        transfer=1000,
        price=Price(hourly=0.0075, monthly=5.0),
        addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
        successor=None,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_type.return_value = mock_type
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_get({"type_id": "g6-nanode-1"}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == "g6-nanode-1"
        assert data["label"] == "Nanode 1GB"
        assert data["price"] == {"hourly": 0.0075, "monthly": 5.0}
        mock_client.get_type.assert_awaited_once_with("g6-nanode-1")


async def test_handle_linode_type_get_rejects_malformed_type_id(
    sample_config: Config,
) -> None:
    """Type get rejects separators in type_id before client creation."""
    for type_id in ("g6/nanode-1", "g6-nanode-1?x=1", "../g6-nanode-1"):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_type_get({"type_id": type_id}, sample_config)

        assert len(result) == 1
        assert "letters, numbers, and hyphens" in result[0].text
        mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_type_id", [None, "", "   ", 123, True])
async def test_handle_linode_type_get_requires_string_type_id(
    sample_config: Config, bad_type_id: Any
) -> None:
    """Type get requires a non-empty string type_id."""
    arguments = {} if bad_type_id is None else {"type_id": bad_type_id}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_type_get(arguments, sample_config)

    assert len(result) == 1
    assert "type_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_type_get_error(sample_config: Config) -> None:
    """Test linode_type_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_type.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_get({"type_id": "g6-nanode-1"}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve Linode type g6-nanode-1" in result[0].text


async def test_handle_linode_volume_get(sample_config: Config) -> None:
    """Test linode_volume_get tool."""
    mock_volume = Volume(
        id=12345,
        label="data-vol",
        status="active",
        size=100,
        region="us-east",
        linode_id=123,
        linode_label="test-instance",
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-02T00:00:00",
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_get({"volume_id": 12345}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert '"id": 12345' in result[0].text
        mock_client.get_volume.assert_called_once_with(12345)


async def test_handle_linode_volume_get_requires_volume_id(
    sample_config: Config,
) -> None:
    """Test linode_volume_get validates volume_id."""
    result = await handle_linode_volume_get({}, sample_config)

    assert len(result) == 1
    assert "volume_id is required" in result[0].text


async def test_handle_linode_volume_types_list(sample_config: Config) -> None:
    """Test linode_volume_types_list tool."""
    volume_types = [
        {
            "id": "volume",
            "label": "Storage Volume",
            "price": {"hourly": 0.0015, "monthly": 0.10},
            "region_prices": [
                {"id": "us-iad", "hourly": 0.00018, "monthly": 0.12},
            ],
            "transfer": 0,
        }
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volume_types.return_value = volume_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_types_list({}, sample_config)

        assert len(result) == 1
        assert "Storage Volume" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_volume_types.assert_called_once()


async def test_handle_linode_volumes_list(sample_config: Config) -> None:
    """Test linode_volumes_list tool."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="eu-west",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=["backup"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" in result[0].text
        mock_client.list_volumes.assert_called_once()


async def test_handle_linode_volumes_list_filter_region(sample_config: Config) -> None:
    """Test linode_volumes_list tool with region filter."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="eu-west",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({"region": "us-east"}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_create_linode_image_upload_tool_def() -> None:
    """Image upload tool should require label, region, and confirm."""
    tool, capability = create_linode_image_upload_tool()
    assert tool.name == "linode_image_upload"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["label", "region", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_image_upload_success(sample_config: Config) -> None:
    """Image upload tool should call the client and return upload details."""
    upload_response = {
        "image": {"id": "private/98765", "label": "upload-image"},
        "upload_to": "https://uploads.example.invalid/image",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.upload_image.return_value = upload_response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_upload(
            {
                "label": "upload-image",
                "region": "us-east",
                "cloud_init": True,
                "description": "Uploaded image",
                "tags": ["prod"],
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "private/98765" in result[0].text
    mock_client.upload_image.assert_awaited_once_with(
        label="upload-image",
        region="us-east",
        cloud_init=True,
        description="Uploaded image",
        tags=["prod"],
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_image_upload_confirm_required(
    sample_config: Config, confirm: object
) -> None:
    """Image upload should require literal confirm=true before client call."""
    arguments: dict[str, object] = {"label": "upload-image", "region": "us-east"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_image_upload(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"region": "us-east", "confirm": True},
            "label must be a non-empty string",
        ),
        (
            {"label": "upload-image", "confirm": True},
            "region must be a non-empty string",
        ),
        (
            {
                "label": "upload-image",
                "region": "us-east",
                "cloud_init": "yes",
                "confirm": True,
            },
            "cloud_init must be a boolean",
        ),
        (
            {
                "label": "upload-image",
                "region": "us-east",
                "tags": ["prod", ""],
                "confirm": True,
            },
            "tags must contain non-empty strings",
        ),
    ],
)
async def test_handle_linode_image_upload_validation_errors(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Image upload should validate required and optional body fields."""
    result = await handle_linode_image_upload(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text


async def test_image_upload_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the upload with request body and no call."""
    result = await handle_linode_image_upload(
        {
            "label": "upload-image",
            "region": "us-east",
            "description": "Uploaded image",
            "tags": ["prod"],
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_image_upload"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/images/upload"
    assert body["would_execute"]["body"] == {
        "label": "upload-image",
        "region": "us-east",
        "description": "Uploaded image",
        "tags": ["prod"],
    }
    assert "confirm=true" not in result[0].text


async def test_create_linode_image_update_tool_def() -> None:
    """Image update tool should require image_id and confirm."""
    tool, capability = create_linode_image_update_tool()
    assert tool.name == "linode_image_update"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["image_id", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_image_update_success(sample_config: Config) -> None:
    """Image update tool should call the client and return image details."""
    mock_image = Image(
        id="private/12345",
        label="renamed-image",
        description="Updated image",
        type="manual",
        is_public=False,
        deprecated=False,
        size=2048,
        vendor="",
        status="available",
        created="2024-01-01T00:00:00",
        created_by="testuser",
        expiry=None,
        eol=None,
        capabilities=["cloud-init"],
        tags=["prod"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_image.return_value = mock_image
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_update(
            {
                "image_id": "private/12345",
                "label": "renamed-image",
                "description": "Updated image",
                "tags": ["prod"],
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "private/12345" in result[0].text
    mock_client.update_image.assert_awaited_once_with(
        image_id="private/12345",
        label="renamed-image",
        description="Updated image",
        tags=["prod"],
    )


async def test_handle_linode_image_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """Image update dry-run should preview the PUT without requiring confirm."""
    result = await handle_linode_image_update(
        {"image_id": "private/12345", "label": "renamed", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_image_update"
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == "/images/private%2F12345"
    assert body["would_execute"]["body"] == {"label": "renamed"}
    assert "confirm=true" not in result[0].text


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_image_update_confirm_required(
    sample_config: Config, confirm: object
) -> None:
    """Image update should require literal confirm=true before client call."""
    arguments: dict[str, object] = {"image_id": "private/12345", "label": "renamed"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_image_update(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("image_id", "message"),
    [
        ("", "image_id must be a non-empty string"),
        ("private/12345?x=1", "image_id must not contain"),
        ("../private/12345", "image_id must not contain"),
        ("private/123/extra", "image_id must match private/<numeric_id>"),
        ("private//123", "image_id must match private/<numeric_id>"),
        ("/private/123", "image_id must match private/<numeric_id>"),
    ],
)
async def test_handle_linode_image_update_rejects_bad_image_ids(
    sample_config: Config, image_id: str, message: str
) -> None:
    """Image update rejects malformed image IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_image_update(
            {"image_id": image_id, "label": "renamed", "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"image_id": "private/12345", "confirm": True}, "at least one"),
        ({"image_id": "private/12345", "label": "", "confirm": True}, "label"),
        (
            {"image_id": "private/12345", "description": 123, "confirm": True},
            "description",
        ),
        ({"image_id": "private/12345", "tags": "prod", "confirm": True}, "tags"),
    ],
)
async def test_handle_linode_image_update_validation_errors(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Image update validates writable request fields before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_image_update(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_kernel_get_tool_def() -> None:
    """Kernel get tool should require kernel_id."""
    tool, capability = create_linode_kernel_get_tool()
    assert tool.name == "linode_kernel_get"
    assert capability.name == "Read"
    assert tool.inputSchema["required"] == ["kernel_id"]
    assert tool.inputSchema["properties"]["kernel_id"]["pattern"] == (
        r"^(?!.*\.\.)linode/[A-Za-z0-9._-]+$"
    )


async def test_handle_linode_kernel_get_success(sample_config: Config) -> None:
    """Kernel get should return a single kernel."""
    kernel = {
        "id": "linode/latest-64bit",
        "label": "Latest 64 bit",
        "version": "6.6.0",
        "architecture": "x86_64",
        "kvm": True,
        "xen": False,
        "pvops": False,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_kernel.return_value = kernel
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernel_get(
            {"kernel_id": "linode/latest-64bit"},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["kernel"] == kernel
        mock_client.get_kernel.assert_awaited_once_with("linode/latest-64bit")


@pytest.mark.parametrize(
    "kernel_id",
    [
        "linode/latest-64bit",
        "linode/grub2",
        "linode/6.12.1-x86_64",
    ],
)
async def test_handle_linode_kernel_get_accepts_valid_kernel_ids(
    sample_config: Config, kernel_id: str
) -> None:
    """Kernel get should accept documented linode/<slug> kernel ID shapes."""
    kernel = {"id": kernel_id, "label": "Kernel"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_kernel.return_value = kernel
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernel_get(
            {"kernel_id": kernel_id},
            sample_config,
        )

    assert json.loads(result[0].text)["kernel"]["id"] == kernel_id
    mock_client.get_kernel.assert_awaited_once_with(kernel_id)


@pytest.mark.parametrize(
    "bad_kernel_id",
    [
        None,
        "",
        "/",
        "linode/..",
        "linode/../x",
        "linode/latest?x=1",
        "linode/latest%3Fx=1",
        "linode/latest%2Fextra",
        "private/latest-64bit",
        "linode/latest/extra",
    ],
)
async def test_handle_linode_kernel_get_rejects_malformed_kernel_id(
    sample_config: Config, bad_kernel_id: object
) -> None:
    """Kernel get should reject malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_kernel_get(
            {"kernel_id": bad_kernel_id}, sample_config
        )

    assert len(result) == 1
    assert "kernel_id" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_image_get_tool_def() -> None:
    """Image get tool should require image_id."""
    tool, capability = create_linode_image_get_tool()
    assert tool.name == "linode_image_get"
    assert capability.name == "Read"
    assert tool.inputSchema["required"] == ["image_id"]
    assert tool.inputSchema["properties"]["image_id"]["pattern"] == (
        r"^(?!.*\.\.)(linode|private)/[A-Za-z0-9._-]+$"
    )


async def test_handle_linode_image_get_success(sample_config: Config) -> None:
    """Image get should return a single image."""
    mock_image = Image(
        id="linode/ubuntu24.04",
        label="Ubuntu 24.04 LTS",
        description="Ubuntu image",
        type="manual",
        is_public=True,
        deprecated=False,
        size=2500,
        vendor="Ubuntu",
        status="available",
        created="2024-04-25T00:00:00",
        created_by="linode",
        expiry=None,
        eol=None,
        capabilities=["cloud-init"],
        tags=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_image.return_value = mock_image
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_get(
            {"image_id": "linode/ubuntu24.04"},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["image"]["id"] == "linode/ubuntu24.04"
        assert body["image"]["label"] == "Ubuntu 24.04 LTS"
        mock_client.get_image.assert_awaited_once_with("linode/ubuntu24.04")


@pytest.mark.parametrize(
    "bad_image_id",
    [
        None,
        "",
        "/",
        "linode/..",
        "linode/../x",
        "private/v2..backup",
        "linode/ubuntu?x=1",
        "linode/ubuntu/extra",
    ],
)
async def test_handle_linode_image_get_rejects_malformed_image_id(
    sample_config: Config, bad_image_id: object
) -> None:
    """Image get should reject malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_image_get(
            {"image_id": bad_image_id}, sample_config
        )

    assert len(result) == 1
    assert "image_id" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_image_create_tool_def() -> None:
    """Image create tool should require disk_id and confirm."""
    tool, capability = create_linode_image_create_tool()
    assert tool.name == "linode_image_create"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["disk_id", "confirm"]


async def test_handle_linode_image_create_success(sample_config: Config) -> None:
    """Test linode_image_create tool."""
    mock_image = Image(
        id="private/12345",
        label="app-image",
        description="Application image",
        type="manual",
        is_public=False,
        deprecated=False,
        size=2048,
        vendor="",
        status="creating",
        created="2024-01-01T00:00:00",
        created_by="testuser",
        expiry=None,
        eol=None,
        capabilities=["cloud-init"],
        tags=["prod"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_image.return_value = mock_image
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_create(
            {
                "disk_id": 123,
                "label": "app-image",
                "description": "Application image",
                "cloud_init": True,
                "tags": ["prod"],
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "private/12345" in result[0].text
        mock_client.create_image.assert_awaited_once_with(
            disk_id=123,
            label="app-image",
            description="Application image",
            cloud_init=True,
            tags=["prod"],
        )


async def test_handle_linode_image_create_confirm_required(
    sample_config: Config,
) -> None:
    """Image create should require confirm=true."""
    result = await handle_linode_image_create({"disk_id": 123}, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_image_create_invalid_tags(sample_config: Config) -> None:
    """Image create should validate tags."""
    result = await handle_linode_image_create(
        {"disk_id": 123, "tags": ["prod", ""], "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "tags" in result[0].text


async def test_image_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_image_create(
        {"disk_id": 123, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_image_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/images"
    assert body["current_state"] is None
    assert any("123" in s for s in body["side_effects"])
    assert "confirm=true" not in result[0].text


async def test_image_create_dry_run_still_validates_disk_id(
    sample_config: Config,
) -> None:
    """Missing/invalid disk_id must error out regardless of dry_run."""
    result = await handle_linode_image_create({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "disk_id must be a positive integer" in result[0].text


async def test_create_linode_images_sharegroups_token_update_tool_def() -> None:
    """Image share group token update tool should require UUID, label, and confirm."""
    tool, capability = create_linode_images_sharegroups_token_update_tool()

    assert tool.name == "linode_images_sharegroups_token_update"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["token_uuid", "label", "confirm"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_images_sharegroups_token_update_success(
    sample_config: Config,
) -> None:
    """Image share group token update should call the client once."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_image_sharegroup_token.return_value = {
            "id": "sharegroup-record-1",
            "label": "renamed-token",
            "token_uuid": "11111111-1111-4111-8111-111111111111",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_sharegroups_token_update(
            {
                "token_uuid": "11111111-1111-4111-8111-111111111111",
                "label": "renamed-token",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "renamed-token" in result[0].text
        mock_client.update_image_sharegroup_token.assert_awaited_once_with(
            token_uuid="11111111-1111-4111-8111-111111111111",
            label="renamed-token",
        )


async def test_create_linode_images_sharegroups_token_create_tool_def() -> None:
    """Image share group token create tool should require UUID and confirm."""
    tool, capability = create_linode_images_sharegroups_token_create_tool()

    assert tool.name == "linode_images_sharegroups_token_create"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["valid_for_sharegroup_uuid", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_images_sharegroups_token_create_success(
    sample_config: Config,
) -> None:
    """Image share group token create should call the client once."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_image_sharegroup_token.return_value = {
            "id": "sharegroup-record-1",
            "label": "partner-token",
            "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_sharegroups_token_create(
            {
                "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
                "label": "partner-token",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "sharegroup-record-1" in result[0].text
        mock_client.create_image_sharegroup_token.assert_awaited_once_with(
            valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111",
            label="partner-token",
        )


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroups_token_create_requires_true_confirm(
    sample_config: Config, bad_confirm: object
) -> None:
    """Image share group token create rejects non-true confirm before the client."""
    arguments: dict[str, Any] = {
        "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111"
    }
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_images_sharegroups_token_create(
            arguments, sample_config
        )

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "bad_uuid", [None, "", "   ", "not-a-uuid", "../", "uuid?x=1", 123, True]
)
async def test_handle_linode_images_sharegroups_token_create_validates_uuid(
    sample_config: Config, bad_uuid: object
) -> None:
    """Image share group token create requires the documented UUID body field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_images_sharegroups_token_create(
            {"valid_for_sharegroup_uuid": bad_uuid, "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "valid_for_sharegroup_uuid" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_label", ["", "   ", 123, True])
async def test_handle_linode_images_sharegroups_token_create_validates_label(
    sample_config: Config, bad_label: object
) -> None:
    """Image share group token create rejects malformed optional labels."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_images_sharegroups_token_create(
            {
                "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
                "label": bad_label,
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "label" in result[0].text
    mock_client_class.assert_not_called()


async def test_image_sharegroup_token_create_dry_run_previews_without_confirm(
    sample_config: Config,
) -> None:
    """Dry-run previews without requiring the confirm gate."""
    result = await handle_linode_images_sharegroups_token_create(
        {
            "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert '"dry_run": true' in result[0].text


async def test_image_sharegroup_token_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews token creation without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_images_sharegroups_token_create(
            {
                "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
                "label": "partner-token",
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_images_sharegroups_token_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/images/sharegroups/tokens"
    assert body["would_execute"]["body"] == {
        "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
        "label": "partner-token",
    }
    mock_client_class.assert_not_called()


async def test_handle_linode_images_list(sample_config: Config) -> None:
    """Test linode_images_list tool."""
    mock_images = [
        Image(
            id="linode/ubuntu22.04",
            label="Ubuntu 22.04",
            description="Ubuntu 22.04 LTS",
            type="manual",
            is_public=True,
            deprecated=False,
            size=2500,
            vendor="linode",
            status="available",
            created="2022-04-21T00:00:00",
            created_by="linode",
            expiry=None,
            eol=None,
            capabilities=["cloud-init"],
            tags=[],
        ),
        Image(
            id="private/12345",
            label="Custom Image",
            description="My custom image",
            type="manual",
            is_public=False,
            deprecated=False,
            size=5000,
            vendor="",
            status="available",
            created="2024-01-01T00:00:00",
            created_by="user@example.com",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=["custom"],
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.return_value = mock_images
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list({}, sample_config)

        assert len(result) == 1
        assert "linode/ubuntu22.04" in result[0].text
        assert "private/12345" in result[0].text
        mock_client.list_images.assert_called_once()


async def test_handle_linode_images_list_filter_public(sample_config: Config) -> None:
    """Test linode_images_list tool with is_public filter."""
    mock_images = [
        Image(
            id="linode/ubuntu22.04",
            label="Ubuntu 22.04",
            description="Ubuntu 22.04 LTS",
            type="manual",
            is_public=True,
            deprecated=False,
            size=2500,
            vendor="linode",
            status="available",
            created="2022-04-21T00:00:00",
            created_by="linode",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=[],
        ),
        Image(
            id="private/12345",
            label="Custom Image",
            description="My custom image",
            type="manual",
            is_public=False,
            deprecated=False,
            size=5000,
            vendor="",
            status="available",
            created="2024-01-01T00:00:00",
            created_by="user@example.com",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=[],
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.return_value = mock_images
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list({"is_public": "false"}, sample_config)

        assert len(result) == 1
        assert "private/12345" in result[0].text
        assert "linode/ubuntu22.04" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_account_error(sample_config: Config) -> None:
    """Test linode_account tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_regions_list_error(sample_config: Config) -> None:
    """Test linode_regions_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_types_list_error(sample_config: Config) -> None:
    """Test linode_types_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_error(sample_config: Config) -> None:
    """Test linode_volumes_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_images_list_error(sample_config: Config) -> None:
    """Test linode_images_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_instance_get_error(sample_config: Config) -> None:
    """Test linode_instance_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": "123456"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_filter_label(sample_config: Config) -> None:
    """Test linode_volumes_list tool with label filter."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="us-east",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=3,
            label="data-backup",
            status="active",
            size=75,
            region="us-east",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-backup",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list(
            {"label_contains": "backup"}, sample_config
        )

        assert len(result) == 1
        assert "backup-vol" in result[0].text
        assert "data-backup" in result[0].text
        assert '"count": 2' in result[0].text


async def test_handle_linode_regions_list_filter_capability(
    sample_config: Config,
) -> None:
    """Test linode_regions_list tool with capability filter."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes", "Block Storage"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list(
            {"capability": "Block Storage"}, sample_config
        )

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "eu-west" not in result[0].text
        assert '"count": 1' in result[0].text


# Stage 3 Tool Tests


async def test_handle_linode_sshkeys_list(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool."""
    mock_keys = [
        SSHKey(
            id=1,
            label="work-laptop",
            ssh_key="ssh-rsa AAAA... user@work",
            created="2024-01-01T00:00:00",
        ),
        SSHKey(
            id=2,
            label="home-desktop",
            ssh_key="ssh-rsa BBBB... user@home",
            created="2024-01-02T00:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list({}, sample_config)

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" in result[0].text
        mock_client.list_ssh_keys.assert_called_once()


async def test_handle_linode_sshkey_get(sample_config: Config) -> None:
    """Test linode_sshkey_get tool."""
    mock_key = SSHKey(
        id=12345,
        label="work-laptop",
        ssh_key="ssh-rsa AAAA... user@work",
        created="2024-01-01T00:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_ssh_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_get({"ssh_key_id": 12345}, sample_config)

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "12345" in result[0].text
        mock_client.get_ssh_key.assert_called_once_with(12345)


async def test_handle_linode_sshkey_get_requires_id(sample_config: Config) -> None:
    """Test linode_sshkey_get requires ssh_key_id."""
    result = await handle_linode_sshkey_get({}, sample_config)

    assert len(result) == 1
    assert "ssh_key_id is required" in result[0].text


async def test_handle_linode_sshkeys_list_filter_label(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool with label filter."""
    mock_keys = [
        SSHKey(
            id=1,
            label="work-laptop",
            ssh_key="ssh-rsa AAAA... user@work",
            created="2024-01-01T00:00:00",
        ),
        SSHKey(
            id=2,
            label="home-desktop",
            ssh_key="ssh-rsa BBBB... user@home",
            created="2024-01-02T00:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list(
            {"label_contains": "work"}, sample_config
        )

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" not in result[0].text


async def test_handle_linode_sshkeys_list_error(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domains_list(sample_config: Config) -> None:
    """Test linode_domains_list tool."""
    mock_domains = [
        Domain(
            id=1,
            domain="example.com",
            type="master",
            status="active",
            soa_email="admin@example.com",
            description="Main domain",
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        Domain(
            id=2,
            domain="test.com",
            type="master",
            status="active",
            soa_email="admin@test.com",
            description="Test domain",
            tags=["staging"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domains.return_value = mock_domains
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domains_list({}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        assert "test.com" in result[0].text
        mock_client.list_domains.assert_called_once()


async def test_handle_linode_domains_list_error(sample_config: Config) -> None:
    """Test linode_domains_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domains.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domains_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_get(sample_config: Config) -> None:
    """Test linode_domain_get tool."""
    mock_domain = Domain(
        id=1,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="Main domain",
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        mock_client.get_domain.assert_called_once_with(1)


async def test_handle_linode_domain_get_missing_id(sample_config: Config) -> None:
    """Test linode_domain_get tool with missing ID."""
    result = await handle_linode_domain_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_get_error(sample_config: Config) -> None:
    """Test linode_domain_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_records_list(sample_config: Config) -> None:
    """Test linode_domain_records_list tool."""
    mock_records = [
        DomainRecord(
            id=1,
            type="A",
            name="www",
            target="192.0.2.1",
            priority=0,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        DomainRecord(
            id=2,
            type="MX",
            name="",
            target="mail.example.com",
            priority=10,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.return_value = mock_records
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "mail.example.com" in result[0].text
        mock_client.list_domain_records.assert_called_once_with(1)


async def test_handle_linode_domain_record_get(sample_config: Config) -> None:
    """Test linode_domain_record_get tool."""
    mock_record = DomainRecord(
        id=2,
        type="A",
        name="www",
        target="192.0.2.1",
        priority=0,
        weight=0,
        port=0,
        ttl_sec=300,
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain_record.return_value = mock_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_get(
            {"domain_id": 1, "record_id": 2}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "www" in result[0].text
        mock_client.get_domain_record.assert_called_once_with(1, 2)


async def test_handle_linode_domain_record_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_domain_record_get tool with missing record_id."""
    result = await handle_linode_domain_record_get({"domain_id": 1}, sample_config)

    assert len(result) == 1
    assert "record_id is required" in result[0].text


async def test_handle_linode_domain_records_list_filter_type(
    sample_config: Config,
) -> None:
    """Test linode_domain_records_list tool with type filter."""
    mock_records = [
        DomainRecord(
            id=1,
            type="A",
            name="www",
            target="192.0.2.1",
            priority=0,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        DomainRecord(
            id=2,
            type="MX",
            name="",
            target="mail.example.com",
            priority=10,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.return_value = mock_records
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1, "type": "A"}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "mail.example.com" not in result[0].text


async def test_handle_linode_domain_records_list_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_domain_records_list tool with missing domain_id."""
    result = await handle_linode_domain_records_list({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_records_list_error(sample_config: Config) -> None:
    """Test linode_domain_records_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


def test_create_linode_firewall_get_tool_schema() -> None:
    """Test linode_firewall_get tool schema."""
    tool, capability = create_linode_firewall_get_tool()

    assert tool.name == "linode_firewall_get"
    assert capability is Capability.Read
    assert "firewall_id" in tool.inputSchema["properties"]
    assert "firewall_id" in tool.inputSchema["required"]


def test_create_linode_firewall_rules_get_tool_schema() -> None:
    """Test linode_firewall_rules_get tool schema."""
    tool, capability = create_linode_firewall_rules_get_tool()

    assert tool.name == "linode_firewall_rules_get"
    assert capability is Capability.Read
    assert "firewall_id" in tool.inputSchema["properties"]
    assert "firewall_id" in tool.inputSchema["required"]


async def test_handle_linode_firewall_get(sample_config: Config) -> None:
    """Test linode_firewall_get tool."""
    mock_firewall = Firewall(
        id=12345,
        label="web-firewall",
        status="enabled",
        rules=FirewallRules(
            inbound=[],
            inbound_policy="DROP",
            outbound=[],
            outbound_policy="ACCEPT",
        ),
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_firewall.return_value = mock_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_get({"firewall_id": 12345}, sample_config)

        assert len(result) == 1
        assert "web-firewall" in result[0].text
        mock_client.get_firewall.assert_awaited_once_with(12345)


async def test_handle_linode_firewall_get_missing_id(sample_config: Config) -> None:
    """Test linode_firewall_get validation."""
    result = await handle_linode_firewall_get({}, sample_config)

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_handle_linode_firewall_rules_get(sample_config: Config) -> None:
    """Test linode_firewall_rules_get tool."""
    mock_rules = FirewallRules(
        inbound=[
            FirewallRule(
                action="ACCEPT",
                protocol="TCP",
                ports="22",
                addresses=FirewallAddresses(ipv4=["0.0.0.0/0"], ipv6=["::/0"]),
                label="allow-ssh",
                description="",
            )
        ],
        inbound_policy="DROP",
        outbound=[],
        outbound_policy="ACCEPT",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_firewall_rules.return_value = mock_rules
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_rules_get(
            {"firewall_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "DROP" in result[0].text
        assert "ACCEPT" in result[0].text
        mock_client.get_firewall_rules.assert_awaited_once_with(12345)


async def test_handle_linode_firewall_rules_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_firewall_rules_get validation."""
    result = await handle_linode_firewall_rules_get({}, sample_config)

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_handle_linode_firewalls_list(sample_config: Config) -> None:
    """Test linode_firewalls_list tool."""
    mock_firewalls = [
        Firewall(
            id=1,
            label="web-firewall",
            status="enabled",
            rules=FirewallRules(
                inbound=[
                    FirewallRule(
                        action="ACCEPT",
                        protocol="TCP",
                        ports="80,443",
                        addresses=FirewallAddresses(ipv4=["0.0.0.0/0"], ipv6=["::/0"]),
                        label="HTTP/HTTPS",
                        description="Allow web traffic",
                    )
                ],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list({}, sample_config)

        assert len(result) == 1
        assert "web-firewall" in result[0].text
        mock_client.list_firewalls.assert_called_once()


async def test_handle_linode_firewalls_list_filter_status(
    sample_config: Config,
) -> None:
    """Test linode_firewalls_list tool with status filter."""
    mock_firewalls = [
        Firewall(
            id=1,
            label="enabled-fw",
            status="enabled",
            rules=FirewallRules(
                inbound=[],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        Firewall(
            id=2,
            label="disabled-fw",
            status="disabled",
            rules=FirewallRules(
                inbound=[],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list(
            {"status": "enabled"}, sample_config
        )

        assert len(result) == 1
        assert "enabled-fw" in result[0].text
        assert "disabled-fw" not in result[0].text


async def test_handle_linode_firewalls_list_error(sample_config: Config) -> None:
    """Test linode_firewalls_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancers_list(sample_config: Config) -> None:
    """Test linode_nodebalancers_list tool."""
    mock_nodebalancers = [
        NodeBalancer(
            id=1,
            label="web-lb",
            hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
            ipv4="192.0.2.1",
            ipv6="2001:db8::1",
            region="us-east",
            client_conn_throttle=0,
            transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.return_value = mock_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list({}, sample_config)

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.list_nodebalancers.assert_called_once()


async def test_handle_linode_nodebalancers_list_filter_region(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancers_list tool with region filter."""
    mock_nodebalancers = [
        NodeBalancer(
            id=1,
            label="us-lb",
            hostname="nb-1.newark.nodebalancer.linode.com",
            ipv4="192.0.2.1",
            ipv6="2001:db8::1",
            region="us-east",
            client_conn_throttle=0,
            transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        NodeBalancer(
            id=2,
            label="eu-lb",
            hostname="nb-2.london.nodebalancer.linode.com",
            ipv4="192.0.2.2",
            ipv6="2001:db8::2",
            region="eu-west",
            client_conn_throttle=0,
            transfer=Transfer(in_=500.0, out=1000.0, total=1500.0),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.return_value = mock_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list(
            {"region": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "us-lb" in result[0].text
        assert "eu-lb" not in result[0].text


async def test_handle_linode_nodebalancers_list_error(sample_config: Config) -> None:
    """Test linode_nodebalancers_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_config_get_tool_definition() -> None:
    """Test linode_nodebalancer_config_get tool definition."""
    tool, capability = create_linode_nodebalancer_config_get_tool()
    assert tool.name == "linode_nodebalancer_config_get"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.inputSchema["properties"]
    assert "config_id" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["nodebalancer_id", "config_id"]


async def test_handle_linode_nodebalancer_config_get(sample_config: Config) -> None:
    """Test linode_nodebalancer_config_get tool."""
    mock_config = {
        "id": 6,
        "port": 80,
        "protocol": "http",
        "algorithm": "roundrobin",
        "stickiness": "none",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.return_value = mock_config
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_get(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == mock_config
        mock_client.get_nodebalancer_config.assert_called_once_with(8, 6)


async def test_handle_linode_nodebalancer_config_get_invalid_arguments(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_get rejects invalid IDs."""
    invalid_cases: list[tuple[dict[str, Any], str]] = [
        ({"config_id": 6}, "nodebalancer_id must be a positive integer"),
        ({"nodebalancer_id": 8}, "config_id must be a positive integer"),
        (
            {"nodebalancer_id": True, "config_id": 6},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 0, "config_id": 6},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 8, "config_id": -1},
            "config_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": "8/9", "config_id": 6},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "6?x"},
            "config_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "../6"},
            "config_id must be a positive integer",
        ),
    ]

    for args, message in invalid_cases:
        result = await handle_linode_nodebalancer_config_get(args, sample_config)
        assert len(result) == 1
        assert message in result[0].text


async def test_handle_linode_nodebalancer_config_get_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_get(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_configs_list_tool_definition() -> None:
    """Test linode_nodebalancer_configs_list tool definition."""
    tool, capability = create_linode_nodebalancer_configs_list_tool()
    assert tool.name == "linode_nodebalancer_configs_list"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.inputSchema["properties"]
    assert "page" in tool.inputSchema["properties"]
    assert "page_size" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_configs_list(sample_config: Config) -> None:
    """Test linode_nodebalancer_configs_list tool."""
    mock_configs = {
        "data": [{"id": 6, "port": 80, "protocol": "http"}],
        "page": 1,
        "pages": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_configs.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_configs_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == mock_configs
        mock_client.list_nodebalancer_configs.assert_called_once_with(
            8, page=None, page_size=None
        )


async def test_handle_linode_nodebalancer_configs_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_configs_list tool with pagination."""
    mock_configs: dict[str, Any] = {"data": [], "page": 2, "pages": 3}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_configs.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_configs_list(
            {"nodebalancer_id": 8, "page": 2, "page_size": 50}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == mock_configs
        mock_client.list_nodebalancer_configs.assert_called_once_with(
            8, page=2, page_size=50
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id must be a positive integer"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        ({"nodebalancer_id": 8, "page": 0}, "page must be at least 1"),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        ({"nodebalancer_id": 8, "page_size": 24}, "page_size must be at least 25"),
        ({"nodebalancer_id": 8, "page_size": 501}, "page_size must be at most 500"),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_configs_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """Test linode_nodebalancer_configs_list rejects invalid arguments."""
    result = await handle_linode_nodebalancer_configs_list(arguments, sample_config)
    assert len(result) == 1
    assert message in result[0].text


async def test_handle_linode_nodebalancer_configs_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_configs_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_configs.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_configs_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_config_nodes_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list tool."""
    mock_nodes = {
        "data": [
            {"id": 1, "label": "node-1", "address": "192.0.2.4:80"},
            {"id": 2, "label": "node-2", "address": "192.0.2.5:80"},
        ],
        "page": 1,
        "pages": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_config_nodes.return_value = mock_nodes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_nodes_list(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "node-1" in result[0].text
        mock_client.list_nodebalancer_config_nodes.assert_called_once_with(
            8, 6, page=None, page_size=None
        )


async def test_handle_linode_nodebalancer_config_nodes_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list tool with pagination."""
    mock_nodes: dict[str, Any] = {"data": [], "page": 2, "pages": 3}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_config_nodes.return_value = mock_nodes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_nodes_list(
            {"nodebalancer_id": 8, "config_id": 6, "page": 2, "page_size": 50},
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == mock_nodes
        mock_client.list_nodebalancer_config_nodes.assert_called_once_with(
            8, 6, page=2, page_size=50
        )


async def test_handle_linode_nodebalancer_config_nodes_list_missing_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects missing nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_missing_config_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects missing config_id."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"nodebalancer_id": 8}, sample_config
    )
    assert len(result) == 1
    assert "config_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects non-integer page."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"nodebalancer_id": 8, "config_id": 6, "page": "abc"}, sample_config
    )
    assert len(result) == 1
    assert "page must be an integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_bool_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects bool nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"nodebalancer_id": True, "config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_zero_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects zero nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"nodebalancer_id": 0, "config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_negative_config_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list rejects negative config_id."""
    result = await handle_linode_nodebalancer_config_nodes_list(
        {"nodebalancer_id": 8, "config_id": -1}, sample_config
    )
    assert len(result) == 1
    assert "config_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_nodes_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_config_nodes.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_nodes_list(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


def test_linode_nodebalancer_config_node_create_tool_definition() -> None:
    """NodeBalancer config node create tool should require inputs and confirm."""
    tool, capability = create_linode_nodebalancer_config_node_create_tool()
    assert tool.name == "linode_nodebalancer_config_node_create"
    assert capability == Capability.Write
    required: list[str] = tool.inputSchema.get("required") or []
    assert "nodebalancer_id" in required
    assert "config_id" in required
    assert "address" in required
    assert "label" in required
    assert "confirm" in required
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_nodebalancer_config_node_create_confirm_required(
    sample_config: Config, confirm_value: object
) -> None:
    """Config node create rejects non-true boolean confirm before client call."""
    arguments: dict[str, Any] = {
        "nodebalancer_id": 8,
        "config_id": 6,
        "address": "192.0.2.4:80",
        "label": "node-1",
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_config_node_create(
            arguments, sample_config
        )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("nodebalancer_id", "8/9", "nodebalancer_id"),
        ("config_id", "6?x", "config_id"),
        ("nodebalancer_id", "..", "nodebalancer_id"),
        ("address", None, "address"),
        ("address", "", "address"),
        ("label", None, "label"),
        ("label", "ab", "label"),
        ("label", "x" * 33, "label"),
        ("mode", "invalid", "mode"),
        ("subnet_id", 0, "subnet_id"),
        ("weight", "50", "weight"),
        ("weight", 0, "weight"),
        ("weight", 256, "weight"),
    ],
)
async def test_handle_linode_nodebalancer_config_node_create_validation_errors(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Config node create validates path params and body before client call."""
    arguments: dict[str, Any] = {
        "nodebalancer_id": 8,
        "config_id": 6,
        "address": "192.0.2.4:80",
        "label": "node-1",
        "confirm": True,
    }
    arguments[field] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_config_node_create(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_config_node_create_success(
    sample_config: Config,
) -> None:
    """Config node create calls the client with the expected body."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer_config_node.return_value = {
            "id": 4,
            "label": "node-1",
            "address": "192.0.2.4:80",
            "mode": "accept",
            "weight": 50,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_create(
            {
                "nodebalancer_id": 8,
                "config_id": 6,
                "address": "192.0.2.4:80",
                "label": "node-1",
                "mode": "accept",
                "weight": 50,
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "node-1" in result[0].text
    mock_client.create_nodebalancer_config_node.assert_awaited_once_with(
        8,
        6,
        {
            "address": "192.0.2.4:80",
            "label": "node-1",
            "mode": "accept",
            "weight": 50,
        },
    )


async def test_handle_linode_nodebalancer_config_node_create_error(
    sample_config: Config,
) -> None:
    """Config node create propagates client errors through execute_tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer_config_node.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_create(
            {
                "nodebalancer_id": 8,
                "config_id": 6,
                "address": "192.0.2.4:80",
                "label": "node-1",
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "failed" in result[0].text.lower() or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_get(sample_config: Config) -> None:
    """Test linode_nodebalancer_get tool."""
    mock_nodebalancer = NodeBalancer(
        id=1,
        label="web-lb",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2001:db8::1",
        region="us-east",
        client_conn_throttle=0,
        transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.get_nodebalancer.assert_called_once_with(1)


async def test_handle_linode_nodebalancer_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_get tool with missing ID."""
    result = await handle_linode_nodebalancer_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_nodebalancer_get_error(sample_config: Config) -> None:
    """Test linode_nodebalancer_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_vpc_configs_list_tool_definition() -> None:
    """Test linode_nodebalancer_vpc_configs_list tool definition."""
    tool, capability = create_linode_nodebalancer_vpc_configs_list_tool()

    assert tool.name == "linode_nodebalancer_vpc_configs_list"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500
    assert tool.inputSchema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_vpc_configs_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_configs_list tool."""
    mock_configs = {
        "data": [
            {
                "id": 6,
                "ipv4_range": "10.0.0.12/30",
                "ipv6_range": None,
                "nodebalancer_id": 8,
                "subnet_id": 1,
                "vpc_id": 1,
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_vpc_configs.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_configs_list(
            {"nodebalancer_id": 8, "page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["data"][0]["id"] == 6
        assert data["results"] == 1
        mock_client.list_nodebalancer_vpc_configs.assert_called_once_with(
            8, page=1, page_size=25
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id must be a positive integer"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        ({"nodebalancer_id": 8, "page": 0}, "page must be at least 1"),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        ({"nodebalancer_id": 8, "page_size": 24}, "page_size must be at least 25"),
        ({"nodebalancer_id": 8, "page_size": 501}, "page_size must be at most 500"),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_vpc_configs_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer VPC config list rejects invalid arguments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_vpc_configs_list(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_vpc_configs_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_configs_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_vpc_configs.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_configs_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_vpc_config_get_tool_definition() -> None:
    """Test linode_nodebalancer_vpc_config_get tool definition."""
    tool, capability = create_linode_nodebalancer_vpc_config_get_tool()

    assert tool.name == "linode_nodebalancer_vpc_config_get"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["vpc_config_id"]["minimum"] == 1
    assert tool.inputSchema["required"] == ["nodebalancer_id", "vpc_config_id"]


async def test_handle_linode_nodebalancer_vpc_config_get(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_config_get tool."""
    mock_config = {
        "id": 456,
        "vpc_id": 789,
        "subnet_id": 101,
        "ipv4_range": "10.0.0.0/24",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_vpc_config.return_value = mock_config
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_config_get(
            {"nodebalancer_id": 123, "vpc_config_id": 456}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == 456
        assert data["vpc_id"] == 789
        mock_client.get_nodebalancer_vpc_config.assert_called_once_with(123, 456)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id must be a positive integer"),
        ({"nodebalancer_id": 0, "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": "123", "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": True, "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2", "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x", "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": "..", "vpc_config_id": 456}, "nodebalancer_id"),
        ({"nodebalancer_id": 123}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": 0}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": "456"}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": False}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": "4/5"}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": "4?x"}, "vpc_config_id"),
        ({"nodebalancer_id": 123, "vpc_config_id": ".."}, "vpc_config_id"),
    ],
)
async def test_handle_linode_nodebalancer_vpc_config_get_invalid_ids(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer VPC config get rejects invalid path parameters."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_vpc_config_get(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_vpc_config_get_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_config_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_vpc_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_config_get(
            {"nodebalancer_id": 123, "vpc_config_id": 456}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_stackscripts_list(sample_config: Config) -> None:
    """Test linode_stackscripts_list tool."""
    mock_stackscripts = [
        StackScript(
            id=1,
            username="testuser",
            user_gravatar_id="abc123",
            label="my-script",
            description="Test script",
            images=["linode/ubuntu22.04"],
            deployments_total=10,
            deployments_active=5,
            is_public=False,
            mine=True,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash\necho hello",
            user_defined_fields=[
                UDF(
                    label="Username",
                    name="username",
                    example="admin",
                    oneof="",
                    default="admin",
                )
            ],
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.return_value = mock_stackscripts
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({}, sample_config)

        assert len(result) == 1
        assert "my-script" in result[0].text
        mock_client.list_stackscripts.assert_called_once()


async def test_handle_linode_stackscripts_list_filter_mine(
    sample_config: Config,
) -> None:
    """Test linode_stackscripts_list tool with mine filter."""
    mock_stackscripts = [
        StackScript(
            id=1,
            username="testuser",
            user_gravatar_id="abc123",
            label="my-script",
            description="My script",
            images=["linode/ubuntu22.04"],
            deployments_total=10,
            deployments_active=5,
            is_public=False,
            mine=True,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash",
            user_defined_fields=[],
        ),
        StackScript(
            id=2,
            username="otheruser",
            user_gravatar_id="def456",
            label="other-script",
            description="Other script",
            images=["linode/ubuntu22.04"],
            deployments_total=100,
            deployments_active=50,
            is_public=True,
            mine=False,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash",
            user_defined_fields=[],
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.return_value = mock_stackscripts
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({"mine": "true"}, sample_config)

        assert len(result) == 1
        assert "my-script" in result[0].text
        assert "other-script" not in result[0].text


async def test_handle_linode_stackscripts_list_error(sample_config: Config) -> None:
    """Test linode_stackscripts_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_stackscript_delete_tool_schema() -> None:
    """Test linode_stackscript_delete tool schema."""
    tool, capability = create_linode_stackscript_delete_tool()

    assert tool.name == "linode_stackscript_delete"
    assert capability == Capability.Destroy
    assert tool.inputSchema["properties"]["stackscript_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["stackscript_id", "confirm"]


async def test_handle_linode_stackscript_delete_dry_run(sample_config: Config) -> None:
    """Dry-run previews the DELETE route without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_stackscript_delete(
            {"stackscript_id": 12345, "confirm": False, "dry_run": True},
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["tool"] == "linode_stackscript_delete"
    assert payload["would_execute"]["method"] == "DELETE"
    assert payload["would_execute"]["path"] == "/linode/stackscripts/12345"
    assert len(payload["side_effects"]) == 1
    mock_client_class.assert_not_called()


async def test_handle_linode_stackscript_delete(sample_config: Config) -> None:
    """Test linode_stackscript_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_stackscript.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_delete(
            {"stackscript_id": 12345, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "12345" in result[0].text
    assert "deleted" in result[0].text.lower()
    mock_client.delete_stackscript.assert_awaited_once_with(12345)


@pytest.mark.parametrize(
    "confirm",
    [None, False, "true", 1],
)
async def test_handle_linode_stackscript_delete_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """StackScript delete rejects missing/non-true confirm before dispatch."""
    arguments: dict[str, Any] = {"stackscript_id": 12345}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_stackscript_delete(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "stackscript_id",
    [None, 0, -1, True, "1/2", "1?x=y", ".."],
)
async def test_handle_linode_stackscript_delete_rejects_invalid_stackscript_id(
    sample_config: Config, stackscript_id: object
) -> None:
    """StackScript delete rejects malformed path parameters before dispatch."""
    arguments: dict[str, Any] = {"confirm": True}
    if stackscript_id is not None:
        arguments["stackscript_id"] = stackscript_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_stackscript_delete(arguments, sample_config)

    assert len(result) == 1
    assert "stackscript_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_stackscript_delete_error(sample_config: Config) -> None:
    """Test linode_stackscript_delete error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_stackscript.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_delete(
            {"stackscript_id": 12345, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_stackscript_create_tool_schema() -> None:
    """Test linode_stackscript_create tool schema."""
    tool, capability = create_linode_stackscript_create_tool()

    assert tool.name == "linode_stackscript_create"
    assert capability.name == "Write"
    assert tool.inputSchema["required"] == ["label", "images", "script", "confirm"]


async def test_handle_linode_stackscript_create(sample_config: Config) -> None:
    """Test linode_stackscript_create tool."""
    mock_stackscript = StackScript(
        id=12345,
        username="testuser",
        user_gravatar_id="abc123",
        label="my-script",
        description="Test script",
        images=["linode/ubuntu22.04"],
        deployments_total=0,
        deployments_active=0,
        is_public=False,
        mine=True,
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        script="#!/bin/bash",
        user_defined_fields=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_stackscript.return_value = mock_stackscript
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_create(
            {
                "label": "my-script",
                "images": ["linode/ubuntu22.04"],
                "script": "#!/bin/bash",
                "description": "Test script",
                "is_public": False,
                "rev_note": "Initial revision",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "my-script" in result[0].text
        assert "12345" in result[0].text
        mock_client.create_stackscript.assert_called_once_with(
            label="my-script",
            images=["linode/ubuntu22.04"],
            script="#!/bin/bash",
            description="Test script",
            is_public=False,
            rev_note="Initial revision",
        )


async def test_handle_linode_stackscript_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Test linode_stackscript_create requires confirmation."""
    result = await handle_linode_stackscript_create(
        {
            "label": "my-script",
            "images": ["linode/ubuntu22.04"],
            "script": "#!/bin/bash",
        },
        sample_config,
    )

    assert len(result) == 1
    assert "Set confirm=true" in result[0].text
    assert "confirm=true" in result[0].text


async def test_handle_linode_stackscript_create_validates_required_fields(
    sample_config: Config,
) -> None:
    """Test linode_stackscript_create required field validation."""
    result = await handle_linode_stackscript_create(
        {
            "label": "my-script",
            "images": [],
            "script": "#!/bin/bash",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "Error" in result[0].text
    assert "images" in result[0].text


# Stage 4: Write operations tests


async def test_handle_linode_sshkey_create(sample_config: Config) -> None:
    """Test linode_sshkey_create tool."""
    mock_key = SSHKey(
        id=12345,
        label="my-key",
        ssh_key="ssh-rsa AAAA...",
        created="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_ssh_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_create(
            {"label": "my-key", "ssh_key": "ssh-rsa AAAA...", "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert "12345" in result[0].text


async def test_handle_linode_sshkey_create_missing_params(
    sample_config: Config,
) -> None:
    """Test linode_sshkey_create tool with missing parameters."""
    result = await handle_linode_sshkey_create(
        {"label": "test", "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "Error" in result[0].text


async def test_handle_linode_sshkey_update(sample_config: Config) -> None:
    """Test linode_sshkey_update tool."""
    mock_key = SSHKey(
        id=12345,
        label="renamed-key",
        ssh_key="ssh-rsa AAAA...",
        created="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_ssh_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_update(
            {"ssh_key_id": 12345, "label": "renamed-key", "confirm": True},
            sample_config,
        )

        mock_client.update_ssh_key.assert_awaited_once_with(12345, "renamed-key")
        assert len(result) == 1
        assert "renamed-key" in result[0].text
        assert "updated" in result[0].text.lower()


async def test_handle_linode_sshkey_update_missing_params(
    sample_config: Config,
) -> None:
    """Test linode_sshkey_update tool with missing parameters."""
    result = await handle_linode_sshkey_update(
        {"ssh_key_id": 12345, "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_sshkey_update_no_confirm(sample_config: Config) -> None:
    """Test linode_sshkey_update tool without confirmation."""
    result = await handle_linode_sshkey_update(
        {"ssh_key_id": 12345, "label": "renamed-key"}, sample_config
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_sshkey_delete(sample_config: Config) -> None:
    """Test linode_sshkey_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_ssh_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_delete(
            {"ssh_key_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_sshkey_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_sshkey_create(
        {"label": "my-key", "ssh_key": "ssh-rsa AAAA", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_sshkey_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/profile/sshkeys"
    assert body["current_state"] is None
    assert any("my-key" in s for s in body["side_effects"])
    assert "confirm=true" not in result[0].text


async def test_sshkey_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_sshkey_create(
        {"ssh_key": "ssh-rsa AAAA", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_sshkey_update_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never calls update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_ssh_key.return_value = {"id": 123, "label": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_sshkey_update(
            {"ssh_key_id": 123, "label": "renamed", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_sshkey_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/profile/sshkeys/123"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_ssh_key.assert_awaited_once_with(123)
        mock_client.update_ssh_key.assert_not_called()


async def test_sshkey_update_dry_run_still_validates_id(
    sample_config: Config,
) -> None:
    """Missing ssh_key_id must error out regardless of dry_run."""
    result = await handle_linode_sshkey_update(
        {"label": "renamed", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "ssh_key_id is required" in result[0].text


async def test_sshkey_delete_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never calls delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_ssh_key.return_value = {"id": 123, "label": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_sshkey_delete(
            {"ssh_key_id": 123, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_sshkey_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/profile/sshkeys/123"
        mock_client.get_ssh_key.assert_awaited_once_with(123)
        mock_client.delete_ssh_key.assert_not_called()
        assert "confirm=true" not in result[0].text


async def test_sshkey_delete_dry_run_still_validates_id(
    sample_config: Config,
) -> None:
    """Missing ssh_key_id must error out regardless of dry_run."""
    result = await handle_linode_sshkey_delete({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "ssh_key_id is required" in result[0].text


async def test_stackscript_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_stackscript_create(
        {
            "label": "my-script",
            "images": ["linode/ubuntu22.04"],
            "script": "#!/bin/bash",
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_stackscript_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/stackscripts"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_stackscript_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_stackscript_create(
        {"images": ["linode/ubuntu22.04"], "script": "#!/bin/bash", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_instance_boot(sample_config: Config) -> None:
    """Test linode_instance_boot tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.boot_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_boot(
            {"instance_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "boot" in result[0].text.lower()


async def test_handle_linode_instance_reboot(sample_config: Config) -> None:
    """Test linode_instance_reboot tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.reboot_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_reboot(
            {"instance_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "reboot" in result[0].text.lower()


async def test_handle_linode_instance_shutdown(sample_config: Config) -> None:
    """Test linode_instance_shutdown tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.shutdown_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_shutdown(
            {"instance_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "shutdown" in result[0].text.lower()


async def test_handle_linode_instance_create_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_create tool without confirmation."""
    result = await handle_linode_instance_create(
        {"region": "us-east", "type": "g6-nanode-1", "firewall_id": 12345},
        sample_config,
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_create_missing_firewall_id(
    sample_config: Config,
) -> None:
    """The current Linode Interfaces generation requires firewall_id at create
    time. The tool must reject the call before any HTTP request when missing.
    """
    result = await handle_linode_instance_create(
        {"region": "us-east", "type": "g6-nanode-1", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


def test_linode_instance_create_tool_schema() -> None:
    """The tool schema must expose firewall_id and the route flags, and must
    not surface the legacy private_ip parameter.
    """
    from linodemcp.tools import create_linode_instance_create_tool

    tool, _ = create_linode_instance_create_tool()
    props: dict[str, Any] = tool.inputSchema["properties"]
    required: list[str] = tool.inputSchema["required"]

    assert "firewall_id" in props, "schema must include firewall_id"
    assert "route_ipv4" in props, "schema must include route_ipv4"
    assert "route_ipv6" in props, "schema must include route_ipv6"
    assert "private_ip" not in props, (
        "schema must not include legacy private_ip parameter"
    )
    assert "firewall_id" in required, "firewall_id must be required"


async def test_handle_linode_instance_create(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_create tool."""
    mock_instance = Instance(
        id=sample_instance_data["id"],
        label=sample_instance_data["label"],
        status=sample_instance_data["status"],
        type=sample_instance_data["type"],
        region=sample_instance_data["region"],
        image=sample_instance_data["image"],
        ipv4=sample_instance_data["ipv4"],
        ipv6=sample_instance_data["ipv6"],
        hypervisor=sample_instance_data["hypervisor"],
        specs=Specs(
            disk=sample_instance_data["specs"]["disk"],
            memory=sample_instance_data["specs"]["memory"],
            vcpus=sample_instance_data["specs"]["vcpus"],
            transfer=sample_instance_data["specs"]["transfer"],
            gpus=sample_instance_data["specs"]["gpus"],
        ),
        alerts=Alerts(
            cpu=sample_instance_data["alerts"]["cpu"],
            network_in=sample_instance_data["alerts"]["network_in"],
            network_out=sample_instance_data["alerts"]["network_out"],
            transfer_quota=sample_instance_data["alerts"]["transfer_quota"],
            io=sample_instance_data["alerts"]["io"],
        ),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(day="Saturday", window="W0"),
            last_successful=None,
        ),
        created=sample_instance_data["created"],
        updated=sample_instance_data["updated"],
        group=sample_instance_data["group"],
        tags=sample_instance_data["tags"],
        watchdog_enabled=sample_instance_data["watchdog_enabled"],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_instance.return_value = mock_instance
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_create(
            {
                "region": "us-east",
                "type": "g6-nanode-1",
                "firewall_id": 12345,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "created" in result[0].text.lower()


async def test_handle_linode_instance_update_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_update tool without confirmation."""
    result = await handle_linode_instance_update(
        {"instance_id": 12345, "label": "updated-instance"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_firewalls_update_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Confirm must be exactly true before the client is called."""
    arguments: dict[str, Any] = {"linode_id": 42, "firewall_ids": [123]}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_update(arguments, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", True, 0, -1])
async def test_handle_linode_instance_firewalls_update_rejects_invalid_linode_id(
    sample_config: Config, linode_id: object
) -> None:
    """Malformed Linode IDs are rejected before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_update(
            {"linode_id": linode_id, "firewall_ids": [123], "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("firewall_ids", ["123", [0], [-1], [True], ["123"]])
async def test_handle_linode_instance_firewalls_update_rejects_invalid_firewall_ids(
    sample_config: Config, firewall_ids: object
) -> None:
    """Invalid firewall_ids are rejected before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_update(
            {"linode_id": 42, "firewall_ids": firewall_ids, "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "firewall_ids must be a list of positive integers" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("page", "2", "page must be an integer"),
        ("page", 0, "page must be at least 1"),
        ("page_size", "25", "page_size must be an integer"),
        ("page_size", 24, "page_size must be at least 25"),
        ("page_size", 501, "page_size must be at most 500"),
    ],
)
async def test_handle_linode_instance_firewalls_update_rejects_invalid_pagination(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Invalid pagination values are rejected before the client call."""
    arguments: dict[str, Any] = {
        "linode_id": 42,
        "firewall_ids": [123],
        "confirm": True,
        field: value,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_update(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


def test_linode_instance_firewalls_update_tool_schema() -> None:
    """The Linode firewall assignment update schema exposes safety controls."""
    tool, capability = create_linode_instance_firewalls_update_tool()
    props: dict[str, Any] = tool.inputSchema["properties"]

    assert tool.name == "linode_instance_firewalls_update"
    assert capability.name == "Write"
    assert "linode_id" in tool.inputSchema["required"]
    assert "firewall_ids" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]
    assert "dry_run" not in tool.inputSchema["required"]
    assert props["dry_run"]["type"] == "boolean"
    assert props["confirm"]["type"] == "boolean"


async def test_handle_linode_instance_firewalls_update(
    sample_config: Config,
) -> None:
    """Test linode_instance_firewalls_update tool."""
    response_data = {"data": [{"id": 123}], "page": 1, "pages": 1, "results": 1}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_instance_firewalls.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_firewalls_update(
            {
                "linode_id": 42,
                "firewall_ids": [123],
                "page": 2,
                "page_size": 25,
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.update_instance_firewalls.assert_awaited_once_with(
        42, [123], page=2, page_size=25
    )


async def test_handle_linode_instance_firewalls_update_allows_empty_firewall_ids(
    sample_config: Config,
) -> None:
    """An empty firewall_ids list is forwarded as the documented removal path."""
    response_data: dict[str, Any] = {
        "data": [],
        "page": 1,
        "pages": 1,
        "results": 0,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_instance_firewalls.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_firewalls_update(
            {"linode_id": 42, "firewall_ids": [], "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.update_instance_firewalls.assert_awaited_once_with(
        42, [], page=None, page_size=None
    )


async def test_instance_firewalls_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the PUT body/query and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_update(
            {
                "linode_id": 42,
                "firewall_ids": [123],
                "page": 2,
                "page_size": 25,
                "dry_run": True,
            },
            sample_config,
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_firewalls_update"
    assert body["would_execute"]["method"] == "PUT"
    assert (
        body["would_execute"]["path"]
        == "/linode/instances/42/firewalls?page=2&page_size=25"
    )
    assert body["would_execute"]["body"] == {"firewall_ids": [123]}
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_update_missing_field(
    sample_config: Config,
) -> None:
    """Test linode_instance_update tool with no update fields."""
    result = await handle_linode_instance_update(
        {"instance_id": 12345, "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "at least one update field" in result[0].text.lower()


def test_linode_instance_update_tool_schema() -> None:
    """The update tool schema exposes documented editable fields."""
    tool, capability = create_linode_instance_update_tool()
    props: dict[str, Any] = tool.inputSchema["properties"]

    assert tool.name == "linode_instance_update"
    assert capability.name == "Write"
    assert "instance_id" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]
    for field in (
        "label",
        "group",
        "tags",
        "alerts",
        "maintenance_policy",
        "watchdog_enabled",
    ):
        assert field in props


async def test_handle_linode_instance_update(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_update tool."""
    mock_instance = Instance(
        id=sample_instance_data["id"],
        label="updated-instance",
        status=sample_instance_data["status"],
        type=sample_instance_data["type"],
        region=sample_instance_data["region"],
        image=sample_instance_data["image"],
        ipv4=sample_instance_data["ipv4"],
        ipv6=sample_instance_data["ipv6"],
        hypervisor=sample_instance_data["hypervisor"],
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(**sample_instance_data["backups"]["schedule"]),
            last_successful=None,
        ),
        created=sample_instance_data["created"],
        updated=sample_instance_data["updated"],
        group=sample_instance_data["group"],
        tags=["updated", "prod"],
        watchdog_enabled=False,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_instance.return_value = mock_instance
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_update(
            {
                "instance_id": 12345,
                "label": "updated-instance",
                "tags": ["updated", "prod"],
                "watchdog_enabled": False,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()
        mock_client.update_instance.assert_called_once_with(
            12345,
            label="updated-instance",
            tags=["updated", "prod"],
            watchdog_enabled=False,
        )


async def test_handle_linode_instance_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_delete tool without confirmation."""
    result = await handle_linode_instance_delete({"instance_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_delete(sample_config: Config) -> None:
    """Test linode_instance_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_delete(
            {"instance_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


def test_linode_instance_mutate_tool_schema_requires_confirm() -> None:
    """Mutate tool schema requires explicit confirmation."""
    tool, capability = create_linode_instance_mutate_tool()

    assert tool.name == "linode_instance_mutate"
    assert capability is Capability.Write
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["allow_auto_disk_resize"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_mutate_rejects_bad_confirm(
    confirm: object, sample_config: Config
) -> None:
    """Mutate rejects missing or non-true confirmation before client calls."""
    arguments: dict[str, Any] = {"linode_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_mutate(arguments, sample_config)

    assert "confirm must be true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_linode_id", ["1/2", "1?x=2", "..", True, 0, -1])
async def test_handle_linode_instance_mutate_rejects_bad_linode_id(
    bad_linode_id: object, sample_config: Config
) -> None:
    """Mutate rejects malformed Linode IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_mutate(
            {"linode_id": bad_linode_id, "confirm": True}, sample_config
        )

    assert "linode_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_mutate_rejects_bad_disk_resize(
    sample_config: Config,
) -> None:
    """Mutate rejects non-boolean allow_auto_disk_resize before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_mutate(
            {
                "linode_id": 123,
                "allow_auto_disk_resize": "true",
                "confirm": True,
            },
            sample_config,
        )

    assert "allow_auto_disk_resize must be a boolean" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_mutate(sample_config: Config) -> None:
    """Test linode_instance_mutate tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.mutate_instance.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_mutate(
            {
                "linode_id": 123,
                "allow_auto_disk_resize": False,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "upgrade" in result[0].text.lower()
        mock_client.mutate_instance.assert_awaited_once_with(
            123, allow_auto_disk_resize=False
        )


async def test_instance_mutate_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must preview mutate without client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_mutate(
            {"linode_id": 123, "allow_auto_disk_resize": False, "dry_run": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_mutate"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/mutate"
    assert body["would_execute"]["body"] == {"allow_auto_disk_resize": False}
    mock_client_class.assert_not_called()


def test_linode_instance_upgrade_interfaces_tool_schema_requires_confirm() -> None:
    """Upgrade interfaces tool schema requires explicit confirmation."""
    tool, capability = create_linode_instance_upgrade_interfaces_tool()

    assert tool.name == "linode_instance_upgrade_interfaces"
    assert capability is Capability.Write
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["api_dry_run"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_upgrade_interfaces_rejects_bad_confirm(
    confirm: object, sample_config: Config
) -> None:
    """Upgrade interfaces rejects missing or non-true confirmation before calls."""
    arguments: dict[str, Any] = {"linode_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_upgrade_interfaces(
            arguments, sample_config
        )

    assert "confirm must be true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_linode_id", ["1/2", "1?x=2", "..", True, 0, -1])
async def test_handle_linode_instance_upgrade_interfaces_rejects_bad_linode_id(
    bad_linode_id: object, sample_config: Config
) -> None:
    """Upgrade interfaces rejects malformed Linode IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_upgrade_interfaces(
            {"linode_id": bad_linode_id, "confirm": True}, sample_config
        )

    assert "linode_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"linode_id": 123, "config_id": "1", "confirm": True},
            "config_id must be an integer",
        ),
        (
            {"linode_id": 123, "config_id": 0, "confirm": True},
            "config_id must be at least 1",
        ),
        (
            {"linode_id": 123, "api_dry_run": "true", "confirm": True},
            "api_dry_run must be a boolean",
        ),
    ],
)
async def test_handle_linode_instance_upgrade_interfaces_rejects_bad_body_fields(
    arguments: dict[str, Any], message: str, sample_config: Config
) -> None:
    """Upgrade interfaces validates optional body fields before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_upgrade_interfaces(
            arguments, sample_config
        )

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_upgrade_interfaces(sample_config: Config) -> None:
    """Test linode_instance_upgrade_interfaces tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.upgrade_instance_interfaces.return_value = {"dry_run": False}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_upgrade_interfaces(
            {
                "linode_id": 123,
                "config_id": 456,
                "api_dry_run": False,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "interface upgrade" in result[0].text.lower()
        mock_client.upgrade_instance_interfaces.assert_awaited_once_with(
            123, config_id=456, dry_run=False
        )


async def test_instance_upgrade_interfaces_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true previews interface upgrade without client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_upgrade_interfaces(
            {"linode_id": 123, "config_id": 456, "api_dry_run": True, "dry_run": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_upgrade_interfaces"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/upgrade-interfaces"
    assert body["would_execute"]["body"] == {"config_id": 456, "dry_run": True}
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_resize_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_resize tool without confirmation."""
    result = await handle_linode_instance_resize(
        {"instance_id": 12345, "type": "g6-standard-1"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_resize(sample_config: Config) -> None:
    """Test linode_instance_resize tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resize_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_resize(
            {"instance_id": 12345, "type": "g6-standard-1", "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "resize" in result[0].text.lower()


async def test_handle_linode_firewall_create(sample_config: Config) -> None:
    """Test linode_firewall_create tool."""
    mock_firewall = Firewall(
        id=12345,
        label="my-firewall",
        status="enabled",
        rules=FirewallRules(
            inbound=[],
            inbound_policy="ACCEPT",
            outbound=[],
            outbound_policy="ACCEPT",
        ),
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_firewall.return_value = mock_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_create(
            {"label": "my-firewall", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "my-firewall" in result[0].text


async def test_handle_linode_firewall_update(sample_config: Config) -> None:
    """Test linode_firewall_update tool."""
    mock_firewall = Firewall(
        id=12345,
        label="updated-firewall",
        status="enabled",
        rules=FirewallRules(
            inbound=[],
            inbound_policy="ACCEPT",
            outbound=[],
            outbound_policy="ACCEPT",
        ),
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_firewall.return_value = mock_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_update(
            {"firewall_id": 12345, "label": "updated-firewall", "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_firewall_delete(sample_config: Config) -> None:
    """Test linode_firewall_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_firewall.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_delete(
            {"firewall_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_firewall_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete.

    Decodes the JSON body so a future renaming of the v0 wire shape or
    a regression where Execute fires anyway gets caught.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall.return_value = {
            "id": 789,
            "label": "prod-fw",
            "status": "enabled",
        }
        mock_client.list_firewall_devices.return_value = {"data": []}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_delete(
            {"firewall_id": 789, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_firewall_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/networking/firewalls/789"
        mock_client.get_firewall.assert_awaited_once_with(789)
        mock_client.delete_firewall.assert_not_called()


async def test_firewall_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall.return_value = {"id": 789, "label": "prod-fw"}
        mock_client.list_firewall_devices.return_value = {"data": []}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_delete(
            {"firewall_id": 789, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_firewall_delete_dry_run_still_validates_firewall_id(
    sample_config: Config,
) -> None:
    """Missing firewall_id must error out regardless of dry_run."""
    result = await handle_linode_firewall_delete(
        {"dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_firewall_delete_dry_run_surfaces_device_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: attached devices appear as removed dependencies."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall.return_value = {"id": 789, "label": "prod-fw"}
        mock_client.list_firewall_devices.return_value = {
            "data": [
                {"id": 1, "entity": {"id": 555, "type": "linode", "label": "web"}},
                {"id": 2, "entity": {"id": 666, "type": "nodebalancer", "label": "lb"}},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_delete(
            {"firewall_id": 789, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 2
        assert {d["kind"] for d in deps} == {"linode", "nodebalancer"}
        assert all(d["action"] == "removed" for d in deps)
        assert body["warnings"]
        mock_client.delete_firewall.assert_not_called()


async def test_firewall_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_firewall_create(
        {"label": "fw-01", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_firewall_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/firewalls"
    assert body["current_state"] is None
    assert any("fw-01" in s for s in body["side_effects"])
    assert "confirm=true" not in result[0].text


async def test_firewall_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_firewall_create({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_firewall_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall.return_value = {"id": 789, "label": "prod-fw"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_update(
            {"firewall_id": 789, "label": "renamed", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_firewall_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/networking/firewalls/789"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_firewall.assert_awaited_once_with(789)
        mock_client.update_firewall.assert_not_called()


async def test_firewall_update_dry_run_still_validates_firewall_id(
    sample_config: Config,
) -> None:
    """Missing firewall_id must error out regardless of dry_run."""
    result = await handle_linode_firewall_update({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_firewall_rules_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch current rules via GET and never replace them."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall_rules.return_value = {"inbound": [], "outbound": []}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_rules_update(
            {"firewall_id": 789, "inbound": [], "outbound": [], "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_firewall_rules_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/networking/firewalls/789/rules"
        mock_client.get_firewall_rules.assert_awaited_once_with(789)
        mock_client.update_firewall_rules.assert_not_called()


async def test_firewall_rules_update_dry_run_still_validates_firewall_id(
    sample_config: Config,
) -> None:
    """Missing firewall_id must error out regardless of dry_run."""
    result = await handle_linode_firewall_rules_update(
        {"inbound": [], "outbound": [], "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_firewall_settings_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch settings via GET and never update them."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall_settings.return_value = {"default_firewall_ids": {}}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_settings_update(
            {"default_firewall_ids": {"linode": 5}, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_firewall_settings_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/networking/firewalls/settings"
        mock_client.get_firewall_settings.assert_awaited_once()
        mock_client.update_firewall_settings.assert_not_called()


async def test_firewall_settings_update_dry_run_still_validates_ids(
    sample_config: Config,
) -> None:
    """Missing default_firewall_ids must error out regardless of dry_run."""
    result = await handle_linode_firewall_settings_update(
        {"dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "default_firewall_ids" in result[0].text


async def test_firewall_device_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the device assignment with no call."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_create,
    )

    result = await handle_linode_firewall_device_create(
        {"firewall_id": 789, "id": 456, "type": "linode", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_firewall_device_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/firewalls/789/devices"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text
    assert len(body["side_effects"]) == 1
    assert "456" in body["side_effects"][0]
    assert "firewall 789" in body["side_effects"][0]


async def test_firewall_device_create_dry_run_still_validates_firewall_id(
    sample_config: Config,
) -> None:
    """Missing firewall_id must error out regardless of dry_run."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_create,
    )

    result = await handle_linode_firewall_device_create(
        {"id": 456, "type": "linode", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_handle_linode_firewall_rules_update(sample_config: Config) -> None:
    """Test linode_firewall_rules_update tool happy path."""
    mock_result = {
        "inbound": [
            {
                "action": "ACCEPT",
                "protocol": "TCP",
                "ports": "22",
                "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                "label": "allow-ssh",
                "description": "",
            }
        ],
        "outbound": [],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_firewall_rules.return_value = mock_result
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_rules_update(
            {
                "firewall_id": 12345,
                "inbound": mock_result["inbound"],
                "outbound": mock_result["outbound"],
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "rules updated" in result[0].text.lower()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_firewall_rules_update_requires_boolean_confirm(
    sample_config: Config, confirm: Any
) -> None:
    """Firewall rules update rejects missing or non-true confirm."""
    arguments: dict[str, Any] = {"firewall_id": 12345}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_firewall_rules_update_missing_id(
    sample_config: Config,
) -> None:
    """Firewall rules update rejects missing firewall_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("firewall_id", ["12345", "../12345", "12345?x=1", True])
async def test_handle_linode_firewall_rules_update_invalid_id(
    sample_config: Config, firewall_id: Any
) -> None:
    """Firewall rules update rejects malformed firewall_id values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(
            {"firewall_id": firewall_id, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "firewall_id must be an integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("firewall_id", [0, -1])
async def test_handle_linode_firewall_rules_update_non_positive_id(
    sample_config: Config, firewall_id: int
) -> None:
    """Firewall rules update rejects non-positive firewall IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(
            {"firewall_id": firewall_id, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "firewall_id" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"firewall_id": 12345, "confirm": True},
        {"firewall_id": 12345, "confirm": True, "inbound": []},
        {"firewall_id": 12345, "confirm": True, "outbound": []},
    ],
)
async def test_handle_linode_firewall_rules_update_requires_explicit_rule_lists(
    sample_config: Config, arguments: dict[str, Any]
) -> None:
    """Firewall rules update requires explicit inbound and outbound rule lists."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(arguments, sample_config)

    assert len(result) == 1
    assert " is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value"),
    [
        ("inbound", "not-a-list"),
        ("outbound", "not-a-list"),
        ("inbound", ["bad-rule"]),
        ("outbound", ["bad-rule"]),
    ],
)
async def test_handle_linode_firewall_rules_update_invalid_rule_lists(
    sample_config: Config, field: str, value: Any
) -> None:
    """Firewall rules update rejects malformed rule lists."""
    arguments: dict[str, Any] = {
        "firewall_id": 12345,
        "confirm": True,
        "inbound": [],
        "outbound": [],
        field: value,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_rules_update(arguments, sample_config)

    assert len(result) == 1
    assert f"{field} must be a list of rule objects" in result[0].text
    mock_client_class.assert_not_called()


async def test_linode_instance_firewalls_apply_tool_definition() -> None:
    """Test linode_instance_firewalls_apply tool definition."""
    tool, capability = create_linode_instance_firewalls_apply_tool()

    assert tool.name == "linode_instance_firewalls_apply"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["linode_id", "confirm"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_instance_firewalls_apply(sample_config: Config) -> None:
    """Test linode_instance_firewalls_apply tool happy path."""
    mock_result = {"id": 123, "label": "web-1"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.apply_linode_firewalls.return_value = mock_result
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_firewalls_apply(
            {"linode_id": 123, "confirm": True}, sample_config
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["message"] == "Firewalls applied to Linode 123 successfully"
    assert body["linode_id"] == 123
    assert body["result"] == mock_result
    mock_client.apply_linode_firewalls.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_firewalls_apply_requires_boolean_confirm(
    sample_config: Config, confirm: Any
) -> None:
    """Linode firewall apply rejects missing or non-true confirm."""
    arguments: dict[str, Any] = {"linode_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_apply(arguments, sample_config)

    assert len(result) == 1
    assert "confirm must be true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("linode_id", [None, "123", "../123", "123?x=1", True, 0, -1])
async def test_handle_linode_instance_firewalls_apply_invalid_linode_id(
    sample_config: Config, linode_id: Any
) -> None:
    """Linode firewall apply rejects malformed Linode IDs before client calls."""
    arguments: dict[str, Any] = {"confirm": True}
    if linode_id is not None:
        arguments["linode_id"] = linode_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_apply(arguments, sample_config)

    assert len(result) == 1
    assert "linode_id" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_firewalls_apply_dry_run(
    sample_config: Config,
) -> None:
    """dry_run=true previews Linode firewall apply without a client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_firewalls_apply(
            {"linode_id": 123, "dry_run": True}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_firewalls_apply"
    assert body["would_execute"] == {
        "method": "POST",
        "path": "/linode/instances/123/firewalls/apply",
    }
    assert "Linode 123" in body["side_effects"][0]
    mock_client_class.assert_not_called()


async def test_linode_firewall_settings_update_tool_definition() -> None:
    """Test linode_firewall_settings_update tool definition."""
    tool, capability = create_linode_firewall_settings_update_tool()

    assert tool.name == "linode_firewall_settings_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["default_firewall_ids", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["default_firewall_ids"]["minProperties"] == 1


async def test_handle_linode_firewall_settings_update(sample_config: Config) -> None:
    """Test linode_firewall_settings_update tool happy path."""
    payload = {"linode": 100, "nodebalancer": 101}
    mock_result = {"default_firewall_ids": payload}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_firewall_settings.return_value = mock_result
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_settings_update(
            {"default_firewall_ids": payload, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "updated successfully" in result[0].text.lower()
    mock_client.update_firewall_settings.assert_awaited_once_with(payload)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_firewall_settings_update_requires_boolean_confirm(
    sample_config: Config, confirm: Any
) -> None:
    """Default firewall update rejects missing or non-true confirm."""
    arguments: dict[str, Any] = {"default_firewall_ids": {"linode": 100}}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_settings_update(arguments, sample_config)

    assert len(result) == 1
    assert "confirm must be true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "default_firewall_ids",
    [
        None,
        {},
        {"linode": 0},
        {"linode": -1},
        {"linode": True},
        {"linode": "100"},
        {"unknown": 100},
    ],
)
async def test_handle_linode_firewall_settings_update_invalid_default_ids(
    sample_config: Config, default_firewall_ids: Any
) -> None:
    """Default firewall update rejects malformed default_firewall_ids."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_settings_update(
            {"default_firewall_ids": default_firewall_ids, "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "default_firewall_ids must be" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_domain_clone(sample_config: Config) -> None:
    """Test linode_domain_clone tool."""
    mock_domain = Domain(
        id=23456,
        domain="clone.example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.clone_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_clone(
            {"domain_id": 12345, "domain": "clone.example.com", "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "clone.example.com" in result[0].text
    mock_client.clone_domain.assert_awaited_once_with(
        domain_id=12345, domain="clone.example.com"
    )


async def test_domain_clone_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the clone and does not call the client."""
    result = await handle_linode_domain_clone(
        {
            "domain_id": 12345,
            "domain": "clone.example.com",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_domain_clone"
    assert body["would_execute"] == {
        "method": "POST",
        "path": "/domains/12345/clone",
        "body": {"domain": "clone.example.com"},
    }
    assert any("clone.example.com" in s for s in body["side_effects"])


async def test_domain_clone_requires_literal_confirm(
    sample_config: Config,
) -> None:
    """Clone rejects missing, false, string, and numeric confirm values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        for confirm in (None, False, "true", 1):
            args: dict[str, Any] = {
                "domain_id": 12345,
                "domain": "clone.example.com",
            }
            if confirm is not None:
                args["confirm"] = confirm
            result = await handle_linode_domain_clone(args, sample_config)
            assert "Set confirm=true" in result[0].text

    mock_client_class.assert_not_called()


async def test_domain_clone_validates_required_arguments(
    sample_config: Config,
) -> None:
    """Clone validates required route/body arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_domain_clone(
            {"domain": "clone.example.com", "confirm": True}, sample_config
        )
        assert "domain_id must be a positive integer" in result[0].text

        result = await handle_linode_domain_clone(
            {"domain_id": 12345, "confirm": True}, sample_config
        )
        assert "domain is required" in result[0].text

        for value in ("123/456", "123?x=1", "..", True, 0):
            result = await handle_linode_domain_clone(
                {
                    "domain_id": value,
                    "domain": "clone.example.com",
                    "confirm": True,
                },
                sample_config,
            )
            assert "domain_id must be a positive integer" in result[0].text

    mock_client_class.assert_not_called()


async def test_handle_linode_domain_create(sample_config: Config) -> None:
    """Test linode_domain_create tool."""
    mock_domain = Domain(
        id=12345,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_create(
            {
                "domain": "example.com",
                "soa_email": "admin@example.com",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "example.com" in result[0].text


async def test_handle_linode_domain_update(sample_config: Config) -> None:
    """Test linode_domain_update tool."""
    mock_domain = Domain(
        id=12345,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="Updated",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_update(
            {"domain_id": 12345, "description": "Updated", "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_domain_update_dry_run_surfaces_field_changes(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: domain update names the SOA-email change."""
    current = Domain(
        id=12345,
        domain="example.com",
        type="master",
        status="active",
        soa_email="old@example.com",
        description="",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.return_value = current
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_update(
            {"domain_id": 12345, "soa_email": "new@example.com", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_domain_update"
        assert any("new@example.com" in s for s in body["side_effects"])
        mock_client.update_domain.assert_not_called()


async def test_handle_linode_domain_delete(sample_config: Config) -> None:
    """Test linode_domain_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_domain.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_delete(
            {"domain_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_domain_delete_dry_run_surfaces_ns_record_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: NS records appear as cascade_deleted dependencies."""

    def _record(
        record_id: int, record_type: str, name: str, target: str
    ) -> DomainRecord:
        return DomainRecord(
            id=record_id,
            type=record_type,
            name=name,
            target=target,
            priority=0,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-15T10:00:00",
            updated="2024-01-15T10:00:00",
        )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.return_value = {"id": 12345, "domain": "example.com"}
        mock_client.list_domain_records.return_value = [
            _record(1, "NS", "example.com", "ns1.linode.com"),
            _record(2, "A", "www", "192.0.2.1"),
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_delete(
            {"domain_id": 12345, "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_domain_delete"
        deps = body["dependencies"]
        assert len(deps) == 1
        assert deps[0]["kind"] == "ns_record"
        assert deps[0]["action"] == "cascade_deleted"
        assert body["warnings"]
        mock_client.delete_domain.assert_not_called()


async def test_handle_linode_domain_record_create(sample_config: Config) -> None:
    """Test linode_domain_record_create tool."""
    mock_record = DomainRecord(
        id=12345,
        type="A",
        name="www",
        target="192.0.2.1",
        priority=0,
        weight=0,
        port=0,
        ttl_sec=300,
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_domain_record.return_value = mock_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_create(
            {
                "domain_id": 12345,
                "type": "A",
                "name": "www",
                "target": "192.0.2.1",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "www" in result[0].text


async def test_handle_linode_domain_record_update(sample_config: Config) -> None:
    """Test linode_domain_record_update tool."""
    mock_record = DomainRecord(
        id=12345,
        type="A",
        name="www",
        target="192.0.2.2",
        priority=0,
        weight=0,
        port=0,
        ttl_sec=300,
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_domain_record.return_value = mock_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_update(
            {
                "domain_id": 12345,
                "record_id": 12345,
                "target": "192.0.2.2",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_domain_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_domain_create(
        {"domain": "example.com", "type": "master", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_domain_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/domains"
    assert body["current_state"] is None
    assert any("example.com" in s for s in body["side_effects"])
    assert "confirm=true" not in result[0].text


async def test_domain_create_dry_run_still_validates_domain(
    sample_config: Config,
) -> None:
    """Missing domain must error out regardless of dry_run."""
    result = await handle_linode_domain_create(
        {"type": "master", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "domain is required" in result[0].text


async def test_domain_record_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the record create with no state and no call."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "type": "A", "target": "192.0.2.1", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_domain_record_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/domains/333/records"
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert "A record" in body["side_effects"][0]
    assert "192.0.2.1" in body["side_effects"][0]


async def test_domain_record_create_dry_run_still_validates_domain_id(
    sample_config: Config,
) -> None:
    """Missing domain_id must error out regardless of dry_run."""
    result = await handle_linode_domain_record_create(
        {"type": "A", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "domain_id is required" in result[0].text


async def test_domain_record_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the record via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_domain_record.return_value = {"id": 555, "type": "A"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_domain_record_update(
            {
                "domain_id": 333,
                "record_id": 555,
                "target": "192.0.2.2",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_domain_record_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/domains/333/records/555"
        assert any("192.0.2.2" in s for s in body["side_effects"])
        mock_client.get_domain_record.assert_awaited_once_with(333, 555)
        mock_client.update_domain_record.assert_not_called()


async def test_handle_linode_domain_record_delete(sample_config: Config) -> None:
    """Test linode_domain_record_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_domain_record.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_delete(
            {"domain_id": 12345, "record_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_volume_create_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_create tool without confirmation."""
    result = await handle_linode_volume_create(
        {"label": "my-volume", "region": "us-east"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_create(sample_config: Config) -> None:
    """Test linode_volume_create tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="creating",
        size=20,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_create(
            {"label": "my-volume", "region": "us-east", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "my-volume" in result[0].text


async def test_handle_linode_volume_clone_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_clone tool without confirmation."""
    result = await handle_linode_volume_clone(
        {"volume_id": 12345, "label": "my-volume-clone"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_clone_requires_label(
    sample_config: Config,
) -> None:
    """Test linode_volume_clone validates label."""
    result = await handle_linode_volume_clone(
        {"volume_id": 12345, "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_volume_clone(sample_config: Config) -> None:
    """Test linode_volume_clone tool."""
    mock_volume = Volume(
        id=23456,
        label="my-volume-clone",
        status="creating",
        size=20,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume-clone",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.clone_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_clone(
            {
                "volume_id": 12345,
                "label": "my-volume-clone",
                "confirm": True,
            },
            sample_config,
        )

        mock_client.clone_volume.assert_awaited_once_with(12345, "my-volume-clone")
        assert len(result) == 1
        assert "my-volume-clone" in result[0].text


async def test_handle_linode_volume_attach(sample_config: Config) -> None:
    """Test linode_volume_attach tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="active",
        size=20,
        region="us-east",
        linode_id=54321,
        linode_label="my-linode",
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.attach_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_attach(
            {"volume_id": 12345, "linode_id": 54321, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "attached" in result[0].text.lower()


async def test_handle_linode_volume_detach(sample_config: Config) -> None:
    """Test linode_volume_detach tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.detach_volume.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_detach(
            {"volume_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "detach" in result[0].text.lower()


async def test_handle_linode_volume_resize_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_resize tool without confirmation."""
    result = await handle_linode_volume_resize(
        {"volume_id": 12345, "size": 40}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_resize(sample_config: Config) -> None:
    """Test linode_volume_resize tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="resizing",
        size=40,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resize_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_resize(
            {"volume_id": 12345, "size": 40, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "resize" in result[0].text.lower()


async def test_handle_linode_volume_update_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_update tool without confirmation."""
    result = await handle_linode_volume_update(
        {"volume_id": 12345, "label": "renamed-volume"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_update_requires_change(
    sample_config: Config,
) -> None:
    """Test linode_volume_update requires label or tags."""
    result = await handle_linode_volume_update(
        {"volume_id": 12345, "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "label or tags" in result[0].text.lower()


async def test_handle_linode_volume_update(sample_config: Config) -> None:
    """Test linode_volume_update tool."""
    mock_volume = Volume(
        id=12345,
        label="renamed-volume",
        status="active",
        size=20,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_renamed-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        tags=["prod"],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_update(
            {
                "volume_id": 12345,
                "label": "renamed-volume",
                "tags": ["prod"],
                "confirm": True,
            },
            sample_config,
        )

        mock_client.update_volume.assert_awaited_once_with(
            volume_id=12345,
            label="renamed-volume",
            tags=["prod"],
        )
        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_volume_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_delete tool without confirmation."""
    result = await handle_linode_volume_delete({"volume_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_delete(sample_config: Config) -> None:
    """Test linode_volume_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_volume.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_delete(
            {"volume_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_volume_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_volume_create(
        {"label": "vol", "region": "us-east", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_volume_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/volumes"
    assert body["current_state"] is None
    assert any("us-east" in s for s in body["side_effects"])
    assert body["warnings"]
    assert "confirm=true" not in result[0].text


async def test_volume_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_volume_create(
        {"region": "us-east", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_volume_clone_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches the source volume via GET and never clones."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = {"id": 333, "label": "src"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_clone(
            {"volume_id": 333, "label": "copy", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_clone"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/volumes/333/clone"
        mock_client.get_volume.assert_awaited_once_with(333)
        mock_client.clone_volume.assert_not_called()


async def test_volume_attach_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never attaches."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = {"id": 333, "label": "vol"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_attach(
            {"volume_id": 333, "linode_id": 444, "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_attach"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/volumes/333/attach"
        mock_client.get_volume.assert_awaited_once_with(333)
        mock_client.attach_volume.assert_not_called()
        assert any("444" in s for s in body["side_effects"])


async def test_volume_attach_dry_run_still_validates_volume_id(
    sample_config: Config,
) -> None:
    """Missing volume_id must error out regardless of dry_run."""
    result = await handle_linode_volume_attach(
        {"linode_id": 444, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "volume_id is required" in result[0].text


async def test_volume_detach_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never detaches."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = {"id": 333, "label": "vol"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_detach(
            {"volume_id": 333, "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_detach"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/volumes/333/detach"
        mock_client.get_volume.assert_awaited_once_with(333)
        mock_client.detach_volume.assert_not_called()


async def test_volume_detach_dry_run_surfaces_current_attachment(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: detach names the instance the volume is on."""
    from linodemcp.linode import Volume

    attached = Volume(
        id=333,
        label="vol",
        status="active",
        size=50,
        region="us-east",
        linode_id=444,
        linode_label="web",
        filesystem_path="/dev/disk/by-id/x",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = attached
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_detach(
            {"volume_id": 333, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert any("444" in s for s in body["side_effects"])
        mock_client.detach_volume.assert_not_called()


async def test_volume_resize_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never resizes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = {"id": 333, "label": "vol"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_resize(
            {"volume_id": 333, "size": 100, "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_resize"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/volumes/333/resize"
        mock_client.get_volume.assert_awaited_once_with(333)
        mock_client.resize_volume.assert_not_called()


async def test_volume_resize_dry_run_surfaces_size_change(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: resize names the size change + grow-only warning."""
    from linodemcp.linode import Volume

    current = Volume(
        id=333,
        label="vol",
        status="active",
        size=50,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/x",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = current
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_resize(
            {"volume_id": 333, "size": 100, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        effect = body["side_effects"][0]
        assert "50 GB" in effect
        assert "100 GB" in effect
        assert body["warnings"]
        mock_client.resize_volume.assert_not_called()


async def test_volume_update_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true fetches state via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = {"id": 333, "label": "vol"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_update(
            {"volume_id": 333, "label": "renamed", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/volumes/333"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_volume.assert_awaited_once_with(333)
        mock_client.update_volume.assert_not_called()


async def test_volume_update_dry_run_still_validates_change(
    sample_config: Config,
) -> None:
    """A volume_id with no label/tags must error out regardless of dry_run."""
    result = await handle_linode_volume_update(
        {"volume_id": 333, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "label or tags is required" in result[0].text


async def test_linode_nodebalancer_firewalls_update_tool_definition() -> None:
    """Test linode_nodebalancer_firewalls_update tool definition."""
    tool, capability = create_linode_nodebalancer_firewalls_update_tool()

    assert tool.name == "linode_nodebalancer_firewalls_update"
    assert capability == Capability.Write
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["firewall_ids"]["type"] == "array"
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "firewall_ids",
        "confirm",
    ]


async def test_handle_linode_nodebalancer_firewalls_update(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewalls_update tool."""
    mock_firewalls = {
        "data": [{"id": 123, "label": "web-fw"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewalls_update(
            {
                "nodebalancer_id": 8,
                "firewall_ids": [123],
                "page": 1,
                "page_size": 25,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["data"][0]["id"] == 123
        mock_client.update_nodebalancer_firewalls.assert_called_once_with(
            8, [123], page=1, page_size=25
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"nodebalancer_id": 8, "firewall_ids": [123]}, "confirm must be true"),
        (
            {"nodebalancer_id": 8, "firewall_ids": [123], "confirm": False},
            "confirm must be true",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [123], "confirm": "true"},
            "confirm must be true",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [123], "confirm": 1},
            "confirm must be true",
        ),
        (
            {"firewall_ids": [123], "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 0, "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "8", "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": True, "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1/2", "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1?x", "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "..", "firewall_ids": [123], "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": 8, "confirm": True},
            "firewall_ids must be a list of positive integers",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [0], "confirm": True},
            "firewall_ids must be a list of positive integers",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": ["123"], "confirm": True},
            "firewall_ids must be a list of positive integers",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [True], "confirm": True},
            "firewall_ids must be a list of positive integers",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [], "page": 0, "confirm": True},
            "page must be at least 1",
        ),
        (
            {"nodebalancer_id": 8, "firewall_ids": [], "page": "1", "confirm": True},
            "page must be an integer",
        ),
        (
            {
                "nodebalancer_id": 8,
                "firewall_ids": [],
                "page_size": 24,
                "confirm": True,
            },
            "page_size must be at least 25",
        ),
        (
            {
                "nodebalancer_id": 8,
                "firewall_ids": [],
                "page_size": 501,
                "confirm": True,
            },
            "page_size must be at most 500",
        ),
    ],
)
async def test_handle_linode_nodebalancer_firewalls_update_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer firewall update rejects invalid arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_firewalls_update(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


def test_linode_nodebalancer_config_rebuild_tool_definition() -> None:
    """Test linode_nodebalancer_config_rebuild tool definition."""
    tool, capability = create_linode_nodebalancer_config_rebuild_tool()

    assert tool.name == "linode_nodebalancer_config_rebuild"
    assert capability == Capability.Write
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "config_id",
        "confirm",
    ]


async def test_handle_linode_nodebalancer_config_rebuild(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_rebuild tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.rebuild_nodebalancer_config.return_value = {"rebuilt": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_rebuild(
            {"nodebalancer_id": 8, "config_id": 6, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == {"rebuilt": True}
        mock_client.rebuild_nodebalancer_config.assert_called_once_with(8, 6)


async def test_handle_linode_nodebalancer_config_rebuild_empty_response(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_rebuild formats an empty response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.rebuild_nodebalancer_config.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_rebuild(
            {"nodebalancer_id": 8, "config_id": 6, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["nodebalancer_id"] == 8
        assert data["config_id"] == 6
        assert "rebuild requested" in data["message"]
        mock_client.rebuild_nodebalancer_config.assert_called_once_with(8, 6)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "confirm must be true"),
        ({"confirm": False}, "confirm must be true"),
        ({"confirm": "true"}, "confirm must be true"),
        ({"confirm": 1}, "confirm must be true"),
        (
            {"nodebalancer_id": 0, "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "8", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": True, "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1/2", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1?x", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "..", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": 0, "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "6", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": False, "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "4/5", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "4?x", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "..", "confirm": True},
            "config_id",
        ),
    ],
)
async def test_handle_linode_nodebalancer_config_rebuild_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer config rebuild rejects invalid arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_config_rebuild(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_config_rebuild_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_rebuild error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.rebuild_nodebalancer_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_rebuild(
            {"nodebalancer_id": 8, "config_id": 6, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_firewalls_update_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewalls_update error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_firewalls.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewalls_update(
            {"nodebalancer_id": 8, "firewall_ids": [], "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_create_no_confirm(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_create tool without confirmation."""
    result = await handle_linode_nodebalancer_create(
        {"region": "us-east"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_nodebalancer_create(sample_config: Config) -> None:
    """Test linode_nodebalancer_create tool."""
    mock_nodebalancer = NodeBalancer(
        id=12345,
        label="my-nodebalancer",
        region="us-east",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2600:3c03::1",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        client_conn_throttle=0,
        transfer=Transfer(in_=100, out=200, total=300),
        tags=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_create(
            {"region": "us-east", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "my-nodebalancer" in result[0].text or "12345" in result[0].text


async def test_handle_linode_nodebalancer_update(sample_config: Config) -> None:
    """Test linode_nodebalancer_update tool."""
    mock_nodebalancer = NodeBalancer(
        id=12345,
        label="updated-nodebalancer",
        region="us-east",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2600:3c03::1",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        client_conn_throttle=5,
        transfer=Transfer(in_=100, out=200, total=300),
        tags=[],
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_update(
            {
                "nodebalancer_id": 12345,
                "label": "updated-nodebalancer",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_nodebalancer_delete(sample_config: Config) -> None:
    """Test linode_nodebalancer_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_nodebalancer.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_delete(
            {"nodebalancer_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_nodebalancer_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = {
            "id": 444,
            "label": "prod-lb",
            "region": "us-east",
        }
        mock_client.list_nodebalancer_configs.return_value = {"data": []}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_delete(
            {"nodebalancer_id": 444, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_nodebalancer_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/nodebalancers/444"
        mock_client.get_nodebalancer.assert_awaited_once_with(444)
        mock_client.delete_nodebalancer.assert_not_called()


async def test_nodebalancer_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = {"id": 444, "label": "prod-lb"}
        mock_client.list_nodebalancer_configs.return_value = {"data": []}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_delete(
            {"nodebalancer_id": 444, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_nodebalancer_delete_dry_run_surfaces_config_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: configs appear as cascade_deleted dependencies."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = {"id": 444, "label": "prod-lb"}
        mock_client.list_nodebalancer_configs.return_value = {
            "data": [
                {"id": 10, "port": 80, "protocol": "http"},
                {"id": 11, "port": 443, "protocol": "https"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_delete(
            {"nodebalancer_id": 444, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 2
        assert all(d["kind"] == "nodebalancer_config" for d in deps)
        assert all(d["action"] == "cascade_deleted" for d in deps)
        assert body["warnings"]
        mock_client.delete_nodebalancer.assert_not_called()


async def test_nodebalancer_delete_dry_run_still_validates_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Missing nodebalancer_id must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_delete(
        {"dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "nodebalancer_id is required" in result[0].text


async def test_nodebalancer_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_nodebalancer_create(
        {"region": "us-east", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_nodebalancer_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/nodebalancers"
    assert body["current_state"] is None
    assert any("us-east" in s for s in body["side_effects"])
    assert body["warnings"]
    assert "confirm=true" not in result[0].text


async def test_nodebalancer_create_dry_run_still_validates_region(
    sample_config: Config,
) -> None:
    """Missing region must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_create({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_nodebalancer_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = {"id": 444, "label": "prod-lb"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_update(
            {"nodebalancer_id": 444, "label": "renamed", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_nodebalancer_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/nodebalancers/444"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_nodebalancer.assert_awaited_once_with(444)
        mock_client.update_nodebalancer.assert_not_called()


async def test_nodebalancer_update_dry_run_still_validates_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Missing nodebalancer_id must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_update({"dry_run": True}, sample_config)

    assert len(result) == 1
    assert "nodebalancer_id is required" in result[0].text


async def test_networking_ip_allocate_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the allocate with no resource state and no call."""
    from linodemcp.tools.linode_instance_ips import (
        handle_linode_networking_ip_allocate,
    )

    result = await handle_linode_networking_ip_allocate(
        {"linode_id": 123, "type": "ipv4", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_networking_ip_allocate"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/ips"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_networking_ip_allocate_dry_run_still_validates_linode_id(
    sample_config: Config,
) -> None:
    """Missing linode_id must error out regardless of dry_run."""
    from linodemcp.tools.linode_instance_ips import (
        handle_linode_networking_ip_allocate,
    )

    result = await handle_linode_networking_ip_allocate(
        {"type": "ipv4", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "linode_id must be an integer" in result[0].text


async def test_ipv6_range_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = list(
        await handle_linode_ipv6_range_create(
            {"prefix_length": 64, "linode_id": 123, "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_ipv6_range_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/ipv6/ranges"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text
    assert len(body["side_effects"]) == 1
    assert "/64" in body["side_effects"][0]
    assert "instance 123" in body["side_effects"][0]


async def test_ipv6_range_create_dry_run_still_validates_prefix_length(
    sample_config: Config,
) -> None:
    """Missing prefix_length must error out regardless of dry_run."""
    result = list(
        await handle_linode_ipv6_range_create({"dry_run": True}, sample_config)
    )

    assert len(result) == 1
    assert "prefix_length" in result[0].text


async def test_linode_nodebalancer_config_node_update_tool_definition() -> None:
    """Test linode_nodebalancer_config_node_update tool definition."""
    tool, capability = create_linode_nodebalancer_config_node_update_tool()
    assert tool.name == "linode_nodebalancer_config_node_update"
    assert capability == Capability.Write
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["node_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["mode"]["enum"] == [
        "accept",
        "reject",
        "drain",
        "backup",
    ]
    assert tool.inputSchema["properties"]["weight"]["maximum"] == 255
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "config_id",
        "node_id",
        "confirm",
    ]


async def test_handle_linode_nodebalancer_config_node_update(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_update tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config_node.return_value = {
            "id": 7,
            "address": "192.0.2.7:80",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_update(
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "address": "192.0.2.7:80",
                "label": "web-7",
                "mode": "drain",
                "subnet_id": 123,
                "weight": 50,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == {"id": 7, "address": "192.0.2.7:80"}
        mock_client.update_nodebalancer_config_node.assert_called_once_with(
            12345,
            6,
            7,
            {
                "address": "192.0.2.7:80",
                "label": "web-7",
                "mode": "drain",
                "subnet_id": 123,
                "weight": 50,
            },
        )


async def test_handle_linode_nodebalancer_config_node_update_empty_response(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_update formats an empty response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config_node.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_update(
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "mode": "reject",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["nodebalancer_id"] == 12345
        assert data["config_id"] == 6
        assert data["node_id"] == 7
        assert "update requested" in data["message"]
        mock_client.update_nodebalancer_config_node.assert_called_once_with(
            12345, 6, 7, {"mode": "reject"}
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "confirm must be true"),
        ({"confirm": False}, "confirm must be true"),
        ({"confirm": "true"}, "confirm must be true"),
        ({"confirm": 1}, "confirm must be true"),
        (
            {"nodebalancer_id": 12345, "config_id": 6, "node_id": 7, "confirm": True},
            "at least one update field is required",
        ),
        (
            {"nodebalancer_id": 0, "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "12345", "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": True, "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1/2", "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1?x", "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "..", "config_id": 6, "node_id": 7, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": 0, "node_id": 7, "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": "6", "node_id": 7, "confirm": True},
            "config_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": False,
                "node_id": 7,
                "confirm": True,
            },
            "config_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": "4/5",
                "node_id": 7,
                "confirm": True,
            },
            "config_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": "4?x",
                "node_id": 7,
                "confirm": True,
            },
            "config_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": "..",
                "node_id": 7,
                "confirm": True,
            },
            "config_id",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": 6, "node_id": 0, "confirm": True},
            "node_id",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": 6, "node_id": "7", "confirm": True},
            "node_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": False,
                "confirm": True,
            },
            "node_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": "7/8",
                "confirm": True,
            },
            "node_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": "7?x",
                "confirm": True,
            },
            "node_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": "..",
                "confirm": True,
            },
            "node_id",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "address": "",
                "confirm": True,
            },
            "address must be a non-empty string",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "label": "ab",
                "confirm": True,
            },
            "label must be 3 to 32 characters",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "mode": "invalid",
                "confirm": True,
            },
            "mode must be one of accept, reject, drain, backup",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "subnet_id": 0,
                "confirm": True,
            },
            "subnet_id must be at least 1",
        ),
        (
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "weight": 256,
                "confirm": True,
            },
            "weight must be at most 255",
        ),
    ],
)
async def test_handle_linode_nodebalancer_config_node_update_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer config node update rejects invalid arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_config_node_update(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_config_node_update_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_update error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config_node.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_update(
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "mode": "reject",
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_config_delete_tool_definition() -> None:
    """Test linode_nodebalancer_config_delete tool definition."""
    tool, capability = create_linode_nodebalancer_config_delete_tool()
    assert tool.name == "linode_nodebalancer_config_delete"
    assert capability == Capability.Destroy
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "config_id",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_nodebalancer_config_delete(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_nodebalancer_config.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_delete(
            {"nodebalancer_id": 12345, "config_id": 6, "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "deleted" in result[0].text.lower()
    mock_client.delete_nodebalancer_config.assert_called_once_with(12345, 6)


@pytest.mark.parametrize("confirm_value", [False, None, "true", 1])
async def test_handle_linode_nodebalancer_config_delete_confirm_rejected(
    confirm_value: object, sample_config: Config
) -> None:
    """Missing, false, string, and numeric confirm are rejected before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_delete(
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "confirm": confirm_value,
            },
            sample_config,
        )

    assert len(result) == 1
    assert "confirm must be true" in result[0].text.lower()
    mock_client.delete_nodebalancer_config.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (
            {"config_id": 6, "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": "1/2", "config_id": 6, "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": "6?x", "confirm": True},
            "config_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 12345, "config_id": "..", "confirm": True},
            "config_id must be a positive integer",
        ),
    ],
)
async def test_handle_linode_nodebalancer_config_delete_invalid_args(
    arguments: dict[str, object], expected: str, sample_config: Config
) -> None:
    """Invalid path arguments are rejected before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_delete(
            arguments, sample_config
        )

    assert len(result) == 1
    assert expected in result[0].text
    mock_client.delete_nodebalancer_config.assert_not_called()


async def test_nodebalancer_config_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.return_value = {
            "id": 222,
            "port": 80,
            "protocol": "http",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_delete(
            {"nodebalancer_id": 111, "config_id": 222, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_nodebalancer_config_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/nodebalancers/111/configs/222"
        mock_client.get_nodebalancer_config.assert_awaited_once_with(111, 222)
        mock_client.delete_nodebalancer_config.assert_not_called()


async def test_nodebalancer_config_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.return_value = {"id": 222}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_delete(
            {"nodebalancer_id": 111, "config_id": 222, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        assert "confirm must be true" not in result[0].text


async def test_nodebalancer_config_delete_dry_run_still_validates_ids(
    sample_config: Config,
) -> None:
    """Missing or invalid IDs must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_config_delete(
        {"config_id": 222, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_linode_nodebalancer_config_node_delete_tool_definition() -> None:
    """Test linode_nodebalancer_config_node_delete tool definition."""
    tool, _ = create_linode_nodebalancer_config_node_delete_tool()
    assert tool.name == "linode_nodebalancer_config_node_delete"
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "config_id",
        "node_id",
        "confirm",
    ]


async def test_handle_linode_nodebalancer_config_node_delete(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_nodebalancer_config_node.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_delete(
            {"nodebalancer_id": 12345, "config_id": 6, "node_id": 7, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()
        mock_client.delete_nodebalancer_config_node.assert_called_once_with(12345, 6, 7)


@pytest.mark.parametrize("confirm_value", [False, None])
async def test_handle_linode_nodebalancer_config_node_delete_confirm_rejected(
    confirm_value: object, sample_config: Config
) -> None:
    """Missing/false confirm is rejected before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_delete(
            {
                "nodebalancer_id": 12345,
                "config_id": 6,
                "node_id": 7,
                "confirm": confirm_value,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" in result[0].text.lower()
        mock_client.delete_nodebalancer_config_node.assert_not_called()


async def test_handle_linode_nodebalancer_config_node_delete_missing_args(
    sample_config: Config,
) -> None:
    """Missing required args are rejected before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_delete(
            {"nodebalancer_id": 12345, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "config_id is required" in result[0].text.lower()
        mock_client.delete_nodebalancer_config_node.assert_not_called()


async def test_nodebalancer_config_node_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config_node.return_value = {
            "id": 333,
            "address": "10.0.0.5:80",
            "label": "web-01",
            "mode": "accept",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_delete(
            {
                "nodebalancer_id": 111,
                "config_id": 222,
                "node_id": 333,
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_nodebalancer_config_node_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert (
            body["would_execute"]["path"] == "/nodebalancers/111/configs/222/nodes/333"
        )
        mock_client.get_nodebalancer_config_node.assert_awaited_once_with(111, 222, 333)
        mock_client.delete_nodebalancer_config_node.assert_not_called()


async def test_nodebalancer_config_node_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config_node.return_value = {"id": 333}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_delete(
            {
                "nodebalancer_id": 111,
                "config_id": 222,
                "node_id": 333,
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_nodebalancer_config_node_delete_dry_run_still_validates_ids(
    sample_config: Config,
) -> None:
    """Missing any ID must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_config_node_delete(
        {"nodebalancer_id": 111, "config_id": 222, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "node_id is required" in result[0].text


# Object Storage tools


async def test_handle_linode_object_storage_buckets_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_buckets_list tool."""
    mock_buckets = [
        {
            "label": "my-bucket",
            "region": "us-east-1",
            "hostname": "my-bucket.us-east-1.linodeobjects.com",
            "created": "2024-01-01T00:00:00",
            "objects": 42,
            "size": 1024000,
            "cluster": "us-east-1",
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets.return_value = mock_buckets
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_list({}, sample_config)

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_buckets.assert_called_once()


async def test_handle_linode_object_storage_buckets_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_buckets_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_buckets_region_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_buckets_region_list tool."""
    mock_buckets = [
        {
            "label": "app-data",
            "region": "us-ord",
            "hostname": "app-data.us-ord-1.linodeobjects.com",
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets_for_region.return_value = mock_buckets
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_region_list(
            {"region_id": "us-ord"}, sample_config
        )

        assert len(result) == 1
        assert "app-data" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_buckets_for_region.assert_called_once_with(
            "us-ord"
        )


async def test_handle_linode_object_storage_buckets_region_list_missing_region_id(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list with missing region_id."""
    result = await handle_linode_object_storage_buckets_region_list({}, sample_config)

    assert len(result) == 1
    assert "region_id is required" in result[0].text


async def test_handle_linode_object_storage_buckets_region_list_rejects_bad_region_id(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list rejects malformed path values."""
    for region_id in ("us/ord", "us?ord", ".."):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_object_storage_buckets_region_list(
                {"region_id": region_id}, sample_config
            )

            assert len(result) == 1
            assert "region_id must be a valid region or cluster ID" in result[0].text
            mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_buckets_region_list_error(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets_for_region.side_effect = Exception(
            "API error"
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_region_list(
            {"region_id": "us-ord"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_bucket_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get tool."""
    mock_bucket = {
        "label": "my-bucket",
        "region": "us-east-1",
        "hostname": "my-bucket.us-east-1.linodeobjects.com",
        "created": "2024-01-01T00:00:00",
        "objects": 42,
        "size": 1024000,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket.return_value = mock_bucket
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        mock_client.get_object_storage_bucket.assert_called_once_with(
            "us-east-1", "my-bucket"
        )


async def test_handle_linode_object_storage_bucket_get_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get with missing region."""
    result = await handle_linode_object_storage_bucket_get(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_bucket_get_missing_label(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get with missing label."""
    result = await handle_linode_object_storage_bucket_get(
        {"region": "us-east-1"}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"region": "us/east", "label": "my-bucket"}, "region must be a valid"),
        ({"region": "us-east", "label": "bad/bucket"}, "label must be a valid"),
        ({"region": "us-east?x=y", "label": "my-bucket"}, "region must be a valid"),
        ({"region": "us-east", "label": ".."}, "label must be a valid"),
    ],
)
async def test_handle_linode_object_storage_bucket_get_rejects_bad_path_params(
    arguments: dict[str, object], message: str, sample_config: Config
) -> None:
    """Object Storage bucket get rejects malformed path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_bucket_get(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"region": "us/east", "label": "my-bucket"}, "region must be a valid"),
        ({"region": "us-east", "label": "bad?bucket"}, "label must be a valid"),
        ({"region": "..", "label": "my-bucket"}, "region must be a valid"),
    ],
)
async def test_handle_linode_object_storage_bucket_contents_rejects_bad_path_params(
    arguments: dict[str, object], message: str, sample_config: Config
) -> None:
    """Bucket contents rejects malformed path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_bucket_contents(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_bucket_contents(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents tool."""
    mock_response = {
        "data": [
            {
                "name": "photos/cat.jpg",
                "etag": "abc123",
                "last_modified": "2024-06-01T00:00:00",
                "owner": "user",
                "size": 512000,
                "is_prefix": False,
            },
        ],
        "is_truncated": False,
        "next_marker": "",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_bucket_contents.return_value = mock_response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_contents(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "cat.jpg" in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_object_storage_bucket_contents_with_prefix(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents with prefix filter."""
    mock_response = {
        "data": [
            {
                "name": "images/logo.png",
                "etag": "def456",
                "last_modified": "2024-06-01T00:00:00",
                "owner": "user",
                "size": 256000,
                "is_prefix": False,
            },
        ],
        "is_truncated": True,
        "next_marker": "images/next.png",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_bucket_contents.return_value = mock_response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_contents(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "prefix": "images/",
                "delimiter": "/",
            },
            sample_config,
        )

        assert len(result) == 1
        assert "logo.png" in result[0].text
        assert "next_marker" in result[0].text
        assert "prefix=images/" in result[0].text


async def test_handle_linode_object_storage_bucket_contents_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents with missing region."""
    result = await handle_linode_object_storage_bucket_contents(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_linode_object_storage_cluster_get_removed_from_registry() -> None:
    """Deprecated Object Storage cluster get tool should not be registered."""
    from linodemcp.server import get_tool_registry
    from linodemcp.version import FEATURE_TOOLS_LIST, REMOVED_FEATURE_TOOLS_LIST

    registry = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_object_storage_cluster_get" not in registry
    assert "linode_object_storage_cluster_get" not in FEATURE_TOOLS_LIST.split(",")
    assert "linode_object_storage_cluster_get" in REMOVED_FEATURE_TOOLS_LIST.split(",")


async def test_handle_linode_object_storage_types_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_types_list tool."""
    mock_types = [
        {
            "id": "objectstorage",
            "label": "Object Storage",
            "price": {"hourly": 0.02, "monthly": 5.0},
            "transfer": 1000,
            "region": "us-east",
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_types_list({}, sample_config)

        assert len(result) == 1
        assert "objectstorage" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_types.assert_called_once()


async def test_handle_linode_object_storage_types_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_types_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_types.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_types_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


# Phase 2: Access Key & Transfer Tests


def test_linode_object_storage_endpoints_list_tool_schema() -> None:
    """Object Storage endpoints list schema has no route-specific arguments."""
    tool, capability = create_linode_object_storage_endpoints_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_endpoints_list"
    assert "required" not in tool.inputSchema


async def test_handle_linode_object_storage_endpoints_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_endpoints_list tool."""
    mock_endpoints = [
        {
            "endpoint_type": "E1",
            "region": "us-sea",
            "s3_endpoint": "us-sea-1.linodeobjects.com",
        }
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_endpoints.return_value = mock_endpoints
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_endpoints_list({}, sample_config)

        assert len(result) == 1
        assert "us-sea-1.linodeobjects.com" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_endpoints.assert_called_once_with()


async def test_handle_linode_object_storage_endpoints_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_endpoints_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_endpoints.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_endpoints_list({}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve Object Storage endpoints" in result[0].text


async def test_handle_linode_object_storage_keys_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_keys_list tool."""
    mock_keys = [
        {
            "id": 1,
            "label": "my-key",
            "access_key": "AKIAIOSFODNN7EXAMPLE",
            "secret_key": "[REDACTED]",
            "limited": False,
            "bucket_access": None,
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_keys_list({}, sample_config)

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_keys.assert_called_once()


async def test_handle_linode_object_storage_keys_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_keys_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_keys.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_keys_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_key_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_get tool."""
    mock_key = {
        "id": 42,
        "label": "my-key",
        "access_key": "AKIAIOSFODNN7EXAMPLE",
        "secret_key": "[REDACTED]",
        "limited": True,
        "bucket_access": [
            {
                "bucket_name": "my-bucket",
                "region": "us-east-1",
                "permissions": "read_only",
            },
        ],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_key_get(
            {"key_id": 42}, sample_config
        )

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert "my-bucket" in result[0].text
        mock_client.get_object_storage_key.assert_called_once_with(42)


async def test_handle_linode_object_storage_key_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_get with missing key_id."""
    result = await handle_linode_object_storage_key_get({}, sample_config)

    assert len(result) == 1
    assert "key_id is required" in result[0].text


def test_linode_object_storage_quotas_list_tool_schema() -> None:
    """Quota list schema has no required route-specific arguments."""
    tool, capability = create_linode_object_storage_quotas_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quotas_list"
    assert "required" not in tool.inputSchema


async def test_handle_linode_object_storage_quotas_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quotas_list tool."""
    mock_quotas = [
        {
            "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
            "quota_limit": 1000,
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_quotas.return_value = mock_quotas
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quotas_list({}, sample_config)

        assert len(result) == 1
        assert "obj-buckets-us-sea-1.linodeobjects.com" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_quotas.assert_called_once_with()


async def test_handle_linode_object_storage_quotas_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quotas_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_quotas.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quotas_list({}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve Object Storage quotas" in result[0].text


def test_linode_object_storage_quota_get_tool_schema() -> None:
    """Quota get schema requires the quota ID."""
    tool, capability = create_linode_object_storage_quota_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quota_get"
    assert tool.inputSchema["required"] == ["obj_quota_id"]
    assert tool.inputSchema["properties"]["obj_quota_id"]["type"] == "string"


async def test_handle_linode_object_storage_quota_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_get tool."""
    mock_quota = {
        "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
        "quota_limit": 1000,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_quota.return_value = mock_quota
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_get(
            {"obj_quota_id": "obj-buckets-us-sea-1.linodeobjects.com"},
            sample_config,
        )

        assert len(result) == 1
        assert "quota_id" in result[0].text
        assert "obj-buckets-us-sea-1.linodeobjects.com" in result[0].text
        mock_client.get_object_storage_quota.assert_called_once_with(
            "obj-buckets-us-sea-1.linodeobjects.com"
        )


async def test_handle_linode_object_storage_quota_get_requires_id(
    sample_config: Config,
) -> None:
    """Quota get requires obj_quota_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_get({}, sample_config)

    assert len(result) == 1
    assert "obj_quota_id must be a valid Object Storage quota ID" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "bad_id",
    ["quota/with/slash", "quota?x=1", "quota#x", "..", "quota..id", "", 123, True],
)
async def test_handle_linode_object_storage_quota_get_rejects_bad_id(
    sample_config: Config, bad_id: Any
) -> None:
    """Quota get rejects malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_get(
            {"obj_quota_id": bad_id}, sample_config
        )

    assert len(result) == 1
    assert "obj_quota_id must be a valid Object Storage quota ID" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_quota_get_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_quota.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_get(
            {"obj_quota_id": "obj-buckets-us-sea-1.linodeobjects.com"},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed to retrieve Object Storage quota" in result[0].text


def test_linode_object_storage_quota_usage_tool_schema() -> None:
    """Quota usage schema requires the quota ID."""
    tool, capability = create_linode_object_storage_quota_usage_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quota_usage"
    assert tool.inputSchema["required"] == ["obj_quota_id"]
    assert tool.inputSchema["properties"]["obj_quota_id"]["type"] == "integer"


async def test_handle_linode_object_storage_quota_usage(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_usage tool."""
    mock_usage = {"quota_id": 123, "usage": {"objects": 7}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_quota_usage.return_value = mock_usage
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_usage(
            {"obj_quota_id": 123}, sample_config
        )

        assert len(result) == 1
        assert "quota_id" in result[0].text
        assert "123" in result[0].text
        mock_client.get_object_storage_quota_usage.assert_called_once_with(123)


async def test_handle_linode_object_storage_quota_usage_requires_id(
    sample_config: Config,
) -> None:
    """Quota usage requires obj_quota_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_usage({}, sample_config)

    assert len(result) == 1
    assert "obj_quota_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("bad_id", ["1/2", "1?x=1", "..", 0, -1, True, 1.9])
async def test_handle_linode_object_storage_quota_usage_rejects_bad_id(
    sample_config: Config, bad_id: Any
) -> None:
    """Quota usage rejects malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_usage(
            {"obj_quota_id": bad_id}, sample_config
        )

    assert len(result) == 1
    assert "obj_quota_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_quota_usage_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_usage tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_quota_usage.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_usage(
            {"obj_quota_id": 123}, sample_config
        )

        assert len(result) == 1
        assert "Failed to retrieve Object Storage quota usage" in result[0].text


async def test_handle_linode_object_storage_transfer(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_transfer tool."""
    mock_transfer = {"used": 1073741824}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_transfer.return_value = mock_transfer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer({}, sample_config)

        assert len(result) == 1
        assert "1073741824" in result[0].text
        mock_client.get_object_storage_transfer.assert_called_once()


async def test_handle_linode_object_storage_transfer_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_transfer tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_transfer.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get tool."""
    mock_access = {"acl": "public-read", "cors_enabled": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.return_value = mock_access
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "public-read" in result[0].text
        mock_client.get_object_storage_bucket_access.assert_called_once_with(
            "us-east-1", "my-bucket"
        )


async def test_handle_linode_object_storage_bucket_access_get_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get with missing region."""
    result = await handle_linode_object_storage_bucket_access_get(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get_missing_label(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get with missing label."""
    result = await handle_linode_object_storage_bucket_access_get(
        {"region": "us-east-1"}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.side_effect = Exception(
            "API error"
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text


# Phase 3: Object Storage Write Bucket Tool Tests


def test_linode_object_storage_cancel_tool_schema() -> None:
    """Object Storage cancel tool should require boolean confirmation."""
    tool, capability = create_linode_object_storage_cancel_tool()

    assert capability is Capability.Write
    assert tool.name == "linode_object_storage_cancel"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_object_storage_cancel_requires_boolean_true_confirm(
    confirm: object,
    sample_config: Config,
) -> None:
    """Object Storage cancel should reject non-true confirm before client use."""
    arguments: dict[str, object] = {}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = await handle_linode_object_storage_cancel(arguments, sample_config)

    assert len(result) == 1
    assert "confirm=true" in result[0].text
    mock_cls.assert_not_called()


async def test_handle_object_storage_cancel_success(
    sample_config: Config,
) -> None:
    """Object Storage cancel should call the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.cancel_object_storage.return_value = {"message": "scheduled"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_cancel(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "scheduled" in result[0].text
    mock_client.cancel_object_storage.assert_called_once_with()


async def test_handle_object_storage_cancel_error(
    sample_config: Config,
) -> None:
    """Object Storage cancel should report client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.cancel_object_storage.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_cancel(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed" in result[0].text


async def test_handle_object_storage_bucket_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket create requires confirm=true."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "my-bucket", "region": "us-east-1"},
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_create_invalid_label(
    sample_config: Config,
) -> None:
    """Test bucket create rejects invalid label."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "AB", "region": "us-east-1", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "at least 3" in result[0].text


async def test_handle_object_storage_bucket_create_invalid_acl(
    sample_config: Config,
) -> None:
    """Test bucket create rejects invalid ACL."""
    result = await handle_linode_object_storage_bucket_create(
        {
            "label": "my-bucket",
            "region": "us-east-1",
            "acl": "bad-acl",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_handle_object_storage_bucket_create_missing_region(
    sample_config: Config,
) -> None:
    """Test bucket create requires region."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "my-bucket", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_object_storage_bucket_create_success(
    sample_config: Config,
) -> None:
    """Test bucket create success."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_object_storage_bucket.return_value = {
            "label": "my-bucket",
            "region": "us-east-1",
            "created": "2024-01-01T00:00:00",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_create(
            {
                "label": "my-bucket",
                "region": "us-east-1",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "created successfully" in result[0].text


async def test_handle_object_storage_bucket_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket delete requires confirm=true."""
    result = await handle_linode_object_storage_bucket_delete(
        {"region": "us-east-1", "label": "my-bucket"},
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_delete_missing_region(
    sample_config: Config,
) -> None:
    """Test bucket delete requires region."""
    result = await handle_linode_object_storage_bucket_delete(
        {"label": "my-bucket", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_object_storage_bucket_delete_success(
    sample_config: Config,
) -> None:
    """Test bucket delete success."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_object_storage_bucket.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "deleted successfully" in result[0].text


async def test_bucket_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete.

    Decodes the JSON body so a future renaming of the v0 wire shape or
    a regression where Execute fires anyway gets caught.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket.return_value = {
            "label": "my-bucket",
            "region": "us-east-1",
            "size": 1024,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_object_storage_bucket_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert (
            body["would_execute"]["path"]
            == "/object-storage/buckets/us-east-1/my-bucket"
        )
        assert body["current_state"]["label"] == "my-bucket"
        mock_client.get_object_storage_bucket.assert_awaited_once_with(
            "us-east-1", "my-bucket"
        )
        mock_client.delete_object_storage_bucket.assert_not_called()


async def test_bucket_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate.

    Catches a regression where the confirm check accidentally fires
    before the dry-run branch.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket.return_value = {"label": "my-bucket"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_bucket_delete_dry_run_still_validates_region(
    sample_config: Config,
) -> None:
    """Missing region must error out regardless of dry_run.

    The spec says dry-run errors on missing required args the same way
    the real call would, so a regression that skips validation on
    dry-run gets caught here.
    """
    result = await handle_linode_object_storage_bucket_delete(
        {"label": "my-bucket", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_bucket_delete_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_object_storage_bucket_delete(
        {"region": "us-east-1", "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_object_storage_bucket_access_allow_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket access allow requires confirm."""
    result = await handle_linode_object_storage_bucket_access_allow(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "public-read",
        },
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_access_allow_invalid_acl(
    sample_config: Config,
) -> None:
    """Test bucket access allow rejects invalid ACL."""
    result = await handle_linode_object_storage_bucket_access_allow(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "bad-acl",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_handle_object_storage_bucket_access_allow_success(
    sample_config: Config,
) -> None:
    """Test bucket access allow success."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.allow_object_storage_bucket_access.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_allow(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "acl": "public-read",
                "cors_enabled": True,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "Access allowed" in result[0].text
        mock_client.allow_object_storage_bucket_access.assert_called_once_with(
            region="us-east-1",
            label="my-bucket",
            acl="public-read",
            cors_enabled=True,
        )


async def test_handle_object_storage_bucket_access_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket access update requires confirm."""
    result = await handle_linode_object_storage_bucket_access_update(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "public-read",
        },
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_access_update_invalid_acl(
    sample_config: Config,
) -> None:
    """Test bucket access update rejects invalid ACL."""
    result = await handle_linode_object_storage_bucket_access_update(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "bad-acl",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_handle_object_storage_bucket_access_update_success(
    sample_config: Config,
) -> None:
    """Test bucket access update success."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_storage_bucket_access.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "acl": "public-read",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "updated successfully" in result[0].text


# Phase 4: Object Storage Access Key Write Tool Tests


async def test_object_storage_key_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Key create should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "my-key"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text
    assert "confirm=true" in result[0].text
    assert "secret_key" in result[0].text


async def test_object_storage_key_create_empty_label(
    sample_config: Config,
) -> None:
    """Key create should reject empty label."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_object_storage_key_create_label_too_long(
    sample_config: Config,
) -> None:
    """Key create should reject label over 50 chars."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "a" * 51, "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "50 characters" in result[0].text


async def test_object_storage_key_create_invalid_json(
    sample_config: Config,
) -> None:
    """Key create should reject invalid bucket_access JSON."""
    result = list(
        await handle_linode_object_storage_key_create(
            {
                "label": "my-key",
                "bucket_access": "not-valid-json",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "Invalid bucket_access JSON" in result[0].text


async def test_object_storage_key_create_invalid_permissions(
    sample_config: Config,
) -> None:
    """Key create should reject invalid permissions."""
    bucket_access = json.dumps(
        [
            {
                "bucket_name": "mybucket",
                "region": "us-east-1",
                "permissions": "admin",
            }
        ]
    )
    result = list(
        await handle_linode_object_storage_key_create(
            {
                "label": "my-key",
                "bucket_access": bucket_access,
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "read_only" in result[0].text


async def test_object_storage_key_create_success(
    sample_config: Config,
) -> None:
    """Key create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_object_storage_key.return_value = {
            "id": 42,
            "label": "my-key",
            "access_key": "AKIAIOSFODNN7EXAMPLE",
            "secret_key": "wJalrXUtnFEMI/bPxRfiCYEXAMPLEKEY",
            "limited": False,
            "bucket_access": [],
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_create(
                {"label": "my-key", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "created successfully" in result[0].text
        assert "IMPORTANT" in result[0].text
        assert "ONLY ONCE" in result[0].text


async def test_object_storage_key_create_missing_env() -> None:
    """Key create should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "my-key", "confirm": True},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_object_storage_key_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Key update should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_update(
            {"key_id": 42, "label": "new-label"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_storage_key_update_invalid_key_id(
    sample_config: Config,
) -> None:
    """Key update should reject invalid key_id."""
    result = list(
        await handle_linode_object_storage_key_update(
            {"key_id": 0, "label": "new-label", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_object_storage_key_update_success(
    sample_config: Config,
) -> None:
    """Key update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_storage_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_update(
                {
                    "key_id": 42,
                    "label": "updated-key",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated successfully" in result[0].text


async def test_object_storage_key_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Key delete should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": 42},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_storage_key_delete_invalid_key_id(
    sample_config: Config,
) -> None:
    """Key delete should reject invalid key_id."""
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": -1, "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_object_storage_key_delete_success(
    sample_config: Config,
) -> None:
    """Key delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_object_storage_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_delete(
                {"key_id": 42, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "revoked successfully" in result[0].text


async def test_object_storage_key_delete_missing_env() -> None:
    """Key delete should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": 42, "confirm": True},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


# Phase 5: Presigned URLs, Object ACL & SSL Tool Tests


async def test_presigned_url_missing_name(
    sample_config: Config,
) -> None:
    """Presigned URL should fail when name is missing."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {"region": "us-east-1", "label": "my-bucket", "method": "GET"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "name" in result[0].text


async def test_presigned_url_invalid_method(
    sample_config: Config,
) -> None:
    """Presigned URL should fail with invalid method."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "DELETE",
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "GET" in result[0].text
    assert "PUT" in result[0].text


async def test_presigned_url_invalid_expires(
    sample_config: Config,
) -> None:
    """Presigned URL should fail with out of range expires_in."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "GET",
                "expires_in": 700000,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "604800" in result[0].text


async def test_presigned_url_success(
    sample_config: Config,
) -> None:
    """Presigned URL should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_presigned_url.return_value = {
            "url": "https://bucket.example.com/photo.jpg?signed=abc",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_presigned_url(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                    "method": "GET",
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "signed=abc" in result[0].text


async def test_presigned_url_missing_env() -> None:
    """Presigned URL should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "GET",
            },
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_object_acl_get_missing_name(
    sample_config: Config,
) -> None:
    """Object ACL get should fail when name is missing."""
    result = list(
        await handle_linode_object_storage_object_acl_get(
            {"region": "us-east-1", "label": "my-bucket"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "name" in result[0].text


async def test_object_acl_get_success(
    sample_config: Config,
) -> None:
    """Object ACL get should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_acl.return_value = {
            "acl": "public-read",
            "acl_xml": "<AccessControlPolicy>...</AccessControlPolicy>",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_object_acl_get(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "public-read" in result[0].text


async def test_object_acl_update_confirm_required(
    sample_config: Config,
) -> None:
    """Object ACL update should require confirm=true."""
    result = list(
        await handle_linode_object_storage_object_acl_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "acl": "public-read",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_acl_update_invalid_acl(
    sample_config: Config,
) -> None:
    """Object ACL update should fail with invalid ACL."""
    result = list(
        await handle_linode_object_storage_object_acl_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "acl": "invalid-acl",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_object_acl_update_success(
    sample_config: Config,
) -> None:
    """Object ACL update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_acl.return_value = {
            "acl": "public-read",
            "acl_xml": "<AccessControlPolicy>...</AccessControlPolicy>",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_object_acl_update(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                    "acl": "public-read",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "public-read" in result[0].text


async def test_ssl_get_success(
    sample_config: Config,
) -> None:
    """SSL get should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_bucket_ssl.return_value = {
            "ssl": True,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_get(
                {"region": "us-east-1", "label": "my-bucket"},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "true" in result[0].text


async def test_ssl_get_missing_env() -> None:
    """SSL get should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_ssl_get(
            {"region": "us-east-1", "label": "my-bucket"},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_ssl_upload_confirm_required(
    sample_config: Config,
) -> None:
    """SSL upload should require confirm=true."""
    result = list(
        await handle_linode_object_storage_ssl_upload(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "certificate": "cert",
                "private_key": "key",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_ssl_upload_success(
    sample_config: Config,
) -> None:
    """SSL upload should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.upload_bucket_ssl.return_value = {"ssl": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_upload(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "certificate": "cert",
                    "private_key": "key",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "SSL certificate uploaded" in result[0].text
        mock_client.upload_bucket_ssl.assert_awaited_once_with(
            "us-east-1", "my-bucket", "cert", "key"
        )


async def test_ssl_upload_missing_private_key(
    sample_config: Config,
) -> None:
    """SSL upload should validate private_key."""
    result = list(
        await handle_linode_object_storage_ssl_upload(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "certificate": "cert",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "private_key is required" in result[0].text


async def test_ssl_delete_confirm_required(
    sample_config: Config,
) -> None:
    """SSL delete should require confirm=true."""
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_ssl_delete_success(
    sample_config: Config,
) -> None:
    """SSL delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_bucket_ssl.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_delete(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "SSL certificate deleted" in result[0].text


async def test_ssl_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch SSL state via GET and never call delete.

    Decodes the JSON body so a future renaming of the v0 wire shape or
    a regression where Execute fires anyway gets caught.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_bucket_ssl.return_value = {"ssl": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_delete(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_object_storage_ssl_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert (
            body["would_execute"]["path"]
            == "/object-storage/buckets/us-east-1/my-bucket/ssl"
        )
        assert body["current_state"] == {"ssl": True}
        mock_client.get_bucket_ssl.assert_awaited_once_with("us-east-1", "my-bucket")
        mock_client.delete_bucket_ssl.assert_not_called()


async def test_ssl_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate.

    Catches a regression where the confirm check accidentally fires
    before the dry-run branch.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_bucket_ssl.return_value = {"ssl": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_delete(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_ssl_delete_dry_run_still_validates_region(
    sample_config: Config,
) -> None:
    """Missing region must error out regardless of dry_run.

    The spec says dry-run errors on missing required args the same way
    the real call would, so a regression that skips validation on
    dry-run gets caught here.
    """
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {"label": "my-bucket", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_ssl_delete_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {"region": "us-east-1", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_obj_bucket_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = list(
        await handle_linode_object_storage_bucket_create(
            {"label": "my-bucket", "region": "us-east-1", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_object_storage_bucket_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/object-storage/buckets"
    assert body["current_state"] is None
    assert any("my-bucket" in s for s in body["side_effects"])
    assert body["warnings"]
    assert "confirm=true" not in result[0].text


async def test_obj_bucket_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = list(
        await handle_linode_object_storage_bucket_create(
            {"region": "us-east-1", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_obj_bucket_access_allow_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch current access via GET and never apply it."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.return_value = {"acl": "private"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_bucket_access_allow(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "acl": "private",
                    "dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_object_storage_bucket_access_allow"
        assert body["would_execute"]["method"] == "POST"
        assert (
            body["would_execute"]["path"]
            == "/object-storage/buckets/us-east-1/my-bucket/access"
        )
        mock_client.get_object_storage_bucket_access.assert_awaited_once_with(
            "us-east-1", "my-bucket"
        )
        mock_client.allow_object_storage_bucket_access.assert_not_called()


async def test_obj_bucket_access_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch current access via GET and never update it."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.return_value = {"acl": "private"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_bucket_access_update(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "acl": "private",
                    "dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_object_storage_bucket_access_update"
        assert body["would_execute"]["method"] == "PUT"
        mock_client.get_object_storage_bucket_access.assert_awaited_once_with(
            "us-east-1", "my-bucket"
        )
        assert any("private" in s for s in body["side_effects"])
        mock_client.update_object_storage_bucket_access.assert_not_called()


async def test_obj_key_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the key create with no call (no secret leak)."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "my-key", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_object_storage_key_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/object-storage/keys"
    assert body["current_state"] is None
    assert any("my-key" in s for s in body["side_effects"])
    assert body["warnings"]
    assert "confirm=true" not in result[0].text


async def test_obj_key_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_obj_key_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch the key (not the secret) via GET and never update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_key.return_value = {"id": 77, "label": "my-key"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_update(
                {"key_id": 77, "label": "renamed", "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_object_storage_key_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/object-storage/keys/77"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_object_storage_key.assert_awaited_once_with(key_id=77)
        mock_client.update_object_storage_key.assert_not_called()


async def test_obj_key_update_dry_run_still_validates_key_id(
    sample_config: Config,
) -> None:
    """Missing key_id must error out regardless of dry_run."""
    result = list(
        await handle_linode_object_storage_key_update(
            {"label": "renamed", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_obj_object_acl_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch current ACL via GET and never update it."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_acl.return_value = {"acl": "private"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_object_acl_update(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "object.txt",
                    "acl": "private",
                    "dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_object_storage_object_acl_update"
        assert body["would_execute"]["method"] == "PUT"
        mock_client.get_object_acl.assert_awaited_once_with(
            "us-east-1", "my-bucket", "object.txt"
        )
        assert any("private" in s for s in body["side_effects"])
        mock_client.update_object_acl.assert_not_called()


async def test_obj_ssl_upload_dry_run_returns_preview_no_key_echoed(
    sample_config: Config,
) -> None:
    """dry_run=true previews the upload with no call and no private key echoed."""
    result = list(
        await handle_linode_object_storage_ssl_upload(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "certificate": "cert-pem",
                "private_key": "key-pem",
                "dry_run": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    text = result[0].text
    body = json.loads(text)
    assert body["tool"] == "linode_object_storage_ssl_upload"
    assert body["would_execute"]["method"] == "POST"
    assert (
        body["would_execute"]["path"]
        == "/object-storage/buckets/us-east-1/my-bucket/ssl"
    )
    assert body["current_state"] is None
    assert "key-pem" not in text


async def test_obj_ssl_upload_dry_run_still_validates_private_key(
    sample_config: Config,
) -> None:
    """Missing private_key must error out regardless of dry_run."""
    result = list(
        await handle_linode_object_storage_ssl_upload(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "certificate": "cert-pem",
                "dry_run": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "private_key is required" in result[0].text


async def test_ssl_delete_missing_env() -> None:
    """SSL delete should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": True,
            },
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


# ── LKE Tool Tests ──────────────────────────────────────────────


async def test_lke_clusters_list_tool_definition() -> None:
    """LKE clusters list tool should have correct name."""
    tool, _ = create_linode_lke_clusters_list_tool()
    assert tool.name == "linode_lke_clusters_list"


async def test_lke_cluster_get_tool_definition() -> None:
    """LKE cluster get tool should require cluster_id."""
    tool, _ = create_linode_lke_cluster_get_tool()
    assert tool.name == "linode_lke_cluster_get"
    assert "cluster_id" in (tool.inputSchema.get("required") or [])


async def test_lke_cluster_create_tool_definition() -> None:
    """LKE cluster create tool should require label, region, k8s_version."""
    tool, _ = create_linode_lke_cluster_create_tool()
    assert tool.name == "linode_lke_cluster_create"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "label" in required
    assert "region" in required
    assert "k8s_version" in required


async def test_lke_cluster_delete_tool_definition() -> None:
    """LKE cluster delete tool should require cluster_id and confirm."""
    tool, _ = create_linode_lke_cluster_delete_tool()
    assert tool.name == "linode_lke_cluster_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "cluster_id" in required
    assert "confirm" in required


async def test_lke_clusters_list(sample_config: Config) -> None:
    """LKE clusters list should return cluster data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_clusters.return_value = [
            {"id": 1, "label": "my-cluster", "region": "us-east", "status": "ready"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_clusters_list({}, sample_config))

        assert len(result) == 1
        assert "my-cluster" in result[0].text


async def test_lke_cluster_get(sample_config: Config) -> None:
    """LKE cluster get should return cluster details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {
            "id": 1,
            "label": "my-cluster",
            "region": "us-east",
            "k8s_version": "1.29",
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "my-cluster" in result[0].text


async def test_lke_cluster_get_missing_id(sample_config: Config) -> None:
    """LKE cluster get should fail without cluster_id."""
    result = list(await handle_linode_lke_cluster_get({}, sample_config))

    assert len(result) == 1
    assert "cluster_id" in result[0].text.lower()


async def test_lke_cluster_create_confirm_required(sample_config: Config) -> None:
    """LKE cluster create should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_create(
            {
                "label": "new-cluster",
                "region": "us-east",
                "k8s_version": "1.29",
                "node_pools": [{"type": "g6-standard-1", "count": 3}],
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_create_missing_label(sample_config: Config) -> None:
    """LKE cluster create should fail without label."""
    result = list(
        await handle_linode_lke_cluster_create(
            {
                "region": "us-east",
                "k8s_version": "1.29",
                "node_pools": [{"type": "g6-standard-1", "count": 3}],
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_lke_cluster_create_success(sample_config: Config) -> None:
    """LKE cluster create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_lke_cluster.return_value = {
            "id": 10,
            "label": "new-cluster",
            "region": "us-east",
            "k8s_version": "1.29",
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_create(
                {
                    "label": "new-cluster",
                    "region": "us-east",
                    "k8s_version": "1.29",
                    "node_pools": [{"type": "g6-standard-1", "count": 3}],
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-cluster" in result[0].text


async def test_lke_cluster_update_confirm_required(sample_config: Config) -> None:
    """LKE cluster update should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_update(
            {"cluster_id": 1, "label": "updated", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_update_success(sample_config: Config) -> None:
    """LKE cluster update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_cluster.return_value = {
            "id": 1,
            "label": "updated",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_update(
                {"cluster_id": 1, "label": "updated", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated" in result[0].text


async def test_lke_cluster_delete_confirm_required(sample_config: Config) -> None:
    """LKE cluster delete should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_delete_success(sample_config: Config) -> None:
    """LKE cluster delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_cluster_recycle_confirm_required(sample_config: Config) -> None:
    """LKE cluster recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_recycle(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_recycle_success(sample_config: Config) -> None:
    """LKE cluster recycle should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_recycle(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_cluster_regenerate_confirm_required(
    sample_config: Config,
) -> None:
    """LKE cluster regenerate should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_regenerate(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_regenerate_success(sample_config: Config) -> None:
    """LKE cluster regenerate should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.regenerate_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_regenerate(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "regenerat" in result[0].text.lower()


async def test_lke_pools_list(sample_config: Config) -> None:
    """LKE pools list should return pool data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_node_pools.return_value = [
            {"id": 100, "type": "g6-standard-1", "count": 3},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pools_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_get(sample_config: Config) -> None:
    """LKE pool get should return pool details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {
            "id": 100,
            "type": "g6-standard-1",
            "count": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_get(
                {"cluster_id": 1, "pool_id": 100}, sample_config
            )
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_create_confirm_required(sample_config: Config) -> None:
    """LKE pool create should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_create(
            {
                "cluster_id": 1,
                "type": "g6-standard-1",
                "count": 3,
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_create_success(sample_config: Config) -> None:
    """LKE pool create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_lke_node_pool.return_value = {
            "id": 200,
            "type": "g6-standard-1",
            "count": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_create(
                {
                    "cluster_id": 1,
                    "type": "g6-standard-1",
                    "count": 3,
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_update_confirm_required(sample_config: Config) -> None:
    """LKE pool update should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_update(
            {"cluster_id": 1, "pool_id": 100, "count": 5, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_update_success(sample_config: Config) -> None:
    """LKE pool update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_node_pool.return_value = {
            "id": 100,
            "type": "g6-standard-1",
            "count": 5,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_update(
                {"cluster_id": 1, "pool_id": 100, "count": 5, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "100" in result[0].text


async def test_lke_pool_delete_confirm_required(sample_config: Config) -> None:
    """LKE pool delete should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_delete(
            {"cluster_id": 1, "pool_id": 100, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_delete_success(sample_config: Config) -> None:
    """LKE pool delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_node_pool.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_delete(
                {"cluster_id": 1, "pool_id": 100, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_pool_recycle_confirm_required(sample_config: Config) -> None:
    """LKE pool recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_recycle(
            {"cluster_id": 1, "pool_id": 100, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_recycle_success(sample_config: Config) -> None:
    """LKE pool recycle should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_node_pool.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_recycle(
                {"cluster_id": 1, "pool_id": 100, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_node_get(sample_config: Config) -> None:
    """LKE node get should return node details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node.return_value = {
            "id": "lke-node-abc",
            "instance_id": 555,
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_get(
                {"cluster_id": 1, "node_id": "lke-node-abc"}, sample_config
            )
        )

        assert len(result) == 1
        assert "lke-node-abc" in result[0].text


async def test_lke_node_get_missing_node_id(sample_config: Config) -> None:
    """LKE node get should fail without node_id."""
    result = list(await handle_linode_lke_node_get({"cluster_id": 1}, sample_config))

    assert len(result) == 1
    assert "node_id" in result[0].text.lower()


async def test_lke_node_delete_confirm_required(sample_config: Config) -> None:
    """LKE node delete should require confirm=true."""
    result = list(
        await handle_linode_lke_node_delete(
            {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_node_delete_success(sample_config: Config) -> None:
    """LKE node delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_node.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_delete(
                {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_node_recycle_confirm_required(sample_config: Config) -> None:
    """LKE node recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_node_recycle(
            {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_node_recycle_success(sample_config: Config) -> None:
    """LKE node recycle should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_node.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_recycle(
                {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_kubeconfig_get(sample_config: Config) -> None:
    """LKE kubeconfig get should return kubeconfig data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_kubeconfig.return_value = {
            "kubeconfig": "YXBpVmVyc2lvbjogdjEK",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_kubeconfig_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "kubeconfig" in result[0].text.lower()


async def test_lke_kubeconfig_delete_confirm_required(
    sample_config: Config,
) -> None:
    """LKE kubeconfig delete should require confirm=true."""
    result = list(
        await handle_linode_lke_kubeconfig_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_kubeconfig_delete_success(sample_config: Config) -> None:
    """LKE kubeconfig delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_kubeconfig.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_kubeconfig_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "regenerated" in result[0].text.lower()


async def test_lke_dashboard_get(sample_config: Config) -> None:
    """LKE dashboard get should return dashboard URL."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_dashboard.return_value = {
            "url": "https://dashboard.example.com",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_dashboard_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "dashboard" in result[0].text.lower()


async def test_lke_api_endpoints_list(sample_config: Config) -> None:
    """LKE API endpoints list should return endpoint data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_api_endpoints.return_value = [
            {"endpoint": "https://api.lke.example.com"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_api_endpoints_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "endpoint" in result[0].text.lower()


async def test_lke_service_token_delete_confirm_required(
    sample_config: Config,
) -> None:
    """LKE service token delete should require confirm=true."""
    result = list(
        await handle_linode_lke_service_token_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_service_token_delete_success(sample_config: Config) -> None:
    """LKE service token delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_service_token.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_service_token_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_cluster_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {
            "id": 123,
            "label": "prod",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_cluster_delete(
            {"cluster_id": 123, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_lke_cluster_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/lke/clusters/123"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.delete_lke_cluster.assert_not_called()


async def test_lke_cluster_delete_dry_run_still_validates_cluster_id(
    sample_config: Config,
) -> None:
    """Missing cluster_id must error regardless of dry_run."""
    result = await handle_linode_lke_cluster_delete({"dry_run": True}, sample_config)
    assert "cluster_id is required" in result[0].text


async def test_lke_pool_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {"id": 10, "count": 3}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_delete(
                {"cluster_id": 123, "pool_id": 10, "dry_run": True},
                sample_config,
            )
        )

        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_lke_pool_delete"
        assert body["would_execute"]["path"] == "/lke/clusters/123/pools/10"
        mock_client.get_lke_node_pool.assert_awaited_once_with(123, 10)
        mock_client.delete_lke_node_pool.assert_not_called()


async def test_lke_pool_delete_dry_run_surfaces_node_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: pool nodes' backing Linodes cascade-delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {
            "id": 10,
            "count": 2,
            "nodes": [
                {"id": "node-a", "instance_id": 9001},
                {"id": "node-b", "instance_id": 9002},
            ],
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_delete(
                {"cluster_id": 123, "pool_id": 10, "dry_run": True},
                sample_config,
            )
        )

        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 2
        assert all(d["kind"] == "instance" for d in deps)
        assert all(d["action"] == "cascade_deleted" for d in deps)
        assert body["warnings"]
        mock_client.delete_lke_node_pool.assert_not_called()


async def test_lke_pool_delete_dry_run_still_validates_ids(
    sample_config: Config,
) -> None:
    """Missing cluster_id must error regardless of dry_run."""
    result = list(
        await handle_linode_lke_pool_delete(
            {"pool_id": 10, "dry_run": True}, sample_config
        )
    )
    assert "cluster_id is required" in result[0].text


async def test_lke_node_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch node state (mixed int+string IDs)."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node.return_value = {"id": "123-abc", "status": "ready"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_node_delete(
            {"cluster_id": 123, "node_id": "123-abc", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_node_delete"
        assert body["would_execute"]["path"] == "/lke/clusters/123/nodes/123-abc"
        mock_client.get_lke_node.assert_awaited_once_with(123, "123-abc")
        mock_client.delete_lke_node.assert_not_called()


async def test_lke_node_delete_dry_run_surfaces_backing_linode(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: the node's backing Linode cascade-deletes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node.return_value = {
            "id": "123-abc",
            "instance_id": 9100,
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_node_delete(
            {"cluster_id": 123, "node_id": "123-abc", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 1
        assert deps[0]["kind"] == "instance"
        assert deps[0]["action"] == "cascade_deleted"
        assert deps[0]["id"] == 9100
        assert body["warnings"]
        mock_client.delete_lke_node.assert_not_called()


async def test_lke_node_delete_dry_run_still_validates_node_id(
    sample_config: Config,
) -> None:
    """Missing node_id must error regardless of dry_run."""
    result = await handle_linode_lke_node_delete(
        {"cluster_id": 123, "dry_run": True}, sample_config
    )
    assert "node_id is required" in result[0].text


async def test_lke_kubeconfig_delete_dry_run_fetches_cluster_not_kubeconfig(
    sample_config: Config,
) -> None:
    """Dry-run must fetch cluster metadata, NOT kubeconfig content.

    Locks the credential-safety design choice: dry-run never surfaces
    the kubeconfig itself to the model. A regression that swaps the
    fetch to ``get_lke_kubeconfig`` would surface a credential.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123, "label": "prod"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_kubeconfig_delete(
                {"cluster_id": 123, "dry_run": True}, sample_config
            )
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_kubeconfig_delete"
        assert body["would_execute"]["path"] == "/lke/clusters/123/kubeconfig"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.get_lke_kubeconfig.assert_not_called()
        mock_client.delete_lke_kubeconfig.assert_not_called()


async def test_lke_service_token_delete_dry_run_fetches_cluster_not_token(
    sample_config: Config,
) -> None:
    """Dry-run must fetch cluster metadata, NOT the service token.

    Same credential-safety design as kubeconfig_delete.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123, "label": "prod"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_service_token_delete(
                {"cluster_id": 123, "dry_run": True}, sample_config
            )
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_service_token_delete"
        assert body["would_execute"]["path"] == "/lke/clusters/123/servicetoken"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.delete_lke_service_token.assert_not_called()


async def test_lke_acl_get(sample_config: Config) -> None:
    """LKE ACL get should return ACL data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_control_plane_acl.return_value = {
            "acl": {
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": []},
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_acl_get({"cluster_id": 1}, sample_config))

        assert len(result) == 1
        assert "acl" in result[0].text.lower()


async def test_lke_acl_update_confirm_required(sample_config: Config) -> None:
    """LKE ACL update should require confirm=true."""
    result = list(
        await handle_linode_lke_acl_update(
            {
                "cluster_id": 1,
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"]},
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_acl_update_success(sample_config: Config) -> None:
    """LKE ACL update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_control_plane_acl.return_value = {
            "acl": {
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": []},
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_acl_update(
                {
                    "cluster_id": 1,
                    "enabled": True,
                    "addresses": {"ipv4": ["10.0.0.0/8"]},
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "acl" in result[0].text.lower()


async def test_lke_acl_delete_confirm_required(sample_config: Config) -> None:
    """LKE ACL delete should require confirm=true."""
    result = list(
        await handle_linode_lke_acl_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_acl_delete_success(sample_config: Config) -> None:
    """LKE ACL delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_control_plane_acl.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_acl_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_versions_list(sample_config: Config) -> None:
    """LKE versions list should return version data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_versions.return_value = [
            {"id": "1.29"},
            {"id": "1.28"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_versions_list({}, sample_config))

        assert len(result) == 1
        assert "1.29" in result[0].text


async def test_lke_version_get(sample_config: Config) -> None:
    """LKE version get should return version details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_version.return_value = {"id": "1.29"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_version_get({"version_id": "1.29"}, sample_config)
        )

        assert len(result) == 1
        assert "1.29" in result[0].text


async def test_lke_version_get_missing_id(sample_config: Config) -> None:
    """LKE version get should fail without version_id."""
    result = list(await handle_linode_lke_version_get({}, sample_config))

    assert len(result) == 1
    assert "version_id" in result[0].text.lower()


async def test_lke_types_list(sample_config: Config) -> None:
    """LKE types list should return type data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_types.return_value = [
            {"id": "g6-standard-1", "label": "Linode 2GB"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_types_list({}, sample_config))

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_tier_versions_list(sample_config: Config) -> None:
    """LKE tier versions list should return tier version data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_tier_versions.return_value = [
            {"id": "1.29", "tier": "standard"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_tier_versions_list(
                {"tier": "standard"}, sample_config
            )
        )

        assert len(result) == 1
        assert "1.29" in result[0].text
        mock_client.list_lke_tier_versions.assert_awaited_once_with("standard")


async def test_lke_tier_versions_list_requires_tier(sample_config: Config) -> None:
    """LKE tier versions list requires tier before client dispatch."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = list(await handle_linode_lke_tier_versions_list({}, sample_config))

    assert "tier must be a non-empty path segment" in result[0].text
    mock_cls.assert_not_called()


@pytest.mark.parametrize(
    "tier",
    [
        "",
        "standard/enterprise",
        "standard?tier",
        "..",
        "#bad",
        "bad&",
        "bad tier",
        "standard%2F..",
    ],
)
async def test_lke_tier_versions_list_rejects_malformed_tier(
    sample_config: Config, tier: object
) -> None:
    """LKE tier versions list rejects malformed tier path segments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = list(
            await handle_linode_lke_tier_versions_list({"tier": tier}, sample_config)
        )

    assert "tier must be a non-empty path segment" in result[0].text
    mock_cls.assert_not_called()


# VPC tool definition tests


async def test_vpcs_list_tool_definition() -> None:
    """VPCs list tool should have correct name."""
    tool, _ = create_linode_vpcs_list_tool()
    assert tool.name == "linode_vpcs_list"


async def test_vlans_list_tool_definition() -> None:
    """VLANs list tool should have correct name."""
    tool, _ = create_linode_vlans_list_tool()
    assert tool.name == "linode_vlans_list"


async def test_vlan_delete_tool_definition() -> None:
    """VLAN delete tool should have correct name and required params."""
    tool, _ = create_linode_vlan_delete_tool()
    assert tool.name == "linode_vlan_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "region_id" in required
    assert "label" in required
    assert "confirm" in required


async def test_vpc_get_tool_definition() -> None:
    """VPC get tool should require vpc_id."""
    tool, _ = create_linode_vpc_get_tool()
    assert tool.name == "linode_vpc_get"
    assert "vpc_id" in (tool.inputSchema.get("required") or [])


async def test_vpc_create_tool_definition() -> None:
    """VPC create tool should require label, region, confirm."""
    tool, _ = create_linode_vpc_create_tool()
    assert tool.name == "linode_vpc_create"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "label" in required
    assert "region" in required
    assert "confirm" in required


async def test_vpc_delete_tool_definition() -> None:
    """VPC delete tool should require vpc_id and confirm."""
    tool, _ = create_linode_vpc_delete_tool()
    assert tool.name == "linode_vpc_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "confirm" in required


async def test_ipv6_range_create_tool_definition() -> None:
    """IPv6 range create tool should require prefix_length and confirm."""
    tool, _ = create_linode_ipv6_range_create_tool()
    assert tool.name == "linode_ipv6_range_create"
    required: list[str] = tool.inputSchema.get("required") or []
    properties: dict[str, Any] = tool.inputSchema.get("properties") or {}
    assert "prefix_length" in required
    assert "confirm" in required
    assert "linode_id" in properties
    assert "route_target" in properties
    assert "linode_id" not in required
    assert "route_target" not in required


async def test_ipv6_range_get_tool_definition() -> None:
    """IPv6 range get tool should require range without confirm."""
    tool, _ = create_linode_ipv6_range_get_tool()
    assert tool.name == "linode_ipv6_range_get"
    required: list[str] = tool.inputSchema.get("required") or []
    properties: dict[str, Any] = tool.inputSchema.get("properties") or {}
    assert "range" in required
    assert "confirm" not in required
    assert "confirm" not in properties


async def test_ipv6_range_delete_tool_definition() -> None:
    """IPv6 range delete tool should require range and confirm."""
    tool, _ = create_linode_ipv6_range_delete_tool()
    assert tool.name == "linode_ipv6_range_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "range" in required
    assert "confirm" in required


async def test_vpc_subnet_create_tool_definition() -> None:
    """VPC subnet create tool should require vpc_id, label, ipv4, confirm."""
    tool, _ = create_linode_vpc_subnet_create_tool()
    assert tool.name == "linode_vpc_subnet_create"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "label" in required
    assert "ipv4" in required
    assert "confirm" in required


async def test_vpc_subnet_delete_tool_definition() -> None:
    """VPC subnet delete tool should require vpc_id, subnet_id, confirm."""
    tool, _ = create_linode_vpc_subnet_delete_tool()
    assert tool.name == "linode_vpc_subnet_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "subnet_id" in required
    assert "confirm" in required


# VPC handler tests


async def test_vpcs_list(sample_config: Config) -> None:
    """VPCs list should return VPC data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpcs.return_value = [
            {"id": 1, "label": "my-vpc", "region": "us-east"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpcs_list({}, sample_config))

        assert len(result) == 1
        assert "my-vpc" in result[0].text


async def test_vlans_list(sample_config: Config) -> None:
    """VLANs list should return VLAN data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vlans.return_value = [
            {"label": "app-vlan", "region": "us-east", "linodes": [123]},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vlans_list({}, sample_config))

        assert len(result) == 1
        assert "app-vlan" in result[0].text
        mock_client.list_vlans.assert_called_once()


async def test_vlan_delete_confirm_required(sample_config: Config) -> None:
    """VLAN delete should require confirm=true."""
    result = list(
        await handle_linode_vlan_delete(
            {"region_id": "us-east", "label": "app-vlan", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vlan_delete_success(sample_config: Config) -> None:
    """VLAN delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_vlan.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vlan_delete(
                {"region_id": "us-east", "label": "app-vlan", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()
        mock_client.delete_vlan.assert_called_once_with("us-east", "app-vlan")


async def test_vlan_delete_dry_run_lists_and_filters(
    sample_config: Config,
) -> None:
    """dry_run lists VLANs and filters to the match, never deleting.

    VLANs have no single-GET endpoint, so the dry-run fetch lists and
    filters. Catches a regression where delete fires on the dry-run path.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vlans.return_value = [
            {"label": "other-vlan", "region": "us-east"},
            {"label": "app-vlan", "region": "us-east", "linodes": [123]},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vlan_delete(
                {"region_id": "us-east", "label": "app-vlan", "dry_run": True},
                sample_config,
            )
        )

        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_vlan_delete"
        assert body["would_execute"]["path"] == "/networking/vlans/us-east/app-vlan"
        assert body["current_state"]["label"] == "app-vlan"
        mock_client.list_vlans.assert_awaited_once()
        mock_client.delete_vlan.assert_not_called()


async def test_vlan_delete_dry_run_not_found_errors(
    sample_config: Config,
) -> None:
    """dry_run on a non-existent VLAN surfaces a not-found error."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vlans.return_value = []
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vlan_delete(
                {"region_id": "us-east", "label": "ghost-vlan", "dry_run": True},
                sample_config,
            )
        )

        assert "VLAN not found" in result[0].text
        mock_client.delete_vlan.assert_not_called()


async def test_vlan_delete_dry_run_still_validates_region(
    sample_config: Config,
) -> None:
    """Missing region_id must error regardless of dry_run."""
    result = list(
        await handle_linode_vlan_delete(
            {"label": "app-vlan", "dry_run": True}, sample_config
        )
    )
    assert "region_id is required" in result[0].text


async def test_vpc_get(sample_config: Config) -> None:
    """VPC get should return VPC details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {
            "id": 1,
            "label": "my-vpc",
            "region": "us-east",
            "description": "test vpc",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_get({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        assert "my-vpc" in result[0].text


async def test_vpc_get_missing_id(sample_config: Config) -> None:
    """VPC get should fail without vpc_id."""
    result = list(await handle_linode_vpc_get({}, sample_config))

    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()


async def test_ipv6_range_get_missing_range(sample_config: Config) -> None:
    """IPv6 range get should fail without range."""
    result = list(await handle_linode_ipv6_range_get({}, sample_config))

    assert len(result) == 1
    assert "range" in result[0].text.lower()


async def test_ipv6_range_get_success(sample_config: Config) -> None:
    """IPv6 range get should return IPv6 range data."""
    ipv6_range = "2001:0db8::"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_ipv6_range.return_value = {
            "range": ipv6_range,
            "region": "us-east",
            "prefix": 64,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_get(
                {"range": ipv6_range},
                sample_config,
            )
        )

        assert len(result) == 1
        assert ipv6_range in result[0].text
        mock_client.get_ipv6_range.assert_called_once_with(ipv6_range)


async def test_vpc_create_confirm_required(sample_config: Config) -> None:
    """VPC create should require confirm=true."""
    result = list(
        await handle_linode_vpc_create(
            {
                "label": "new-vpc",
                "region": "us-east",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_create_missing_label(sample_config: Config) -> None:
    """VPC create should fail without label."""
    result = list(
        await handle_linode_vpc_create(
            {"region": "us-east", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_vpc_create_success(sample_config: Config) -> None:
    """VPC create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_vpc.return_value = {
            "id": 10,
            "label": "new-vpc",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_create(
                {
                    "label": "new-vpc",
                    "region": "us-east",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-vpc" in result[0].text


async def test_vpc_update_confirm_required(sample_config: Config) -> None:
    """VPC update should require confirm=true."""
    result = list(
        await handle_linode_vpc_update(
            {"vpc_id": 1, "label": "updated", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_update_success(sample_config: Config) -> None:
    """VPC update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_vpc.return_value = {
            "id": 1,
            "label": "updated-vpc",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_update(
                {"vpc_id": 1, "label": "updated-vpc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated-vpc" in result[0].text


async def test_vpc_delete_confirm_required(sample_config: Config) -> None:
    """VPC delete should require confirm=true."""
    result = list(
        await handle_linode_vpc_delete(
            {"vpc_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_delete_success(sample_config: Config) -> None:
    """VPC delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_vpc.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_delete(
                {"vpc_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_vpc_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {
            "id": 123,
            "label": "prod-vpc",
            "region": "us-east",
        }
        mock_client.list_vpc_subnets.return_value = []
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_delete(
                {"vpc_id": 123, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_vpc_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/vpcs/123"
        mock_client.get_vpc.assert_awaited_once_with(123)
        mock_client.delete_vpc.assert_not_called()


async def test_vpc_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {"id": 123, "label": "prod-vpc"}
        mock_client.list_vpc_subnets.return_value = []
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_delete(
                {"vpc_id": 123, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_vpc_delete_dry_run_surfaces_subnet_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: subnets appear as cascade_deleted dependencies."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {"id": 123, "label": "prod-vpc"}
        mock_client.list_vpc_subnets.return_value = [
            {"id": 1, "label": "subnet-a", "linodes": [{"id": 456}]},
            {"id": 2, "label": "subnet-b", "linodes": []},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_delete(
                {"vpc_id": 123, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 2
        assert all(d["kind"] == "vpc_subnet" for d in deps)
        assert all(d["action"] == "cascade_deleted" for d in deps)
        assert any("interface" in w for w in body["warnings"])
        mock_client.delete_vpc.assert_not_called()


async def test_vpc_delete_dry_run_still_validates_vpc_id(
    sample_config: Config,
) -> None:
    """Missing vpc_id must error out regardless of dry_run."""
    result = list(
        await handle_linode_vpc_delete(
            {"dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "vpc_id is required" in result[0].text


async def test_ipv6_range_create_validation_errors(sample_config: Config) -> None:
    """IPv6 range create should validate confirmation and documented fields."""
    cases: list[tuple[dict[str, Any], str]] = [
        (
            {"prefix_length": 64, "linode_id": 123, "confirm": False},
            "confirm=true",
        ),
        ({"linode_id": 123, "confirm": True}, "prefix_length"),
        (
            {"prefix_length": 48, "linode_id": 123, "confirm": True},
            "56 or 64",
        ),
        ({"prefix_length": 64, "confirm": True}, "linode_id or route_target"),
        (
            {
                "prefix_length": 64,
                "linode_id": 123,
                "route_target": "2001:0db8::1",
                "confirm": True,
            },
            "mutually exclusive",
        ),
        (
            {"prefix_length": 64, "linode_id": "bad-id", "confirm": True},
            "valid integer",
        ),
        (
            {"prefix_length": 64, "route_target": "   ", "confirm": True},
            "non-empty",
        ),
    ]

    for arguments, expected_message in cases:
        result = list(await handle_linode_ipv6_range_create(arguments, sample_config))

        assert len(result) == 1
        assert expected_message in result[0].text


async def test_ipv6_range_create_success_with_linode_id(
    sample_config: Config,
) -> None:
    """IPv6 range create should call the retryable client with linode_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_ipv6_range.return_value = {
            "range": "2001:0db8::/64",
            "route_target": "2001:0db8::1",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_create(
                {"prefix_length": "64", "linode_id": "123", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "2001:0db8::/64" in result[0].text
        mock_client.create_ipv6_range.assert_called_once_with(
            prefix_length=64,
            linode_id=123,
            route_target=None,
        )


async def test_ipv6_range_create_success_with_route_target(
    sample_config: Config,
) -> None:
    """IPv6 range create should call the retryable client with route_target."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_ipv6_range.return_value = {
            "range": "2001:0db8::/56",
            "route_target": "2001:0db8::1",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_create(
                {
                    "prefix_length": 56,
                    "route_target": " 2001:0db8::1 ",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "2001:0db8::/56" in result[0].text
        mock_client.create_ipv6_range.assert_called_once_with(
            prefix_length=56,
            linode_id=None,
            route_target="2001:0db8::1",
        )


async def test_ipv6_range_delete_confirm_required(sample_config: Config) -> None:
    """IPv6 range delete should require confirm=true."""
    result = list(
        await handle_linode_ipv6_range_delete(
            {"range": "2001:0db8::", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_ipv6_range_delete_missing_range(sample_config: Config) -> None:
    """IPv6 range delete should fail without range."""
    result = list(
        await handle_linode_ipv6_range_delete(
            {"confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "range" in result[0].text.lower()


async def test_ipv6_range_delete_success(sample_config: Config) -> None:
    """IPv6 range delete should succeed with valid input."""
    ipv6_range = "2001:0db8::"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_ipv6_range.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_delete(
                {"range": ipv6_range, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()
        mock_client.delete_ipv6_range.assert_called_once_with(ipv6_range)


async def test_ipv6_range_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch the range via GET and never call delete."""
    ipv6_range = "2001:0db8::/64"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_ipv6_range.return_value = {
            "range": ipv6_range,
            "region": "us-east",
            "prefix": 64,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_delete(
                {"range": ipv6_range, "dry_run": True},
                sample_config,
            )
        )

        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_ipv6_range_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == f"/networking/ipv6/ranges/{ipv6_range}"
        mock_client.get_ipv6_range.assert_awaited_once_with(ipv6_range)
        mock_client.delete_ipv6_range.assert_not_called()


async def test_ipv6_range_delete_dry_run_still_validates_range(
    sample_config: Config,
) -> None:
    """Missing range must error regardless of dry_run."""
    result = list(
        await handle_linode_ipv6_range_delete({"dry_run": True}, sample_config)
    )
    assert "range is required" in result[0].text


async def test_vpc_ips_list(sample_config: Config) -> None:
    """VPC IPs list should return IP data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_ips.return_value = [
            {"address": "10.0.0.1", "vpc_id": 1, "subnet_id": 1},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ips_list({}, sample_config))

        assert len(result) == 1
        assert "10.0.0.1" in result[0].text


async def test_vpc_ip_list(sample_config: Config) -> None:
    """VPC IP list should return IPs for a specific VPC."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_ip.return_value = [
            {"address": "10.0.0.2", "vpc_id": 1, "subnet_id": 1},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ip_list({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        assert "10.0.0.2" in result[0].text


async def test_vpc_ip_list_missing_id(sample_config: Config) -> None:
    """VPC IP list should fail without vpc_id."""
    result = list(await handle_linode_vpc_ip_list({}, sample_config))

    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()


async def test_vpc_subnets_list(sample_config: Config) -> None:
    """VPC subnets list should return subnet data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_subnets.return_value = [
            {"id": 1, "label": "my-subnet", "ipv4": "10.0.0.0/24"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnets_list({"vpc_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "my-subnet" in result[0].text


async def test_vpc_subnet_get(sample_config: Config) -> None:
    """VPC subnet get should return subnet details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {
            "id": 1,
            "label": "my-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_get(
                {"vpc_id": 1, "subnet_id": 1}, sample_config
            )
        )

        assert len(result) == 1
        assert "my-subnet" in result[0].text


async def test_vpc_subnet_get_missing_ids(sample_config: Config) -> None:
    """VPC subnet get should fail without required IDs."""
    result = list(await handle_linode_vpc_subnet_get({}, sample_config))
    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()

    result = list(await handle_linode_vpc_subnet_get({"vpc_id": 1}, sample_config))
    assert len(result) == 1
    assert "subnet_id" in result[0].text.lower()


async def test_vpc_subnet_create_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet create should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {
                "vpc_id": 1,
                "label": "new-subnet",
                "ipv4": "10.0.0.0/24",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_create_missing_label(
    sample_config: Config,
) -> None:
    """VPC subnet create should fail without label."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {
                "vpc_id": 1,
                "ipv4": "10.0.0.0/24",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_vpc_subnet_create_success(sample_config: Config) -> None:
    """VPC subnet create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_vpc_subnet.return_value = {
            "id": 5,
            "label": "new-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_create(
                {
                    "vpc_id": 1,
                    "label": "new-subnet",
                    "ipv4": "10.0.0.0/24",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-subnet" in result[0].text


async def test_vpc_subnet_update_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet update should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_update(
            {
                "vpc_id": 1,
                "subnet_id": 1,
                "label": "updated",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_update_success(sample_config: Config) -> None:
    """VPC subnet update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_vpc_subnet.return_value = {
            "id": 1,
            "label": "updated-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_update(
                {
                    "vpc_id": 1,
                    "subnet_id": 1,
                    "label": "updated-subnet",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated-subnet" in result[0].text


async def test_vpc_subnet_delete_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet delete should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_delete(
            {"vpc_id": 1, "subnet_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_delete_success(sample_config: Config) -> None:
    """VPC subnet delete should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_vpc_subnet.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_delete(
                {"vpc_id": 1, "subnet_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_vpc_subnet_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {
            "id": 10,
            "label": "web-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_delete(
                {"vpc_id": 123, "subnet_id": 10, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_vpc_subnet_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/vpcs/123/subnets/10"
        mock_client.get_vpc_subnet.assert_awaited_once_with(123, 10)
        mock_client.delete_vpc_subnet.assert_not_called()


async def test_vpc_subnet_delete_dry_run_surfaces_linode_dependencies(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: Linodes with interfaces in the subnet detach.

    The walk reads the already-fetched subnet state, so no extra GET fires.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {
            "id": 10,
            "label": "web-subnet",
            "linodes": [{"id": 456}, {"id": 789}],
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_delete(
                {"vpc_id": 123, "subnet_id": 10, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        deps = body["dependencies"]
        assert len(deps) == 2
        assert all(d["kind"] == "instance" for d in deps)
        assert all(d["action"] == "detached" for d in deps)
        assert body["warnings"]
        mock_client.delete_vpc_subnet.assert_not_called()


async def test_vpc_subnet_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {"id": 10, "label": "web-subnet"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_delete(
                {"vpc_id": 123, "subnet_id": 10, "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_vpc_subnet_delete_dry_run_still_validates_ids(
    sample_config: Config,
) -> None:
    """Missing IDs must error out regardless of dry_run."""
    result = list(
        await handle_linode_vpc_subnet_delete(
            {"subnet_id": 10, "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "vpc_id is required" in result[0].text


async def test_vpc_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = list(
        await handle_linode_vpc_create(
            {"label": "vpc-01", "region": "us-east", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_vpc_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/vpcs"
    assert body["current_state"] is None
    assert any("vpc-01" in s for s in body["side_effects"])
    assert "confirm=true" not in result[0].text


async def test_vpc_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = list(
        await handle_linode_vpc_create(
            {"region": "us-east", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_vpc_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {"id": 55, "label": "prod-vpc"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_update(
                {"vpc_id": 55, "label": "renamed", "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_vpc_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/vpcs/55"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_vpc.assert_awaited_once_with(55)
        mock_client.update_vpc.assert_not_called()


async def test_vpc_update_dry_run_still_validates_vpc_id(
    sample_config: Config,
) -> None:
    """Missing vpc_id must error out regardless of dry_run."""
    result = list(await handle_linode_vpc_update({"dry_run": True}, sample_config))

    assert len(result) == 1
    assert "vpc_id is required" in result[0].text


async def test_vpc_subnet_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the subnet create with no call."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {
                "vpc_id": 55,
                "label": "subnet-01",
                "ipv4": "10.0.0.0/24",
                "dry_run": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_vpc_subnet_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/vpcs/55/subnets"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text
    assert len(body["side_effects"]) == 1
    assert "subnet-01" in body["side_effects"][0]
    assert "10.0.0.0/24" in body["side_effects"][0]


async def test_vpc_subnet_create_dry_run_still_validates_vpc_id(
    sample_config: Config,
) -> None:
    """Missing vpc_id must error out regardless of dry_run."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {"label": "subnet-01", "ipv4": "10.0.0.0/24", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "vpc_id is required" in result[0].text


async def test_vpc_subnet_update_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {"id": 10, "label": "sub"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_update(
                {"vpc_id": 55, "subnet_id": 10, "label": "renamed", "dry_run": True},
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_vpc_subnet_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/vpcs/55/subnets/10"
        mock_client.get_vpc_subnet.assert_awaited_once_with(55, 10)
        mock_client.update_vpc_subnet.assert_not_called()


async def test_vpc_subnet_update_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = list(
        await handle_linode_vpc_subnet_update(
            {"vpc_id": 55, "subnet_id": 10, "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


# ── Instance Backups tool definition tests ──


async def test_instance_backups_list_tool_definition() -> None:
    """Backups list tool should require instance_id."""
    tool, _ = create_linode_instance_backups_list_tool()
    assert tool.name == "linode_instance_backups_list"
    assert "instance_id" in (tool.inputSchema.get("required") or [])


async def test_instance_backup_get_tool_definition() -> None:
    """Backup get tool should require instance_id and backup_id."""
    tool, _ = create_linode_instance_backup_get_tool()
    assert tool.name == "linode_instance_backup_get"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "backup_id" in required


async def test_instance_backup_create_tool_def() -> None:
    """Backup create tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_backup_create_tool()
    assert tool.name == "linode_instance_backup_create"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


async def test_instance_backup_restore_tool_def() -> None:
    """Backup restore should require instance_id, backup_id, linode_id, confirm."""
    tool, _ = create_linode_instance_backup_restore_tool()
    assert tool.name == "linode_instance_backup_restore"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "backup_id" in required
    assert "linode_id" in required
    assert "confirm" in required


async def test_instance_backups_enable_tool_def() -> None:
    """Backups enable tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_backups_enable_tool()
    assert tool.name == "linode_instance_backups_enable"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


async def test_instance_backups_cancel_tool_def() -> None:
    """Backups cancel tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_backups_cancel_tool()
    assert tool.name == "linode_instance_backups_cancel"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


# ── Instance Backups handler tests ──


async def test_instance_backups_list_missing_id(
    sample_config: Config,
) -> None:
    """Backups list should fail without instance_id."""
    result = list(await handle_linode_instance_backups_list({}, sample_config))
    assert len(result) == 1
    assert "instance_id" in result[0].text.lower()


async def test_instance_backups_list_success(
    sample_config: Config,
) -> None:
    """Backups list should return backup data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_backups.return_value = {
            "automatic": [],
            "snapshot": {
                "current": None,
                "in_progress": None,
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_backups_list(
                {"instance_id": 123}, sample_config
            )
        )
        assert len(result) == 1
        assert "automatic" in result[0].text


async def test_instance_backup_create_no_confirm(
    sample_config: Config,
) -> None:
    """Backup create should require confirm=true."""
    result = list(
        await handle_linode_instance_backup_create({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_backup_create_success(
    sample_config: Config,
) -> None:
    """Backup create should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.create_instance_backup.return_value = {
            "id": 456,
            "label": "my-snap",
            "status": "pending",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_backup_create(
                {
                    "instance_id": 123,
                    "label": "my-snap",
                    "confirm": True,
                },
                sample_config,
            )
        )
        assert len(result) == 1
        assert "my-snap" in result[0].text


async def test_instance_backups_enable_no_confirm(
    sample_config: Config,
) -> None:
    """Backups enable should require confirm=true."""
    result = list(
        await handle_linode_instance_backups_enable({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_backups_cancel_no_confirm(
    sample_config: Config,
) -> None:
    """Backups cancel should require confirm=true."""
    result = list(
        await handle_linode_instance_backups_cancel({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_backup_restore_no_confirm(
    sample_config: Config,
) -> None:
    """Backup restore should require confirm=true."""
    result = list(
        await handle_linode_instance_backup_restore(
            {
                "instance_id": 123,
                "backup_id": 456,
                "linode_id": 789,
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_backup_get_missing_ids(
    sample_config: Config,
) -> None:
    """Backup get should fail without backup_id."""
    result = list(
        await handle_linode_instance_backup_get({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "backup_id" in result[0].text.lower()


# ── Instance Disks tool definition tests ──


async def test_instance_disks_list_tool_def() -> None:
    """Disks list tool should require instance_id."""
    tool, _ = create_linode_instance_disks_list_tool()
    assert tool.name == "linode_instance_disks_list"
    assert "instance_id" in (tool.inputSchema.get("required") or [])


async def test_instance_disk_get_tool_def() -> None:
    """Disk get tool should require instance_id and disk_id."""
    tool, _ = create_linode_instance_disk_get_tool()
    assert tool.name == "linode_instance_disk_get"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required


async def test_instance_disk_create_tool_def() -> None:
    """Disk create should require instance_id, label, size, confirm."""
    tool, _ = create_linode_instance_disk_create_tool()
    assert tool.name == "linode_instance_disk_create"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "label" in required
    assert "size" in required
    assert "confirm" in required


async def test_instance_disk_update_tool_def() -> None:
    """Disk update should require instance_id, disk_id, confirm."""
    tool, _ = create_linode_instance_disk_update_tool()
    assert tool.name == "linode_instance_disk_update"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required
    assert "confirm" in required


async def test_instance_disk_delete_tool_def() -> None:
    """Disk delete should require instance_id, disk_id, confirm."""
    tool, _ = create_linode_instance_disk_delete_tool()
    assert tool.name == "linode_instance_disk_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required
    assert "confirm" in required


async def test_instance_disk_clone_tool_def() -> None:
    """Disk clone should require instance_id, disk_id, confirm."""
    tool, _ = create_linode_instance_disk_clone_tool()
    assert tool.name == "linode_instance_disk_clone"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required
    assert "confirm" in required


async def test_instance_disk_resize_tool_def() -> None:
    """Disk resize should require instance_id, disk_id, size, confirm."""
    tool, _ = create_linode_instance_disk_resize_tool()
    assert tool.name == "linode_instance_disk_resize"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required
    assert "size" in required
    assert "confirm" in required


# ── Instance Disks handler tests ──


async def test_instance_disks_list_success(
    sample_config: Config,
) -> None:
    """Disks list should return disk data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_disks.return_value = [
            {"id": 1, "label": "boot", "size": 25000},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_disks_list({"instance_id": 123}, sample_config)
        )
        assert len(result) == 1
        assert "boot" in result[0].text


async def test_instance_disk_create_no_confirm(
    sample_config: Config,
) -> None:
    """Disk create should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_create(
            {
                "instance_id": 123,
                "label": "data",
                "size": 5000,
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_disk_delete_no_confirm(
    sample_config: Config,
) -> None:
    """Disk delete should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_delete(
            {"instance_id": 123, "disk_id": 1},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_disk_get_missing_disk_id(
    sample_config: Config,
) -> None:
    """Disk get should fail without disk_id."""
    result = list(
        await handle_linode_instance_disk_get({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "disk_id" in result[0].text.lower()


async def test_instance_disk_update_no_confirm(
    sample_config: Config,
) -> None:
    """Disk update should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_update(
            {"instance_id": 123, "disk_id": 1},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_disk_clone_no_confirm(
    sample_config: Config,
) -> None:
    """Disk clone should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_clone(
            {"instance_id": 123, "disk_id": 1},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_disk_resize_no_confirm(
    sample_config: Config,
) -> None:
    """Disk resize should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_resize(
            {
                "instance_id": 123,
                "disk_id": 1,
                "size": 30000,
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


# ── Instance IPs tool definition tests ──


async def test_instance_ips_list_tool_def() -> None:
    """IPs list tool should require instance_id."""
    tool, _ = create_linode_instance_ips_list_tool()
    assert tool.name == "linode_instance_ips_list"
    assert "instance_id" in (tool.inputSchema.get("required") or [])


async def test_instance_ip_get_tool_def() -> None:
    """IP get tool should require instance_id and address."""
    tool, _ = create_linode_instance_ip_get_tool()
    assert tool.name == "linode_instance_ip_get"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "address" in required


async def test_instance_ip_allocate_tool_def() -> None:
    """IP allocate should require instance_id, type, confirm."""
    tool, _ = create_linode_instance_ip_allocate_tool()
    assert tool.name == "linode_instance_ip_allocate"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "type" in required
    assert "confirm" in required


async def test_instance_ip_update_tool_def() -> None:
    """IP update should require instance_id, address, rdns, confirm."""
    tool, _ = create_linode_instance_ip_update_tool()
    assert tool.name == "linode_instance_ip_update"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "address" in required
    assert "rdns" in required
    assert "confirm" in required


async def test_instance_ip_delete_tool_def() -> None:
    """IP delete should require instance_id, address, confirm."""
    tool, _ = create_linode_instance_ip_delete_tool()
    assert tool.name == "linode_instance_ip_delete"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "address" in required
    assert "confirm" in required


# ── Instance IPs handler tests ──


async def test_instance_ips_list_success(
    sample_config: Config,
) -> None:
    """IPs list should return IP data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_ips.return_value = {
            "ipv4": {
                "public": [{"address": "192.0.2.1"}],
            },
            "ipv6": {
                "slaac": {"address": "2001:db8::1"},
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_ips_list({"instance_id": 123}, sample_config)
        )
        assert len(result) == 1
        assert "192.0.2.1" in result[0].text


async def test_instance_ip_get_missing_address(
    sample_config: Config,
) -> None:
    """IP get should fail without address."""
    result = list(
        await handle_linode_instance_ip_get({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "address" in result[0].text.lower()


async def test_instance_ip_allocate_no_confirm(
    sample_config: Config,
) -> None:
    """IP allocate should require confirm=true."""
    result = list(
        await handle_linode_instance_ip_allocate(
            {"instance_id": 123, "type": "ipv4"},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_ip_allocate_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the allocate with no resource state and no call."""
    result = list(
        await handle_linode_instance_ip_allocate(
            {"instance_id": 123, "type": "ipv4", "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_ip_allocate"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/ips"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_instance_ip_allocate_dry_run_still_validates_type(
    sample_config: Config,
) -> None:
    """Missing type must error out regardless of dry_run."""
    result = list(
        await handle_linode_instance_ip_allocate(
            {"instance_id": 123, "dry_run": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "type is required" in result[0].text


async def test_instance_ip_update_no_confirm(
    sample_config: Config,
) -> None:
    """IP update should require confirm=true."""
    result = list(
        await handle_linode_instance_ip_update(
            {
                "instance_id": 123,
                "address": "192.0.2.1",
                "rdns": "host.example.com",
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_ip_update_missing_rdns(
    sample_config: Config,
) -> None:
    """IP update should require an rdns argument."""
    result = list(
        await handle_linode_instance_ip_update(
            {
                "instance_id": 123,
                "address": "192.0.2.1",
                "confirm": True,
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "rdns" in result[0].text.lower()


async def test_instance_ip_delete_no_confirm(
    sample_config: Config,
) -> None:
    """IP delete should require confirm=true."""
    result = list(
        await handle_linode_instance_ip_delete(
            {
                "instance_id": 123,
                "address": "192.0.2.1",
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


# ── Instance Actions tool definition tests ──


async def test_instance_clone_tool_def() -> None:
    """Clone tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_clone_tool()
    assert tool.name == "linode_instance_clone"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


async def test_instance_migrate_tool_def() -> None:
    """Migrate tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_migrate_tool()
    assert tool.name == "linode_instance_migrate"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


async def test_instance_rebuild_tool_def() -> None:
    """Rebuild should require instance_id, image, root_pass, confirm."""
    tool, _ = create_linode_instance_rebuild_tool()
    assert tool.name == "linode_instance_rebuild"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "image" in required
    assert "root_pass" in required
    assert "confirm" in required


async def test_instance_rescue_tool_def() -> None:
    """Rescue tool should require instance_id and confirm."""
    tool, _ = create_linode_instance_rescue_tool()
    assert tool.name == "linode_instance_rescue"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "confirm" in required


async def test_instance_password_reset_tool_def() -> None:
    """Password reset should require instance_id, root_pass, confirm."""
    tool, _ = create_linode_instance_password_reset_tool()
    assert tool.name == "linode_instance_password_reset"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "root_pass" in required
    assert "confirm" in required


# ── Instance Actions handler tests ──


async def test_instance_clone_no_confirm(
    sample_config: Config,
) -> None:
    """Clone should require confirm=true."""
    result = list(
        await handle_linode_instance_clone({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_clone_success(
    sample_config: Config,
) -> None:
    """Clone should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.clone_instance.return_value = {
            "id": 999,
            "label": "cloned",
            "status": "provisioning",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_clone(
                {"instance_id": 123, "confirm": True},
                sample_config,
            )
        )
        assert len(result) == 1
        assert "cloned" in result[0].text


async def test_instance_migrate_no_confirm(
    sample_config: Config,
) -> None:
    """Migrate should require confirm=true."""
    result = list(
        await handle_linode_instance_migrate({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_rebuild_no_confirm(
    sample_config: Config,
) -> None:
    """Rebuild should require confirm=true."""
    result = list(
        await handle_linode_instance_rebuild(
            {
                "instance_id": 123,
                "image": "linode/ubuntu22.04",
                "root_pass": "S3cure!Pass123",
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_rebuild_missing_image(
    sample_config: Config,
) -> None:
    """Rebuild should fail without image."""
    result = list(
        await handle_linode_instance_rebuild(
            {
                "instance_id": 123,
                "root_pass": "S3cure!Pass123",
                "confirm": True,
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "image" in result[0].text.lower()


async def test_instance_rescue_no_confirm(
    sample_config: Config,
) -> None:
    """Rescue should require confirm=true."""
    result = list(
        await handle_linode_instance_rescue({"instance_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_password_reset_no_confirm(
    sample_config: Config,
) -> None:
    """Password reset should require confirm=true."""
    result = list(
        await handle_linode_instance_password_reset(
            {
                "instance_id": 123,
                "root_pass": "NewPass123!",
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_password_reset_missing_pass(
    sample_config: Config,
) -> None:
    """Password reset should fail without root_pass."""
    result = list(
        await handle_linode_instance_password_reset(
            {"instance_id": 123, "confirm": True},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "root_pass" in result[0].text.lower()


# ---------------------------------------------------------------------------
# execute_tool wrapper tests
# ---------------------------------------------------------------------------


async def test_execute_tool_missing_environment(sample_config: Config) -> None:
    """execute_tool returns an error when the requested environment doesn't exist."""
    result = await handle_linode_profile({"environment": "nonexistent"}, sample_config)
    assert len(result) == 1
    assert "error" in result[0].text.lower()


async def test_execute_tool_empty_token(sample_config: Config) -> None:
    """execute_tool returns an error when the Linode token is empty."""
    from linodemcp.config import EnvironmentConfig, LinodeConfig

    bad_config = Config(
        server=sample_config.server,
        observability=sample_config.observability,
        resilience=sample_config.resilience,
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="",
                ),
            ),
        },
    )
    result = await handle_linode_profile({}, bad_config)
    assert len(result) == 1
    assert "error" in result[0].text.lower()


async def test_execute_tool_client_lifecycle(sample_config: Config) -> None:
    """execute_tool enters and exits the RetryableClient context manager."""
    mock_profile = Profile(
        username="lifecycle",
        email="lc@test.com",
        timezone="UTC",
        email_notifications=False,
        restricted=False,
        two_factor_auth=False,
        uid=1,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        await handle_linode_profile({}, sample_config)

        mock_client.__aenter__.assert_called_once()
        mock_client.__aexit__.assert_called_once()


async def test_execute_tool_callback_exception(sample_config: Config) -> None:
    """execute_tool catches handler exceptions and wraps them in error text."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile({}, sample_config)

        assert len(result) == 1
        assert "Failed to" in result[0].text
        assert "boom" in result[0].text


# ---------------------------------------------------------------------------
# Instance status filter tests
# ---------------------------------------------------------------------------


def _make_instance(
    instance_id: int,
    label: str,
    status: str,
    sample_instance_data: dict[str, Any],
) -> Instance:
    """Build an Instance with the given id, label, and status."""
    return Instance(
        id=instance_id,
        label=label,
        status=status,
        type="g6-standard-1",
        region="us-east",
        image="linode/ubuntu22.04",
        ipv4=["192.0.2.1"],
        ipv6="2001:db8::1/64",
        hypervisor="kvm",
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=True,
            available=True,
            schedule=Schedule(day="Saturday", window="W22"),
            last_successful=None,
        ),
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
        group="",
        tags=[],
        watchdog_enabled=True,
    )


async def test_instance_status_filter_returns_matching(
    sample_config: Config,
    sample_instance_data: dict[str, Any],
) -> None:
    """Filtering by status=running keeps only running instances."""
    instances = [
        _make_instance(1, "web-1", "running", sample_instance_data),
        _make_instance(2, "db-1", "offline", sample_instance_data),
        _make_instance(3, "web-2", "running", sample_instance_data),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = instances
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list(
            {"status": "running"}, sample_config
        )

    data = json.loads(result[0].text)
    assert data["count"] == 2
    labels = [inst["label"] for inst in data["instances"]]
    assert "web-1" in labels
    assert "web-2" in labels
    assert "db-1" not in labels


async def test_instance_no_filter_returns_all(
    sample_config: Config,
    sample_instance_data: dict[str, Any],
) -> None:
    """Without a status filter, all instances are returned."""
    instances = [
        _make_instance(1, "web-1", "running", sample_instance_data),
        _make_instance(2, "db-1", "offline", sample_instance_data),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = instances
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list({}, sample_config)

    data = json.loads(result[0].text)
    assert data["count"] == 2


# ---------------------------------------------------------------------------
# Region capability filter tests
# ---------------------------------------------------------------------------


async def test_region_capability_filter(sample_config: Config) -> None:
    """Filtering regions by capability keeps only matching regions."""
    regions = [
        Region(
            id="us-east",
            label="Newark",
            country="us",
            capabilities=["Linodes", "Kubernetes"],
            status="ok",
            resolvers=Resolver(ipv4="8.8.8.8", ipv6="::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="8.8.4.4", ipv6="::2"),
            site_type="core",
        ),
        Region(
            id="us-west",
            label="Fremont",
            country="us",
            capabilities=["Linodes", "Kubernetes"],
            status="ok",
            resolvers=Resolver(ipv4="1.1.1.1", ipv6="::3"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list(
            {"capability": "Kubernetes"}, sample_config
        )

    data = json.loads(result[0].text)
    assert data["count"] == 2
    region_ids = [r["id"] for r in data["regions"]]
    assert "us-east" in region_ids
    assert "us-west" in region_ids
    assert "eu-west" not in region_ids


async def test_region_no_filter_returns_all(sample_config: Config) -> None:
    """Without filters, all regions are returned."""
    regions = [
        Region(
            id="us-east",
            label="Newark",
            country="us",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="8.8.8.8", ipv6="::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="8.8.4.4", ipv6="::2"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({}, sample_config)

    data = json.loads(result[0].text)
    assert data["count"] == 2


# ---------------------------------------------------------------------------
# Instance Deep: success-path tests (Phase 2)
# ---------------------------------------------------------------------------


async def test_handle_linode_instance_backup_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backup get should return backup data when both IDs are valid."""
    mock_linode_client.get_instance_backup.return_value = {
        "id": 100,
        "label": "daily-backup",
        "status": "successful",
        "type": "auto",
    }
    result = await handle_linode_instance_backup_get(
        {"instance_id": 123, "backup_id": 100}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 100
    assert data["label"] == "daily-backup"
    mock_linode_client.get_instance_backup.assert_called_once_with(123, 100)


async def test_handle_linode_instance_backup_restore_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backup restore should succeed with confirm=true and valid IDs."""
    mock_linode_client.restore_instance_backup.return_value = None
    result = await handle_linode_instance_backup_restore(
        {
            "instance_id": 123,
            "backup_id": 100,
            "linode_id": 456,
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Backup 100 restored to instance 456"
    assert data["instance_id"] == 123
    assert data["backup_id"] == 100
    mock_linode_client.restore_instance_backup.assert_called_once_with(
        123, 100, 456, overwrite=False
    )


async def test_handle_linode_instance_backups_enable_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backups enable should succeed with confirm=true."""
    mock_linode_client.enable_instance_backups.return_value = None
    result = await handle_linode_instance_backups_enable(
        {"instance_id": 123, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Backups enabled for instance 123"
    assert data["instance_id"] == 123
    mock_linode_client.enable_instance_backups.assert_called_once_with(123)


async def test_handle_linode_instance_backups_cancel_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backups cancel should succeed with confirm=true."""
    mock_linode_client.cancel_instance_backups.return_value = None
    result = await handle_linode_instance_backups_cancel(
        {"instance_id": 123, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Backups cancelled for instance 123"
    assert data["instance_id"] == 123
    mock_linode_client.cancel_instance_backups.assert_called_once_with(123)


async def test_instance_backups_cancel_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the instance via GET and never cancel."""
    mock_linode_client.get_instance.return_value = {
        "id": 123,
        "label": "my-linode",
        "status": "running",
    }
    result = await handle_linode_instance_backups_cancel(
        {"instance_id": 123, "dry_run": True}, sample_config
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_backups_cancel"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/backups/cancel"
    mock_linode_client.get_instance.assert_awaited_once_with(123)
    mock_linode_client.cancel_instance_backups.assert_not_called()


async def test_instance_backups_cancel_dry_run_still_validates_instance_id(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing instance_id must error regardless of dry_run."""
    result = await handle_linode_instance_backups_cancel(
        {"dry_run": True}, sample_config
    )
    assert "instance_id" in result[0].text.lower()
    mock_linode_client.cancel_instance_backups.assert_not_called()


async def test_handle_linode_instance_disk_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk get should return disk data when both IDs are valid."""
    mock_linode_client.get_instance_disk.return_value = {
        "id": 10,
        "label": "Ubuntu Disk",
        "size": 51200,
        "filesystem": "ext4",
        "status": "ready",
    }
    result = await handle_linode_instance_disk_get(
        {"instance_id": 123, "disk_id": 10}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 10
    assert data["label"] == "Ubuntu Disk"
    mock_linode_client.get_instance_disk.assert_called_once_with(123, 10)


async def test_handle_linode_instance_disk_create_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk create should succeed with valid args and confirm=true."""
    mock_linode_client.create_instance_disk.return_value = {
        "id": 50,
        "label": "my-disk",
        "size": 1024,
        "filesystem": "ext4",
        "status": "ready",
    }
    result = await handle_linode_instance_disk_create(
        {"instance_id": 123, "label": "my-disk", "size": 1024, "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 50
    assert data["label"] == "my-disk"
    mock_linode_client.create_instance_disk.assert_called_once_with(
        123, label="my-disk", size=1024, filesystem=None, image=None, root_pass=None
    )


async def test_handle_linode_instance_disk_update_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk update should succeed with valid args and confirm=true."""
    mock_linode_client.update_instance_disk.return_value = {
        "id": 10,
        "label": "renamed-disk",
        "size": 51200,
    }
    result = await handle_linode_instance_disk_update(
        {
            "instance_id": 123,
            "disk_id": 10,
            "label": "renamed-disk",
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 10
    assert data["label"] == "renamed-disk"
    mock_linode_client.update_instance_disk.assert_called_once_with(
        123, 10, label="renamed-disk"
    )


async def test_handle_linode_instance_disk_delete_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk delete should succeed with valid args and confirm=true."""
    mock_linode_client.delete_instance_disk.return_value = None
    result = await handle_linode_instance_disk_delete(
        {"instance_id": 123, "disk_id": 10, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Disk 10 deleted from instance 123"
    assert data["instance_id"] == 123
    assert data["disk_id"] == 10
    mock_linode_client.delete_instance_disk.assert_called_once_with(123, 10)


async def test_instance_disk_delete_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the disk via GET and never call delete."""
    mock_linode_client.get_instance_disk.return_value = {
        "id": 10,
        "label": "boot",
        "size": 25600,
    }
    result = await handle_linode_instance_disk_delete(
        {"instance_id": 123, "disk_id": 10, "dry_run": True}, sample_config
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_disk_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks/10"
    mock_linode_client.get_instance_disk.assert_awaited_once_with(123, 10)
    mock_linode_client.delete_instance_disk.assert_not_called()


async def test_instance_disk_delete_dry_run_still_validates_disk_id(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing disk_id must error regardless of dry_run."""
    result = await handle_linode_instance_disk_delete(
        {"instance_id": 123, "dry_run": True}, sample_config
    )
    assert "disk_id" in result[0].text.lower()
    mock_linode_client.delete_instance_disk.assert_not_called()


async def test_instance_backup_create_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never snapshot."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_backup_create(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_backup_create"
    assert body["would_execute"]["path"] == "/linode/instances/123/backups"
    mock_linode_client.create_instance_backup.assert_not_called()


async def test_instance_backup_restore_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the backup via GET and never restore."""
    mock_linode_client.get_instance_backup.return_value = {"id": 456}

    result = await handle_linode_instance_backup_restore(
        {"instance_id": 123, "backup_id": 456, "linode_id": 999, "dry_run": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_backup_restore"
    assert body["would_execute"]["path"] == "/linode/instances/123/backups/456/restore"
    mock_linode_client.restore_instance_backup.assert_not_called()


async def test_instance_backup_restore_dry_run_overwrite_side_effects(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Phase 2 Tier A walk: overwrite=true warns the target is destroyed."""
    mock_linode_client.get_instance_backup.return_value = {"id": 456}

    result = await handle_linode_instance_backup_restore(
        {
            "instance_id": 123,
            "backup_id": 456,
            "linode_id": 999,
            "overwrite": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert len(body["side_effects"]) == 1
    assert "999" in body["side_effects"][0]
    assert body["warnings"]
    mock_linode_client.restore_instance_backup.assert_not_called()


async def test_instance_backups_enable_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never enable."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_backups_enable(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_backups_enable"
    assert body["would_execute"]["path"] == "/linode/instances/123/backups/enable"
    mock_linode_client.enable_instance_backups.assert_not_called()


async def test_instance_disk_create_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the instance via GET and never create."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_disk_create(
        {"instance_id": 123, "label": "data", "size": 10240, "dry_run": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_disk_create"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks"
    mock_linode_client.create_instance_disk.assert_not_called()
    assert len(body["side_effects"]) == 1
    assert "data" in body["side_effects"][0]
    assert "10240" in body["side_effects"][0]


async def test_instance_disk_update_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the disk via GET and never update."""
    mock_linode_client.get_instance_disk.return_value = {"id": 789}

    result = await handle_linode_instance_disk_update(
        {"instance_id": 123, "disk_id": 789, "label": "renamed", "dry_run": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_disk_update"
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks/789"
    mock_linode_client.update_instance_disk.assert_not_called()


async def test_instance_disk_clone_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the disk via GET and never clone."""
    mock_linode_client.get_instance_disk.return_value = {
        "id": 789,
        "label": "boot",
        "size": 25600,
    }

    result = await handle_linode_instance_disk_clone(
        {"instance_id": 123, "disk_id": 789, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_disk_clone"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks/789/clone"
    assert "25600 MB" in body["side_effects"][0]
    mock_linode_client.clone_instance_disk.assert_not_called()


async def test_instance_disk_resize_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the disk via GET and never resize."""
    mock_linode_client.get_instance_disk.return_value = {"id": 789, "size": 10240}

    result = await handle_linode_instance_disk_resize(
        {"instance_id": 123, "disk_id": 789, "size": 20480, "dry_run": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_disk_resize"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks/789/resize"
    effect = body["side_effects"][0]
    assert "10240 MB" in effect
    assert "20480 MB" in effect
    assert body["warnings"]
    mock_linode_client.resize_instance_disk.assert_not_called()


async def test_handle_linode_instance_disk_clone_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk clone should succeed with valid args and confirm=true."""
    mock_linode_client.clone_instance_disk.return_value = {
        "id": 99,
        "label": "cloned-disk",
        "size": 51200,
    }
    result = await handle_linode_instance_disk_clone(
        {"instance_id": 123, "disk_id": 10, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 99
    assert data["label"] == "cloned-disk"
    mock_linode_client.clone_instance_disk.assert_called_once_with(123, 10)


async def test_handle_linode_instance_disk_resize_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk resize should succeed with valid args and confirm=true."""
    mock_linode_client.resize_instance_disk.return_value = None
    result = await handle_linode_instance_disk_resize(
        {"instance_id": 123, "disk_id": 10, "size": 65536, "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Disk 10 resized to 65536 MB"
    assert data["instance_id"] == 123
    assert data["disk_id"] == 10
    assert data["size"] == 65536
    mock_linode_client.resize_instance_disk.assert_called_once_with(123, 10, 65536)


async def test_handle_linode_instance_ip_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP get should return IP data when instance_id and address are valid."""
    mock_linode_client.get_instance_ip.return_value = {
        "address": "203.0.113.1",
        "type": "ipv4",
        "public": True,
        "region": "us-east",
    }
    result = await handle_linode_instance_ip_get(
        {"instance_id": 123, "address": "203.0.113.1"}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["address"] == "203.0.113.1"
    assert data["region"] == "us-east"
    mock_linode_client.get_instance_ip.assert_called_once_with(123, "203.0.113.1")


async def test_handle_linode_instance_ip_allocate_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP allocate should succeed with confirm=true."""
    mock_linode_client.allocate_instance_ip.return_value = {
        "address": "198.51.100.5",
        "type": "ipv4",
        "public": True,
    }
    result = await handle_linode_instance_ip_allocate(
        {"instance_id": 123, "type": "ipv4", "public": True, "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["address"] == "198.51.100.5"
    mock_linode_client.allocate_instance_ip.assert_called_once_with(
        123, ip_type="ipv4", public=True
    )


async def test_handle_linode_instance_ip_update_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP update should update RDNS with confirm=true."""
    mock_linode_client.update_instance_ip.return_value = {
        "address": "203.0.113.1",
        "rdns": "host.example.com",
    }
    result = await handle_linode_instance_ip_update(
        {
            "instance_id": 123,
            "address": "203.0.113.1",
            "rdns": "host.example.com",
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["address"] == "203.0.113.1"
    assert data["rdns"] == "host.example.com"
    mock_linode_client.update_instance_ip.assert_called_once_with(
        123,
        "203.0.113.1",
        "host.example.com",
    )


async def test_handle_linode_instance_ip_update_null_rdns_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP update should allow null RDNS."""
    mock_linode_client.update_instance_ip.return_value = {
        "address": "203.0.113.1",
        "rdns": None,
    }
    result = await handle_linode_instance_ip_update(
        {
            "instance_id": 123,
            "address": "203.0.113.1",
            "rdns": None,
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["rdns"] is None
    mock_linode_client.update_instance_ip.assert_called_once_with(
        123,
        "203.0.113.1",
        None,
    )


async def test_handle_linode_instance_ip_delete_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP delete should succeed with confirm=true."""
    mock_linode_client.delete_instance_ip.return_value = None
    result = await handle_linode_instance_ip_delete(
        {"instance_id": 123, "address": "203.0.113.1", "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "IP 203.0.113.1 deleted from instance 123"
    assert data["instance_id"] == 123
    assert data["address"] == "203.0.113.1"
    mock_linode_client.delete_instance_ip.assert_called_once_with(123, "203.0.113.1")


async def test_instance_ip_delete_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the IP via GET and never call delete."""
    mock_linode_client.get_instance_ip.return_value = {
        "address": "203.0.113.1",
        "type": "ipv4",
        "public": True,
    }
    result = await handle_linode_instance_ip_delete(
        {"instance_id": 123, "address": "203.0.113.1", "dry_run": True},
        sample_config,
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_ip_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/linode/instances/123/ips/203.0.113.1"
    mock_linode_client.get_instance_ip.assert_awaited_once_with(123, "203.0.113.1")
    mock_linode_client.delete_instance_ip.assert_not_called()


async def test_instance_ip_delete_dry_run_still_validates_address(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing address must error regardless of dry_run."""
    result = await handle_linode_instance_ip_delete(
        {"instance_id": 123, "dry_run": True}, sample_config
    )
    assert "address" in result[0].text.lower()
    mock_linode_client.delete_instance_ip.assert_not_called()


async def test_handle_linode_instance_migrate_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Migrate should succeed with confirm=true."""
    mock_linode_client.migrate_instance.return_value = None
    result = await handle_linode_instance_migrate(
        {"instance_id": 123, "region": "eu-west", "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Migration initiated for instance 123"
    assert data["instance_id"] == 123
    mock_linode_client.migrate_instance.assert_called_once_with(123, region="eu-west")


async def test_handle_linode_instance_rebuild_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Rebuild should succeed with confirm=true and required fields."""
    mock_linode_client.rebuild_instance.return_value = {
        "id": 123,
        "label": "my-linode",
        "status": "rebuilding",
    }
    result = await handle_linode_instance_rebuild(
        {
            "instance_id": 123,
            "image": "linode/ubuntu24.04",
            "root_pass": "Str0ngP@ssw0rd!",
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 123
    assert data["status"] == "rebuilding"
    mock_linode_client.rebuild_instance.assert_called_once_with(
        123,
        image="linode/ubuntu24.04",
        root_pass="Str0ngP@ssw0rd!",
        authorized_keys=None,
        authorized_users=None,
    )


async def test_instance_rebuild_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the instance via GET and never rebuild."""
    mock_linode_client.get_instance.return_value = {
        "id": 123,
        "label": "my-linode",
        "status": "running",
    }
    result = await handle_linode_instance_rebuild(
        {
            "instance_id": 123,
            "image": "linode/ubuntu24.04",
            "root_pass": "Str0ngP@ssw0rd!",
            "dry_run": True,
        },
        sample_config,
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_rebuild"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/rebuild"
    mock_linode_client.get_instance.assert_awaited_once_with(123)
    mock_linode_client.rebuild_instance.assert_not_called()


async def test_instance_rebuild_dry_run_still_validates_root_pass(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing root_pass must error regardless of dry_run."""
    result = await handle_linode_instance_rebuild(
        {"instance_id": 123, "image": "linode/ubuntu24.04", "dry_run": True},
        sample_config,
    )
    assert "root_pass is required" in result[0].text
    mock_linode_client.rebuild_instance.assert_not_called()


async def test_instance_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_instance_create(
        {
            "region": "us-east",
            "type": "g6-nanode-1",
            "firewall_id": 789,
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances"
    assert body["current_state"] is None
    assert any("g6-nanode-1" in s for s in body["side_effects"])
    assert body["warnings"]
    assert "confirm=true" not in result[0].text


async def test_instance_create_dry_run_still_validates_firewall_id(
    sample_config: Config,
) -> None:
    """Missing firewall_id must error out regardless of dry_run."""
    result = await handle_linode_instance_create(
        {"region": "us-east", "type": "g6-nanode-1", "dry_run": True},
        sample_config,
    )

    assert "firewall_id is required" in result[0].text


async def test_instance_boot_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never boot."""
    mock_linode_client.get_instance.return_value = {"id": 123, "status": "offline"}

    result = await handle_linode_instance_boot(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_boot"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/boot"
    mock_linode_client.get_instance.assert_awaited_once_with(123)
    mock_linode_client.boot_instance.assert_not_called()


async def test_instance_reboot_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never reboot."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_reboot(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_reboot"
    assert body["would_execute"]["path"] == "/linode/instances/123/reboot"
    mock_linode_client.reboot_instance.assert_not_called()


async def test_instance_shutdown_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never shut down."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_shutdown(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_shutdown"
    assert body["would_execute"]["path"] == "/linode/instances/123/shutdown"
    mock_linode_client.shutdown_instance.assert_not_called()


async def test_instance_resize_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never resize."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_resize(
        {"instance_id": 123, "type": "g6-standard-1", "dry_run": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_resize"
    assert body["would_execute"]["path"] == "/linode/instances/123/resize"
    mock_linode_client.resize_instance.assert_not_called()


async def test_instance_resize_dry_run_still_validates_type(
    sample_config: Config,
) -> None:
    """Missing type must error out regardless of dry_run."""
    result = await handle_linode_instance_resize(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    assert "type is required" in result[0].text


async def test_instance_clone_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never clone."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_clone(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_clone"
    assert body["would_execute"]["path"] == "/linode/instances/123/clone"
    mock_linode_client.clone_instance.assert_not_called()


async def test_instance_migrate_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never migrate."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_migrate(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_migrate"
    assert body["would_execute"]["path"] == "/linode/instances/123/migrate"
    mock_linode_client.migrate_instance.assert_not_called()


def _instance_with(**overrides: Any) -> Any:
    """Build a real Instance dataclass with the given fields set."""
    from dataclasses import fields as dataclass_fields

    from linodemcp.linode import Instance

    kwargs: dict[str, Any] = {f.name: None for f in dataclass_fields(Instance)}
    kwargs.update(overrides)
    return Instance(**kwargs)


async def test_instance_resize_dry_run_surfaces_type_change(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: resize names the type change + price warning."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = _instance_with(type="g6-nanode-1")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_resize(
            {"instance_id": 123, "type": "g6-standard-1", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        effect = body["side_effects"][0]
        assert "g6-nanode-1" in effect
        assert "g6-standard-1" in effect
        assert body["warnings"]
        mock_client.resize_instance.assert_not_called()


async def test_instance_migrate_dry_run_surfaces_region_change(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: migrate names the region change."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = _instance_with(region="us-east")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_migrate(
            {"instance_id": 123, "region": "us-west", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        effect = body["side_effects"][0]
        assert "us-east" in effect
        assert "us-west" in effect
        mock_client.migrate_instance.assert_not_called()


async def test_instance_rescue_dry_run_returns_preview_without_mutating(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch state via GET and never rescue."""
    mock_linode_client.get_instance.return_value = {"id": 123}

    result = await handle_linode_instance_rescue(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_rescue"
    assert body["would_execute"]["path"] == "/linode/instances/123/rescue"
    mock_linode_client.rescue_instance.assert_not_called()


async def test_handle_linode_instance_rescue_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Rescue should succeed with confirm=true."""
    mock_linode_client.rescue_instance.return_value = None
    result = await handle_linode_instance_rescue(
        {"instance_id": 123, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Rescue mode initiated for instance 123"
    assert data["instance_id"] == 123
    mock_linode_client.rescue_instance.assert_called_once_with(123, devices=None)


async def test_handle_linode_instance_password_reset_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Password reset should succeed with confirm=true and root_pass."""
    mock_linode_client.reset_instance_password.return_value = None
    result = await handle_linode_instance_password_reset(
        {"instance_id": 123, "root_pass": "NewStr0ngP@ss!", "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Password reset for instance 123"
    assert data["instance_id"] == 123
    mock_linode_client.reset_instance_password.assert_called_once_with(
        123, "NewStr0ngP@ss!"
    )


async def test_instance_password_reset_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must fetch the instance via GET and never reset."""
    mock_linode_client.get_instance.return_value = {
        "id": 123,
        "label": "my-linode",
        "status": "offline",
    }
    result = await handle_linode_instance_password_reset(
        {"instance_id": 123, "root_pass": "NewStr0ngP@ss!", "dry_run": True},
        sample_config,
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_password_reset"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/password"
    mock_linode_client.get_instance.assert_awaited_once_with(123)
    mock_linode_client.reset_instance_password.assert_not_called()


async def test_instance_password_reset_dry_run_still_validates_root_pass(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing root_pass must error regardless of dry_run."""
    result = await handle_linode_instance_password_reset(
        {"instance_id": 123, "dry_run": True}, sample_config
    )
    assert "root_pass is required" in result[0].text
    mock_linode_client.reset_instance_password.assert_not_called()


def _running_instance() -> Any:
    """Build a real Instance dataclass marked running, for side-effect walks."""
    from dataclasses import fields as dataclass_fields

    from linodemcp.linode import Instance

    instance_kwargs: dict[str, Any] = {
        field.name: None for field in dataclass_fields(Instance)
    }
    instance_kwargs["status"] = "running"
    instance_kwargs["image"] = "linode/debian12"
    return Instance(**instance_kwargs)


async def test_instance_rebuild_dry_run_surfaces_disk_side_effects(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: each disk is erased; the image is named in a warning."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = _running_instance()
        mock_client.list_instance_disks.return_value = [
            {"id": 1, "label": "boot", "size": 25600, "filesystem": "ext4"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_rebuild(
            {
                "instance_id": 123,
                "image": "linode/ubuntu24.04",
                "root_pass": "Str0ngP@ssw0rd!",
                "dry_run": True,
            },
            sample_config,
        )

        body = json.loads(result[0].text)
        assert len(body["side_effects"]) == 1
        assert any("linode/debian12" in w for w in body["warnings"])
        mock_client.rebuild_instance.assert_not_called()


async def test_instance_rescue_dry_run_surfaces_side_effects(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: rescue-mode reboot side effect + downtime warning."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = _running_instance()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_rescue(
            {"instance_id": 123, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert len(body["side_effects"]) == 1
        assert body["warnings"]
        mock_client.rescue_instance.assert_not_called()


async def test_instance_password_reset_dry_run_surfaces_side_effects(
    sample_config: Config,
) -> None:
    """Phase 2 Tier A walk: power-down/reboot side effect + downtime warning."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = _running_instance()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_password_reset(
            {"instance_id": 123, "root_pass": "Str0ngP@ssw0rd!", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert len(body["side_effects"]) == 1
        assert body["warnings"]
        mock_client.reset_instance_password.assert_not_called()


async def test_handle_linode_instance_backup_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backup get should return error text when the API call fails."""
    mock_linode_client.get_instance_backup.side_effect = Exception("API error")
    result = await handle_linode_instance_backup_get(
        {"instance_id": 123, "backup_id": 100}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_disk_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk get should return error text when the API call fails."""
    mock_linode_client.get_instance_disk.side_effect = Exception("API error")
    result = await handle_linode_instance_disk_get(
        {"instance_id": 123, "disk_id": 10}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_ip_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP get should return error text when the API call fails."""
    mock_linode_client.get_instance_ip.side_effect = Exception("API error")
    result = await handle_linode_instance_ip_get(
        {"instance_id": 123, "address": "203.0.113.1"}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_ip_update_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP update should return error text when the API call fails."""
    mock_linode_client.update_instance_ip.side_effect = Exception("API error")
    result = await handle_linode_instance_ip_update(
        {
            "instance_id": 123,
            "address": "203.0.113.1",
            "rdns": "host.example.com",
            "confirm": True,
        },
        sample_config,
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_migrate_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Migrate should return error text when the API call fails."""
    mock_linode_client.migrate_instance.side_effect = Exception("API error")
    result = await handle_linode_instance_migrate(
        {"instance_id": 123, "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_monitor_services_list_tool() -> None:
    """Test linode_monitor_services_list tool creation."""
    tool, capability = create_linode_monitor_services_list_tool()
    assert tool.name == "linode_monitor_services_list"
    assert capability is Capability.Read
    assert tool.inputSchema["type"] == "object"
    assert "environment" in tool.inputSchema["properties"]
    assert "confirm" not in tool.inputSchema["properties"]


async def test_handle_linode_monitor_services_list(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """Test linode_monitor_services_list tool handler."""
    mock_linode_client.list_monitor_services.return_value = {
        "data": [{"label": "Databases", "service_type": "dbaas"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    result = await handle_linode_monitor_services_list({}, sample_config)

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["services"][0]["service_type"] == "dbaas"
    assert payload["results"] == 1
    mock_linode_client.list_monitor_services.assert_awaited_once_with()


async def test_handle_linode_monitor_services_list_error(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """Test linode_monitor_services_list error handling."""
    mock_linode_client.list_monitor_services.side_effect = Exception("API error")
    result = await handle_linode_monitor_services_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to list monitor services: API error" in result[0].text


def test_create_linode_monitor_service_get_tool() -> None:
    """Tool definition advertises required service_type without confirm."""
    tool, capability = create_linode_monitor_service_get_tool()
    assert tool.name == "linode_monitor_service_get"
    assert capability is Capability.Read
    schema = tool.inputSchema
    assert "confirm" not in schema["properties"]
    assert schema["required"] == ["service_type"]
    assert schema["properties"]["service_type"]["pattern"] == "^[A-Za-z0-9_-]+$"


async def test_handle_linode_monitor_service_get(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Handler returns monitor service data from a successful client call."""
    mock_linode_client.get_monitor_service.return_value = {
        "label": "Databases",
        "service_type": "dbaas",
    }
    result = await handle_linode_monitor_service_get(
        {"service_type": "dbaas"}, sample_config
    )
    assert len(result) == 1
    text = result[0].text
    assert "Databases" in text
    assert "dbaas" in text
    mock_linode_client.get_monitor_service.assert_awaited_once_with("dbaas")


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_handle_linode_monitor_service_get_bad_service_type(
    bad_service_type: str, sample_config: Config
) -> None:
    """Malformed service_type values return a validation error."""
    result = await handle_linode_monitor_service_get(
        {"service_type": bad_service_type}, sample_config
    )
    assert len(result) == 1
    assert "service_type" in result[0].text
    assert "Error" in result[0].text


async def test_handle_linode_monitor_service_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """API errors surface as a 'Failed to' message in the response text."""
    mock_linode_client.get_monitor_service.side_effect = Exception("API error")
    result = await handle_linode_monitor_service_get(
        {"service_type": "dbaas"}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_monitor_service_alert_definition_get_tool() -> None:
    """Tool definition advertises required service_type and alert_id."""
    tool, capability = create_linode_monitor_service_alert_definition_get_tool()
    assert tool.name == "linode_monitor_service_alert_definition_get"
    assert capability is Capability.Read
    schema = tool.inputSchema
    assert "confirm" not in schema["properties"]
    assert schema["required"] == ["service_type", "alert_id"]
    assert schema["properties"]["service_type"]["pattern"] == "^[A-Za-z0-9_-]+$"
    assert schema["properties"]["alert_id"]["type"] == "integer"
    assert schema["properties"]["alert_id"]["minimum"] == 1


async def test_handle_linode_monitor_service_alert_definition_get(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Handler returns alert definition data from a successful client call."""
    mock_linode_client.get_monitor_service_alert_definition.return_value = {
        "id": 12345,
        "label": "CPU high",
    }
    result = await handle_linode_monitor_service_alert_definition_get(
        {"service_type": "dbaas", "alert_id": 12345}, sample_config
    )
    assert len(result) == 1
    text = result[0].text
    assert "CPU high" in text
    assert "dbaas" in text
    mock_linode_client.get_monitor_service_alert_definition.assert_awaited_once_with(
        "dbaas", 12345
    )


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_handle_linode_monitor_service_alert_definition_get_bad_service_type(
    bad_service_type: str, sample_config: Config
) -> None:
    """Malformed service_type values return a validation error."""
    result = await handle_linode_monitor_service_alert_definition_get(
        {"service_type": bad_service_type, "alert_id": 12345}, sample_config
    )
    assert len(result) == 1
    assert "service_type" in result[0].text
    assert "Error" in result[0].text


@pytest.mark.parametrize(
    "bad_alert_id", [None, True, "12345", "1/2", "1?x", "..", 12.9, 0, -1]
)
async def test_handle_linode_monitor_service_alert_definition_get_bad_alert_id(
    bad_alert_id: object, sample_config: Config
) -> None:
    """Malformed alert_id values return a validation error."""
    args: dict[str, object] = {"service_type": "dbaas"}
    if bad_alert_id is not None:
        args["alert_id"] = bad_alert_id
    result = await handle_linode_monitor_service_alert_definition_get(
        args, sample_config
    )
    assert len(result) == 1
    assert "alert_id" in result[0].text
    assert "Error" in result[0].text


def test_create_linode_monitor_service_token_create_tool() -> None:
    """Tool definition advertises required service_type and entity_ids."""
    tool, _ = create_linode_monitor_service_token_create_tool()
    assert tool.name == "linode_monitor_service_token_create"
    schema = tool.inputSchema
    required = schema["required"]
    assert "service_type" in required
    assert "entity_ids" in required
    props = schema["properties"]
    assert props["entity_ids"]["type"] == "array"
    assert props["entity_ids"]["items"]["type"] == "integer"
    assert props["entity_ids"]["minItems"] == 1


async def test_handle_linode_monitor_service_token_create(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Handler returns the token and expiry from a successful client call."""
    mock_linode_client.create_monitor_service_token.return_value = {
        "token": "jwt.payload.signature",
        "expiry": "2026-06-01T00:00:00Z",
    }
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": [1, 2, 3], "confirm": True},
        sample_config,
    )
    assert len(result) == 1
    text = result[0].text
    assert "jwt.payload.signature" in text
    assert "2026-06-01T00:00:00Z" in text
    assert "dbaas" in text
    mock_linode_client.create_monitor_service_token.assert_awaited_once_with(
        "dbaas", [1, 2, 3]
    )


async def test_handle_linode_monitor_service_token_create_missing_service_type(
    sample_config: Config,
) -> None:
    """Missing or empty service_type returns a validation error."""
    result = await handle_linode_monitor_service_token_create(
        {"entity_ids": [1], "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "service_type" in result[0].text
    assert "Error" in result[0].text


async def test_handle_linode_monitor_service_token_create_missing_entity_ids(
    sample_config: Config,
) -> None:
    """Missing entity_ids returns a validation error."""
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "entity_ids" in result[0].text
    assert "Error" in result[0].text


async def test_handle_linode_monitor_service_token_create_empty_entity_ids(
    sample_config: Config,
) -> None:
    """Empty entity_ids list returns a validation error."""
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": [], "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "entity_ids" in result[0].text
    assert "Error" in result[0].text


async def test_handle_linode_monitor_service_token_create_non_int_entity_ids(
    sample_config: Config,
) -> None:
    """Non-integer entity_ids (including bool) are rejected."""
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": ["abc"], "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "entity_ids" in result[0].text
    # bool is a subclass of int; reject it explicitly.
    result_bool = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": [True], "confirm": True}, sample_config
    )
    assert "entity_ids" in result_bool[0].text


async def test_handle_linode_monitor_service_token_create_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """API errors surface as a 'Failed to' message in the response text."""
    mock_linode_client.create_monitor_service_token.side_effect = Exception("API error")
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": [1], "confirm": True}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_tfa_enable_tool() -> None:
    """Profile TFA enable tool exposes a strict confirmation gate."""
    tool, capability = create_linode_profile_tfa_enable_tool()

    assert tool.name == "linode_profile_tfa_enable"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_tfa_enable_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile TFA enable requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {}
        if confirm is not None:
            arguments["confirm"] = confirm

        result = await handle_linode_profile_tfa_enable(arguments, sample_config)

        assert len(result) == 1
        assert "confirm=true" in result[0].text


async def test_handle_linode_profile_tfa_enable_success(
    sample_config: Config,
) -> None:
    """Profile TFA enable calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.create_profile_tfa_secret.return_value = {
            "secret": "5FXX6KLACOC33GTC",
            "expiry": "2026-01-01T00:00:00",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_enable(
            {"confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "warning": (
            "IMPORTANT: Save this two-factor authentication secret now. "
            "It must be confirmed before two-factor authentication is enabled."
        ),
        "secret": "5FXX6KLACOC33GTC",
        "expiry": "2026-01-01T00:00:00",
    }
    mock_client.create_profile_tfa_secret.assert_awaited_once_with()


async def test_handle_linode_profile_tfa_enable_error(
    sample_config: Config,
) -> None:
    """Profile TFA enable surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.create_profile_tfa_secret.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_enable(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_tfa_disable_tool() -> None:
    """Profile TFA disable tool exposes a strict confirmation gate."""
    tool, capability = create_linode_profile_tfa_disable_tool()

    assert tool.name == "linode_profile_tfa_disable"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_tfa_disable_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile TFA disable requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {}
        if confirm is not None:
            arguments["confirm"] = confirm

        result = await handle_linode_profile_tfa_disable(arguments, sample_config)

        assert len(result) == 1
        assert "confirm=true" in result[0].text


async def test_handle_linode_profile_tfa_disable_success(
    sample_config: Config,
) -> None:
    """Profile TFA disable calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.disable_profile_tfa.return_value = {}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_disable(
            {"confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {}
    mock_client.disable_profile_tfa.assert_awaited_once_with()


async def test_handle_linode_profile_tfa_disable_error(
    sample_config: Config,
) -> None:
    """Profile TFA disable surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.disable_profile_tfa.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_disable(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_tfa_enable_confirm_tool() -> None:
    """Profile TFA enable confirm tool exposes the documented body field."""
    tool, capability = create_linode_profile_tfa_enable_confirm_tool()

    assert tool.name == "linode_profile_tfa_enable_confirm"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["tfa_code", "confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["tfa_code"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_tfa_enable_confirm_requires_tfa_code(
    sample_config: Config,
) -> None:
    """Profile TFA enable confirm validates tfa_code before calling the client."""
    for tfa_code in (None, "", "   ", 123, True):
        result = await handle_linode_profile_tfa_enable_confirm(
            {"tfa_code": tfa_code, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "tfa_code" in result[0].text


async def test_handle_linode_profile_tfa_enable_confirm_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile TFA enable confirm requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {"tfa_code": "123456"}
        if confirm is not None:
            arguments["confirm"] = confirm

        result = await handle_linode_profile_tfa_enable_confirm(
            arguments, sample_config
        )

        assert len(result) == 1
        assert "confirm=true" in result[0].text


async def test_handle_linode_profile_tfa_enable_confirm_success(
    sample_config: Config,
) -> None:
    """Profile TFA enable confirm calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.confirm_profile_tfa_enable.return_value = {
            "scratch": "setup-token",
            "expiry": "2026-01-01T00:00:00",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_enable_confirm(
            {"tfa_code": "123456", "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "scratch": "setup-token",
        "expiry": "2026-01-01T00:00:00",
    }
    mock_client.confirm_profile_tfa_enable.assert_awaited_once_with(tfa_code="123456")


async def test_handle_linode_profile_tfa_enable_confirm_error(
    sample_config: Config,
) -> None:
    """Profile TFA enable confirm surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.confirm_profile_tfa_enable.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_enable_confirm(
            {"tfa_code": "123456", "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_phone_number_send_tool() -> None:
    """Profile phone number send tool exposes schema and write capability."""
    tool, capability = create_linode_profile_phone_number_send_tool()

    assert tool.name == "linode_profile_phone_number_send"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["iso_code", "phone_number", "confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["iso_code"]["minLength"] == 1
    assert tool.inputSchema["properties"]["phone_number"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_phone_number_send_requires_iso_code(
    sample_config: Config,
) -> None:
    """Profile phone number send validates iso_code before client calls."""
    for iso_code in (None, "", "   ", 123, True):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            mock_client = AsyncMock()
            mock_client.__aenter__.return_value = mock_client
            mock_client.__aexit__.return_value = None
            mock_client_class.return_value = mock_client

            result = await handle_linode_profile_phone_number_send(
                {
                    "iso_code": iso_code,
                    "phone_number": "+15551234567",
                    "confirm": True,
                },
                sample_config,
            )

        assert len(result) == 1
        assert "iso_code" in result[0].text
        mock_client.send_profile_phone_number_verification.assert_not_called()


async def test_handle_linode_profile_phone_number_send_requires_phone_number(
    sample_config: Config,
) -> None:
    """Profile phone number send validates phone_number before client calls."""
    for phone_number in (None, "", "   ", 123, True):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            mock_client = AsyncMock()
            mock_client.__aenter__.return_value = mock_client
            mock_client.__aexit__.return_value = None
            mock_client_class.return_value = mock_client

            result = await handle_linode_profile_phone_number_send(
                {"iso_code": "US", "phone_number": phone_number, "confirm": True},
                sample_config,
            )

        assert len(result) == 1
        assert "phone_number" in result[0].text
        mock_client.send_profile_phone_number_verification.assert_not_called()


async def test_handle_linode_profile_phone_number_send_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile phone number send requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {
            "iso_code": "US",
            "phone_number": "+15551234567",
        }
        if confirm is not None:
            arguments["confirm"] = confirm

        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            mock_client = AsyncMock()
            mock_client.__aenter__.return_value = mock_client
            mock_client.__aexit__.return_value = None
            mock_client_class.return_value = mock_client

            result = await handle_linode_profile_phone_number_send(
                arguments, sample_config
            )

        assert len(result) == 1
        assert "confirm=true" in result[0].text
        mock_client.send_profile_phone_number_verification.assert_not_called()


async def test_handle_linode_profile_phone_number_send_success(
    sample_config: Config,
) -> None:
    """Profile phone number send calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.send_profile_phone_number_verification.return_value = {}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_send(
            {
                "iso_code": " US ",
                "phone_number": " +15551234567 ",
                "confirm": True,
            },
            sample_config,
        )

    assert json.loads(result[0].text) == {}
    mock_client.send_profile_phone_number_verification.assert_awaited_once_with(
        "US", "+15551234567"
    )


async def test_handle_linode_profile_phone_number_send_error(
    sample_config: Config,
) -> None:
    """Profile phone number send surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.send_profile_phone_number_verification.side_effect = Exception(
            "API error"
        )
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_send(
            {"iso_code": "US", "phone_number": "+15551234567", "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_phone_number_delete_tool() -> None:
    """Profile phone number delete tool exposes schema and write capability."""
    tool, capability = create_linode_profile_phone_number_delete_tool()

    assert tool.name == "linode_profile_phone_number_delete"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_phone_number_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile phone number delete requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {}
        if confirm is not None:
            arguments["confirm"] = confirm

        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            mock_client = AsyncMock()
            mock_client.__aenter__.return_value = mock_client
            mock_client.__aexit__.return_value = None
            mock_client_class.return_value = mock_client

            result = await handle_linode_profile_phone_number_delete(
                arguments, sample_config
            )

        assert len(result) == 1
        assert "confirm=true" in result[0].text
        mock_client.delete_profile_phone_number.assert_not_called()


async def test_handle_linode_profile_phone_number_delete_success(
    sample_config: Config,
) -> None:
    """Profile phone number delete calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.delete_profile_phone_number.return_value = {}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_delete(
            {"confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {}
    mock_client.delete_profile_phone_number.assert_awaited_once_with()


async def test_handle_linode_profile_phone_number_delete_error(
    sample_config: Config,
) -> None:
    """Profile phone number delete surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.delete_profile_phone_number.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_delete(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_phone_number_verify_tool() -> None:
    """Profile phone number verify tool exposes schema and write capability."""
    tool, capability = create_linode_profile_phone_number_verify_tool()

    assert tool.name == "linode_profile_phone_number_verify"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["otp_code", "confirm"]
    assert "environment" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["otp_code"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_phone_number_verify_requires_otp_code(
    sample_config: Config,
) -> None:
    """Profile phone number verify validates otp_code before client calls."""
    for otp_code in (None, "", "   ", 123, True):
        result = await handle_linode_profile_phone_number_verify(
            {"otp_code": otp_code, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "otp_code" in result[0].text


async def test_handle_linode_profile_phone_number_verify_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile phone number verify requires explicit boolean confirmation."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, Any] = {"otp_code": "123456"}
        if confirm is not None:
            arguments["confirm"] = confirm

        result = await handle_linode_profile_phone_number_verify(
            arguments, sample_config
        )

        assert len(result) == 1
        assert "confirm=true" in result[0].text


async def test_handle_linode_profile_phone_number_verify_success(
    sample_config: Config,
) -> None:
    """Profile phone number verify calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.verify_profile_phone_number.return_value = {}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_verify(
            {"otp_code": " 123456 ", "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {}
    mock_client.verify_profile_phone_number.assert_awaited_once_with("123456")


async def test_handle_linode_profile_phone_number_verify_error(
    sample_config: Config,
) -> None:
    """Profile phone number verify surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.verify_profile_phone_number.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_verify(
            {"otp_code": "123456", "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_security_questions_list_tool() -> None:
    """Profile security questions list tool exposes a read-only schema."""
    tool, capability = create_linode_profile_security_questions_list_tool()

    assert tool.name == "linode_profile_security_questions_list"
    assert capability == Capability.Read
    assert "required" not in tool.inputSchema


async def test_handle_linode_profile_security_questions_list_success(
    sample_config: Config,
) -> None:
    """Profile security questions list handler calls the retryable client."""
    payload = {
        "security_questions": [
            {"id": 1, "question": "In what city were you born?"},
            {"id": 2, "question": "What was your first pet's name?"},
        ]
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_security_questions.return_value = payload
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_questions_list({}, sample_config)

    assert json.loads(result[0].text) == payload
    mock_client.list_profile_security_questions.assert_awaited_once_with()


async def test_handle_linode_profile_security_questions_list_error(
    sample_config: Config,
) -> None:
    """Profile security questions list handler propagates client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_security_questions.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_questions_list({}, sample_config)

    assert "API error" in result[0].text


def test_create_linode_profile_security_questions_answer_tool() -> None:
    """Profile security questions tool exposes schema and write capability."""
    tool, capability = create_linode_profile_security_questions_answer_tool()

    assert tool.name == "linode_profile_security_questions_answer"
    assert capability == Capability.Write
    assert "security_questions" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


async def test_handle_linode_profile_security_questions_answer_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile security questions rejects non-true confirm values first."""
    for value in (None, False, "true", 1):
        arguments: dict[str, Any] = {
            "security_questions": [
                {"question_id": 1, "response": "Gotham City"},
                {"question_id": 2, "response": "Blue"},
                {"question_id": 3, "response": "Pizza"},
            ],
        }
        if value is not None:
            arguments["confirm"] = value

        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_profile_security_questions_answer(
                arguments, sample_config
            )

        assert "Set confirm=true" in result[0].text
        mock_client_class.assert_not_called()


async def test_handle_linode_profile_security_questions_answer_validates_questions(
    sample_config: Config,
) -> None:
    """Profile security questions validates input shape before client calls."""
    invalid_values: tuple[Any, ...] = (
        [],
        "not-a-list",
        [{"question_id": 0, "response": "Blue"}],
        [{"question_id": True, "response": "Blue"}],
        [{"question_id": 1, "response": "Blue"}],
        [
            {"question_id": 1, "response": "no"},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
    )

    for security_questions in invalid_values:
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_profile_security_questions_answer(
                {"security_questions": security_questions, "confirm": True},
                sample_config,
            )

        assert "Error" in result[0].text
        mock_client_class.assert_not_called()


async def test_handle_linode_profile_security_questions_answer_success(
    sample_config: Config,
) -> None:
    """Profile security questions handler calls the retryable client."""
    questions = [
        {"question_id": 1, "response": "Gotham City", "security_question": "ignored"},
        {"question_id": 2, "response": "Blue"},
        {"question_id": 3, "response": "Pizza"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.answer_profile_security_questions.return_value = {
            "security_questions": []
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_questions_answer(
            {"security_questions": questions, "confirm": True}, sample_config
        )

    data = json.loads(result[0].text)
    assert data == {"security_questions": []}
    mock_client.answer_profile_security_questions.assert_awaited_once_with(
        [
            {"question_id": 1, "response": "Gotham City"},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ]
    )


async def test_handle_linode_profile_security_questions_answer_error(
    sample_config: Config,
) -> None:
    """Profile security questions handler propagates client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.answer_profile_security_questions.side_effect = Exception(
            "API error"
        )
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_questions_answer(
            {
                "security_questions": [
                    {"question_id": 1, "response": "Gotham City"},
                    {"question_id": 2, "response": "Blue"},
                    {"question_id": 3, "response": "Pizza"},
                ],
                "confirm": True,
            },
            sample_config,
        )

    assert "API error" in result[0].text


def test_create_linode_profile_token_create_tool() -> None:
    """Profile token create tool exposes documented body fields."""
    tool, capability = create_linode_profile_token_create_tool()

    assert tool.name == "linode_profile_token_create"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["confirm"]
    assert tool.inputSchema["properties"]["label"]["maxLength"] == 100
    assert "expiry" in tool.inputSchema["properties"]
    assert "scopes" in tool.inputSchema["properties"]


async def test_handle_linode_profile_token_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile token create requires explicit confirmation."""
    result = await handle_linode_profile_token_create(
        {"label": "api-token"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_profile_token_create_validates_optional_fields(
    sample_config: Config,
) -> None:
    """Profile token create validates optional body fields before the client call."""
    invalid_arguments = (
        {"label": "", "confirm": True},
        {"label": "   ", "confirm": True},
        {"label": "x" * 101, "confirm": True},
        {"label": 123, "confirm": True},
        {"scopes": "", "confirm": True},
        {"scopes": 123, "confirm": True},
        {"expiry": 123, "confirm": True},
    )

    for arguments in invalid_arguments:
        result = await handle_linode_profile_token_create(arguments, sample_config)

        assert len(result) == 1
        assert "Error" in result[0].text


async def test_handle_linode_profile_token_create_success(
    sample_config: Config,
) -> None:
    """Profile token create returns the one-time token with a warning."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.create_profile_token.return_value = {
            "id": 12345,
            "label": "api-token",
            "scopes": "linodes:read_only",
            "expiry": "2026-01-01T00:00:00",
            "token": "abcdefghijklmnop",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_create(
            {
                "label": "api-token",
                "scopes": "linodes:read_only",
                "expiry": "2026-01-01T00:00:00",
                "confirm": True,
            },
            sample_config,
        )

    assert json.loads(result[0].text) == {
        "warning": (
            "IMPORTANT: The token below is shown ONLY ONCE. "
            "Save it now - it cannot be retrieved later."
        ),
        "token": {
            "id": 12345,
            "label": "api-token",
            "scopes": "linodes:read_only",
            "expiry": "2026-01-01T00:00:00",
            "token": "abcdefghijklmnop",
        },
    }
    mock_client.create_profile_token.assert_awaited_once_with(
        expiry="2026-01-01T00:00:00",
        label="api-token",
        scopes="linodes:read_only",
    )


async def test_handle_linode_profile_token_create_error(
    sample_config: Config,
) -> None:
    """Profile token create surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.create_profile_token.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_create(
            {"label": "api-token", "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_tokens_list_tool() -> None:
    """Profile token list tool exposes only optional environment."""
    tool, capability = create_linode_profile_tokens_list_tool()

    assert tool.name == "linode_profile_tokens_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema
    assert "environment" in tool.inputSchema["properties"]


async def test_handle_linode_profile_tokens_list_success(
    sample_config: Config,
) -> None:
    """Profile token list calls the retryable client and redacts secrets."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_tokens.return_value = [
            {
                "id": 12345,
                "label": "api-token",
                "token": "secret-token",
                "access_token": "secret-access-token",
                "secret": "secret-value",
            },
            {"id": 67890, "label": "ci-token"},
        ]
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tokens_list({}, sample_config)

    assert json.loads(result[0].text) == {
        "tokens": [
            {"id": 12345, "label": "api-token"},
            {"id": 67890, "label": "ci-token"},
        ]
    }
    mock_client.list_profile_tokens.assert_awaited_once_with()


async def test_handle_linode_profile_tokens_list_error(
    sample_config: Config,
) -> None:
    """Profile token list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_tokens.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tokens_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_token_get_tool() -> None:
    """Profile token get tool exposes token_id."""
    tool, capability = create_linode_profile_token_get_tool()

    assert tool.name == "linode_profile_token_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["token_id"]
    assert tool.inputSchema["properties"]["token_id"]["minimum"] == 1


async def test_handle_linode_profile_token_get_requires_token_id(
    sample_config: Config,
) -> None:
    """Profile token get validates token_id before calling the client."""
    for token_id in (
        None,
        True,
        False,
        0,
        -1,
        "123",
        "12/../34?x=1",
        "..",
        "/",
        "?",
    ):
        result = await handle_linode_profile_token_get(
            {"token_id": token_id}, sample_config
        )

        assert len(result) == 1
        assert "token_id" in result[0].text


async def test_handle_linode_profile_token_get_success(
    sample_config: Config,
) -> None:
    """Profile token get calls the retryable client and returns token details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.get_profile_token.return_value = {
            "id": 12345,
            "label": "api-token",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert json.loads(result[0].text) == {"id": 12345, "label": "api-token"}
    mock_client.get_profile_token.assert_awaited_once_with(12345)


async def test_handle_linode_profile_token_get_redacts_secret_fields(
    sample_config: Config,
) -> None:
    """Profile token get does not expose secret token material."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.get_profile_token.return_value = {
            "id": 12345,
            "label": "api-token",
            "token": "secret-token",
            "access_token": "secret-access-token",
            "secret": "secret-value",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert json.loads(result[0].text) == {"id": 12345, "label": "api-token"}
    mock_client.get_profile_token.assert_awaited_once_with(12345)


async def test_handle_linode_profile_token_get_error(
    sample_config: Config,
) -> None:
    """Profile token get surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.get_profile_token.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_logins_list_tool() -> None:
    """Profile login list tool exposes only environment arguments."""
    tool, capability = create_linode_profile_logins_list_tool()

    assert tool.name == "linode_profile_logins_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema
    assert "environment" in tool.inputSchema["properties"]


async def test_handle_linode_profile_logins_list_success(
    sample_config: Config,
) -> None:
    """Profile login list calls the retryable client and returns logins."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_logins.return_value = [
            {"id": 12345, "ip": "192.0.2.10"},
            {"id": 67890, "ip": "192.0.2.11"},
        ]
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_logins_list({}, sample_config)

    assert json.loads(result[0].text) == {
        "logins": [
            {"id": 12345, "ip": "192.0.2.10"},
            {"id": 67890, "ip": "192.0.2.11"},
        ]
    }
    mock_client.list_profile_logins.assert_awaited_once_with()


async def test_handle_linode_profile_logins_list_error(
    sample_config: Config,
) -> None:
    """Profile login list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_logins.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_logins_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_login_get_tool() -> None:
    """Profile login get tool exposes login_id."""
    tool, capability = create_linode_profile_login_get_tool()

    assert tool.name == "linode_profile_login_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["login_id"]
    assert tool.inputSchema["properties"]["login_id"]["minimum"] == 1


async def test_handle_linode_profile_login_get_requires_login_id(
    sample_config: Config,
) -> None:
    """Profile login get validates login_id before calling the client."""
    for login_id in (
        None,
        True,
        False,
        0,
        -1,
        "123",
        "12/../34?x=1",
        "..",
        "/",
        "?",
    ):
        result = await handle_linode_profile_login_get(
            {"login_id": login_id}, sample_config
        )

        assert len(result) == 1
        assert "login_id" in result[0].text


async def test_handle_linode_profile_login_get_success(
    sample_config: Config,
) -> None:
    """Profile login get calls the retryable client and returns login details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.get_profile_login.return_value = {
            "id": 12345,
            "ip": "192.0.2.10",
            "datetime": "2024-01-02T03:04:05",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_get(
            {"login_id": 12345}, sample_config
        )

    assert json.loads(result[0].text) == {
        "id": 12345,
        "ip": "192.0.2.10",
        "datetime": "2024-01-02T03:04:05",
    }
    mock_client.get_profile_login.assert_awaited_once_with(12345)


async def test_handle_linode_profile_login_get_error(
    sample_config: Config,
) -> None:
    """Profile login get surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.get_profile_login.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_get(
            {"login_id": 12345}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_token_revoke_tool() -> None:
    """Profile token revoke tool exposes token_id and confirm."""
    tool, capability = create_linode_profile_token_revoke_tool()

    assert tool.name == "linode_profile_token_revoke"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["token_id", "confirm"]
    assert tool.inputSchema["properties"]["token_id"]["minimum"] == 1


async def test_handle_linode_profile_token_revoke_requires_token_id(
    sample_config: Config,
) -> None:
    """Profile token revoke validates token_id before calling the client."""
    for token_id in (
        None,
        True,
        False,
        0,
        -1,
        "123",
        "12/../34?x=1",
        "..",
        "/",
        "?",
    ):
        result = await handle_linode_profile_token_revoke(
            {"token_id": token_id, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "token_id" in result[0].text


async def test_handle_linode_profile_token_revoke_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile token revoke requires explicit confirmation."""
    result = await handle_linode_profile_token_revoke(
        {"token_id": 12345}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_profile_token_revoke_success(
    sample_config: Config,
) -> None:
    """Profile token revoke calls the retryable client and returns success."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_revoke(
            {"token_id": 12345, "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "message": "Profile token 12345 revoked successfully"
    }
    mock_client.delete_profile_token.assert_awaited_once_with(12345)


def test_create_linode_profile_token_update_tool() -> None:
    """Profile token update tool exposes token_id and label."""
    tool, capability = create_linode_profile_token_update_tool()

    assert tool.name == "linode_profile_token_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["token_id", "label", "confirm"]
    assert tool.inputSchema["properties"]["token_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["label"]["maxLength"] == 100


async def test_handle_linode_profile_token_update_requires_token_id(
    sample_config: Config,
) -> None:
    """Profile token update validates token_id before calling the client."""
    for token_id in (
        None,
        True,
        False,
        0,
        -1,
        "123",
        "12/../34?x=1",
        "..",
        "/",
        "?",
    ):
        result = await handle_linode_profile_token_update(
            {"token_id": token_id, "label": "new-label", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "token_id" in result[0].text


async def test_handle_linode_profile_token_update_requires_label(
    sample_config: Config,
) -> None:
    """Profile token update validates label before calling the client."""
    for label in (None, "", "   ", 123, "x" * 101):
        result = await handle_linode_profile_token_update(
            {"token_id": 12345, "label": label, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "label" in result[0].text


async def test_handle_linode_profile_token_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Profile token update requires explicit confirmation."""
    result = await handle_linode_profile_token_update(
        {"token_id": 12345, "label": "new-label"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_profile_token_update_success(
    sample_config: Config,
) -> None:
    """Profile token update calls the retryable client and returns token details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.update_profile_token.return_value = {
            "id": 12345,
            "label": "new-label",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_update(
            {"token_id": 12345, "label": "new-label", "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {"id": 12345, "label": "new-label"}
    mock_client.update_profile_token.assert_awaited_once_with(12345, label="new-label")


async def test_handle_linode_profile_token_update_error(
    sample_config: Config,
) -> None:
    """Profile token update surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.update_profile_token.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_update(
            {"token_id": 12345, "label": "new-label", "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_devices_list_tool() -> None:
    """Profile trusted device list tool exposes only environment arguments."""
    tool, capability = create_linode_profile_devices_list_tool()

    assert tool.name == "linode_profile_devices_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema
    assert "environment" in tool.inputSchema["properties"]


async def test_handle_linode_profile_devices_list_success(
    sample_config: Config,
) -> None:
    """Profile trusted device list calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_devices.return_value = [
            {"id": 123, "user_agent": "Mozilla/5.0"},
            {"id": 456, "user_agent": "curl/8.0"},
        ]
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_devices_list({}, sample_config)

    assert json.loads(result[0].text) == {
        "devices": [
            {"id": 123, "user_agent": "Mozilla/5.0"},
            {"id": 456, "user_agent": "curl/8.0"},
        ]
    }
    mock_client.list_profile_devices.assert_awaited_once_with()


async def test_handle_linode_profile_devices_list_error(
    sample_config: Config,
) -> None:
    """Profile trusted device list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.list_profile_devices.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_devices_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_apps_list_tool() -> None:
    tool, capability = create_linode_profile_apps_list_tool()

    assert tool.name == "linode_profile_apps_list"
    assert capability is Capability.Read
    assert "required" not in tool.inputSchema
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


def test_linode_profile_apps_list_tool_is_exported_and_registered() -> None:
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_profile_apps_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_apps_list" in tools_mod.__all__
    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_profile_apps_list"].capability is Capability.Read


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": True}, "page must be an integer"),
        ({"page": "1"}, "page must be an integer"),
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": True}, "page_size must be an integer"),
        ({"page_size": "50"}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_profile_apps_list_rejects_invalid_pagination(
    arguments: dict[str, object], message: str, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_apps_list(arguments, sample_config)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_apps_list_success(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_profile_apps.return_value = {
            "data": [{"id": 123, "label": "authorized-app"}],
            "page": 2,
            "pages": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_apps_list(
            {"page": 2, "page_size": 50}, sample_config
        )

    assert json.loads(result[0].text) == {
        "data": [{"id": 123, "label": "authorized-app"}],
        "page": 2,
        "pages": 3,
    }
    mock_client.list_profile_apps.assert_awaited_once_with(page=2, page_size=50)


async def test_handle_linode_profile_apps_list_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_profile_apps.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_apps_list({}, sample_config)

    assert "Failed to list Linode profile OAuth app authorizations" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_app_get_tool() -> None:
    tool, capability = create_linode_profile_app_get_tool()

    assert tool.name == "linode_profile_app_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["app_id"]
    assert tool.inputSchema["properties"]["app_id"]["minimum"] == 1


def test_linode_profile_app_get_tool_is_exported_and_registered() -> None:
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_profile_app_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_app_get" in tools_mod.__all__
    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_profile_app_get"].capability is Capability.Read


@pytest.mark.parametrize(
    "app_id", [None, 0, -1, True, "123", "/", "?", "..", "12/../34?x=1"]
)
async def test_handle_linode_profile_app_get_requires_positive_integer_app_id(
    app_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_app_get({"app_id": app_id}, sample_config)

    assert "app_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_app_get_success(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_app.return_value = {
            "id": 123,
            "label": "authorized-app",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_get({"app_id": 123}, sample_config)

    assert json.loads(result[0].text) == {"id": 123, "label": "authorized-app"}
    mock_client.get_profile_app.assert_awaited_once_with(123)


async def test_handle_linode_profile_app_get_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_app.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_get({"app_id": 123}, sample_config)

    assert "Failed to retrieve Linode profile OAuth app authorization" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_app_revoke_tool() -> None:
    tool, capability = create_linode_profile_app_revoke_tool()

    assert tool.name == "linode_profile_app_revoke"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["app_id", "confirm"]
    assert tool.inputSchema["properties"]["app_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


def test_linode_profile_app_revoke_tool_is_exported_and_registered() -> None:
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_profile_app_revoke_tool" in tools_mod.__all__
    assert "handle_linode_profile_app_revoke" in tools_mod.__all__
    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_profile_app_revoke"].capability is Capability.Destroy


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_profile_app_revoke_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"app_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_app_revoke(arguments, sample_config)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "app_id", [None, 0, -1, True, "123", "/", "?", "..", "12/../34?x=1"]
)
async def test_handle_linode_profile_app_revoke_requires_positive_integer_app_id(
    app_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_app_revoke(
            {"app_id": app_id, "confirm": True}, sample_config
        )

    assert "app_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_app_revoke_success(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_profile_app.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_revoke(
            {"app_id": 123, "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "message": "Profile app 123 revoked successfully"
    }
    mock_client.delete_profile_app.assert_awaited_once_with(123)


async def test_handle_linode_profile_app_revoke_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_profile_app.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_revoke(
            {"app_id": 123, "confirm": True}, sample_config
        )

    assert "Failed to revoke Linode profile OAuth app access" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_device_get_tool() -> None:
    tool, capability = create_linode_profile_device_get_tool()

    assert tool.name == "linode_profile_device_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["device_id"]
    assert tool.inputSchema["properties"]["device_id"]["minimum"] == 1


@pytest.mark.parametrize("device_id", [None, 0, -1, True, "123", "/", "?", ".."])
async def test_handle_linode_profile_device_get_requires_positive_integer_device_id(
    device_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_device_get(
            {"device_id": device_id}, sample_config
        )

    assert "device_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_device_get_success(
    sample_config: Config,
) -> None:
    device = {
        "id": 123,
        "created": "2018-01-01T01:01:01",
        "expiry": "2018-01-31T01:01:01",
        "last_authenticated": "2018-01-05T12:57:12",
        "last_remote_addr": "203.0.113.1",
        "user_agent": "Mozilla/5.0",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_device.return_value = device
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_get(
            {"device_id": 123}, sample_config
        )

    assert json.loads(result[0].text) == device
    mock_client.get_profile_device.assert_awaited_once_with(123)


async def test_handle_linode_profile_device_get_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile_device.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_get(
            {"device_id": 123}, sample_config
        )

    assert "Failed to retrieve Linode profile trusted device" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_device_revoke_tool() -> None:
    tool, capability = create_linode_profile_device_revoke_tool()

    assert tool.name == "linode_profile_device_revoke"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["device_id", "confirm"]
    assert tool.inputSchema["properties"]["device_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_profile_device_revoke_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"device_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_device_revoke(arguments, sample_config)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("device_id", [None, 0, -1, True, "123", "/", "?", ".."])
async def test_handle_linode_profile_device_revoke_requires_positive_integer_device_id(
    device_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_device_revoke(
            {"device_id": device_id, "confirm": True}, sample_config
        )

    assert "device_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_device_revoke_success(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_profile_device.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_revoke(
            {"device_id": 123, "confirm": True}, sample_config
        )

    assert "Profile trusted device 123 revoked successfully" in result[0].text
    mock_client.delete_profile_device.assert_awaited_once_with(123)


async def test_handle_linode_profile_device_revoke_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_profile_device.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_revoke(
            {"device_id": 123, "confirm": True}, sample_config
        )

    assert "Failed to revoke Linode profile trusted device" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_groups_list_tool() -> None:
    """Placement groups list tool schema supports optional pagination."""
    tool, capability = create_linode_placement_groups_list_tool()

    assert tool.name == "linode_placement_groups_list"
    assert capability is Capability.Read
    assert "page" not in tool.inputSchema.get("required", [])
    assert "page_size" not in tool.inputSchema.get("required", [])
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


def test_linode_placement_groups_list_tool_is_exported_and_registered() -> None:
    """Placement groups list tool is exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_placement_groups_list_tool" in tools_mod.__all__
    assert "handle_linode_placement_groups_list" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_groups_list"].capability is Capability.Read


@pytest.mark.parametrize(
    ("arguments", "error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": True}, "page must be an integer"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": False}, "page_size must be an integer"),
        ({"page_size": "25"}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_placement_groups_list_rejects_invalid_pagination(
    arguments: dict[str, Any], error: str, sample_config: Config
) -> None:
    """Placement groups list validates pagination arguments."""
    result = await handle_linode_placement_groups_list(arguments, sample_config)

    assert len(result) == 1
    assert error in result[0].text


async def test_handle_linode_placement_groups_list_success(
    sample_config: Config,
) -> None:
    """Placement groups list handler returns client response."""
    response_data: dict[str, Any] = {
        "data": [{"id": 123, "label": "pg-a"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_placement_groups.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_groups_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.list_placement_groups.assert_awaited_once_with(page=2, page_size=25)


async def test_handle_linode_placement_groups_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Placement groups list handler reports client exceptions."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_placement_groups.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_groups_list({}, sample_config)

    assert len(result) == 1
    assert "API error" in result[0].text


def test_create_linode_placement_group_get_tool() -> None:
    """Placement group get tool schema requires only group_id."""
    tool, capability = create_linode_placement_group_get_tool()

    assert tool.name == "linode_placement_group_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["group_id"]
    assert tool.inputSchema["properties"]["group_id"]["minimum"] == 1


@pytest.mark.parametrize("group_id", [None, 0, -1, True, "789", "/", "?", ".."])
async def test_handle_linode_placement_group_get_requires_positive_group_id(
    group_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_get(
            {"group_id": group_id}, sample_config
        )

    assert "group_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_get_success(
    sample_config: Config,
) -> None:
    response_data = {"id": 789, "label": "pg-a"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_get(
            {"group_id": 789},
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_placement_group.assert_awaited_once_with(789)


async def test_handle_linode_placement_group_get_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_get(
            {"group_id": 789},
            sample_config,
        )

    assert "Failed to get placement group" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_create_tool() -> None:
    """Placement group create tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_create_tool()

    assert tool.name == "linode_placement_group_create"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "label",
        "region",
        "placement_group_type",
        "placement_group_policy",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["label"]["minLength"] == 1
    assert tool.inputSchema["properties"]["region"]["minLength"] == 1
    assert tool.inputSchema["properties"]["placement_group_type"]["enum"] == [
        "anti_affinity:local"
    ]
    assert tool.inputSchema["properties"]["placement_group_policy"]["enum"] == [
        "strict",
        "flexible",
    ]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_placement_group_create_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {
        "label": "pg-a",
        "region": "us-mia",
        "placement_group_type": "anti_affinity:local",
        "placement_group_policy": "strict",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_create(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "label",
    [None, "", True, 1, [], {}, "/", "?", "..", "bad/label", "bad?label"],
)
async def test_handle_linode_placement_group_create_requires_valid_label(
    label: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_create(
            {
                "label": label,
                "region": "us-mia",
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": "strict",
                "confirm": True,
            },
            sample_config,
        )

    assert "label must start and end" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("region", [None, "", True, 1, [], {}])
async def test_handle_linode_placement_group_create_requires_valid_region(
    region: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_create(
            {
                "label": "pg-a",
                "region": region,
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": "strict",
                "confirm": True,
            },
            sample_config,
        )

    assert "region must be a non-empty string" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "placement_group_type", [None, "", "affinity:local", "anti-affinity:local", 1]
)
async def test_handle_linode_placement_group_create_requires_valid_type(
    placement_group_type: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_create(
            {
                "label": "pg-a",
                "region": "us-mia",
                "placement_group_type": placement_group_type,
                "placement_group_policy": "strict",
                "confirm": True,
            },
            sample_config,
        )

    assert "placement_group_type must be anti_affinity:local" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "placement_group_policy", [None, "", "best-effort", "STRICT", 1]
)
async def test_handle_linode_placement_group_create_requires_valid_policy(
    placement_group_policy: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_create(
            {
                "label": "pg-a",
                "region": "us-mia",
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": placement_group_policy,
                "confirm": True,
            },
            sample_config,
        )

    assert "placement_group_policy must be strict or flexible" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_create_success(
    sample_config: Config,
) -> None:
    response_data = {"id": 789, "label": "pg-a"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_placement_group.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_create(
            {
                "label": "pg-a",
                "region": "us-mia",
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": "strict",
                "confirm": True,
            },
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.create_placement_group.assert_awaited_once_with(
        "pg-a", "us-mia", "anti_affinity:local", "strict"
    )


async def test_handle_linode_placement_group_create_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_create(
            {
                "label": "pg-a",
                "region": "us-mia",
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": "strict",
                "confirm": True,
            },
            sample_config,
        )

    assert "Failed to create placement group" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_delete_tool() -> None:
    """Placement group delete tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_delete_tool()

    assert tool.name == "linode_placement_group_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["group_id", "confirm"]
    assert tool.inputSchema["properties"]["group_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_placement_group_delete_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"group_id": 789}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_delete(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("group_id", [None, 0, -1, True, "789", "/", "?", ".."])
async def test_handle_linode_placement_group_delete_requires_positive_group_id(
    group_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_delete(
            {"group_id": group_id, "confirm": True}, sample_config
        )

    assert "group_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_delete_success(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_placement_group.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_delete(
            {"group_id": 789, "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == {
        "message": "Placement group 789 deleted successfully"
    }
    mock_client.delete_placement_group.assert_awaited_once_with(789)


async def test_handle_linode_placement_group_delete_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_delete(
            {"group_id": 789, "confirm": True},
            sample_config,
        )

    assert "Failed to delete placement group" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_update_tool() -> None:
    """Placement group update tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_update_tool()

    assert tool.name == "linode_placement_group_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["group_id", "label", "confirm"]
    assert tool.inputSchema["properties"]["group_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["label"]["minLength"] == 1
    assert "pattern" in tool.inputSchema["properties"]["label"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_placement_group_update_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"group_id": 789, "label": "new-label"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_update(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("group_id", [None, 0, -1, True, "789", "/", "?", ".."])
async def test_handle_linode_placement_group_update_requires_positive_group_id(
    group_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_update(
            {"group_id": group_id, "label": "new-label", "confirm": True},
            sample_config,
        )

    assert "group_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "label",
    [None, "", True, 1, [], {}, "/", "?", "..", "bad/label", "bad?label"],
)
async def test_handle_linode_placement_group_update_requires_valid_label(
    label: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_update(
            {"group_id": 789, "label": label, "confirm": True},
            sample_config,
        )

    assert "label must start and end" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_update_success(
    sample_config: Config,
) -> None:
    response_data = {"id": 789, "label": "new-label"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_placement_group.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_update(
            {"group_id": 789, "label": "new-label", "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.update_placement_group.assert_awaited_once_with(789, "new-label")


async def test_handle_linode_placement_group_update_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_update(
            {"group_id": 789, "label": "new-label", "confirm": True},
            sample_config,
        )

    assert "Failed to update placement group" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_assign_tool() -> None:
    """Placement group assign tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_assign_tool()

    assert tool.name == "linode_placement_group_assign"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["group_id", "linodes", "confirm"]
    assert tool.inputSchema["properties"]["group_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["linodes"]["minItems"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_placement_group_assign_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"group_id": 789, "linodes": [123]}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_assign(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("group_id", [None, 0, -1, True, "789", "/", "?", ".."])
async def test_handle_linode_placement_group_assign_requires_positive_group_id(
    group_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_assign(
            {"group_id": group_id, "linodes": [123], "confirm": True}, sample_config
        )

    assert "group_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "linodes", [None, [], [0], [-1], [True], ["123"], "/", "?", ".."]
)
async def test_handle_linode_placement_group_assign_requires_linode_ids(
    linodes: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_assign(
            {"group_id": 789, "linodes": linodes, "confirm": True}, sample_config
        )

    assert "linodes must be a non-empty array of positive integers" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_assign_success(
    sample_config: Config,
) -> None:
    response_data = {"linodes": [123, 456]}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.assign_placement_group.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_assign(
            {"group_id": 789, "linodes": [123, 456], "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.assign_placement_group.assert_awaited_once_with(789, [123, 456])


async def test_handle_linode_placement_group_assign_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.assign_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_assign(
            {"group_id": 789, "linodes": [123], "confirm": True},
            sample_config,
        )

    assert "Failed to assign Linodes to placement group" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_unassign_tool() -> None:
    """Placement group unassign tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_unassign_tool()

    assert tool.name == "linode_placement_group_unassign"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["group_id", "linodes", "confirm"]
    assert tool.inputSchema["properties"]["group_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["linodes"]["minItems"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_placement_group_unassign_requires_boolean_confirm(
    confirm: object, sample_config: Config
) -> None:
    arguments: dict[str, object] = {"group_id": 789, "linodes": [123]}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_unassign(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("group_id", [None, 0, -1, True, "789", "/", "?", ".."])
async def test_handle_linode_placement_group_unassign_requires_positive_group_id(
    group_id: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_unassign(
            {"group_id": group_id, "linodes": [123], "confirm": True}, sample_config
        )

    assert "group_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "linodes", [None, [], [0], [-1], [True], ["123"], "/", "?", ".."]
)
async def test_handle_linode_placement_group_unassign_requires_linode_ids(
    linodes: object, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": linodes, "confirm": True}, sample_config
        )

    assert "linodes must be a non-empty array of positive integers" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_unassign_success(
    sample_config: Config,
) -> None:
    response_data = {"linodes": [123, 456]}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.unassign_placement_group.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": [123, 456], "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == response_data
    mock_client.unassign_placement_group.assert_awaited_once_with(789, [123, 456])


async def test_handle_linode_placement_group_unassign_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.unassign_placement_group.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": [123], "confirm": True},
            sample_config,
        )

    assert "Failed to unassign Linodes from placement group" in result[0].text
    assert "API error" in result[0].text


async def test_create_linode_nodebalancer_stats_tool_definition() -> None:
    """Test linode_nodebalancer_stats tool definition."""
    tool, capability = create_linode_nodebalancer_stats_tool()
    assert tool.name == "linode_nodebalancer_stats"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_stats(sample_config: Config) -> None:
    """Test linode_nodebalancer_stats tool."""
    mock_stats = {
        "data": {
            "connections": [[1526391300000, 0]],
            "traffic": {
                "in": [[1526391300000, 631.21]],
                "out": [[1526391300000, 103.44]],
            },
        },
        "title": "linode.com - balancer12345 (12345) - day (5 min avg)",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_stats.return_value = mock_stats
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_stats(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        content = result[0].text
        assert "connections" in content
        assert "traffic" in content
        mock_client.get_nodebalancer_stats.assert_called_once_with(1)


async def test_handle_linode_nodebalancer_stats_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_stats tool with missing ID."""
    result = await handle_linode_nodebalancer_stats({}, sample_config)
    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_nodebalancer_stats_error(sample_config: Config) -> None:
    """Test linode_nodebalancer_stats tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_stats.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_stats(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_firewalls_list_tool_definition() -> None:
    """Test linode_nodebalancer_firewalls_list tool definition."""
    tool, capability = create_linode_nodebalancer_firewalls_list_tool()

    assert tool.name == "linode_nodebalancer_firewalls_list"
    assert capability == Capability.Read
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_firewalls_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewalls_list tool."""
    mock_firewalls: dict[str, Any] = {
        "data": [
            {
                "id": 123,
                "label": "web-fw",
                "status": "enabled",
                "rules": {"inbound": [], "outbound": []},
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewalls_list(
            {"nodebalancer_id": 8, "page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["data"][0]["id"] == 123
        assert data["results"] == 1
        mock_client.list_nodebalancer_firewalls.assert_called_once_with(
            8, page=1, page_size=25
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id must be a positive integer"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        ({"nodebalancer_id": 8, "page": 0}, "page must be at least 1"),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        ({"nodebalancer_id": 8, "page_size": 24}, "page_size must be at least 25"),
        ({"nodebalancer_id": 8, "page_size": 501}, "page_size must be at most 500"),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_firewalls_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer firewall list rejects invalid arguments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_firewalls_list(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_linode_nodebalancer_config_node_get_tool_definition() -> None:
    """Test linode_nodebalancer_config_node_get tool definition."""
    tool, _capability = create_linode_nodebalancer_config_node_get_tool()
    assert tool.name == "linode_nodebalancer_config_node_get"
    assert "nodebalancer_id" in tool.inputSchema["properties"]
    assert "config_id" in tool.inputSchema["properties"]
    assert "node_id" in tool.inputSchema["properties"]
    assert set(tool.inputSchema["required"]) == {
        "nodebalancer_id",
        "config_id",
        "node_id",
    }


async def test_handle_linode_nodebalancer_config_node_get(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_get tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_cls.return_value.__aenter__.return_value = mock_client
        mock_client.get_nodebalancer_config_node.return_value = {
            "id": 4,
            "label": "node-1",
            "address": "192.168.1.10:80",
            "weight": 100,
            "mode": "accept",
        }

        result = await handle_linode_nodebalancer_config_node_get(
            {"nodebalancer_id": 8, "config_id": 6, "node_id": 4},
            sample_config,
        )

        mock_client.get_nodebalancer_config_node.assert_called_once_with(8, 6, 4)
        response = result[0].text
        assert '"id": 4' in response


async def test_handle_linode_nodebalancer_config_node_get_invalid_arguments(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_get with invalid arguments."""
    for args in [
        {},
        {"nodebalancer_id": 8, "config_id": 6},
        {"nodebalancer_id": 8, "node_id": 4},
        {"config_id": 6, "node_id": 4},
        {"nodebalancer_id": -1, "config_id": 6, "node_id": 4},
    ]:
        result = await handle_linode_nodebalancer_config_node_get(args, sample_config)
        response = result[0].text
        assert "error" in response.lower()


async def test_handle_linode_nodebalancer_config_node_get_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_cls.return_value.__aenter__.return_value = mock_client
        mock_client.get_nodebalancer_config_node.side_effect = Exception("API error")

        result = await handle_linode_nodebalancer_config_node_get(
            {"nodebalancer_id": 8, "config_id": 6, "node_id": 4},
            sample_config,
        )
        response = result[0].text
        assert "error" in response.lower()


async def test_handle_linode_nodebalancer_firewalls_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewalls_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancer_firewalls.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewalls_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


def test_linode_nodebalancer_config_update_tool_definition() -> None:
    """Test linode_nodebalancer_config_update tool definition."""
    tool, capability = create_linode_nodebalancer_config_update_tool()

    assert tool.name == "linode_nodebalancer_config_update"
    assert capability == Capability.Write
    assert tool.inputSchema["properties"]["nodebalancer_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["required"] == [
        "nodebalancer_id",
        "config_id",
        "confirm",
    ]


async def test_handle_linode_nodebalancer_config_update(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_update tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config.return_value = {
            "id": 6,
            "nodebalancer_id": 8,
            "port": 443,
            "protocol": "https",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_update(
            {
                "nodebalancer_id": 8,
                "config_id": 6,
                "port": 443,
                "protocol": "https",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == 6
        assert data["port"] == 443
        mock_client.update_nodebalancer_config.assert_called_once_with(
            8, 6, {"port": 443, "protocol": "https"}
        )


async def test_handle_linode_nodebalancer_config_update_empty_response(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_update formats an empty response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_update(
            {"nodebalancer_id": 8, "config_id": 6, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["nodebalancer_id"] == 8
        assert data["config_id"] == 6
        assert "update requested" in data["message"]
        mock_client.update_nodebalancer_config.assert_called_once_with(8, 6, {})


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "confirm must be true"),
        ({"confirm": False}, "confirm must be true"),
        ({"confirm": "true"}, "confirm must be true"),
        ({"confirm": 1}, "confirm must be true"),
        (
            {"nodebalancer_id": 0, "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "8", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": True, "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1/2", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "1?x", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": "..", "config_id": 6, "confirm": True},
            "nodebalancer_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": 0, "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "6", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": False, "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "4/5", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "4?x", "confirm": True},
            "config_id",
        ),
        (
            {"nodebalancer_id": 8, "config_id": "..", "confirm": True},
            "config_id",
        ),
    ],
)
async def test_handle_linode_nodebalancer_config_update_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer config update rejects invalid arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_config_update(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_config_update_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_update error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_update(
            {"nodebalancer_id": 8, "config_id": 6, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_config_create_tool_definition() -> None:
    """Test linode_nodebalancer_config_create tool definition."""
    tool, capability = create_linode_nodebalancer_config_create_tool()
    assert tool.name == "linode_nodebalancer_config_create"
    assert capability == Capability.Write
    assert "nodebalancer_id" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["nodebalancer_id", "confirm"]
    # Verify optional body fields are present
    props = tool.inputSchema["properties"]
    assert "port" in props
    assert "protocol" in props
    assert "algorithm" in props
    assert "stickiness" in props
    assert "check" in props
    assert "nodes" in props


async def test_handle_linode_nodebalancer_config_create(sample_config: Config) -> None:
    """Test linode_nodebalancer_config_create tool happy path."""
    mock_result = {
        "id": 99,
        "nodebalancer_id": 8,
        "port": 80,
        "protocol": "http",
        "algorithm": "roundrobin",
        "stickiness": "none",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer_config.return_value = mock_result
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_create(
            {
                "nodebalancer_id": 8,
                "port": 80,
                "protocol": "http",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == mock_result
        mock_client.create_nodebalancer_config.assert_called_once_with(
            8, {"port": 80, "protocol": "http"}
        )


async def test_handle_linode_nodebalancer_config_create_confirm_required(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_create rejects missing/false confirm."""
    invalid_confirm: list[dict[str, Any]] = [
        {},  # missing
        {"nodebalancer_id": 8, "confirm": False},
        {"nodebalancer_id": 8, "confirm": "true"},
        {"nodebalancer_id": 8, "confirm": 1},
    ]

    for args in invalid_confirm:
        result = await handle_linode_nodebalancer_config_create(args, sample_config)
        assert len(result) == 1
        assert "confirm must be true" in result[0].text


async def test_handle_linode_nodebalancer_config_create_invalid_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_create rejects invalid nodebalancer_id."""
    invalid_cases: list[tuple[dict[str, Any], str]] = [
        ({"confirm": True}, "nodebalancer_id must be a positive integer"),
        (
            {"nodebalancer_id": True, "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": 0, "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": -1, "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": "8/9", "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
        (
            {"nodebalancer_id": "../8", "confirm": True},
            "nodebalancer_id must be a positive integer",
        ),
    ]

    for args, message in invalid_cases:
        result = await handle_linode_nodebalancer_config_create(args, sample_config)
        assert len(result) == 1
        assert message in result[0].text


async def test_handle_linode_nodebalancer_config_create_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_create error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer_config.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_create(
            {"nodebalancer_id": 8, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_firewall_rule_version_get(
    sample_config: Config,
) -> None:
    """Test the firewall rule version get tool handler."""
    from linodemcp.linode import FirewallAddresses, FirewallRule
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_rule_version_get

    mock_rule = FirewallRule(
        action="ACCEPT",
        protocol="TCP",
        ports="22",
        addresses=FirewallAddresses(ipv4=["0.0.0.0/0"], ipv6=["::/0"]),
        label="allow-ssh",
        description="Allow SSH traffic",
    )

    async def mock_execute_tool(
        cfg: Any, arguments: Any, description: Any, call_fn: Any
    ) -> Any:
        mock_client = MagicMock()
        mock_client.get_firewall_rule_version = AsyncMock(return_value=mock_rule)
        rule_data = await call_fn(mock_client)
        return [TextContent(type="text", text=json.dumps(rule_data))]

    with patch(
        "linodemcp.tools.linode_firewalls.execute_tool", side_effect=mock_execute_tool
    ):
        result = await handle_linode_firewall_rule_version_get(
            {"firewall_id": 12345, "version": "v1"}, sample_config
        )
        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["action"] == "ACCEPT"
        assert data["label"] == "allow-ssh"


async def test_handle_linode_firewall_rule_version_get_missing_args(
    sample_config: Config,
) -> None:
    """Test the firewall rule version get tool rejects missing arguments."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_rule_version_get

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": 12345}, sample_config
    )
    assert len(result) == 1
    assert "version is required" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"version": "v1"}, sample_config
    )
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    result = await handle_linode_firewall_rule_version_get({}, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    # Invalid firewall_id types
    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": True, "version": "v1"}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text or "valid integer" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": -1, "version": "v1"}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": 0, "version": "v1"}, sample_config
    )
    assert len(result) == 1
    # 0 is falsy, caught by the "required" check
    assert "required" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": "abc", "version": "v1"}, sample_config
    )
    assert len(result) == 1
    assert "valid integer" in result[0].text


def test_create_linode_firewall_template_get_tool_schema() -> None:
    """Test linode_firewall_template_get tool schema."""
    tool, capability = create_linode_firewall_template_get_tool()

    assert tool.name == "linode_firewall_template_get"
    assert capability is Capability.Read
    assert "slug" in tool.inputSchema["properties"]
    assert "slug" in tool.inputSchema["required"]
    assert "page" in tool.inputSchema["properties"]
    assert "page_size" in tool.inputSchema["properties"]


async def test_handle_linode_firewall_template_get(sample_config: Config) -> None:
    """Test linode_firewall_template_get tool."""

    mock_template = FirewallTemplate(
        slug="allow-http",
        label="Allow HTTP",
        description="Allow HTTP traffic on port 80",
        rules=FirewallRules(
            inbound=[],
            outbound=[],
            inbound_policy="DROP",
            outbound_policy="ACCEPT",
        ),
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_firewall_template.return_value = mock_template
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_template_get(
            {"slug": "allow-http"}, sample_config
        )

        assert len(result) == 1
        assert "allow-http" in result[0].text
        mock_client.get_firewall_template.assert_awaited_once_with(
            "allow-http", None, None
        )


async def test_handle_linode_firewall_template_get_missing_slug(
    sample_config: Config,
) -> None:
    """Test linode_firewall_template_get validation."""
    result = await handle_linode_firewall_template_get({}, sample_config)

    assert len(result) == 1
    assert "slug is required" in result[0].text


async def test_handle_linode_firewall_template_get_rejects_path_traversal(
    sample_config: Config,
) -> None:
    """Test that path traversal characters in slug are rejected."""
    # Test with /
    result = await handle_linode_firewall_template_get(
        {"slug": "allow/http"}, sample_config
    )
    assert len(result) == 1
    assert "path separators" in result[0].text

    # Test with ?
    result = await handle_linode_firewall_template_get(
        {"slug": "allow?http"}, sample_config
    )
    assert len(result) == 1
    assert "path separators" in result[0].text

    # Test with ..
    result = await handle_linode_firewall_template_get(
        {"slug": "allow/../http"}, sample_config
    )
    assert len(result) == 1
    assert "path separators" in result[0].text


async def test_handle_linode_firewall_template_get_rejects_non_string_slug(
    sample_config: Config,
) -> None:
    """Test that non-string slug values are rejected."""
    # Test with int
    result = await handle_linode_firewall_template_get({"slug": 123}, sample_config)
    assert len(result) == 1
    assert "must be a string" in result[0].text

    # Test with bool
    result = await handle_linode_firewall_template_get({"slug": True}, sample_config)
    assert len(result) == 1
    assert "must be a string" in result[0].text


async def test_handle_linode_firewall_template_get_rejects_invalid_pagination(
    sample_config: Config,
) -> None:
    """Test that invalid pagination parameters are rejected."""
    # Test with negative page
    result = await handle_linode_firewall_template_get(
        {"slug": "allow-http", "page": -1}, sample_config
    )
    assert len(result) == 1
    assert "page must be a positive integer" in result[0].text

    # Test with zero page_size
    result = await handle_linode_firewall_template_get(
        {"slug": "allow-http", "page_size": 0}, sample_config
    )
    assert len(result) == 1
    assert "page_size must be a positive integer" in result[0].text

    # Test with non-int page
    result = await handle_linode_firewall_template_get(
        {"slug": "allow-http", "page": "abc"}, sample_config
    )
    assert len(result) == 1
    assert "page must be a positive integer" in result[0].text


async def test_handle_linode_firewall_device_get(
    sample_config: Config,
) -> None:
    """Test the firewall device get tool handler."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_device_get

    mock_device = {
        "id": 456,
        "label": "linode-123",
        "type": "linode",
        "created": "2018-01-01T01:01:01",
        "updated": "2018-01-01T01:01:01",
    }

    async def mock_execute_tool(
        cfg: Any, arguments: Any, description: Any, call_fn: Any
    ) -> Any:
        mock_client = MagicMock()
        mock_client.get_firewall_device = AsyncMock(return_value=mock_device)
        device_data = await call_fn(mock_client)
        return [TextContent(type="text", text=json.dumps(device_data))]

    with patch(
        "linodemcp.tools.linode_firewalls.execute_tool", side_effect=mock_execute_tool
    ):
        result = await handle_linode_firewall_device_get(
            {"firewall_id": 12345, "device_id": 456}, sample_config
        )
        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == 456
        assert data["label"] == "linode-123"


async def test_handle_linode_firewall_device_get_missing_args(
    sample_config: Config,
) -> None:
    """Test the firewall device get tool rejects missing arguments."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_device_get

    result = await handle_linode_firewall_device_get(
        {"firewall_id": 12345}, sample_config
    )
    assert len(result) == 1
    assert "device_id is required" in result[0].text

    result = await handle_linode_firewall_device_get({"device_id": 456}, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    result = await handle_linode_firewall_device_get({}, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    # Invalid types
    result = await handle_linode_firewall_device_get(
        {"firewall_id": True, "device_id": 456}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text or "valid integer" in result[0].text

    result = await handle_linode_firewall_device_get(
        {"firewall_id": 12345, "device_id": "abc"}, sample_config
    )
    assert len(result) == 1
    assert "valid integer" in result[0].text


async def test_handle_linode_firewall_device_create(sample_config: Config) -> None:
    """Test successful firewall device creation."""
    with patch(
        "linodemcp.tools.linode_firewalls_write.handle_linode_firewall_device_create",
        new_callable=AsyncMock,
    ) as mock_handle:
        mock_handle.return_value = [
            TextContent(
                type="text",
                text=(
                    '{"message": "Firewall device created successfully", '
                    '"device": {"id": 456}}'
                ),
            )
        ]

        # Import here to avoid circular imports
        from linodemcp.tools.linode_firewalls_write import (
            handle_linode_firewall_device_create,
        )

        arguments = {
            "firewall_id": 12345,
            "id": 123,
            "type": "linode",
            "confirm": True,
        }
        result = await handle_linode_firewall_device_create(arguments, sample_config)

        assert len(result) == 1
        assert "Firewall device created successfully" in result[0].text
        mock_handle.assert_awaited_once_with(arguments, sample_config)


async def test_handle_linode_firewall_device_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Test that firewall device creation requires confirm=True."""
    # Import here to avoid circular imports
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_create,
    )

    # Test with confirm=False
    arguments = {
        "firewall_id": 12345,
        "id": 123,
        "type": "linode",
        "confirm": False,
    }
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "confirmation" in result[0].text
    assert "confirm=true" in result[0].text

    # Test with missing confirm
    arguments = {"firewall_id": 12345, "id": 123, "type": "linode"}
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "confirmation" in result[0].text
    assert "confirm=true" in result[0].text


async def test_handle_linode_firewall_device_create_missing_required_args(
    sample_config: Config,
) -> None:
    """Test that firewall device creation requires all required arguments."""
    # Import here to avoid circular imports
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_create,
    )

    # Test missing firewall_id
    arguments = {"id": 123, "type": "linode", "confirm": True}
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    # Test missing id
    arguments = {"firewall_id": 12345, "type": "linode", "confirm": True}
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "id is required" in result[0].text

    # Test missing type
    arguments = {"firewall_id": 12345, "id": 123, "confirm": True}
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "type is required" in result[0].text


async def test_handle_linode_firewall_device_create_invalid_args(
    sample_config: Config,
) -> None:
    """Test that firewall device creation validates argument types."""
    # Import here to avoid circular imports
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_create,
    )

    # Test invalid firewall_id
    arguments = {
        "firewall_id": "invalid",
        "id": 123,
        "type": "linode",
        "confirm": True,
    }
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "firewall_id must be a valid integer" in result[0].text

    # Test invalid id
    arguments = {
        "firewall_id": 12345,
        "id": "invalid",
        "type": "linode",
        "confirm": True,
    }
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "id must be a valid integer" in result[0].text

    # Test invalid type
    arguments = {
        "firewall_id": 12345,
        "id": 123,
        "type": 123,
        "confirm": True,
    }
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "type must be a string" in result[0].text

    # Test empty type
    arguments = {
        "firewall_id": 12345,
        "id": 123,
        "type": "",
        "confirm": True,
    }
    result = await handle_linode_firewall_device_create(arguments, sample_config)
    assert len(result) == 1
    assert "type must be a non-empty string" in result[0].text


async def test_handle_linode_firewall_devices_list(sample_config: Config) -> None:
    """Test firewall devices list handler."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_devices_list

    mock_devices = {"data": [{"id": 123}], "page": 1, "pages": 1, "results": 1}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewall_devices.return_value = mock_devices
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_devices_list(
            {"firewall_id": 12345}, sample_config
        )

    assert len(result) == 1
    result_data = json.loads(result[0].text)
    assert result_data["results"] == 1
    mock_client.list_firewall_devices.assert_awaited_once_with(
        12345, page=None, page_size=None
    )


async def test_handle_linode_firewall_devices_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test firewall devices list handler pagination."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_devices_list

    mock_devices: dict[str, Any] = {"data": [], "page": 2, "pages": 5, "results": 0}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewall_devices.return_value = mock_devices
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_devices_list(
            {"firewall_id": 12345, "page": 2, "page_size": 25}, sample_config
        )

    result_data = json.loads(result[0].text)
    assert result_data["page"] == 2
    mock_client.list_firewall_devices.assert_awaited_once_with(
        12345, page=2, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "firewall_id is required"),
        ({"firewall_id": False}, "firewall_id must be a valid integer"),
        ({"firewall_id": "abc"}, "firewall_id must be a valid integer"),
        ({"firewall_id": 0}, "firewall_id must be a positive integer"),
        ({"firewall_id": -1}, "firewall_id must be a positive integer"),
        ({"firewall_id": 1, "page": False}, "page must be a valid integer"),
        ({"firewall_id": 1, "page": "abc"}, "page must be a valid integer"),
        ({"firewall_id": 1, "page": 0}, "page must be a positive integer"),
        (
            {"firewall_id": 1, "page_size": "abc"},
            "page_size must be a valid integer",
        ),
        ({"firewall_id": 1, "page_size": 0}, "page_size must be a positive integer"),
    ],
)
async def test_handle_linode_firewall_devices_list_invalid_args(
    sample_config: Config,
    arguments: dict[str, Any],
    expected: str,
) -> None:
    """Test firewall devices list handler argument validation."""
    from linodemcp.tools.linode_firewalls import handle_linode_firewall_devices_list

    result = await handle_linode_firewall_devices_list(arguments, sample_config)
    assert len(result) == 1
    assert expected in result[0].text


async def test_lke_cluster_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_lke_cluster_create(
        {
            "label": "k8s-prod",
            "region": "us-east",
            "k8s_version": "1.29",
            "node_pools": [{"type": "g6-standard-2", "count": 3}],
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_lke_cluster_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/lke/clusters"
    assert body["current_state"] is None
    assert any("k8s-prod" in s for s in body["side_effects"])
    assert body["warnings"]


async def test_lke_cluster_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """Missing label must error out regardless of dry_run."""
    result = await handle_linode_lke_cluster_create(
        {
            "region": "us-east",
            "k8s_version": "1.29",
            "node_pools": [{"type": "g6-standard-2", "count": 3}],
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_lke_cluster_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the cluster via GET and never calls update."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123, "label": "k8s"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_cluster_update(
            {"cluster_id": "123", "label": "renamed", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_cluster_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/lke/clusters/123"
        assert any("renamed" in s for s in body["side_effects"])
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.update_lke_cluster.assert_not_called()


async def test_lke_cluster_recycle_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the cluster via GET and never recycles."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_cluster_recycle(
            {"cluster_id": "123", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_cluster_recycle"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/lke/clusters/123/recycle"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.recycle_lke_cluster.assert_not_called()


async def test_lke_cluster_regenerate_dry_run_hides_token(
    sample_config: Config,
) -> None:
    """dry_run fetches the cluster, not the rotated service token."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123, "label": "k8s"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_cluster_regenerate(
            {"cluster_id": "123", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        assert "service_token" not in result[0].text
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_cluster_regenerate"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/lke/clusters/123/regenerate"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.regenerate_lke_cluster.assert_not_called()


async def test_lke_pool_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the cluster via GET and never creates a pool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 123}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_pool_create(
            {
                "cluster_id": "123",
                "type": "g6-standard-2",
                "count": 3,
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_pool_create"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/lke/clusters/123/pools"
        mock_client.get_lke_cluster.assert_awaited_once_with(123)
        mock_client.create_lke_node_pool.assert_not_called()


async def test_lke_pool_create_dry_run_still_validates_type(
    sample_config: Config,
) -> None:
    """Missing type must error out regardless of dry_run."""
    result = await handle_linode_lke_pool_create(
        {"cluster_id": "123", "count": 3, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "type is required" in result[0].text


async def test_lke_pool_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the pool via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {"id": 10, "cluster_id": 123}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_pool_update(
            {"cluster_id": "123", "pool_id": "10", "count": 5, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_pool_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/lke/clusters/123/pools/10"
        assert any("5 node" in s for s in body["side_effects"])
        mock_client.get_lke_node_pool.assert_awaited_once_with(123, 10)
        mock_client.update_lke_node_pool.assert_not_called()


async def test_lke_pool_recycle_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the pool via GET and never recycles."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {"id": 10, "cluster_id": 123}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_pool_recycle(
            {"cluster_id": "123", "pool_id": "10", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_pool_recycle"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/lke/clusters/123/pools/10/recycle"
        mock_client.get_lke_node_pool.assert_awaited_once_with(123, 10)
        mock_client.recycle_lke_node_pool.assert_not_called()


async def test_lke_node_recycle_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the node via GET and never recycles."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node.return_value = {"id": "abc-123"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_node_recycle(
            {"cluster_id": "123", "node_id": "abc-123", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_node_recycle"
        assert body["would_execute"]["method"] == "POST"
        assert (
            body["would_execute"]["path"] == "/lke/clusters/123/nodes/abc-123/recycle"
        )
        mock_client.get_lke_node.assert_awaited_once_with(123, "abc-123")
        mock_client.recycle_lke_node.assert_not_called()


async def test_lke_acl_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the ACL via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_control_plane_acl.return_value = {"enabled": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_acl_update(
            {"cluster_id": "123", "acl": {"enabled": True}, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_acl_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/lke/clusters/123/control_plane_acl"
        assert any("enabled" in s for s in body["side_effects"])
        mock_client.get_lke_control_plane_acl.assert_awaited_once_with(123)
        mock_client.update_lke_control_plane_acl.assert_not_called()


async def test_lke_acl_update_dry_run_still_validates_acl(
    sample_config: Config,
) -> None:
    """Missing acl must error out regardless of dry_run."""
    result = await handle_linode_lke_acl_update(
        {"cluster_id": "123", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "acl is required" in result[0].text


async def test_lke_acl_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the ACL via GET and never deletes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_control_plane_acl.return_value = {"enabled": True}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_acl_delete(
            {"cluster_id": "123", "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_acl_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/lke/clusters/123/control_plane_acl"
        mock_client.get_lke_control_plane_acl.assert_awaited_once_with(123)
        mock_client.delete_lke_control_plane_acl.assert_not_called()


async def test_monitor_service_token_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the token create with no resource state."""
    result = await handle_linode_monitor_service_token_create(
        {"service_type": "dbaas", "entity_ids": [1, 2], "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_monitor_service_token_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/monitor/services/dbaas/token"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_monitor_alert_definition_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the alert-definition create with no state."""
    from linodemcp.tools.linode_monitor_write import (
        handle_linode_monitor_service_alert_definition_create,
    )

    result = await handle_linode_monitor_service_alert_definition_create(
        {
            "service_type": "dbaas",
            "label": "high-cpu",
            "severity": 2,
            "rule_criteria": {"rules": [{"metric": "cpu", "operator": "gt"}]},
            "trigger_conditions": {"criteria_condition": "ALL"},
            "channel_ids": [546],
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_monitor_service_alert_definition_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/monitor/services/dbaas/alert-definitions"
    assert body["current_state"] is None


async def test_monitor_alert_definition_delete_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true fetches the definition via GET and never deletes."""
    from linodemcp.tools.linode_monitor_write import (
        handle_linode_monitor_service_alert_definition_delete,
    )

    mock_linode_client.get_monitor_service_alert_definition.return_value = {
        "id": 20000,
        "label": "high-cpu",
    }

    result = await handle_linode_monitor_service_alert_definition_delete(
        {"service_type": "dbaas", "alert_id": 20000, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_monitor_service_alert_definition_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert (
        body["would_execute"]["path"]
        == "/monitor/services/dbaas/alert-definitions/20000"
    )
    mock_linode_client.get_monitor_service_alert_definition.assert_awaited_once_with(
        "dbaas", 20000
    )
    mock_linode_client.delete_monitor_service_alert_definition.assert_not_called()


async def test_monitor_alert_definition_update_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true fetches the definition via GET and never updates."""
    from linodemcp.tools.linode_monitor_write import (
        handle_linode_monitor_alert_definition_update,
    )

    mock_linode_client.get_monitor_service_alert_definition.return_value = {
        "id": 20000,
        "label": "high-cpu",
    }

    result = await handle_linode_monitor_alert_definition_update(
        {
            "service_type": "dbaas",
            "alert_id": 20000,
            "label": "renamed-alert",
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_monitor_alert_definition_update"
    assert body["would_execute"]["method"] == "PUT"
    assert (
        body["would_execute"]["path"]
        == "/monitor/services/dbaas/alert-definitions/20000"
    )
    mock_linode_client.get_monitor_service_alert_definition.assert_awaited_once_with(
        "dbaas", 20000
    )
    mock_linode_client.update_monitor_alert_definition.assert_not_called()


async def test_instance_ip_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the IP via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance_ip.return_value = {"address": "192.0.2.10"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_ip_update(
            {
                "instance_id": 123,
                "address": "192.0.2.10",
                "rdns": "host.example.com",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_instance_ip_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/linode/instances/123/ips/192.0.2.10"
        mock_client.get_instance_ip.assert_awaited_once_with(123, "192.0.2.10")
        mock_client.update_instance_ip.assert_not_called()


async def test_instance_ip_update_dry_run_still_validates_address(
    sample_config: Config,
) -> None:
    """A missing address errors out under dry_run."""
    result = await handle_linode_instance_ip_update(
        {"instance_id": 123, "rdns": "host.example.com", "dry_run": True},
        sample_config,
    )
    assert len(result) == 1
    assert "address is required" in result[0].text


async def test_networking_ip_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the IP via GET and never updates."""
    from linodemcp.tools.linode_instance_ips import (
        handle_linode_networking_ip_update,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_networking_ip.return_value = {"address": "192.0.2.20"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_networking_ip_update(
            {
                "address": "192.0.2.20",
                "rdns": "host.example.com",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_networking_ip_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/networking/ips/192.0.2.20"
        assert any("host.example.com" in s for s in body["side_effects"])
        mock_client.get_networking_ip.assert_awaited_once_with("192.0.2.20")
        mock_client.update_networking_ip.assert_not_called()


async def test_networking_ip_update_dry_run_still_validates_address(
    sample_config: Config,
) -> None:
    """A missing address errors out under dry_run."""
    from linodemcp.tools.linode_instance_ips import (
        handle_linode_networking_ip_update,
    )

    result = await handle_linode_networking_ip_update(
        {"rdns": "host.example.com", "dry_run": True}, sample_config
    )
    assert len(result) == 1
    assert "address is required" in result[0].text


async def test_ipv4_share_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the share POST with no call."""
    from linodemcp.tools.linode_networking import handle_linode_ipv4_share

    result = await handle_linode_ipv4_share(
        {"ips": ["192.0.2.10"], "linode_id": 123, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_ipv4_share"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/ipv4/share"
    assert body["would_execute"]["body"] == {
        "ips": ["192.0.2.10"],
        "linode_id": 123,
    }
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_ipv4_share_dry_run_still_validates_linode_id(
    sample_config: Config,
) -> None:
    """A missing linode_id errors out under dry_run."""
    from linodemcp.tools.linode_networking import handle_linode_ipv4_share

    result = await handle_linode_ipv4_share(
        {"ips": ["192.0.2.10"], "dry_run": True}, sample_config
    )
    assert len(result) == 1
    assert "linode_id is required" in result[0].text


async def test_ipv4_assign_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the assign POST with no call."""
    from linodemcp.tools.linode_networking import handle_linode_ipv4_assign

    result = await handle_linode_ipv4_assign(
        {
            "region": "us-east",
            "assignments": [{"address": "192.0.2.10", "linode_id": 123}],
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_ipv4_assign"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/networking/ipv4/assign"
    assert body["would_execute"]["body"] == {
        "region": "us-east",
        "assignments": [{"address": "192.0.2.10", "linode_id": 123}],
    }
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


async def test_ipv4_assign_dry_run_still_validates_region(
    sample_config: Config,
) -> None:
    """A missing region errors out under dry_run."""
    from linodemcp.tools.linode_networking import handle_linode_ipv4_assign

    result = await handle_linode_ipv4_assign(
        {
            "assignments": [{"address": "192.0.2.10", "linode_id": 123}],
            "dry_run": True,
        },
        sample_config,
    )
    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_instance_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the instance via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = {"id": 123, "label": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_update(
            {"instance_id": 123, "label": "renamed", "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_instance_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/linode/instances/123"
        mock_client.get_instance.assert_awaited_once_with(123)
        mock_client.update_instance.assert_not_called()


async def test_instance_update_dry_run_still_validates_id(
    sample_config: Config,
) -> None:
    """A missing instance_id errors out under dry_run."""
    result = await handle_linode_instance_update(
        {"label": "renamed", "dry_run": True}, sample_config
    )
    assert len(result) == 1
    assert "instance_id is required" in result[0].text


async def test_object_storage_cancel_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the cancel POST with no call."""
    result = await handle_linode_object_storage_cancel({"dry_run": True}, sample_config)

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_object_storage_cancel"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/object-storage/cancel"
    assert body["current_state"] is None
    assert "confirm=true" not in result[0].text


_PG_CREATE_ARGS = {
    "label": "pg-1",
    "region": "us-east",
    "placement_group_type": "anti_affinity:local",
    "placement_group_policy": "strict",
}


async def test_placement_group_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create POST with no call."""
    result = await handle_linode_placement_group_create(
        {**_PG_CREATE_ARGS, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_placement_group_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/placement/groups"
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert "will be created in region" in body["side_effects"][0]


async def test_placement_group_create_dry_run_still_validates_label(
    sample_config: Config,
) -> None:
    """An invalid label errors out under dry_run."""
    result = await handle_linode_placement_group_create(
        {**_PG_CREATE_ARGS, "label": "", "dry_run": True}, sample_config
    )
    assert len(result) == 1
    assert "label" in result[0].text


async def test_placement_group_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the group via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {"id": 7, "label": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_placement_group_update(
            {"group_id": 7, "label": "renamed", "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_placement_group_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/placement/groups/7"
        assert len(body["side_effects"]) == 1
        assert "renamed" in body["side_effects"][0]
        mock_client.get_placement_group.assert_awaited_once_with(7)
        mock_client.update_placement_group.assert_not_called()


async def test_placement_group_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the group via GET, surfaces members, never deletes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {
            "id": 7,
            "members": [{"linode_id": 111}, {"linode_id": 222}],
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_placement_group_delete(
            {"group_id": 7, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_placement_group_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/placement/groups/7"
        assert len(body["dependencies"]) == 2
        assert {d["id"] for d in body["dependencies"]} == {111, 222}
        assert all(d["action"] == "detached" for d in body["dependencies"])
        assert len(body["warnings"]) == 1
        mock_client.get_placement_group.assert_awaited_once_with(7)
        mock_client.delete_placement_group.assert_not_called()


async def test_placement_group_assign_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the group via GET and never assigns."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {"id": 7}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_placement_group_assign(
            {"group_id": 7, "linodes": [123], "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_placement_group_assign"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/placement/groups/7/assign"
        assert len(body["side_effects"]) == 1
        assert "123" in body["side_effects"][0]
        assert "assigned to placement group 7" in body["side_effects"][0]
        mock_client.get_placement_group.assert_awaited_once_with(7)
        mock_client.assign_placement_group.assert_not_called()


async def test_placement_group_unassign_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the group via GET and never unassigns."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {"id": 7}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_placement_group_unassign(
            {"group_id": 7, "linodes": [123], "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_placement_group_unassign"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/placement/groups/7/unassign"
        assert len(body["side_effects"]) == 1
        assert "123" in body["side_effects"][0]
        assert "removed from placement group 7" in body["side_effects"][0]
        mock_client.get_placement_group.assert_awaited_once_with(7)
        mock_client.unassign_placement_group.assert_not_called()


async def test_nb_firewalls_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the NodeBalancer via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = {"id": 8}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_firewalls_update(
            {"nodebalancer_id": 8, "firewall_ids": [1], "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_nodebalancer_firewalls_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/nodebalancers/8/firewalls"
        mock_client.get_nodebalancer.assert_awaited_once_with(8)
        mock_client.update_nodebalancer_firewalls.assert_not_called()


async def test_nb_config_rebuild_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the config via GET and never rebuilds."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.return_value = {"id": 6}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_rebuild(
            {"nodebalancer_id": 8, "config_id": 6, "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_nodebalancer_config_rebuild"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/nodebalancers/8/configs/6/rebuild"
        mock_client.get_nodebalancer_config.assert_awaited_once_with(8, 6)
        mock_client.rebuild_nodebalancer_config.assert_not_called()


async def test_nb_config_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the config via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config.return_value = {"id": 6}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_update(
            {"nodebalancer_id": 8, "config_id": 6, "port": 80, "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_nodebalancer_config_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/nodebalancers/8/configs/6"
        mock_client.get_nodebalancer_config.assert_awaited_once_with(8, 6)
        mock_client.update_nodebalancer_config.assert_not_called()


async def test_nb_config_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the create POST with no call."""
    result = await handle_linode_nodebalancer_config_create(
        {"nodebalancer_id": 8, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_nodebalancer_config_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/nodebalancers/8/configs"
    assert body["current_state"] is None


async def test_nb_config_node_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the node create POST with no call."""
    result = await handle_linode_nodebalancer_config_node_create(
        {"nodebalancer_id": 8, "config_id": 6, "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_nodebalancer_config_node_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/nodebalancers/8/configs/6/nodes"
    assert body["current_state"] is None


async def test_nb_config_node_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the node via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer_config_node.return_value = {"id": 7}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_update(
            {
                "nodebalancer_id": 8,
                "config_id": 6,
                "node_id": 7,
                "label": "renamed",
                "dry_run": True,
            },
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_nodebalancer_config_node_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/nodebalancers/8/configs/6/nodes/7"
        mock_client.get_nodebalancer_config_node.assert_awaited_once_with(8, 6, 7)
        mock_client.update_nodebalancer_config_node.assert_not_called()


async def test_account_tag_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the tag create POST with no call."""
    result = await handle_linode_account_tag_create(
        {"label": "my-tag", "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_account_tag_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/tags"
    assert body["current_state"] is None


async def test_account_tag_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the tag delete DELETE with no call."""
    result = await handle_linode_account_tag_delete(
        {"tag_label": "my-tag", "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_account_tag_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/tags/my-tag"
    assert body["current_state"] is None


async def test_account_support_ticket_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the ticket create POST with no call."""
    result = await handle_linode_account_support_ticket_create(
        {"summary": "S", "description": "D", "dry_run": True}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_account_support_ticket_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/support/tickets"
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert "opened" in body["side_effects"][0]


async def test_account_support_ticket_close_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the ticket via GET and never closes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.return_value = {"id": 42}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_account_support_ticket_close(
            {"ticket_id": 42, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_account_support_ticket_close"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/support/tickets/42/close"
        assert len(body["side_effects"]) == 1
        assert "ticket 42" in body["side_effects"][0]
        mock_client.get_support_ticket.assert_awaited_once_with(42)
        mock_client.close_support_ticket.assert_not_called()


async def test_account_support_ticket_reply_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the ticket via GET and never replies."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.return_value = {"id": 42}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_account_support_ticket_reply_create(
            {"ticket_id": 42, "description": "hi", "dry_run": True},
            sample_config,
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_account_support_ticket_reply_create"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/support/tickets/42/replies"
        assert len(body["side_effects"]) == 1
        assert "ticket 42" in body["side_effects"][0]
        mock_client.get_support_ticket.assert_awaited_once_with(42)
        mock_client.create_support_ticket_reply.assert_not_called()


async def test_account_support_ticket_attachment_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the ticket via GET and never attaches."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.return_value = {"id": 42}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_account_support_ticket_attachment_create(
            {"ticket_id": 42, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_account_support_ticket_attachment_create"
        assert body["would_execute"]["method"] == "POST"
        assert body["would_execute"]["path"] == "/support/tickets/42/attachments"
        assert len(body["side_effects"]) == 1
        assert "ticket 42" in body["side_effects"][0]
        mock_client.get_support_ticket.assert_awaited_once_with(42)
        mock_client.create_support_ticket_attachment.assert_not_called()


def _profile_preview_body(result: list[TextContent]) -> dict[str, Any]:
    """Decode a profile dry-run preview body."""
    assert len(result) == 1
    body: dict[str, Any] = json.loads(result[0].text)
    return body


async def test_profile_preferences_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches preferences via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_profile_preferences.return_value = {"theme": "dark"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_profile_preferences_update(
            {"preferences": {"theme": "light"}, "dry_run": True}, sample_config
        )

        body = _profile_preview_body(result)
        assert body["tool"] == "linode_profile_preferences_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/profile/preferences"
        assert len(body["side_effects"]) == 1
        mock_client.update_profile_preferences.assert_not_called()


async def test_profile_tfa_enable_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the TFA enable POST and never generates a secret."""
    result = await handle_linode_profile_tfa_enable({"dry_run": True}, sample_config)
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_tfa_enable"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/profile/tfa-enable"
    assert body["current_state"] is None
    assert "secret" not in str(body["current_state"])
    assert len(body["side_effects"]) == 1
    assert "confirmed" in body["side_effects"][0]


async def test_profile_tfa_disable_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the TFA disable POST with no call."""
    result = await handle_linode_profile_tfa_disable({"dry_run": True}, sample_config)
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_tfa_disable"
    assert body["would_execute"]["path"] == "/profile/tfa-disable"
    assert body["current_state"] is None
    assert len(body["side_effects"]) == 1
    assert len(body["warnings"]) == 1
    assert "security" in body["warnings"][0]


async def test_profile_tfa_enable_confirm_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the TFA confirm POST with no call."""
    result = await handle_linode_profile_tfa_enable_confirm(
        {"tfa_code": "123456", "dry_run": True}, sample_config
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_tfa_enable_confirm"
    assert body["would_execute"]["path"] == "/profile/tfa-enable-confirm"
    assert len(body["side_effects"]) == 1
    assert "enabled" in body["side_effects"][0]


async def test_profile_phone_number_send_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the phone-number POST with no call."""
    result = await handle_linode_profile_phone_number_send(
        {"iso_code": "US", "phone_number": "5551234567", "dry_run": True},
        sample_config,
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_phone_number_send"
    assert body["would_execute"]["path"] == "/profile/phone-number"
    assert len(body["side_effects"]) == 1
    assert "verification code" in body["side_effects"][0]


async def test_profile_phone_number_verify_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the phone-number verify POST with no call."""
    result = await handle_linode_profile_phone_number_verify(
        {"otp_code": "000111", "dry_run": True}, sample_config
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_phone_number_verify"
    assert body["would_execute"]["path"] == "/profile/phone-number/verify"
    assert len(body["side_effects"]) == 1
    assert "verified" in body["side_effects"][0]


async def test_profile_phone_number_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the phone-number DELETE with no call."""
    result = await handle_linode_profile_phone_number_delete(
        {"dry_run": True}, sample_config
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_phone_number_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/profile/phone-number"


async def test_profile_security_questions_answer_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the security-questions POST with no call."""
    result = await handle_linode_profile_security_questions_answer(
        {
            "security_questions": [
                {"question_id": 1, "response": "answer1"},
                {"question_id": 2, "response": "answer2"},
                {"question_id": 3, "response": "answer3"},
            ],
            "dry_run": True,
        },
        sample_config,
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_security_questions_answer"
    assert body["would_execute"]["path"] == "/profile/security-questions"
    assert len(body["side_effects"]) == 1
    assert "answers are saved" in body["side_effects"][0]


async def test_profile_token_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the token create POST and never echoes a token."""
    result = await handle_linode_profile_token_create(
        {"label": "ci", "dry_run": True}, sample_config
    )
    body = _profile_preview_body(result)
    assert body["tool"] == "linode_profile_token_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/profile/tokens"
    assert body["current_state"] is None
    assert "token" not in str(body["current_state"])
    assert len(body["side_effects"]) == 1
    assert "ci" in body["side_effects"][0]
    assert len(body["warnings"]) == 1
    assert "once" in body["warnings"][0]


async def test_profile_token_update_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the token metadata via GET and never updates."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_profile_token.return_value = {"id": 9, "label": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_profile_token_update(
            {"token_id": 9, "label": "renamed", "dry_run": True}, sample_config
        )

        body = _profile_preview_body(result)
        assert body["tool"] == "linode_profile_token_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/profile/tokens/9"
        assert len(body["side_effects"]) == 1
        assert "renamed" in body["side_effects"][0]
        mock_client.get_profile_token.assert_awaited_once_with(9)
        mock_client.update_profile_token.assert_not_called()


async def test_profile_token_revoke_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the token metadata via GET and never revokes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_profile_token.return_value = {"id": 9}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_profile_token_revoke(
            {"token_id": 9, "dry_run": True}, sample_config
        )

        body = _profile_preview_body(result)
        assert body["tool"] == "linode_profile_token_revoke"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/profile/tokens/9"
        mock_client.get_profile_token.assert_awaited_once_with(9)
        mock_client.delete_profile_token.assert_not_called()


async def test_profile_app_revoke_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the app via GET and never revokes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_profile_app.return_value = {"id": 5}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_profile_app_revoke(
            {"app_id": 5, "dry_run": True}, sample_config
        )

        body = _profile_preview_body(result)
        assert body["tool"] == "linode_profile_app_revoke"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/profile/apps/5"
        mock_client.get_profile_app.assert_awaited_once_with(5)
        mock_client.delete_profile_app.assert_not_called()


async def test_profile_device_revoke_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true fetches the device via GET and never revokes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_profile_device.return_value = {"id": 3}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_profile_device_revoke(
            {"device_id": 3, "dry_run": True}, sample_config
        )

        body = _profile_preview_body(result)
        assert body["tool"] == "linode_profile_device_revoke"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/profile/devices/3"
        mock_client.get_profile_device.assert_awaited_once_with(3)
        mock_client.delete_profile_device.assert_not_called()


async def test_instance_delete_dry_run_dependency_walk(
    sample_config: Config,
) -> None:
    """dry_run surfaces attached volumes + public IPs, warns, never deletes."""
    from dataclasses import fields as dataclass_fields
    from types import SimpleNamespace

    from linodemcp.linode import Instance

    instance_kwargs: dict[str, Any] = {
        field.name: None for field in dataclass_fields(Instance)
    }
    instance_kwargs["status"] = "running"

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = Instance(**instance_kwargs)
        mock_client.list_volumes.return_value = [
            SimpleNamespace(id=6789, label="data-vol", size=50, linode_id=123),
            SimpleNamespace(id=1, label="other-vol", size=10, linode_id=999),
        ]
        mock_client.list_instance_ips.return_value = {
            "ipv4": {"public": [{"address": "198.51.100.10"}]}
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_instance_delete(
            {"instance_id": 123, "dry_run": True}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_instance_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/linode/instances/123"

        deps = body["dependencies"]
        assert sorted(d["kind"] for d in deps) == ["public_ip", "volume"]

        volume_dep = next(d for d in deps if d["kind"] == "volume")
        assert volume_dep["id"] == 6789
        assert volume_dep["action"] == "detached"

        ip_dep = next(d for d in deps if d["kind"] == "public_ip")
        assert ip_dep["label"] == "198.51.100.10"
        assert ip_dep["action"] == "released"

        assert body["warnings"]
        mock_client.delete_instance.assert_not_called()


async def test_volume_delete_dry_run_dependency_walk(
    sample_config: Config,
) -> None:
    """dry_run surfaces the attached instance and never deletes."""
    from dataclasses import fields as dataclass_fields

    from linodemcp.linode import Volume

    volume_kwargs: dict[str, Any] = {
        field.name: None for field in dataclass_fields(Volume)
    }
    volume_kwargs.update({"id": 789, "linode_id": 456, "linode_label": "attached-host"})

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_volume.return_value = Volume(**volume_kwargs)
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_volume_delete(
            {"volume_id": 789, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_volume_delete"
        deps = body["dependencies"]
        assert len(deps) == 1
        assert deps[0]["kind"] == "instance"
        assert deps[0]["id"] == 456
        assert deps[0]["label"] == "attached-host"
        assert deps[0]["action"] == "detached"
        assert body["warnings"]
        mock_client.delete_volume.assert_not_called()


async def test_lke_cluster_delete_dry_run_dependency_walk(
    sample_config: Config,
) -> None:
    """dry_run lists node pools as cascade dependencies and never deletes."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {"id": 55, "label": "prod"}
        mock_client.list_lke_node_pools.return_value = [
            {"id": 1, "type": "g6-standard-2", "count": 3},
            {"id": 2, "type": "g6-standard-4", "count": 2},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_lke_cluster_delete(
            {"cluster_id": 55, "dry_run": True}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["tool"] == "linode_lke_cluster_delete"
        deps = body["dependencies"]
        assert len(deps) == 2
        assert all(dep["kind"] == "node_pool" for dep in deps)
        assert all(dep["action"] == "cascade_deleted" for dep in deps)
        assert any("5 node(s)" in warning for warning in body["warnings"])
        mock_client.delete_lke_cluster.assert_not_called()


def test_create_linode_instance_config_create_tool_schema() -> None:
    """Instance config create tool exposes required body and confirm fields."""
    tool, capability = create_linode_instance_config_create_tool()

    assert tool.name == "linode_instance_config_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["type"] == "string"
    assert tool.inputSchema["properties"]["label"]["type"] == "string"
    assert tool.inputSchema["properties"]["devices"]["type"] == "object"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert set(tool.inputSchema["required"]) == {
        "instance_id",
        "label",
        "devices",
        "confirm",
    }


async def test_handle_linode_instance_config_create_success(
    sample_config: Config,
) -> None:
    """Instance config create handler calls the client with validated inputs."""
    devices = {"sda": {"disk_id": 123}}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_instance_config.return_value = {
            "id": 987,
            "label": "boot-config",
            "devices": devices,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_create(
            {
                "instance_id": "456",
                "label": "boot-config",
                "devices": devices,
                "confirm": True,
            },
            sample_config,
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["id"] == 987
    assert body["label"] == "boot-config"
    mock_client.create_instance_config.assert_awaited_once_with(
        456, label="boot-config", devices=devices
    )


async def test_handle_linode_instance_config_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the config create without requiring confirm or mutating."""
    devices = {"sda": {"disk_id": 123}}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance.return_value = {"id": 456, "label": "vm"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_create(
            {
                "instance_id": "456",
                "label": "boot-config",
                "devices": devices,
                "dry_run": True,
            },
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/456/configs"
    assert body["current_state"]["id"] == 456
    assert "boot-config" in body["side_effects"][0]
    mock_client.get_instance.assert_awaited_once_with(456)
    mock_client.create_instance_config.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_config_create_requires_boolean_confirm(
    sample_config: Config,
    confirm: Any,
) -> None:
    """Missing, false, string, and numeric confirms fail before client calls."""
    arguments: dict[str, Any] = {
        "instance_id": "456",
        "label": "boot-config",
        "devices": {"sda": {"disk_id": 123}},
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_config_create(arguments, sample_config)

    assert result[0].text == "Error: Set confirm=true to proceed."
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("instance_id", ["12/34", "12?bad", ".."])
async def test_handle_linode_instance_config_create_rejects_malformed_instance_id(
    sample_config: Config,
    instance_id: str,
) -> None:
    """Malformed path parameters are rejected before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_config_create(
            {
                "instance_id": instance_id,
                "label": "boot-config",
                "devices": {"sda": {"disk_id": 123}},
                "confirm": True,
            },
            sample_config,
        )

    assert "integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"instance_id": "456", "devices": {"sda": {}}, "confirm": True}, "label"),
        (
            {
                "instance_id": "456",
                "label": {"not": "string"},
                "devices": {"sda": {}},
                "confirm": True,
            },
            "label",
        ),
        (
            {"instance_id": "456", "label": "boot-config", "confirm": True},
            "devices",
        ),
        (
            {
                "instance_id": "456",
                "label": "boot-config",
                "devices": {},
                "confirm": True,
            },
            "devices",
        ),
        (
            {
                "instance_id": "456",
                "label": "boot-config",
                "devices": [],
                "confirm": True,
            },
            "devices",
        ),
    ],
)
async def test_handle_linode_instance_config_create_validates_required_arguments(
    sample_config: Config,
    arguments: dict[str, Any],
    message: str,
) -> None:
    """Required body arguments are validated before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_config_create(arguments, sample_config)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_instance_disk_password_reset_tool_def() -> None:
    """Disk password reset should require IDs, password, confirm, and expose dry_run."""
    tool, capability = create_linode_instance_disk_password_reset_tool()
    assert tool.name == "linode_instance_disk_password_reset"
    assert capability is Capability.Write
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    assert "disk_id" in required
    assert "password" in required
    assert "confirm" in required
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_handle_linode_instance_disk_password_reset_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk password reset should call the client with valid args and confirm=true."""
    mock_linode_client.reset_instance_disk_password.return_value = None

    result = await handle_linode_instance_disk_password_reset(
        {
            "instance_id": 123,
            "disk_id": 10,
            "password": "NewStr0ngP@ss!",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["message"] == "Root password reset for disk 10 on instance 123"
    assert data["instance_id"] == 123
    assert data["disk_id"] == 10
    mock_linode_client.reset_instance_disk_password.assert_called_once_with(
        123, 10, "NewStr0ngP@ss!"
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_instance_disk_password_reset_requires_boolean_confirm(
    mock_linode_client: AsyncMock, sample_config: Config, confirm: Any
) -> None:
    """Missing, false, string, and numeric confirm values are rejected."""
    arguments: dict[str, Any] = {
        "instance_id": 123,
        "disk_id": 10,
        "password": "NewStr0ngP@ss!",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_disk_password_reset(arguments, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()
    mock_linode_client.reset_instance_disk_password.assert_not_called()


@pytest.mark.parametrize("field", ["instance_id", "disk_id"])
@pytest.mark.parametrize("value", ["1/2", "1?x=2", ".."])
async def test_instance_disk_password_reset_rejects_malformed_path_params(
    mock_linode_client: AsyncMock, sample_config: Config, field: str, value: str
) -> None:
    """Malformed finite IDs are rejected before the client call."""
    arguments: dict[str, Any] = {
        "instance_id": 123,
        "disk_id": 10,
        "password": "NewStr0ngP@ss!",
        "confirm": True,
    }
    arguments[field] = value

    result = await handle_linode_instance_disk_password_reset(arguments, sample_config)

    assert len(result) == 1
    assert "valid integer" in result[0].text.lower()
    mock_linode_client.reset_instance_disk_password.assert_not_called()


async def test_instance_disk_password_reset_missing_password(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing password is rejected before confirm/client handling."""
    result = await handle_linode_instance_disk_password_reset(
        {"instance_id": 123, "disk_id": 10, "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "password is required" in result[0].text
    mock_linode_client.reset_instance_disk_password.assert_not_called()


async def test_instance_disk_password_reset_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true fetches the disk and never resets the password."""
    mock_linode_client.get_instance_disk.return_value = {"id": 10, "label": "boot"}

    result = await handle_linode_instance_disk_password_reset(
        {
            "instance_id": 123,
            "disk_id": 10,
            "password": "NewStr0ngP@ss!",
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_disk_password_reset"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/disks/10/password"
    assert body["warnings"]
    mock_linode_client.get_instance_disk.assert_awaited_once_with(123, 10)
    mock_linode_client.reset_instance_disk_password.assert_not_called()


async def test_instance_volumes_list_tool_def() -> None:
    """Linode volumes list tool should require instance_id and expose pagination."""
    tool, capability = create_linode_instance_volumes_list_tool()
    assert tool.name == "linode_instance_volumes_list"
    assert capability is Capability.Read
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    props = tool.inputSchema["properties"]
    assert props["page"]["minimum"] == 1
    assert props["page_size"]["minimum"] == 25
    assert props["page_size"]["maximum"] == 500


async def test_instance_volumes_list_success(sample_config: Config) -> None:
    """Linode volumes list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_volumes.return_value = {
            "data": [{"id": 123, "label": "data"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_volumes_list(
                {"instance_id": 42, "page": 1, "page_size": 25}, sample_config
            )
        )

    assert len(result) == 1
    assert "data" in result[0].text
    mock_client.list_instance_volumes.assert_awaited_once_with(42, page=1, page_size=25)


@pytest.mark.parametrize("instance_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_instance_volumes_list_rejects_invalid_instance_id(
    sample_config: Config, instance_id: object
) -> None:
    """Linode volumes list handler rejects malformed instance IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        result = list(
            await handle_linode_instance_volumes_list(
                {"instance_id": instance_id}, sample_config
            )
        )

    assert len(result) == 1
    assert "instance_id" in result[0].text.lower()
    mc.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"instance_id": 42, "page": "x"}, "page"),
        ({"instance_id": 42, "page": True}, "page"),
        ({"instance_id": 42, "page": 0}, "page"),
        ({"instance_id": 42, "page_size": "x"}, "page_size"),
        ({"instance_id": 42, "page_size": True}, "page_size"),
        ({"instance_id": 42, "page_size": 24}, "page_size"),
        ({"instance_id": 42, "page_size": 501}, "page_size"),
    ],
)
async def test_instance_volumes_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Linode volumes list handler validates pagination before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        result = list(
            await handle_linode_instance_volumes_list(arguments, sample_config)
        )

    assert len(result) == 1
    assert message in result[0].text
    mc.assert_not_called()


async def test_instance_firewalls_list_tool_def() -> None:
    """Linode firewalls list tool should require instance_id and expose pagination."""
    tool, _ = create_linode_instance_firewalls_list_tool()
    assert tool.name == "linode_instance_firewalls_list"
    required: list[str] = tool.inputSchema.get("required") or []
    assert "instance_id" in required
    props = tool.inputSchema["properties"]
    assert props["page"]["minimum"] == 1
    assert props["page_size"]["minimum"] == 25
    assert props["page_size"]["maximum"] == 500


async def test_instance_firewalls_list_success(sample_config: Config) -> None:
    """Linode firewalls list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_firewalls.return_value = {
            "data": [{"id": 123, "label": "web"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_firewalls_list(
                {"instance_id": 42, "page": 1, "page_size": 25}, sample_config
            )
        )

    assert len(result) == 1
    assert "web" in result[0].text
    mock_client.list_instance_firewalls.assert_awaited_once_with(
        42, page=1, page_size=25
    )


@pytest.mark.parametrize("instance_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_instance_firewalls_list_rejects_invalid_instance_id(
    sample_config: Config, instance_id: object
) -> None:
    """Linode firewalls list handler rejects malformed instance IDs."""
    result = list(
        await handle_linode_instance_firewalls_list(
            {"instance_id": instance_id}, sample_config
        )
    )

    assert len(result) == 1
    assert "instance_id" in result[0].text.lower()


async def test_instance_interface_firewalls_list_tool_def() -> None:
    """Linode interface firewalls list tool requires both path params."""
    tool, capability = create_linode_instance_interface_firewalls_list_tool()
    assert tool.name == "linode_instance_interface_firewalls_list"
    assert capability is Capability.Read
    required: list[str] = tool.inputSchema.get("required") or []
    assert required == ["linode_id", "interface_id"]
    props = tool.inputSchema["properties"]
    assert props["linode_id"]["minimum"] == 1
    assert props["interface_id"]["minimum"] == 1


async def test_instance_interface_firewalls_list_success(sample_config: Config) -> None:
    """Linode interface firewalls list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.list_instance_interface_firewalls.return_value = {
            "data": [{"id": 123, "label": "web"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_interface_firewalls_list(
                {"linode_id": 42, "interface_id": 7}, sample_config
            )
        )

    assert len(result) == 1
    assert "web" in result[0].text
    mock_client.list_instance_interface_firewalls.assert_awaited_once_with(42, 7)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"linode_id": "bad/id", "interface_id": 7}, "linode_id"),
        ({"linode_id": "bad?query", "interface_id": 7}, "linode_id"),
        ({"linode_id": "..", "interface_id": 7}, "linode_id"),
        ({"linode_id": True, "interface_id": 7}, "linode_id"),
        ({"linode_id": 0, "interface_id": 7}, "linode_id"),
        ({"linode_id": -1, "interface_id": 7}, "linode_id"),
        ({"linode_id": 42, "interface_id": "bad/id"}, "interface_id"),
        ({"linode_id": 42, "interface_id": "bad?query"}, "interface_id"),
        ({"linode_id": 42, "interface_id": ".."}, "interface_id"),
        ({"linode_id": 42, "interface_id": True}, "interface_id"),
        ({"linode_id": 42, "interface_id": 0}, "interface_id"),
        ({"linode_id": 42, "interface_id": -1}, "interface_id"),
    ],
)
async def test_instance_interface_firewalls_list_rejects_invalid_path_args(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Linode interface firewalls list handler rejects malformed path args."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        result = list(
            await handle_linode_instance_interface_firewalls_list(
                arguments, sample_config
            )
        )

    assert len(result) == 1
    assert message in result[0].text.lower()
    mc.assert_not_called()


async def test_instance_firewalls_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Linode firewalls list handler validates pagination before client call."""
    result = list(
        await handle_linode_instance_firewalls_list(
            {"instance_id": 42, "page_size": 24}, sample_config
        )
    )

    assert len(result) == 1
    assert "page_size" in result[0].text
