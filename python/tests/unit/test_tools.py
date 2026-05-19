"""Unit tests for MCP tools."""

import json
from typing import Any
from unittest.mock import AsyncMock, patch

import pytest

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
    create_linode_image_create_tool,
    create_linode_instance_backup_create_tool,
    create_linode_instance_backup_get_tool,
    create_linode_instance_backup_restore_tool,
    create_linode_instance_backups_cancel_tool,
    create_linode_instance_backups_enable_tool,
    create_linode_instance_backups_list_tool,
    create_linode_instance_clone_tool,
    create_linode_instance_disk_clone_tool,
    create_linode_instance_disk_create_tool,
    create_linode_instance_disk_delete_tool,
    create_linode_instance_disk_get_tool,
    create_linode_instance_disk_resize_tool,
    create_linode_instance_disk_update_tool,
    create_linode_instance_disks_list_tool,
    create_linode_instance_ip_allocate_tool,
    create_linode_instance_ip_delete_tool,
    create_linode_instance_ip_get_tool,
    create_linode_instance_ip_update_tool,
    create_linode_instance_ips_list_tool,
    create_linode_instance_migrate_tool,
    create_linode_instance_password_reset_tool,
    create_linode_instance_rebuild_tool,
    create_linode_instance_rescue_tool,
    create_linode_instance_update_tool,
    create_linode_ipv6_range_create_tool,
    create_linode_ipv6_range_delete_tool,
    create_linode_ipv6_range_get_tool,
    create_linode_lke_cluster_create_tool,
    create_linode_lke_cluster_delete_tool,
    create_linode_lke_cluster_get_tool,
    create_linode_lke_clusters_list_tool,
    create_linode_monitor_service_token_create_tool,
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
    handle_linode_firewall_update,
    handle_linode_firewalls_list,
    handle_linode_image_create,
    handle_linode_images_list,
    handle_linode_instance_backup_create,
    handle_linode_instance_backup_get,
    handle_linode_instance_backup_restore,
    handle_linode_instance_backups_cancel,
    handle_linode_instance_backups_enable,
    handle_linode_instance_backups_list,
    handle_linode_instance_boot,
    handle_linode_instance_clone,
    handle_linode_instance_create,
    handle_linode_instance_delete,
    handle_linode_instance_disk_clone,
    handle_linode_instance_disk_create,
    handle_linode_instance_disk_delete,
    handle_linode_instance_disk_get,
    handle_linode_instance_disk_resize,
    handle_linode_instance_disk_update,
    handle_linode_instance_disks_list,
    handle_linode_instance_get,
    handle_linode_instance_ip_allocate,
    handle_linode_instance_ip_delete,
    handle_linode_instance_ip_get,
    handle_linode_instance_ip_update,
    handle_linode_instance_ips_list,
    handle_linode_instance_migrate,
    handle_linode_instance_password_reset,
    handle_linode_instance_reboot,
    handle_linode_instance_rebuild,
    handle_linode_instance_rescue,
    handle_linode_instance_resize,
    handle_linode_instance_shutdown,
    handle_linode_instance_update,
    handle_linode_instances_list,
    handle_linode_ipv6_range_create,
    handle_linode_ipv6_range_delete,
    handle_linode_ipv6_range_get,
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
    handle_linode_monitor_service_token_create,
    handle_linode_nodebalancer_create,
    handle_linode_nodebalancer_delete,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancer_update,
    handle_linode_nodebalancers_list,
    handle_linode_object_storage_bucket_access_allow,
    handle_linode_object_storage_bucket_access_get,
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_bucket_contents,
    handle_linode_object_storage_bucket_create,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_bucket_get,
    handle_linode_object_storage_buckets_list,
    handle_linode_object_storage_clusters_list,
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
    handle_linode_stackscripts_list,
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
    assert "Error" in result[0].text
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


async def test_handle_linode_object_storage_clusters_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_clusters_list tool."""
    mock_clusters = [
        {
            "id": "us-east-1",
            "region": "us-east",
            "domain": "us-east-1.linodeobjects.com",
            "status": "available",
            "static_site": {"domain": "website-us-east-1.linodeobjects.com"},
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_clusters.return_value = mock_clusters
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_clusters_list({}, sample_config)

        assert len(result) == 1
        assert "us-east-1" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_clusters.assert_called_once()


async def test_handle_linode_object_storage_clusters_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_clusters_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_clusters.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_clusters_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


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

        result = list(await handle_linode_lke_tier_versions_list({}, sample_config))

        assert len(result) == 1
        assert "1.29" in result[0].text


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
