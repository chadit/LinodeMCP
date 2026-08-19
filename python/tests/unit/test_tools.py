"""Unit tests for MCP tools."""

import json
from typing import Any
from unittest.mock import AsyncMock, patch

import pytest
from mcp.types import TextContent

from linodemcp.config import Config
from linodemcp.genpb.linode.mcp.v1 import (
    common_pb2,
    sshkey_pb2,
)
from linodemcp.gentools import (
    create_linode_account_agreement_acknowledge_tool,
    create_linode_account_availability_list_tool,
    create_linode_account_beta_enroll_tool,
    create_linode_account_beta_get_tool,
    create_linode_account_event_get_tool,
    create_linode_account_invoice_item_list_tool,
    create_linode_account_maintenance_list_tool,
    create_linode_account_oauth_client_get_tool,
    create_linode_account_payment_method_get_tool,
    create_linode_account_settings_get_tool,
    create_linode_firewall_get_tool,
    create_linode_firewall_rules_get_tool,
    create_linode_firewall_template_get_tool,
    create_linode_image_get_tool,
    create_linode_instance_backup_get_tool,
    create_linode_instance_backup_list_tool,
    create_linode_instance_backups_cancel_tool,
    create_linode_instance_config_delete_tool,
    create_linode_instance_config_get_tool,
    create_linode_instance_config_interface_get_tool,
    create_linode_instance_config_interface_list_tool,
    create_linode_instance_config_list_tool,
    create_linode_instance_disk_delete_tool,
    create_linode_instance_disk_get_tool,
    create_linode_instance_disk_list_tool,
    create_linode_instance_firewall_list_tool,
    create_linode_instance_interface_firewall_list_tool,
    create_linode_instance_ip_delete_tool,
    create_linode_instance_ip_get_tool,
    create_linode_instance_ip_list_tool,
    create_linode_instance_password_reset_tool,
    create_linode_instance_stats_get_tool,
    create_linode_instance_volume_list_tool,
    create_linode_ipv6_range_delete_tool,
    create_linode_ipv6_range_get_tool,
    create_linode_kernel_get_tool,
    create_linode_kernel_list_tool,
    create_linode_lke_cluster_get_tool,
    create_linode_lke_cluster_list_tool,
    create_linode_maintenance_policy_list_tool,
    create_linode_managed_contact_get_tool,
    create_linode_managed_contact_list_tool,
    create_linode_managed_credential_list_tool,
    create_linode_managed_issue_get_tool,
    create_linode_managed_issue_list_tool,
    create_linode_managed_linode_settings_list_tool,
    create_linode_managed_service_get_tool,
    create_linode_managed_sshkey_get_tool,
    create_linode_monitor_service_alert_definition_get_tool,
    create_linode_monitor_service_get_tool,
    create_linode_monitor_service_list_tool,
    create_linode_nodebalancer_config_delete_tool,
    create_linode_nodebalancer_config_get_tool,
    create_linode_nodebalancer_config_list_tool,
    create_linode_nodebalancer_config_node_delete_tool,
    create_linode_nodebalancer_config_node_get_tool,
    create_linode_nodebalancer_firewall_list_tool,
    create_linode_nodebalancer_stats_get_tool,
    create_linode_nodebalancer_vpc_config_get_tool,
    create_linode_nodebalancer_vpc_config_list_tool,
    create_linode_object_storage_cancel_tool,
    create_linode_object_storage_endpoint_list_tool,
    create_linode_object_storage_quota_get_tool,
    create_linode_object_storage_quota_list_tool,
    create_linode_object_storage_quota_usage_get_tool,
    create_linode_placement_group_assign_tool,
    create_linode_placement_group_create_tool,
    create_linode_placement_group_get_tool,
    create_linode_placement_group_list_tool,
    create_linode_placement_group_unassign_tool,
    create_linode_placement_group_update_tool,
    create_linode_profile_app_get_tool,
    create_linode_profile_app_list_tool,
    create_linode_profile_device_get_tool,
    create_linode_profile_device_list_tool,
    create_linode_profile_login_get_tool,
    create_linode_profile_login_list_tool,
    create_linode_profile_phone_number_delete_tool,
    create_linode_profile_security_question_list_tool,
    create_linode_profile_tfa_disable_tool,
    create_linode_profile_token_get_tool,
    create_linode_profile_token_list_tool,
    create_linode_region_availability_get_tool,
    create_linode_region_availability_list_tool,
    create_linode_region_get_tool,
    create_linode_stackscript_create_tool,
    create_linode_stackscript_delete_tool,
    create_linode_support_ticket_get_tool,
    create_linode_support_ticket_list_tool,
    create_linode_support_ticket_reply_list_tool,
    create_linode_tag_create_tool,
    create_linode_tag_list_tool,
    create_linode_tag_object_list_tool,
    create_linode_vlan_delete_tool,
    create_linode_vlan_list_tool,
    create_linode_vpc_delete_tool,
    create_linode_vpc_get_tool,
    create_linode_vpc_list_tool,
    create_linode_vpc_subnet_delete_tool,
    handle_linode_account_agreement_acknowledge,
    handle_linode_account_availability_list,
    handle_linode_account_beta_enroll,
    handle_linode_account_beta_get,
    handle_linode_account_beta_list,
    handle_linode_account_child_account_list,
    handle_linode_account_event_get,
    handle_linode_account_get,
    handle_linode_account_invoice_item_list,
    handle_linode_account_maintenance_list,
    handle_linode_account_notification_list,
    handle_linode_account_oauth_client_get,
    handle_linode_account_payment_method_get,
    handle_linode_account_payment_method_list,
    handle_linode_account_service_transfer_list,
    handle_linode_account_settings_get,
    handle_linode_domain_clone,
    handle_linode_domain_create,
    handle_linode_domain_get,
    handle_linode_domain_list,
    handle_linode_domain_record_create,
    handle_linode_domain_record_get,
    handle_linode_domain_record_list,
    handle_linode_domain_record_update,
    handle_linode_domain_update,
    handle_linode_firewall_delete,
    handle_linode_firewall_get,
    handle_linode_firewall_list,
    handle_linode_firewall_rules_get,
    handle_linode_firewall_template_get,
    handle_linode_firewall_template_list,
    handle_linode_image_get,
    handle_linode_image_list,
    handle_linode_instance_backup_get,
    handle_linode_instance_backup_list,
    handle_linode_instance_backups_cancel,
    handle_linode_instance_config_delete,
    handle_linode_instance_config_get,
    handle_linode_instance_config_interface_get,
    handle_linode_instance_config_interface_list,
    handle_linode_instance_config_list,
    handle_linode_instance_delete,
    handle_linode_instance_disk_delete,
    handle_linode_instance_disk_get,
    handle_linode_instance_disk_list,
    handle_linode_instance_firewall_list,
    handle_linode_instance_get,
    handle_linode_instance_interface_firewall_list,
    handle_linode_instance_ip_delete,
    handle_linode_instance_ip_get,
    handle_linode_instance_ip_list,
    handle_linode_instance_list,
    handle_linode_instance_password_reset,
    handle_linode_instance_resize,
    handle_linode_instance_stats_get,
    handle_linode_instance_volume_list,
    handle_linode_ipv6_pool_list,
    handle_linode_ipv6_range_delete,
    handle_linode_ipv6_range_get,
    handle_linode_ipv6_range_list,
    handle_linode_kernel_get,
    handle_linode_kernel_list,
    handle_linode_lke_api_endpoint_list,
    handle_linode_lke_cluster_get,
    handle_linode_lke_cluster_list,
    handle_linode_lke_cluster_regenerate,
    handle_linode_lke_dashboard_get,
    handle_linode_lke_kubeconfig_get,
    handle_linode_lke_node_get,
    handle_linode_lke_node_recycle,
    handle_linode_lke_pool_get,
    handle_linode_lke_pool_list,
    handle_linode_lke_pool_recycle,
    handle_linode_lke_tier_version_list,
    handle_linode_lke_type_list,
    handle_linode_lke_version_get,
    handle_linode_lke_version_list,
    handle_linode_maintenance_policy_list,
    handle_linode_managed_contact_get,
    handle_linode_managed_contact_list,
    handle_linode_managed_credential_list,
    handle_linode_managed_issue_get,
    handle_linode_managed_issue_list,
    handle_linode_managed_linode_settings_list,
    handle_linode_managed_service_get,
    handle_linode_managed_service_list,
    handle_linode_managed_sshkey_get,
    handle_linode_monitor_service_alert_definition_get,
    handle_linode_monitor_service_get,
    handle_linode_monitor_service_list,
    handle_linode_network_transfer_price_list,
    handle_linode_nodebalancer_config_delete,
    handle_linode_nodebalancer_config_get,
    handle_linode_nodebalancer_config_list,
    handle_linode_nodebalancer_config_node_delete,
    handle_linode_nodebalancer_config_node_get,
    handle_linode_nodebalancer_config_node_list,
    handle_linode_nodebalancer_delete,
    handle_linode_nodebalancer_firewall_list,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancer_list,
    handle_linode_nodebalancer_stats_get,
    handle_linode_nodebalancer_vpc_config_get,
    handle_linode_nodebalancer_vpc_config_list,
    handle_linode_object_storage_bucket_access_get,
    handle_linode_object_storage_bucket_by_region_list,
    handle_linode_object_storage_bucket_create,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_bucket_get,
    handle_linode_object_storage_bucket_list,
    handle_linode_object_storage_cancel,
    handle_linode_object_storage_endpoint_list,
    handle_linode_object_storage_key_delete,
    handle_linode_object_storage_key_get,
    handle_linode_object_storage_key_list,
    handle_linode_object_storage_object_acl_get,
    handle_linode_object_storage_object_acl_update,
    handle_linode_object_storage_quota_get,
    handle_linode_object_storage_quota_list,
    handle_linode_object_storage_quota_usage_get,
    handle_linode_object_storage_ssl_delete,
    handle_linode_object_storage_ssl_get,
    handle_linode_object_storage_ssl_upload,
    handle_linode_object_storage_transfer_get,
    handle_linode_object_storage_type_list,
    handle_linode_placement_group_assign,
    handle_linode_placement_group_create,
    handle_linode_placement_group_get,
    handle_linode_placement_group_list,
    handle_linode_placement_group_unassign,
    handle_linode_placement_group_update,
    handle_linode_profile_app_get,
    handle_linode_profile_app_list,
    handle_linode_profile_device_get,
    handle_linode_profile_device_list,
    handle_linode_profile_get,
    handle_linode_profile_login_get,
    handle_linode_profile_login_list,
    handle_linode_profile_phone_number_delete,
    handle_linode_profile_security_question_list,
    handle_linode_profile_tfa_disable,
    handle_linode_profile_token_get,
    handle_linode_profile_token_list,
    handle_linode_region_availability_get,
    handle_linode_region_availability_list,
    handle_linode_region_get,
    handle_linode_region_list,
    handle_linode_sshkey_create,
    handle_linode_sshkey_delete,
    handle_linode_sshkey_get,
    handle_linode_sshkey_list,
    handle_linode_sshkey_update,
    handle_linode_stackscript_create,
    handle_linode_stackscript_delete,
    handle_linode_stackscript_list,
    handle_linode_support_ticket_get,
    handle_linode_support_ticket_list,
    handle_linode_support_ticket_reply_list,
    handle_linode_tag_create,
    handle_linode_tag_list,
    handle_linode_tag_object_list,
    handle_linode_type_get,
    handle_linode_type_list,
    handle_linode_vlan_delete,
    handle_linode_vlan_list,
    handle_linode_volume_attach,
    handle_linode_volume_clone,
    handle_linode_volume_create,
    handle_linode_volume_delete,
    handle_linode_volume_detach,
    handle_linode_volume_get,
    handle_linode_volume_list,
    handle_linode_volume_resize,
    handle_linode_volume_type_list,
    handle_linode_volume_update,
    handle_linode_vpc_delete,
    handle_linode_vpc_get,
    handle_linode_vpc_ip_all_list,
    handle_linode_vpc_ip_list,
    handle_linode_vpc_list,
    handle_linode_vpc_subnet_delete,
    handle_linode_vpc_subnet_get,
    handle_linode_vpc_subnet_list,
)
from linodemcp.gentools.oauth_client_thumbnail import (
    create_linode_account_oauth_client_thumbnail_get_tool,
    handle_linode_account_oauth_client_thumbnail_get,
)
from linodemcp.linode import (
    Profile,
    Volume,
)
from linodemcp.profiles import Capability
from linodemcp.tools.linode_object_storage import object_storage_key_to_response_dict
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema as proto_schema

# A public key long enough to pass the declared length window; the contract
# refuses anything under 80 characters, which a stub like "ssh-rsa AAAA" is.
SAMPLE_SSH_KEY = "ssh-rsa " + "A" * 96 + " user@host"


async def test_handle_linode_profile(
    sample_config: Config, sample_profile_data: dict[str, Any]
) -> None:
    """Test linode_profile_get tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = sample_profile_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_get({}, sample_config)

        assert len(result) == 1
        assert "testuser" in result[0].text
        assert "test@example.com" in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_profile_get")


async def test_handle_linode_profile_with_environment(sample_config: Config) -> None:
    """Test linode_profile_get tool with environment parameter."""
    raw_profile = {
        "username": "envuser",
        "email": "env@example.com",
        "timezone": "UTC",
        "email_notifications": True,
        "restricted": False,
        "two_factor_auth": False,
        "uid": 99999,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_get(
            {"environment": "default"}, sample_config
        )

        assert len(result) == 1
        assert "envuser" in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_profile_get")


async def test_handle_linode_profile_missing_environment(sample_config: Config) -> None:
    """Test linode_profile_get tool with missing environment."""
    result = await handle_linode_profile_get(
        {"environment": "nonexistent"}, sample_config
    )

    assert len(result) == 1
    assert "Error" in result[0].text or "error" in result[0].text


async def test_linode_instance_config_delete_tool_definition() -> None:
    """Test linode_instance_config_delete tool definition."""
    tool, capability = create_linode_instance_config_delete_tool()

    assert tool.name == "linode_instance_config_delete"
    assert capability == Capability.Destroy
    assert tool.input_schema["required"] == ["linode_id", "config_id", "confirm"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"


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


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": 0, "config_id": 6, "confirm": True},
        {"linode_id": -1, "config_id": 6, "confirm": True},
        {"linode_id": True, "config_id": 6, "confirm": True},
        {"linode_id": "1/2", "config_id": 6, "confirm": True},
        {"linode_id": "1?x", "config_id": 6, "confirm": True},
        {"linode_id": "..", "config_id": 6, "confirm": True},
        {"linode_id": 123, "confirm": True},
        {"linode_id": 123, "config_id": 0, "confirm": True},
        {"linode_id": 123, "config_id": -1, "confirm": True},
        {"linode_id": 123, "config_id": True, "confirm": True},
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
    assert (
        "positive integer" in result[0].text
        or "confirm=true" in result[0].text
        or "is required" in result[0].text
    )


async def test_handle_linode_instance_config_delete_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_delete error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_call.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_delete(
            {"linode_id": 123, "config_id": 6, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert (
        "Failed to remove configuration profile 6 from instance 123" in result[0].text
    )


async def test_linode_instance_config_get_tool_definition() -> None:
    """Test linode_instance_config_get tool definition."""
    tool, capability = create_linode_instance_config_get_tool()

    assert tool.name == "linode_instance_config_get"
    assert capability == Capability.Read
    assert tool.input_schema["required"] == ["linode_id", "config_id"]
    assert "linode_id" in tool.input_schema["properties"]
    assert "config_id" in tool.input_schema["properties"]


async def test_handle_linode_instance_config_get(sample_config: Config) -> None:
    """Test linode_instance_config_get tool."""
    mock_config = {"id": 6, "label": "boot-config", "not_in_proto": "dropped"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_config
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_get(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "boot-config" in result[0].text
    assert "not_in_proto" not in result[0].text
    mock_client.route_raw.assert_called_once_with("linode_instance_config_get", 123, 6)


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
    assert "positive integer" in result[0].text or "is required" in result[0].text


async def test_handle_linode_instance_config_get_error(sample_config: Config) -> None:
    """Test linode_instance_config_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
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
    assert tool.input_schema["required"] == [
        "linode_id",
        "config_id",
        "interface_id",
    ]
    assert "linode_id" in tool.input_schema["properties"]
    assert "config_id" in tool.input_schema["properties"]
    assert "interface_id" in tool.input_schema["properties"]


async def test_handle_linode_instance_config_interface_get(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interface_get tool."""
    mock_interface = {"id": 9, "purpose": "vlan"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_interface
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_get(
            {"linode_id": 123, "config_id": 6, "interface_id": 9}, sample_config
        )

    assert len(result) == 1
    assert "vlan" in result[0].text
    mock_client.route_raw.assert_called_once_with(
        "linode_instance_config_interface_get", 123, 6, 9
    )


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
    assert "positive integer" in result[0].text or "is required" in result[0].text


async def test_handle_linode_instance_config_interface_get_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interface_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_get(
            {"linode_id": 123, "config_id": 6, "interface_id": 9}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_config_interfaces_list_tool_definition() -> None:
    """Test linode_instance_config_interface_list tool definition."""
    tool, capability = create_linode_instance_config_interface_list_tool()

    assert tool.name == "linode_instance_config_interface_list"
    assert capability == Capability.Read
    assert tool.input_schema["additionalProperties"] is False
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "linode_id",
        "config_id",
    }
    assert tool.input_schema["properties"]["linode_id"]["type"] == "integer"
    assert tool.input_schema["properties"]["config_id"]["type"] == "integer"
    assert tool.input_schema["required"] == ["linode_id", "config_id"]


async def test_handle_linode_instance_config_interfaces_list(
    sample_config: Config,
) -> None:
    """The handler normalizes the bare-array response into the tool envelope.

    The endpoint returns a bare JSON array, so the handler wraps it before
    serializing the {count, interfaces} proto envelope.
    """
    mock_interfaces = [
        {
            "id": 202,
            "active": True,
            "purpose": "vpc",
            "label": "eth1",
            "ipam_address": "10.0.0.1/24",
            "primary": True,
            "subnet_id": 55,
            "vpc_id": 77,
            "ipv4": {"nat_1_1": "192.0.2.10", "vpc": "10.0.0.5"},
            "ip_ranges": ["2001:db8::/64", "203.0.113.0/24"],
        },
        {"id": 101, "active": False, "purpose": "public", "primary": False},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_interfaces
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_list(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 2
    assert [iface["id"] for iface in payload["interfaces"]] == [202, 101]
    assert payload["interfaces"][0] == mock_interfaces[0]
    mock_client.route_raw.assert_called_once_with(
        "linode_instance_config_interface_list", 123, 6, query=""
    )


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
    """linode_instance_config_interface_list rejects malformed path parameters."""
    result = await handle_linode_instance_config_interface_list(
        arguments, sample_config
    )

    assert len(result) == 1
    assert "positive integer" in result[0].text or "is required" in result[0].text


async def test_handle_linode_instance_config_interfaces_list_error(
    sample_config: Config,
) -> None:
    """Test linode_instance_config_interface_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_interface_list(
            {"linode_id": 123, "config_id": 6}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_linode_instance_configs_list_tool_definition() -> None:
    """Test linode_instance_config_list tool definition."""
    tool, capability = create_linode_instance_config_list_tool()

    assert tool.name == "linode_instance_config_list"
    assert capability == Capability.Read
    assert tool.input_schema["required"] == ["linode_id"]


def test_create_linode_instance_stats_tool_schema() -> None:
    """Linode instance stats tool advertises the proto-generated input schema."""
    tool, capability = create_linode_instance_stats_get_tool()

    assert tool.name == "linode_instance_stats_get"
    assert capability == Capability.Read
    assert tool.input_schema == proto_schema("linode.mcp.v1.InstanceStatsGetInput")
    assert tool.input_schema["required"] == ["linode_id"]


async def test_handle_linode_instance_stats(sample_config: Config) -> None:
    """Test linode_instance_stats_get tool."""
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
        mock_client.route_raw.return_value = stats_payload
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_stats_get(
            {"linode_id": 123456}, sample_config
        )

    assert len(result) == 1
    assert "linode123" in result[0].text
    assert "1715731200000" in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_instance_stats_get", 123456)


@pytest.mark.parametrize("linode_id", [None, 0, -1, True, "1", "1/2", "1?x", ".."])
async def test_handle_linode_instance_stats_rejects_invalid_linode_id(
    sample_config: Config, linode_id: object
) -> None:
    """Malformed Linode IDs are rejected before the client call."""
    arguments = {} if linode_id is None else {"linode_id": linode_id}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_stats_get(arguments, sample_config)

    assert len(result) == 1
    assert (
        "linode_id must be a positive integer" in result[0].text
        or "linode_id is required" in result[0].text
    )
    mock_client_class.assert_not_called()


async def test_handle_linode_instance_configs_list(sample_config: Config) -> None:
    """Test linode_instance_config_list tool."""
    mock_configs = {
        "data": [{"id": 6, "label": "boot-config"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_list(
            {"linode_id": 123, "page": 2, "page_size": 50}, sample_config
        )

    assert len(result) == 1
    assert "boot-config" in result[0].text
    mock_client.route_raw.assert_called_once_with(
        "linode_instance_config_list", 123, query="page=2&page_size=50"
    )


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
    """linode_instance_config_list rejects malformed path parameters."""
    result = await handle_linode_instance_config_list(arguments, sample_config)

    assert len(result) == 1
    assert (
        "linode_id must be a positive integer" in result[0].text
        or "linode_id is required" in result[0].text
    )


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
    """linode_instance_config_list rejects invalid pagination."""
    result = await handle_linode_instance_config_list(arguments, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_instance_configs_list_error(sample_config: Config) -> None:
    """Test linode_instance_config_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_config_list(
            {"linode_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_instances_list(sample_config: Config) -> None:
    """Test linode_instance_list tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 123456, "label": "test-instance", "status": "running"}]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_list({}, sample_config)

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "123456" in result[0].text
        assert "running" in result[0].text


async def test_handle_linode_instances_list_with_status_filter(
    sample_config: Config,
) -> None:
    """Test linode_instance_list tool with status filter."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 123456, "label": "running-instance", "status": "running"},
                {"id": 789012, "label": "stopped-instance", "status": "stopped"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_list({"status": "running"}, sample_config)

        assert len(result) == 1
        assert "running-instance" in result[0].text
        assert "stopped-instance" not in result[0].text
        assert '"count": 1' in result[0].text
        assert "status=running" in result[0].text


async def test_handle_linode_instances_list_error(sample_config: Config) -> None:
    """Test linode_instance_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_list({}, sample_config)

        assert len(result) == 1
        assert (
            "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()
        )


async def test_handle_linode_instance_get(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_get tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = sample_instance_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": 123456}, sample_config
        )

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "running" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_instance_get", 123456)


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
    """Test linode_account_get tool."""
    raw_account = {
        "first_name": "Test",
        "last_name": "User",
        "email": "test@example.com",
        "company": "TestCo",
        "address_1": "123 Test St",
        "address_2": "",
        "city": "Test City",
        "state": "TS",
        "zip": "12345",
        "country": "US",
        "phone": "555-1234",
        "balance": 100.50,
        "balance_uninvoiced": 50.25,
        "capabilities": ["Linodes", "Block Storage"],
        "active_since": "2020-01-01T00:00:00",
        "euuid": "abcd-1234",
        "billing_source": "linode",
        "active_promotions": [],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_account
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_get({}, sample_config)

        assert len(result) == 1
        assert "Test" in result[0].text
        assert "test@example.com" in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_account_get")


async def test_create_linode_account_beta_enroll_tool() -> None:
    """Test linode_account_beta_enroll tool schema."""
    tool, capability = create_linode_account_beta_enroll_tool()

    assert tool.name == "linode_account_beta_enroll"
    assert capability is Capability.Admin
    assert tool.input_schema["properties"]["id"]["type"] == "string"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"
    assert "id" in tool.input_schema.get("required", [])
    assert "confirm" in tool.input_schema.get("required", [])


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
    assert body["side_effects"] == []
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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_beta_enroll(
            {"id": "distributed-beta", "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "message": "Account beta enrollment requested successfully",
        "id": "distributed-beta",
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_beta_enroll", body={"id": "distributed-beta"}
    )


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
        (
            {"id": "   ", "confirm": True},
            "id must contain only letters, numbers, underscores, and hyphens",
        ),
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
    """Test linode_account_agreement_acknowledge tool schema."""
    tool, capability = create_linode_account_agreement_acknowledge_tool()

    assert tool.name == "linode_account_agreement_acknowledge"
    assert capability.name == "Admin"
    assert "eu_model" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.input_schema.get("required", [])


async def test_account_agreements_ack_schema_requires_confirm() -> None:
    """The schema requires confirm for mutating acknowledgement calls."""
    tool, _capability = create_linode_account_agreement_acknowledge_tool()

    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.input_schema.get("required", [])


async def test_handle_linode_account_agreements_acknowledge_dry_run(
    sample_config: Config,
) -> None:
    """dry_run=true previews acknowledgement without confirm or client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreement_acknowledge(
            {"eu_model": True, "dry_run": True}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_account_agreement_acknowledge"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/account/agreements"
    assert body["would_execute"]["body"] == {"eu_model": True}
    assert body["current_state"] is None
    assert body["side_effects"] == []
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge(
    sample_config: Config,
) -> None:
    """Test linode_account_agreement_acknowledge tool."""
    response_data = {"accepted": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_agreement_acknowledge(
            {"eu_model": True, "privacy_policy": True, "confirm": True},
            sample_config,
        )

    assert json.loads(result[0].text) == {
        "message": "Account agreements acknowledged successfully"
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_agreement_acknowledge",
        body={"eu_model": True, "privacy_policy": True},
    )


async def test_handle_linode_account_agreements_acknowledge_rejects_false(
    sample_config: Config,
) -> None:
    """Agreement acknowledge rejects a false value locally (matches Go)."""
    result = await handle_linode_account_agreement_acknowledge(
        {"billing_agreement": False, "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "billing_agreement must be true when provided" in result[0].text


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_account_agreements_acknowledge_requires_boolean_confirm(
    sample_config: Config, bad_confirm: object
) -> None:
    """Agreement acknowledgement rejects non-true confirm before client call."""
    arguments: dict[str, object] = {"eu_model": True}
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreement_acknowledge(
            arguments, sample_config
        )

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge_requires_field(
    sample_config: Config,
) -> None:
    """Agreement acknowledgement requires at least one agreement field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreement_acknowledge(
            {"confirm": True}, sample_config
        )

    assert "at least one account agreement field" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_agreements_acknowledge_requires_boolean_field(
    sample_config: Config,
) -> None:
    """Agreement acknowledgement rejects non-boolean agreement values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_agreement_acknowledge(
            {"confirm": True, "eu_model": "true"}, sample_config
        )

    assert "eu_model must be a boolean" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_contacts_list_tool() -> None:
    """Test linode_managed_contact_list tool schema."""
    tool, capability = create_linode_managed_contact_list_tool()

    assert tool.name == "linode_managed_contact_list"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert "required" not in tool.input_schema
    assert "environment" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


async def test_handle_linode_managed_contacts_list(sample_config: Config) -> None:
    """Test linode_managed_contact_list tool emits the proto envelope."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "name": "Primary", "email": "ops@example.com"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        # Proto-canonical {count, managed_contacts}: the element emits id/name/email
        # plus the always-present updated string; the optional group and nested
        # phone message are omitted when absent.
        assert json.loads(result[0].text) == {
            "count": 1,
            "managed_contacts": [
                {"id": 1, "name": "Primary", "email": "ops@example.com", "updated": ""}
            ],
        }
        mock_client.route_raw.assert_awaited_once_with(
            "linode_managed_contact_list", query="page=1&page_size=25"
        )


async def test_handle_linode_managed_contacts_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_contact_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_list({"page": 0}, sample_config)

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("page_size", "expected"),
    [
        (24, "page_size must be an integer from 25 through 500"),
        (501, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_handle_linode_managed_contacts_list_rejects_invalid_page_size(
    sample_config: Config, page_size: int, expected: str
) -> None:
    """Test linode_managed_contact_list rejects out-of-range page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_list(
            {"page_size": page_size}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_issues_list_tool() -> None:
    """Test linode_managed_issue_list tool schema."""
    tool, capability = create_linode_managed_issue_list_tool()

    assert tool.name == "linode_managed_issue_list"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert "required" not in tool.input_schema
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


async def test_handle_linode_managed_issues_list(sample_config: Config) -> None:
    """Test linode_managed_issue_list tool emits the proto envelope."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "entity": {"label": "web-1"}}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issue_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        # Proto-canonical {count, managed_issues}: created is the always-present
        # string, services the always-present repeated list, and the nested entity
        # message is emitted (present in the raw) with its default sub-fields.
        assert json.loads(result[0].text) == {
            "count": 1,
            "managed_issues": [
                {
                    "id": 1,
                    "created": "",
                    "services": [],
                    "entity": {"id": 0, "label": "web-1", "type": "", "url": ""},
                }
            ],
        }
        mock_client.route_raw.assert_awaited_once_with(
            "linode_managed_issue_list", query="page=1&page_size=25"
        )


async def test_handle_linode_managed_issues_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_issue_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_issue_list({"page": 0}, sample_config)

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("page_size", "expected"),
    [
        (24, "page_size must be an integer from 25 through 500"),
        (501, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_handle_linode_managed_issues_list_rejects_invalid_page_size(
    sample_config: Config, page_size: int, expected: str
) -> None:
    """Test linode_managed_issue_list rejects out-of-range page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_issue_list(
            {"page_size": page_size}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_linode_settings_list_tool() -> None:
    """Test linode_managed_linode_settings_list tool schema."""
    tool, capability = create_linode_managed_linode_settings_list_tool()

    assert tool.name == "linode_managed_linode_settings_list"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert "required" not in tool.input_schema
    assert "environment" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_linode_settings_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    # Proto-canonical {count, managed_linode_settings}: id/label/group present;
    # the nested ssh message is omitted because the raw element lacks it.
    assert json.loads(result[0].text) == {
        "count": 1,
        "managed_linode_settings": [{"id": 123, "label": "web-1", "group": "prod"}],
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_managed_linode_settings_list", query="page=2&page_size=25"
    )


async def test_handle_linode_managed_linode_settings_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects invalid page."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page": 0}, sample_config
        )

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_linode_settings_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects bad page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page_size": 501}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_list_rejects_non_integer_page(
    sample_config: Config,
) -> None:
    """Managed service list validates pagination before any client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_list(
            {"page": "two"}, sample_config
        )

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_linode_settings_list_rejects_low_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_linode_settings_list rejects low page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_linode_settings_list(
            {"page_size": 24}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_credentials_list_tool() -> None:
    """Test linode_managed_credential_list tool schema."""
    tool, capability = create_linode_managed_credential_list_tool()

    assert tool.name == "linode_managed_credential_list"
    assert capability == Capability.Read
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


async def test_handle_linode_managed_credentials_list(sample_config: Config) -> None:
    """Test linode_managed_credential_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"id": 1, "label": "credential"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_credential_list(
            {"page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        # Proto-canonical {count, managed_credentials}: the element emits id/label
        # plus the always-present last_decrypted string; the secret material is
        # never in the list body.
        assert json.loads(result[0].text) == {
            "count": 1,
            "managed_credentials": [
                {"id": 1, "label": "credential", "last_decrypted": ""}
            ],
        }
        mock_client.route_raw.assert_awaited_once_with(
            "linode_managed_credential_list", query="page=1&page_size=25"
        )


async def test_handle_linode_managed_credentials_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_managed_credential_list rejects invalid pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_list({"page": 0}, sample_config)

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_credentials_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Test linode_managed_credential_list rejects invalid page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_credential_list(
            {"page_size": 501}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_managed_ssh_key_get_tool() -> None:
    """Test linode_managed_sshkey_get tool schema."""
    tool, capability = create_linode_managed_sshkey_get_tool()

    assert tool.name == "linode_managed_sshkey_get"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert "required" not in tool.input_schema


async def test_handle_linode_managed_ssh_key_get(sample_config: Config) -> None:
    """Managed SSH key get emits {ssh_key} proto-canonically (unknown fields drop)."""
    response_data: dict[str, Any] = {
        "ssh_key": "ssh-rsa AAAAmanagedkey linode-managed",
        "not_in_proto": "dropped",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_sshkey_get({}, sample_config)

    assert len(result) == 1
    assert json.loads(result[0].text) == {
        "ssh_key": "ssh-rsa AAAAmanagedkey linode-managed"
    }
    mock_client.route_raw.assert_awaited_once_with("linode_managed_sshkey_get")


async def test_handle_linode_managed_ssh_key_get_propagates_errors(
    sample_config: Config,
) -> None:
    """Test linode_managed_sshkey_get reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_sshkey_get({}, sample_config)

    assert "Failed to retrieve the managed SSH key" in result[0].text
    assert "boom" in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_managed_sshkey_get")


async def test_create_linode_managed_issue_get_tool() -> None:
    """Test linode_managed_issue_get tool schema."""
    tool, capability = create_linode_managed_issue_get_tool()

    assert tool.name == "linode_managed_issue_get"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert tool.input_schema["required"] == ["issue_id"]
    assert tool.input_schema["properties"]["issue_id"]["type"] == "integer"


async def test_handle_linode_managed_issue_get(sample_config: Config) -> None:
    """Test linode_managed_issue_get tool."""
    response_data: dict[str, Any] = {"id": 77, "entity": {"label": "web-1"}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issue_get({"issue_id": 77}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == 77
        assert data["entity"]["label"] == "web-1"
        assert data["services"] == []
        mock_client.route_raw.assert_awaited_once_with("linode_managed_issue_get", 77)


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
    if arguments:
        assert "issue_id must be a positive integer" in result[0].text
    else:
        # An absent id is told apart from an unusable one by the bounded reader.
        assert "issue_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_issue_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed issue handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_issue_get({"issue_id": 77}, sample_config)

    assert len(result) == 1
    assert "Failed to retrieve managed issue 77: boom" in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_managed_issue_get", 77)


async def test_create_linode_managed_contact_get_tool() -> None:
    """Test linode_managed_contact_get tool schema."""
    tool, capability = create_linode_managed_contact_get_tool()

    assert tool.name == "linode_managed_contact_get"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert tool.input_schema["required"] == ["contact_id"]
    assert "contact_id" in tool.input_schema["properties"]


async def test_handle_linode_managed_contact_get(sample_config: Config) -> None:
    """Test linode_managed_contact_get tool."""
    response_data: dict[str, Any] = {"id": 42, "name": "Primary on-call", "phone": {}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_get(
            {"contact_id": 42}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == 42
        assert body["name"] == "Primary on-call"
        assert "group" not in body
        assert body["phone"] == {}
        mock_client.route_raw.assert_awaited_once_with("linode_managed_contact_get", 42)


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
    if arguments:
        assert "contact_id must be a positive integer" in result[0].text
    else:
        # An absent id is told apart from an unusable one by the bounded reader.
        assert "contact_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contact_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed contact handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_get(
            {"contact_id": 42}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve managed contact 42: boom" in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_managed_contact_get", 42)


async def test_create_linode_managed_service_get_tool() -> None:
    """Test linode_managed_service_get tool schema."""
    tool, capability = create_linode_managed_service_get_tool()

    assert tool.name == "linode_managed_service_get"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert tool.input_schema["required"] == ["service_id"]
    assert "service_id" in tool.input_schema["properties"]
    assert "confirm" not in tool.input_schema["properties"]


async def test_handle_linode_managed_service_get(sample_config: Config) -> None:
    """Test linode_managed_service_get tool."""
    response_data: dict[str, Any] = {"id": 314, "label": "web monitor"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_service_get(
            {"service_id": 314}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == 314
        assert body["label"] == "web monitor"
        assert body["credentials"] == []
        mock_client.route_raw.assert_awaited_once_with(
            "linode_managed_service_get", 314
        )


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
    if arguments:
        assert "service_id must be a positive integer" in result[0].text
    else:
        # An absent id is told apart from an unusable one by the bounded reader.
        assert "service_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test Managed service handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_service_get(
            {"service_id": 314}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve managed service 314: boom" in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_managed_service_get", 314)


async def test_create_linode_account_beta_get_tool() -> None:
    """Test linode_account_beta_get tool schema."""
    tool, capability = create_linode_account_beta_get_tool()

    assert tool.name == "linode_account_beta_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["beta_id"]
    assert tool.input_schema["properties"]["beta_id"]["type"] == "string"


async def test_handle_linode_account_beta_get(sample_config: Config) -> None:
    """Test linode_account_beta_get tool."""
    response_data = {"id": "example-open", "label": "Example Open Beta"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_beta_get(
            {"beta_id": "example-open"}, sample_config
        )

    data = json.loads(result[0].text)
    assert data["id"] == "example-open"
    assert data["label"] == "Example Open Beta"
    assert "description" not in data
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_beta_get", "example-open"
    )


async def test_handle_linode_account_beta_get_requires_beta_id(
    sample_config: Config,
) -> None:
    """Account beta get requires beta_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get({}, sample_config)

    assert "beta_id is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments", [{"beta_id": 123}, {"beta_id": ""}, {"beta_id": "   "}]
)
async def test_handle_linode_account_beta_get_rejects_non_string_beta_id(
    arguments: dict[str, Any], sample_config: Config
) -> None:
    """Account beta get rejects non-string or blank beta_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get(arguments, sample_config)

    assert "beta_id must be a non-empty string" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("beta_id", ["example/open", "example?open", ".."])
async def test_handle_linode_account_beta_get_rejects_malformed_beta_id(
    beta_id: str, sample_config: Config
) -> None:
    """Account beta get rejects charset-invalid beta_id values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_get(
            {"beta_id": beta_id}, sample_config
        )

    assert "beta_id must contain only" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_account_settings_get_tool() -> None:
    """Test linode_account_settings_get tool schema."""
    tool, capability = create_linode_account_settings_get_tool()

    assert tool.name == "linode_account_settings_get"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {"environment"}
    assert "required" not in tool.input_schema


async def test_handle_linode_account_settings_get(sample_config: Config) -> None:
    """Test linode_account_settings_get tool."""
    response_data: dict[str, Any] = {
        "backups_enabled": True,
        "managed": False,
        "network_helper": True,
        "longview_subscription": None,
        "object_storage": "akamai",
        "interfaces_for_new_linodes": "legacy_config",
        "maintenance_policy": "linode/migrate",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_get({}, sample_config)

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["backups_enabled"] is True
    assert body["object_storage"] == "akamai"
    assert body["interfaces_for_new_linodes"] == "legacy_config"
    assert body["maintenance_policy"] == "linode/migrate"
    assert "longview_subscription" not in body
    mock_client.route_raw.assert_awaited_once_with("linode_account_settings_get")


async def test_create_linode_account_maintenance_list_tool() -> None:
    """Test linode_account_maintenance_list tool schema."""
    tool, capability = create_linode_account_maintenance_list_tool()

    assert tool.name == "linode_account_maintenance_list"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "page",
        "page_size",
    }
    assert "required" not in tool.input_schema


async def test_handle_linode_account_maintenance_list(sample_config: Config) -> None:
    """Test linode_account_maintenance_list tool returns the proto envelope."""
    response_data: dict[str, Any] = {
        "data": [{"entity": {"id": 123, "type": "linode"}, "status": "pending"}],
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_maintenance_list({}, sample_config)

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "filter" not in payload
    element = payload["account_maintenances"][0]
    assert element["status"] == "pending"
    assert element["entity"]["id"] == 123
    assert element["entity"]["type"] == "linode"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_maintenance_list", query=""
    )


async def test_handle_linode_account_maintenance_list_rejects_bad_page(
    sample_config: Config,
) -> None:
    """Invalid pagination short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_maintenance_list(
            {"page": "abc"}, sample_config
        )

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_notification_list_returns_proto_envelope(
    sample_config: Config,
) -> None:
    """Notification list wraps the raw page in the proto envelope."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "label": "Scheduled maintenance",
                "message": "Maintenance is scheduled for a Linode.",
                "severity": "major",
                "type": "maintenance",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_notification_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "page" not in payload
    element = payload["account_notifications"][0]
    assert element["label"] == "Scheduled maintenance"
    assert element["severity"] == "major"


async def test_handle_linode_account_notification_list_rejects_bad_page_size(
    sample_config: Config,
) -> None:
    """Invalid page_size short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_notification_list(
            {"page_size": 1}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_payment_method_list_returns_proto_envelope(
    sample_config: Config,
) -> None:
    """Payment method list wraps the raw page in the proto envelope.

    The credit-card data sub-object is a google.protobuf.Struct, so it
    round-trips intact; the list only ever returns the masked last_four.
    """
    response_data: dict[str, Any] = {
        "data": [
            {
                "id": 321,
                "type": "credit_card",
                "is_default": True,
                "data": {"card_type": "Visa", "last_four": "1111"},
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "page" not in payload
    element = payload["account_payment_methods"][0]
    assert element["id"] == 321
    assert element["is_default"] is True
    assert element["data"]["last_four"] == "1111"


async def test_handle_linode_account_payment_method_list_rejects_bad_page(
    sample_config: Config,
) -> None:
    """Invalid page short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_payment_method_list(
            {"page": 0}, sample_config
        )

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_child_account_list_returns_proto_envelope(
    sample_config: Config,
) -> None:
    """Child account list wraps the raw page in the proto envelope.

    The credit_card sub-object carries only the masked expiry and last_four.
    """
    response_data: dict[str, Any] = {
        "data": [
            {
                "euuid": "A1BC2DEF-3456-7890-ABCD-EF1234567890",
                "company": "Child Co",
                "credit_card": {"expiry": "11/2026", "last_four": "1111"},
                "capabilities": ["Linodes"],
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_child_account_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "page" not in payload
    element = payload["account_child_accounts"][0]
    assert element["euuid"] == "A1BC2DEF-3456-7890-ABCD-EF1234567890"
    assert element["credit_card"]["last_four"] == "1111"
    assert element["capabilities"] == ["Linodes"]


async def test_handle_linode_account_child_account_list_rejects_bad_page_size(
    sample_config: Config,
) -> None:
    """Invalid page_size short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_child_account_list(
            {"page_size": 1}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_invoice_item_list_rejects_bad_page_size(
    sample_config: Config,
) -> None:
    """Invalid page_size short-circuits after the invoice_id check."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_invoice_item_list(
            {"invoice_id": 123, "page_size": 1}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_beta_list_rejects_non_integer_page(
    sample_config: Config,
) -> None:
    """Non-integer page raises in pagination parsing before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_beta_list({"page": "two"}, sample_config)

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_ipv6_range_list_returns_count_envelope(
    sample_config: Config,
) -> None:
    """IPv6 range list wraps the data page in a count envelope."""
    response_data: dict[str, Any] = {
        "data": [{"range": "2600:3c00::/64", "region": "us-east"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_ipv6_range_list(
            {"page": 1, "page_size": 25}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["ipv6_ranges"][0]["range"] == "2600:3c00::/64"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_ipv6_range_list", query="page=1&page_size=25"
    )


async def test_handle_linode_ipv6_range_list_rejects_non_integer_page(
    sample_config: Config,
) -> None:
    """Non-integer page short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_ipv6_range_list({"page": "x"}, sample_config)

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_ipv6_pool_list_returns_proto_envelope(
    sample_config: Config,
) -> None:
    """IPv6 pool list decodes the data page into the proto count envelope."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "range": "2600:3c03::/64",
                "region": "us-east",
                "prefix": 64,
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_ipv6_pool_list(
            {"page": 1, "page_size": 25}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    pool = payload["ipv6_pools"][0]
    assert pool["range"] == "2600:3c03::/64"
    assert pool["region"] == "us-east"
    assert pool["prefix"] == 64
    mock_client.route_raw.assert_awaited_once_with(
        "linode_ipv6_pool_list", query="page=1&page_size=25"
    )


async def test_handle_linode_ipv6_pool_list_rejects_non_integer_page(
    sample_config: Config,
) -> None:
    """Non-integer page short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_ipv6_pool_list({"page": "x"}, sample_config)

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_ipv6_pool_list_rejects_page_below_minimum(
    sample_config: Config,
) -> None:
    """A page below the minimum is rejected before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_ipv6_pool_list({"page": 0}, sample_config)

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_ipv6_range_list_rejects_page_size_above_maximum(
    sample_config: Config,
) -> None:
    """A page_size above the maximum is rejected before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_ipv6_range_list({"page_size": 501}, sample_config)

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_firewall_template_list_emits_nested_rules(
    sample_config: Config,
) -> None:
    """Firewall template list decodes the nested rules ruleset into the proto."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "slug": "public",
                "rules": {
                    "inbound": [
                        {
                            "action": "ACCEPT",
                            "protocol": "TCP",
                            "ports": "443",
                            "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                            "label": "allow-https",
                            "description": "Allow HTTPS",
                        }
                    ],
                    "inbound_policy": "DROP",
                    "outbound": [],
                    "outbound_policy": "ACCEPT",
                },
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_template_list(
            {"page": 1, "page_size": 25}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    template = payload["firewall_templates"][0]
    assert template["slug"] == "public"
    rules = template["rules"]
    assert rules["inbound_policy"] == "DROP"
    assert rules["outbound"] == []
    assert rules["inbound"][0]["ports"] == "443"
    assert rules["inbound"][0]["addresses"]["ipv4"] == ["0.0.0.0/0"]
    mock_client.route_raw.assert_awaited_once_with(
        "linode_firewall_template_list", query="page=1&page_size=25"
    )


async def test_handle_linode_network_transfer_price_list_reuses_linode_type(
    sample_config: Config,
) -> None:
    """Network transfer price list reuses the shared LinodeType element shape."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "id": "distributed_network_transfer",
                "label": "Distributed Network Transfer",
                "price": {"hourly": 0.01, "monthly": 0.0},
                "region_prices": [{"id": "id-cgk", "hourly": 0.015, "monthly": 0.0}],
                "transfer": 0,
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_network_transfer_price_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    price = payload["network_transfer_prices"][0]
    assert price["id"] == "distributed_network_transfer"
    assert price["price"] == {"hourly": 0.01, "monthly": 0.0}
    assert price["region_prices"][0]["id"] == "id-cgk"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_network_transfer_price_list", query=""
    )


async def test_handle_linode_account_service_transfer_list_returns_envelope(
    sample_config: Config,
) -> None:
    """Service transfer list wraps the raw page in the proto envelope."""
    response_data: dict[str, Any] = {
        "data": [
            {
                "token": "abc-123",
                "status": "pending",
                "is_sender": True,
                "entities": {"linodes": [111, 222]},
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_service_transfer_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    element = payload["account_service_transfers"][0]
    assert element["token"] == "abc-123"
    assert element["is_sender"] is True
    assert element["entities"]["linodes"] == [111, 222]


async def test_handle_linode_account_service_transfer_list_rejects_bad_page(
    sample_config: Config,
) -> None:
    """Invalid pagination short-circuits before the client is constructed."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_service_transfer_list(
            {"page": 0}, sample_config
        )

    assert "page must be an integer greater than or equal to 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_maintenance_policies_list_tool() -> None:
    """Test linode_maintenance_policy_list tool schema."""
    tool, capability = create_linode_maintenance_policy_list_tool()

    assert tool.name == "linode_maintenance_policy_list"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "page",
        "page_size",
    }
    assert "required" not in tool.input_schema


async def test_handle_linode_maintenance_policies_list(sample_config: Config) -> None:
    """Test linode_maintenance_policy_list tool emits the proto list envelope."""
    response_data: dict[str, Any] = {
        "data": [{"slug": "linode/migrate", "label": "Migrate"}],
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_maintenance_policy_list({}, sample_config)

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["maintenance_policies"][0]["slug"] == "linode/migrate"
    assert payload["maintenance_policies"][0]["label"] == "Migrate"
    assert "data" not in payload
    mock_client.route_raw.assert_awaited_once_with(
        "linode_maintenance_policy_list", query=""
    )


async def test_handle_linode_maintenance_policy_list_rejects_bad_page(
    sample_config: Config,
) -> None:
    """Maintenance policy list validates pagination before any client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_maintenance_policy_list(
            {"page": "x"}, sample_config
        )

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_create_linode_account_availability_list_tool() -> None:
    """Test linode_account_availability_list tool schema."""
    tool, capability = create_linode_account_availability_list_tool()

    assert tool.name == "linode_account_availability_list"
    assert capability is Capability.Read
    assert "page" not in tool.input_schema.get("required", [])
    assert "page_size" not in tool.input_schema.get("required", [])


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": "2"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": False}, "page_size must be an integer"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
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
        "data": [
            {
                "region": "us-east",
                "available": ["Linodes", "NodeBalancers"],
                "unavailable": ["Kubernetes"],
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_availability_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["account_availabilities"][0]["region"] == "us-east"
    assert body["account_availabilities"][0]["unavailable"] == ["Kubernetes"]
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_availability_list", query="page=2&page_size=25"
    )


async def test_create_linode_account_tags_list_tool() -> None:
    """Test linode_tag_list tool schema."""
    tool, capability = create_linode_tag_list_tool()

    assert tool.name == "linode_tag_list"
    assert capability is Capability.Read
    assert "page" not in tool.input_schema.get("required", [])
    assert "page_size" not in tool.input_schema.get("required", [])


async def test_handle_linode_account_tags_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account tag listing validates page."""
    result = await handle_linode_tag_list({"page": 0}, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_account_tags_list(sample_config: Config) -> None:
    """Test linode_tag_list tool."""
    response_data: dict[str, Any] = {
        "data": [{"label": "production"}, {"label": "web"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_tag_list(
            {"page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "count": 2,
            "tags": [
                {
                    "label": "production",
                    "domains": [],
                    "linodes": [],
                    "nodebalancers": [],
                    "volumes": [],
                },
                {
                    "label": "web",
                    "domains": [],
                    "linodes": [],
                    "nodebalancers": [],
                    "volumes": [],
                },
            ],
        }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_tag_list", query="page=2&page_size=25"
    )


async def test_create_linode_account_tag_objects_list_tool() -> None:
    """Test linode_tag_object_list tool schema."""
    tool, capability = create_linode_tag_object_list_tool()

    assert tool.name == "linode_tag_object_list"
    assert capability is Capability.Read
    assert "tag_label" in tool.input_schema["required"]
    assert "page" not in tool.input_schema["required"]


async def test_handle_linode_account_tag_objects_list_requires_label(
    sample_config: Config,
) -> None:
    """Tagged object listing requires a non-empty tag label."""
    result = await handle_linode_tag_object_list({}, sample_config)

    assert len(result) == 1
    assert "tag_label" in result[0].text


async def test_handle_linode_account_tag_objects_list_rejects_path_unsafe_label(
    sample_config: Config,
) -> None:
    """Tagged object listing rejects a path-unsafe tag_label (matches Go)."""
    result = await handle_linode_tag_object_list(
        {"tag_label": "bad#tag"}, sample_config
    )

    assert len(result) == 1
    assert "tag_label must not contain '?', '#', or '..'" in result[0].text


async def test_handle_linode_account_tag_objects_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Tagged object listing validates page."""
    result = await handle_linode_tag_object_list(
        {"tag_label": "production", "page": 0}, sample_config
    )

    assert len(result) == 1
    assert "page" in result[0].text


async def test_handle_linode_account_tag_objects_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Tagged object listing validates page_size."""
    result = await handle_linode_tag_object_list(
        {"tag_label": "production", "page_size": 10}, sample_config
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_tag_objects_list(sample_config: Config) -> None:
    """Test linode_tag_object_list tool."""
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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_tag_object_list(
            {"tag_label": "production", "page": 2, "page_size": 25},
            sample_config,
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload == {
            "count": 1,
            "tagged_objects": [
                {
                    "type": "linode",
                    "label": "",
                    "data": {"id": 123, "label": "web-1"},
                }
            ],
        }
        assert "data" not in payload
        assert "page" not in payload
        mock_client.route_raw.assert_awaited_once_with(
            "linode_tag_object_list", "production", query="page=2&page_size=25"
        )


async def test_create_linode_account_tag_create_tool() -> None:
    """Test linode_tag_create tool schema."""
    tool, capability = create_linode_tag_create_tool()

    assert tool.name == "linode_tag_create"
    assert capability is Capability.Write
    assert "label" in tool.input_schema["required"]
    assert "confirm" in tool.input_schema["required"]
    assert "linodes" not in tool.input_schema["required"]


async def test_handle_linode_account_tag_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Tag creation requires confirmation."""
    result = await handle_linode_tag_create(
        {"label": "production", "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_tag_create_rejects_non_boolean_confirm(
    sample_config: Config,
) -> None:
    """Tag creation requires confirm to be true boolean."""
    result = await handle_linode_tag_create(
        {"confirm": "yes", "label": "production"}, sample_config
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_linode_account_tag_create_requires_label(
    sample_config: Config,
) -> None:
    """Tag creation requires a non-empty label."""
    result = await handle_linode_tag_create(
        {"confirm": True, "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "label" in result[0].text


async def test_handle_linode_account_tag_create_rejects_blank_label(
    sample_config: Config,
) -> None:
    """Tag creation rejects a blank label."""
    result = await handle_linode_tag_create(
        {"confirm": True, "label": "   ", "linodes": [123]}, sample_config
    )

    assert len(result) == 1
    assert "label" in result[0].text


async def test_handle_linode_account_tag_create_rejects_invalid_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation validates resource ID lists."""
    result = await handle_linode_tag_create(
        {"confirm": True, "label": "production", "linodes": [123, "bad"]}, sample_config
    )

    assert len(result) == 1
    assert "linodes" in result[0].text


async def test_handle_linode_account_tag_create_rejects_non_list_resource_ids(
    sample_config: Config,
) -> None:
    """A scalar passed where a list of IDs is expected is rejected."""
    result = await handle_linode_tag_create(
        {"confirm": True, "label": "production", "linodes": "123"}, sample_config
    )

    assert len(result) == 1
    assert "linodes must be an array of integers" in result[0].text


async def test_handle_linode_account_tag_create_rejects_non_positive_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation rejects non-positive resource IDs."""
    result = await handle_linode_tag_create(
        {"confirm": True, "label": "production", "linodes": [0]}, sample_config
    )

    assert len(result) == 1
    assert "positive integers" in result[0].text


async def test_handle_linode_account_tag_create_rejects_boolean_resource_ids(
    sample_config: Config,
) -> None:
    """Tag creation rejects boolean resource IDs."""
    result = await handle_linode_tag_create(
        {"confirm": True, "label": "production", "volumes": [True]}, sample_config
    )

    assert len(result) == 1
    assert "volumes" in result[0].text


async def test_handle_linode_account_tag_create(sample_config: Config) -> None:
    """Test linode_tag_create tool."""
    response_data: dict[str, Any] = {"label": "production"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_tag_create(
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
            "tag": {
                "label": "production",
                "domains": [],
                "linodes": [],
                "nodebalancers": [],
                "volumes": [],
            },
        }
        mock_client.route_raw.assert_awaited_once_with(
            "linode_tag_create",
            body={
                "label": "production",
                "domains": [1],
                "linodes": [2],
                "nodebalancers": [3],
                "volumes": [4],
            },
        )


async def test_handle_linode_account_tag_create_omits_empty_resource_lists(
    sample_config: Config,
) -> None:
    """Tag creation omits empty resource lists."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"label": "production"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        await handle_linode_tag_create(
            {"confirm": True, "label": "production", "linodes": []}, sample_config
        )

        # A list the caller supplied empty travels as an empty list: the shared
        # body reader treats it as a value that clears the field, where the
        # hand-written handlers dropped it.
        mock_client.route_raw.assert_awaited_once_with(
            "linode_tag_create",
            body={"label": "production", "linodes": []},
        )


async def test_handle_linode_account_tag_create_reports_client_errors(
    sample_config: Config,
) -> None:
    """Tag creation reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_tag_create(
            {"confirm": True, "label": "production"}, sample_config
        )

        assert len(result) == 1
        assert "Failed to create tag" in result[0].text


async def test_account_tag_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account tag create tool should be exported and registered."""
    from linodemcp import gentools as gentools_mod

    assert "create_linode_tag_create_tool" in gentools_mod.__all__
    assert "handle_linode_tag_create" in gentools_mod.__all__

    from linodemcp.server import get_tool_registry

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_tag_create"].capability is Capability.Write


async def test_create_linode_account_support_ticket_get_tool() -> None:
    """Test linode_support_ticket_get tool schema."""
    tool, capability = create_linode_support_ticket_get_tool()

    assert tool.name == "linode_support_ticket_get"
    assert capability is Capability.Read
    assert "ticket_id" in tool.input_schema["required"]


async def test_create_linode_account_support_tickets_list_tool() -> None:
    """Test linode_support_ticket_list tool schema."""
    tool, capability = create_linode_support_ticket_list_tool()

    assert tool.name == "linode_support_ticket_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]


async def test_handle_linode_account_support_tickets_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Support ticket listing validates page_size."""
    result = await handle_linode_support_ticket_list({"page_size": 10}, sample_config)

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_support_tickets_list(
    sample_config: Config,
) -> None:
    """Test linode_support_ticket_list emits the proto list envelope."""
    response_data: dict[str, Any] = {
        "data": [{"id": 789, "summary": "Need help", "opened_by": "alice"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_list(
            {"page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["support_tickets"][0]["id"] == 789
        assert payload["support_tickets"][0]["summary"] == "Need help"
        assert payload["support_tickets"][0]["opened_by"] == "alice"
        # paginated list never echoes a filter
        assert "filter" not in payload
        # the raw page envelope (page/pages/results) is dropped for the contract
        assert "page" not in payload
    mock_client.route_raw.assert_awaited_once_with(
        "linode_support_ticket_list", query="page=2&page_size=25"
    )


async def test_handle_linode_account_support_tickets_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support tickets list handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to retrieve items" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_support_ticket_get_requires_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval requires a positive ticket_id."""
    result = await handle_linode_support_ticket_get({}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get_rejects_bad_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval rejects invalid ticket IDs."""
    result = await handle_linode_support_ticket_get({"ticket_id": 0}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get_rejects_bool_id(
    sample_config: Config,
) -> None:
    """Support ticket retrieval rejects bool ticket IDs."""
    result = await handle_linode_support_ticket_get({"ticket_id": True}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_get(
    sample_config: Config,
) -> None:
    """Test linode_support_ticket_get tool."""
    response_data: dict[str, Any] = {"id": 123, "summary": "Need help"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_get(
            {"ticket_id": 123}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == 123
        assert body["summary"] == "Need help"
        assert body["attachments"] == []
        assert body["closable"] is False
        assert "closed" not in body
        assert "entity" not in body
        mock_client.route_raw.assert_awaited_once_with("linode_support_ticket_get", 123)


async def test_handle_linode_account_support_ticket_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support ticket get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_get(
            {"ticket_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve support ticket" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_oauth_client_get_tool() -> None:
    """Test linode_account_oauth_client_get tool schema."""
    tool, capability = create_linode_account_oauth_client_get_tool()

    assert tool.name == "linode_account_oauth_client_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["client_id"]
    assert tool.input_schema["properties"]["client_id"]["type"] == "string"


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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_get(
            {"client_id": "client-123"}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == "client-123"
        assert body["label"] == "Example OAuth Client"
        assert body["public"] is False
        assert body["thumbnail_url"] == ""
        mock_client.route_raw.assert_awaited_once_with(
            "linode_account_oauth_client_get", "client-123"
        )


async def test_create_linode_account_payment_method_get_tool() -> None:
    """Test linode_account_payment_method_get tool schema."""
    tool, capability = create_linode_account_payment_method_get_tool()

    assert tool.name == "linode_account_payment_method_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["payment_method_id"]
    assert tool.input_schema["properties"]["payment_method_id"]["type"] == "integer"


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
        ({"payment_method_id": "123"}, "payment_method_id must be a positive integer"),
        ({"payment_method_id": True}, "payment_method_id must be a positive integer"),
        ({"payment_method_id": 0}, "payment_method_id must be a positive integer"),
        ({"payment_method_id": "12/3"}, "payment_method_id must be a positive integer"),
        ({"payment_method_id": "12?3"}, "payment_method_id must be a positive integer"),
        ({"payment_method_id": ".."}, "payment_method_id must be a positive integer"),
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
    """Payment method get emits AccountPaymentMethod proto-canonically."""
    response_data: dict[str, Any] = {
        "id": 123,
        "type": "credit_card",
        "is_default": True,
        "data": {"card_type": "Visa", "last_four": "1234", "expiry": "12/2027"},
        "not_in_proto": "dropped",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_get(
            {"payment_method_id": 123}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "id": 123,
            "type": "credit_card",
            "is_default": True,
            "data": {"card_type": "Visa", "last_four": "1234", "expiry": "12/2027"},
        }
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_account_payment_method_get", 123
        )


async def test_handle_linode_account_payment_method_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test payment method get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_payment_method_get(
            {"payment_method_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve payment method" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_oauth_client_thumbnail_get_tool() -> None:
    """Test linode_account_oauth_client_thumbnail_get tool schema."""
    tool, capability = create_linode_account_oauth_client_thumbnail_get_tool()

    assert tool.name == "linode_account_oauth_client_thumbnail_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["client_id"]
    assert tool.input_schema["properties"]["client_id"]["type"] == "string"


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
    """OAuth client thumbnail retrieval rejects malformed client IDs.

    Every one answers before a client is opened, which is what keeps a bad id
    off the wire rather than into a path segment.
    """
    for bad_client_id in (123, "   ", "client/id", "client?id", ".."):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_account_oauth_client_thumbnail_get(
                {"client_id": bad_client_id}, sample_config
            )

        assert len(result) == 1
        assert "client_id" in result[0].text
        mock_client_class.assert_not_called()


async def test_handle_linode_account_oauth_client_thumbnail_get(
    sample_config: Config,
) -> None:
    """Thumbnail get serializes {client_id, thumbnail_png_base64} proto-canonically."""
    # The client base64-encodes the raw PNG under thumbnail_png_base64; the
    # handler stitches in client_id and serializes through OAuthClientThumbnail,
    # so any extra key the client returns must drop.
    response_data: dict[str, Any] = {
        "thumbnail_png_base64": "iVBORw0KGgo=",
        "not_in_proto": "dropped",
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
        assert json.loads(result[0].text) == {
            "client_id": "client-123",
            "thumbnail_png_base64": "iVBORw0KGgo=",
        }
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
    assert "Failed to get account OAuth client thumbnail 'client-123'" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_oauth_client_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test OAuth client get handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_oauth_client_get(
            {"client_id": "client-123"}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve OAuth client" in result[0].text
    assert "boom" in result[0].text


async def test_create_linode_account_support_ticket_replies_list_tool() -> None:
    """Test linode_support_ticket_reply_list tool schema."""
    tool, capability = create_linode_support_ticket_reply_list_tool()

    assert tool.name == "linode_support_ticket_reply_list"
    assert capability is Capability.Read
    assert "ticket_id" in tool.input_schema["required"]
    assert "page" not in tool.input_schema["required"]
    assert "page_size" not in tool.input_schema["required"]


async def test_handle_linode_account_support_ticket_replies_list_requires_ticket_id(
    sample_config: Config,
) -> None:
    """Support ticket reply listing requires a positive ticket_id."""
    result = await handle_linode_support_ticket_reply_list({}, sample_config)

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_rejects_bad_id(
    sample_config: Config,
) -> None:
    """Support ticket reply listing rejects invalid ticket IDs."""
    result = await handle_linode_support_ticket_reply_list(
        {"ticket_id": 0}, sample_config
    )

    assert len(result) == 1
    assert "ticket_id" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_rejects_page_size(
    sample_config: Config,
) -> None:
    """Support ticket reply listing validates page_size."""
    result = await handle_linode_support_ticket_reply_list(
        {"ticket_id": 123, "page_size": 10}, sample_config
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list(
    sample_config: Config,
) -> None:
    """Test linode_support_ticket_reply_list tool emits the proto list envelope."""
    response_data: dict[str, Any] = {
        "data": [{"id": 456, "description": "Thanks"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_reply_list(
            {"ticket_id": 123, "page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["support_ticket_replies"][0]["id"] == 456
        assert payload["support_ticket_replies"][0]["description"] == "Thanks"
        assert "page" not in payload
        mock_client.route_raw.assert_awaited_once_with(
            "linode_support_ticket_reply_list", 123, query="page=2&page_size=25"
        )


async def test_create_linode_account_event_get_tool() -> None:
    """Test account event get tool schema."""
    tool, capability = create_linode_account_event_get_tool()

    assert tool.name == "linode_account_event_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["event_id"]


async def test_create_linode_account_invoice_items_list_tool() -> None:
    """Test linode_account_invoice_item_list tool schema."""
    tool, capability = create_linode_account_invoice_item_list_tool()

    assert tool.name == "linode_account_invoice_item_list"
    assert capability is Capability.Read
    assert tool.input_schema.get("required") == ["invoice_id"]
    properties = tool.input_schema.get("properties", {})
    assert properties["invoice_id"]["type"] == "integer"
    assert properties["page_size"]["type"] == "integer"


async def test_handle_linode_account_invoice_items_list(sample_config: Config) -> None:
    """Test linode_account_invoice_item_list tool."""
    response_data = {
        "data": [{"label": "Compute Instance", "amount": 12.34}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_invoice_item_list(
            {"invoice_id": 123, "page": 2, "page_size": 25}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["account_invoice_items"][0]["label"] == "Compute Instance"
    assert payload["account_invoice_items"][0]["amount"] == 12.34
    assert "page" not in payload
    mock_client.route_raw.assert_awaited_once_with(
        "linode_account_invoice_item_list", 123, query="page=2&page_size=25"
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "invoice_id is required"),
        ({"invoice_id": 0}, "invoice_id must be a positive integer"),
        ({"invoice_id": False}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123/456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123?456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": ".."}, "invoice_id must be a positive integer"),
        ({"invoice_id": 123, "page": True}, "page must be an integer"),
        (
            {"invoice_id": 123, "page_size": 501},
            "page_size must be an integer from 25 through 500",
        ),
    ],
)
async def test_handle_linode_account_invoice_items_list_rejects_invalid_arguments(
    arguments: dict[str, Any], expected_error: str, sample_config: Config
) -> None:
    """Account invoice items list validates arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_account_invoice_item_list(arguments, sample_config)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_invoice_items_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Account invoice items list reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_invoice_item_list(
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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_event_get({"event_id": 123}, sample_config)

    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 123
    assert data["action"] == "linode_create"
    assert data["status"] == "finished"
    assert data["seen"] is False
    assert "entity" not in data
    mock_client.route_raw.assert_awaited_once_with("linode_account_event_get", 123)


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
    if event_id is None:
        assert "event_id is required" in result[0].text
    else:
        assert "event_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_account_event_get_reports_client_errors(
    sample_config: Config,
) -> None:
    """Account event get reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_event_get({"event_id": 123}, sample_config)

    assert len(result) == 1
    assert "Failed to retrieve account event" in result[0].text
    assert "boom" in result[0].text


async def test_handle_linode_account_support_ticket_replies_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Test support ticket replies list handler reports client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_support_ticket_reply_list(
            {"ticket_id": 123}, sample_config
        )

    assert len(result) == 1
    assert "Failed to retrieve items" in result[0].text
    assert "boom" in result[0].text


async def test_account_support_ticket_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket get tool should be exported and registered."""
    from linodemcp import gentools as gentools_mod

    assert "create_linode_support_ticket_get_tool" in gentools_mod.__all__
    assert "handle_linode_support_ticket_get" in gentools_mod.__all__

    from linodemcp.server import get_tool_registry

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_support_ticket_get"].capability is Capability.Read


async def test_create_linode_regions_get_tool() -> None:
    """Region get tool is read-only and requires region_id."""
    tool, capability = create_linode_region_get_tool()

    assert tool.name == "linode_region_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["region_id"]


async def test_linode_regions_get_tool_is_exported_and_registered() -> None:
    """Region get tool should be exported and registered."""
    from linodemcp import gentools as gentools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_region_get_tool" in gentools_mod.__all__
    assert "handle_linode_region_get" in gentools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_region_get"].capability is Capability.Read


async def test_handle_linode_regions_get(sample_config: Config) -> None:
    """Test linode_region_get tool: raw API response decoded through the proto."""
    raw_region = {
        "id": "us-east",
        "label": "Newark, NJ",
        "country": "us",
        "capabilities": ["Linodes", "Block Storage"],
        "status": "ok",
        "resolvers": {"ipv4": "192.0.2.1", "ipv6": "2001:db8::1"},
        "site_type": "core",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_region
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_get({"region_id": "us-east"}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == "us-east"
        assert data["label"] == "Newark, NJ"
        assert data["resolvers"] == {
            "ipv4": "192.0.2.1",
            "ipv6": "2001:db8::1",
        }
        mock_client.route_raw.assert_awaited_once_with("linode_region_get", "us-east")


async def test_handle_linode_regions_get_rejects_malformed_region_id(
    sample_config: Config,
) -> None:
    """Region get rejects separators in region_id."""
    for region_id in ("us/east", "us-east?x=1", "../us-east"):
        result = await handle_linode_region_get({"region_id": region_id}, sample_config)

        assert len(result) == 1
        assert "letters, numbers, and hyphens" in result[0].text


async def test_handle_linode_regions_get_requires_region_id(
    sample_config: Config,
) -> None:
    """Region get requires region_id."""
    result = await handle_linode_region_get({}, sample_config)

    assert len(result) == 1
    assert "region_id is required" in result[0].text


async def test_handle_linode_regions_get_error(sample_config: Config) -> None:
    """Test linode_region_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_get({"region_id": "us-east"}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_create_linode_regions_availability_list_tool() -> None:
    """Regions availability list tool is read-only and has no route inputs."""
    tool, capability = create_linode_region_availability_list_tool()

    assert tool.name == "linode_region_availability_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema


async def test_handle_linode_regions_availability_list(sample_config: Config) -> None:
    """Test linode_region_availability_list tool."""
    availability = [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"},
        {"available": False, "plan": "g6-standard-2", "region": "us-west"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": availability}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_availability_list({}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["count"] == 2
        assert "availability" not in data
        assert data["region_availabilities"] == availability
        mock_client.route_raw.assert_awaited_once_with(
            "linode_region_availability_list", query=""
        )


async def test_handle_linode_regions_availability_list_error(
    sample_config: Config,
) -> None:
    """Test linode_region_availability_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_availability_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_create_linode_regions_availability_get_tool() -> None:
    """Region availability tool is read-only and requires region_id."""
    tool, capability = create_linode_region_availability_get_tool()

    assert tool.name == "linode_region_availability_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["region_id"]


async def test_handle_linode_regions_availability_get(sample_config: Config) -> None:
    """Test linode_region_availability_get tool."""
    availability = [
        {
            "available": True,
            "plan": "g6-standard-1",
            "region": "us-east",
            "not_in_proto": "dropped",
        },
        {"available": False, "plan": "g6-standard-2", "region": "us-east"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = availability
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_availability_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["count"] == 2
        assert len(data["region_availabilities"]) == 2
        assert data["region_availabilities"][0]["plan"] == "g6-standard-1"
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_region_availability_get", "us-east", query=""
        )


async def test_handle_linode_regions_availability_get_rejects_malformed_region_id(
    sample_config: Config,
) -> None:
    """Region availability rejects separators in region_id."""
    for region_id in ("us/east", "us-east?x=1", "../us-east"):
        result = await handle_linode_region_availability_get(
            {"region_id": region_id}, sample_config
        )

        assert len(result) == 1
        assert "letters, numbers, and hyphens" in result[0].text


async def test_handle_linode_regions_availability_get_requires_region_id(
    sample_config: Config,
) -> None:
    """Region availability requires region_id."""
    result = await handle_linode_region_availability_get({}, sample_config)

    assert len(result) == 1
    assert "region_id is required" in result[0].text


async def test_handle_linode_regions_availability_get_error(
    sample_config: Config,
) -> None:
    """Test linode_region_availability_get error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_availability_get(
            {"region_id": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_regions_list(sample_config: Config) -> None:
    """Test linode_region_list tool."""
    raw_regions: dict[str, Any] = {
        "data": [
            {
                "id": "us-east",
                "label": "Newark, NJ",
                "country": "us",
                "capabilities": ["Linodes", "Block Storage"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.1", "ipv6": "2001:db8::1"},
                "site_type": "core",
            },
            {
                "id": "eu-west",
                "label": "London, UK",
                "country": "uk",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.2", "ipv6": "2001:db8::2"},
                "site_type": "core",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list({}, sample_config)

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "eu-west" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_region_list", query="")


async def test_handle_linode_regions_list_filter_country(sample_config: Config) -> None:
    """Test linode_region_list tool with country filter."""
    raw_regions: dict[str, Any] = {
        "data": [
            {
                "id": "us-east",
                "label": "Newark, NJ",
                "country": "us",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.1", "ipv6": "2001:db8::1"},
                "site_type": "core",
            },
            {
                "id": "us-west",
                "label": "Fremont, CA",
                "country": "us",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.2", "ipv6": "2001:db8::2"},
                "site_type": "core",
            },
            {
                "id": "eu-west",
                "label": "London, UK",
                "country": "uk",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.3", "ipv6": "2001:db8::3"},
                "site_type": "core",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list({"country": "us"}, sample_config)

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 2
        assert "us-east" in result[0].text
        assert "us-west" in result[0].text
        assert "eu-west" not in result[0].text


def test_linode_kernels_list_tool_schema() -> None:
    """The kernels list tool exposes pagination fields."""
    tool, capability = create_linode_kernel_list_tool()
    assert tool.name == "linode_kernel_list"
    assert capability is Capability.Read
    props: dict[str, Any] = tool.input_schema["properties"]
    # page/page_size convert to int32 proto fields; the generated schema carries
    # int32 bounds instead of the old hand-built minimum:1 / 25-500 refinements.
    assert props["page"]["type"] == "integer"
    assert props["page_size"]["type"] == "integer"
    assert "required" not in tool.input_schema


async def test_handle_linode_kernels_list(sample_config: Config) -> None:
    """Test linode_kernel_list tool."""
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
        mock_client.route_raw.return_value = response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernel_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["kernels"][0]["id"] == "linode/latest-64bit"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_kernel_list", query="page=2&page_size=25"
    )


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
        result = await handle_linode_kernel_list(arguments, sample_config)

    assert len(result) == 1
    assert "page" in result[0].text
    mock_client_class.assert_not_called()


def _type_list_page() -> dict[str, Any]:
    """Return a raw /linode/types page with two full instance-type elements."""
    return {
        "data": [
            {
                "id": "g6-nanode-1",
                "label": "Nanode 1GB",
                "class": "nanode",
                "disk": 25600,
                "memory": 1024,
                "vcpus": 1,
                "gpus": 0,
                "network_out": 1000,
                "transfer": 1000,
                "price": {"hourly": 0.0075, "monthly": 5.0},
                "addons": {"backups": {"price": {"hourly": 0.003, "monthly": 2.0}}},
            },
            {
                "id": "g6-standard-2",
                "label": "Linode 4GB",
                "class": "standard",
                "disk": 81920,
                "memory": 4096,
                "vcpus": 2,
                "gpus": 0,
                "network_out": 4000,
                "transfer": 4000,
                "price": {"hourly": 0.03, "monthly": 20.0},
                "addons": {"backups": {"price": {"hourly": 0.008, "monthly": 5.0}}},
            },
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }


async def test_handle_linode_types_list(sample_config: Config) -> None:
    """Proto-canonical envelope: count plus full InstanceType elements."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = _type_list_page()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_list({}, sample_config)

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 2
        assert "filter" not in body
        assert body["types"][0]["id"] == "g6-nanode-1"
        # The whole element flows through unmodified: fields the old curated
        # handler dropped (gpus/network_out/transfer/addons) are present now.
        assert body["types"][1] == {
            "id": "g6-standard-2",
            "label": "Linode 4GB",
            "class": "standard",
            "disk": 81920,
            "memory": 4096,
            "vcpus": 2,
            "gpus": 0,
            "network_out": 4000,
            "transfer": 4000,
            "price": {"hourly": 0.03, "monthly": 20.0},
            "addons": {"backups": {"price": {"hourly": 0.008, "monthly": 5.0}}},
        }
        mock_client.route_raw.assert_awaited_once_with("linode_type_list", query="")


async def test_handle_linode_types_list_filter_class(sample_config: Config) -> None:
    """Class filter keeps matching elements and echoes the applied filter."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = _type_list_page()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_list({"class": "standard"}, sample_config)

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert body["filter"] == "class=standard"
        assert body["types"][0]["id"] == "g6-standard-2"
        assert "g6-nanode-1" not in result[0].text


async def test_handle_linode_type_get(sample_config: Config) -> None:
    """Type get emits the InstanceType proto-canonically (unknown fields drop)."""
    raw_type: dict[str, Any] = {
        "id": "g6-nanode-1",
        "label": "Nanode 1GB",
        "class": "nanode",
        "disk": 25600,
        "memory": 1024,
        "vcpus": 1,
        "gpus": 0,
        "network_out": 1000,
        "transfer": 1000,
        "price": {"hourly": 0.0075, "monthly": 5.0},
        "addons": {"backups": {"price": {"hourly": 0.003, "monthly": 2.0}}},
        "not_in_proto": "dropped",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_type
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_get({"type_id": "g6-nanode-1"}, sample_config)

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == "g6-nanode-1"
        assert data["label"] == "Nanode 1GB"
        assert data["price"] == {"hourly": 0.0075, "monthly": 5.0}
        assert "successor" not in data
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_type_get", "g6-nanode-1")


async def test_handle_linode_type_get_includes_successor(
    sample_config: Config,
) -> None:
    """A type with a successor includes the successor field in the response."""
    raw_type: dict[str, Any] = {
        "id": "g6-standard-2",
        "label": "Linode 4GB",
        "class": "standard",
        "disk": 81920,
        "memory": 4096,
        "vcpus": 2,
        "gpus": 0,
        "network_out": 4000,
        "transfer": 4000,
        "price": {"hourly": 0.036, "monthly": 24.0},
        "addons": {"backups": {"price": {"hourly": 0.008, "monthly": 5.0}}},
        "successor": "g7-standard-2",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_type
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_get(
            {"type_id": "g6-standard-2"}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["successor"] == "g7-standard-2"


async def test_handle_linode_type_get_rejects_malformed_type_id(
    sample_config: Config,
) -> None:
    """Type get rejects separators in type_id before client creation."""
    for type_id in ("g6/nanode-1", "g6-nanode-1?x=1", "../g6-nanode-1"):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_type_get({"type_id": type_id}, sample_config)

        assert len(result) == 1
        assert "type_id must not contain '/', '?', '#', or '..'" in result[0].text
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
    assert "type_id must be a non-empty string" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_type_get_error(sample_config: Config) -> None:
    """Test linode_type_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_get({"type_id": "g6-nanode-1"}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve Linode type: " in result[0].text


async def test_handle_linode_volume_get(sample_config: Config) -> None:
    """Volume get emits the VolumeGetResponse envelope (unknown fields drop)."""
    raw_volume: dict[str, Any] = {
        "id": 12345,
        "label": "data-vol",
        "status": "active",
        "size": 100,
        "region": "us-east",
        "linode_id": 123,
        "linode_label": "test-instance",
        "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
        "tags": ["production"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-02T00:00:00",
        "hardware_type": "nvme",
        "not_in_proto": "dropped",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_get({"volume_id": 12345}, sample_config)

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["volume"]["label"] == "data-vol"
        assert body["volume"]["id"] == 12345
        assert body["volume"]["linode_id"] == 123
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_volume_get", 12345)


async def test_handle_linode_volume_get_requires_volume_id(
    sample_config: Config,
) -> None:
    """Test linode_volume_get validates volume_id."""
    result = await handle_linode_volume_get({}, sample_config)

    assert len(result) == 1
    assert "volume_id is required" in result[0].text


async def test_handle_linode_volume_types_list(sample_config: Config) -> None:
    """Test linode_volume_type_list tool."""
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
        mock_client.route_raw.return_value = {"data": volume_types}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_type_list({}, sample_config)

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert "filter" not in body
        assert body["volume_types"][0] == {
            "id": "volume",
            "label": "Storage Volume",
            "price": {"hourly": 0.0015, "monthly": 0.1},
            "region_prices": [{"id": "us-iad", "hourly": 0.00018, "monthly": 0.12}],
            "transfer": 0,
        }
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_volumes_list(sample_config: Config) -> None:
    """Test linode_volume_list tool."""
    raw_volumes = {
        "data": [
            {
                "id": 1,
                "label": "data-vol",
                "status": "active",
                "size": 100,
                "region": "us-east",
                "linode_id": 123,
                "linode_label": "test-instance",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
                "tags": ["production"],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
            {
                "id": 2,
                "label": "backup-vol",
                "status": "active",
                "size": 50,
                "region": "eu-west",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
                "tags": ["backup"],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_list({}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_volume_list", query="")


async def test_handle_linode_volumes_list_filter_region(sample_config: Config) -> None:
    """Test linode_volume_list tool with region filter."""
    raw_volumes: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "data-vol",
                "status": "active",
                "size": 100,
                "region": "us-east",
                "linode_id": 123,
                "linode_label": "test-instance",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
            {
                "id": 2,
                "label": "backup-vol",
                "status": "active",
                "size": 50,
                "region": "eu-west",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_list({"region": "us-east"}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_create_linode_kernel_get_tool_def() -> None:
    """Kernel get tool should require kernel_id."""
    tool, capability = create_linode_kernel_get_tool()
    assert tool.name == "linode_kernel_get"
    assert capability.name == "Read"
    assert tool.input_schema["required"] == ["kernel_id"]
    assert "kernel_id" in tool.input_schema["properties"]


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
        mock_client.route_raw.return_value = kernel
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernel_get(
            {"kernel_id": "linode/latest-64bit"},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == "linode/latest-64bit"
        assert body["label"] == "Latest 64 bit"
        assert body["kvm"] is True
        assert body["deprecated"] is False
        assert "xen" not in body
        mock_client.route_raw.assert_awaited_once_with(
            "linode_kernel_get", "linode/latest-64bit"
        )


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
        mock_client.route_raw.return_value = kernel
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_kernel_get(
            {"kernel_id": kernel_id},
            sample_config,
        )

    assert json.loads(result[0].text)["id"] == kernel_id
    mock_client.route_raw.assert_awaited_once_with("linode_kernel_get", kernel_id)


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
    assert tool.input_schema["required"] == ["image_id"]
    assert "image_id" in tool.input_schema["properties"]


async def test_handle_linode_image_get_success(sample_config: Config) -> None:
    """Image get should return a single image."""
    raw_image = {
        "id": "linode/ubuntu24.04",
        "label": "Ubuntu 24.04 LTS",
        "description": "Ubuntu image",
        "type": "manual",
        "is_public": True,
        "deprecated": False,
        "size": 2500,
        "vendor": "Ubuntu",
        "status": "available",
        "created": "2024-04-25T00:00:00",
        "created_by": "linode",
        "expiry": None,
        "eol": None,
        "capabilities": ["cloud-init"],
        "tags": [],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_image
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_get(
            {"image_id": "linode/ubuntu24.04"},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["id"] == "linode/ubuntu24.04"
        assert body["label"] == "Ubuntu 24.04 LTS"
        mock_client.route_raw.assert_awaited_once_with(
            "linode_image_get", "linode/ubuntu24.04"
        )


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


async def test_handle_linode_images_list(sample_config: Config) -> None:
    """Test linode_image_list tool."""
    raw_page: dict[str, Any] = {
        "data": [
            {
                "id": "linode/ubuntu22.04",
                "label": "Ubuntu 22.04",
                "description": "Ubuntu 22.04 LTS",
                "type": "manual",
                "is_public": True,
                "deprecated": False,
                "size": 2500,
                "vendor": "linode",
                "status": "available",
                "created": "2022-04-21T00:00:00",
                "created_by": "linode",
                "capabilities": ["cloud-init"],
                "tags": [],
            },
            {
                "id": "private/12345",
                "label": "Custom Image",
                "description": "My custom image",
                "type": "manual",
                "is_public": False,
                "deprecated": False,
                "size": 5000,
                "vendor": "",
                "status": "available",
                "created": "2024-01-01T00:00:00",
                "created_by": "user@example.com",
                "capabilities": [],
                "tags": ["custom"],
            },
        ],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_list({}, sample_config)

        assert len(result) == 1
        assert "linode/ubuntu22.04" in result[0].text
        assert "private/12345" in result[0].text
        assert '"count": 2' in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_image_list", query="")


async def test_handle_linode_images_list_filter_public(sample_config: Config) -> None:
    """Test linode_image_list tool with is_public filter."""
    raw_page: dict[str, Any] = {
        "data": [
            {
                "id": "linode/ubuntu22.04",
                "label": "Ubuntu 22.04",
                "description": "Ubuntu 22.04 LTS",
                "type": "manual",
                "is_public": True,
                "deprecated": False,
                "size": 2500,
                "vendor": "linode",
                "status": "available",
                "created": "2022-04-21T00:00:00",
                "created_by": "linode",
                "capabilities": [],
                "tags": [],
            },
            {
                "id": "private/12345",
                "label": "Custom Image",
                "description": "My custom image",
                "type": "manual",
                "is_public": False,
                "deprecated": False,
                "size": 5000,
                "vendor": "",
                "status": "available",
                "created": "2024-01-01T00:00:00",
                "created_by": "user@example.com",
                "capabilities": [],
                "tags": [],
            },
        ],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_list({"is_public": "false"}, sample_config)

        assert len(result) == 1
        assert "private/12345" in result[0].text
        assert "linode/ubuntu22.04" not in result[0].text
        assert '"count": 1' in result[0].text
        assert '"filter": "is_public=false"' in result[0].text


async def test_handle_linode_account_error(sample_config: Config) -> None:
    """Test linode_account_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_get({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_regions_list_error(sample_config: Config) -> None:
    """Test linode_region_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_types_list_error(sample_config: Config) -> None:
    """Test linode_type_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_type_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_error(sample_config: Config) -> None:
    """Test linode_volume_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_images_list_error(sample_config: Config) -> None:
    """Test linode_image_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_image_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_instance_get_error(sample_config: Config) -> None:
    """Test linode_instance_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": 123456}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_filter_label(sample_config: Config) -> None:
    """Test linode_volume_list tool with label filter."""
    raw_volumes: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "data-vol",
                "status": "active",
                "size": 100,
                "region": "us-east",
                "linode_id": 123,
                "linode_label": "test-instance",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
            {
                "id": 2,
                "label": "backup-vol",
                "status": "active",
                "size": 50,
                "region": "us-east",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
            {
                "id": 3,
                "label": "data-backup",
                "status": "active",
                "size": 75,
                "region": "us-east",
                "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-backup",
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "hardware_type": "hdd",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_list(
            {"label_contains": "backup"}, sample_config
        )

        assert len(result) == 1
        assert "backup-vol" in result[0].text
        assert "data-backup" in result[0].text
        assert '"count": 2' in result[0].text


async def test_handle_linode_regions_list_filter_capability(
    sample_config: Config,
) -> None:
    """Test linode_region_list tool with capability filter."""
    raw_regions: dict[str, Any] = {
        "data": [
            {
                "id": "us-east",
                "label": "Newark, NJ",
                "country": "us",
                "capabilities": ["Linodes", "Block Storage"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.1", "ipv6": "2001:db8::1"},
                "site_type": "core",
            },
            {
                "id": "eu-west",
                "label": "London, UK",
                "country": "uk",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "192.0.2.2", "ipv6": "2001:db8::2"},
                "site_type": "core",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list(
            {"capability": "Block Storage"}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert "us-east" in result[0].text
        assert "eu-west" not in result[0].text


async def test_handle_linode_sshkeys_list(sample_config: Config) -> None:
    """Test linode_sshkey_list tool."""
    raw_keys: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "work-laptop",
                "ssh_key": "ssh-rsa AAAA... user@work",
                "created": "2024-01-01T00:00:00",
            },
            {
                "id": 2,
                "label": "home-desktop",
                "ssh_key": "ssh-rsa BBBB... user@home",
                "created": "2024-01-02T00:00:00",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_list({}, sample_config)

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_sshkey_list", query="")


async def test_handle_linode_sshkey_get(sample_config: Config) -> None:
    """Test linode_sshkey_get tool."""
    raw_key = {
        "id": 12345,
        "label": "work-laptop",
        "ssh_key": "ssh-rsa AAAA... user@work",
        "created": "2024-01-01T00:00:00",
        "not_in_proto": "dropped",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_get({"ssh_key_id": 12345}, sample_config)

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "12345" in result[0].text
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_sshkey_get", 12345)


async def test_handle_linode_sshkey_get_requires_id(sample_config: Config) -> None:
    """Test linode_sshkey_get requires ssh_key_id."""
    result = await handle_linode_sshkey_get({}, sample_config)

    assert len(result) == 1
    assert "ssh_key_id must be a positive integer" in result[0].text


async def test_handle_linode_sshkeys_list_filter_label(sample_config: Config) -> None:
    """Test linode_sshkey_list tool with label filter."""
    raw_keys: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "work-laptop",
                "ssh_key": "ssh-rsa AAAA... user@work",
                "created": "2024-01-01T00:00:00",
            },
            {
                "id": 2,
                "label": "home-desktop",
                "ssh_key": "ssh-rsa BBBB... user@home",
                "created": "2024-01-02T00:00:00",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_list(
            {"label_contains": "work"}, sample_config
        )

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" not in result[0].text


async def test_handle_linode_sshkeys_list_error(sample_config: Config) -> None:
    """Test linode_sshkey_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domains_list(sample_config: Config) -> None:
    """Test linode_domain_list tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {
                    "id": 1,
                    "domain": "example.com",
                    "type": "master",
                    "status": "active",
                },
                {
                    "id": 2,
                    "domain": "test.com",
                    "type": "master",
                    "status": "active",
                },
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_list({}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        assert "test.com" in result[0].text
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_domains_list_error(sample_config: Config) -> None:
    """Test linode_domain_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_get(sample_config: Config) -> None:
    """Test linode_domain_get tool."""
    raw_domain = {
        "id": 1,
        "domain": "example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
        "description": "Main domain",
        "tags": ["production"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_domain_get", 1)


async def test_handle_linode_domain_get_missing_id(sample_config: Config) -> None:
    """Test linode_domain_get tool with missing ID."""
    result = await handle_linode_domain_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_get_error(sample_config: Config) -> None:
    """Test linode_domain_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_records_list(sample_config: Config) -> None:
    """Test linode_domain_record_list tool."""
    mock_page: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "type": "A",
                "name": "www",
                "target": "192.0.2.1",
                "ttl_sec": 300,
            },
            {
                "id": 2,
                "type": "MX",
                "name": "",
                "target": "mail.example.com",
                "priority": 10,
                "ttl_sec": 300,
            },
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_list({"domain_id": 1}, sample_config)

        body = json.loads(result[0].text)
        assert body["count"] == 2
        assert "filter" not in body
        assert [r["target"] for r in body["records"]] == [
            "192.0.2.1",
            "mail.example.com",
        ]
        # The full proto DomainRecord is emitted, including fields the old
        # handler curated away (weight/port/service/protocol/tag/timestamps).
        assert body["records"][0]["weight"] == 0
        assert "tag" in body["records"][0]
        mock_client.route_raw.assert_called_once_with(
            "linode_domain_record_list", 1, query=""
        )


async def test_handle_linode_domain_record_get(sample_config: Config) -> None:
    """Test linode_domain_record_get tool."""
    raw_record = {
        "id": 2,
        "type": "A",
        "name": "www",
        "target": "192.0.2.1",
        "priority": 0,
        "weight": 0,
        "port": 0,
        "ttl_sec": 300,
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_get(
            {"domain_id": 1, "record_id": 2}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "www" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_domain_record_get", 1, 2)


async def test_handle_linode_domain_record_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_domain_record_get tool with missing record_id."""
    result = await handle_linode_domain_record_get({"domain_id": 1}, sample_config)

    assert len(result) == 1
    assert "record_id must be a positive integer" in result[0].text


async def test_handle_linode_domain_records_list_filter_type(
    sample_config: Config,
) -> None:
    """Test linode_domain_record_list tool with type filter."""
    mock_page: dict[str, Any] = {
        "data": [
            {"id": 1, "type": "A", "name": "www", "target": "192.0.2.1"},
            {"id": 2, "type": "MX", "name": "", "target": "mail.example.com"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_list(
            {"domain_id": 1, "type": "A"}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert body["filter"] == "type=A"
        assert "192.0.2.1" in result[0].text
        assert "mail.example.com" not in result[0].text


async def test_handle_linode_domain_records_list_filter_name_contains(
    sample_config: Config,
) -> None:
    """name_contains keeps records whose name has the substring (case-insensitive)."""
    mock_page: dict[str, Any] = {
        "data": [
            {"id": 1, "type": "A", "name": "WWW", "target": "192.0.2.1"},
            {"id": 2, "type": "A", "name": "api", "target": "192.0.2.2"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_list(
            {"domain_id": 1, "name_contains": "ww"}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert body["filter"] == "name_contains=ww"
        assert body["records"][0]["name"] == "WWW"


async def test_handle_linode_domain_records_list_filter_type_and_name(
    sample_config: Config,
) -> None:
    """Both filters apply together and the echo joins them with a comma."""
    mock_page: dict[str, Any] = {
        "data": [
            {"id": 1, "type": "A", "name": "www", "target": "192.0.2.1"},
            {"id": 2, "type": "A", "name": "api", "target": "192.0.2.2"},
            {"id": 3, "type": "MX", "name": "www", "target": "mail.example.com"},
        ],
        "page": 1,
        "pages": 1,
        "results": 3,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_list(
            {"domain_id": 1, "type": "A", "name_contains": "www"}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert body["filter"] == "type=A, name_contains=www"
        assert body["records"][0]["id"] == 1


async def test_handle_linode_domain_records_list_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_domain_record_list tool with missing domain_id."""
    result = await handle_linode_domain_record_list({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_records_list_error(sample_config: Config) -> None:
    """Test linode_domain_record_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_list({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


def test_create_linode_firewall_get_tool_schema() -> None:
    """Test linode_firewall_get tool schema."""
    tool, capability = create_linode_firewall_get_tool()

    assert tool.name == "linode_firewall_get"
    assert capability is Capability.Read
    assert "firewall_id" in tool.input_schema["properties"]
    assert "firewall_id" in tool.input_schema["required"]


def test_create_linode_firewall_rules_get_tool_schema() -> None:
    """Test linode_firewall_rules_get tool schema."""
    tool, capability = create_linode_firewall_rules_get_tool()

    assert tool.name == "linode_firewall_rules_get"
    assert capability is Capability.Read
    assert "firewall_id" in tool.input_schema["properties"]
    assert "firewall_id" in tool.input_schema["required"]


async def test_handle_linode_firewall_get(sample_config: Config) -> None:
    """Test linode_firewall_get tool."""
    raw_firewall: dict[str, Any] = {
        "id": 12345,
        "label": "web-firewall",
        "status": "enabled",
        "rules": {
            "inbound": [],
            "inbound_policy": "DROP",
            "outbound": [],
            "outbound_policy": "ACCEPT",
        },
        "tags": ["production"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_get({"firewall_id": 12345}, sample_config)

        assert len(result) == 1
        assert "web-firewall" in result[0].text
        mock_client.route_raw.assert_awaited_once_with("linode_firewall_get", 12345)


async def test_handle_linode_firewall_get_missing_id(sample_config: Config) -> None:
    """Test linode_firewall_get validation."""
    result = await handle_linode_firewall_get({}, sample_config)

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_handle_linode_firewall_get_rejects_negative_id(
    sample_config: Config,
) -> None:
    """Firewall get rejects a non-positive firewall_id locally (matches Go)."""
    result = await handle_linode_firewall_get({"firewall_id": -1}, sample_config)

    assert len(result) == 1
    assert "firewall_id must be a positive integer" in result[0].text


async def test_handle_linode_firewall_rules_get_rejects_negative_id(
    sample_config: Config,
) -> None:
    """Firewall rules get rejects a non-positive firewall_id locally (matches Go)."""
    result = await handle_linode_firewall_rules_get({"firewall_id": -1}, sample_config)

    assert len(result) == 1
    assert "firewall_id must be a positive integer" in result[0].text


async def test_handle_linode_firewall_rules_get(sample_config: Config) -> None:
    """Test linode_firewall_rules_get tool."""
    raw_rules = {
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
        "inbound_policy": "DROP",
        "outbound": [],
        "outbound_policy": "ACCEPT",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_rules
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_rules_get(
            {"firewall_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "DROP" in result[0].text
        assert "ACCEPT" in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_firewall_rules_get", 12345
        )


async def test_handle_linode_firewall_rules_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_firewall_rules_get validation."""
    result = await handle_linode_firewall_rules_get({}, sample_config)

    assert len(result) == 1
    assert "firewall_id is required" in result[0].text


async def test_handle_linode_firewalls_list(sample_config: Config) -> None:
    """Test linode_firewall_list tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "web-firewall", "status": "enabled"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_list({}, sample_config)

        assert len(result) == 1
        assert "web-firewall" in result[0].text
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_firewalls_list_filter_status(
    sample_config: Config,
) -> None:
    """Test linode_firewall_list tool with status filter."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "enabled-fw", "status": "enabled"},
                {"id": 2, "label": "disabled-fw", "status": "disabled"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_list({"status": "enabled"}, sample_config)

        assert len(result) == 1
        assert "enabled-fw" in result[0].text
        assert "disabled-fw" not in result[0].text


async def test_handle_linode_firewalls_list_filter_label_contains(
    sample_config: Config,
) -> None:
    """label_contains keeps matching firewalls and echoes the applied filter."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-web", "status": "enabled"},
                {"id": 2, "label": "staging-db", "status": "enabled"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_list(
            {"label_contains": "web"}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["firewalls"][0]["label"] == "prod-web"
    assert payload["filter"] == "label_contains=web"


async def test_handle_linode_firewalls_list_error(sample_config: Config) -> None:
    """Test linode_firewall_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancers_list(sample_config: Config) -> None:
    """Test linode_nodebalancer_list tool."""
    raw_nodebalancers = {
        "data": [
            {
                "id": 1,
                "label": "web-lb",
                "hostname": "nb-192-0-2-1.newark.nodebalancer.linode.com",
                "ipv4": "192.0.2.1",
                "ipv6": "2001:db8::1",
                "region": "us-east",
                "client_conn_throttle": 0,
                "transfer": {"in": 1000.0, "out": 2000.0, "total": 3000.0},
                "tags": ["production"],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_list({}, sample_config)

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_list", query=""
        )


async def test_handle_linode_nodebalancers_list_filter_region(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_list tool with region filter."""
    raw_nodebalancers: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "us-lb",
                "hostname": "nb-1.newark.nodebalancer.linode.com",
                "ipv4": "192.0.2.1",
                "ipv6": "2001:db8::1",
                "region": "us-east",
                "client_conn_throttle": 0,
                "transfer": {"in": 1000.0, "out": 2000.0, "total": 3000.0},
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            },
            {
                "id": 2,
                "label": "eu-lb",
                "hostname": "nb-2.london.nodebalancer.linode.com",
                "ipv4": "192.0.2.2",
                "ipv6": "2001:db8::2",
                "region": "eu-west",
                "client_conn_throttle": 0,
                "transfer": {"in": 500.0, "out": 1000.0, "total": 1500.0},
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_list(
            {"region": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "us-lb" in result[0].text
        assert "eu-lb" not in result[0].text


async def test_handle_linode_nodebalancers_list_filter_label_contains(
    sample_config: Config,
) -> None:
    """label_contains is a case-insensitive substring match and echoes both filters."""
    raw_nodebalancers: dict[str, Any] = {
        "data": [
            {
                "id": 1,
                "label": "prod-web-lb",
                "hostname": "nb-1.newark.nodebalancer.linode.com",
                "ipv4": "192.0.2.1",
                "ipv6": "2001:db8::1",
                "region": "us-east",
                "client_conn_throttle": 0,
                "transfer": {"in": 1000.0, "out": 2000.0, "total": 3000.0},
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            },
            {
                "id": 2,
                "label": "prod-db-lb",
                "hostname": "nb-2.newark.nodebalancer.linode.com",
                "ipv4": "192.0.2.2",
                "ipv6": "2001:db8::2",
                "region": "us-east",
                "client_conn_throttle": 0,
                "transfer": {"in": 500.0, "out": 1000.0, "total": 1500.0},
                "tags": [],
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_list(
            {"region": "us-east", "label_contains": "WEB"}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["filter"] == "region=us-east, label_contains=WEB"
    assert body["nodebalancers"][0]["label"] == "prod-web-lb"


async def test_handle_linode_nodebalancers_list_error(sample_config: Config) -> None:
    """Test linode_nodebalancer_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_config_get_tool_definition() -> None:
    """Test linode_nodebalancer_config_get tool definition."""
    tool, capability = create_linode_nodebalancer_config_get_tool()
    assert tool.name == "linode_nodebalancer_config_get"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert "config_id" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id", "config_id"]


async def test_handle_linode_nodebalancer_config_get(sample_config: Config) -> None:
    """Test linode_nodebalancer_config_get tool."""
    mock_config = {
        "id": 6,
        "port": 80,
        "protocol": "http",
        "algorithm": "roundrobin",
        "stickiness": "none",
        "nodes_status": {"up": 0, "down": 0},
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_config
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_get(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["id"] == 6
        assert data["port"] == 80
        assert data["protocol"] == "http"
        assert data["nodes_status"] == {"up": 0, "down": 0}
        assert data["check_passive"] is False
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_get", 8, 6
        )


async def test_handle_linode_nodebalancer_config_get_invalid_arguments(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_get rejects invalid IDs."""
    invalid_cases: list[tuple[dict[str, Any], str]] = [
        ({"config_id": 6}, "nodebalancer_id is required"),
        ({"nodebalancer_id": 8}, "config_id is required"),
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
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_get(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_configs_list_tool_definition() -> None:
    """Test linode_nodebalancer_config_list tool definition."""
    tool, capability = create_linode_nodebalancer_config_list_tool()
    assert tool.name == "linode_nodebalancer_config_list"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_configs_list(sample_config: Config) -> None:
    """Test linode_nodebalancer_config_list tool."""
    mock_configs = {
        "data": [{"id": 6, "port": 80, "protocol": "http"}],
        "page": 1,
        "pages": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_list(
            {"nodebalancer_id": 8}, sample_config
        )

        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert "filter" not in body
        assert body["configs"][0]["id"] == 6
        assert body["configs"][0]["port"] == 80
        assert body["configs"][0]["protocol"] == "http"
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_list", 8, query=""
        )


async def test_handle_linode_nodebalancer_configs_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_list tool with pagination."""
    mock_configs: dict[str, Any] = {"data": [], "page": 2, "pages": 3}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_list(
            {"nodebalancer_id": 8, "page": 2, "page_size": 50}, sample_config
        )

        body = json.loads(result[0].text)
        assert body == {"count": 0, "configs": []}
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_list", 8, query="page=2&page_size=50"
        )


async def test_handle_linode_nodebalancer_type_list(sample_config: Config) -> None:
    """Proto-canonical envelope: count plus full LinodeType elements."""
    from linodemcp.gentools import handle_linode_nodebalancer_type_list

    mock_types = [
        {
            "id": "nb-1",
            "label": "Standard",
            "price": {"hourly": 0.015, "monthly": 10.0},
            "region_prices": [{"id": "id-cgk", "hourly": 0.018, "monthly": 12.0}],
            "transfer": 0,
            "ignored": "extra",
        },
        {"id": "nb-2", "label": "Premium"},
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_types}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_type_list({}, sample_config)

    body = json.loads(result[0].text)
    assert body["count"] == 2
    assert "filter" not in body
    # Unknown API fields (ignored) drop; the modeled fields flow through.
    assert body["nodebalancer_types"][0] == {
        "id": "nb-1",
        "label": "Standard",
        "price": {"hourly": 0.015, "monthly": 10.0},
        "region_prices": [{"id": "id-cgk", "hourly": 0.018, "monthly": 12.0}],
        "transfer": 0,
    }
    # An element missing price/region_prices/transfer emits the proto defaults:
    # the unset price message is omitted, region_prices is [], transfer is 0.
    assert body["nodebalancer_types"][1] == {
        "id": "nb-2",
        "label": "Premium",
        "region_prices": [],
        "transfer": 0,
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_nodebalancer_type_list", query=""
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id is required"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        (
            {"nodebalancer_id": 8, "page": 0},
            "page must be an integer greater than or equal to 1",
        ),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        (
            {"nodebalancer_id": 8, "page_size": 24},
            "page_size must be an integer from 25 through 500",
        ),
        (
            {"nodebalancer_id": 8, "page_size": 501},
            "page_size must be an integer from 25 through 500",
        ),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_configs_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """Test linode_nodebalancer_config_list rejects invalid arguments."""
    result = await handle_linode_nodebalancer_config_list(arguments, sample_config)
    assert len(result) == 1
    assert message in result[0].text


async def test_handle_linode_nodebalancer_configs_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_config_nodes_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list tool."""
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
        mock_client.route_raw.return_value = mock_nodes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_list(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "node-1" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_node_list", 8, 6, query=""
        )


async def test_handle_linode_nodebalancer_config_nodes_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list tool with pagination."""
    mock_nodes: dict[str, Any] = {"data": [], "page": 2, "pages": 3}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_nodes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_list(
            {"nodebalancer_id": 8, "config_id": 6, "page": 2, "page_size": 50},
            sample_config,
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == {"count": 0, "nodes": []}
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_node_list", 8, 6, query="page=2&page_size=50"
        )


async def test_handle_linode_nodebalancer_config_nodes_list_missing_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects missing nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id is required" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_missing_config_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects missing config_id."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"nodebalancer_id": 8}, sample_config
    )
    assert len(result) == 1
    assert "config_id is required" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_invalid_page(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects non-integer page."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"nodebalancer_id": 8, "config_id": 6, "page": "abc"}, sample_config
    )
    assert len(result) == 1
    assert "page must be an integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_bool_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects bool nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"nodebalancer_id": True, "config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_zero_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects zero nodebalancer_id."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"nodebalancer_id": 0, "config_id": 6}, sample_config
    )
    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_negative_config_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list rejects negative config_id."""
    result = await handle_linode_nodebalancer_config_node_list(
        {"nodebalancer_id": 8, "config_id": -1}, sample_config
    )
    assert len(result) == 1
    assert "config_id must be a positive integer" in result[0].text


async def test_handle_linode_nodebalancer_config_nodes_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_config_node_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_config_node_list(
            {"nodebalancer_id": 8, "config_id": 6}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_get(sample_config: Config) -> None:
    """Test linode_nodebalancer_get tool."""
    raw_nodebalancer = {
        "id": 1,
        "label": "web-lb",
        "hostname": "nb-192-0-2-1.newark.nodebalancer.linode.com",
        "ipv4": "192.0.2.1",
        "ipv6": "2001:db8::1",
        "region": "us-east",
        "client_conn_throttle": 0,
        "transfer": {"in": 1000.0, "out": 2000.0, "total": 3000.0},
        "tags": ["production"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_nodebalancer_get", 1)


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
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_vpc_configs_list_tool_definition() -> None:
    """Test linode_nodebalancer_vpc_config_list tool definition."""
    tool, capability = create_linode_nodebalancer_vpc_config_list_tool()

    assert tool.name == "linode_nodebalancer_vpc_config_list"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_vpc_configs_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_config_list tool."""
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
        mock_client.route_raw.return_value = mock_configs
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_config_list(
            {"nodebalancer_id": 8, "page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data["count"] == 1
        assert "filter" not in data
        assert data["vpc_configs"][0]["id"] == 6
        assert data["vpc_configs"][0]["vpc_id"] == 1
        assert data["vpc_configs"][0]["subnet_id"] == 1
        assert data["vpc_configs"][0]["nodebalancer_id"] == 8
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_vpc_config_list", 8, query="page=1&page_size=25"
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id is required"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        (
            {"nodebalancer_id": 8, "page": 0},
            "page must be an integer greater than or equal to 1",
        ),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        (
            {"nodebalancer_id": 8, "page_size": 24},
            "page_size must be an integer from 25 through 500",
        ),
        (
            {"nodebalancer_id": 8, "page_size": 501},
            "page_size must be an integer from 25 through 500",
        ),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_vpc_configs_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer VPC config list rejects invalid arguments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_vpc_config_list(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_nodebalancer_vpc_configs_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_vpc_config_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_config_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_vpc_config_get_tool_definition() -> None:
    """Test linode_nodebalancer_vpc_config_get tool definition."""
    tool, capability = create_linode_nodebalancer_vpc_config_get_tool()

    assert tool.name == "linode_nodebalancer_vpc_config_get"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert "vpc_config_id" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id", "vpc_config_id"]


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
        mock_client.route_raw.return_value = mock_config
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
        assert "ipv4_range_id" not in data
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_vpc_config_get", 123, 456
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id is required"),
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
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_vpc_config_get(
            {"nodebalancer_id": 123, "vpc_config_id": 456}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_stackscripts_list(sample_config: Config) -> None:
    """Test linode_stackscript_list tool emits the proto list envelope."""
    raw_page = {
        "data": [
            {
                "id": 1,
                "username": "testuser",
                "user_gravatar_id": "abc123",
                "label": "my-script",
                "description": "Test script",
                "images": ["linode/ubuntu22.04"],
                "deployments_total": 10,
                "deployments_active": 5,
                "is_public": False,
                "mine": True,
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
                "script": "#!/bin/bash\necho hello",
                "user_defined_fields": [
                    {
                        "label": "Username",
                        "name": "username",
                        "example": "admin",
                        "default": "admin",
                    }
                ],
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_list({}, sample_config)

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["stackscripts"][0]["label"] == "my-script"
        # The full proto element is emitted, not the curated subset: the script
        # body and the user-defined field survive.
        assert payload["stackscripts"][0]["script"] == "#!/bin/bash\necho hello"
        assert payload["stackscripts"][0]["user_defined_fields"][0]["name"] == (
            "username"
        )
        assert "filter" not in payload
        mock_client.route_raw.assert_called_once_with(
            "linode_stackscript_list", query=""
        )


async def test_handle_linode_stackscripts_list_filter_mine(
    sample_config: Config,
) -> None:
    """Test linode_stackscript_list tool with mine filter."""
    raw_page = {
        "data": [
            {
                "id": 1,
                "username": "testuser",
                "label": "my-script",
                "is_public": False,
                "mine": True,
            },
            {
                "id": 2,
                "username": "otheruser",
                "label": "other-script",
                "is_public": True,
                "mine": False,
            },
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_list({"mine": "true"}, sample_config)

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["filter"] == "mine=true"
        assert "my-script" in result[0].text
        assert "other-script" not in result[0].text


async def test_handle_linode_stackscripts_list_filter_is_public(
    sample_config: Config,
) -> None:
    """is_public filter keeps only matching scripts and echoes the filter."""
    raw_page = {
        "data": [
            {"id": 1, "label": "public-one", "is_public": True, "mine": False},
            {"id": 2, "label": "private-one", "is_public": False, "mine": True},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_list(
            {"is_public": "false"}, sample_config
        )

        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["filter"] == "is_public=false"
        assert payload["stackscripts"][0]["label"] == "private-one"


async def test_handle_linode_stackscripts_list_filter_label_contains(
    sample_config: Config,
) -> None:
    """label_contains filter is case-insensitive substring and combines with mine."""
    raw_page = {
        "data": [
            {"id": 1, "label": "web-server", "is_public": False, "mine": True},
            {"id": 2, "label": "db-backup", "is_public": False, "mine": True},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_page
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_list(
            {"mine": "true", "label_contains": "WEB"}, sample_config
        )

        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["filter"] == "mine=true, label_contains=WEB"
        assert payload["stackscripts"][0]["label"] == "web-server"


async def test_handle_linode_stackscripts_list_error(sample_config: Config) -> None:
    """Test linode_stackscript_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_stackscript_delete_tool_schema() -> None:
    """Test linode_stackscript_delete tool schema."""
    tool, capability = create_linode_stackscript_delete_tool()

    assert tool.name == "linode_stackscript_delete"
    assert capability == Capability.Destroy
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "stackscript_id",
        "confirm",
        "dry_run",
        "mode",
        "plan_id",
    }
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"
    assert set(tool.input_schema["required"]) == {"stackscript_id", "confirm"}


async def test_handle_linode_stackscript_delete_dry_run(sample_config: Config) -> None:
    """Dry-run previews the DELETE route with the read script as state.

    The state comes from the declared read, so rev_note reaches the preview: the
    API sends it and the StackScript message models it.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "id": 12345,
            "label": "deploy",
            "script": "#!/bin/bash",
            "rev_note": "first cut",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_delete(
            {"stackscript_id": 12345, "confirm": False, "dry_run": True},
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["tool"] == "linode_stackscript_delete"
    assert payload["would_execute"]["method"] == "DELETE"
    assert payload["would_execute"]["path"] == "/linode/stackscripts/12345"
    assert payload["current_state"]["label"] == "deploy"
    assert payload["current_state"]["rev_note"] == "first cut"
    mock_client.route_call.assert_not_called()


async def test_handle_linode_stackscript_delete(sample_config: Config) -> None:
    """Test linode_stackscript_delete tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscript_delete(
            {"stackscript_id": 12345, "confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "12345" in result[0].text
    assert "deleted" in result[0].text.lower()
    mock_client.route_call.assert_awaited_once_with(
        "linode_stackscript_delete", 12345, retry=False
    )


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
    assert (
        "stackscript_id must be a positive integer" in result[0].text
        or "stackscript_id is required" in result[0].text
    )
    mock_client_class.assert_not_called()


async def test_handle_linode_stackscript_delete_error(sample_config: Config) -> None:
    """Test linode_stackscript_delete error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_call.side_effect = Exception("API error")
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
    assert set(tool.input_schema["required"]) == {"label", "script", "confirm"}
    assert "images" in tool.input_schema["properties"]


async def test_handle_linode_stackscript_create(sample_config: Config) -> None:
    """Test linode_stackscript_create tool."""
    raw_stackscript = {
        "id": 12345,
        "username": "testuser",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
        "is_public": False,
        "mine": True,
        "created": "2024-01-15T10:00:00",
        "updated": "2024-01-15T10:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_stackscript
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
        payload = json.loads(result[0].text)
        assert (
            payload["message"]
            == "StackScript 'my-script' (ID: 12345) created successfully"
        )
        assert payload["stackscript"]["id"] == 12345
        assert payload["stackscript"]["label"] == "my-script"
        mock_client.route_raw.assert_called_once_with(
            "linode_stackscript_create",
            body={
                "label": "my-script",
                "images": ["linode/ubuntu22.04"],
                "script": "#!/bin/bash",
                "description": "Test script",
                "is_public": False,
                "rev_note": "Initial revision",
            },
            retry=False,
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


async def test_handle_linode_sshkey_create(sample_config: Config) -> None:
    """Test linode_sshkey_create tool."""
    raw_key = {
        "id": 12345,
        "label": "my-key",
        "ssh_key": SAMPLE_SSH_KEY,
        "created": "2024-01-15T10:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_create(
            {"label": "my-key", "ssh_key": SAMPLE_SSH_KEY, "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        expected = serialize_api_response(
            {
                "message": "SSH key 'my-key' (ID: 12345) created successfully",
                "ssh_key": raw_key,
            },
            sshkey_pb2.SSHKeyWriteResponse(),
        )
        out = json.loads(result[0].text)
        assert out == expected
        # The public key is public information and is restored in full.
        assert out["ssh_key"]["ssh_key"] == SAMPLE_SSH_KEY


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
    raw_key = {
        "id": 12345,
        "label": "renamed-key",
        "ssh_key": SAMPLE_SSH_KEY,
        "created": "2024-01-15T10:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_update(
            {"ssh_key_id": 12345, "label": "renamed-key", "confirm": True},
            sample_config,
        )

        mock_client.route_raw.assert_awaited_once_with(
            "linode_sshkey_update", 12345, body={"label": "renamed-key"}
        )
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


async def test_sshkey_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_sshkey_create(
        {"label": "my-key", "ssh_key": SAMPLE_SSH_KEY, "dry_run": True},
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


async def test_sshkey_update_dry_run_still_validates_id(
    sample_config: Config,
) -> None:
    """Missing ssh_key_id must error out regardless of dry_run."""
    result = await handle_linode_sshkey_update(
        {"label": "renamed", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "ssh_key_id must be a positive integer" in result[0].text


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


async def test_handle_linode_instance_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_delete tool without confirmation."""
    result = await handle_linode_instance_delete({"instance_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_resize_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_resize tool without confirmation."""
    result = await handle_linode_instance_resize(
        {"instance_id": 12345, "type": "g6-standard-1"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


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
    assert "firewall_id must be a positive integer" in result[0].text


async def test_handle_linode_domain_clone(sample_config: Config) -> None:
    """Test linode_domain_clone tool emits the full proto domain element."""
    raw_domain = {
        "id": 23456,
        "domain": "clone.example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_clone(
            {"domain_id": 12345, "domain": "clone.example.com", "confirm": True},
            sample_config,
        )

    assert len(result) == 1
    payload = json.loads(result[0].text)
    expected = "Domain 12345 cloned as 'clone.example.com' (ID: 23456)"
    assert payload["message"] == expected
    assert payload["domain"]["soa_email"] == "admin@example.com"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_domain_clone", 12345, body={"domain": "clone.example.com"}, retry=False
    )


async def test_domain_clone_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true reads the zone being cloned and does not clone it."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"id": 12345, "domain": "example.com"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_domain_clone(
            {
                "domain_id": 12345,
                "domain": "clone.example.com",
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    # The one call is the declared read: a clone that ran would show a second
    # route_raw carrying the body.
    mock_client.route_raw.assert_awaited_once_with("linode_domain_get", 12345)
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
    """Test linode_domain_create tool sends the documented body and full element."""
    raw_domain = {
        "id": 12345,
        "domain": "example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_create(
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "ttl_sec": 3600,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "example.com" in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_domain_create",
            body={
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "ttl_sec": 3600,
            },
            retry=False,
        )


async def test_handle_linode_domain_update(sample_config: Config) -> None:
    """Test linode_domain_update tool sends the documented PUT body."""
    raw_domain = {
        "id": 12345,
        "domain": "example.com",
        "type": "master",
        "status": "disabled",
        "soa_email": "admin@example.com",
        "description": "Updated",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_update(
            {
                "domain_id": 12345,
                "description": "Updated",
                "status": "disabled",
                "ttl_sec": 7200,
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "modified" in result[0].text.lower()
        mock_client.route_raw.assert_awaited_once_with(
            "linode_domain_update",
            12345,
            body={"description": "Updated", "status": "disabled", "ttl_sec": 7200},
        )


async def test_handle_linode_domain_record_create(sample_config: Config) -> None:
    """Test linode_domain_record_create sends documented body, full element."""
    raw_record = {
        "id": 12345,
        "type": "A",
        "name": "www",
        "target": "8.8.8.8",
        "service": "_http",
        "protocol": "_tcp",
        "tag": "issue",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_create(
            {
                "domain_id": 12345,
                "type": "A",
                "name": "www",
                "target": "8.8.8.8",
                "service": "_http",
                "protocol": "_tcp",
                "tag": "issue",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == "A record (ID: 12345) created successfully"
        assert payload["record"]["name"] == "www"
        mock_client.route_raw.assert_awaited_once_with(
            "linode_domain_record_create",
            12345,
            body={
                "type": "A",
                "name": "www",
                "target": "8.8.8.8",
                "service": "_http",
                "protocol": "_tcp",
                "tag": "issue",
            },
            retry=False,
        )


async def test_handle_linode_domain_record_update(sample_config: Config) -> None:
    """Test linode_domain_record_update sends the documented PUT body."""
    raw_record = {
        "id": 12345,
        "type": "A",
        "name": "www",
        "target": "192.0.2.2",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_record
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
        payload = json.loads(result[0].text)
        assert payload["message"] == "Record 12345 modified successfully"
        assert payload["record"]["target"] == "192.0.2.2"
        mock_client.route_raw.assert_awaited_once_with(
            "linode_domain_record_update", 12345, 12345, body={"target": "192.0.2.2"}
        )


async def test_domain_create_dry_run_returns_preview(sample_config: Config) -> None:
    """dry_run=true previews the create with no resource state and no call."""
    result = await handle_linode_domain_create(
        {
            "domain": "example.com",
            "type": "master",
            "soa_email": "admin@example.com",
            "dry_run": True,
        },
        sample_config,
    )

    assert len(result) == 1
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_domain_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/domains"
    assert body["would_execute"]["body"] == {
        "domain": "example.com",
        "type": "master",
        "soa_email": "admin@example.com",
    }
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
        {"domain_id": 333, "type": "A", "target": "8.8.8.8", "dry_run": True},
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
    assert "8.8.8.8" in body["side_effects"][0]


async def test_domain_record_create_dry_run_still_validates_domain_id(
    sample_config: Config,
) -> None:
    """Missing domain_id must error out regardless of dry_run."""
    result = await handle_linode_domain_record_create(
        {"type": "A", "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "domain_id must be a positive integer" in result[0].text


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
                "target": "8.8.4.4",
                "dry_run": True,
            },
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["tool"] == "linode_domain_record_update"
        assert body["would_execute"]["method"] == "PUT"
        assert body["would_execute"]["path"] == "/domains/333/records/555"
        assert any("8.8.4.4" in s for s in body["side_effects"])
        mock_client.get_domain_record.assert_awaited_once_with(333, 555)
        mock_client.route_raw.assert_not_called()


async def test_handle_linode_volume_create_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_create tool without confirmation."""
    result = await handle_linode_volume_create(
        {"label": "my-volume", "region": "us-east"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_create(sample_config: Config) -> None:
    """Test linode_volume_create tool sends documented body, full element."""
    raw_volume = {
        "id": 12345,
        "label": "my-volume",
        "status": "creating",
        "size": 20,
        "region": "us-east",
        "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        "hardware_type": "nvme",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_create(
            {"label": "my-volume", "region": "us-east", "confirm": True}, sample_config
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == (
            "Volume 'my-volume' (ID: 12345) created successfully in us-east"
        )
        assert payload["volume"]["filesystem_path"].endswith("my-volume")
        # No size supplied -> omitted so the API applies its 20 GB default.
        mock_client.route_raw.assert_awaited_once_with(
            "linode_volume_create",
            body={"label": "my-volume", "region": "us-east"},
            retry=False,
        )


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


async def test_handle_linode_volume_clone_rejects_negative_id(
    sample_config: Config,
) -> None:
    """Volume clone rejects a non-positive volume_id locally (matches Go)."""
    result = await handle_linode_volume_clone(
        {"volume_id": -1, "label": "clone-vol", "confirm": True}, sample_config
    )

    assert len(result) == 1
    assert "volume_id must be a positive integer" in result[0].text


async def test_handle_linode_volume_clone(sample_config: Config) -> None:
    """Test linode_volume_clone tool sends documented body, full element."""
    raw_volume = {
        "id": 23456,
        "label": "my-volume-clone",
        "status": "creating",
        "size": 20,
        "region": "us-east",
        "hardware_type": "nvme",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
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

        mock_client.route_raw.assert_awaited_once_with(
            "linode_volume_clone", 12345, body={"label": "my-volume-clone"}, retry=False
        )
        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == (
            'Volume 12345 cloned successfully as "my-volume-clone"'
        )
        assert payload["volume"]["hardware_type"] == "nvme"


async def test_handle_linode_volume_attach(sample_config: Config) -> None:
    """Test linode_volume_attach tool sends documented body, full element."""
    raw_volume = {
        "id": 12345,
        "label": "my-volume",
        "status": "active",
        "size": 20,
        "region": "us-east",
        "linode_id": 54321,
        "linode_label": "my-linode",
        "hardware_type": "nvme",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_attach(
            {"volume_id": 12345, "linode_id": 54321, "confirm": True}, sample_config
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == (
            "Volume 12345 attached to Linode 54321 successfully"
        )
        assert payload["volume"]["linode_id"] == 54321
        # persist_across_boots not supplied -> omitted so the API applies its default.
        mock_client.route_raw.assert_awaited_once_with(
            "linode_volume_attach",
            12345,
            body={"linode_id": 54321},
        )


async def test_handle_linode_volume_detach(sample_config: Config) -> None:
    """Test linode_volume_detach tool."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = None
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
    """Test linode_volume_resize tool sends documented body, full element."""
    raw_volume = {
        "id": 12345,
        "label": "my-volume",
        "status": "resizing",
        "size": 40,
        "region": "us-east",
        "hardware_type": "nvme",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_resize(
            {"volume_id": 12345, "size": 40, "confirm": True}, sample_config
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == (
            "Volume 12345 resize to 40 GB initiated successfully"
        )
        assert payload["volume"]["size"] == 40
        mock_client.route_raw.assert_awaited_once_with(
            "linode_volume_resize", 12345, body={"size": 40}
        )


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
    """Test linode_volume_update tool sends documented PUT body, full element."""
    raw_volume = {
        "id": 12345,
        "label": "renamed-volume",
        "status": "active",
        "size": 20,
        "region": "us-east",
        "tags": ["prod"],
        "hardware_type": "nvme",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_volume
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

        mock_client.route_raw.assert_awaited_once_with(
            "linode_volume_update",
            12345,
            body={"label": "renamed-volume", "tags": ["prod"]},
        )
        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["message"] == "Volume 12345 updated successfully"
        assert payload["volume"]["tags"] == ["prod"]


async def test_handle_linode_volume_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_delete tool without confirmation."""
    result = await handle_linode_volume_delete({"volume_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


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
        mock_client.route_call.assert_not_called()


async def test_volume_detach_dry_run_surfaces_current_attachment(
    sample_config: Config,
) -> None:
    """Phase 2 Tier B walk: detach names the instance the volume is on."""

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
        mock_client.route_call.assert_not_called()


async def test_volume_update_dry_run_still_validates_change(
    sample_config: Config,
) -> None:
    """A volume_id with no label/tags must error out regardless of dry_run."""
    result = await handle_linode_volume_update(
        {"volume_id": 333, "dry_run": True}, sample_config
    )

    assert len(result) == 1
    assert "label or tags is required" in result[0].text


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


async def test_nodebalancer_delete_dry_run_still_validates_nodebalancer_id(
    sample_config: Config,
) -> None:
    """Missing nodebalancer_id must error out regardless of dry_run."""
    result = await handle_linode_nodebalancer_delete(
        {"dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "nodebalancer_id must be a positive integer" in result[0].text


async def test_linode_nodebalancer_config_delete_tool_definition() -> None:
    """Test linode_nodebalancer_config_delete tool definition."""
    tool, capability = create_linode_nodebalancer_config_delete_tool()
    assert tool.name == "linode_nodebalancer_config_delete"
    assert capability == Capability.Destroy
    assert tool.input_schema["required"] == [
        "nodebalancer_id",
        "config_id",
        "confirm",
    ]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
    assert "nodebalancer_id is required" in result[0].text


async def test_linode_nodebalancer_config_node_delete_tool_definition() -> None:
    """Test linode_nodebalancer_config_node_delete tool definition."""
    tool, _ = create_linode_nodebalancer_config_node_delete_tool()
    assert tool.name == "linode_nodebalancer_config_node_delete"
    assert tool.input_schema["required"] == [
        "nodebalancer_id",
        "config_id",
        "node_id",
        "confirm",
    ]


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


async def test_handle_linode_object_storage_buckets_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_list tool."""
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
        mock_client.route_raw.return_value = {"data": mock_buckets}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_list({}, sample_config)

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_object_storage_buckets_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_buckets_region_list(
    sample_config: Config,
) -> None:
    """By-region list emits the shared bucket list envelope (unknown fields drop)."""
    mock_buckets = [
        {
            "label": "app-data",
            "region": "us-ord",
            "hostname": "app-data.us-ord-1.linodeobjects.com",
            "not_in_proto": "dropped",
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_buckets}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_by_region_list(
            {"region": "us-ord"}, sample_config
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert body["buckets"][0]["label"] == "app-data"
        # The region is an input echo, not part of the ObjectStorageBucketListResponse
        # envelope, so the output carries only count + buckets.
        assert "region" not in body
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_bucket_by_region_list", "us-ord", query=""
        )


async def test_handle_linode_object_storage_buckets_region_list_missing_region_id(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list with missing region_id."""
    result = await handle_linode_object_storage_bucket_by_region_list({}, sample_config)

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_buckets_region_list_rejects_bad_region_id(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list rejects malformed path values."""
    for region in ("us/ord", "us?ord", "..", "US-ORD", "us--ord"):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_object_storage_bucket_by_region_list(
                {"region": region}, sample_config
            )

            assert len(result) == 1
            assert "region must be a valid region or cluster ID" in result[0].text
            mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_buckets_region_list_error(
    sample_config: Config,
) -> None:
    """Test region-scoped bucket list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_by_region_list(
            {"region": "us-ord"}, sample_config
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
        mock_client.route_raw.return_value = mock_bucket
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_bucket_get", "us-east-1", "my-bucket"
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
    """Test linode_object_storage_type_list tool."""
    # The API returns region_prices as an array of {id, hourly, monthly}
    # objects; the handler must pass it through as an array (matching Go).
    mock_types = [
        {
            "id": "objectstorage",
            "label": "Object Storage",
            "price": {"hourly": 0.02, "monthly": 5.0},
            "transfer": 1000,
            "region_prices": [
                {"id": "us-east", "hourly": 0.02, "monthly": 5.0},
            ],
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_types}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_type_list({}, sample_config)

        assert len(result) == 1
        assert '"count": 1' in result[0].text
        body = json.loads(result[0].text)
        assert body["types"][0]["region_prices"] == [
            {"id": "us-east", "hourly": 0.02, "monthly": 5.0},
        ]
        mock_client.route_raw.assert_called_once()


def test_object_storage_key_to_response_dict_shapes_nested_grants() -> None:
    """A fully populated key keeps its bucket_access and regions as object lists."""
    shaped = object_storage_key_to_response_dict(
        {
            "id": 42,
            "label": "prod-key",
            "access_key": "AKIA",
            "secret_key": "shh",
            "limited": True,
            "bucket_access": [
                {
                    "bucket_name": "assets",
                    "region": "us-east",
                    "permissions": "read_write",
                },
            ],
            "regions": [
                {"id": "us-east", "s3_endpoint": "us-east-1.linodeobjects.com"},
            ],
        }
    )

    assert shaped["id"] == 42
    assert shaped["secret_key"] == "shh"
    assert shaped["bucket_access"] == [
        {"bucket_name": "assets", "region": "us-east", "permissions": "read_write"},
    ]
    assert shaped["regions"] == [
        {"id": "us-east", "s3_endpoint": "us-east-1.linodeobjects.com"},
    ]


def test_object_storage_key_to_response_dict_defaults_missing_fields() -> None:
    """Absent nested grants and a null secret coerce to safe defaults."""
    shaped = object_storage_key_to_response_dict(
        {"id": 7, "label": "minimal", "access_key": "AKIB", "secret_key": None}
    )

    assert shaped["secret_key"] == ""
    assert shaped["bucket_access"] == []
    assert shaped["regions"] == []
    assert shaped["limited"] is False


async def test_handle_linode_object_storage_types_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_type_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_type_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


def test_linode_object_storage_endpoints_list_tool_schema() -> None:
    """Object Storage endpoints list schema has no route-specific arguments."""
    tool, capability = create_linode_object_storage_endpoint_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_endpoint_list"
    assert "required" not in tool.input_schema


async def test_handle_linode_object_storage_endpoints_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_endpoint_list tool."""
    mock_endpoints = [
        {
            "endpoint_type": "E1",
            "region": "us-sea",
            "s3_endpoint": "us-sea-1.linodeobjects.com",
        }
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_endpoints}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_endpoint_list({}, sample_config)

        assert len(result) == 1
        assert "us-sea-1.linodeobjects.com" in result[0].text
        assert '"count": 1' in result[0].text
        # No page arguments were supplied, so both stay None and the query
        # string is omitted, leaving the API's own default page in effect.
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_endpoint_list", query=""
        )


async def test_handle_linode_object_storage_endpoints_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_endpoint_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_endpoint_list({}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve items" in result[0].text


async def test_handle_linode_object_storage_keys_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_list emits the proto list envelope.

    The list endpoint returns keys WITHOUT secret material, so the mock omits
    secret_key and the proto serializes it as the empty default (no secret leak,
    and no Go-vs-Python divergence on the field).
    """
    mock_keys = [
        {
            "id": 1,
            "label": "my-key",
            "access_key": "AKIAIOSFODNN7EXAMPLE",
            "limited": True,
            "bucket_access": [
                {
                    "bucket_name": "my-bucket",
                    "region": "us-east-1",
                    "permissions": "read_write",
                }
            ],
            "regions": [
                {"id": "us-east-1", "s3_endpoint": "us-east-1.linodeobjects.com"}
            ],
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_keys}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_key_list({}, sample_config)

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["keys"][0]["label"] == "my-key"
        assert payload["keys"][0]["secret_key"] == ""
        assert payload["keys"][0]["bucket_access"][0]["bucket_name"] == "my-bucket"
        assert payload["keys"][0]["regions"][0]["s3_endpoint"] == (
            "us-east-1.linodeobjects.com"
        )
        assert "filter" not in payload
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_object_storage_keys_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_key_list({}, sample_config)

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
        mock_client.route_raw.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_key_get(
            {"key_id": 42}, sample_config
        )

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert "my-bucket" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_key_get", 42
        )


async def test_handle_linode_object_storage_key_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_get with missing key_id."""
    result = await handle_linode_object_storage_key_get({}, sample_config)

    assert len(result) == 1
    assert "key_id is required" in result[0].text


def test_linode_object_storage_quotas_list_tool_schema() -> None:
    """Quota list schema has no required route-specific arguments."""
    tool, capability = create_linode_object_storage_quota_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quota_list"
    assert "required" not in tool.input_schema


async def test_handle_linode_object_storage_quotas_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_list tool."""
    mock_quotas = [
        {
            "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
            "quota_limit": 1000,
        },
    ]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"data": mock_quotas}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_list({}, sample_config)

        assert len(result) == 1
        assert "obj-buckets-us-sea-1.linodeobjects.com" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_quota_list", query=""
        )


async def test_handle_linode_object_storage_quotas_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_list tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_list({}, sample_config)

        assert len(result) == 1
        assert "Failed to retrieve items" in result[0].text


def test_linode_object_storage_quota_get_tool_schema() -> None:
    """Quota get schema requires the quota ID."""
    tool, capability = create_linode_object_storage_quota_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quota_get"
    assert tool.input_schema["required"] == ["obj_quota_id"]
    assert tool.input_schema["properties"]["obj_quota_id"]["type"] == "string"


async def test_handle_linode_object_storage_quota_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_get tool."""
    mock_quota = {
        "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
        "quota_limit": 1000,
        "not_in_proto": "dropped",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_quota
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
        assert "not_in_proto" not in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_quota_get", "obj-buckets-us-sea-1.linodeobjects.com"
        )


async def test_handle_linode_object_storage_quota_get_requires_id(
    sample_config: Config,
) -> None:
    """Quota get requires obj_quota_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_get({}, sample_config)

    assert len(result) == 1
    assert "obj_quota_id is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("bad_id", "message"),
    [
        ("quota/with/slash", "must not contain path separators"),
        ("quota?x=1", "must not contain path separators"),
        ("quota#x", "must not contain path separators"),
        ("..", "must not contain path separators"),
        ("quota..id", "must not contain path separators"),
        ("", "obj_quota_id is required"),
        (123, "obj_quota_id is required"),
        (True, "obj_quota_id is required"),
    ],
)
async def test_handle_linode_object_storage_quota_get_rejects_bad_id(
    sample_config: Config, bad_id: Any, message: str
) -> None:
    """Quota get rejects malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_get(
            {"obj_quota_id": bad_id}, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_quota_get_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
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
    tool, capability = create_linode_object_storage_quota_usage_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_object_storage_quota_usage_get"
    assert tool.input_schema["required"] == ["obj_quota_id"]
    assert tool.input_schema["properties"]["obj_quota_id"]["type"] == "string"


async def test_handle_linode_object_storage_quota_usage(
    sample_config: Config,
) -> None:
    """Quota usage get keeps int64 byte counts as JSON numbers, unknown fields drop."""
    mock_usage = {
        "quota_limit": 1000000000000,
        "usage": 5368709120,
        "not_in_proto": "dropped",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_usage
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_usage_get(
            {"obj_quota_id": "obj-bucket-us-ord-1"}, sample_config
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "quota_limit": 1000000000000,
            "usage": 5368709120,
        }
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_quota_usage_get", "obj-bucket-us-ord-1"
        )


async def test_handle_linode_object_storage_quota_usage_requires_id(
    sample_config: Config,
) -> None:
    """Quota usage requires obj_quota_id."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_usage_get({}, sample_config)

    assert len(result) == 1
    assert "obj_quota_id is required" in result[0].text
    mock_client_class.assert_not_called()


# The two languages worded this one refusal differently and the contract now
# carries one wording for both: an id nobody sent reads as required, and one the
# route cannot address names the characters it will not take.
@pytest.mark.parametrize(
    ("bad_id", "expected"),
    [
        ("1/2", "obj_quota_id must not contain"),
        ("1?x=1", "obj_quota_id must not contain"),
        ("..", "obj_quota_id must not contain"),
        (0, "obj_quota_id is required"),
        (-1, "obj_quota_id is required"),
        (True, "obj_quota_id is required"),
        (1.9, "obj_quota_id is required"),
    ],
)
async def test_handle_linode_object_storage_quota_usage_rejects_bad_id(
    sample_config: Config, bad_id: Any, expected: str
) -> None:
    """Quota usage rejects malformed path parameters before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_object_storage_quota_usage_get(
            {"obj_quota_id": bad_id}, sample_config
        )

    assert len(result) == 1
    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_object_storage_quota_usage_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_quota_usage_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_quota_usage_get(
            {"obj_quota_id": "obj-bucket-us-ord-1"}, sample_config
        )

        assert len(result) == 1
        assert "Failed to retrieve Object Storage quota usage" in result[0].text


async def test_handle_linode_object_storage_transfer(
    sample_config: Config,
) -> None:
    """Transfer get emits the int64 used byte count as a JSON string, unknown drop."""
    mock_transfer = {"used": 1073741824, "not_in_proto": "dropped"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_transfer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer_get({}, sample_config)

        assert len(result) == 1
        assert json.loads(result[0].text) == {"used": 1073741824}
        mock_client.route_raw.assert_called_once()


async def test_handle_linode_object_storage_transfer_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_transfer_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer_get({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get tool."""
    mock_access = {"acl": "public-read", "cors_enabled": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_access
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "public-read" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_object_storage_bucket_access_get", "us-east-1", "my-bucket"
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
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text


def test_linode_object_storage_cancel_tool_schema() -> None:
    """Object Storage cancel tool should require boolean confirmation."""
    tool, capability = create_linode_object_storage_cancel_tool()

    assert capability is Capability.Write
    assert tool.name == "linode_object_storage_cancel"
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert "confirm" in tool.input_schema["required"]


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
    """Object Storage cancel returns the fixed confirmation, matching Go.

    The retry=False keyword is the contract's retry_disabled reaching the call:
    a replayed cancel would act twice on account service state.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        # The cancel endpoint returns an empty body; the handler emits the fixed
        # confirmation message regardless, so the API return is not echoed.
        mock_client.route_call.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_cancel(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == {
        "message": "Object Storage cancellation requested successfully"
    }
    mock_client.route_call.assert_called_once_with(
        "linode_object_storage_cancel", retry=False
    )


async def test_handle_object_storage_cancel_error(
    sample_config: Config,
) -> None:
    """Object Storage cancel should report client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_call.side_effect = Exception("API error")
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
    assert result[0].text == (
        "Error: acl must be one of: "
        "private, public-read, authenticated-read, public-read-write"
    )


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
        mock_client.route_raw.return_value = {
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
    assert "key_id must be a positive integer" in result[0].text


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
        mock_client.route_raw.return_value = {
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
    assert result[0].text == (
        "Error: acl must be one of: "
        "private, public-read, authenticated-read, public-read-write"
    )


async def test_object_acl_update_success(
    sample_config: Config,
) -> None:
    """Object ACL update should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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
        mock_client.route_raw.assert_awaited_once_with(
            "linode_object_storage_object_acl_update",
            "us-east-1",
            "my-bucket",
            body={"name": "photo.jpg", "acl": "public-read"},
        )
        payload = json.loads(result[0].text)
        assert (
            payload["message"]
            == "ACL for object 'photo.jpg' in bucket 'my-bucket' modified successfully"
        )
        assert payload["acl"]["acl"] == "public-read"
        acl_xml = "<AccessControlPolicy>...</AccessControlPolicy>"
        assert payload["acl"]["acl_xml"] == acl_xml


async def test_ssl_get_success(
    sample_config: Config,
) -> None:
    """SSL get should succeed with valid input."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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
        mock_client.route_raw.return_value = {"ssl": True}
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
        mock_client.route_raw.assert_awaited_once_with(
            "linode_object_storage_ssl_upload",
            "us-east-1",
            "my-bucket",
            body={"certificate": "cert", "private_key": "key"},
        )
        payload = json.loads(result[0].text)
        assert (
            payload["message"]
            == "SSL certificate uploaded to bucket 'my-bucket' in region 'us-east-1'"
        )
        assert payload["ssl"] == {"ssl": True}


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


async def test_lke_clusters_list_tool_definition() -> None:
    """LKE clusters list tool should have correct name."""
    tool, _ = create_linode_lke_cluster_list_tool()
    assert tool.name == "linode_lke_cluster_list"


async def test_lke_cluster_get_tool_definition() -> None:
    """LKE cluster get tool should require cluster_id."""
    tool, _ = create_linode_lke_cluster_get_tool()
    assert tool.name == "linode_lke_cluster_get"
    assert "cluster_id" in (tool.input_schema.get("required") or [])


async def test_lke_clusters_list(sample_config: Config) -> None:
    """LKE clusters list should return cluster data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {
                    "id": 1,
                    "label": "my-cluster",
                    "region": "us-east",
                    "status": "ready",
                },
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_cluster_list({}, sample_config))

        assert len(result) == 1
        assert "my-cluster" in result[0].text
        mock_client.route_raw.assert_called_once_with(
            "linode_lke_cluster_list", query=""
        )


async def test_lke_clusters_list_no_filter_returns_all(sample_config: Config) -> None:
    """LKE cluster list without a label filter should return every cluster."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-cluster", "region": "us-east"},
                {"id": 2, "label": "dev-cluster", "region": "us-west"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_cluster_list({}, sample_config))

        payload = json.loads(result[0].text)
        assert payload["count"] == 2
        assert "filter" not in payload


async def test_lke_clusters_list_filters_by_label_substring(
    sample_config: Config,
) -> None:
    """LKE cluster list label filter is a case-insensitive substring match."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-cluster", "region": "us-east"},
                {"id": 2, "label": "dev-cluster", "region": "us-west"},
                {"id": 3, "label": "staging-prod", "region": "eu-west"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_list({"label": "PROD"}, sample_config)
        )

        payload = json.loads(result[0].text)
        assert payload["count"] == 2
        labels = {cluster["label"] for cluster in payload["clusters"]}
        assert labels == {"prod-cluster", "staging-prod"}
        assert payload["filter"] == "label=PROD"


async def test_lke_cluster_get(sample_config: Config) -> None:
    """LKE cluster get should return cluster details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


@pytest.mark.parametrize(
    "handler",
    [
        handle_linode_lke_cluster_get,
        handle_linode_lke_kubeconfig_get,
        handle_linode_lke_dashboard_get,
        handle_linode_lke_api_endpoint_list,
    ],
)
@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "cluster_id is required"),
        ({"cluster_id": "not-a-number"}, "cluster_id must be a positive integer"),
    ],
)
async def test_lke_cluster_id_path_handlers_reject_bad_id(
    sample_config: Config,
    handler: Any,
    arguments: dict[str, Any],
    expected: str,
) -> None:
    """Every cluster_id path handler rejects a missing or non-integer id."""
    result = list(await handler(arguments, sample_config))

    assert len(result) == 1
    assert expected in result[0].text


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


async def test_lke_pools_list(sample_config: Config) -> None:
    """LKE pools list should return pool data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 100, "type": "g6-standard-1", "count": 3},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert "filter" not in body
        assert body["pools"][0]["id"] == 100
        assert body["pools"][0]["type"] == "g6-standard-1"
        assert body["pools"][0]["count"] == 3
        mock_client.route_raw.assert_awaited_once_with(
            "linode_lke_pool_list", 1, query=""
        )


async def test_lke_pools_list_missing_cluster_id(sample_config: Config) -> None:
    """A missing cluster_id is rejected before any client call."""
    result = await handle_linode_lke_pool_list({}, sample_config)

    assert len(result) == 1
    assert "cluster_id is required" in result[0].text


async def test_lke_pools_list_non_integer_cluster_id(sample_config: Config) -> None:
    """A non-integer cluster_id is rejected with a clear message."""
    result = await handle_linode_lke_pool_list({"cluster_id": "abc"}, sample_config)

    assert len(result) == 1
    assert "cluster_id must be a positive integer" in result[0].text


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "cluster_id is required"),
        ({"cluster_id": 1}, "pool_id is required"),
        ({"cluster_id": "x", "pool_id": 2}, "cluster_id must be a positive integer"),
        ({"cluster_id": 1, "pool_id": "y"}, "pool_id must be a positive integer"),
    ],
)
async def test_lke_pool_get_invalid_ids(
    sample_config: Config, arguments: dict[str, Any], expected: str
) -> None:
    """pool_get rejects missing or non-integer cluster_id/pool_id before any call."""
    result = await handle_linode_lke_pool_get(arguments, sample_config)

    assert len(result) == 1
    assert expected in result[0].text


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "cluster_id is required"),
        ({"cluster_id": 1}, "node_id is required"),
        ({"cluster_id": "x", "node_id": "n"}, "cluster_id must be a positive integer"),
    ],
)
async def test_lke_node_get_invalid_ids(
    sample_config: Config, arguments: dict[str, Any], expected: str
) -> None:
    """node_get rejects missing cluster_id/node_id and non-integer cluster_id."""
    result = await handle_linode_lke_node_get(arguments, sample_config)

    assert len(result) == 1
    assert expected in result[0].text


async def test_lke_pool_get(sample_config: Config) -> None:
    """LKE pool get should return pool details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


async def test_lke_node_get(sample_config: Config) -> None:
    """LKE node get should return node details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


async def test_lke_kubeconfig_get(sample_config: Config) -> None:
    """LKE kubeconfig get should return kubeconfig data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


async def test_lke_dashboard_get(sample_config: Config) -> None:
    """LKE dashboard get should return dashboard URL."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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
        mock_client.route_raw.return_value = {
            "data": [
                {"endpoint": "https://api.lke.example.com"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_api_endpoint_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "endpoint" in result[0].text.lower()


async def test_lke_versions_list(sample_config: Config) -> None:
    """LKE versions list should return version data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": "1.29"},
                {"id": "1.28"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_version_list({}, sample_config))

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 2
        assert body["versions"][0]["id"] == "1.29"
        assert body["versions"][1]["id"] == "1.28"
        assert "filter" not in body


async def test_lke_version_get(sample_config: Config) -> None:
    """LKE version get should return version details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {"id": "1.29"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_version_get({"version": "1.29"}, sample_config)
        )

        assert len(result) == 1
        assert "1.29" in result[0].text


async def test_lke_version_get_missing_id(sample_config: Config) -> None:
    """LKE version get should fail without version."""
    result = list(await handle_linode_lke_version_get({}, sample_config))

    assert len(result) == 1
    assert "version" in result[0].text.lower()


async def test_lke_version_get_rejects_path_separator(sample_config: Config) -> None:
    """LKE version get rejects a path-unsafe version locally (matches Go)."""
    result = list(
        await handle_linode_lke_version_get(
            {"version": "1.31/../secrets"}, sample_config
        )
    )

    assert len(result) == 1
    assert "version must be a Kubernetes version ID" in result[0].text


async def test_lke_types_list(sample_config: Config) -> None:
    """LKE types list returns the proto-canonical LinodeType envelope."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {
                    "id": "g6-standard-1",
                    "label": "Linode 2GB",
                    "price": {"hourly": 0.018, "monthly": 12.0},
                    "region_prices": [
                        {"id": "id-cgk", "hourly": 0.021, "monthly": 14.0}
                    ],
                    "transfer": 0,
                },
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_type_list({}, sample_config))

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["count"] == 1
        assert "filter" not in body
        assert body["lke_types"][0] == {
            "id": "g6-standard-1",
            "label": "Linode 2GB",
            "price": {"hourly": 0.018, "monthly": 12.0},
            "region_prices": [{"id": "id-cgk", "hourly": 0.021, "monthly": 14.0}],
            "transfer": 0,
        }


async def test_lke_tier_versions_list(sample_config: Config) -> None:
    """LKE tier versions list should return tier version data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": "1.29", "tier": "standard"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_tier_version_list(
                {"tier": "standard"}, sample_config
            )
        )

        assert len(result) == 1
        assert "1.29" in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_lke_tier_version_list", "standard", query=""
        )


async def test_lke_tier_versions_list_requires_tier(sample_config: Config) -> None:
    """LKE tier versions list requires tier before client dispatch."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = list(await handle_linode_lke_tier_version_list({}, sample_config))

    assert "tier is required" in result[0].text
    mock_cls.assert_not_called()


@pytest.mark.parametrize(
    "tier",
    [
        "standard/enterprise",
        "standard?tier",
        "..",
        "#bad",
        "bad&",
        "bad tier",
        "standard%2F..",
        "internal",
    ],
)
async def test_lke_tier_versions_list_rejects_malformed_tier(
    sample_config: Config, tier: object
) -> None:
    """LKE tier versions list rejects any tier outside the standard|enterprise set."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = list(
            await handle_linode_lke_tier_version_list({"tier": tier}, sample_config)
        )

    assert "tier must be one of: standard, enterprise" in result[0].text
    mock_cls.assert_not_called()


async def test_vpcs_list_tool_definition() -> None:
    """VPCs list tool should have correct name."""
    tool, _ = create_linode_vpc_list_tool()
    assert tool.name == "linode_vpc_list"


async def test_vlans_list_tool_definition() -> None:
    """VLANs list tool should have correct name."""
    tool, _ = create_linode_vlan_list_tool()
    assert tool.name == "linode_vlan_list"


async def test_vlan_delete_tool_definition() -> None:
    """VLAN delete tool should have correct name and required params."""
    tool, _ = create_linode_vlan_delete_tool()
    assert tool.name == "linode_vlan_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "region_id" in required
    assert "label" in required
    assert "confirm" in required


async def test_vpc_get_tool_definition() -> None:
    """VPC get tool should require vpc_id."""
    tool, _ = create_linode_vpc_get_tool()
    assert tool.name == "linode_vpc_get"
    assert "vpc_id" in (tool.input_schema.get("required") or [])


async def test_vpc_delete_tool_definition() -> None:
    """VPC delete tool should require vpc_id and confirm."""
    tool, _ = create_linode_vpc_delete_tool()
    assert tool.name == "linode_vpc_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "vpc_id" in required
    assert "confirm" in required


async def test_ipv6_range_get_tool_definition() -> None:
    """IPv6 range get tool should require range without confirm."""
    tool, _ = create_linode_ipv6_range_get_tool()
    assert tool.name == "linode_ipv6_range_get"
    required: list[str] = tool.input_schema.get("required") or []
    properties: dict[str, Any] = tool.input_schema.get("properties") or {}
    assert "range" in required
    assert "confirm" not in required
    assert "confirm" not in properties


async def test_ipv6_range_delete_tool_definition() -> None:
    """IPv6 range delete tool should require range and confirm."""
    tool, _ = create_linode_ipv6_range_delete_tool()
    assert tool.name == "linode_ipv6_range_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "range" in required
    assert "confirm" in required


async def test_vpc_subnet_delete_tool_definition() -> None:
    """VPC subnet delete tool should require vpc_id, subnet_id, confirm."""
    tool, _ = create_linode_vpc_subnet_delete_tool()
    assert tool.name == "linode_vpc_subnet_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "vpc_id" in required
    assert "subnet_id" in required
    assert "confirm" in required


async def test_vpcs_list(sample_config: Config) -> None:
    """VPCs list should return VPC data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "my-vpc", "region": "us-east"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_list({}, sample_config))

        assert len(result) == 1
        assert "my-vpc" in result[0].text
        mock_client.route_raw.assert_called_once_with("linode_vpc_list", query="")


async def test_vpcs_list_no_filter_returns_all(sample_config: Config) -> None:
    """VPC list without filters should return every VPC and no filter key."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-vpc", "region": "us-east"},
                {"id": 2, "label": "dev-vpc", "region": "us-west"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_list({}, sample_config))

        payload = json.loads(result[0].text)
        assert payload["count"] == 2
        assert "filter" not in payload


async def test_vpcs_list_filters_by_label_substring(sample_config: Config) -> None:
    """VPC list label filter is a case-insensitive substring match."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-vpc", "region": "us-east"},
                {"id": 2, "label": "dev-vpc", "region": "us-west"},
                {"id": 3, "label": "staging-prod", "region": "us-east"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_list({"label": "PROD"}, sample_config))

        payload = json.loads(result[0].text)
        assert payload["count"] == 2
        labels = {vpc["label"] for vpc in payload["vpcs"]}
        assert labels == {"prod-vpc", "staging-prod"}
        assert payload["filter"] == "label=PROD"


async def test_vpcs_list_filters_by_region_exact(sample_config: Config) -> None:
    """VPC list region filter is a case-insensitive exact match, not substring."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "a", "region": "us-east"},
                {"id": 2, "label": "b", "region": "us-west"},
                {"id": 3, "label": "c", "region": "us-east-1"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_list({"region": "US-EAST"}, sample_config)
        )

        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["vpcs"][0]["id"] == 1
        assert payload["filter"] == "region=US-EAST"


async def test_vpcs_list_filters_by_label_and_region(sample_config: Config) -> None:
    """VPC list applies label and region filters together."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "prod-vpc", "region": "us-east"},
                {"id": 2, "label": "prod-vpc", "region": "us-west"},
                {"id": 3, "label": "dev-vpc", "region": "us-east"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_list(
                {"label": "prod", "region": "us-east"}, sample_config
            )
        )

        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["vpcs"][0]["id"] == 1
        assert payload["filter"] == "label=prod, region=us-east"


async def test_vlans_list(sample_config: Config) -> None:
    """VLANs list should return VLAN data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"label": "app-vlan", "region": "us-east", "linodes": [123]},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vlan_list({}, sample_config))

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["vlans"] == [
            {"label": "app-vlan", "region": "us-east", "linodes": [123]}
        ]
        assert "filter" not in payload
        mock_client.route_raw.assert_called_once()


async def test_vlans_list_rejects_non_integer_page(sample_config: Config) -> None:
    """VLAN list rejects a non-integer page before constructing the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vlan_list({"page": "first"}, sample_config))

        assert len(result) == 1
        assert "page must be an integer" in result[0].text
        mock_client.route_raw.assert_not_called()


async def test_vlans_list_rejects_page_size_below_minimum(
    sample_config: Config,
) -> None:
    """VLAN list rejects a page_size under the minimum before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vlan_list({"page_size": 10}, sample_config))

        assert len(result) == 1
        assert "page_size must be an integer from 25 through 500" in result[0].text
        mock_client.route_raw.assert_not_called()


async def test_vlans_list_rejects_page_size_above_maximum(
    sample_config: Config,
) -> None:
    """VLAN list rejects a page_size over the maximum before the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vlan_list({"page_size": 999}, sample_config))

        assert len(result) == 1
        assert "page_size must be an integer from 25 through 500" in result[0].text
        mock_client.route_raw.assert_not_called()


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
        mock_client.route_call.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vlan_delete(
                {
                    "region_id": "us-east",
                    "label": "app-vlan",
                    "confirm": True,
                    "confirm_bypass_dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert (
            payload["message"]
            == "VLAN app-vlan deleted successfully from region us-east"
        )
        mock_client.route_call.assert_called_once_with(
            "linode_vlan_delete", "us-east", "app-vlan", retry=False
        )


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


async def test_vlan_delete_rejects_malformed_region(sample_config: Config) -> None:
    """VLAN delete rejects a non-slug region_id locally (matches Go)."""
    result = list(
        await handle_linode_vlan_delete(
            {"region_id": "US_EAST", "label": "app-vlan", "dry_run": True},
            sample_config,
        )
    )
    assert "region_id must be a lowercase region slug" in result[0].text


async def test_vpc_get(sample_config: Config) -> None:
    """VPC get should return VPC details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


async def test_vpc_get_rejects_non_integer_id(sample_config: Config) -> None:
    """VPC get rejects a vpc_id that is not a positive integer."""
    result = list(await handle_linode_vpc_get({"vpc_id": "abc"}, sample_config))

    assert len(result) == 1
    assert "vpc_id must be a positive integer" in result[0].text


async def test_ipv6_range_get_missing_range(sample_config: Config) -> None:
    """IPv6 range get should fail without range."""
    result = list(await handle_linode_ipv6_range_get({}, sample_config))

    assert len(result) == 1
    assert "range" in result[0].text.lower()


async def test_ipv6_range_get_success(sample_config: Config) -> None:
    """IPv6 range get emits the detail shape (is_bgp + linodes) proto-canonically."""
    ipv6_range = "2001:0db8::/64"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        # The detail endpoint returns is_bgp and the bound Linode IDs and omits
        # route_target; the extra key must drop through the serializer.
        mock_client.route_raw.return_value = {
            "range": ipv6_range,
            "region": "us-east",
            "prefix": 64,
            "is_bgp": False,
            "linodes": [12345, 12346],
            "not_in_proto": "dropped",
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
        # route_target is implicit-presence so it serializes as "" on the detail
        # GET; is_bgp is optional so a genuine false still serializes.
        assert json.loads(result[0].text) == {
            "range": ipv6_range,
            "region": "us-east",
            "prefix": 64,
            "route_target": "",
            "is_bgp": False,
            "linodes": [12345, 12346],
        }
        mock_client.route_raw.assert_called_once_with(
            "linode_ipv6_range_get", ipv6_range
        )


async def test_ipv6_range_get_rejects_malformed_range(sample_config: Config) -> None:
    """IPv6 range get rejects a non-prefix range locally (matches Go)."""
    result = list(
        await handle_linode_ipv6_range_get({"range": "not-a-range"}, sample_config)
    )

    assert len(result) == 1
    assert "range must be a valid IPv6 prefix" in result[0].text


async def test_ipv6_range_delete_rejects_malformed_range(
    sample_config: Config,
) -> None:
    """IPv6 range delete rejects a non-prefix range locally (matches Go)."""
    result = list(
        await handle_linode_ipv6_range_delete(
            {
                "range": "2001:db8::1/64",
                "confirm": True,
                "confirm_bypass_dry_run": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "range must be a valid IPv6 prefix" in result[0].text


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
    assert "vpc_id must be a positive integer" in result[0].text


async def test_ipv6_range_delete_confirm_required(sample_config: Config) -> None:
    """IPv6 range delete should require confirm=true."""
    result = list(
        await handle_linode_ipv6_range_delete(
            {"range": "2001:0db8::/64", "confirm": False},
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
    ipv6_range = "2001:0db8::/64"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_call.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_ipv6_range_delete(
                {
                    "range": ipv6_range,
                    "confirm": True,
                    "confirm_bypass_dry_run": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body == {
            "message": "IPv6 range deleted",
            "range": ipv6_range,
        }
        mock_client.route_call.assert_called_once_with(
            "linode_ipv6_range_delete", ipv6_range, retry=False
        )


async def test_ipv6_range_delete_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch the range via GET and never call delete."""
    ipv6_range = "2001:0db8::/64"
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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
        assert (
            body["would_execute"]["path"] == "/networking/ipv6/ranges/2001:0db8::%2F64"
        )
        mock_client.route_raw.assert_awaited_once_with(
            "linode_ipv6_range_get", ipv6_range
        )
        mock_client.route_call.assert_not_called()


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
        mock_client.route_raw.return_value = {
            "data": [
                {"address": "10.0.0.1", "vpc_id": 1, "subnet_id": 1},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ip_all_list({}, sample_config))

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["ips"][0]["address"] == "10.0.0.1"
        assert payload["ips"][0]["vpc_id"] == 1
        assert "data" not in payload


async def test_vpc_ip_list(sample_config: Config) -> None:
    """VPC IP list should return IPs for a specific VPC."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"address": "10.0.0.2", "vpc_id": 1, "subnet_id": 1},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ip_list({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        payload = json.loads(result[0].text)
        assert payload["count"] == 1
        assert payload["ips"][0]["address"] == "10.0.0.2"
        assert payload["ips"][0]["vpc_id"] == 1
        assert "data" not in payload


async def test_vpc_ip_list_missing_id(sample_config: Config) -> None:
    """VPC IP list should fail without vpc_id."""
    result = list(await handle_linode_vpc_ip_list({}, sample_config))

    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()


async def test_vpc_ip_list_rejects_non_integer_id(sample_config: Config) -> None:
    """A non-integer vpc_id is rejected before the client is called."""
    result = list(
        await handle_linode_vpc_ip_list({"vpc_id": "not-a-number"}, sample_config)
    )

    assert len(result) == 1
    assert "vpc_id must be a positive integer" in result[0].text


async def test_vpc_subnets_list(sample_config: Config) -> None:
    """VPC subnets list should return proto-canonical subnet data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 1, "label": "my-subnet", "ipv4": "10.0.0.0/24"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_subnet_list({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        assert json.loads(result[0].text) == {
            "count": 1,
            "subnets": [
                {
                    "id": 1,
                    "label": "my-subnet",
                    "ipv4": "10.0.0.0/24",
                    "linodes": [],
                    "created": "",
                    "updated": "",
                }
            ],
        }
        mock_client.route_raw.assert_awaited_once_with(
            "linode_vpc_subnet_list", 1, query=""
        )


async def test_vpc_subnet_list_missing_id(sample_config: Config) -> None:
    """VPC subnet list fails without vpc_id."""
    result = list(await handle_linode_vpc_subnet_list({}, sample_config))

    assert len(result) == 1
    assert "vpc_id is required" in result[0].text


async def test_vpc_subnet_list_rejects_non_integer_id(sample_config: Config) -> None:
    """VPC subnet list rejects a vpc_id that is not a positive integer."""
    result = list(await handle_linode_vpc_subnet_list({"vpc_id": "x"}, sample_config))

    assert len(result) == 1
    assert "vpc_id must be a positive integer" in result[0].text


async def test_vpc_subnet_get(sample_config: Config) -> None:
    """VPC subnet get should return subnet details."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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


async def test_instance_backups_list_tool_definition() -> None:
    """Backups list tool should require linode_id."""
    tool, _ = create_linode_instance_backup_list_tool()
    assert tool.name == "linode_instance_backup_list"
    assert "linode_id" in (tool.input_schema.get("required") or [])


async def test_instance_backup_get_tool_definition() -> None:
    """Backup get tool should require linode_id and backup_id."""
    tool, _ = create_linode_instance_backup_get_tool()
    assert tool.name == "linode_instance_backup_get"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "backup_id" in required


async def test_instance_backups_cancel_tool_def() -> None:
    """Backups cancel tool should require linode_id and confirm."""
    tool, _ = create_linode_instance_backups_cancel_tool()
    assert tool.name == "linode_instance_backups_cancel"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "confirm" in required


async def test_instance_backups_list_missing_id(
    sample_config: Config,
) -> None:
    """Backups list should fail without linode_id."""
    result = list(await handle_linode_instance_backup_list({}, sample_config))
    assert len(result) == 1
    assert "linode_id" in result[0].text.lower()


async def test_instance_backups_list_success(
    sample_config: Config,
) -> None:
    """Backups list emits the proto envelope: automatic[] plus the snapshot object.

    The endpoint returns a nested object (not a {count, key} list). The proto
    output keeps automatic as a list, and the nullable snapshot.current /
    snapshot.in_progress message fields drop out when the API returns null.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "automatic": [
                {"id": 42, "label": "", "status": "successful", "type": "auto"}
            ],
            "snapshot": {
                "current": {"id": 99, "label": "nightly", "status": "successful"},
                "in_progress": None,
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_backup_list({"linode_id": 123}, sample_config)
        )
        assert len(result) == 1
        body = json.loads(result[0].text)
        assert [b["id"] for b in body["automatic"]] == [42]
        assert body["snapshot"]["current"]["id"] == 99
        # in_progress was null, so the proto message field is omitted entirely.
        assert "in_progress" not in body["snapshot"]


async def test_instance_backups_list_invalid_id(
    sample_config: Config,
) -> None:
    """Backups list rejects a non-integer linode_id before any client call."""
    result = list(
        await handle_linode_instance_backup_list(
            {"linode_id": "not-a-number"}, sample_config
        )
    )
    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text


async def test_instance_backups_cancel_no_confirm(
    sample_config: Config,
) -> None:
    """Backups cancel should require confirm=true."""
    result = list(
        await handle_linode_instance_backups_cancel({"linode_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_backup_get_missing_ids(
    sample_config: Config,
) -> None:
    """Backup get should fail without backup_id."""
    result = list(
        await handle_linode_instance_backup_get({"linode_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "backup_id" in result[0].text.lower()


async def test_instance_backup_get_invalid_backup_id(
    sample_config: Config,
) -> None:
    """Backup get rejects a non-integer backup_id before any client call."""
    result = list(
        await handle_linode_instance_backup_get(
            {"linode_id": 123, "backup_id": "nope"}, sample_config
        )
    )
    assert len(result) == 1
    assert "backup_id must be a positive integer" in result[0].text


async def test_instance_backup_get_invalid_linode_id(
    sample_config: Config,
) -> None:
    """Backup get propagates a bad linode_id error before reaching backup_id."""
    result = list(
        await handle_linode_instance_backup_get(
            {"linode_id": "bad", "backup_id": 5}, sample_config
        )
    )
    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text


async def test_instance_disks_list_tool_def() -> None:
    """Disks list tool should require linode_id."""
    tool, _ = create_linode_instance_disk_list_tool()
    assert tool.name == "linode_instance_disk_list"
    assert "linode_id" in (tool.input_schema.get("required") or [])


async def test_instance_disk_get_tool_def() -> None:
    """Disk get tool should require linode_id and disk_id."""
    tool, _ = create_linode_instance_disk_get_tool()
    assert tool.name == "linode_instance_disk_get"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "disk_id" in required


async def test_instance_disk_delete_tool_def() -> None:
    """Disk delete should require linode_id, disk_id, confirm."""
    tool, _ = create_linode_instance_disk_delete_tool()
    assert tool.name == "linode_instance_disk_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "disk_id" in required
    assert "confirm" in required


async def test_instance_disks_list_success(
    sample_config: Config,
) -> None:
    """Disks list should return disk data."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "boot", "size": 25000},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_disk_list({"linode_id": 123}, sample_config)
        )
        assert len(result) == 1
        assert "boot" in result[0].text


async def test_instance_disk_delete_no_confirm(
    sample_config: Config,
) -> None:
    """Disk delete should require confirm=true."""
    result = list(
        await handle_linode_instance_disk_delete(
            {"linode_id": 123, "disk_id": 1},
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
        await handle_linode_instance_disk_get({"linode_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "disk_id" in result[0].text.lower()


async def test_instance_ips_list_tool_def() -> None:
    """IPs list tool should require linode_id."""
    tool, _ = create_linode_instance_ip_list_tool()
    assert tool.name == "linode_instance_ip_list"
    assert "linode_id" in (tool.input_schema.get("required") or [])


async def test_instance_ip_get_tool_def() -> None:
    """IP get tool should require linode_id and address."""
    tool, _ = create_linode_instance_ip_get_tool()
    assert tool.name == "linode_instance_ip_get"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "address" in required


async def test_instance_ip_delete_tool_def() -> None:
    """IP delete should require linode_id, address, confirm."""
    tool, _ = create_linode_instance_ip_delete_tool()
    assert tool.name == "linode_instance_ip_delete"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "address" in required
    assert "confirm" in required


async def test_instance_ips_list_success(
    sample_config: Config,
) -> None:
    """IPs list emits the nested proto object: ipv4 categories plus ipv6.

    The endpoint returns the full address configuration as a nested object, so
    the proto output models ipv4.public/private/shared/reserved and ipv6 rather
    than a flat list. The omitted ipv4 categories come back as empty lists.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
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
            await handle_linode_instance_ip_list({"linode_id": 123}, sample_config)
        )
        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["ipv4"]["public"][0]["address"] == "192.0.2.1"
        # categories the API omitted are emitted as empty lists by the proto.
        assert body["ipv4"]["private"] == []
        assert body["ipv6"]["slaac"]["address"] == "2001:db8::1"


async def test_instance_ips_list_invalid_id(
    sample_config: Config,
) -> None:
    """IPs list rejects a non-integer linode_id before any client call."""
    result = list(
        await handle_linode_instance_ip_list({"linode_id": "bogus"}, sample_config)
    )
    assert len(result) == 1
    assert "linode_id must be a positive integer" in result[0].text


async def test_instance_ip_get_missing_address(
    sample_config: Config,
) -> None:
    """IP get should fail without address."""
    result = list(
        await handle_linode_instance_ip_get({"linode_id": 123}, sample_config)
    )
    assert len(result) == 1
    assert "address" in result[0].text.lower()


async def test_instance_ip_delete_no_confirm(
    sample_config: Config,
) -> None:
    """IP delete should require confirm=true."""
    result = list(
        await handle_linode_instance_ip_delete(
            {
                "linode_id": 123,
                "address": "192.0.2.1",
            },
            sample_config,
        )
    )
    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_instance_password_reset_tool_def() -> None:
    """Password reset should require linode_id, root_pass, confirm."""
    tool, _ = create_linode_instance_password_reset_tool()
    assert tool.name == "linode_instance_password_reset"
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    assert "root_pass" in required
    assert "confirm" in required


async def test_instance_password_reset_no_confirm(
    sample_config: Config,
) -> None:
    """Password reset should require confirm=true."""
    result = list(
        await handle_linode_instance_password_reset(
            {
                "linode_id": 123,
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
            {"linode_id": 123, "confirm": True},
            sample_config,
        )
    )
    assert len(result) == 1
    assert "root_pass" in result[0].text.lower()


async def test_execute_tool_missing_environment(sample_config: Config) -> None:
    """execute_tool returns an error when the requested environment doesn't exist."""
    result = await handle_linode_profile_get(
        {"environment": "nonexistent"}, sample_config
    )
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
    result = await handle_linode_profile_get({}, bad_config)
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
        mock_client.route_raw.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        await handle_linode_profile_get({}, sample_config)

        mock_client.__aenter__.assert_called_once()
        mock_client.__aexit__.assert_called_once()


async def test_execute_tool_callback_exception(sample_config: Config) -> None:
    """execute_tool catches handler exceptions and wraps them in error text."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_get({}, sample_config)

        assert len(result) == 1
        assert "Failed to" in result[0].text
        assert "boom" in result[0].text


async def test_instance_status_filter_returns_matching(
    sample_config: Config,
) -> None:
    """Filtering by status=running keeps only running instances."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "web-1", "status": "running"},
                {"id": 2, "label": "db-1", "status": "offline"},
                {"id": 3, "label": "web-2", "status": "running"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_list({"status": "running"}, sample_config)

    data = json.loads(result[0].text)
    assert data["count"] == 2
    labels = [inst["label"] for inst in data["instances"]]
    assert "web-1" in labels
    assert "web-2" in labels
    assert "db-1" not in labels


async def test_instance_no_filter_returns_all(
    sample_config: Config,
) -> None:
    """Without a status filter, all instances are returned."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 1, "label": "web-1", "status": "running"},
                {"id": 2, "label": "db-1", "status": "offline"},
            ]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_list({}, sample_config)

    data = json.loads(result[0].text)
    assert data["count"] == 2


async def test_region_capability_filter(sample_config: Config) -> None:
    """Filtering regions by capability keeps only matching regions."""
    raw_regions: dict[str, Any] = {
        "data": [
            {
                "id": "us-east",
                "label": "Newark",
                "country": "us",
                "capabilities": ["Linodes", "Kubernetes"],
                "status": "ok",
                "resolvers": {"ipv4": "8.8.8.8", "ipv6": "::1"},
                "site_type": "core",
            },
            {
                "id": "eu-west",
                "label": "London",
                "country": "uk",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "8.8.4.4", "ipv6": "::2"},
                "site_type": "core",
            },
            {
                "id": "us-west",
                "label": "Fremont",
                "country": "us",
                "capabilities": ["Linodes", "Kubernetes"],
                "status": "ok",
                "resolvers": {"ipv4": "1.1.1.1", "ipv6": "::3"},
                "site_type": "core",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list(
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
    raw_regions: dict[str, Any] = {
        "data": [
            {
                "id": "us-east",
                "label": "Newark",
                "country": "us",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "8.8.8.8", "ipv6": "::1"},
                "site_type": "core",
            },
            {
                "id": "eu-west",
                "label": "London",
                "country": "uk",
                "capabilities": ["Linodes"],
                "status": "ok",
                "resolvers": {"ipv4": "8.8.4.4", "ipv6": "::2"},
                "site_type": "core",
            },
        ]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_region_list({}, sample_config)

    data = json.loads(result[0].text)
    assert data["count"] == 2


async def test_handle_linode_instance_backup_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backup get should return backup data when both IDs are valid."""
    mock_linode_client.route_raw.return_value = {
        "id": 100,
        "label": "daily-backup",
        "status": "successful",
        "type": "auto",
    }
    result = await handle_linode_instance_backup_get(
        {"linode_id": 123, "backup_id": 100}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 100
    assert data["label"] == "daily-backup"
    mock_linode_client.route_raw.assert_any_call("linode_instance_backup_get", 123, 100)


async def test_handle_linode_instance_backups_cancel_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backups cancel should succeed with confirm=true."""
    mock_linode_client.route_call.return_value = None
    result = await handle_linode_instance_backups_cancel(
        {"linode_id": 123, "confirm": True}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert (
        data["message"]
        == "Backup service canceled for instance 123. All backups have been deleted."
    )
    assert data["linode_id"] == 123
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_instance_backups_cancel", 123
    )


async def test_instance_backups_cancel_dry_run_returns_preview(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """dry_run=true must read the instance through the declared GET, never cancel.

    The cancel is addressed by linode_id and the read by instance_id, so this
    also pins that the declared mapping fills the read with the id the caller
    named rather than a zero.
    """
    mock_linode_client.route_raw.return_value = {
        "id": 123,
        "label": "my-linode",
        "status": "running",
    }
    result = await handle_linode_instance_backups_cancel(
        {"linode_id": 123, "dry_run": True}, sample_config
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_backups_cancel"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/linode/instances/123/backups/cancel"
    mock_linode_client.route_raw.assert_awaited_once_with("linode_instance_get", 123)
    mock_linode_client.route_call.assert_not_called()


async def test_instance_backups_cancel_dry_run_still_validates_instance_id(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Missing linode_id must error regardless of dry_run."""
    result = await handle_linode_instance_backups_cancel(
        {"dry_run": True}, sample_config
    )
    assert "linode_id" in result[0].text.lower()
    mock_linode_client.route_call.assert_not_called()


async def test_handle_linode_instance_disk_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk get should return disk data when both IDs are valid."""
    mock_linode_client.route_raw.return_value = {
        "id": 10,
        "label": "Ubuntu Disk",
        "size": 51200,
        "filesystem": "ext4",
        "status": "ready",
    }
    result = await handle_linode_instance_disk_get(
        {"linode_id": 123, "disk_id": 10}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["id"] == 10
    assert data["label"] == "Ubuntu Disk"
    mock_linode_client.route_raw.assert_any_call("linode_instance_disk_get", 123, 10)


async def test_handle_linode_instance_ip_get_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP get should return IP data when linode_id and address are valid."""
    mock_linode_client.route_raw.return_value = {
        "address": "203.0.113.1",
        "type": "ipv4",
        "public": True,
        "region": "us-east",
    }
    result = await handle_linode_instance_ip_get(
        {"linode_id": 123, "address": "203.0.113.1"}, sample_config
    )
    assert len(result) == 1
    data = json.loads(result[0].text)
    assert data["address"] == "203.0.113.1"
    assert data["region"] == "us-east"
    mock_linode_client.route_raw.assert_called_once_with(
        "linode_instance_ip_get", 123, "203.0.113.1"
    )


async def test_instance_resize_dry_run_still_validates_type(
    sample_config: Config,
) -> None:
    """Missing type must error out regardless of dry_run."""
    result = await handle_linode_instance_resize(
        {"instance_id": 123, "dry_run": True}, sample_config
    )

    assert "type is required" in result[0].text


async def test_handle_linode_instance_backup_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Backup get should return error text when the API call fails."""
    mock_linode_client.route_raw.side_effect = Exception("API error")
    result = await handle_linode_instance_backup_get(
        {"linode_id": 123, "backup_id": 100}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_disk_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Disk get should return error text when the API call fails."""
    mock_linode_client.route_raw.side_effect = Exception("API error")
    result = await handle_linode_instance_disk_get(
        {"linode_id": 123, "disk_id": 10}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_instance_ip_get_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """IP get should return error text when the API call fails."""
    mock_linode_client.route_raw.side_effect = Exception("API error")
    result = await handle_linode_instance_ip_get(
        {"linode_id": 123, "address": "203.0.113.1"}, sample_config
    )
    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_monitor_services_list_tool() -> None:
    """Test linode_monitor_service_list tool creation."""
    tool, capability = create_linode_monitor_service_list_tool()
    assert tool.name == "linode_monitor_service_list"
    assert capability is Capability.Read
    assert tool.input_schema["type"] == "object"
    assert "environment" in tool.input_schema["properties"]
    assert "confirm" not in tool.input_schema["properties"]


async def test_handle_linode_monitor_services_list(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """Test linode_monitor_service_list tool handler."""
    mock_linode_client.route_raw.return_value = {
        "data": [{"label": "Databases", "service_type": "dbaas"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    result = await handle_linode_monitor_service_list({}, sample_config)

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["services"][0]["service_type"] == "dbaas"
    assert payload["services"][0]["label"] == "Databases"
    assert "results" not in payload
    assert "page" not in payload
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_monitor_service_list", query=""
    )


async def test_handle_linode_monitor_services_list_error(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """Test linode_monitor_service_list error handling."""
    mock_linode_client.route_raw.side_effect = Exception("API error")
    result = await handle_linode_monitor_service_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to retrieve items: API error" in result[0].text


def test_create_linode_monitor_service_get_tool() -> None:
    """Tool definition advertises required service_type without confirm."""
    tool, capability = create_linode_monitor_service_get_tool()
    assert tool.name == "linode_monitor_service_get"
    assert capability is Capability.Read
    schema = tool.input_schema
    assert "confirm" not in schema["properties"]
    assert schema["required"] == ["service_type"]
    assert "service_type" in schema["properties"]


async def test_handle_linode_monitor_service_get(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Handler returns monitor service data from a successful client call."""
    mock_linode_client.route_raw.return_value = {
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
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_monitor_service_get", "dbaas"
    )


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
    mock_linode_client.route_raw.side_effect = Exception("API error")
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
    schema = tool.input_schema
    assert "confirm" not in schema["properties"]
    assert sorted(schema["required"]) == ["alert_id", "service_type"]
    assert "service_type" in schema["properties"]
    assert schema["properties"]["alert_id"]["type"] == "integer"


async def test_handle_linode_monitor_service_alert_definition_get(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Handler returns alert definition data from a successful client call."""
    mock_linode_client.route_raw.return_value = {
        "id": 12345,
        "label": "CPU high",
        "service_type": "dbaas",
        "not_in_proto": "dropped",
    }
    result = await handle_linode_monitor_service_alert_definition_get(
        {"service_type": "dbaas", "alert_id": 12345}, sample_config
    )
    assert len(result) == 1
    text = result[0].text
    assert "CPU high" in text
    assert "dbaas" in text
    assert "not_in_proto" not in text
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_monitor_service_alert_definition_get", "dbaas", 12345
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


def test_create_linode_profile_tfa_disable_tool() -> None:
    """Profile TFA disable tool exposes a strict confirmation gate."""
    tool, capability = create_linode_profile_tfa_disable_tool()

    assert tool.name == "linode_profile_tfa_disable"
    assert capability is Capability.Admin
    assert tool.input_schema["required"] == ["confirm"]
    assert "environment" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
        mock_client.route_call.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_disable(
            {"confirm": True}, sample_config
        )

    expected = serialize_api_response(
        {"message": "Profile two-factor authentication disabled successfully"},
        common_pb2.MessageResponse(),
    )
    assert json.loads(result[0].text) == expected
    mock_client.route_call.assert_awaited_once_with(
        "linode_profile_tfa_disable", retry=False
    )


async def test_handle_linode_profile_tfa_disable_error(
    sample_config: Config,
) -> None:
    """Profile TFA disable surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_call.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_tfa_disable(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_phone_number_delete_tool() -> None:
    """Profile phone number delete tool exposes schema and write capability."""
    tool, capability = create_linode_profile_phone_number_delete_tool()

    assert tool.name == "linode_profile_phone_number_delete"
    assert capability is Capability.Admin
    assert tool.input_schema["required"] == ["confirm"]
    assert "environment" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
        mock_client.route_raw.assert_not_called()


async def test_handle_linode_profile_phone_number_delete_success(
    sample_config: Config,
) -> None:
    """Profile phone number delete calls the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_call.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_delete(
            {"confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == {
        "message": "Profile phone number deleted successfully"
    }
    mock_client.route_call.assert_awaited_once_with(
        "linode_profile_phone_number_delete", retry=False
    )


async def test_handle_linode_profile_phone_number_delete_error(
    sample_config: Config,
) -> None:
    """Profile phone number delete surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_call.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_phone_number_delete(
            {"confirm": True}, sample_config
        )

    assert len(result) == 1
    assert "Failed to delete profile phone number" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_security_questions_list_tool() -> None:
    """Profile security questions list tool exposes a read-only schema."""
    tool, capability = create_linode_profile_security_question_list_tool()

    assert tool.name == "linode_profile_security_question_list"
    assert capability == Capability.Read
    assert "required" not in tool.input_schema


async def test_handle_linode_profile_security_questions_list_success(
    sample_config: Config,
) -> None:
    """Profile security questions list handler emits the proto list envelope."""
    payload = {
        "security_questions": [
            {"id": 1, "question": "In what city were you born?", "response": "Gotham"},
            {"id": 2, "question": "What was your first pet's name?"},
        ]
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = payload
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    parsed = json.loads(result[0].text)
    assert parsed["count"] == 2
    assert "page" not in parsed
    assert parsed["security_questions"][0] == {
        "id": 1,
        "question": "In what city were you born?",
        "response": "Gotham",
    }
    # The unanswered question has no response, so the optional field is omitted.
    assert parsed["security_questions"][1] == {
        "id": 2,
        "question": "What was your first pet's name?",
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_security_question_list", query=""
    )


async def test_handle_linode_profile_security_questions_list_empty_envelope(
    sample_config: Config,
) -> None:
    """A missing security_questions key yields an empty proto list, not an error."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    assert json.loads(result[0].text) == {"count": 0, "security_questions": []}


@pytest.mark.parametrize(
    "api_response",
    [{"security_questions": None}, {"unrelated": True}],
    ids=["null", "extra-field"],
)
async def test_handle_linode_profile_security_questions_list_accepts_empty_shapes(
    sample_config: Config, api_response: dict[str, Any]
) -> None:
    """A null member is an empty list, the shape linodego's []T field decodes to."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = api_response
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    assert json.loads(result[0].text) == {"count": 0, "security_questions": []}


@pytest.mark.parametrize(
    "questions", [{}, "", 0, False], ids=["object", "string", "number", "boolean"]
)
async def test_handle_linode_profile_security_questions_list_rejects_falsey_non_arrays(
    sample_config: Config, questions: Any
) -> None:
    """Falsey non-array members are malformed responses, not empty lists."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {"security_questions": questions}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    assert result[0].text.startswith(
        "Failed to retrieve items: list response data must be an array"
    )


@pytest.mark.parametrize(
    "api_response",
    [[], None, "", 0, False],
    ids=["array", "null", "string", "number", "boolean"],
)
async def test_handle_linode_profile_security_questions_list_rejects_non_object(
    sample_config: Config, api_response: Any
) -> None:
    """A non-object root is rejected before the member lookup can raise."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = api_response
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    assert result[0].text.startswith(
        "Failed to retrieve items: list response must be an object"
    )


async def test_handle_linode_profile_security_questions_list_error(
    sample_config: Config,
) -> None:
    """Profile security questions list handler propagates client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_security_question_list({}, sample_config)

    assert "API error" in result[0].text


def test_create_linode_profile_tokens_list_tool() -> None:
    """Profile token list tool exposes only optional environment."""
    tool, capability = create_linode_profile_token_list_tool()

    assert tool.name == "linode_profile_token_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema
    assert "environment" in tool.input_schema["properties"]


async def test_handle_linode_profile_tokens_list_success(
    sample_config: Config,
) -> None:
    """Profile token list emits the proto envelope and never leaks a secret."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {
            "data": [
                {
                    "id": 12345,
                    "label": "api-token",
                    "scopes": "linodes:read_write",
                    "token": "secret-token",
                    "access_token": "secret-access-token",
                    "secret": "secret-value",
                },
                {"id": 67890, "label": "ci-token"},
            ]
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_list({}, sample_config)

    parsed = json.loads(result[0].text)
    # The proto PersonalAccessToken models no secret field, so every secret the
    # API returned is dropped by construction.
    assert "secret-token" not in result[0].text
    assert "secret-access-token" not in result[0].text
    assert "secret-value" not in result[0].text
    assert parsed["count"] == 2
    assert "page" not in parsed
    # scopes and created are implicit-presence strings (emit their zero value);
    # expiry is optional, so it is omitted when unset.
    assert parsed["profile_tokens"][0] == {
        "id": 12345,
        "label": "api-token",
        "scopes": "linodes:read_write",
        "created": "",
    }
    assert parsed["profile_tokens"][1] == {
        "id": 67890,
        "label": "ci-token",
        "scopes": "",
        "created": "",
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_token_list", query=""
    )


async def test_handle_linode_profile_tokens_list_error(
    sample_config: Config,
) -> None:
    """Profile token list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_profile_token_list_threads_pagination(
    sample_config: Config,
) -> None:
    """Profile token list passes a validated page/page_size pair to the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {"data": []}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_list(
            {"page": 3, "page_size": 50}, sample_config
        )

    parsed = json.loads(result[0].text)
    assert parsed == {"count": 0, "profile_tokens": []}
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_token_list", query="page=3&page_size=50"
    )


def test_create_linode_profile_token_get_tool() -> None:
    """Profile token get tool exposes token_id."""
    tool, capability = create_linode_profile_token_get_tool()

    assert tool.name == "linode_profile_token_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["token_id"]
    assert tool.input_schema["properties"]["token_id"]["type"] == "integer"


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
        mock_client.route_raw.return_value = {
            "id": 12345,
            "label": "api-token",
            "scopes": "*",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert json.loads(result[0].text) == {
        "id": 12345,
        "label": "api-token",
        "scopes": "*",
        "created": "",
    }
    mock_client.route_raw.assert_awaited_once_with("linode_profile_token_get", 12345)


async def test_handle_linode_profile_token_get_redacts_secret_fields(
    sample_config: Config,
) -> None:
    """Profile token get does not expose secret token material."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {
            "id": 12345,
            "label": "api-token",
            "scopes": "*",
            "token": "secret-token",
            "access_token": "secret-access-token",
            "secret": "secret-value",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert json.loads(result[0].text) == {
        "id": 12345,
        "label": "api-token",
        "scopes": "*",
        "created": "",
    }
    assert "secret-token" not in result[0].text
    assert "secret-access-token" not in result[0].text
    assert "secret-value" not in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_profile_token_get", 12345)


async def test_handle_linode_profile_token_get_error(
    sample_config: Config,
) -> None:
    """Profile token get surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_token_get(
            {"token_id": 12345}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_logins_list_tool() -> None:
    """Profile login list tool exposes only environment arguments."""
    tool, capability = create_linode_profile_login_list_tool()

    assert tool.name == "linode_profile_login_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema
    assert "environment" in tool.input_schema["properties"]


async def test_handle_linode_profile_logins_list_success(
    sample_config: Config,
) -> None:
    """Profile login list calls the retryable client and returns logins."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {
            "data": [
                {"id": 12345, "ip": "192.0.2.10"},
                {"id": 67890, "ip": "192.0.2.11"},
            ]
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_list({}, sample_config)

    assert json.loads(result[0].text) == {
        "count": 2,
        "profile_logins": [
            {
                "datetime": "",
                "id": 12345,
                "ip": "192.0.2.10",
                "restricted": False,
                "status": "",
                "username": "",
            },
            {
                "datetime": "",
                "id": 67890,
                "ip": "192.0.2.11",
                "restricted": False,
                "status": "",
                "username": "",
            },
        ],
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_login_list", query=""
    )


async def test_handle_linode_profile_logins_list_empty(
    sample_config: Config,
) -> None:
    """Profile login list returns an empty proto envelope when there are no logins."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {"data": []}
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_list(
            {"page": 1, "page_size": 25}, sample_config
        )

    assert json.loads(result[0].text) == {"count": 0, "profile_logins": []}
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_login_list", query="page=1&page_size=25"
    )


async def test_handle_linode_profile_logins_list_error(
    sample_config: Config,
) -> None:
    """Profile login list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_login_get_tool() -> None:
    """Profile login get tool exposes login_id."""
    tool, capability = create_linode_profile_login_get_tool()

    assert tool.name == "linode_profile_login_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["login_id"]
    assert "login_id" in tool.input_schema["properties"]


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
        mock_client.route_raw.return_value = {
            "id": 12345,
            "ip": "192.0.2.10",
            "datetime": "2024-01-02T03:04:05",
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_get(
            {"login_id": 12345}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["id"] == 12345
    assert body["ip"] == "192.0.2.10"
    assert body["datetime"] == "2024-01-02T03:04:05"
    assert body["restricted"] is False
    assert body["username"] == ""
    mock_client.route_raw.assert_awaited_once_with("linode_profile_login_get", 12345)


async def test_handle_linode_profile_login_get_error(
    sample_config: Config,
) -> None:
    """Profile login get surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_login_get(
            {"login_id": 12345}, sample_config
        )

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_devices_list_tool() -> None:
    """Profile trusted device list tool exposes only environment arguments."""
    tool, capability = create_linode_profile_device_list_tool()

    assert tool.name == "linode_profile_device_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema
    assert "environment" in tool.input_schema["properties"]


async def test_handle_linode_profile_devices_list_success(
    sample_config: Config,
) -> None:
    """Profile trusted device list emits the proto list envelope."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.return_value = {
            "data": [
                {
                    "id": 123,
                    "created": "2024-05-01T00:01:01",
                    "expiry": "2024-08-01T00:01:01",
                    "last_authenticated": "2024-06-01T00:01:01",
                    "last_remote_addr": "192.0.2.1",
                    "user_agent": "Mozilla/5.0",
                },
                {"id": 456, "user_agent": "curl/8.0"},
            ]
        }
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_list({}, sample_config)

    parsed = json.loads(result[0].text)
    assert parsed["count"] == 2
    assert "page" not in parsed
    assert parsed["profile_devices"][0] == {
        "id": 123,
        "created": "2024-05-01T00:01:01",
        "expiry": "2024-08-01T00:01:01",
        "last_authenticated": "2024-06-01T00:01:01",
        "last_remote_addr": "192.0.2.1",
        "user_agent": "Mozilla/5.0",
    }
    # The second device sets only id and user_agent. The implicit-presence
    # string fields emit their zero value (Go's EmitDefaultValues), while expiry
    # is optional, so it is omitted when the API returns null for a non-expiring
    # device.
    assert parsed["profile_devices"][1] == {
        "id": 456,
        "created": "",
        "last_authenticated": "",
        "last_remote_addr": "",
        "user_agent": "curl/8.0",
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_device_list", query=""
    )


async def test_handle_linode_profile_devices_list_error(
    sample_config: Config,
) -> None:
    """Profile trusted device list surfaces client errors."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_list({}, sample_config)

    assert len(result) == 1
    assert "Failed to" in result[0].text
    assert "API error" in result[0].text


async def test_handle_linode_profile_device_list_rejects_bad_pagination(
    sample_config: Config,
) -> None:
    """Profile device list rejects an out-of-range page_size before the call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_list(
            {"page_size": 10}, sample_config
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    mock_client.route_raw.assert_not_awaited()


def test_create_linode_profile_apps_list_tool() -> None:
    tool, capability = create_linode_profile_app_list_tool()

    assert tool.name == "linode_profile_app_list"
    assert capability is Capability.Read
    assert "required" not in tool.input_schema
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


def test_linode_profile_apps_list_tool_is_exported_and_registered() -> None:
    from linodemcp import gentools as gentools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_profile_app_list_tool" in gentools_mod.__all__
    assert "handle_linode_profile_app_list" in gentools_mod.__all__
    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_profile_app_list"].capability is Capability.Read


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page": True}, "page must be an integer"),
        ({"page": "1"}, "page must be an integer"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
        ({"page_size": True}, "page_size must be an integer"),
        ({"page_size": "50"}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_profile_apps_list_rejects_invalid_pagination(
    arguments: dict[str, object], message: str, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_profile_app_list(arguments, sample_config)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_profile_apps_list_success(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 123, "label": "authorized-app"}],
            "page": 2,
            "pages": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_list(
            {"page": 2, "page_size": 50}, sample_config
        )

    assert json.loads(result[0].text) == {
        "count": 1,
        "profile_apps": [
            {
                "id": 123,
                "label": "authorized-app",
                "scopes": "",
                "website": "",
            }
        ],
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_profile_app_list", query="page=2&page_size=50"
    )


async def test_handle_linode_profile_apps_list_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_list({}, sample_config)

    assert "Failed to retrieve items: " in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_app_get_tool() -> None:
    tool, capability = create_linode_profile_app_get_tool()

    assert tool.name == "linode_profile_app_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["app_id"]
    assert "app_id" in tool.input_schema["properties"]


def test_linode_profile_app_get_tool_is_exported_and_registered() -> None:
    from linodemcp import gentools as gentools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_profile_app_get_tool" in gentools_mod.__all__
    assert "handle_linode_profile_app_get" in gentools_mod.__all__
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
        mock_client.route_raw.return_value = {
            "id": 123,
            "label": "authorized-app",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_get({"app_id": 123}, sample_config)

    body = json.loads(result[0].text)
    assert body["id"] == 123
    assert body["label"] == "authorized-app"
    assert body["scopes"] == ""
    assert body["website"] == ""
    mock_client.route_raw.assert_awaited_once_with("linode_profile_app_get", 123)


async def test_handle_linode_profile_app_get_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_app_get({"app_id": 123}, sample_config)

    assert "Failed to retrieve authorized app 123: " in result[0].text
    assert "API error" in result[0].text


def test_create_linode_profile_device_get_tool() -> None:
    tool, capability = create_linode_profile_device_get_tool()

    assert tool.name == "linode_profile_device_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["device_id"]
    assert tool.input_schema["properties"]["device_id"]["type"] == "integer"


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
        mock_client.route_raw.return_value = {
            **device,
            "not_in_proto": "dropped",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_get(
            {"device_id": 123}, sample_config
        )

    assert json.loads(result[0].text) == device
    assert "not_in_proto" not in result[0].text
    mock_client.route_raw.assert_awaited_once_with("linode_profile_device_get", 123)


async def test_handle_linode_profile_device_get_error(sample_config: Config) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile_device_get(
            {"device_id": 123}, sample_config
        )

    assert "Failed to retrieve trusted device 123: " in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_groups_list_tool() -> None:
    """Placement groups list tool schema supports optional pagination."""
    tool, capability = create_linode_placement_group_list_tool()

    assert tool.name == "linode_placement_group_list"
    assert capability is Capability.Read
    assert "page" not in tool.input_schema.get("required", [])
    assert "page_size" not in tool.input_schema.get("required", [])
    assert "environment" in tool.input_schema["properties"]
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]


def test_linode_placement_groups_list_tool_is_exported_and_registered() -> None:
    """Placement groups list tool is exported and registered."""
    from linodemcp import gentools as gentools_mod
    from linodemcp.server import get_tool_registry

    assert "create_linode_placement_group_list_tool" in gentools_mod.__all__
    assert "handle_linode_placement_group_list" in gentools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_list"].capability is Capability.Read


@pytest.mark.parametrize(
    ("arguments", "error"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page": True}, "page must be an integer"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
        ({"page_size": False}, "page_size must be an integer"),
        ({"page_size": "25"}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_placement_groups_list_rejects_invalid_pagination(
    arguments: dict[str, Any], error: str, sample_config: Config
) -> None:
    """Placement groups list validates pagination arguments."""
    result = await handle_linode_placement_group_list(arguments, sample_config)

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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == {
        "count": 1,
        "placement_groups": [
            {
                "id": 123,
                "label": "pg-a",
                "region": "",
                "placement_group_type": "",
                "placement_group_policy": "",
                "is_compliant": False,
                "members": [],
            }
        ],
    }
    mock_client.route_raw.assert_awaited_once_with(
        "linode_placement_group_list", query="page=2&page_size=25"
    )


async def test_handle_linode_placement_groups_list_reports_client_errors(
    sample_config: Config,
) -> None:
    """Placement groups list handler reports client exceptions."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_list({}, sample_config)

    assert len(result) == 1
    assert "API error" in result[0].text


def test_create_linode_placement_group_get_tool() -> None:
    """Placement group get tool schema requires only group_id."""
    tool, capability = create_linode_placement_group_get_tool()

    assert tool.name == "linode_placement_group_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["group_id"]
    assert "group_id" in tool.input_schema["properties"]


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
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_get(
            {"group_id": 789},
            sample_config,
        )

    data = json.loads(result[0].text)
    assert data["id"] == 789
    assert data["label"] == "pg-a"
    assert data["members"] == []
    assert "migrations" not in data
    mock_client.route_raw.assert_awaited_once_with("linode_placement_group_get", 789)


async def test_handle_linode_placement_group_get_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_get(
            {"group_id": 789},
            sample_config,
        )

    assert "Failed to retrieve placement group 789" in result[0].text
    assert "API error" in result[0].text


def test_create_linode_placement_group_create_tool() -> None:
    """Placement group create tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_create_tool()

    assert tool.name == "linode_placement_group_create"
    assert capability is Capability.Write
    assert set(tool.input_schema["required"]) == {
        "label",
        "region",
        "placement_group_type",
        "placement_group_policy",
        "confirm",
    }
    for key in (
        "label",
        "region",
        "placement_group_type",
        "placement_group_policy",
        "confirm",
    ):
        assert key in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
    ("label", "message"),
    [
        # The label rules live on PlacementGroupCreateInput: absence answers
        # the contract's "label is required", a value of another type falls to
        # the body builder's sentence, and a bad shape answers the pattern.
        (None, "label is required"),
        ("", "label is required"),
        (True, "label must be a string"),
        (1, "label must be a string"),
        ([], "label must be a string"),
        ({}, "label must be a string"),
        ("/", "label must start and end"),
        ("?", "label must start and end"),
        ("..", "label must start and end"),
        ("bad/label", "label must start and end"),
        ("bad?label", "label must start and end"),
    ],
)
async def test_handle_linode_placement_group_create_requires_valid_label(
    label: object, message: str, sample_config: Config
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

    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("region", "message"),
    [
        (None, "region is required"),
        ("", "region is required"),
        (True, "region must be a string"),
        (1, "region must be a string"),
        ([], "region must be a string"),
        ({}, "region must be a string"),
    ],
)
async def test_handle_linode_placement_group_create_requires_valid_region(
    region: object, message: str, sample_config: Config
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

    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("placement_group_type", "message"),
    [
        # The membership reader reads the raw argument: an absent, non-string,
        # or empty value answers the required sentence, a string outside the
        # declared reader_values the vocabulary one.
        (None, "placement_group_type is required"),
        ("", "placement_group_type is required"),
        (1, "placement_group_type is required"),
        ("affinity:local", "placement_group_type must be anti_affinity:local"),
        ("anti-affinity:local", "placement_group_type must be anti_affinity:local"),
    ],
)
async def test_handle_linode_placement_group_create_requires_valid_type(
    placement_group_type: object, message: str, sample_config: Config
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

    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("placement_group_policy", "message"),
    [
        (None, "placement_group_policy is required"),
        ("", "placement_group_policy is required"),
        (1, "placement_group_policy is required"),
        ("best-effort", "placement_group_policy must be one of: flexible, strict"),
        ("STRICT", "placement_group_policy must be one of: flexible, strict"),
    ],
)
async def test_handle_linode_placement_group_create_requires_valid_policy(
    placement_group_policy: object, message: str, sample_config: Config
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

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_create_success(
    sample_config: Config,
) -> None:
    response_data = {"id": 789, "label": "pg-a"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
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

    body = json.loads(result[0].text)
    assert body["message"] == "Placement group 'pg-a' created successfully"
    assert body["placement_group"]["id"] == 789
    assert body["placement_group"]["label"] == "pg-a"
    assert body["placement_group"]["members"] == []
    # A replayed create bills for a second group, so the route is never retried.
    mock_client.route_raw.assert_awaited_once_with(
        "linode_placement_group_create",
        body={
            "label": "pg-a",
            "region": "us-mia",
            "placement_group_type": "anti_affinity:local",
            "placement_group_policy": "strict",
        },
        retry=False,
    )


async def test_handle_linode_placement_group_create_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
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


def test_create_linode_placement_group_update_tool() -> None:
    """Placement group update tool schema requires confirmation."""
    tool, capability = create_linode_placement_group_update_tool()

    assert tool.name == "linode_placement_group_update"
    assert capability is Capability.Write
    assert set(tool.input_schema["required"]) == {"group_id", "label", "confirm"}
    assert "group_id" in tool.input_schema["properties"]
    assert "label" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
    ("label", "message"),
    [
        (None, "label must be a non-empty string"),
        ("", "label must be a non-empty string"),
        (True, "label must be a non-empty string"),
        (1, "label must be a non-empty string"),
        ([], "label must be a non-empty string"),
        ({}, "label must be a non-empty string"),
        ("/", "label must start and end"),
        ("?", "label must start and end"),
        ("..", "label must start and end"),
        ("bad/label", "label must start and end"),
        ("bad?label", "label must start and end"),
    ],
)
async def test_handle_linode_placement_group_update_requires_valid_label(
    label: object, message: str, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_update(
            {"group_id": 789, "label": label, "confirm": True},
            sample_config,
        )

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_update_success(
    sample_config: Config,
) -> None:
    response_data = {"id": 789, "label": "new-label"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_update(
            {"group_id": 789, "label": "new-label", "confirm": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["id"] == 789
    assert body["label"] == "new-label"
    assert body["members"] == []
    mock_client.route_raw.assert_awaited_once_with(
        "linode_placement_group_update",
        789,
        body={"label": "new-label"},
        retry=False,
    )


async def test_handle_linode_placement_group_update_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
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
    assert set(tool.input_schema["required"]) == {"group_id", "confirm"}
    assert "group_id" in tool.input_schema["properties"]
    assert "linodes" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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


# The two arms the membership routes' id list tells apart: a value that is not
# a list at all, and a list the route cannot use.
PLACEMENT_LINODES_ARRAY = "linodes must be a JSON array of positive integer Linode IDs"
PLACEMENT_LINODES_SHAPE = (
    "linodes must be a non-empty array of distinct positive integer Linode IDs"
)


@pytest.mark.parametrize(
    ("linodes", "expected"),
    [
        ([], PLACEMENT_LINODES_SHAPE),
        ([0], PLACEMENT_LINODES_SHAPE),
        ([-1], PLACEMENT_LINODES_SHAPE),
        ([True], PLACEMENT_LINODES_SHAPE),
        (["123"], PLACEMENT_LINODES_SHAPE),
        ([123, 123], PLACEMENT_LINODES_SHAPE),
        (None, PLACEMENT_LINODES_ARRAY),
        ("/", PLACEMENT_LINODES_ARRAY),
        ("?", PLACEMENT_LINODES_ARRAY),
        ("..", PLACEMENT_LINODES_ARRAY),
    ],
)
async def test_handle_linode_placement_group_assign_requires_linode_ids(
    linodes: object, expected: str, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_assign(
            {"group_id": 789, "linodes": linodes, "confirm": True}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_assign_success(
    sample_config: Config,
) -> None:
    response_data = {"linodes": [123, 456]}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_assign(
            {"group_id": 789, "linodes": [123, 456], "confirm": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["message"] == "Assigned 2 Linode(s) to placement group 789"
    assert body["placement_group"]["members"] == []
    mock_client.route_raw.assert_awaited_once_with(
        "linode_placement_group_assign",
        789,
        body={"linodes": [123, 456]},
        retry=False,
    )


async def test_handle_linode_placement_group_assign_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
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
    assert set(tool.input_schema["required"]) == {"group_id", "confirm"}
    assert "group_id" in tool.input_schema["properties"]
    assert "linodes" in tool.input_schema["properties"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


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
    ("linodes", "expected"),
    [
        ([], PLACEMENT_LINODES_SHAPE),
        ([0], PLACEMENT_LINODES_SHAPE),
        ([-1], PLACEMENT_LINODES_SHAPE),
        ([True], PLACEMENT_LINODES_SHAPE),
        (["123"], PLACEMENT_LINODES_SHAPE),
        ([123, 123], PLACEMENT_LINODES_SHAPE),
        (None, PLACEMENT_LINODES_ARRAY),
        ("/", PLACEMENT_LINODES_ARRAY),
        ("?", PLACEMENT_LINODES_ARRAY),
        ("..", PLACEMENT_LINODES_ARRAY),
    ],
)
async def test_handle_linode_placement_group_unassign_requires_linode_ids(
    linodes: object, expected: str, sample_config: Config
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": linodes, "confirm": True}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_placement_group_unassign_success(
    sample_config: Config,
) -> None:
    response_data = {"linodes": [123, 456]}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": [123, 456], "confirm": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["message"] == "Linodes unassigned from placement group 789 successfully"
    assert body["placement_group"]["members"] == []
    mock_client.route_raw.assert_awaited_once_with(
        "linode_placement_group_unassign",
        789,
        body={"linodes": [123, 456]},
        retry=False,
    )


async def test_handle_linode_placement_group_unassign_reports_client_errors(
    sample_config: Config,
) -> None:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = RuntimeError("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_placement_group_unassign(
            {"group_id": 789, "linodes": [123], "confirm": True},
            sample_config,
        )

    assert "Failed to unassign placement group 789" in result[0].text
    assert "API error" in result[0].text


async def test_create_linode_nodebalancer_stats_tool_definition() -> None:
    """Test linode_nodebalancer_stats_get tool definition."""
    tool, capability = create_linode_nodebalancer_stats_get_tool()
    assert tool.name == "linode_nodebalancer_stats_get"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_stats(sample_config: Config) -> None:
    """Test linode_nodebalancer_stats_get tool."""
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
        mock_client.route_raw.return_value = mock_stats
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_stats_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        content = result[0].text
        assert "connections" in content
        assert "traffic" in content
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_stats_get", 1
        )


async def test_handle_linode_nodebalancer_stats_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_stats_get tool with missing ID."""
    result = await handle_linode_nodebalancer_stats_get({}, sample_config)
    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_nodebalancer_stats_error(sample_config: Config) -> None:
    """Test linode_nodebalancer_stats_get tool error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_stats_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_linode_nodebalancer_firewalls_list_tool_definition() -> None:
    """Test linode_nodebalancer_firewall_list tool definition."""
    tool, capability = create_linode_nodebalancer_firewall_list_tool()

    assert tool.name == "linode_nodebalancer_firewall_list"
    assert capability == Capability.Read
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["nodebalancer_id"]


async def test_handle_linode_nodebalancer_firewalls_list(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewall_list tool."""
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
        mock_client.route_raw.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewall_list(
            {"nodebalancer_id": 8, "page": 1, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        data = json.loads(result[0].text)
        assert data == {
            "count": 1,
            "firewalls": [
                {
                    "id": 123,
                    "label": "web-fw",
                    "status": "enabled",
                    "rules": {
                        "inbound": [],
                        "inbound_policy": "",
                        "outbound": [],
                        "outbound_policy": "",
                    },
                    "tags": [],
                    "created": "2024-01-01T00:00:00",
                    "updated": "2024-01-01T00:00:00",
                }
            ],
        }
        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_firewall_list", 8, query="page=1&page_size=25"
        )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "nodebalancer_id is required"),
        ({"nodebalancer_id": 0}, "nodebalancer_id"),
        ({"nodebalancer_id": "8"}, "nodebalancer_id"),
        ({"nodebalancer_id": True}, "nodebalancer_id"),
        ({"nodebalancer_id": "1/2"}, "nodebalancer_id"),
        ({"nodebalancer_id": "1?x"}, "nodebalancer_id"),
        ({"nodebalancer_id": ".."}, "nodebalancer_id"),
        (
            {"nodebalancer_id": 8, "page": 0},
            "page must be an integer greater than or equal to 1",
        ),
        ({"nodebalancer_id": 8, "page": "1"}, "page must be an integer"),
        (
            {"nodebalancer_id": 8, "page_size": 24},
            "page_size must be an integer from 25 through 500",
        ),
        (
            {"nodebalancer_id": 8, "page_size": 501},
            "page_size must be an integer from 25 through 500",
        ),
        ({"nodebalancer_id": 8, "page_size": False}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_nodebalancer_firewalls_list_invalid_arguments(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """NodeBalancer firewall list rejects invalid arguments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_nodebalancer_firewall_list(
            arguments, sample_config
        )

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_linode_nodebalancer_config_node_get_tool_definition() -> None:
    """Test linode_nodebalancer_config_node_get tool definition."""
    tool, _capability = create_linode_nodebalancer_config_node_get_tool()
    assert tool.name == "linode_nodebalancer_config_node_get"
    assert "nodebalancer_id" in tool.input_schema["properties"]
    assert "config_id" in tool.input_schema["properties"]
    assert "node_id" in tool.input_schema["properties"]
    assert set(tool.input_schema["required"]) == {
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
        mock_client.route_raw.return_value = {
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

        mock_client.route_raw.assert_called_once_with(
            "linode_nodebalancer_config_node_get", 8, 6, 4
        )
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
        mock_client.route_raw.side_effect = Exception("API error")

        result = await handle_linode_nodebalancer_config_node_get(
            {"nodebalancer_id": 8, "config_id": 6, "node_id": 4},
            sample_config,
        )
        response = result[0].text
        assert "error" in response.lower()


async def test_handle_linode_nodebalancer_firewalls_list_error(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_firewall_list error handling."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_firewall_list(
            {"nodebalancer_id": 8}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_firewall_rule_version_get(
    sample_config: Config,
) -> None:
    """Test the firewall rule version get tool handler.

    The /history/rules/{version} endpoint returns one rule-version snapshot: a
    firewall-shaped object with a top-level version and the full ruleset. The
    handler decodes it into the FirewallRuleVersion proto element, the same
    element the rule-version LIST path emits.
    """
    from linodemcp.gentools import handle_linode_firewall_rule_version_get

    raw_rule_version: dict[str, Any] = {
        "id": 12345,
        "label": "web-firewall",
        "status": "enabled",
        "version": 2,
        "rules": {
            "inbound": [
                {
                    "action": "ACCEPT",
                    "protocol": "TCP",
                    "ports": "22",
                    "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                    "label": "allow-ssh",
                    "description": "Allow SSH traffic",
                }
            ],
            "inbound_policy": "DROP",
            "outbound": [],
            "outbound_policy": "ACCEPT",
        },
        "tags": [],
        "created": "2025-01-01T00:00:00",
        "updated": "2025-01-02T00:00:00",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_rule_version
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_rule_version_get(
            {"firewall_id": 12345, "version": 2}, sample_config
        )
        assert len(result) == 1
        mock_client.route_raw.assert_awaited_once_with(
            "linode_firewall_rule_version_get", 12345, 2
        )
        data = json.loads(result[0].text)
        # The full FirewallRuleVersion envelope, not a single curated rule.
        assert data["version"] == 2
        assert data["label"] == "web-firewall"
        assert data["rules"]["inbound"][0]["label"] == "allow-ssh"


async def test_handle_linode_firewall_rule_version_get_missing_args(
    sample_config: Config,
) -> None:
    """Test the firewall rule version get tool rejects missing arguments."""
    from linodemcp.gentools import handle_linode_firewall_rule_version_get

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
        {"firewall_id": 0, "version": 1}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": "abc", "version": 1}, sample_config
    )
    assert len(result) == 1
    assert "firewall_id must be a positive integer" in result[0].text

    # version must be a positive integer, the same as firewall_id.
    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": 12345, "version": "v1"}, sample_config
    )
    assert len(result) == 1
    assert "version must be a positive integer" in result[0].text

    result = await handle_linode_firewall_rule_version_get(
        {"firewall_id": 12345, "version": 0}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text


def test_create_linode_firewall_template_get_tool_schema() -> None:
    """Test linode_firewall_template_get tool schema."""
    tool, capability = create_linode_firewall_template_get_tool()

    assert tool.name == "linode_firewall_template_get"
    assert capability is Capability.Read
    assert "slug" in tool.input_schema["properties"]
    assert "slug" in tool.input_schema["required"]
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]


async def test_handle_linode_firewall_template_get(sample_config: Config) -> None:
    """Test linode_firewall_template_get tool.

    The by-slug template endpoint returns a single bare template object. The
    handler decodes it into the FirewallTemplate proto element ({slug, rules}),
    the same element the template LIST path emits.
    """
    raw_template: dict[str, Any] = {
        "slug": "public",
        "label": "Allow HTTP",
        "description": "Allow HTTP traffic on port 80",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_template
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_template_get(
            {"slug": "public"}, sample_config
        )

        assert len(result) == 1
        # The proto element carries slug + the full ruleset (label/description are
        # not part of the FirewallTemplate proto, so they are dropped).
        assert '"slug": "public"' in result[0].text
        assert '"inbound_policy": "DROP"' in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_firewall_template_get", "public"
        )


async def test_handle_linode_firewall_template_get_with_pagination(
    sample_config: Config,
) -> None:
    """Template get appends page/page_size to the request the way Go does."""
    raw_template: dict[str, Any] = {
        "slug": "vpc",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = raw_template
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_template_get(
            {"slug": "vpc", "page": 2, "page_size": 25}, sample_config
        )

        assert len(result) == 1
        assert '"slug": "vpc"' in result[0].text
        mock_client.route_raw.assert_awaited_once_with(
            "linode_firewall_template_get", "vpc", query="page=2&page_size=25"
        )


async def test_handle_linode_firewall_template_get_rejects_bad_pagination(
    sample_config: Config,
) -> None:
    """Template get now rejects through the shared reader, bounds included."""
    result = await handle_linode_firewall_template_get(
        {"slug": "public", "page": 0}, sample_config
    )
    assert len(result) == 1
    assert "page must be an integer greater than or equal to 1" in result[0].text


async def test_handle_linode_firewall_template_get_rejects_invalid_pagination(
    sample_config: Config,
) -> None:
    """The shared bounds apply here too: page_size outside 25-500 is rejected."""
    result = await handle_linode_firewall_template_get(
        {"slug": "public", "page": -1}, sample_config
    )
    assert len(result) == 1
    assert "page must be an integer greater than or equal to 1" in result[0].text

    result = await handle_linode_firewall_template_get(
        {"slug": "public", "page_size": 0}, sample_config
    )
    assert len(result) == 1
    assert "page_size must be an integer from 25 through 500" in result[0].text

    result = await handle_linode_firewall_template_get(
        {"slug": "public", "page_size": 501}, sample_config
    )
    assert len(result) == 1
    assert "page_size must be an integer from 25 through 500" in result[0].text


async def test_handle_linode_firewall_device_get(
    sample_config: Config,
) -> None:
    """Test the firewall device get tool handler."""
    from linodemcp.gentools import handle_linode_firewall_device_get

    mock_device = {
        "id": 456,
        "entity": {
            "id": 123,
            "label": "linode-123",
            "type": "linode",
            "url": "/v4/linode/instances/123",
        },
        "created": "2018-01-01T01:01:01",
        "updated": "2018-01-01T01:01:01",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_device
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_device_get(
            {"firewall_id": 12345, "device_id": 456}, sample_config
        )
        assert len(result) == 1
        mock_client.route_raw.assert_awaited_once_with(
            "linode_firewall_device_get", 12345, 456
        )
        data = json.loads(result[0].text)
        assert data["id"] == 456
        assert data["entity"]["label"] == "linode-123"
        assert data["entity"]["type"] == "linode"
        assert "parent_entity" not in data["entity"]


async def test_handle_linode_firewall_device_get_missing_args(
    sample_config: Config,
) -> None:
    """Test the firewall device get tool rejects missing arguments."""
    from linodemcp.gentools import handle_linode_firewall_device_get

    result = await handle_linode_firewall_device_get(
        {"firewall_id": 12345}, sample_config
    )
    assert len(result) == 1
    assert "device id must be a positive integer" in result[0].text

    result = await handle_linode_firewall_device_get({"device_id": 456}, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    result = await handle_linode_firewall_device_get({}, sample_config)
    assert len(result) == 1
    assert "firewall_id is required" in result[0].text

    result = await handle_linode_firewall_device_get(
        {"firewall_id": True, "device_id": 456}, sample_config
    )
    assert len(result) == 1
    assert "positive integer" in result[0].text or "valid integer" in result[0].text

    result = await handle_linode_firewall_device_get(
        {"firewall_id": 12345, "device_id": "abc"}, sample_config
    )
    assert len(result) == 1
    assert "device id must be a positive integer" in result[0].text


async def test_handle_linode_firewall_devices_list(sample_config: Config) -> None:
    """Test firewall devices list handler."""
    from linodemcp.gentools import handle_linode_firewall_device_list

    mock_devices = {"data": [{"id": 123}], "page": 1, "pages": 1, "results": 1}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_devices
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_device_list(
            {"firewall_id": 12345}, sample_config
        )

    assert len(result) == 1
    result_data = json.loads(result[0].text)
    assert result_data["count"] == 1
    assert "filter" not in result_data
    assert result_data["devices"][0]["id"] == 123
    mock_client.route_raw.assert_awaited_once_with(
        "linode_firewall_device_list", 12345, query=""
    )


async def test_handle_linode_firewall_devices_list_with_pagination(
    sample_config: Config,
) -> None:
    """Test firewall devices list handler pagination."""
    from linodemcp.gentools import handle_linode_firewall_device_list

    mock_devices: dict[str, Any] = {"data": [], "page": 2, "pages": 5, "results": 0}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = mock_devices
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_device_list(
            {"firewall_id": 12345, "page": 2, "page_size": 25}, sample_config
        )

    result_data = json.loads(result[0].text)
    assert result_data == {"count": 0, "devices": []}
    mock_client.route_raw.assert_awaited_once_with(
        "linode_firewall_device_list", 12345, query="page=2&page_size=25"
    )


async def test_handle_linode_firewall_rule_version_list(
    sample_config: Config,
) -> None:
    """The single history object becomes one snapshot with rules.version lifted."""
    from linodemcp.gentools import handle_linode_firewall_rule_version_list

    history: dict[str, Any] = {
        "id": 7,
        "label": "prod-fw",
        "status": "enabled",
        "rules": {
            "inbound": [],
            "inbound_policy": "DROP",
            "outbound": [],
            "outbound_policy": "ACCEPT",
            "version": 2,
        },
        "tags": ["edge"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-02T00:00:00",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = history
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        result = await handle_linode_firewall_rule_version_list(
            {"firewall_id": 7}, sample_config
        )
    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["firewall_rule_versions"][0]["version"] == 2
    assert body["firewall_rule_versions"][0]["rules"]["inbound_policy"] == "DROP"
    assert body["firewall_rule_versions"][0]["tags"] == ["edge"]
    mock_client.route_raw.assert_awaited_once_with(
        "linode_firewall_rule_version_list", 7, query=""
    )


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"firewall_id": True},
        {"firewall_id": 0},
        {"firewall_id": -3},
        {"firewall_id": "nope"},
    ],
)
async def test_handle_linode_firewall_rule_version_list_invalid(
    sample_config: Config, arguments: dict[str, Any]
) -> None:
    """Every unusable firewall_id answers the one sentence the rule declares.

    The five shapes used to split three ways across a hand-written reader. The
    declared rule speaks for all of them now, which is the sentence the Go twin
    already answered.
    """
    expected = "firewall_id must be a positive integer"
    from linodemcp.gentools import handle_linode_firewall_rule_version_list

    result = await handle_linode_firewall_rule_version_list(arguments, sample_config)
    assert len(result) == 1
    assert expected in result[0].text


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "firewall_id is required"),
        ({"firewall_id": False}, "firewall_id must be a positive integer"),
        ({"firewall_id": "abc"}, "firewall_id must be a positive integer"),
        ({"firewall_id": 0}, "firewall_id must be a positive integer"),
        ({"firewall_id": -1}, "firewall_id must be a positive integer"),
        ({"firewall_id": 1, "page": False}, "page must be an integer"),
        ({"firewall_id": 1, "page": "abc"}, "page must be an integer"),
        (
            {"firewall_id": 1, "page": 0},
            "page must be an integer greater than or equal to 1",
        ),
        (
            {"firewall_id": 1, "page_size": "abc"},
            "page_size must be an integer",
        ),
        (
            {"firewall_id": 1, "page_size": 0},
            "page_size must be an integer from 25 through 500",
        ),
    ],
)
async def test_handle_linode_firewall_devices_list_invalid_args(
    sample_config: Config,
    arguments: dict[str, Any],
    expected: str,
) -> None:
    """Test firewall devices list handler argument validation."""
    from linodemcp.gentools import handle_linode_firewall_device_list

    result = await handle_linode_firewall_device_list(arguments, sample_config)
    assert len(result) == 1
    assert expected in result[0].text


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
        mock_client.route_raw.assert_not_called()


async def test_account_tag_create_dry_run_returns_preview(
    sample_config: Config,
) -> None:
    """dry_run=true previews the tag create POST with no call."""
    result = await handle_linode_tag_create(
        {"label": "my-tag", "dry_run": True}, sample_config
    )
    body = json.loads(result[0].text)
    assert body["tool"] == "linode_tag_create"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/tags"
    assert body["current_state"] is None


def _profile_preview_body(result: list[TextContent]) -> dict[str, Any]:
    """Decode a profile dry-run preview body."""
    assert len(result) == 1
    body: dict[str, Any] = json.loads(result[0].text)
    return body


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


async def test_instance_volumes_list_tool_def() -> None:
    """Linode volumes list tool should require linode_id and expose pagination."""
    tool, capability = create_linode_instance_volume_list_tool()
    assert tool.name == "linode_instance_volume_list"
    assert capability is Capability.Read
    assert tool.input_schema == proto_schema("linode.mcp.v1.InstanceVolumeListInput")
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    props = tool.input_schema["properties"]
    assert "page" in props
    assert "page_size" in props


async def test_instance_volumes_list_success(sample_config: Config) -> None:
    """Linode volumes list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 123, "label": "data"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client
        result = list(
            await handle_linode_instance_volume_list(
                {"linode_id": 42, "page": 1, "page_size": 25}, sample_config
            )
        )
    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "filter" not in payload
    assert payload["volumes"][0]["id"] == 123
    assert payload["volumes"][0]["label"] == "data"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_instance_volume_list", 42, query="page=1&page_size=25"
    )


@pytest.mark.parametrize("linode_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_instance_volumes_list_rejects_invalid_instance_id(
    sample_config: Config, linode_id: object
) -> None:
    """Linode volumes list handler rejects malformed instance IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        result = list(
            await handle_linode_instance_volume_list(
                {"linode_id": linode_id}, sample_config
            )
        )
    assert len(result) == 1
    assert "linode_id" in result[0].text.lower()
    mc.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"linode_id": 42, "page": "x"}, "page"),
        ({"linode_id": 42, "page": True}, "page"),
        ({"linode_id": 42, "page": 0}, "page"),
        ({"linode_id": 42, "page_size": "x"}, "page_size"),
        ({"linode_id": 42, "page_size": True}, "page_size"),
        ({"linode_id": 42, "page_size": 24}, "page_size"),
        ({"linode_id": 42, "page_size": 501}, "page_size"),
    ],
)
async def test_instance_volumes_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Linode volumes list handler validates pagination before client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        result = list(
            await handle_linode_instance_volume_list(arguments, sample_config)
        )

    assert len(result) == 1
    assert message in result[0].text
    mc.assert_not_called()


async def test_instance_firewalls_list_tool_def() -> None:
    """Linode firewalls list tool should require linode_id and expose pagination."""
    tool, _ = create_linode_instance_firewall_list_tool()
    assert tool.name == "linode_instance_firewall_list"
    assert tool.input_schema == proto_schema("linode.mcp.v1.InstanceFirewallListInput")
    required: list[str] = tool.input_schema.get("required") or []
    assert "linode_id" in required
    props = tool.input_schema["properties"]
    assert "page" in props
    assert "page_size" in props


async def test_instance_firewalls_list_success(sample_config: Config) -> None:
    """Linode firewalls list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 123, "label": "web"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_firewall_list(
                {"linode_id": 42, "page": 1, "page_size": 25}, sample_config
            )
        )

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "filter" not in payload
    assert payload["firewalls"][0]["id"] == 123
    assert payload["firewalls"][0]["label"] == "web"
    mock_client.route_raw.assert_awaited_once_with(
        "linode_instance_firewall_list", 42, query="page=1&page_size=25"
    )


@pytest.mark.parametrize("linode_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_instance_firewalls_list_rejects_invalid_instance_id(
    sample_config: Config, linode_id: object
) -> None:
    """Linode firewalls list handler rejects malformed instance IDs."""
    result = list(
        await handle_linode_instance_firewall_list(
            {"linode_id": linode_id}, sample_config
        )
    )

    assert len(result) == 1
    assert "linode_id" in result[0].text.lower()


async def test_instance_interface_firewalls_list_tool_def() -> None:
    """Linode interface firewalls list tool requires both path params."""
    tool, capability = create_linode_instance_interface_firewall_list_tool()
    assert tool.name == "linode_instance_interface_firewall_list"
    assert capability is Capability.Read
    required: list[str] = tool.input_schema.get("required") or []
    assert required == ["linode_id", "interface_id"]


async def test_instance_interface_firewalls_list_success(sample_config: Config) -> None:
    """Linode interface firewalls list handler returns API result."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mc:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "data": [{"id": 123, "label": "web"}],
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mc.return_value = mock_client

        result = list(
            await handle_linode_instance_interface_firewall_list(
                {"linode_id": 42, "interface_id": 7}, sample_config
            )
        )

    assert len(result) == 1
    assert "web" in result[0].text
    mock_client.route_raw.assert_awaited_once_with(
        "linode_instance_interface_firewall_list", 42, 7, query=""
    )


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
            await handle_linode_instance_interface_firewall_list(
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
        await handle_linode_instance_firewall_list(
            {"linode_id": 42, "page_size": 24}, sample_config
        )
    )

    assert len(result) == 1
    assert "page_size" in result[0].text


async def test_handle_linode_domain_update_rejects_non_integer_soa_timer(
    sample_config: Config,
) -> None:
    """Domain update rejects a non-integer SOA timer with Go's message."""
    from linodemcp.gentools import handle_linode_domain_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_domain_update(
            {"domain_id": 5, "confirm": True, "expire_sec": "soon"},
            sample_config,
        )

    assert result[0].text == "Error: expire_sec must be an integer"
    mock_client_class.assert_not_called()
