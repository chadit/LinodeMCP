"""Unit tests for MCP server dispatch."""

from __future__ import annotations

import asyncio
import dataclasses
import json
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, Mock, patch

import httpx
import pytest
from mcp.types import ListToolsRequest, ListToolsResult

from linodemcp.config import BuiltinOverride, UserProfileConfig
from linodemcp.linode import (
    Client,
    DomainZoneFile,
    NetworkError,
    Profile,
    RetryableClient,
)
from linodemcp.profiles import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
    Capability,
)
from linodemcp.server import Server, get_tool_registry
from linodemcp.tools import handle_hello, handle_version

if TYPE_CHECKING:
    from linodemcp.config import Config


def _full_access_config(base: Config) -> Config:
    """Return a copy of ``base`` switched to the full-access built-in.

    The Server fixtures default to no ``active_profile`` set, which means
    the resolver picks the read-only ``default`` built-in. Tests that need
    every registered tool (or a specific Write tool) opt into full-access
    via this helper and the matching override.
    """
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={
            "full-access": BuiltinOverride(disabled=False),
        },
    )


async def test_server_construction_stores_config(sample_config: Config) -> None:
    """Server stores the config and creates an MCP server instance."""
    srv = Server(sample_config)
    assert srv.config is sample_config
    assert srv.mcp is not None


async def test_shutdown_returns_immediately_with_no_inflight(
    sample_config: Config,
) -> None:
    """shutdown() with empty inflight should return True quickly."""
    srv = Server(sample_config)
    # Generous timeout: must not block when nothing is inflight.
    assert await srv.shutdown(timeout=1.0) is True


async def test_shutdown_drains_inflight_dispatch(sample_config: Config) -> None:
    """shutdown() blocks until an in-flight dispatch completes."""
    srv = Server(sample_config)

    # Patch handle_hello to await an event we control. While it waits,
    # shutdown() must not return; once we set the event, the dispatch
    # finishes and shutdown should resolve True.
    release = asyncio.Event()

    async def slow_hello(_arguments: dict[str, Any]) -> list[Any]:
        await release.wait()
        return []

    with patch("linodemcp.server.handle_hello", side_effect=slow_hello):
        dispatch_task = asyncio.create_task(srv.dispatch("hello", {"name": "x"}))
        # Yield so the dispatch starts and increments the inflight counter.
        await asyncio.sleep(0)

        shutdown_task = asyncio.create_task(srv.shutdown(timeout=5.0))
        # Yield to let shutdown register its waiter; it should be parked.
        await asyncio.sleep(0)
        assert not shutdown_task.done(), (
            "shutdown must block while dispatch is in flight"
        )

        release.set()
        await dispatch_task
        assert await shutdown_task is True


async def test_shutdown_times_out_on_stuck_dispatch(sample_config: Config) -> None:
    """shutdown() returns False when the deadline elapses before drain."""
    srv = Server(sample_config)

    never_release = asyncio.Event()

    async def stuck_hello(_arguments: dict[str, Any]) -> list[Any]:
        await never_release.wait()
        return []

    with patch("linodemcp.server.handle_hello", side_effect=stuck_hello):
        dispatch_task = asyncio.create_task(srv.dispatch("hello", {"name": "x"}))
        await asyncio.sleep(0)

        # Tight deadline: drain cannot complete because dispatch is stuck.
        assert await srv.shutdown(timeout=0.05) is False

        # Cleanup: release the stuck dispatch so the test exits cleanly.
        never_release.set()
        await dispatch_task


async def test_server_none_config_raises() -> None:
    """Passing None as config raises ValueError."""
    with pytest.raises(ValueError, match="config cannot be None"):
        Server(cast("Config", None))


async def test_hello_handler_returns_greeting() -> None:
    """handle_hello returns a greeting with the provided name."""
    result = await handle_hello({"name": "Test"})
    assert len(result) == 1
    assert "Hello, Test!" in result[0].text
    assert "running" in result[0].text


async def test_hello_handler_default_name() -> None:
    """handle_hello uses 'World' when no name is given."""
    result = await handle_hello({})
    assert len(result) == 1
    assert "Hello, World!" in result[0].text


async def test_version_handler_returns_version_info() -> None:
    """handle_version returns JSON with version data."""
    result = await handle_version({})
    assert len(result) == 1
    assert "version" in result[0].text.lower()
    assert "0.1.0" in result[0].text


async def test_config_handler_profile_dispatch(
    sample_config: Config, sample_profile_data: dict[str, Any]
) -> None:
    """Config-based handler for linode_profile calls the client and returns data."""
    from linodemcp.tools import handle_linode_profile

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


async def test_all_listed_tools_have_handlers(
    sample_config: Config,
) -> None:
    """Every tool in the tools module should have both a creator and a handler."""
    srv = Server(sample_config)

    # Server constructed without errors means _register_tools ran cleanly.
    assert srv.mcp is not None

    # Dynamically discover all create_*_tool and handle_* functions
    # from the tools module to verify parity.
    from linodemcp import tools as tools_mod

    create_funcs = [
        getattr(tools_mod, name)
        for name in dir(tools_mod)
        if name.startswith("create_") and name.endswith("_tool")
    ]

    tool_names = [fn()[0].name for fn in create_funcs]

    # No duplicate tool names
    seen: set[str] = set()
    duplicates: set[str] = set()
    for name in tool_names:
        if name in seen:
            duplicates.add(name)
        seen.add(name)
    assert not duplicates, f"Duplicate tool names: {duplicates}"

    # Collect all handle_* functions from the tools module.
    handle_funcs = {name for name in dir(tools_mod) if name.startswith("handle_")}
    # "hello" and "version" handlers don't follow the linode_ pattern
    config_handles = {
        n for n in handle_funcs if n not in ("handle_hello", "handle_version")
    }

    # Direction 1: Every handler must have a matching tool.
    for handler_name in config_handles:
        tool_name = handler_name.replace("handle_", "", 1)
        assert tool_name in seen, (
            f"Handler {handler_name} has no matching tool '{tool_name}'"
        )

    # Direction 2: Every tool (except hello/version) must have a handler.
    non_utility_tools = {t for t in seen if t not in ("hello", "version")}
    for tool_name in non_utility_tools:
        handler_name = f"handle_{tool_name}"
        assert handler_name in handle_funcs, (
            f"Tool '{tool_name}' has no matching handler '{handler_name}'"
        )

    # The number of config-based tools must match the number of
    # config-based handlers exactly (no orphans on either side).
    assert len(non_utility_tools) == len(config_handles), (
        f"Tool/handler count mismatch: {len(non_utility_tools)} tools"
        f" vs {len(config_handles)} handlers"
    )


async def test_placement_group_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_get_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_get" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_get"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_get" in srv.registered_tool_names


async def test_placement_group_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_create_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_create" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_create"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_create" in srv.registered_tool_names


async def test_placement_group_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_delete_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_delete" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_delete"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_delete" in srv.registered_tool_names


async def test_placement_group_assign_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group assign tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_assign_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_assign" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_assign"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_assign" in srv.registered_tool_names


async def test_placement_group_unassign_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group unassign tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_unassign_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_unassign" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_unassign"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_unassign" in srv.registered_tool_names


async def test_placement_group_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Placement group update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_placement_group_update_tool" in tools_mod.__all__
    assert "handle_linode_placement_group_update" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_placement_group_update"].capability is not None

    srv = Server(_full_access_config(sample_config))
    assert "linode_placement_group_update" in srv.registered_tool_names


async def test_profile_preferences_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile preferences get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_preferences_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_preferences_get" in tools_mod.__all__

    registry_names = {entry.name for entry in get_tool_registry()}
    assert "linode_profile_preferences_get" in registry_names

    srv = Server(_full_access_config(sample_config))
    assert "linode_profile_preferences_get" in srv.registered_tool_names


async def test_profile_preferences_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile preferences update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_preferences_update_tool" in tools_mod.__all__
    assert "handle_linode_profile_preferences_update" in tools_mod.__all__

    registry_names = {entry.name for entry in get_tool_registry()}
    assert "linode_profile_preferences_update" in registry_names

    srv = Server(_full_access_config(sample_config))
    assert "linode_profile_preferences_update" in srv.registered_tool_names


async def test_object_storage_cancel_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage cancel tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_cancel_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_cancel" in tools_mod.__all__

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_object_storage_cancel"].capability is Capability.Write

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_cancel" in srv.registered_tool_names


async def test_object_storage_bucket_access_allow_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage bucket access allow tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_bucket_access_allow_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_bucket_access_allow" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_bucket_access_allow" in srv.registered_tool_names


async def test_object_storage_quotas_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage quotas list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_quotas_list_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_quotas_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_quotas_list" in srv.registered_tool_names


async def test_object_storage_endpoints_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage endpoints list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_endpoints_list_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_endpoints_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_endpoints_list" in srv.registered_tool_names


async def test_network_transfer_prices_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Network transfer prices tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_network_transfer_prices_tool" in tools_mod.__all__
    assert "handle_linode_network_transfer_prices" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_network_transfer_prices" in srv.registered_tool_names


async def test_network_transfer_prices_handler_returns_client_response(
    sample_config: Config,
) -> None:
    """Network transfer prices handler returns the client response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_network_transfer_prices.return_value = {
            "data": [{"id": "transfer"}]
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_network_transfer_prices", {})

    mock_client.get_network_transfer_prices.assert_awaited_once_with()
    assert json.loads(result[0].text) == {"data": [{"id": "transfer"}]}


async def test_network_transfer_prices_handler_returns_error_response_on_client_failure(
    sample_config: Config,
) -> None:
    """Network transfer prices returns a tool error response on client failure."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_network_transfer_prices.side_effect = NetworkError(
            "GetNetworkTransferPrices", RuntimeError("timeout")
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_network_transfer_prices", {})

    assert "failed to retrieve network transfer prices" in result[0].text.lower()
    assert "GetNetworkTransferPrices" in result[0].text


async def test_object_storage_buckets_region_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage region bucket list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_buckets_region_list_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_buckets_region_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_buckets_region_list" in srv.registered_tool_names


async def test_object_storage_quota_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage quota get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_quota_get_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_quota_get" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_quota_get" in srv.registered_tool_names


async def test_object_storage_quota_usage_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Object Storage quota usage tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_object_storage_quota_usage_tool" in tools_mod.__all__
    assert "handle_linode_object_storage_quota_usage" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_object_storage_quota_usage" in srv.registered_tool_names


async def test_domain_clone_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Domain clone tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_domain_clone_tool" in tools_mod.__all__
    assert "handle_linode_domain_clone" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_domain_clone" in srv.registered_tool_names


async def test_domain_record_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Domain record get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_domain_record_get_tool" in tools_mod.__all__
    assert "handle_linode_domain_record_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_domain_record_get" in srv.registered_tool_names


async def test_domain_zone_file_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Domain zone file get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_domain_zone_file_get_tool" in tools_mod.__all__
    assert "handle_linode_domain_zone_file_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_domain_zone_file_get" in srv.registered_tool_names


async def test_domain_zone_file_get_handler_returns_client_response(
    sample_config: Config,
) -> None:
    """Domain zone file handler returns the client response."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain_zone_file.return_value = DomainZoneFile(
            zone_file=["$ORIGIN example.com."]
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_domain_zone_file_get", {"domain_id": 1})

    mock_client.get_domain_zone_file.assert_awaited_once_with(1)
    assert json.loads(result[0].text) == {"zone_file": ["$ORIGIN example.com."]}


@pytest.mark.parametrize(
    "domain_id",
    [None, 0, -1, True, "1", "1/2", "1?x=y", ".."],
)
async def test_domain_zone_file_get_handler_rejects_invalid_domain_id(
    sample_config: Config, domain_id: Any
) -> None:
    """Domain zone file handler rejects invalid path parameters first."""
    arguments: dict[str, Any] = {} if domain_id is None else {"domain_id": domain_id}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_domain_zone_file_get", arguments)

    mock_client_class.assert_not_called()
    assert "domain_id must be a positive integer" in result[0].text


async def test_regions_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Region get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_regions_get_tool" in tools_mod.__all__
    assert "handle_linode_regions_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_regions_get" in srv.registered_tool_names


async def test_firewall_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_get" in srv.registered_tool_names


async def test_firewall_rules_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall rules get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_rules_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_rules_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_rules_get" in srv.registered_tool_names


async def test_firewall_rule_version_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall rule version get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_rule_version_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_rule_version_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_rule_version_get" in srv.registered_tool_names


async def test_firewall_device_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Verify the firewall device get tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_device_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_device_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_device_get" in srv.registered_tool_names


async def test_firewall_device_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Verify the firewall device create tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_device_create_tool" in tools_mod.__all__
    assert "handle_linode_firewall_device_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_firewall_device_create" in srv.registered_tool_names


async def test_firewall_device_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Verify the firewall device delete tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_device_delete_tool" in tools_mod.__all__
    assert "handle_linode_firewall_device_delete" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_firewall_device_delete" in srv.registered_tool_names


async def test_firewall_rule_versions_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall rule versions list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_rule_versions_list_tool" in tools_mod.__all__
    assert "handle_linode_firewall_rule_versions_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_rule_versions_list" in srv.registered_tool_names


async def test_firewall_devices_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Verify the firewall devices list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_devices_list_tool" in tools_mod.__all__
    assert "handle_linode_firewall_devices_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_devices_list" in srv.registered_tool_names


async def test_firewall_rules_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall rules update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_rules_update_tool" in tools_mod.__all__
    assert "handle_linode_firewall_rules_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_firewall_rules_update" in srv.registered_tool_names


async def test_account_settings_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account settings update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_settings_update_tool" in tools_mod.__all__
    assert "handle_linode_account_settings_update" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_settings_update_tool()
    assert tool.name == "linode_account_settings_update"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]
    assert "dry_run" in tool.inputSchema["properties"]

    cfg = dataclasses.replace(
        sample_config,
        active_profile="account-settings-write",
        profiles={
            "account-settings-write": UserProfileConfig(
                description="account settings write",
                allowed_tools=("linode_account_settings_update",),
            ),
        },
    )
    srv = Server(cfg)
    assert "linode_account_settings_update" in srv.registered_tool_names


async def test_account_settings_update_handler_updates_settings(
    sample_config: Config,
) -> None:
    """Account settings update handler returns updated settings."""
    from linodemcp.tools import handle_linode_account_settings_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account_settings.return_value = {
            "network_helper": False,
            "object_storage": "active",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_update(
            {
                "network_helper": False,
                "object_storage": "active",
                "confirm": True,
            },
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Account settings updated successfully"
    assert payload["settings"] == {
        "network_helper": False,
        "object_storage": "active",
    }
    mock_client.update_account_settings.assert_awaited_once_with(
        network_helper=False, object_storage="active"
    )


async def test_account_settings_update_dry_run_previews_without_update(
    sample_config: Config,
) -> None:
    """Dry run previews the settings update without calling the update route."""
    from linodemcp.tools import handle_linode_account_settings_update

    result = await handle_linode_account_settings_update(
        {"network_helper": False, "confirm": False, "dry_run": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_settings_update"
    assert payload["would_execute"]["method"] == "PUT"
    assert payload["would_execute"]["path"] == "/account/settings"
    assert payload["would_execute"]["body"] == {"network_helper": False}
    assert payload["side_effects"] == ["Linode account settings are updated."]


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_account_settings_update_requires_boolean_confirm(
    sample_config: Config, confirm: Any
) -> None:
    """Live settings updates require explicit boolean confirm before client call."""
    from linodemcp.tools import handle_linode_account_settings_update

    arguments = {"network_helper": False}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_update(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client.update_account_settings.assert_not_called()


async def test_account_settings_update_requires_a_field(sample_config: Config) -> None:
    """Settings update rejects empty route bodies before client calls."""
    from linodemcp.tools import handle_linode_account_settings_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_update(
            {"confirm": True}, sample_config
        )

    assert "At least one account settings field" in result[0].text
    mock_client.update_account_settings.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"network_helper": "false", "confirm": True},
            "network_helper must be a boolean",
        ),
        ({"managed": 1, "confirm": True}, "managed must be a boolean"),
        ({"object_storage": True, "confirm": True}, "object_storage must be a string"),
        (
            {"maintenance_policy": 1, "confirm": True},
            "maintenance_policy must be a string",
        ),
        (
            {"network_helper": False, "extra_setting": "x", "confirm": True},
            "Unsupported account settings field(s): extra_setting",
        ),
    ],
)
async def test_account_settings_update_rejects_invalid_fields(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """Settings update validates route body fields before client calls."""
    from linodemcp.tools import handle_linode_account_settings_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client

        result = await handle_linode_account_settings_update(arguments, sample_config)

    assert message in result[0].text
    mock_client.update_account_settings.assert_not_called()


async def test_firewall_settings_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall settings update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_settings_update_tool" in tools_mod.__all__
    assert "handle_linode_firewall_settings_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_firewall_settings_update" in srv.registered_tool_names


async def test_firewall_settings_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall settings get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_settings_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_settings_get" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_firewall_settings_get_tool()
    assert tool.name == "linode_firewall_settings_get"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    srv = Server(sample_config)
    assert "linode_firewall_settings_get" in srv.registered_tool_names


async def test_firewall_settings_get_handler_returns_settings(
    sample_config: Config,
) -> None:
    """Firewall settings get handler returns client results."""
    from linodemcp.tools import handle_linode_firewall_settings_get

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_firewall_settings.return_value = {
            "default_firewall_ids": {"linode": 100},
            "page": 2,
            "pages": 3,
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_settings_get(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["default_firewall_ids"]["linode"] == 100
    mock_client.get_firewall_settings.assert_awaited_once_with(page=2, page_size=25)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": "bad"}, "page must be a valid integer"),
        ({"page": True}, "page must be a valid integer"),
        ({"page": 0}, "page must be a positive integer"),
        ({"page": -1}, "page must be a positive integer"),
        ({"page_size": "bad"}, "page_size must be a valid integer"),
        ({"page_size": True}, "page_size must be a valid integer"),
        ({"page_size": 0}, "page_size must be a positive integer"),
        ({"page_size": -1}, "page_size must be a positive integer"),
    ],
)
async def test_firewall_settings_get_rejects_invalid_pagination(
    arguments: dict[str, Any],
    message: str,
    sample_config: Config,
) -> None:
    """Firewall settings get handler validates pagination before client calls."""
    from linodemcp.tools import handle_linode_firewall_settings_get

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_settings_get(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_firewall_templates_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall templates list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_templates_list_tool" in tools_mod.__all__
    assert "handle_linode_firewall_templates_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_firewall_templates_list_tool()
    assert tool.name == "linode_firewall_templates_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    srv = Server(sample_config)
    assert "linode_firewall_templates_list" in srv.registered_tool_names


async def test_firewall_templates_list_handler_returns_templates(
    sample_config: Config,
) -> None:
    """Firewall templates list handler returns client results."""
    from linodemcp.tools import handle_linode_firewall_templates_list

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewall_templates.return_value = {
            "data": [{"slug": "allow-http", "label": "Allow HTTP"}],
            "page": 2,
            "pages": 3,
            "results": 1,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_templates_list(
            {"page": 2, "page_size": 25}, sample_config
        )

    assert len(result) == 1
    payload = json.loads(result[0].text)
    assert payload["data"][0]["slug"] == "allow-http"
    mock_client.list_firewall_templates.assert_awaited_once_with(page=2, page_size=25)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": "bad"}, "page must be a valid integer"),
        ({"page": True}, "page must be a valid integer"),
        ({"page": 0}, "page must be a positive integer"),
        ({"page": -1}, "page must be a positive integer"),
        ({"page_size": "bad"}, "page_size must be a valid integer"),
        ({"page_size": True}, "page_size must be a valid integer"),
        ({"page_size": 0}, "page_size must be a positive integer"),
        ({"page_size": -1}, "page_size must be a positive integer"),
    ],
)
async def test_firewall_templates_list_rejects_invalid_pagination(
    arguments: dict[str, Any],
    message: str,
    sample_config: Config,
) -> None:
    """Firewall templates list handler validates pagination before client calls."""
    from linodemcp.tools import handle_linode_firewall_templates_list

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_firewall_templates_list(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_firewall_template_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall template get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_template_get_tool" in tools_mod.__all__
    assert "handle_linode_firewall_template_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_firewall_template_get" in srv.registered_tool_names


async def test_volume_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Volume get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_volume_get_tool" in tools_mod.__all__
    assert "handle_linode_volume_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_volume_get" in srv.registered_tool_names


async def test_volume_clone_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Volume clone tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_volume_clone_tool" in tools_mod.__all__
    assert "handle_linode_volume_clone" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_volume_clone" in srv.registered_tool_names


async def test_ipv6_range_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """IPv6 range create tool should be exported and registered.

    Uses full-access because Phase 4 filters Write tools out of the
    default profile; the test only cares that the registration path
    sees the tool, not the profile filter.
    """
    from linodemcp import tools as tools_mod

    assert "create_linode_ipv6_range_create_tool" in tools_mod.__all__
    assert "handle_linode_ipv6_range_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_ipv6_range_create" in srv.registered_tool_names


async def test_ipv6_ranges_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """IPv6 ranges list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_ipv6_ranges_list_tool" in tools_mod.__all__
    assert "handle_linode_ipv6_ranges_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_ipv6_ranges_list" in srv.registered_tool_names


async def test_ipv6_pools_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """IPv6 pools list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_ipv6_pools_list_tool" in tools_mod.__all__
    assert "handle_linode_ipv6_pools_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_ipv6_pools_list" in srv.registered_tool_names


async def test_ipv6_pools_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """IPv6 pools list is callable through server dispatch."""
    response_data = {
        "data": [{"range": "2001:0db8::", "region": "us-east", "prefix": 124}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ipv6_pools.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_ipv6_pools_list", {})

    assert json.loads(result[0].text) == {
        "count": 1,
        "ipv6_pools": response_data["data"],
    }
    mock_client.list_ipv6_pools.assert_awaited_once_with()


async def test_account_beta_enroll_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account beta enroll tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    srv = Server(_full_access_config(sample_config))

    assert "create_linode_account_beta_enroll_tool" in tools_mod.__all__
    assert "handle_linode_account_beta_enroll" in tools_mod.__all__
    assert "linode_account_beta_enroll" in srv.registered_tool_names


async def test_account_beta_enroll_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account beta enroll is callable through server dispatch."""
    response_data = {"id": "distributed-beta", "label": "Distributed Beta"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.enroll_account_beta.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_beta_enroll",
            {"id": "distributed-beta", "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.enroll_account_beta.assert_awaited_once_with("distributed-beta")


async def test_account_agreements_acknowledge_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account agreements acknowledge tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    srv = Server(_full_access_config(sample_config))

    assert "create_linode_account_agreements_acknowledge_tool" in tools_mod.__all__
    assert "handle_linode_account_agreements_acknowledge" in tools_mod.__all__
    assert "linode_account_agreements_acknowledge" in srv.registered_tool_names


async def test_account_agreements_acknowledge_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account agreements acknowledge is callable through server dispatch."""
    response_data = {"accepted": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.acknowledge_account_agreements.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_agreements_acknowledge",
            {"eu_model": True, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.acknowledge_account_agreements.assert_awaited_once_with(
        {"eu_model": True}
    )


async def test_account_payment_method_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payment-method delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    srv = Server(_full_access_config(sample_config))

    assert "create_linode_account_payment_method_delete_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_method_delete" in tools_mod.__all__
    assert "linode_account_payment_method_delete" in srv.registered_tool_names


async def test_account_payment_method_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payment-method delete is callable through server dispatch."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_account_payment_method.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_delete",
            {"payment_method_id": 123, "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Payment method deleted successfully"
    mock_client.delete_account_payment_method.assert_awaited_once_with(123)


async def test_account_agreements_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account agreements list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_agreements_list_tool" in tools_mod.__all__
    assert "handle_linode_account_agreements_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_agreements_list" in srv.registered_tool_names


async def test_account_agreements_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account agreements list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": "eu_model", "label": "EU Model Contract"}]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_agreements.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_agreements_list", {})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_agreements.assert_awaited_once_with()


async def test_account_logins_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account logins list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_logins_list_tool" in tools_mod.__all__
    assert "handle_linode_account_logins_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_logins_list" in srv.registered_tool_names


async def test_account_logins_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account logins list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": 123, "ip": "192.0.2.10"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_logins.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_logins_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_logins.assert_awaited_once_with(page=2, page_size=25)


async def test_account_logins_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account logins list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_logins_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_logins_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account logins list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_logins_list", {"page_size": 10})

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_logins_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account logins list rejects non-integer pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_logins_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_databases_engines_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Database engines list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_databases_engines_list_tool" in tools_mod.__all__
    assert "handle_linode_databases_engines_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_databases_engines_list_tool()
    assert tool.name == "linode_databases_engines_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_databases_engines_list"].capability is Capability.Read

    srv = Server(_full_access_config(sample_config))
    assert "linode_databases_engines_list" in srv.registered_tool_names

    list_tools = srv.mcp.request_handlers[ListToolsRequest]
    result = await list_tools(ListToolsRequest(method="tools/list"))
    list_result = cast("ListToolsResult", result.root)
    assert "linode_databases_engines_list" in {tool.name for tool in list_result.tools}


async def test_databases_engines_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Database engines list dispatch should call the retryable client."""
    response_data: dict[str, object] = {
        "data": [{"id": "mysql", "version": "8.0.26"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_database_engines.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_databases_engines_list", {"page": 2, "page_size": 25}
        )

    payload = json.loads(result[0].text)
    assert payload["engines"] == response_data["data"]
    assert payload["count"] == 1
    assert payload["page"] == 1
    mock_client.list_database_engines.assert_awaited_once_with(page=2, page_size=25)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page_size": 10}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page": "2"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_databases_engines_list_rejects_invalid_pagination(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Database engines list validates pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_databases_engines_list", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_logins_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account logins list enforces page_size upper bound before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_logins_list", {"page_size": 501})

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_logins_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account logins list rejects bool pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_logins_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_users_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account users list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_users_list_tool" in tools_mod.__all__
    assert "handle_linode_account_users_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_users_list" in srv.registered_tool_names


async def test_account_users_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account users list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"username": "alice", "email": "alice@example.com"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_users.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_users_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_users.assert_awaited_once_with(page=2, page_size=25)


async def test_account_users_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account users list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_users_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_users_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account users list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_users_list", {"page_size": 10})

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_users_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account users list rejects non-integer pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_users_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_users_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account users list enforces page_size upper bound before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_users_list", {"page_size": 501})

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_users_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account users list rejects bool pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_users_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_user_delete_tool_is_exported_registered_and_schema(
    sample_config: Config,
) -> None:
    """Account user delete tool should be exported, registered, and gated."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_delete_tool" in tools_mod.__all__
    assert "handle_linode_account_user_delete" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_user_delete_tool()
    assert tool.name == "linode_account_user_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["username", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_user_delete" in srv.registered_tool_names


async def test_account_user_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account user delete is callable through server dispatch."""
    response_data: dict[str, object] = {"message": "deleted"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_account_user.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_delete",
            {"username": "alice", "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Account user deleted successfully",
        "result": response_data,
    }
    mock_client.delete_account_user.assert_awaited_once_with("alice")


async def test_account_user_delete_requires_boolean_confirm(
    sample_config: Config,
) -> None:
    """Account user delete rejects non-true confirm values before client calls."""
    for confirm in (None, False, "true", 1):
        arguments: dict[str, object] = {"username": "alice"}
        if confirm is not None:
            arguments["confirm"] = confirm

        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch("linode_account_user_delete", arguments)

        assert "Set confirm=true" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_user_delete_validates_username(
    sample_config: Config,
) -> None:
    """Account user delete rejects malformed usernames before client calls."""
    for username in (
        None,
        "",
        " alice",
        "alice bob",
        "alice/ops",
        "alice?debug",
        "alice#frag",
        "alice%2Fops",
        "alice@example",
        "alice:ops",
        r"alice\ops",
        "..",
    ):
        arguments: dict[str, object] = {"confirm": True}
        if username is not None:
            arguments["username"] = username

        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch("linode_account_user_delete", arguments)

        assert "username" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_user_delete_dry_run_encodes_username(
    sample_config: Config,
) -> None:
    """Account user delete dry-run previews the encoded DELETE path."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_delete",
            {"username": "alice_ops", "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_user_delete"
    assert payload["would_execute"]["method"] == "DELETE"
    assert payload["would_execute"]["path"] == "/account/users/alice_ops"
    mock_client_class.assert_not_called()


async def test_account_user_delete_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Account user delete returns shared error output for client failures."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_account_user.side_effect = RuntimeError("boom")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_delete",
            {"username": "alice", "confirm": True},
        )

    assert "boom" in result[0].text
    mock_client.delete_account_user.assert_awaited_once_with("alice")


async def test_account_settings_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account settings get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_settings_get_tool" in tools_mod.__all__
    assert "handle_linode_account_settings_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_settings_get" in srv.registered_tool_names


async def test_account_settings_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account settings get is callable through server dispatch."""
    response_data: dict[str, object] = {
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

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_settings_get", {})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_settings.assert_awaited_once_with()


async def test_account_settings_managed_enable_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account managed enable tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_settings_managed_enable_tool" in tools_mod.__all__
    assert "handle_linode_account_settings_managed_enable" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_settings_managed_enable_tool()
    assert capability is Capability.Write
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_settings_managed_enable" in srv.registered_tool_names


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_account_settings_managed_enable_rejects_non_true_confirm(
    sample_config: Config,
    confirm: object,
) -> None:
    """Account managed enable requires literal confirm=true before client calls."""
    arguments: dict[str, object] = {}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_settings_managed_enable", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_settings_managed_enable_dry_run_skips_client(
    sample_config: Config,
) -> None:
    """Account managed enable dry run previews without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_settings_managed_enable",
            {"dry_run": True, "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == "/account/settings/managed-enable"
    mock_client_class.assert_not_called()


async def test_account_settings_managed_enable_dry_run_requires_confirm(
    sample_config: Config,
) -> None:
    """Account managed enable dry run still requires the confirm safety gate."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_settings_managed_enable", {"dry_run": True}
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_settings_managed_enable_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account managed enable is callable through server dispatch."""
    response_data: dict[str, object] = {"managed": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.enable_account_managed.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_settings_managed_enable", {"confirm": True}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.enable_account_managed.assert_awaited_once_with()


async def test_account_transfer_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account transfer get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_transfer_get_tool" in tools_mod.__all__
    assert "handle_linode_account_transfer_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_transfer_get" in srv.registered_tool_names


async def test_account_transfer_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account transfer get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "billable": 12.5,
        "quota": 5000,
        "used": 42.0,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_transfer.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_transfer_get", {})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_transfer.assert_awaited_once_with()


async def test_account_maintenance_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account maintenance list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_maintenance_list_tool" in tools_mod.__all__
    assert "handle_linode_account_maintenance_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_maintenance_list" in srv.registered_tool_names


async def test_account_maintenance_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account maintenance list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"entity": {"id": 123, "type": "linode"}, "status": "pending"}],
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_maintenance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_maintenance_list", {})

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_maintenance.assert_awaited_once_with()


async def test_account_oauth_clients_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account OAuth clients list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_clients_list_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_clients_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_oauth_clients_list" in srv.registered_tool_names


async def test_account_oauth_clients_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account OAuth clients list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": "client-1", "label": "Example client"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_oauth_clients.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_clients_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_oauth_clients.assert_awaited_once_with(
        page=2, page_size=25
    )


async def test_account_oauth_clients_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account OAuth clients list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_oauth_clients_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_clients_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account OAuth clients list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_clients_list", {"page_size": 10}
        )

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_clients_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account OAuth clients list rejects non-integer pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_oauth_clients_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_clients_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account OAuth clients list enforces page_size upper bound."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_clients_list", {"page_size": 501}
        )

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_clients_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account OAuth clients list rejects bool pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_oauth_clients_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_client_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account OAuth client update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_update_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_oauth_client_update" in srv.registered_tool_names


async def test_account_oauth_client_update_schema_requires_confirm(
    sample_config: Config,
) -> None:
    """Account OAuth client update schema requires boolean confirm and dry_run."""
    registry = {entry.name: entry for entry in get_tool_registry()}
    tool = registry["linode_account_oauth_client_update"].tool

    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "id" not in tool.inputSchema["properties"]


async def test_account_oauth_client_update_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account OAuth client update is callable through server dispatch."""
    response_data = {"id": "client-1", "label": "Updated"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account_oauth_client.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_update",
            {"client_id": "client-1", "label": "Updated", "confirm": True},
        )

    assert json.loads(result[0].text) == {
        "message": "OAuth client updated successfully",
        "client": response_data,
    }
    mock_client.update_account_oauth_client.assert_awaited_once_with(
        "client-1", label="Updated"
    )


async def test_account_oauth_client_update_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Account OAuth client update dry-run previews without client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_update",
            {
                "client_id": "client-1",
                "label": "Updated",
                "confirm": False,
                "dry_run": True,
            },
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == "/account/oauth-clients/client-1"
    assert body["would_execute"]["body"] == {"label": "Updated"}
    mock_client_class.assert_not_called()


async def test_account_oauth_client_update_requires_confirm_true(
    sample_config: Config,
) -> None:
    """Account OAuth client update rejects non-true confirm values."""
    for confirm in (None, False, "true", 1):
        args: dict[str, object] = {"client_id": "client-1", "label": "Updated"}
        if confirm is not None:
            args["confirm"] = confirm
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch("linode_account_oauth_client_update", args)

        assert "Set confirm=true" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_oauth_client_update_rejects_invalid_client_id(
    sample_config: Config,
) -> None:
    """Account OAuth client update validates client_id before client calls."""
    for client_id in ("client/1", "client?1", "..", " client-1", ""):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch(
                "linode_account_oauth_client_update",
                {"client_id": client_id, "label": "Updated", "confirm": True},
            )

        assert "client_id must be" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_oauth_client_update_requires_update_field(
    sample_config: Config,
) -> None:
    """Account OAuth client update requires a documented body field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_update",
            {"client_id": "client-1", "confirm": True},
        )

    assert "At least one OAuth client field is required" in result[0].text
    mock_client_class.assert_not_called()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_update",
            {"client_id": "client-1", "id": "different-client", "confirm": True},
        )

    assert "At least one OAuth client field is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_client_thumbnail_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account OAuth client thumbnail update tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_account_oauth_client_thumbnail_update_tool" in tools_mod.__all__
    )
    assert "handle_linode_account_oauth_client_thumbnail_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_oauth_client_thumbnail_update" in srv.registered_tool_names


async def test_account_oauth_client_thumbnail_update_schema_requires_confirm() -> None:
    """Thumbnail update schema requires boolean confirm and dry_run."""
    registry = {entry.name: entry for entry in get_tool_registry()}
    tool = registry["linode_account_oauth_client_thumbnail_update"].tool

    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert (
        tool.inputSchema["properties"]["client_id"]["pattern"]
        == r"^[A-Za-z0-9][A-Za-z0-9_-]*$"
    )


async def test_account_oauth_client_thumbnail_update_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Thumbnail update is callable through server dispatch."""
    response_data = {"id": "client-1", "thumbnail_url": "https://example.com/t.png"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account_oauth_client_thumbnail.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_thumbnail_update",
            {"client_id": "client-1", "confirm": True},
        )

    assert json.loads(result[0].text) == {
        "message": "OAuth client thumbnail updated successfully",
        "client": response_data,
    }
    mock_client.update_account_oauth_client_thumbnail.assert_awaited_once_with(
        "client-1"
    )


async def test_account_oauth_client_thumbnail_update_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Thumbnail update dry-run previews encoded path without client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_thumbnail_update",
            {"client_id": "client-1", "confirm": False, "dry_run": True},
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["would_execute"] == {
        "method": "PUT",
        "path": "/account/oauth-clients/client-1/thumbnail",
        "body": {},
    }
    mock_client_class.assert_not_called()


async def test_account_oauth_client_thumbnail_update_requires_confirm_true(
    sample_config: Config,
) -> None:
    """Thumbnail update rejects non-true confirm values before client calls."""
    for confirm in (None, False, "true", 1):
        args: dict[str, object] = {"client_id": "client-1"}
        if confirm is not None:
            args["confirm"] = confirm
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch(
                "linode_account_oauth_client_thumbnail_update", args
            )

        assert "Set confirm=true" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_oauth_client_thumbnail_update_rejects_invalid_client_id(
    sample_config: Config,
) -> None:
    """Thumbnail update validates client_id before client calls."""
    for client_id in ("client/1", "client?1", "..", " client-1", ""):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            srv = Server(_full_access_config(sample_config))
            result = await srv.dispatch(
                "linode_account_oauth_client_thumbnail_update",
                {"client_id": client_id, "confirm": True},
            )

        assert "client_id must be" in result[0].text
        mock_client_class.assert_not_called()


async def test_account_events_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account events list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_events_list_tool" in tools_mod.__all__
    assert "handle_linode_account_events_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_events_list" in srv.registered_tool_names


async def test_account_events_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account events list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": 123, "action": "linode_create", "status": "finished"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_events.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_events_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_events.assert_awaited_once_with(page=2, page_size=25)


async def test_account_invoices_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account invoices list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_invoices_list_tool" in tools_mod.__all__
    assert "handle_linode_account_invoices_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_invoices_list" in srv.registered_tool_names


async def test_account_invoices_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account invoices list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": 123, "label": "Invoice #123"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_invoices.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_invoices_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_invoices.assert_awaited_once_with(page=2, page_size=25)


async def test_account_payments_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payments list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payments_list_tool" in tools_mod.__all__
    assert "handle_linode_account_payments_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_payments_list_tool()
    assert tool.name == "linode_account_payments_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "page", "page_size"}
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema

    srv = Server(sample_config)
    assert "linode_account_payments_list" in srv.registered_tool_names


async def test_account_payments_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payments list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": 123, "date": "2024-01-02T03:04:05"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_payments.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payments_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_payments.assert_awaited_once_with(page=2, page_size=25)


async def test_account_payments_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account payments list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payments_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payments_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account payments list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payments_list", {"page_size": 10})

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payments_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account payments list rejects non-integer pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payments_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payments_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account payments list enforces page_size upper bound."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payments_list", {"page_size": 501})

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payments_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account payments list rejects bool pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payments_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_methods_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payment methods list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_methods_list_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_methods_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_payment_methods_list_tool()
    assert tool.name == "linode_account_payment_methods_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "page", "page_size"}
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema

    srv = Server(sample_config)
    assert "linode_account_payment_methods_list" in srv.registered_tool_names


async def test_account_payment_methods_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payment methods list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"id": 123, "type": "credit_card", "is_default": True}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_payment_methods.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_methods_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_payment_methods.assert_awaited_once_with(
        page=2, page_size=25
    )


async def test_account_payment_methods_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account payment methods list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payment_methods_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_methods_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account payment methods list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_methods_list", {"page_size": 10}
        )

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_methods_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account payment methods list rejects non-integer pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_methods_list", {"page": "2"}
        )

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_methods_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account payment methods list enforces page_size upper bound."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_methods_list", {"page_size": 501}
        )

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_methods_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account payment methods list rejects bool pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_methods_list", {"page": True}
        )

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_notifications_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account notifications list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_notifications_list_tool" in tools_mod.__all__
    assert "handle_linode_account_notifications_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_notifications_list" in srv.registered_tool_names


async def test_account_notifications_list_schema_has_no_route_inputs(
    sample_config: Config,
) -> None:
    """Account notifications list schema exposes no route-specific inputs."""
    from linodemcp import tools as tools_mod

    tool, capability = tools_mod.create_linode_account_notifications_list_tool()

    assert tool.name == "linode_account_notifications_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "page", "page_size"}
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema

    srv = Server(sample_config)
    assert "linode_account_notifications_list" in srv.registered_tool_names


async def test_account_notifications_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account notifications list is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"type": "ticket_important", "message": "Ticket updated"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_notifications.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_notifications_list", {"page": 2, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_notifications.assert_awaited_once_with(
        page=2, page_size=25
    )


async def test_account_notifications_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account notifications list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_notifications_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_notifications_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account notifications list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_notifications_list", {"page_size": 10}
        )

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_notifications_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account notifications list rejects non-integer pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_notifications_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_notifications_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account notifications list enforces page_size upper bound."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_notifications_list", {"page_size": 501}
        )

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_notifications_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account notifications list rejects bool pagination."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_notifications_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoices_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account invoices list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoices_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoices_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account invoices list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoices_list", {"page_size": 10})

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoices_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account invoices list rejects non-integer pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoices_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoices_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account invoices list enforces page_size upper bound before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoices_list", {"page_size": 501})

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoices_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account invoices list rejects bool pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoices_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoice_items_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account invoice items list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_invoice_items_list_tool" in tools_mod.__all__
    assert "handle_linode_account_invoice_items_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_invoice_items_list" in srv.registered_tool_names


async def test_account_invoice_items_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account invoice items list is callable through server dispatch."""
    response_data: dict[str, object] = {
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

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_invoice_items_list",
            {"invoice_id": 123, "page": 2, "page_size": 25},
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
        ({"invoice_id": True}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123/456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": "123?456"}, "invoice_id must be a positive integer"),
        ({"invoice_id": ".."}, "invoice_id must be a positive integer"),
        ({"invoice_id": 123, "page": 0}, "page must be at least 1"),
        ({"invoice_id": 123, "page_size": 10}, "page_size must be at least 25"),
        ({"invoice_id": 123, "page": "2"}, "page must be an integer"),
    ],
)
async def test_account_invoice_items_list_rejects_invalid_arguments(
    arguments: dict[str, object], expected_error: str, sample_config: Config
) -> None:
    """Account invoice items list validates route inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoice_items_list", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_account_event_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account event get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_event_get_tool" in tools_mod.__all__
    assert "handle_linode_account_event_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_event_get" in srv.registered_tool_names


async def test_account_event_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account event get is callable through server dispatch."""
    response_data: dict[str, object] = {
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

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_event_get", {"event_id": 123})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_event.assert_awaited_once_with(123)


async def test_account_event_get_rejects_invalid_event_id(
    sample_config: Config,
) -> None:
    """Account event get validates event_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_event_get", {"event_id": "1/2"})

    assert "event_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_event_seen_tool_is_exported_registered_and_profiled(
    sample_config: Config,
) -> None:
    """Account event seen tool should be exported, registered, and profiled."""
    from linodemcp import tools as tools_mod
    from linodemcp.profiles.builtin import categories

    assert "create_linode_account_event_seen_tool" in tools_mod.__all__
    assert "handle_linode_account_event_seen" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_event_seen_tool()
    assert tool.name == "linode_account_event_seen"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["event_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert set(tool.inputSchema["required"]) == {"event_id", "confirm"}
    assert "account" in categories("linode_account_event_seen")

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_event_seen" in srv.registered_tool_names


async def test_account_event_seen_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account event seen is callable through server dispatch."""
    response_data: dict[str, object] = {}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.mark_account_event_seen.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_event_seen", {"event_id": 123, "confirm": True}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.mark_account_event_seen.assert_awaited_once_with(123)


async def test_account_event_seen_handler_calls_retryable_wrapper(
    sample_config: Config,
) -> None:
    """Account event seen handler calls the RetryableClient wrapper method."""
    response_data: dict[str, object] = {}

    with patch(
        "linodemcp.linode.RetryableClient.mark_account_event_seen",
        new_callable=AsyncMock,
    ) as mock_mark_seen:
        mock_mark_seen.return_value = response_data
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_event_seen", {"event_id": 123, "confirm": True}
        )

    assert json.loads(result[0].text) == response_data
    mock_mark_seen.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "event_id must be a positive integer"),
        ({"event_id": 0, "confirm": True}, "event_id must be a positive integer"),
        ({"event_id": True, "confirm": True}, "event_id must be a positive integer"),
        ({"event_id": "1/2", "confirm": True}, "event_id must be a positive integer"),
        ({"event_id": "1?2", "confirm": True}, "event_id must be a positive integer"),
        ({"event_id": "..", "confirm": True}, "event_id must be a positive integer"),
    ],
)
async def test_account_event_seen_rejects_invalid_event_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account event seen validates event_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_event_seen", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1, 0])
async def test_account_event_seen_requires_explicit_true_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Account event seen rejects missing/non-true confirm before client calls."""
    arguments: dict[str, object] = {"event_id": 123}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_event_seen", arguments)

    assert "Set confirm=true to proceed" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_event_seen_dry_run_fetches_event_without_marking_seen(
    sample_config: Config,
) -> None:
    """Account event seen dry-run fetches current state without mutating."""
    current_event = {"id": 123, "seen": False}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_event.return_value = current_event
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_event_seen",
            {"event_id": 123, "confirm": False, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_event_seen"
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/events/123/seen",
    }
    assert payload["current_state"] == current_event
    assert len(payload["side_effects"]) == 1
    assert "earlier events" in payload["side_effects"][0]
    mock_client.get_account_event.assert_awaited_once_with(123)
    mock_client.mark_account_event_seen.assert_not_called()


async def test_account_events_list_rejects_invalid_page(
    sample_config: Config,
) -> None:
    """Account events list validates page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_events_list", {"page": 0})

    assert "page must be at least 1" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_events_list_rejects_invalid_page_size(
    sample_config: Config,
) -> None:
    """Account events list validates page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_events_list", {"page_size": 10})

    assert "page_size must be at least 25" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_events_list_rejects_non_integer_pagination(
    sample_config: Config,
) -> None:
    """Account events list rejects non-integer pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_events_list", {"page": "2"})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_events_list_rejects_oversized_page_size(
    sample_config: Config,
) -> None:
    """Account events list enforces page_size upper bound before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_events_list", {"page_size": 501})

    assert "page_size must be at most 500" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_events_list_rejects_boolean_pagination(
    sample_config: Config,
) -> None:
    """Account events list rejects bool pagination before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_events_list", {"page": True})

    assert "page must be an integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_client_get_beta_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented beta program route."""
    response_data = {"id": "example-open", "label": "Example Open Beta"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_beta("example/open")

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/betas/example%2Fopen")
    await client.close()


async def test_retryable_client_get_beta_uses_retry_wrapper() -> None:
    """Read-only beta get delegates through the retry wrapper."""
    response_data = {"id": "example-open", "label": "Example Open Beta"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_beta = AsyncMock(return_value=response_data)

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_beta("example-open")

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_beta.assert_awaited_once_with("example-open")


async def test_client_get_database_mysql_config_uses_exact_path() -> None:
    """Low-level client uses the documented MySQL config route."""
    response_data = {
        "data": [
            {
                "name": "connect_timeout",
                "type": "integer",
                "minimum": 2,
                "maximum": 31536000,
            }
        ]
    }
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_mysql_config()

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/databases/mysql/config")
    await client.close()


async def test_client_get_database_mysql_config_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabaseMySQLConfig"),
    ):
        await client.get_database_mysql_config()

    await client.close()


async def test_client_get_database_postgresql_config_uses_exact_path() -> None:
    """Low-level client uses the documented PostgreSQL config route."""
    response_data = {
        "data": [
            {
                "name": "max_connections",
                "type": "integer",
                "minimum": 1,
                "maximum": 5000,
            }
        ]
    }
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_postgresql_config()

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/databases/postgresql/config")
    await client.close()


async def test_client_get_database_postgresql_config_maps_http_error() -> None:
    """Low-level client maps PostgreSQL config HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabasePostgreSQLConfig"),
    ):
        await client.get_database_postgresql_config()

    await client.close()


async def test_retryable_client_get_database_mysql_config_uses_retry_wrapper() -> None:
    """Read-only MySQL config get delegates through the retry wrapper."""
    response_data = {"data": [{"name": "connect_timeout"}]}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_mysql_config = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_mysql_config()

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_mysql_config.assert_awaited_once_with()


async def test_retryable_client_get_database_postgresql_config_retry_wrapper() -> None:
    """Read-only PostgreSQL config get delegates through the retry wrapper."""
    response_data = {"data": [{"name": "max_connections"}]}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_postgresql_config = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_postgresql_config()

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_postgresql_config.assert_awaited_once_with()


async def test_database_mysql_config_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL config get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_mysql_config_get_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_config_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_database_mysql_config_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_mysql_config_get"
    )
    assert entry.capability == Capability.Read
    assert entry.tool.inputSchema.get("required") is None
    assert "environment" in entry.tool.inputSchema["properties"]
    assert "linode_database_mysql_config_get" in get_version_info().features["tools"]


async def test_database_mysql_config_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL config get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"name": "connect_timeout", "type": "integer", "minimum": 2}]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_mysql_config.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_mysql_config_get", {})

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_mysql_config.assert_awaited_once_with()


async def test_database_postgresql_config_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL config get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_postgresql_config_get_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_config_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_database_postgresql_config_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_postgresql_config_get"
    )
    assert entry.capability == Capability.Read
    assert entry.tool.inputSchema.get("required") is None
    assert "environment" in entry.tool.inputSchema["properties"]
    assert (
        "linode_database_postgresql_config_get" in get_version_info().features["tools"]
    )


async def test_database_postgresql_config_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL config get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "data": [{"name": "max_connections", "type": "integer", "minimum": 1}]
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_postgresql_config.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_postgresql_config_get", {})

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_postgresql_config.assert_awaited_once_with()


async def test_client_pg_database_instance_get_uses_exact_path() -> None:
    """Low-level client uses the documented PostgreSQL database route."""
    response_data = {"id": 123, "label": "primary-db", "engine": "postgresql"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_postgresql_instance(123)

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/databases/postgresql/instances/123")
    await client.close()


async def test_client_get_database_postgresql_instance_maps_http_error() -> None:
    """Low-level client maps PostgreSQL instance HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabasePostgreSQLInstance"),
    ):
        await client.get_database_postgresql_instance(123)

    await client.close()


async def test_retryable_client_pg_database_instance_get_retries() -> None:
    """Read-only PostgreSQL database instance get delegates through retry."""
    response_data = {"id": 123, "label": "primary-db"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_postgresql_instance = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_postgresql_instance(123)

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_postgresql_instance.assert_awaited_once_with(123)


async def test_database_postgresql_instance_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL database instance get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_postgresql_instance_get_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_get" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_postgresql_instance_get_tool()
    assert tool.name == "linode_database_postgresql_instance_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["instance_id"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(sample_config)
    assert "linode_database_postgresql_instance_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_postgresql_instance_get"
    )
    assert entry.capability == Capability.Read
    assert (
        "linode_database_postgresql_instance_get"
        in get_version_info().features["tools"]
    )


async def test_database_postgresql_instance_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL database instance get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_postgresql_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instance_get", {"instance_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_postgresql_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_postgresql_instance_get_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL instance get rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instance_get", arguments
        )

    assert expected_error in result[0].text
    mock_client.get_database_postgresql_instance.assert_not_called()


async def test_client_patch_postgresql_database_instance_uses_exact_encoded_path() -> (
    None
):
    """Low-level client uses the documented PostgreSQL patch route."""
    response_data = {"id": 123, "label": "primary-db"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.patch_postgresql_database_instance("12/3")

    assert result == response_data
    make_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances/12%2F3/patch"
    )
    await client.close()


async def test_client_patch_postgresql_database_instance_maps_http_error() -> None:
    """Low-level client maps PostgreSQL patch HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="PatchPostgresqlDatabaseInstance"),
    ):
        await client.patch_postgresql_database_instance(123)

    await client.close()


async def test_retryable_client_patch_postgresql_database_instance_no_retry() -> None:
    """PostgreSQL database patch delegates once without retry replay."""
    response_data = {"id": 123, "label": "primary-db"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.patch_postgresql_database_instance = AsyncMock(
        return_value=response_data
    )

    with patch.object(retry_client, "_execute_with_retry", AsyncMock()) as retry:
        result = await retry_client.patch_postgresql_database_instance(123)

    assert result == response_data
    retry.assert_not_called()
    retry_client.client.patch_postgresql_database_instance.assert_awaited_once_with(123)


async def test_database_postgresql_instance_patch_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL database patch tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_postgresql_instance_patch_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_patch" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_postgresql_instance_patch_tool()
    assert tool.name == "linode_database_postgresql_instance_patch"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_patch" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_postgresql_instance_patch"
    )
    assert entry.capability == Capability.Write
    assert (
        "linode_database_postgresql_instance_patch"
        in get_version_info().features["tools"]
    )


async def test_database_postgresql_instance_patch_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL database patch is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.patch_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_patch",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.patch_postgresql_database_instance.assert_awaited_once_with(123)


async def test_database_postgresql_instance_patch_dry_run_previews_without_call(
    sample_config: Config,
) -> None:
    """PostgreSQL patch dry-run previews the encoded route without client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_patch",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == (
        "/databases/postgresql/instances/123/patch"
    )
    mock_client.patch_postgresql_database_instance.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_patch_rejects_non_true_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL patch requires literal confirm=true before client call."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_patch", arguments
        )

    assert "Set confirm=true to proceed." in result[0].text
    mock_client.patch_postgresql_database_instance.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "instance_id must be a positive integer"),
        ({"instance_id": 0, "confirm": True}, "instance_id must be a positive integer"),
        (
            {"instance_id": "123", "confirm": True},
            "instance_id must be a positive integer",
        ),
        (
            {"instance_id": True, "confirm": True},
            "instance_id must be a positive integer",
        ),
        (
            {"instance_id": "1/2", "confirm": True},
            "instance_id must be a positive integer",
        ),
        (
            {"instance_id": "1?x=y", "confirm": True},
            "instance_id must be a positive integer",
        ),
        (
            {"instance_id": "..", "confirm": True},
            "instance_id must be a positive integer",
        ),
    ],
)
async def test_database_postgresql_instance_patch_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL patch rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_patch", arguments
        )

    assert expected_error in result[0].text
    mock_client.patch_postgresql_database_instance.assert_not_called()


async def test_client_pg_database_instance_ssl_get_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented PostgreSQL database SSL route."""
    response_data = {"ssl_ca_certificate": "-----BEGIN CERTIFICATE-----"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_postgresql_instance_ssl("1/2")

    assert result == response_data
    make_request.assert_awaited_once_with(
        "GET", "/databases/postgresql/instances/1%2F2/ssl"
    )
    await client.close()


async def test_client_get_database_postgresql_instance_ssl_maps_http_error() -> None:
    """Low-level client maps PostgreSQL SSL route HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabasePostgreSQLInstanceSSL"),
    ):
        await client.get_database_postgresql_instance_ssl(123)

    await client.close()


async def test_retryable_client_get_database_postgresql_instance_ssl_retries() -> None:
    """Read-only PostgreSQL database SSL get delegates through retry."""
    response_data = {"ssl_ca_certificate": "certificate"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_postgresql_instance_ssl = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_postgresql_instance_ssl(123)

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_postgresql_instance_ssl.assert_awaited_once_with(
        123
    )


async def test_database_postgresql_instance_ssl_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL database SSL get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert (
        "create_linode_database_postgresql_instance_ssl_get_tool" in tools_mod.__all__
    )
    assert "handle_linode_database_postgresql_instance_ssl_get" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_ssl_get_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_ssl_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["instance_id"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(sample_config)
    assert "linode_database_postgresql_instance_ssl_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_postgresql_instance_ssl_get"
    )
    assert entry.capability == Capability.Read
    assert (
        "linode_database_postgresql_instance_ssl_get"
        in get_version_info().features["tools"]
    )


async def test_database_postgresql_instance_ssl_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL database SSL get is callable through server dispatch."""
    response_data: dict[str, object] = {"ssl_ca_certificate": "certificate"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_postgresql_instance_ssl.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instance_ssl_get", {"instance_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_postgresql_instance_ssl.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_postgresql_instance_ssl_get_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL SSL get rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instance_ssl_get", arguments
        )

    assert expected_error in result[0].text
    mock_client.get_database_postgresql_instance_ssl.assert_not_called()


async def test_client_get_database_mysql_instance_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented MySQL database instance route."""
    response_data = {"id": 123, "label": "primary-db", "engine": "mysql"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_mysql_instance(123)

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/databases/mysql/instances/123")
    await client.close()


async def test_client_get_database_mysql_instance_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabaseMySQLInstance"),
    ):
        await client.get_database_mysql_instance(123)

    await client.close()


async def test_retryable_client_get_database_mysql_instance_uses_retry_wrapper() -> (
    None
):
    """Read-only MySQL database instance get delegates through retry."""
    response_data = {"id": 123, "label": "primary-db"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_mysql_instance = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_mysql_instance(123)

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_mysql_instance.assert_awaited_once_with(123)


async def test_database_mysql_instance_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL database instance get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_mysql_instance_get_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_get" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_get_tool()
    assert tool.name == "linode_database_mysql_instance_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["instance_id"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(sample_config)
    assert "linode_database_mysql_instance_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_mysql_instance_get"
    )
    assert entry.capability == Capability.Read
    assert "linode_database_mysql_instance_get" in get_version_info().features["tools"]


async def test_database_mysql_instance_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL database instance get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_mysql_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_mysql_instance_get", {"instance_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_mysql_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_mysql_instance_get_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL instance get rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_mysql_instance_get", arguments)

    assert expected_error in result[0].text
    mock_client.get_database_mysql_instance.assert_not_called()


async def test_database_mysql_instance_credentials_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL database credentials get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert (
        "create_linode_database_mysql_instance_credentials_get_tool"
        in tools_mod.__all__
    )
    assert "handle_linode_database_mysql_instance_credentials_get" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_mysql_instance_credentials_get_tool()
    )
    assert tool.name == "linode_database_mysql_instance_credentials_get"
    assert capability is Capability.Write
    assert tool.description is not None
    assert "sensitive password material" in tool.description
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    default_srv = Server(sample_config)
    assert (
        "linode_database_mysql_instance_credentials_get"
        not in default_srv.registered_tool_names
    )
    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_credentials_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_mysql_instance_credentials_get"
    )
    assert entry.capability == Capability.Write
    assert (
        "linode_database_mysql_instance_credentials_get"
        in get_version_info().features["tools"]
    )


async def test_database_mysql_instance_credentials_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL database credentials get is callable through server dispatch."""
    response_data: dict[str, object] = {"username": "linode", "password": "secret"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_mysql_instance_credentials.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_credentials_get",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_mysql_instance_credentials.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_mysql_instance_credentials_get_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL credentials get rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_credentials_get", arguments
        )

    assert expected_error in result[0].text
    mock_client.get_database_mysql_instance_credentials.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_credentials_get_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL credentials get rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_credentials_get", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_credentials_get_dry_run_previews(
    sample_config: Config,
) -> None:
    """MySQL credentials get dry-run returns a GET preview without credentials."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_credentials_get",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "GET",
        "path": "/databases/mysql/instances/123/credentials",
    }
    assert "sensitive password material" in payload["warnings"][0]
    mock_client_class.assert_not_called()


async def test_client_get_database_postgresql_credentials_route_and_errors() -> None:
    """Low-level client uses the documented PostgreSQL credentials route."""
    response_data = {"username": "linode", "password": "secret"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_postgresql_instance_credentials(123)
    assert result == response_data
    make_request.assert_awaited_once_with(
        "GET", "/databases/postgresql/instances/123/credentials"
    )
    await client.close()

    error_client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            error_client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabasePostgreSQLInstanceCredentials"),
    ):
        await error_client.get_database_postgresql_instance_credentials(123)
    await error_client.close()


async def test_retryable_client_get_database_postgresql_credentials_retries() -> None:
    """Read-only PostgreSQL credentials get delegates through retry."""
    response_data = {"username": "linode", "password": "secret"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_postgresql_instance_credentials = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_postgresql_instance_credentials(123)
    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_postgresql_instance_credentials.assert_awaited_once_with(
        123
    )


async def test_database_postgresql_credentials_get_tool_registration(
    sample_config: Config,
) -> None:
    """PostgreSQL credentials get tool is exported and gated as write."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert (
        "create_linode_database_postgresql_instance_credentials_get_tool"
        in tools_mod.__all__
    )
    assert (
        "handle_linode_database_postgresql_instance_credentials_get"
        in tools_mod.__all__
    )
    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_credentials_get_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_credentials_get"
    assert capability is Capability.Write
    assert "sensitive password material" in (tool.description or "")
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.name not in Server(sample_config).registered_tool_names
    assert tool.name in Server(_full_access_config(sample_config)).registered_tool_names
    assert tool.name in get_version_info().features["tools"]
    entry = next(item for item in get_tool_registry() if item.name == tool.name)
    assert entry.capability == Capability.Write


async def test_database_postgresql_credentials_get_dispatches(
    sample_config: Config,
) -> None:
    """PostgreSQL credentials get returns the client response."""
    response_data: dict[str, object] = {"username": "linode", "password": "secret"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_postgresql_instance_credentials.return_value = (
            response_data
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_credentials_get",
            {"instance_id": 123, "confirm": True},
        )
    assert json.loads(result[0].text) == response_data
    mock_client.get_database_postgresql_instance_credentials.assert_awaited_once_with(
        123
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_postgresql_credentials_get_rejects_bad_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Malformed path params are rejected before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_credentials_get", arguments
        )
    assert expected_error in result[0].text
    mock_client.get_database_postgresql_instance_credentials.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_credentials_get_requires_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Non-true confirm values are rejected before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_credentials_get", arguments
        )
    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_postgresql_credentials_get_dry_run_previews(
    sample_config: Config,
) -> None:
    """Dry-run previews the GET route without constructing a client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_credentials_get",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )
    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "GET",
        "path": "/databases/postgresql/instances/123/credentials",
    }
    assert "sensitive password material" in payload["warnings"][0]
    mock_client_class.assert_not_called()


async def test_client_get_database_mysql_instance_ssl_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented MySQL database SSL route."""
    response_data = {"ssl_ca_certificate": "-----BEGIN CERTIFICATE-----"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_database_mysql_instance_ssl("1/2")

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/databases/mysql/instances/1%2F2/ssl")
    await client.close()


async def test_client_get_database_mysql_instance_ssl_maps_http_error() -> None:
    """Low-level client maps SSL route HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client,
            "make_request",
            AsyncMock(side_effect=httpx.ConnectError("boom")),
        ),
        pytest.raises(NetworkError, match="GetDatabaseMySQLInstanceSSL"),
    ):
        await client.get_database_mysql_instance_ssl(123)

    await client.close()


async def test_retryable_client_get_database_mysql_instance_ssl_retries() -> None:
    """Read-only MySQL database SSL get delegates through retry."""
    response_data = {"ssl_ca_certificate": "certificate"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_database_mysql_instance_ssl = AsyncMock(
        return_value=response_data
    )

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_database_mysql_instance_ssl(123)

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_database_mysql_instance_ssl.assert_awaited_once_with(123)


async def test_database_mysql_instance_ssl_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL database SSL get tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_mysql_instance_ssl_get_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_ssl_get" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_ssl_get_tool()
    assert tool.name == "linode_database_mysql_instance_ssl_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["instance_id"]
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(sample_config)
    assert "linode_database_mysql_instance_ssl_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_mysql_instance_ssl_get"
    )
    assert entry.capability == Capability.Read
    assert (
        "linode_database_mysql_instance_ssl_get" in get_version_info().features["tools"]
    )


async def test_database_mysql_instance_ssl_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL database SSL get is callable through server dispatch."""
    response_data: dict[str, object] = {"ssl_ca_certificate": "certificate"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_mysql_instance_ssl.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_mysql_instance_ssl_get", {"instance_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_mysql_instance_ssl.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "instance_id is required"),
        ({"instance_id": 0}, "instance_id must be at least 1"),
        ({"instance_id": "123"}, "instance_id must be an integer"),
        ({"instance_id": True}, "instance_id must be an integer"),
        ({"instance_id": "1/2"}, "instance_id must be an integer"),
        ({"instance_id": "1?x=y"}, "instance_id must be an integer"),
        ({"instance_id": ".."}, "instance_id must be an integer"),
    ],
)
async def test_database_mysql_instance_ssl_get_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL SSL get rejects malformed path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_mysql_instance_ssl_get", arguments)

    assert expected_error in result[0].text
    mock_client.get_database_mysql_instance_ssl.assert_not_called()


async def test_account_beta_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account beta get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_beta_get_tool" in tools_mod.__all__
    assert "handle_linode_account_beta_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_beta_get" in srv.registered_tool_names


async def test_account_beta_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account beta get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "id": "example-open",
        "label": "Example Open Beta",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_beta.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_beta_get", {"beta_id": "example-open"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_beta.assert_awaited_once_with("example-open")


async def test_account_beta_get_rejects_malformed_beta_id(
    sample_config: Config,
) -> None:
    """Account beta get rejects malformed beta_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_beta_get", {"beta_id": "example/open"}
        )

    assert "beta_id must not contain" in result[0].text
    mock_client_class.assert_not_called()


async def test_beta_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Beta get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_beta_get_tool" in tools_mod.__all__
    assert "handle_linode_beta_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_beta_get" in srv.registered_tool_names

    entry = next(item for item in get_tool_registry() if item.name == "linode_beta_get")
    assert entry.tool.inputSchema["required"] == ["beta_id"]
    assert entry.tool.inputSchema["properties"]["beta_id"]["type"] == "string"


async def test_beta_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Beta get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "id": "example-open",
        "label": "Example Open Beta",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_beta.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_beta_get", {"beta_id": "example-open"})

    assert json.loads(result[0].text) == response_data
    mock_client.get_beta.assert_awaited_once_with("example-open")


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "beta_id is required"),
        ({"beta_id": 123}, "beta_id must be a string"),
        ({"beta_id": ""}, "beta_id is required"),
        ({"beta_id": "example/open"}, "beta_id must contain only"),
        ({"beta_id": "example?open"}, "beta_id must contain only"),
        ({"beta_id": ".."}, "beta_id must contain only"),
    ],
)
async def test_beta_get_rejects_malformed_beta_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Beta get rejects malformed beta_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_beta_get", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_account_child_account_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Child account get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_child_account_get_tool" in tools_mod.__all__
    assert "handle_linode_account_child_account_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_child_account_get" in srv.registered_tool_names


async def test_account_child_account_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Child account get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56",
        "company": "Example Child",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_child_account.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_child_account_get",
            {"euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56"},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_child_account.assert_awaited_once_with(
        "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "euuid is required"),
        ({"euuid": 123}, "euuid must be a string"),
        ({"euuid": "   "}, "euuid is required"),
        ({"euuid": "child/account"}, "euuid must not contain"),
        ({"euuid": "child?account"}, "euuid must not contain"),
        ({"euuid": ".."}, "euuid must not contain"),
    ],
)
async def test_account_child_account_get_rejects_invalid_euuid(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Child account get rejects invalid euuid before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_child_account_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_child_account_get_schema_requires_euuid(
    sample_config: Config,
) -> None:
    """Child account get schema includes the required euuid path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_child_account_get"
    )

    assert entry.tool.inputSchema["required"] == ["euuid"]
    assert entry.tool.inputSchema["properties"]["euuid"]["type"] == "string"


async def test_account_service_transfer_accept_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Service transfer accept tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_service_transfer_accept_tool" in tools_mod.__all__
    assert "handle_linode_account_service_transfer_accept" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_service_transfer_accept_tool()
    assert tool.name == "linode_account_service_transfer_accept"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["token", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_service_transfer_accept" in srv.registered_tool_names


async def test_account_service_transfer_accept_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Service transfer accept is callable through server dispatch."""
    response_data: dict[str, object] = {
        "token": "transfer-token",
        "accepted": True,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.accept_account_service_transfer.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_accept",
            {"token": "transfer-token", "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.accept_account_service_transfer.assert_awaited_once_with(
        "transfer-token"
    )


async def test_account_service_transfer_accept_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Service transfer accept dry-run previews encoded route without client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_accept",
            {"token": "transfer#token", "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["would_execute"]["method"] == "POST"
    assert (
        payload["would_execute"]["path"]
        == "/account/service-transfers/transfer%23token/accept"
    )
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "confirm_value",
    [None, False, "true", 1],
)
async def test_account_service_transfer_accept_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Service transfer accept rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"token": "transfer-token"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_accept", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"confirm": True}, "token is required"),
        ({"token": 123, "confirm": True}, "token must be a string"),
        ({"token": "   ", "confirm": True}, "token is required"),
        ({"token": " transfer-token", "confirm": True}, "token must not contain"),
        ({"token": "transfer/token", "confirm": True}, "token must not contain"),
        ({"token": "transfer?token", "confirm": True}, "token must not contain"),
        ({"token": "..", "confirm": True}, "token must not contain"),
    ],
)
async def test_account_service_transfer_accept_rejects_invalid_token(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Service transfer accept rejects invalid token before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_accept", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_service_transfer_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Service transfer get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_service_transfer_get_tool" in tools_mod.__all__
    assert "handle_linode_account_service_transfer_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_service_transfer_get" in srv.registered_tool_names


async def test_account_service_transfer_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Service transfer get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "token": "transfer-token",
        "entities": {"linodes": [123]},
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_service_transfer.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_service_transfer_get", {"token": "transfer-token"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_service_transfer.assert_awaited_once_with("transfer-token")


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "token is required"),
        ({"token": 123}, "token must be a string"),
        ({"token": "   "}, "token is required"),
        ({"token": " transfer-token"}, "token must not contain"),
        ({"token": "transfer/token"}, "token must not contain"),
        ({"token": "transfer?token"}, "token must not contain"),
        ({"token": ".."}, "token must not contain"),
    ],
)
async def test_account_service_transfer_get_rejects_invalid_token(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Service transfer get rejects invalid token before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_service_transfer_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_service_transfer_get_schema_requires_token(
    sample_config: Config,
) -> None:
    """Service transfer get schema includes the required token path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_service_transfer_get"
    )

    assert entry.tool.inputSchema["required"] == ["token"]
    assert entry.tool.inputSchema["properties"]["token"]["type"] == "string"


async def test_account_service_transfer_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Service transfer delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_service_transfer_delete_tool" in tools_mod.__all__
    assert "handle_linode_account_service_transfer_delete" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_service_transfer_delete" in srv.registered_tool_names


async def test_account_service_transfer_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Service transfer delete is callable through server dispatch."""
    response_data: dict[str, object] = {"token": "transfer-token"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_account_service_transfer.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_delete",
            {"token": "transfer-token", "confirm": True},
        )

    assert json.loads(result[0].text) == {
        "message": "Service transfer canceled successfully",
        "result": response_data,
    }
    mock_client.delete_account_service_transfer.assert_awaited_once_with(
        "transfer-token"
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "token is required"),
        ({"token": 123, "confirm": True}, "token must be a string"),
        ({"token": "   ", "confirm": True}, "token is required"),
        ({"token": " transfer-token", "confirm": True}, "token must not contain"),
        ({"token": "transfer/token", "confirm": True}, "token must not contain"),
        ({"token": "transfer?token", "confirm": True}, "token must not contain"),
        ({"token": "..", "confirm": True}, "token must not contain"),
    ],
)
async def test_account_service_transfer_delete_rejects_invalid_token(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Service transfer delete rejects invalid token before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_delete", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_account_service_transfer_delete_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Service transfer delete requires explicit confirm=true before client calls."""
    arguments: dict[str, object] = {"token": "transfer-token"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_delete", arguments)

    assert "Set confirm=true to proceed" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_service_transfer_delete_dry_run_encodes_token(
    sample_config: Config,
) -> None:
    """Service transfer delete dry-run previews the encoded DELETE route."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_delete",
            {"token": "transfer#token", "confirm": False, "dry_run": True},
        )

    body = json.loads(result[0].text)
    assert body["would_execute"]["method"] == "DELETE"
    assert (
        body["would_execute"]["path"] == "/account/service-transfers/transfer%23token"
    )
    mock_client_class.assert_not_called()


async def test_account_service_transfer_delete_schema(
    sample_config: Config,
) -> None:
    """Service transfer delete schema requires token and confirm."""
    Server(_full_access_config(sample_config))
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_service_transfer_delete"
    )

    assert entry.tool.inputSchema["required"] == ["token", "confirm"]
    assert entry.tool.inputSchema["properties"]["token"]["type"] == "string"
    assert entry.tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert entry.tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


async def test_account_oauth_client_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """OAuth client get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_get_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_oauth_client_get" in srv.registered_tool_names


async def test_account_oauth_client_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """OAuth client get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "id": "client-123",
        "label": "Example OAuth Client",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_client_get", {"client_id": "client-123"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_oauth_client.assert_awaited_once_with("client-123")


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "client_id is required"),
        ({"client_id": 123}, "client_id must be a string"),
        ({"client_id": "   "}, "client_id is required"),
        ({"client_id": "client/id"}, "client_id must not contain"),
        ({"client_id": "client?id"}, "client_id must not contain"),
        ({"client_id": ".."}, "client_id must not contain"),
    ],
)
async def test_account_oauth_client_get_rejects_invalid_client_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """OAuth client get rejects invalid client_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_oauth_client_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_client_get_schema_requires_client_id(
    sample_config: Config,
) -> None:
    """OAuth client get schema includes the required client_id path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_oauth_client_get"
    )

    assert entry.tool.inputSchema["required"] == ["client_id"]
    assert entry.tool.inputSchema["properties"]["client_id"]["type"] == "string"


async def test_account_oauth_client_thumbnail_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """OAuth client thumbnail get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_thumbnail_get_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_thumbnail_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_oauth_client_thumbnail_get" in srv.registered_tool_names


async def test_account_oauth_client_thumbnail_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """OAuth client thumbnail get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "thumbnail_url": "https://example.test/thumb.png",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_oauth_client_thumbnail.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_client_thumbnail_get", {"client_id": "client-123"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_oauth_client_thumbnail.assert_awaited_once_with(
        "client-123"
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "client_id"),
        ({"client_id": 123}, "client_id"),
        ({"client_id": "   "}, "client_id"),
        ({"client_id": "client/id"}, "client_id"),
        ({"client_id": "client?id"}, "client_id"),
        ({"client_id": ".."}, "client_id"),
    ],
)
async def test_account_oauth_client_thumbnail_get_rejects_invalid_client_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """OAuth client thumbnail get rejects invalid client_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_oauth_client_thumbnail_get", arguments
        )

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_oauth_client_thumbnail_get_schema_requires_client_id(
    sample_config: Config,
) -> None:
    """OAuth client thumbnail get schema includes the required client_id path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_oauth_client_thumbnail_get"
    )

    assert entry.tool.inputSchema["required"] == ["client_id"]
    assert entry.tool.inputSchema["properties"]["client_id"]["type"] == "string"


async def test_account_invoice_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account invoice get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_invoice_get_tool" in tools_mod.__all__
    assert "handle_linode_account_invoice_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_invoice_get" in srv.registered_tool_names


async def test_account_invoice_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account invoice get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "label": "Invoice 123"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_invoice.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoice_get", {"invoice_id": 123})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_invoice.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "invoice_id is required"),
        ({"invoice_id": "123"}, "invoice_id must be an integer"),
        ({"invoice_id": True}, "invoice_id must be an integer"),
        ({"invoice_id": 0}, "invoice_id must be at least 1"),
        ({"invoice_id": "12/3"}, "invoice_id must be an integer"),
        ({"invoice_id": "12?3"}, "invoice_id must be an integer"),
        ({"invoice_id": ".."}, "invoice_id must be an integer"),
    ],
)
async def test_account_invoice_get_rejects_invalid_invoice_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account invoice get rejects invalid invoice_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_invoice_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_invoice_get_schema_requires_invoice_id(
    sample_config: Config,
) -> None:
    """Account invoice get schema includes the required invoice_id path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_invoice_get"
    )

    assert entry.tool.inputSchema["required"] == ["invoice_id"]
    assert entry.tool.inputSchema["properties"]["invoice_id"]["type"] == "integer"
    assert entry.tool.inputSchema["properties"]["invoice_id"]["minimum"] == 1


async def test_account_payment_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payment get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_get_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_payment_get" in srv.registered_tool_names


async def test_account_payment_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payment get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "usd": "10.00"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_payment.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payment_get", {"payment_id": 123})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_payment.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "payment_id is required"),
        ({"payment_id": "123"}, "payment_id must be an integer"),
        ({"payment_id": True}, "payment_id must be an integer"),
        ({"payment_id": 0}, "payment_id must be at least 1"),
        ({"payment_id": "12/3"}, "payment_id must be an integer"),
        ({"payment_id": "12?3"}, "payment_id must be an integer"),
        ({"payment_id": ".."}, "payment_id must be an integer"),
    ],
)
async def test_account_payment_get_rejects_invalid_payment_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account payment get rejects invalid IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payment_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_get_schema_requires_payment_id(
    sample_config: Config,
) -> None:
    """Account payment get schema includes the required path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_payment_get"
    )

    assert entry.tool.inputSchema["required"] == ["payment_id"]
    prop = entry.tool.inputSchema["properties"]["payment_id"]
    assert prop["type"] == "integer"
    assert prop["minimum"] == 1


async def test_account_payment_method_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payment method get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_method_get_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_method_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_payment_method_get" in srv.registered_tool_names


async def test_account_payment_method_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payment method get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 123, "type": "credit_card"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_payment_method.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_payment_method_get", {"payment_method_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_payment_method.assert_awaited_once_with(123)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "payment_method_id is required"),
        ({"payment_method_id": "123"}, "payment_method_id must be an integer"),
        ({"payment_method_id": True}, "payment_method_id must be an integer"),
        ({"payment_method_id": 0}, "payment_method_id must be at least 1"),
        ({"payment_method_id": "12/3"}, "payment_method_id must be an integer"),
        ({"payment_method_id": "12?3"}, "payment_method_id must be an integer"),
        ({"payment_method_id": ".."}, "payment_method_id must be an integer"),
    ],
)
async def test_account_payment_method_get_rejects_invalid_payment_method_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account payment method get rejects invalid IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_payment_method_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_method_get_schema_requires_payment_method_id(
    sample_config: Config,
) -> None:
    """Account payment method get schema includes the required path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_payment_method_get"
    )

    assert entry.tool.inputSchema["required"] == ["payment_method_id"]
    prop = entry.tool.inputSchema["properties"]["payment_method_id"]
    assert prop["type"] == "integer"
    assert prop["minimum"] == 1


async def test_account_payment_method_make_default_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account payment method make-default tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_method_make_default_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_method_make_default" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_payment_method_make_default" in srv.registered_tool_names


async def test_account_payment_method_make_default_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account payment method make-default is callable through dispatch."""
    response_data: dict[str, object] = {"id": 123, "is_default": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.make_account_payment_method_default.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_make_default",
            {"payment_method_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == {
        "message": "Default payment method updated successfully",
        "payment_method": response_data,
    }
    mock_client.make_account_payment_method_default.assert_awaited_once_with(123)


async def test_account_payment_method_make_default_dry_run_skips_client(
    sample_config: Config,
) -> None:
    """Account payment method make-default dry-run previews without a client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_make_default",
            {"payment_method_id": 123, "confirm": False, "dry_run": True},
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == "/account/payment-methods/123/make-default"
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "payment_method_id is required"),
        (
            {"payment_method_id": "123", "confirm": True},
            "payment_method_id must be an integer",
        ),
        (
            {"payment_method_id": True, "confirm": True},
            "payment_method_id must be an integer",
        ),
        (
            {"payment_method_id": 0, "confirm": True},
            "payment_method_id must be at least 1",
        ),
        (
            {"payment_method_id": "12/3", "confirm": True},
            "payment_method_id must be an integer",
        ),
        (
            {"payment_method_id": "12?3", "confirm": True},
            "payment_method_id must be an integer",
        ),
        (
            {"payment_method_id": "..", "confirm": True},
            "payment_method_id must be an integer",
        ),
    ],
)
async def test_account_payment_method_make_default_rejects_invalid_payment_method_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account payment method make-default rejects invalid IDs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_make_default", arguments
        )

    assert message in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_account_payment_method_make_default_requires_boolean_confirm_true(
    sample_config: Config, confirm: object
) -> None:
    """Account payment method make-default requires explicit confirm=true."""
    arguments: dict[str, object] = {"payment_method_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_make_default", arguments
        )

    assert "Set confirm=true to proceed" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_payment_method_make_default_schema_requires_confirm_and_dry_run(
    sample_config: Config,
) -> None:
    """Account payment method make-default schema covers confirm and dry-run."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_payment_method_make_default"
    )

    assert entry.capability is Capability.Write
    assert entry.tool.inputSchema["required"] == ["payment_method_id", "confirm"]
    properties = entry.tool.inputSchema["properties"]
    assert properties["payment_method_id"]["type"] == "integer"
    assert properties["payment_method_id"]["minimum"] == 1
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"


async def test_account_login_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account login get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_login_get_tool" in tools_mod.__all__
    assert "handle_linode_account_login_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_login_get" in srv.registered_tool_names


async def test_account_login_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account login get is callable through server dispatch."""
    response_data: dict[str, object] = {"id": 456, "username": "alice"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_login.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_login_get", {"login_id": 456})

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_login.assert_awaited_once_with(456)


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "login_id is required"),
        ({"login_id": "456"}, "login_id must be an integer"),
        ({"login_id": True}, "login_id must be an integer"),
        ({"login_id": 0}, "login_id must be at least 1"),
        ({"login_id": "45/6"}, "login_id must be an integer"),
        ({"login_id": "45?6"}, "login_id must be an integer"),
        ({"login_id": ".."}, "login_id must be an integer"),
    ],
)
async def test_account_login_get_rejects_invalid_login_id(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account login get rejects invalid login_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_login_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_login_get_schema_requires_login_id(
    sample_config: Config,
) -> None:
    """Account login get schema includes the required login_id path param."""
    Server(sample_config)
    entry = next(
        item for item in get_tool_registry() if item.name == "linode_account_login_get"
    )

    assert entry.tool.inputSchema["required"] == ["login_id"]
    assert entry.tool.inputSchema["properties"]["login_id"]["type"] == "integer"
    assert entry.tool.inputSchema["properties"]["login_id"]["minimum"] == 1


async def test_client_get_account_user_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented account user route."""
    response_data = {"username": "alice-dev"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_account_user("alice/dev")

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/account/users/alice%2Fdev")
    await client.close()


async def test_retryable_client_get_account_user_uses_retry_wrapper() -> None:
    """Read-only account user get delegates through the retry wrapper."""
    response_data = {"username": "alice-dev"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_account_user = AsyncMock(return_value=response_data)

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_account_user("alice-dev")

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_account_user.assert_awaited_once_with("alice-dev")


async def test_account_user_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account user get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_get_tool" in tools_mod.__all__
    assert "handle_linode_account_user_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_user_get" in srv.registered_tool_names


async def test_account_user_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account user get is callable through server dispatch."""
    response_data: dict[str, object] = {"username": "alice-dev", "restricted": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_user.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_user_get", {"username": "alice-dev"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_user.assert_awaited_once_with("alice-dev")


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "username is required"),
        ({"username": ""}, "username is required"),
        ({"username": " alice"}, "username must contain only"),
        ({"username": 123}, "username must be a string"),
        ({"username": True}, "username must be a string"),
        ({"username": "alice/dev"}, "username must contain only"),
        ({"username": "alice?dev"}, "username must contain only"),
        ({"username": ".."}, "username must contain only"),
    ],
)
async def test_account_user_get_rejects_invalid_username(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account user get rejects invalid username before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_user_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_user_get_schema_requires_username(
    sample_config: Config,
) -> None:
    """Account user get schema includes the required username path param."""
    Server(sample_config)
    entry = next(
        item for item in get_tool_registry() if item.name == "linode_account_user_get"
    )

    assert entry.tool.inputSchema["required"] == ["username"]
    username_schema = entry.tool.inputSchema["properties"]["username"]
    assert username_schema["type"] == "string"
    assert username_schema["pattern"] == "^[A-Za-z0-9][A-Za-z0-9_-]*$"


async def test_client_get_account_user_grants_uses_exact_encoded_path() -> None:
    """Low-level client uses the documented account user grants route."""
    response_data = {"global": {"account_access": "read_only"}}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.get_account_user_grants("alice/dev")

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/account/users/alice%2Fdev/grants")
    await client.close()


async def test_retryable_client_get_account_user_grants_uses_retry_wrapper() -> None:
    """Read-only account user grants delegates through the retry wrapper."""
    response_data = {"global": {"account_access": "read_only"}}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = Mock()
    retry_client.client.get_account_user_grants = AsyncMock(return_value=response_data)

    async def _execute(call: Any, *args: Any) -> Any:
        return await call(*args)

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_account_user_grants("alice-dev")

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_account_user_grants.assert_awaited_once_with("alice-dev")


async def test_account_user_grants_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account user grants get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_grants_get_tool" in tools_mod.__all__
    assert "handle_linode_account_user_grants_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_user_grants_get" in srv.registered_tool_names


async def test_account_user_grants_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account user grants get is callable through server dispatch."""
    response_data = {"global": {"account_access": "read_only"}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_user_grants.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_user_grants_get", {"username": "alice-dev"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_user_grants.assert_awaited_once_with("alice-dev")


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "username is required"),
        ({"username": ""}, "username is required"),
        ({"username": " alice"}, "username must contain only"),
        ({"username": 123}, "username must be a string"),
        ({"username": True}, "username must be a string"),
        ({"username": "alice/dev"}, "username must contain only"),
        ({"username": "alice?dev"}, "username must contain only"),
        ({"username": ".."}, "username must contain only"),
    ],
)
async def test_account_user_grants_get_rejects_invalid_username(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account user grants get rejects invalid username before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_user_grants_get", arguments)

    assert message in result[0].text
    mock_client_class.assert_not_called()


async def test_account_user_grants_get_schema_requires_username(
    sample_config: Config,
) -> None:
    """Account user grants schema includes the required username path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_user_grants_get"
    )

    assert entry.tool.inputSchema["required"] == ["username"]
    username_schema = entry.tool.inputSchema["properties"]["username"]
    assert username_schema["type"] == "string"
    assert username_schema["pattern"] == "^[A-Za-z0-9][A-Za-z0-9_-]*$"


async def test_client_update_account_user_grants_sends_exact_route_and_body() -> None:
    """Client user grants update sends the documented PUT body."""
    response_data = {
        "global": {"account_access": "read_only", "add_linodes": True},
        "linode": [{"id": 123, "permissions": "read_write"}],
        "volume": [{"id": 456, "permissions": None}],
    }
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.update_account_user_grants(
            "alice/dev",
            {
                "global": {"account_access": "read_only", "add_linodes": True},
                "linode": [{"id": 123, "permissions": "read_write"}],
                "volume": [{"id": 456, "permissions": None}],
            },
        )

    assert result == response_data
    make_request.assert_awaited_once_with(
        "PUT",
        "/account/users/alice%2Fdev/grants",
        {
            "global": {"account_access": "read_only", "add_linodes": True},
            "linode": [{"id": 123, "permissions": "read_write"}],
            "volume": [{"id": 456, "permissions": None}],
        },
    )
    await client.close()


async def test_retryable_client_update_account_user_grants_does_not_replay() -> None:
    """Mutating grants update delegates once without generic retry replay."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.update_account_user_grants = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError("UpdateAccountUserGrants", Exception("boom"))
    )
    try:
        with pytest.raises(NetworkError):
            await retry_client.update_account_user_grants(
                "alice-dev", {"global": {"account_access": "read_only"}}
            )
    finally:
        await retry_client.close()

    retry_client.client.update_account_user_grants.assert_awaited_once_with(
        "alice-dev", {"global": {"account_access": "read_only"}}
    )


async def test_account_user_grants_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account user grants update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_grants_update_tool" in tools_mod.__all__
    assert "handle_linode_account_user_grants_update" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_user_grants_update_tool()
    assert tool.name == "linode_account_user_grants_update"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["username"]["pattern"] == (
        "^[A-Za-z0-9][A-Za-z0-9_-]*$"
    )
    assert tool.inputSchema["properties"]["global"]["type"] == "object"
    assert tool.inputSchema["properties"]["linode"]["type"] == "array"
    assert tool.inputSchema["properties"]["linode"]["items"]["required"] == [
        "id",
        "permissions",
    ]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["username", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_user_grants_update" in srv.registered_tool_names


async def test_account_user_grants_update_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account user grants update dispatches through the registered handler."""
    response_data = {
        "global": {"account_access": "read_only", "add_linodes": True},
        "linode": [{"id": 123, "permissions": "read_write"}],
    }
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock(return_value=response_data)
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_grants_update",
            {
                "username": "alice-dev",
                "global": {"account_access": "read_only", "add_linodes": True},
                "linode": [{"id": 123, "permissions": "read_write"}],
                "confirm": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Account user grants updated successfully"
    assert payload["grants"] == response_data
    mock_client.update_account_user_grants.assert_awaited_once_with(
        "alice-dev",
        {
            "global": {"account_access": "read_only", "add_linodes": True},
            "linode": [{"id": 123, "permissions": "read_write"}],
        },
    )


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_user_grants_update_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Account user grants update rejects non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock()
    arguments: dict[str, object] = {
        "username": "alice-dev",
        "global": {"account_access": "read_only"},
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_grants_update", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.update_account_user_grants.assert_not_called()


@pytest.mark.parametrize(
    "username",
    [None, "", "   ", 123, True, "alice/dev", "alice?dev", ".."],
)
async def test_account_user_grants_update_rejects_invalid_username(
    sample_config: Config, username: object
) -> None:
    """Account user grants update validates username before client calls."""
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock()
    arguments: dict[str, object] = {
        "global": {"account_access": "read_only"},
        "confirm": True,
    }
    if username is not None:
        arguments["username"] = username

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_grants_update", arguments)

    assert "username" in result[0].text
    mock_client.update_account_user_grants.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"username": "alice-dev", "confirm": True}, "at least one grant field"),
        (
            {"username": "alice-dev", "global": "admin", "confirm": True},
            "global must be an object",
        ),
        (
            {
                "username": "alice-dev",
                "global": {"account_access": "admin"},
                "confirm": True,
            },
            "global.account_access must be 'read_only', 'read_write', or null",
        ),
        (
            {
                "username": "alice-dev",
                "global": {"add_linodes": "true"},
                "confirm": True,
            },
            "global.add_linodes must be a boolean",
        ),
        (
            {
                "username": "alice-dev",
                "global": {"unknown": True},
                "confirm": True,
            },
            "global has unknown fields: unknown",
        ),
        (
            {"username": "alice-dev", "linode": "read_only", "confirm": True},
            "linode must be an array of grant objects",
        ),
        (
            {"username": "alice-dev", "linode": ["read_only"], "confirm": True},
            "linode[0] must be an object",
        ),
        (
            {
                "username": "alice-dev",
                "linode": [{"permissions": "read_only"}],
                "confirm": True,
            },
            "linode[0].id must be an integer",
        ),
        (
            {
                "username": "alice-dev",
                "linode": [{"id": 123}],
                "confirm": True,
            },
            "linode[0].permissions is required",
        ),
        (
            {
                "username": "alice-dev",
                "linode": [{"id": True, "permissions": "read_only"}],
                "confirm": True,
            },
            "linode[0].id must be an integer",
        ),
        (
            {
                "username": "alice-dev",
                "linode": [{"id": 123, "permissions": "admin"}],
                "confirm": True,
            },
            "linode[0].permissions must be 'read_only', 'read_write', or null",
        ),
        (
            {
                "username": "alice-dev",
                "linode": [{"id": 123, "permissions": "read_only", "label": "web"}],
                "confirm": True,
            },
            "linode[0] has unknown fields: label",
        ),
        (
            {"username": "alice-dev", "unknown": "read_only", "confirm": True},
            "unknown grant update fields: unknown",
        ),
    ],
)
async def test_account_user_grants_update_rejects_invalid_body(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account user grants update validates the body before client calls."""
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_grants_update", arguments)

    assert expected_error in result[0].text
    mock_client.update_account_user_grants.assert_not_called()


async def test_account_user_grants_update_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Account user grants update dry-run previews the encoded request."""
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_grants_update",
            {
                "username": "alice-dev",
                "global": {"account_access": "read_only", "add_linodes": True},
                "volume": [{"id": 456, "permissions": None}],
                "confirm": True,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_user_grants_update"
    assert payload["would_execute"] == {
        "method": "PUT",
        "path": "/account/users/alice-dev/grants",
        "body": {
            "global": {"account_access": "read_only", "add_linodes": True},
            "volume": [{"id": 456, "permissions": None}],
        },
    }
    mock_client.update_account_user_grants.assert_not_called()


async def test_account_user_grants_update_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Account user grants update reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.update_account_user_grants = AsyncMock(
        side_effect=NetworkError("UpdateAccountUserGrants", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_grants_update",
            {
                "username": "alice-dev",
                "global": {"account_access": "read_only"},
                "confirm": True,
            },
        )

    assert "UpdateAccountUserGrants" in result[0].text
    mock_client.update_account_user_grants.assert_awaited_once_with(
        "alice-dev", {"global": {"account_access": "read_only"}}
    )


async def test_account_availability_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account availability get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_availability_get_tool" in tools_mod.__all__
    assert "handle_linode_account_availability_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_availability_get" in srv.registered_tool_names


async def test_account_availability_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account availability get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "available": ["Linodes", "NodeBalancers"],
        "region": "us-east",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account_availability.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_availability_get", {"region_id": "us-east"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_account_availability.assert_awaited_once_with("us-east")


async def test_account_availability_get_rejects_missing_region_id(
    sample_config: Config,
) -> None:
    """Account availability get requires region_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_availability_get", {})

    assert "region_id is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_availability_get_rejects_non_string_region_id(
    sample_config: Config,
) -> None:
    """Account availability get rejects non-string region IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_availability_get", {"region_id": 123}
        )

    assert "region_id must be a string" in result[0].text
    mock_client_class.assert_not_called()


async def test_account_availability_get_rejects_blank_region_id(
    sample_config: Config,
) -> None:
    """Account availability get rejects blank region IDs."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_availability_get", {"region_id": "   "}
        )

    assert "region_id is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "region_id", ["us/east", "us-east?x=1", "..", "å", "-", "US-EAST"]
)
async def test_account_availability_get_rejects_malformed_region_id(
    sample_config: Config, region_id: str
) -> None:
    """Account availability get rejects malformed region path parameters."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_availability_get", {"region_id": region_id}
        )

    assert (
        "region_id must be a lowercase region slug with letters, numbers, and hyphens"
        in result[0].text
    )
    mock_client_class.assert_not_called()


async def test_account_availability_get_schema_requires_region_id(
    sample_config: Config,
) -> None:
    """Account availability get schema includes the required region path param."""
    Server(sample_config)
    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_account_availability_get"
    )

    assert entry.tool.inputSchema["required"] == ["region_id"]
    assert entry.tool.inputSchema["properties"]["region_id"]["type"] == "string"


async def test_account_availability_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account availability list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_availability_list_tool" in tools_mod.__all__
    assert "handle_linode_account_availability_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_availability_list" in srv.registered_tool_names


async def test_account_availability_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account availability list is callable through server dispatch."""
    response_data = {
        "data": [{"service": "Linodes", "available": True}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_availability.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_availability_list", {})

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_availability.assert_awaited_once_with(
        page=None, page_size=None
    )


async def test_client_list_betas_uses_exact_route_and_query() -> None:
    """Low-level client uses the documented global Beta programs list route."""
    response_data = {
        "data": [{"id": "VPC", "label": "VPC Beta"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.list_betas(page=2, page_size=50)

    assert result == response_data
    make_request.assert_awaited_once_with("GET", "/betas?page=2&page_size=50")
    await client.close()


async def test_account_betas_list_tool_remains_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account beta enrollment list tool remains exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_betas_list_tool" in tools_mod.__all__
    assert "handle_linode_account_betas_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_betas_list_tool()
    assert tool.name == "linode_account_betas_list"
    assert capability is Capability.Read

    srv = Server(sample_config)
    assert "linode_account_betas_list" in srv.registered_tool_names


async def test_account_betas_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account beta enrollment list remains callable through server dispatch."""
    response_data: dict[str, Any] = {"data": [], "page": 1, "pages": 1, "results": 0}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_betas.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_betas_list", {"page": 1, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_betas.assert_awaited_once_with(page=1, page_size=25)


async def test_database_cluster_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL database create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_cluster_create_tool" in tools_mod.__all__
    assert "handle_linode_database_cluster_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_cluster_create_tool()
    assert tool.name == "linode_database_cluster_create"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {
        "label",
        "type",
        "engine",
        "region",
        "confirm",
    }
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_cluster_create" in srv.registered_tool_names


async def test_database_cluster_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL database create is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}
    arguments: dict[str, Any] = {
        "label": "primary-db",
        "type": "g6-dedicated-2",
        "engine": "mysql/8.0",
        "region": "us-east",
        "allow_list": ["192.0.2.1/32"],
        "cluster_size": 3,
        "engine_config": {"binlog_retention_period": 600},
        "fork": {"source": 456},
        "private_network": "vpc-1",
        "ssl_connection": True,
        "confirm": True,
    }
    expected_payload = {
        key: value for key, value in arguments.items() if key != "confirm"
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_cluster_create", arguments)

    assert json.loads(result[0].text) == response_data
    mock_client.create_mysql_database_instance.assert_awaited_once_with(
        expected_payload
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_cluster_create_rejects_non_true_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database create requires literal confirm=true before client calls."""
    arguments: dict[str, Any] = {
        "label": "primary-db",
        "type": "g6-dedicated-2",
        "engine": "mysql/8.0",
        "region": "us-east",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_cluster_create", arguments)

    assert "Set confirm=true to proceed" in result[0].text
    mock_client.create_mysql_database_instance.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        (
            {
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "confirm": True,
            },
            "label is required",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "cluster_size": True,
                "confirm": True,
            },
            "cluster_size must be an integer",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "ssl_connection": "true",
                "confirm": True,
            },
            "ssl_connection must be a boolean",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "unknown": "value",
                "confirm": True,
            },
            "unsupported argument: unknown",
        ),
        (
            {
                "label": "primary-db",
                "engine": "mysql/8.0",
                "region": "us-east",
                "confirm": True,
            },
            "type is required",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "region": "us-east",
                "confirm": True,
            },
            "engine is required",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "confirm": True,
            },
            "region is required",
        ),
        (
            {
                "label": " primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "confirm": True,
            },
            "label must not include leading or trailing whitespace",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "allow_list": ["192.0.2.1/32", ""],
                "confirm": True,
            },
            "allow_list must be an array of non-empty strings",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "cluster_size": 0,
                "confirm": True,
            },
            "cluster_size must be at least 1",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "engine_config": [],
                "confirm": True,
            },
            "engine_config must be an object",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "fork": "source-1",
                "confirm": True,
            },
            "fork must be an object",
        ),
        (
            {
                "label": "primary-db",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "private_network": "",
                "confirm": True,
            },
            "private_network must be a non-empty string",
        ),
    ],
)
async def test_database_cluster_create_rejects_invalid_arguments(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL database create rejects invalid inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_cluster_create", arguments)

    assert expected_error in result[0].text
    mock_client.create_mysql_database_instance.assert_not_called()


async def test_database_cluster_create_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """MySQL database create dry-run returns the POST preview and body."""
    arguments: dict[str, Any] = {
        "label": "primary-db",
        "type": "g6-dedicated-2",
        "engine": "mysql/8.0",
        "region": "us-east",
        "confirm": True,
        "dry_run": True,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_cluster_create", arguments)

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/mysql/instances",
        "body": {
            "label": "primary-db",
            "type": "g6-dedicated-2",
            "engine": "mysql/8.0",
            "region": "us-east",
        },
    }

    mock_client.create_mysql_database_instance.assert_not_called()


async def test_client_create_postgresql_db_sends_exact_route_and_body() -> None:
    """Client sends the documented PostgreSQL database create route and body."""
    payload = {
        "label": "primary-pg",
        "type": "g6-dedicated-2",
        "engine": "postgresql/17",
        "region": "us-east",
    }
    response_data = {"id": 321, "label": "primary-pg"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.create_postgresql_database_instance(payload)

    assert result == response_data
    make_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances", payload
    )
    await client.close()


async def test_client_create_postgresql_database_instance_maps_http_error() -> None:
    """Client maps PostgreSQL database create HTTP errors to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client, "make_request", AsyncMock(side_effect=httpx.ConnectError("boom"))
        ),
        pytest.raises(NetworkError, match="CreatePostgresqlDatabaseInstance"),
    ):
        await client.create_postgresql_database_instance(
            {
                "label": "primary-pg",
                "type": "g6-dedicated-2",
                "engine": "postgresql/17",
                "region": "us-east",
            }
        )
    await client.close()


async def test_retryable_create_postgresql_db_delegates_without_retry() -> None:
    """PostgreSQL database create delegates once to avoid replaying creates."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.create_postgresql_database_instance = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError("CreatePostgresqlDatabaseInstance", Exception("boom"))
    )

    with pytest.raises(NetworkError, match="CreatePostgresqlDatabaseInstance"):
        await retry_client.create_postgresql_database_instance({"label": "primary-pg"})

    retry_client.client.create_postgresql_database_instance.assert_awaited_once_with(
        {"label": "primary-pg"}
    )
    await retry_client.close()


async def test_database_postgresql_instance_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL database create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_postgresql_instance_create_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_create" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_create_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_create"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {
        "label",
        "type",
        "engine",
        "region",
        "confirm",
    }
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_create" in srv.registered_tool_names


async def test_database_postgresql_instance_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL database create is callable through server dispatch."""
    response_data = {"id": 321, "label": "primary-pg"}
    arguments: dict[str, Any] = {
        "label": "primary-pg",
        "type": "g6-dedicated-2",
        "engine": "postgresql/17",
        "region": "us-east",
        "allow_list": ["192.0.2.1/32"],
        "cluster_size": 3,
        "engine_config": {"shared_buffers": "256MB"},
        "fork": {"source": 456},
        "private_network": "vpc-1",
        "ssl_connection": True,
        "confirm": True,
    }
    expected_payload = {
        key: value for key, value in arguments.items() if key != "confirm"
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_create", arguments
        )

    assert json.loads(result[0].text) == response_data
    mock_client.create_postgresql_database_instance.assert_awaited_once_with(
        expected_payload
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_create_rejects_non_true_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL database create requires literal confirm=true before client calls."""
    arguments: dict[str, Any] = {
        "label": "primary-pg",
        "type": "g6-dedicated-2",
        "engine": "postgresql/17",
        "region": "us-east",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_create", arguments
        )

    assert "Set confirm=true to proceed" in result[0].text
    mock_client.create_postgresql_database_instance.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        (
            {
                "type": "g6-dedicated-2",
                "engine": "postgresql/17",
                "region": "us-east",
                "confirm": True,
            },
            "label is required",
        ),
        (
            {
                "label": "primary-pg",
                "type": "g6-dedicated-2",
                "engine": "postgresql/17",
                "region": "us-east",
                "cluster_size": True,
                "confirm": True,
            },
            "cluster_size must be an integer",
        ),
        (
            {
                "label": "primary-pg",
                "type": "g6-dedicated-2",
                "engine": "postgresql/17",
                "region": "us-east",
                "unknown": "value",
                "confirm": True,
            },
            "unsupported argument: unknown",
        ),
        (
            {
                "label": "primary-pg",
                "type": "g6-dedicated-2",
                "engine": "mysql/8.0",
                "region": "us-east",
                "confirm": True,
            },
            "engine must be a PostgreSQL engine ID",
        ),
        (
            {
                "label": "primary-pg",
                "type": "g6-dedicated-2",
                "engine": "postgresql/",
                "region": "us-east",
                "confirm": True,
            },
            "engine must be a PostgreSQL engine ID",
        ),
    ],
)
async def test_database_postgresql_instance_create_rejects_invalid_arguments(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL database create rejects invalid inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_create", arguments
        )

    assert expected_error in result[0].text
    mock_client.create_postgresql_database_instance.assert_not_called()


async def test_database_postgresql_instance_create_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """PostgreSQL database create dry-run returns the POST preview and body."""
    arguments: dict[str, Any] = {
        "label": "primary-pg",
        "type": "g6-dedicated-2",
        "engine": "postgresql/17",
        "region": "us-east",
        "confirm": True,
        "dry_run": True,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_create", arguments
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/postgresql/instances",
        "body": {
            "label": "primary-pg",
            "type": "g6-dedicated-2",
            "engine": "postgresql/17",
            "region": "us-east",
        },
    }
    mock_client.create_postgresql_database_instance.assert_not_called()


async def test_client_reset_postgresql_database_credentials_uses_exact_path() -> None:
    """Client sends the documented PostgreSQL credential reset route."""
    response_data = {"id": 123, "username": "linode", "password": "secret"}
    response = Mock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(
        client, "make_request", new=AsyncMock(return_value=response)
    ) as make_request:
        result = await client.reset_postgresql_database_credentials(123)

    assert result == response_data
    make_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances/123/credentials/reset"
    )
    await client.close()


async def test_client_reset_postgresql_database_credentials_encodes_path() -> None:
    """Client URL-encodes PostgreSQL credential reset path parameters."""
    response = Mock()
    response.json.return_value = {"id": "123/456"}
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(
        client, "make_request", new=AsyncMock(return_value=response)
    ) as make_request:
        await client.reset_postgresql_database_credentials("123/456")

    make_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances/123%2F456/credentials/reset"
    )
    await client.close()


async def test_client_reset_postgresql_database_credentials_maps_http_error() -> None:
    """Client maps PostgreSQL credential reset HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")

    with (
        patch.object(
            client,
            "make_request",
            new=AsyncMock(side_effect=httpx.HTTPError("boom")),
        ),
        pytest.raises(NetworkError, match="ResetPostgresqlDatabaseCredentials"),
    ):
        await client.reset_postgresql_database_credentials(123)

    await client.close()


async def test_retryable_reset_pg_database_credentials_no_retry() -> None:
    """PostgreSQL credential reset delegates once to avoid replaying resets."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.reset_postgresql_database_credentials = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError(
            "ResetPostgresqlDatabaseCredentials", Exception("boom")
        )
    )

    with pytest.raises(NetworkError, match="ResetPostgresqlDatabaseCredentials"):
        await retry_client.reset_postgresql_database_credentials(123)

    retry_client.client.reset_postgresql_database_credentials.assert_awaited_once_with(
        123
    )
    await retry_client.close()


async def test_database_postgresql_credentials_reset_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL credential reset tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_database_postgresql_credentials_reset_tool" in tools_mod.__all__
    )
    assert "handle_linode_database_postgresql_credentials_reset" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_credentials_reset_tool()
    )
    assert tool.name == "linode_database_postgresql_credentials_reset"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_credentials_reset" in srv.registered_tool_names


async def test_database_postgresql_credentials_reset_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL credential reset is callable through server dispatch."""
    response_data = {"id": 123, "username": "linode", "password": "secret"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.reset_postgresql_database_credentials.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_credentials_reset",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.reset_postgresql_database_credentials.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_credentials_reset_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL credential reset rejects non-true confirm before calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_credentials_reset", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"confirm": True},
        {"instance_id": 0, "confirm": True},
        {"instance_id": True, "confirm": True},
        {"instance_id": "123/456", "confirm": True},
        {"instance_id": "123?456", "confirm": True},
        {"instance_id": "..", "confirm": True},
    ],
)
async def test_database_postgresql_credentials_reset_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """PostgreSQL credential reset rejects invalid path params before calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_credentials_reset", arguments
        )

    assert "instance_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_postgresql_credentials_reset_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """PostgreSQL credential reset dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_credentials_reset",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/postgresql/instances/123/credentials/reset",
    }
    mock_client_class.assert_not_called()


async def test_database_mysql_credentials_reset_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL credential reset tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_credentials_reset_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_credentials_reset" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_credentials_reset_tool()
    assert tool.name == "linode_database_mysql_credentials_reset"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_credentials_reset" in srv.registered_tool_names


async def test_database_mysql_credentials_reset_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL credential reset is callable through server dispatch."""
    response_data = {"id": 123, "username": "linode", "password": "secret"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.reset_mysql_database_credentials.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_credentials_reset",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.reset_mysql_database_credentials.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_credentials_reset_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL credential reset rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_credentials_reset", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"confirm": True},
        {"instance_id": 0, "confirm": True},
        {"instance_id": True, "confirm": True},
        {"instance_id": "123/456", "confirm": True},
        {"instance_id": "123?456", "confirm": True},
        {"instance_id": "..", "confirm": True},
    ],
)
async def test_database_mysql_credentials_reset_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """MySQL credential reset rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_credentials_reset", arguments
        )

    assert "instance_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_credentials_reset_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """MySQL credential reset dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_credentials_reset",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/mysql/instances/123/credentials/reset",
    }
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL Managed Database delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_instance_delete_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_delete" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_delete_tool()
    assert tool.name == "linode_database_mysql_instance_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_delete" in srv.registered_tool_names


async def test_database_mysql_instance_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL Managed Database delete is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_delete",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.delete_mysql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_delete_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database delete rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_delete", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"confirm": True},
        {"instance_id": 0, "confirm": True},
        {"instance_id": True, "confirm": True},
        {"instance_id": "123/456", "confirm": True},
        {"instance_id": "123?456", "confirm": True},
        {"instance_id": "..", "confirm": True},
    ],
)
async def test_database_mysql_instance_delete_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """MySQL database delete rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_delete", arguments)

    assert "instance_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_delete_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """MySQL database delete dry-run returns the DELETE preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_delete",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "DELETE",
        "path": "/databases/mysql/instances/123",
    }
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_delete_client_sends_exact_route() -> None:
    """Client PostgreSQL database delete sends the documented DELETE route."""
    response_data: dict[str, Any] = {"id": 123, "label": "primary-pg"}
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock(  # type: ignore[method-assign]
        return_value=_JsonResponse(response_data)
    )

    try:
        result = await client.delete_postgresql_database_instance(123)
    finally:
        await client.close()

    assert result == response_data
    client.make_request.assert_awaited_once_with(
        "DELETE", "/databases/postgresql/instances/123"
    )


async def test_database_postgresql_instance_delete_client_encodes_path() -> None:
    """Client PostgreSQL database delete URL-encodes path parameters."""
    response_data: dict[str, Any] = {"id": "123/456"}
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock(  # type: ignore[method-assign]
        return_value=_JsonResponse(response_data)
    )

    try:
        result = await client.delete_postgresql_database_instance("123/456")
    finally:
        await client.close()

    assert result == response_data
    client.make_request.assert_awaited_once_with(
        "DELETE", "/databases/postgresql/instances/123%2F456"
    )


async def test_database_postgresql_instance_delete_retryable_delegates_once() -> None:
    """Retryable PostgreSQL delete does not replay destructive calls."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.delete_postgresql_database_instance = AsyncMock(  # type: ignore[method-assign]
        side_effect=httpx.HTTPError("temporary")
    )

    try:
        with pytest.raises(httpx.HTTPError):
            await retry_client.delete_postgresql_database_instance(123)
    finally:
        await retry_client.close()

    retry_client.client.delete_postgresql_database_instance.assert_awaited_once_with(
        123
    )


async def test_database_postgresql_instance_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_postgresql_instance_delete_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_delete" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_delete_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_delete" in srv.registered_tool_names


async def test_database_postgresql_instance_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database delete is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-pg"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_delete",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.delete_postgresql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_delete_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL database delete rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_delete", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"confirm": True},
        {"instance_id": 0, "confirm": True},
        {"instance_id": True, "confirm": True},
        {"instance_id": "123/456", "confirm": True},
        {"instance_id": "123?456", "confirm": True},
        {"instance_id": "..", "confirm": True},
    ],
)
async def test_database_postgresql_instance_delete_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """PostgreSQL database delete rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_delete", arguments
        )

    assert "instance_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_delete_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """PostgreSQL database delete dry-run returns the DELETE preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_delete",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "DELETE",
        "path": "/databases/postgresql/instances/123",
    }
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_resume_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL Managed Database resume tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_instance_resume_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_resume" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_resume_tool()
    assert tool.name == "linode_database_mysql_instance_resume"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_resume" in srv.registered_tool_names


async def test_database_mysql_instance_resume_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL Managed Database resume is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resume_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_resume",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.resume_mysql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_resume_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database resume rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_resume", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "instance_id is required"),
        ({"instance_id": 0, "confirm": True}, "instance_id must be at least 1"),
        ({"instance_id": True, "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123/456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123?456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "..", "confirm": True}, "instance_id must be an integer"),
    ],
)
async def test_database_mysql_instance_resume_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL database resume rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_resume", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_resume_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """MySQL database resume dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_resume",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/mysql/instances/123/resume",
    }
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_resume_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database resume tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_postgresql_instance_resume_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_resume" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_resume_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_resume"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_resume" in srv.registered_tool_names


async def test_database_postgresql_instance_resume_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database resume is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resume_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_resume",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.resume_postgresql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_resume_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL database resume rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_resume", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "instance_id is required"),
        ({"instance_id": 0, "confirm": True}, "instance_id must be at least 1"),
        ({"instance_id": True, "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123/456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123?456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "..", "confirm": True}, "instance_id must be an integer"),
    ],
)
async def test_database_postgresql_instance_resume_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL database resume rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_resume", arguments
        )

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_resume_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """PostgreSQL database resume dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_resume",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/postgresql/instances/123/resume",
    }
    mock_client_class.assert_not_called()


async def test_suspend_mysql_database_instance_sends_encoded_post() -> None:
    """Low-level client sends the documented MySQL suspend route."""
    response_data = {"id": 123, "label": "primary-db"}
    response = _JsonResponse(response_data)
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.suspend_mysql_database_instance("123/456")

    assert result == response_data
    make_request.assert_awaited_once_with(
        "POST", "/databases/mysql/instances/123%2F456/suspend"
    )
    await client.close()


async def test_suspend_mysql_database_instance_wraps_http_errors() -> None:
    """Low-level client maps suspend route HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client, "make_request", AsyncMock(side_effect=httpx.ConnectError("boom"))
        ),
        pytest.raises(NetworkError, match="SuspendMysqlDatabaseInstance"),
    ):
        await client.suspend_mysql_database_instance(123)
    await client.close()


async def test_retryable_suspend_mysql_database_instance_delegates_once() -> None:
    """Retryable suspend wrapper delegates once to avoid replaying side effects."""
    retryable = RetryableClient("https://api.linode.test/v4", "token")
    with patch.object(
        retryable.client, "suspend_mysql_database_instance", new_callable=AsyncMock
    ) as suspend:
        suspend.return_value = {"id": 123}

        result = await retryable.suspend_mysql_database_instance(123)

    assert result == {"id": 123}
    suspend.assert_awaited_once_with(123)


async def test_database_mysql_instance_suspend_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL Managed Database suspend tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_instance_suspend_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_suspend" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_suspend_tool()
    assert tool.name == "linode_database_mysql_instance_suspend"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_suspend" in srv.registered_tool_names


async def test_database_mysql_instance_suspend_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL Managed Database suspend is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.suspend_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_suspend",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.suspend_mysql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_suspend_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database suspend rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_suspend", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "instance_id is required"),
        ({"instance_id": 0, "confirm": True}, "instance_id must be at least 1"),
        ({"instance_id": True, "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123/456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123?456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "..", "confirm": True}, "instance_id must be an integer"),
    ],
)
async def test_database_mysql_instance_suspend_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL database suspend rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_suspend", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_suspend_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """MySQL database suspend dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_suspend",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/mysql/instances/123/suspend",
    }
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_suspend_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database suspend tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_database_postgresql_instance_suspend_tool" in tools_mod.__all__
    )
    assert "handle_linode_database_postgresql_instance_suspend" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_suspend_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_suspend"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_suspend" in srv.registered_tool_names


async def test_database_postgresql_instance_suspend_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database suspend is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.suspend_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_suspend",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.suspend_postgresql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_suspend_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL database suspend rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_suspend", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"confirm": True}, "instance_id is required"),
        ({"instance_id": 0, "confirm": True}, "instance_id must be at least 1"),
        ({"instance_id": True, "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123/456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "123?456", "confirm": True}, "instance_id must be an integer"),
        ({"instance_id": "..", "confirm": True}, "instance_id must be an integer"),
    ],
)
async def test_database_postgresql_instance_suspend_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL database suspend rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_suspend", arguments
        )

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_database_postgresql_instance_suspend_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """PostgreSQL database suspend dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_suspend",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/postgresql/instances/123/suspend",
    }
    mock_client_class.assert_not_called()


async def test_client_update_mysql_database_instance_uses_exact_path_and_body() -> None:
    """Client sends the documented MySQL database update route and body."""
    response_data = {"id": 123, "label": "primary-db"}
    response = _JsonResponse(response_data)
    payload = {"label": "primary-db", "updates": {"hour_of_day": 2}}
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.update_mysql_database_instance(123, payload)

    assert result == response_data
    make_request.assert_awaited_once_with(
        "PUT", "/databases/mysql/instances/123", payload
    )
    await client.close()


async def test_client_update_mysql_database_instance_encodes_path_param() -> None:
    """Client URL-encodes the documented path parameter boundary."""
    response = _JsonResponse({"id": 123})
    payload = {"label": "primary-db"}
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        await client.update_mysql_database_instance(cast("Any", "12/3?"), payload)

    make_request.assert_awaited_once_with(
        "PUT", "/databases/mysql/instances/12%2F3%3F", payload
    )
    await client.close()


async def test_client_update_mysql_database_instance_maps_http_error() -> None:
    """Client maps MySQL database update HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client, "make_request", AsyncMock(side_effect=httpx.ConnectError("boom"))
        ),
        pytest.raises(NetworkError, match="UpdateMysqlDatabaseInstance"),
    ):
        await client.update_mysql_database_instance(123, {"label": "primary-db"})
    await client.close()


async def test_retryable_client_update_mysql_db_delegates_without_retry() -> None:
    """Mutating MySQL database update is not replayed through retry wrapper."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.update_mysql_database_instance = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError("UpdateMysqlDatabaseInstance", Exception("boom"))
    )
    try:
        with pytest.raises(NetworkError):
            await retry_client.update_mysql_database_instance(
                123, {"label": "primary-db"}
            )
    finally:
        await retry_client.close()

    retry_client.client.update_mysql_database_instance.assert_awaited_once_with(
        123, {"label": "primary-db"}
    )


async def test_database_mysql_instance_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL database update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_instance_update_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_update" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_update_tool()
    assert tool.name == "linode_database_mysql_instance_update"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {"instance_id", "confirm"}
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_update" in srv.registered_tool_names


async def test_database_mysql_instance_update_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL database update is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}
    arguments: dict[str, Any] = {
        "instance_id": 123,
        "allow_list": ["192.0.2.1/32"],
        "engine_config": {"binlog_retention_period": 600},
        "label": "primary-db",
        "private_network": {"vpc_id": 456},
        "type": "g6-dedicated-4",
        "updates": {"hour_of_day": 2},
        "version": "8.0.35",
        "confirm": True,
    }
    expected_payload = {
        key: value
        for key, value in arguments.items()
        if key not in {"instance_id", "confirm"}
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_update", arguments)

    assert json.loads(result[0].text) == response_data
    mock_client.update_mysql_database_instance.assert_awaited_once_with(
        123, expected_payload
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_update_rejects_non_true_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database update requires literal confirm=true before client calls."""
    arguments: dict[str, Any] = {"instance_id": 123, "label": "primary-db"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_update", arguments)

    assert "Set confirm=true to proceed" in result[0].text
    mock_client.update_mysql_database_instance.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"label": "primary-db", "confirm": True}, "instance_id is required"),
        (
            {"instance_id": "123", "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "12/3", "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "12?3", "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "12#3", "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "..", "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": True, "label": "primary-db", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": 0, "label": "primary-db", "confirm": True},
            "instance_id must be at least 1",
        ),
        (
            {"instance_id": 123, "confirm": True},
            "at least one update field is required",
        ),
        (
            {"instance_id": 123, "unknown": "value", "confirm": True},
            "unsupported argument: unknown",
        ),
        (
            {"instance_id": 123, "allow_list": [""], "confirm": True},
            "allow_list must be an array of non-empty strings",
        ),
        (
            {"instance_id": 123, "engine_config": [], "confirm": True},
            "engine_config must be an object",
        ),
        (
            {"instance_id": 123, "engine_config": None, "confirm": True},
            "engine_config must be an object",
        ),
        (
            {"instance_id": 123, "private_network": "vpc", "confirm": True},
            "private_network must be an object or null",
        ),
        (
            {"instance_id": 123, "updates": "weekly", "confirm": True},
            "updates must be an object",
        ),
        (
            {"instance_id": 123, "updates": None, "confirm": True},
            "updates must be an object",
        ),
        (
            {"instance_id": 123, "label": " primary-db", "confirm": True},
            "label must not include leading or trailing whitespace",
        ),
        (
            {"instance_id": 123, "type": 42, "confirm": True},
            "type must be a string",
        ),
        (
            {"instance_id": 123, "version": "", "confirm": True},
            "version must be a non-empty string",
        ),
    ],
)
async def test_database_mysql_instance_update_rejects_invalid_arguments(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL database update rejects invalid inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_update", arguments)

    assert expected_error in result[0].text
    mock_client.update_mysql_database_instance.assert_not_called()


async def test_database_mysql_instance_update_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """MySQL database update dry-run returns the PUT preview and body."""
    arguments: dict[str, Any] = {
        "instance_id": 123,
        "label": "primary-db",
        "confirm": True,
        "dry_run": True,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_update", arguments)

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "PUT",
        "path": "/databases/mysql/instances/123",
        "body": {"label": "primary-db"},
    }
    mock_client.update_mysql_database_instance.assert_not_called()


async def test_client_update_postgresql_database_instance_exact_path_body() -> None:
    """Client sends the documented PostgreSQL database update route and body."""
    response_data = {"id": 321, "label": "pg-primary"}
    response = _JsonResponse(response_data)
    payload = {"label": "pg-primary", "updates": {"hour_of_day": 3}}
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        result = await client.update_postgresql_database_instance(321, payload)

    assert result == response_data
    make_request.assert_awaited_once_with(
        "PUT", "/databases/postgresql/instances/321", payload
    )
    await client.close()


async def test_client_update_postgresql_database_instance_encodes_path_param() -> None:
    """Client URL-encodes the PostgreSQL instance ID path parameter boundary."""
    response = _JsonResponse({"id": 321})
    payload = {"label": "pg-primary"}
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=response)
    ) as make_request:
        await client.update_postgresql_database_instance(cast("Any", "32/1?"), payload)

    make_request.assert_awaited_once_with(
        "PUT", "/databases/postgresql/instances/32%2F1%3F", payload
    )
    await client.close()


async def test_client_update_postgresql_database_instance_maps_http_error() -> None:
    """Client maps PostgreSQL database update HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client, "make_request", AsyncMock(side_effect=httpx.ConnectError("boom"))
        ),
        pytest.raises(NetworkError, match="UpdatePostgreSQLDatabaseInstance"),
    ):
        await client.update_postgresql_database_instance(321, {"label": "pg-primary"})
    await client.close()


async def test_retryable_client_update_postgresql_db_delegates_without_retry() -> None:
    """Mutating PostgreSQL database update is not replayed through retry wrapper."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.update_postgresql_database_instance = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError("UpdatePostgresqlDatabaseInstance", Exception("boom"))
    )
    try:
        with pytest.raises(NetworkError):
            await retry_client.update_postgresql_database_instance(
                321, {"label": "pg-primary"}
            )
    finally:
        await retry_client.close()

    retry_client.client.update_postgresql_database_instance.assert_awaited_once_with(
        321, {"label": "pg-primary"}
    )


async def test_database_postgresql_instance_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL database update tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_postgresql_instance_update_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instance_update" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_database_postgresql_instance_update_tool()
    )
    assert tool.name == "linode_database_postgresql_instance_update"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {"instance_id", "confirm"}
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_postgresql_instance_update" in srv.registered_tool_names
    assert (
        "linode_database_postgresql_instance_update"
        in get_version_info().features["tools"]
    )


async def test_database_postgresql_instance_update_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL database update is callable through server dispatch."""
    response_data = {"id": 321, "label": "pg-primary"}
    arguments: dict[str, Any] = {
        "instance_id": 321,
        "allow_list": ["192.0.2.10/32"],
        "engine_config": {"shared_buffers": "1GB"},
        "label": "pg-primary",
        "private_network": {"vpc_id": 456},
        "type": "g6-dedicated-4",
        "updates": {"hour_of_day": 3},
        "version": "16",
        "confirm": True,
    }
    expected_payload = {
        key: value
        for key, value in arguments.items()
        if key not in {"instance_id", "confirm"}
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_postgresql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_update", arguments
        )

    assert json.loads(result[0].text) == response_data
    mock_client.update_postgresql_database_instance.assert_awaited_once_with(
        321, expected_payload
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_postgresql_instance_update_rejects_non_true_confirm(
    sample_config: Config, confirm: object
) -> None:
    """PostgreSQL database update requires literal confirm=true before client calls."""
    arguments: dict[str, Any] = {"instance_id": 321, "label": "pg-primary"}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_update", arguments
        )

    assert "Set confirm=true to proceed" in result[0].text
    mock_client.update_postgresql_database_instance.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"label": "pg-primary", "confirm": True}, "instance_id is required"),
        (
            {"instance_id": "321", "label": "pg-primary", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "32/1", "label": "pg-primary", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "32?1", "label": "pg-primary", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": "..", "label": "pg-primary", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": True, "label": "pg-primary", "confirm": True},
            "instance_id must be an integer",
        ),
        (
            {"instance_id": 0, "label": "pg-primary", "confirm": True},
            "instance_id must be at least 1",
        ),
        (
            {"instance_id": 321, "confirm": True},
            "at least one update field is required",
        ),
        (
            {"instance_id": 321, "unknown": "value", "confirm": True},
            "unsupported argument: unknown",
        ),
        (
            {"instance_id": 321, "allow_list": [""], "confirm": True},
            "allow_list must be an array of non-empty strings",
        ),
        (
            {"instance_id": 321, "engine_config": [], "confirm": True},
            "engine_config must be an object",
        ),
        (
            {"instance_id": 321, "private_network": "vpc", "confirm": True},
            "private_network must be an object or null",
        ),
        (
            {"instance_id": 321, "updates": "weekly", "confirm": True},
            "updates must be an object",
        ),
        (
            {"instance_id": 321, "label": " pg-primary", "confirm": True},
            "label must not include leading or trailing whitespace",
        ),
        (
            {"instance_id": 321, "type": 42, "confirm": True},
            "type must be a string",
        ),
        (
            {"instance_id": 321, "version": "", "confirm": True},
            "version must be a non-empty string",
        ),
    ],
)
async def test_database_postgresql_instance_update_rejects_invalid_arguments(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL database update rejects invalid inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_update", arguments
        )

    assert expected_error in result[0].text
    mock_client.update_postgresql_database_instance.assert_not_called()


async def test_database_postgresql_instance_update_dry_run_previews_without_client_call(
    sample_config: Config,
) -> None:
    """PostgreSQL database update dry-run returns the PUT preview and body."""
    arguments: dict[str, Any] = {
        "instance_id": 321,
        "label": "pg-primary",
        "confirm": True,
        "dry_run": True,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_postgresql_instance_update", arguments
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "PUT",
        "path": "/databases/postgresql/instances/321",
        "body": {"label": "pg-primary"},
    }
    mock_client.update_postgresql_database_instance.assert_not_called()


async def test_database_mysql_instance_patch_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL Managed Database patch tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_mysql_instance_patch_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instance_patch" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instance_patch_tool()
    assert tool.name == "linode_database_mysql_instance_patch"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["instance_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["instance_id", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_database_mysql_instance_patch" in srv.registered_tool_names
    assert (
        "linode_database_mysql_instance_patch" in get_version_info().features["tools"]
    )


async def test_database_mysql_instance_patch_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL Managed Database patch is callable through server dispatch."""
    response_data = {"id": 123, "label": "primary-db"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.patch_mysql_database_instance.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_patch",
            {"instance_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.patch_mysql_database_instance.assert_awaited_once_with(123)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_database_mysql_instance_patch_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """MySQL database patch rejects non-true confirm before client calls."""
    arguments: dict[str, object] = {"instance_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_patch", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "arguments",
    [
        {"confirm": True},
        {"instance_id": 0, "confirm": True},
        {"instance_id": True, "confirm": True},
        {"instance_id": "123/456", "confirm": True},
        {"instance_id": "123?456", "confirm": True},
        {"instance_id": "..", "confirm": True},
    ],
)
async def test_database_mysql_instance_patch_rejects_invalid_instance_id(
    sample_config: Config, arguments: dict[str, object]
) -> None:
    """MySQL database patch rejects invalid path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_database_mysql_instance_patch", arguments)

    assert "instance_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


async def test_database_mysql_instance_patch_dry_run_encodes_path(
    sample_config: Config,
) -> None:
    """MySQL database patch dry-run returns the POST preview."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_database_mysql_instance_patch",
            {"instance_id": 123, "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/databases/mysql/instances/123/patch",
    }
    mock_client_class.assert_not_called()


async def test_database_mysql_instances_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """MySQL Managed Database list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_mysql_instances_list_tool" in tools_mod.__all__
    assert "handle_linode_database_mysql_instances_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_mysql_instances_list_tool()
    assert tool.name == "linode_database_mysql_instances_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_database_mysql_instances_list" in srv.registered_tool_names


async def test_database_mysql_instances_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """MySQL Managed Database list is callable through server dispatch."""
    response_data = {
        "data": [{"id": 123, "label": "primary-db"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_mysql_database_instances.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_mysql_instances_list", {"page": 1, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_mysql_database_instances.assert_awaited_once_with(
        page=1, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "1"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_database_mysql_instances_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL Managed Database list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_mysql_instances_list", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": False}, "page_size must be an integer"),
    ],
)
async def test_database_mysql_instances_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """MySQL Managed Database list rejects invalid page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_mysql_instances_list", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_client_list_postgresql_database_instances_uses_exact_query() -> None:
    """Client sends the documented PostgreSQL database instances list route."""
    client = Client("https://api.linode.test/v4", "token")
    response_data = {
        "data": [{"id": 456, "label": "pg-db"}],
        "page": 2,
        "pages": 3,
        "results": 75,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as make_request:
        make_request.return_value = _JsonResponse(response_data)
        result = await client.list_postgresql_database_instances(page=2, page_size=25)

    assert result == response_data
    make_request.assert_awaited_once_with(
        "GET", "/databases/postgresql/instances?page=2&page_size=25"
    )
    await client.close()


async def test_retryable_client_list_postgresql_databases_uses_retry() -> None:
    """Retryable PostgreSQL list delegates through retry."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    response_data = {
        "data": [{"id": 456, "label": "pg-db"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    retry_client.client.list_postgresql_database_instances = AsyncMock(  # type: ignore[method-assign]
        return_value=response_data
    )

    with patch.object(
        retry_client, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:

        async def run_call(call: Any) -> dict[str, Any]:
            result = await call()
            return cast("dict[str, Any]", result)

        execute_with_retry.side_effect = run_call
        result = await retry_client.list_postgresql_database_instances(
            page=1, page_size=50
        )

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.list_postgresql_database_instances.assert_awaited_once_with(
        page=1, page_size=50
    )
    await retry_client.client.close()


async def test_database_postgresql_instances_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database list tool should be exported and registered."""
    from linodemcp import tools as tools_mod
    from linodemcp.version import get_version_info

    assert "create_linode_database_postgresql_instances_list_tool" in tools_mod.__all__
    assert "handle_linode_database_postgresql_instances_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_postgresql_instances_list_tool()
    assert tool.name == "linode_database_postgresql_instances_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_database_postgresql_instances_list" in srv.registered_tool_names
    assert "linode_database_postgresql_instances_list" in get_version_info().features[
        "tools"
    ].split(",")


async def test_database_postgresql_instances_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """PostgreSQL Managed Database list is callable through server dispatch."""
    response_data = {
        "data": [{"id": 456, "label": "pg-db"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_postgresql_database_instances.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instances_list",
            {"page": 1, "page_size": 25},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_postgresql_database_instances.assert_awaited_once_with(
        page=1, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "1"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_database_postgresql_instances_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL Managed Database list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instances_list", arguments
        )

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": False}, "page_size must be an integer"),
    ],
)
async def test_database_postgresql_instances_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """PostgreSQL Managed Database list rejects invalid page_size."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_postgresql_instances_list", arguments
        )

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()


async def test_database_instances_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Managed Database list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_instances_list_tool" in tools_mod.__all__
    assert "handle_linode_database_instances_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_database_instances_list_tool()
    assert tool.name == "linode_database_instances_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_database_instances_list" in srv.registered_tool_names


async def test_database_instances_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Managed Database list is callable through server dispatch."""
    response_data = {
        "data": [{"id": 123, "label": "primary-db"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_database_instances.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_instances_list", {"page": 1, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_database_instances.assert_awaited_once_with(page=1, page_size=25)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "1"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_database_instances_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Managed Database list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_instances_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_database_instances.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_database_instances_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Managed Database list rejects invalid page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_instances_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_database_instances.assert_not_called()


async def test_betas_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Global betas list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_betas_list_tool" in tools_mod.__all__
    assert "handle_linode_betas_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_betas_list_tool()
    assert tool.name == "linode_betas_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_betas_list" in srv.registered_tool_names


async def test_betas_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Global betas list is callable through server dispatch."""
    response_data = {
        "data": [{"id": "VPC", "label": "VPC Beta"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_betas.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_betas_list", {"page": 1, "page_size": 25})

    assert json.loads(result[0].text) == response_data
    mock_client.list_betas.assert_awaited_once_with(page=1, page_size=25)


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "1"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_betas_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Global betas list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_betas_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_betas.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_betas_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Global betas list rejects invalid page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_betas_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_betas.assert_not_called()


async def test_account_child_accounts_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account child accounts list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_child_accounts_list_tool" in tools_mod.__all__
    assert "handle_linode_account_child_accounts_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_child_accounts_list_tool()
    assert tool.name == "linode_account_child_accounts_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_account_child_accounts_list" in srv.registered_tool_names


async def test_account_child_accounts_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account child accounts list is callable through server dispatch."""
    response_data = {
        "data": [
            {
                "euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56",
                "company": "Example Child",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_child_accounts.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_child_accounts_list", {"page": 1, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_child_accounts.assert_awaited_once_with(
        page=1, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "1"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_account_child_accounts_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account child accounts list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_child_accounts_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_account_child_accounts.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "25"}, "page_size must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_account_child_accounts_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account child accounts list rejects invalid page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_child_accounts_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_account_child_accounts.assert_not_called()


async def test_account_service_transfers_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account service transfers list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_service_transfers_list_tool" in tools_mod.__all__
    assert "handle_linode_account_service_transfers_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_service_transfers_list_tool()
    assert tool.name == "linode_account_service_transfers_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500

    srv = Server(sample_config)
    assert "linode_account_service_transfers_list" in srv.registered_tool_names


async def test_account_service_transfers_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account service transfers list is callable through server dispatch."""
    response_data = {
        "data": [{"token": "service-token-example", "status": "pending"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_account_service_transfers.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_service_transfers_list", {"page": 1, "page_size": 25}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_account_service_transfers.assert_awaited_once_with(
        page=1, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page": "abc"}, "page must be an integer"),
        ({"page": True}, "page must be an integer"),
    ],
)
async def test_account_service_transfers_list_rejects_invalid_page(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account service transfers list rejects invalid page before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_service_transfers_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_account_service_transfers.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page_size": "abc"}, "page_size must be an integer"),
        ({"page_size": False}, "page_size must be an integer"),
    ],
)
async def test_account_service_transfers_list_rejects_invalid_page_size(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Account service transfers list rejects invalid page_size before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_service_transfers_list", arguments)

    assert expected_error in result[0].text
    mock_client.list_account_service_transfers.assert_not_called()


async def test_account_child_account_token_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Child-account token create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_child_account_token_create_tool" in tools_mod.__all__
    assert "handle_linode_account_child_account_token_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_child_account_token_create_tool()
    assert tool.name == "linode_account_child_account_token_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["euuid"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_child_account_token_create" in srv.registered_tool_names


async def test_account_child_account_token_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Child-account token create is callable through server dispatch."""
    response_data = {"token": "proxy-token", "expiry": "2026-06-02T00:00:00"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_account_child_account_token.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_child_account_token_create",
            {"euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56", "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.create_account_child_account_token.assert_awaited_once_with(
        "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
    )


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_child_account_token_create_requires_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Token creation rejects missing or non-literal confirm before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        arguments: dict[str, object] = {"euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56"}
        if confirm_value is not None:
            arguments["confirm"] = confirm_value
        result = await srv.dispatch(
            "linode_account_child_account_token_create", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_child_account_token.assert_not_called()


@pytest.mark.parametrize(
    "euuid",
    [
        None,
        "",
        "   ",
        123,
        "child/account",
        "child?account",
        "child#account",
        "..",
        "%2F",
        "%3F",
        "%23",
        "%2e%2e",
        "child-123",
        "A1BC2DEF-34GH-567I-J890KLMN12O34P5/",
    ],
)
async def test_account_child_account_token_create_rejects_invalid_euuid(
    sample_config: Config, euuid: object
) -> None:
    """Token creation validates child-account path parameter before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        arguments: dict[str, object] = {"confirm": True}
        if euuid is not None:
            arguments["euuid"] = euuid
        result = await srv.dispatch(
            "linode_account_child_account_token_create", arguments
        )

    assert "euuid must match the child account EUUID format" in result[0].text
    mock_client.create_account_child_account_token.assert_not_called()


async def test_account_child_account_token_create_dry_run_skips_client(
    sample_config: Config,
) -> None:
    """Token creation dry-run previews request without calling the client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_child_account_token_create",
            {
                "euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56",
                "confirm": True,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token",
    }
    mock_client.create_account_child_account_token.assert_not_called()


async def test_account_tags_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account tags list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_tags_list_tool" in tools_mod.__all__
    assert "handle_linode_account_tags_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_tags_list" in srv.registered_tool_names


async def test_account_tag_objects_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account tag objects list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_tag_objects_list_tool" in tools_mod.__all__
    assert "handle_linode_account_tag_objects_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_tag_objects_list" in srv.registered_tool_names


async def test_account_tag_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account tag delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_tag_delete_tool" in tools_mod.__all__
    assert "handle_linode_account_tag_delete" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_tag_delete" in srv.registered_tool_names


async def test_account_support_ticket_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_create_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_support_ticket_create" in srv.registered_tool_names


async def test_account_support_ticket_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support ticket create is callable through server dispatch."""
    response_data = {"id": 789, "summary": "Need help"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_support_ticket_create",
            {
                "confirm": True,
                "summary": "Need help",
                "description": "Details",
            },
        )

    assert json.loads(result[0].text) == {
        "message": "Support ticket opened successfully",
        "ticket": response_data,
    }
    mock_client.create_support_ticket.assert_awaited_once()


async def test_account_support_ticket_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_get_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_support_ticket_get" in srv.registered_tool_names


async def test_managed_stats_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Managed stats tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_managed_stats_tool" in tools_mod.__all__
    assert "handle_linode_managed_stats" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_managed_stats" in srv.registered_tool_names


async def test_managed_stats_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Managed stats is callable through server dispatch."""
    response_data: dict[str, object] = {"data": {"cpu": []}}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_managed_stats.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_managed_stats", {})

    assert len(result) == 1
    assert json.loads(result[0].text) == response_data
    mock_client.get_managed_stats.assert_awaited_once_with()


async def test_account_support_tickets_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support tickets list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_tickets_list_tool" in tools_mod.__all__
    assert "handle_linode_account_support_tickets_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_support_tickets_list" in srv.registered_tool_names


async def test_account_support_tickets_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support tickets list is callable through server dispatch."""
    response_data = {"data": [{"id": 789}], "page": 1, "pages": 1, "results": 1}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_tickets.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_account_support_tickets_list", {})

    assert json.loads(result[0].text) == response_data
    mock_client.list_support_tickets.assert_awaited_once_with(page=None, page_size=None)


async def test_account_support_ticket_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support ticket get is callable through server dispatch."""
    response_data = {"id": 123, "summary": "Need help"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_support_ticket_get", {"ticket_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_support_ticket.assert_awaited_once_with(123)


async def test_account_support_ticket_replies_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket replies list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_replies_list_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_replies_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_account_support_ticket_replies_list" in srv.registered_tool_names


async def test_account_support_ticket_replies_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support ticket replies list is callable through server dispatch."""
    response_data = {"data": [{"id": 456}], "page": 1, "pages": 1, "results": 1}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_support_ticket_replies.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_account_support_ticket_replies_list", {"ticket_id": 123}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.list_support_ticket_replies.assert_awaited_once_with(
        123, page=None, page_size=None
    )


class _JsonResponse:
    def __init__(self, data: dict[str, Any]) -> None:
        self._data = data

    def json(self) -> dict[str, Any]:
        return self._data


async def test_database_engines_client_sends_exact_route_and_query() -> None:
    """Client database engines list sends the documented GET route and query."""
    response_data: dict[str, Any] = {
        "data": [{"id": "mysql", "version": "8.0.26"}],
        "page": 2,
        "pages": 3,
        "results": 61,
    }
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock(  # type: ignore[method-assign]
        return_value=_JsonResponse(response_data)
    )

    try:
        result = await client.list_database_engines(page=2, page_size=25)
    finally:
        await client.close()

    assert result == response_data
    client.make_request.assert_awaited_once_with(
        "GET", "/databases/engines?page=2&page_size=25"
    )


async def test_database_engines_client_maps_http_errors() -> None:
    """Client database engines list maps HTTP errors to NetworkError."""
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock(  # type: ignore[method-assign]
        side_effect=httpx.HTTPError("boom")
    )

    try:
        with pytest.raises(NetworkError, match="ListDatabaseEngines"):
            await client.list_database_engines()
    finally:
        await client.close()

    client.make_request.assert_awaited_once_with("GET", "/databases/engines")


@pytest.mark.parametrize(
    ("kwargs", "message"),
    [
        ({"page": 0}, "page must be an integer at least 1"),
        ({"page": True}, "page must be an integer at least 1"),
        ({"page": "2"}, "page must be an integer at least 1"),
        ({"page_size": 10}, "page_size must be an integer between 25 and 500"),
        ({"page_size": 501}, "page_size must be an integer between 25 and 500"),
        ({"page_size": True}, "page_size must be an integer between 25 and 500"),
        ({"page_size": "25"}, "page_size must be an integer between 25 and 500"),
    ],
)
async def test_database_engines_client_rejects_invalid_pagination(
    kwargs: dict[str, Any], message: str
) -> None:
    """Client database engines list validates pagination before requests."""
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock()  # type: ignore[method-assign]

    try:
        with pytest.raises(ValueError, match=message):
            await client.list_database_engines(**kwargs)
    finally:
        await client.close()

    client.make_request.assert_not_called()


async def test_database_engines_retryable_client_delegates_with_pagination() -> None:
    """Retryable database engines list delegates with pagination."""
    response_data: dict[str, Any] = {
        "data": [{"id": "postgresql", "version": "16"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.list_database_engines = AsyncMock(  # type: ignore[method-assign]
        return_value=response_data
    )

    try:
        result = await retry_client.list_database_engines(page=3, page_size=50)
    finally:
        await retry_client.close()

    assert result == response_data
    retry_client.client.list_database_engines.assert_awaited_once_with(
        page=3, page_size=50
    )


@pytest.mark.parametrize("restricted", [True, False])
async def test_account_user_create_client_sends_exact_route_and_body(
    restricted: bool,
) -> None:
    """Client user create sends the documented POST body."""
    client = Client("https://api.example.test/v4", "token")
    client.make_request = AsyncMock(  # type: ignore[method-assign]
        return_value=_JsonResponse(
            {
                "username": "newuser",
                "email": "new@example.test",
                "restricted": restricted,
            }
        )
    )

    try:
        result = await client.create_account_user(
            "newuser", "new@example.test", restricted
        )
    finally:
        await client.close()

    assert result["username"] == "newuser"
    client.make_request.assert_awaited_once_with(
        "POST",
        "/account/users",
        {"username": "newuser", "email": "new@example.test", "restricted": restricted},
    )


async def test_account_user_create_retryable_client_does_not_replay() -> None:
    """Retryable user create delegates once without generic retry replay."""
    retry_client = RetryableClient("https://api.example.test/v4", "token")
    retry_client.client.create_account_user = AsyncMock(  # type: ignore[method-assign]
        side_effect=NetworkError("CreateAccountUser", Exception("boom"))
    )
    try:
        with pytest.raises(NetworkError):
            await retry_client.create_account_user("newuser", "new@example.test", True)
    finally:
        await retry_client.close()

    retry_client.client.create_account_user.assert_awaited_once_with(
        "newuser", "new@example.test", True
    )


async def test_account_user_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account user create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_create_tool" in tools_mod.__all__
    assert "handle_linode_account_user_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_user_create_tool()
    assert tool.name == "linode_account_user_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["username"]["minLength"] == 1
    assert tool.inputSchema["properties"]["email"]["minLength"] == 1
    assert tool.inputSchema["properties"]["restricted"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == [
        "username",
        "email",
        "restricted",
        "confirm",
    ]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_user_create" in srv.registered_tool_names


async def test_account_user_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account user create dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock(
        return_value={
            "username": "newuser",
            "email": "new@example.test",
            "restricted": True,
        }
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_create",
            {
                "username": "newuser",
                "email": "new@example.test",
                "restricted": True,
                "confirm": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Account user created successfully"
    assert payload["user"]["username"] == "newuser"
    mock_client.create_account_user.assert_awaited_once_with(
        "newuser", "new@example.test", True
    )


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_user_create_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Account user create rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock()
    arguments: dict[str, object] = {
        "username": "newuser",
        "email": "new@example.test",
        "restricted": True,
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_create", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_user.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("username", None, "username is required"),
        ("username", 123, "username is required"),
        ("username", "   ", "username is required"),
        ("email", None, "email is required"),
        ("email", 123, "email is required"),
        ("email", "   ", "email is required"),
    ],
)
async def test_account_user_create_validates_required_arguments(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Account user create validates required string arguments first."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock()
    arguments: dict[str, object] = {
        "username": "newuser",
        "email": "new@example.test",
        "restricted": True,
        "confirm": True,
    }
    if value is None:
        arguments.pop(field)
    else:
        arguments[field] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_create", arguments)

    assert message in result[0].text
    mock_client.create_account_user.assert_not_called()


@pytest.mark.parametrize("restricted_value", [None, "true", "false", 1, 0])
async def test_account_user_create_requires_boolean_restricted(
    sample_config: Config, restricted_value: object
) -> None:
    """Account user create rejects ambiguous access semantics before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock()
    arguments: dict[str, object] = {
        "username": "newuser",
        "email": "new@example.test",
        "confirm": True,
    }
    if restricted_value is not None:
        arguments["restricted"] = restricted_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_user_create", arguments)

    assert "restricted is required and must be a boolean" in result[0].text
    mock_client.create_account_user.assert_not_called()


async def test_account_user_create_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Account user create dry-run previews the request without creating."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_create",
            {
                "username": "newuser",
                "email": "new@example.test",
                "restricted": True,
                "confirm": True,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_user_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/users",
        "body": {
            "username": "newuser",
            "email": "new@example.test",
            "restricted": True,
        },
    }
    mock_client.create_account_user.assert_not_called()


async def test_account_user_create_dry_run_requires_confirm(
    sample_config: Config,
) -> None:
    """Account user create dry-run still requires explicit confirm."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_create",
            {
                "username": "newuser",
                "email": "new@example.test",
                "restricted": True,
                "dry_run": True,
            },
        )

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_user.assert_not_called()


async def test_account_user_create_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Account user create reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.create_account_user = AsyncMock(
        side_effect=NetworkError("CreateAccountUser", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_user_create",
            {
                "username": "newuser",
                "email": "new@example.test",
                "restricted": True,
                "confirm": True,
            },
        )

    assert "CreateAccountUser" in result[0].text
    mock_client.create_account_user.assert_awaited_once()


async def test_account_oauth_client_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """OAuth client create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_create_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_oauth_client_create_tool()
    assert tool.name == "linode_account_oauth_client_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["label"]["minLength"] == 1
    assert tool.inputSchema["properties"]["redirect_uri"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_oauth_client_create" in srv.registered_tool_names


async def test_account_payment_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Payment create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_create_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_payment_create_tool()
    assert tool.name == "linode_account_payment_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["payment_method_id"]["type"] == "integer"
    assert tool.inputSchema["properties"]["payment_method_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["usd"]["type"] == "string"
    assert tool.inputSchema["properties"]["usd"]["pattern"] == (
        r"^(?!0+(?:\.0{1,2})?$)\d+(?:\.\d{1,2})?$"
    )
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_payment_create" in srv.registered_tool_names


async def test_account_service_transfer_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Service transfer create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_service_transfer_create_tool" in tools_mod.__all__
    assert "handle_linode_account_service_transfer_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_service_transfer_create_tool()
    assert tool.name == "linode_account_service_transfer_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["linode_ids"]["type"] == "array"
    assert tool.inputSchema["properties"]["linode_ids"]["minItems"] == 1
    assert tool.inputSchema["properties"]["linode_ids"]["items"] == {
        "type": "integer",
        "minimum": 1,
    }
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "linode_ids" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_service_transfer_create" in srv.registered_tool_names


async def test_account_payment_method_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Payment method create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_payment_method_create_tool" in tools_mod.__all__
    assert "handle_linode_account_payment_method_create" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_payment_method_create_tool()
    assert tool.name == "linode_account_payment_method_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["type"]["enum"] == ["credit_card"]
    assert tool.inputSchema["properties"]["data"]["type"] == "object"
    assert tool.inputSchema["properties"]["is_default"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_payment_method_create" in srv.registered_tool_names


async def test_account_promo_credit_add_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Promo credit add tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_promo_credit_add_tool" in tools_mod.__all__
    assert "handle_linode_account_promo_credit_add" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_promo_credit_add_tool()
    assert tool.name == "linode_account_promo_credit_add"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["promo_code"]["type"] == "string"
    assert tool.inputSchema["properties"]["promo_code"]["minLength"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["promo_code", "confirm"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_promo_credit_add" in srv.registered_tool_names


async def test_account_oauth_client_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """OAuth client delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_delete_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_delete" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_oauth_client_delete_tool()
    assert tool.name == "linode_account_oauth_client_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["properties"]["client_id"]["minLength"] == 1
    assert (
        tool.inputSchema["properties"]["client_id"]["pattern"]
        == r"^[A-Za-z0-9][A-Za-z0-9_-]*$"
    )
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_oauth_client_delete" in srv.registered_tool_names


async def test_account_oauth_client_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """OAuth client create dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.create_account_oauth_client = AsyncMock(
        return_value={
            "id": "client-123",
            "label": "demo-client",
            "redirect_uri": "https://example.com/cb",
            "secret": "shown-once",
        }
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_create",
            {
                "label": "demo-client",
                "redirect_uri": "https://example.com/cb",
                "confirm": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["label"] == "demo-client"
    assert payload["secret"] == "shown-once"
    mock_client.create_account_oauth_client.assert_awaited_once_with(
        "demo-client", "https://example.com/cb"
    )


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_oauth_client_create_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """OAuth client create rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_oauth_client = AsyncMock()
    arguments: dict[str, object] = {
        "label": "demo-client",
        "redirect_uri": "https://example.com/cb",
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_oauth_client_create", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_oauth_client.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("label", None, "label is required"),
        ("label", 123, "label must be a string"),
        ("label", "   ", "label is required"),
        ("redirect_uri", None, "redirect_uri is required"),
        ("redirect_uri", 123, "redirect_uri must be a string"),
        ("redirect_uri", "   ", "redirect_uri is required"),
    ],
)
async def test_account_oauth_client_create_validates_required_arguments(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """OAuth client create validates required string arguments first."""
    mock_client = AsyncMock()
    mock_client.create_account_oauth_client = AsyncMock()
    arguments: dict[str, object] = {
        "label": "demo-client",
        "redirect_uri": "https://example.com/cb",
        "confirm": True,
    }
    if value is None:
        arguments.pop(field)
    else:
        arguments[field] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_oauth_client_create", arguments)

    assert message in result[0].text
    mock_client.create_account_oauth_client.assert_not_called()


async def test_account_oauth_client_create_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """OAuth client create dry-run previews the request without creating."""
    mock_client = AsyncMock()
    mock_client.create_account_oauth_client = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_create",
            {
                "label": "demo-client",
                "redirect_uri": "https://example.com/cb",
                "confirm": False,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_oauth_client_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/oauth-clients",
        "body": {
            "label": "demo-client",
            "redirect_uri": "https://example.com/cb",
        },
    }
    mock_client.create_account_oauth_client.assert_not_called()


async def test_account_oauth_client_create_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """OAuth client create reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.create_account_oauth_client = AsyncMock(
        side_effect=NetworkError("CreateAccountOAuthClient", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_create",
            {
                "label": "demo-client",
                "redirect_uri": "https://example.com/cb",
                "confirm": True,
            },
        )

    assert "CreateAccountOAuthClient" in result[0].text
    mock_client.create_account_oauth_client.assert_awaited_once()


async def test_account_payment_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Payment create dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.create_account_payment = AsyncMock(
        return_value={"id": 456, "payment_method_id": 123, "usd": "25.00"}
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_create",
            {"payment_method_id": 123, "usd": "25.00", "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["payment_method_id"] == 123
    assert payload["usd"] == "25.00"
    mock_client.create_account_payment.assert_awaited_once_with(123, "25.00")


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_payment_create_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Payment create rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_payment = AsyncMock()
    arguments: dict[str, object] = {"payment_method_id": 123, "usd": "25.00"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_payment_create", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_payment.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("payment_method_id", None, "payment_method_id is required"),
        (
            "payment_method_id",
            False,
            "payment_method_id must be a positive integer",
        ),
        ("payment_method_id", 0, "payment_method_id must be a positive integer"),
        (
            "payment_method_id",
            "123",
            "payment_method_id must be a positive integer",
        ),
        ("usd", None, "usd is required"),
        ("usd", 123, "usd must be a string"),
        ("usd", "   ", "usd is required"),
        (
            "usd",
            "abc",
            "usd must be a positive dollar amount with up to two decimals",
        ),
        (
            "usd",
            "-1",
            "usd must be a positive dollar amount with up to two decimals",
        ),
        (
            "usd",
            "0",
            "usd must be a positive dollar amount with up to two decimals",
        ),
        (
            "usd",
            "0.00",
            "usd must be a positive dollar amount with up to two decimals",
        ),
        (
            "usd",
            "1e6",
            "usd must be a positive dollar amount with up to two decimals",
        ),
        (
            "usd",
            "25.001",
            "usd must be a positive dollar amount with up to two decimals",
        ),
    ],
)
async def test_account_payment_create_validates_required_arguments(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Payment create validates required arguments before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_payment = AsyncMock()
    arguments: dict[str, object] = {
        "payment_method_id": 123,
        "usd": "25.00",
        "confirm": True,
    }
    if value is None:
        arguments.pop(field)
    else:
        arguments[field] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_payment_create", arguments)

    assert message in result[0].text
    mock_client.create_account_payment.assert_not_called()


async def test_account_payment_create_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Payment create dry-run previews the request without creating."""
    mock_client = AsyncMock()
    mock_client.create_account_payment = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_create",
            {
                "payment_method_id": 123,
                "usd": "25.00",
                "confirm": False,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_payment_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/payments",
        "body": {"payment_method_id": 123, "usd": "25.00"},
    }
    mock_client.create_account_payment.assert_not_called()


async def test_account_payment_create_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Payment create reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.create_account_payment = AsyncMock(
        side_effect=NetworkError("CreateAccountPayment", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_create",
            {"payment_method_id": 123, "usd": "25.00", "confirm": True},
        )

    assert "CreateAccountPayment" in result[0].text
    mock_client.create_account_payment.assert_awaited_once()


async def test_account_service_transfer_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Service transfer create dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.create_account_service_transfer = AsyncMock(
        return_value={
            "token": "service-transfer-token",
            "status": "pending",
            "entities": {"linodes": [123, 456]},
        }
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_create",
            {"linode_ids": [123, 456], "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["token"] == "service-transfer-token"
    assert payload["entities"]["linodes"] == [123, 456]
    mock_client.create_account_service_transfer.assert_awaited_once_with([123, 456])


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_service_transfer_create_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Service transfer create rejects missing/non-true confirm first."""
    mock_client = AsyncMock()
    mock_client.create_account_service_transfer = AsyncMock()
    arguments: dict[str, object] = {"linode_ids": [123]}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_create", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_service_transfer.assert_not_called()


@pytest.mark.parametrize(
    ("value", "message"),
    [
        (None, "linode_ids is required"),
        ([], "linode_ids must be a non-empty list of positive integers"),
        ("123", "linode_ids must be a non-empty list of positive integers"),
        ([0], "linode_ids must be a non-empty list of positive integers"),
        ([-1], "linode_ids must be a non-empty list of positive integers"),
        ([True], "linode_ids must be a non-empty list of positive integers"),
        (["123"], "linode_ids must be a non-empty list of positive integers"),
    ],
)
async def test_account_service_transfer_create_validates_linode_ids(
    sample_config: Config, value: object, message: str
) -> None:
    """Service transfer create validates Linode IDs before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_service_transfer = AsyncMock()
    arguments: dict[str, object] = {"linode_ids": [123], "confirm": True}
    if value is None:
        arguments.pop("linode_ids")
    else:
        arguments["linode_ids"] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_service_transfer_create", arguments)

    assert message in result[0].text
    mock_client.create_account_service_transfer.assert_not_called()


async def test_account_service_transfer_create_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Service transfer create dry-run previews without creating."""
    mock_client = AsyncMock()
    mock_client.create_account_service_transfer = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_create",
            {"linode_ids": [123, 456], "confirm": False, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_service_transfer_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/service-transfers",
        "body": {"entities": {"linodes": [123, 456]}},
    }
    mock_client.create_account_service_transfer.assert_not_called()


async def test_account_service_transfer_create_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Service transfer create reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.create_account_service_transfer = AsyncMock(
        side_effect=NetworkError("CreateAccountServiceTransfer", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_service_transfer_create",
            {"linode_ids": [123], "confirm": True},
        )

    assert "CreateAccountServiceTransfer" in result[0].text
    mock_client.create_account_service_transfer.assert_awaited_once()


async def test_account_payment_method_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Payment method create dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.create_account_payment_method = AsyncMock(
        return_value={
            "id": 123,
            "type": "credit_card",
            "is_default": True,
        }
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_create",
            {
                "type": "credit_card",
                "data": {"nonce": "payment-token"},
                "is_default": True,
                "confirm": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["type"] == "credit_card"
    assert payload["is_default"] is True
    mock_client.create_account_payment_method.assert_awaited_once_with(
        "credit_card", {"nonce": "payment-token"}, True
    )


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_payment_method_create_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Payment method create rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_payment_method = AsyncMock()
    arguments: dict[str, object] = {
        "type": "credit_card",
        "data": {"nonce": "payment-token"},
        "is_default": True,
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_payment_method_create", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.create_account_payment_method.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("type", None, "type is required"),
        ("type", 123, "type must be a string"),
        ("type", "   ", "type is required"),
        ("type", "paypal", "type must be credit_card"),
        ("data", None, "data is required"),
        ("data", 123, "data must be an object"),
        ("data", "payment-token", "data must be an object"),
        ("is_default", None, "is_default must be a boolean"),
        ("is_default", "true", "is_default must be a boolean"),
        ("is_default", 1, "is_default must be a boolean"),
    ],
)
async def test_account_payment_method_create_validates_required_arguments(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Payment method create validates required arguments before client call."""
    mock_client = AsyncMock()
    mock_client.create_account_payment_method = AsyncMock()
    arguments: dict[str, object] = {
        "type": "credit_card",
        "data": {"nonce": "payment-token"},
        "is_default": True,
        "confirm": True,
    }
    if value is None:
        arguments.pop(field)
    else:
        arguments[field] = value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_payment_method_create", arguments)

    assert message in result[0].text
    mock_client.create_account_payment_method.assert_not_called()


async def test_account_payment_method_create_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Payment method create dry-run previews the request without creating."""
    mock_client = AsyncMock()
    mock_client.create_account_payment_method = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_create",
            {
                "type": "credit_card",
                "data": {"nonce": "payment-token"},
                "is_default": True,
                "confirm": False,
                "dry_run": True,
            },
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_payment_method_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/payment-methods",
        "body": {
            "type": "credit_card",
            "data": {"redacted": True},
            "is_default": True,
        },
    }
    assert "payment-token" not in result[0].text
    mock_client.create_account_payment_method.assert_not_called()


async def test_account_payment_method_create_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Payment method create reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.create_account_payment_method = AsyncMock(
        side_effect=NetworkError("CreateAccountPaymentMethod", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_payment_method_create",
            {
                "type": "credit_card",
                "data": {"nonce": "payment-token"},
                "is_default": True,
                "confirm": True,
            },
        )

    assert "CreateAccountPaymentMethod" in result[0].text
    mock_client.create_account_payment_method.assert_awaited_once()


async def test_account_promo_credit_add_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Promo credit add dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock(
        return_value={
            "description": "Promo credit",
            "summary": "$100 credit",
            "credit_remaining": "100.00",
        }
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_promo_credit_add",
            {"promo_code": "PROMO123", "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["summary"] == "$100 credit"
    mock_client.add_account_promo_credit.assert_awaited_once_with("PROMO123")


async def test_account_promo_credit_add_rejects_missing_confirm(
    sample_config: Config,
) -> None:
    """Promo credit add rejects omitted confirm before client call."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_promo_credit_add", {"promo_code": "PROMO123"}
        )

    assert "Set confirm=true" in result[0].text
    mock_client.add_account_promo_credit.assert_not_called()


@pytest.mark.parametrize("confirm_value", [False, "true", 1])
async def test_account_promo_credit_add_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """Promo credit add rejects non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock()
    arguments: dict[str, object] = {"promo_code": "PROMO123", "confirm": confirm_value}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_promo_credit_add", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.add_account_promo_credit.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"confirm": True}, "promo_code is required"),
        ({"promo_code": 123, "confirm": True}, "promo_code must be a string"),
        ({"promo_code": "   ", "confirm": True}, "promo_code is required"),
    ],
)
async def test_account_promo_credit_add_validates_required_arguments(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Promo credit add validates promo_code before client calls."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_promo_credit_add", arguments)

    assert message in result[0].text
    mock_client.add_account_promo_credit.assert_not_called()


async def test_account_promo_credit_add_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """Promo credit add dry-run previews the request without applying it."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_promo_credit_add",
            {"promo_code": "PROMO123", "confirm": True, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_promo_credit_add"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/promo-codes",
        "body": {"promo_code": "PROMO123"},
    }
    mock_client.add_account_promo_credit.assert_not_called()


async def test_account_promo_credit_add_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """Promo credit add reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.add_account_promo_credit = AsyncMock(
        side_effect=NetworkError("AddAccountPromoCredit", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_promo_credit_add",
            {"promo_code": "PROMO123", "confirm": True},
        )

    assert "AddAccountPromoCredit" in result[0].text
    mock_client.add_account_promo_credit.assert_awaited_once()


async def test_account_oauth_client_delete_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """OAuth client delete dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.delete_account_oauth_client = AsyncMock(
        return_value={"id": "client-123", "deleted": True}
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_delete",
            {"client_id": "client-123", "confirm": True},
        )

    assert json.loads(result[0].text) == {"id": "client-123", "deleted": True}
    mock_client.delete_account_oauth_client.assert_awaited_once_with("client-123")


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_oauth_client_delete_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """OAuth client delete rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.delete_account_oauth_client = AsyncMock()
    arguments: dict[str, object] = {"client_id": "client-123"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_oauth_client_delete", arguments)

    assert "Set confirm=true" in result[0].text
    mock_client.delete_account_oauth_client.assert_not_called()


@pytest.mark.parametrize(
    ("client_id", "message"),
    [
        (None, "client_id is required"),
        (123, "client_id must be a string"),
        ("   ", "client_id is required"),
        ("client/123", "client_id must contain only"),
        ("client?123", "client_id must contain only"),
        ("..", "client_id must contain only"),
    ],
)
async def test_account_oauth_client_delete_validates_client_id(
    sample_config: Config, client_id: object, message: str
) -> None:
    """OAuth client delete validates client_id before client calls."""
    mock_client = AsyncMock()
    mock_client.delete_account_oauth_client = AsyncMock()
    arguments: dict[str, object] = {"confirm": True}
    if client_id is not None:
        arguments["client_id"] = client_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_oauth_client_delete", arguments)

    assert message in result[0].text
    mock_client.delete_account_oauth_client.assert_not_called()


async def test_account_oauth_client_delete_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """OAuth client delete dry-run previews the request without deleting."""
    mock_client = AsyncMock()
    mock_client.delete_account_oauth_client = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_delete",
            {"client_id": "client-123", "confirm": False, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_oauth_client_delete"
    assert payload["would_execute"] == {
        "method": "DELETE",
        "path": "/account/oauth-clients/client-123",
    }
    mock_client.delete_account_oauth_client.assert_not_called()


async def test_account_oauth_client_delete_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """OAuth client delete reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.delete_account_oauth_client = AsyncMock(
        side_effect=NetworkError("DeleteAccountOAuthClient", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_delete",
            {"client_id": "client-123", "confirm": True},
        )

    assert "DeleteAccountOAuthClient" in result[0].text
    mock_client.delete_account_oauth_client.assert_awaited_once_with("client-123")


async def test_account_oauth_client_reset_secret_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """OAuth client secret reset tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_oauth_client_reset_secret_tool" in tools_mod.__all__
    assert "handle_linode_account_oauth_client_reset_secret" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_oauth_client_reset_secret_tool()
    assert tool.name == "linode_account_oauth_client_reset_secret"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["client_id"]["type"] == "string"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "client_id" in tool.inputSchema["required"]
    assert "confirm" in tool.inputSchema["required"]

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_oauth_client_reset_secret" in srv.registered_tool_names


async def test_account_oauth_client_reset_secret_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """OAuth client secret reset dispatches through the registered handler."""
    mock_client = AsyncMock()
    mock_client.reset_account_oauth_client_secret = AsyncMock(
        return_value={"id": "client-123", "secret": "shown-once"}
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_reset_secret",
            {"client_id": "client-123", "confirm": True},
        )

    payload = json.loads(result[0].text)
    assert payload["secret"] == "shown-once"
    mock_client.reset_account_oauth_client_secret.assert_awaited_once_with("client-123")


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_oauth_client_reset_secret_requires_boolean_confirm(
    sample_config: Config, confirm_value: object
) -> None:
    """OAuth client secret reset rejects missing/non-true confirm before client call."""
    mock_client = AsyncMock()
    mock_client.reset_account_oauth_client_secret = AsyncMock()
    arguments: dict[str, object] = {"client_id": "client-123"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_reset_secret", arguments
        )

    assert "Set confirm=true" in result[0].text
    mock_client.reset_account_oauth_client_secret.assert_not_called()


@pytest.mark.parametrize(
    ("client_id", "message"),
    [
        (None, "client_id is required"),
        (123, "client_id must be a string"),
        ("   ", "client_id is required"),
        ("client/123", "client_id must not contain"),
        ("client?123", "client_id must not contain"),
        ("..", "client_id must not contain"),
    ],
)
async def test_account_oauth_client_reset_secret_validates_client_id(
    sample_config: Config, client_id: object, message: str
) -> None:
    """OAuth client secret reset validates client_id before client call."""
    mock_client = AsyncMock()
    mock_client.reset_account_oauth_client_secret = AsyncMock()
    arguments: dict[str, object] = {"confirm": True}
    if client_id is not None:
        arguments["client_id"] = client_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_reset_secret", arguments
        )

    assert message in result[0].text
    mock_client.reset_account_oauth_client_secret.assert_not_called()


async def test_account_oauth_client_reset_secret_dry_run_skips_client_call(
    sample_config: Config,
) -> None:
    """OAuth client secret reset dry-run previews the request without resetting."""
    mock_client = AsyncMock()
    mock_client.reset_account_oauth_client_secret = AsyncMock()

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_reset_secret",
            {"client_id": "client#123", "confirm": False, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_oauth_client_reset_secret"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/oauth-clients/client%23123/reset-secret",
    }
    mock_client.reset_account_oauth_client_secret.assert_not_called()


async def test_account_oauth_client_reset_secret_tool_propagates_client_error(
    sample_config: Config,
) -> None:
    """OAuth client secret reset reports client errors from dispatch."""
    mock_client = AsyncMock()
    mock_client.reset_account_oauth_client_secret = AsyncMock(
        side_effect=NetworkError("ResetAccountOAuthClientSecret", Exception("boom"))
    )
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_oauth_client_reset_secret",
            {"client_id": "client-123", "confirm": True},
        )

    assert "ResetAccountOAuthClientSecret" in result[0].text
    mock_client.reset_account_oauth_client_secret.assert_awaited_once_with("client-123")


async def test_account_cancel_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account cancel tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_cancel_tool" in tools_mod.__all__
    assert "handle_linode_account_cancel" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_cancel" in srv.registered_tool_names

    tool = next(
        item.tool
        for item in get_tool_registry()
        if item.name == "linode_account_cancel"
    )
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert "confirm" in tool.inputSchema.get("required", [])


async def test_account_cancel_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Account cancel is callable through server dispatch."""
    response_data = {"survey_link": "https://example.com/survey"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.cancel_account.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_cancel",
            {"comments": "No longer needed", "confirm": True},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.cancel_account.assert_awaited_once_with(comments="No longer needed")


@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_account_cancel_requires_explicit_boolean_confirm(
    sample_config: Config, confirm_value: Any
) -> None:
    """Account cancel rejects missing or non-true confirm before client calls."""
    arguments: dict[str, Any] = {"comments": "No longer needed"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_account_cancel", arguments)

    if confirm_value is None:
        assert "confirm" in result[0].text
    else:
        assert "Set confirm=true to proceed" in result[0].text
    mock_client.cancel_account.assert_not_called()


async def test_account_cancel_dry_run_skips_client_call(sample_config: Config) -> None:
    """Account cancel dry-run previews without requiring confirm."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_cancel",
            {"comments": "No longer needed", "confirm": False, "dry_run": True},
        )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_cancel"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/account/cancel",
        "body": {"comments": "No longer needed"},
    }
    assert len(payload["side_effects"]) == 1
    assert len(payload["warnings"]) == 1
    assert "irreversible" in payload["warnings"][0]
    mock_client.cancel_account.assert_not_called()


async def test_account_cancel_rejects_non_string_comments(
    sample_config: Config,
) -> None:
    """Account cancel validates optional comments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_cancel", {"comments": 123, "confirm": True}
        )

    assert "comments must be a string" in result[0].text
    mock_client.cancel_account.assert_not_called()


async def test_account_support_ticket_close_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket close tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_close_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_close" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_support_ticket_close" in srv.registered_tool_names


async def test_account_support_ticket_close_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support ticket close is callable through server dispatch."""
    response_data = {"id": 123, "status": "closed"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.close_support_ticket.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_account_support_ticket_close",
            {"ticket_id": 123, "confirm": True},
        )

    assert json.loads(result[0].text) == {
        "message": "Support ticket closed successfully",
        "ticket": response_data,
    }
    mock_client.close_support_ticket.assert_awaited_once_with(123)


async def test_account_support_ticket_reply_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket reply create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_support_ticket_reply_create_tool" in tools_mod.__all__
    assert "handle_linode_account_support_ticket_reply_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_account_support_ticket_reply_create" in srv.registered_tool_names


async def test_account_support_ticket_attachment_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Support ticket attachment create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_account_support_ticket_attachment_create_tool"
        in tools_mod.__all__
    )
    assert "handle_linode_account_support_ticket_attachment_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert (
        "linode_account_support_ticket_attachment_create" in srv.registered_tool_names
    )


async def test_account_support_ticket_attachment_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Support ticket attachment create is callable through server dispatch."""
    srv = Server(_full_access_config(sample_config))
    response_data: dict[str, Any] = {"id": 789, "file": "attachment.txt"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_support_ticket_attachment.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await srv.dispatch(
            "linode_account_support_ticket_attachment_create",
            {"ticket_id": 123, "file": "/Users/e/a.txt", "confirm": True},
        )

    assert len(result) == 1
    assert json.loads(result[0].text) == {
        "message": "Support ticket attachment created successfully",
        "attachment": response_data,
    }
    mock_client.create_support_ticket_attachment.assert_awaited_once_with(
        123, "/Users/e/a.txt"
    )


async def test_default_profile_filters_to_read_only(sample_config: Config) -> None:
    """Server with no active_profile registers only Read+Meta tools.

    The default built-in's allow list is strictly smaller than the full
    registry, and it must NOT include obvious mutators like
    ``linode_instance_create``. A Read tool (``linode_instances_list``)
    confirms the filter is letting through the right side.
    """
    full_registry = get_tool_registry()
    srv = Server(sample_config)

    assert srv.active_profile.name == "default"
    assert srv.registered_tool_names, "default profile must register some tools"
    assert len(srv.registered_tool_names) < len(full_registry), (
        "default profile should filter the registry, not pass it through"
    )
    assert "linode_instances_list" in srv.registered_tool_names
    assert "linode_instance_create" not in srv.registered_tool_names
    assert "hello" in srv.registered_tool_names


async def test_full_access_profile_registers_every_tool(
    sample_config: Config,
) -> None:
    """Full-access (with override enabling it) registers the entire registry."""
    full_registry = get_tool_registry()
    cfg = _full_access_config(sample_config)
    srv = Server(cfg)

    assert srv.active_profile.name == "full-access"
    assert len(srv.registered_tool_names) == len(full_registry)
    expected_names = {entry.name for entry in full_registry}
    assert srv.registered_tool_names == expected_names


async def test_disabled_builtin_profile_raises(sample_config: Config) -> None:
    """Selecting a disabled built-in raises at server construction."""
    cfg = dataclasses.replace(
        sample_config,
        active_profile="compute-admin",
        profiles_builtin_overrides={
            "compute-admin": BuiltinOverride(disabled=True),
        },
    )

    with pytest.raises(ActiveProfileDisabledError, match="compute-admin"):
        Server(cfg)


async def test_unknown_active_profile_raises(sample_config: Config) -> None:
    """A typo in active_profile raises rather than silently falling back."""
    cfg = dataclasses.replace(sample_config, active_profile="does-not-exist")

    with pytest.raises(ActiveProfileUnknownError, match="does-not-exist"):
        Server(cfg)


async def test_user_defined_profile_registers_listed_tools_only(
    sample_config: Config,
) -> None:
    """A user-defined profile's allow list maps one-to-one to registered names.

    Picks two known Read tools to keep the assertion independent of any
    capability tag changes; the profile filter is name-based by spec.
    """
    cfg = dataclasses.replace(
        sample_config,
        active_profile="minimal",
        profiles={
            "minimal": UserProfileConfig(
                description="just two read tools for the filter test",
                allowed_tools=("linode_instances_list", "linode_account"),
            ),
        },
    )
    srv = Server(cfg)

    assert srv.active_profile.name == "minimal"
    assert srv.registered_tool_names == {
        "linode_instances_list",
        "linode_account",
    }
    # Mutators outside the allow list must not slip through.
    assert "linode_instance_create" not in srv.registered_tool_names


async def test_instance_ip_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Instance IP update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_instance_ip_update_tool" in tools_mod.__all__
    assert "handle_linode_instance_ip_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_instance_ip_update" in srv.registered_tool_names


# Phase 5: reload_profile tests. Each one exercises a path the hot-reload
# code is responsible for: success (state swaps), error (state preserved),
# and convergence (repeated reloads don't accumulate leftover tools).
async def test_reload_profile_swaps_allowed_set(sample_config: Config) -> None:
    """Reloading from default to full-access adds the writes; back removes."""
    srv = Server(sample_config)

    before = set(srv.registered_tool_names)
    assert srv.active_profile.name == "default"
    assert "linode_instance_create" not in before, "default starts without write tools"

    await srv.reload_profile(_full_access_config(sample_config))

    after = set(srv.registered_tool_names)
    assert srv.active_profile.name == "full-access"
    assert "linode_instance_create" in after, (
        "reload to full-access must add the write tools"
    )
    assert "linode_instances_list" in after, "reads stay registered"
    assert after > before, "full-access is a strict superset of default"

    # Reload back to default and confirm the writes come off.
    await srv.reload_profile(sample_config)
    back = set(srv.registered_tool_names)
    assert srv.active_profile.name == "default"
    assert "linode_instance_create" not in back, (
        "reload back to default removes write tools"
    )
    assert back == before, "default↔full-access round-trip must be reversible"


async def test_reload_profile_dispatch_gate_updates(sample_config: Config) -> None:
    """The dispatch gate honors the post-reload allow set.

    A call to a tool that was permitted under the previous profile but
    not the new one must raise ``ValueError``. The new profile's tools
    become invocable on the same server instance.
    """
    srv = Server(_full_access_config(sample_config))

    # full-access allows linode_instances_list AND linode_instance_create.
    # The default profile drops linode_instance_create; after reload,
    # dispatching it must raise.
    await srv.reload_profile(sample_config)

    with pytest.raises(ValueError, match="Unknown tool"):
        await srv.dispatch("linode_instance_create", {})


async def test_reload_profile_disabled_builtin_is_no_op(
    sample_config: Config,
) -> None:
    """A failed reload (disabled built-in) must not mutate state."""
    srv = Server(sample_config)
    before_profile = srv.active_profile.name
    before_tools = set(srv.registered_tool_names)

    bad = dataclasses.replace(
        sample_config,
        active_profile="compute-admin",
        profiles_builtin_overrides={
            "compute-admin": BuiltinOverride(disabled=True),
        },
    )

    with pytest.raises(ActiveProfileDisabledError):
        await srv.reload_profile(bad)

    assert srv.active_profile.name == before_profile
    assert set(srv.registered_tool_names) == before_tools


async def test_reload_profile_unknown_is_no_op(sample_config: Config) -> None:
    """A failed reload (unknown profile name) must not mutate state."""
    srv = Server(sample_config)
    before_profile = srv.active_profile.name
    before_tools = set(srv.registered_tool_names)

    bad = dataclasses.replace(sample_config, active_profile="not-a-real-profile")

    with pytest.raises(ActiveProfileUnknownError):
        await srv.reload_profile(bad)

    assert srv.active_profile.name == before_profile
    assert set(srv.registered_tool_names) == before_tools


async def test_reload_profile_repeated_cycles_converge(
    sample_config: Config,
) -> None:
    """Repeated A→B→A cycles must end at the final profile with no leftover.

    Guards against accumulation bugs where the swap path forgets to clear
    state between reloads, ending in a state that's neither A nor B.
    """
    srv = Server(sample_config)
    full = _full_access_config(sample_config)

    for _ in range(3):
        await srv.reload_profile(full)
        await srv.reload_profile(sample_config)

    await srv.reload_profile(full)

    fresh = Server(full)
    assert srv.active_profile.name == "full-access"
    assert set(srv.registered_tool_names) == set(fresh.registered_tool_names)


def test_linode_images_sharegroups_token_create_registered() -> None:
    """Image share group token create tool should be registered from exports."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}
    assert "linode_images_sharegroups_token_create" in entries
    assert entries["linode_images_sharegroups_token_create"].capability.name == "Write"


def test_linode_images_sharegroups_token_update_registered() -> None:
    """Image share group token update tool should be registered from exports."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}
    assert "linode_images_sharegroups_token_update" in entries
    assert entries["linode_images_sharegroups_token_update"].capability.name == "Write"


def test_linode_image_create_registered() -> None:
    """Image create tool should be registered from tools exports."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}
    assert "linode_image_create" in entries
    assert entries["linode_image_create"].capability.name == "Write"


async def test_validate_scopes_no_token_raises_sentinel(
    sample_config: Config,
) -> None:
    """Phase 6.4c: empty token surfaces as TokenNotConfiguredError.

    The validator never makes an API call when the token is absent.
    main.py uses this signal to decide policy by profile elevation.
    """
    from linodemcp.config import EnvironmentConfig, LinodeConfig
    from linodemcp.profiles import TokenNotConfiguredError

    cfg = dataclasses.replace(
        sample_config,
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(api_url="https://example.invalid", token=""),
            ),
        },
    )
    srv = Server(cfg)

    with pytest.raises(TokenNotConfiguredError):
        await srv.validate_scopes()


async def test_profile_is_elevated_reflects_required_scopes(
    sample_config: Config,
) -> None:
    """Phase 6.4c policy helper: ``:read_write`` scopes mark elevation.

    Default profile carries only :read_only scopes and must not be
    elevated; full-access carries write scopes and must be. main.py
    uses this to decide whether missing-token is fail vs warn.
    """
    from linodemcp.profiles import profile_is_elevated

    default_srv = Server(sample_config)
    assert not profile_is_elevated(default_srv.active_profile), (
        "default profile is read-only and must not be classified elevated"
    )

    full_srv = Server(_full_access_config(sample_config))
    assert profile_is_elevated(full_srv.active_profile), (
        "full-access carries :read_write scopes and must be classified elevated"
    )


async def test_linode_regions_availability_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Regions availability list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_regions_availability_list_tool" in tools_mod.__all__
    assert "handle_linode_regions_availability_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_regions_availability_list" in srv.registered_tool_names


async def test_linode_regions_availability_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Region availability tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_regions_availability_get_tool" in tools_mod.__all__
    assert "handle_linode_regions_availability_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_regions_availability_get" in srv.registered_tool_names


async def test_profile_security_questions_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile security questions list tool is exported and registered."""
    import linodemcp.tools as tools_mod

    assert "create_linode_profile_security_questions_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_security_questions_list" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_profile_security_questions_list" in srv.registered_tool_names


async def test_profile_security_questions_answer_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile security questions tool is exported and registered."""
    import linodemcp.tools as tools_mod

    assert "create_linode_profile_security_questions_answer_tool" in tools_mod.__all__
    assert "handle_linode_profile_security_questions_answer" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_security_questions_answer" in srv.registered_tool_names


async def test_profile_tfa_enable_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile TFA enable tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_tfa_enable_tool" in tools_mod.__all__
    assert "handle_linode_profile_tfa_enable" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_tfa_enable" in srv.registered_tool_names


async def test_profile_tfa_disable_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile TFA disable tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_tfa_disable_tool" in tools_mod.__all__
    assert "handle_linode_profile_tfa_disable" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_tfa_disable" in srv.registered_tool_names


async def test_profile_tfa_enable_confirm_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile TFA enable confirm tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_tfa_enable_confirm_tool" in tools_mod.__all__
    assert "handle_linode_profile_tfa_enable_confirm" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_tfa_enable_confirm" in srv.registered_tool_names


async def test_profile_phone_number_send_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile phone number send tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_phone_number_send_tool" in tools_mod.__all__
    assert "handle_linode_profile_phone_number_send" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_phone_number_send" in srv.registered_tool_names


async def test_profile_phone_number_delete_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile phone number delete tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_phone_number_delete_tool" in tools_mod.__all__
    assert "handle_linode_profile_phone_number_delete" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_phone_number_delete" in srv.registered_tool_names


async def test_profile_phone_number_verify_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile phone number verify tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_phone_number_verify_tool" in tools_mod.__all__
    assert "handle_linode_profile_phone_number_verify" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_phone_number_verify" in srv.registered_tool_names


async def test_profile_token_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile token create tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_token_create_tool" in tools_mod.__all__
    assert "handle_linode_profile_token_create" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_token_create" in srv.registered_tool_names


async def test_profile_tokens_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile token list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_tokens_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_tokens_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_tokens_list" in srv.registered_tool_names


async def test_profile_token_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile token get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_token_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_token_get" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_token_get" in srv.registered_tool_names


async def test_profile_logins_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile login list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_logins_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_logins_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_logins_list" in srv.registered_tool_names


async def test_profile_login_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile login get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_login_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_login_get" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_login_get" in srv.registered_tool_names


async def test_profile_token_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile token update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_token_update_tool" in tools_mod.__all__
    assert "handle_linode_profile_token_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_token_update" in srv.registered_tool_names


async def test_profile_token_revoke_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Profile token revoke tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_profile_token_revoke_tool" in tools_mod.__all__
    assert "handle_linode_profile_token_revoke" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert srv.active_profile.name == "full-access"
    assert "linode_profile_token_revoke" in srv.registered_tool_names


async def test_profile_devices_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    import linodemcp.tools as tools_mod

    srv = Server(_full_access_config(sample_config))
    registry_names = {entry.name for entry in get_tool_registry()}

    assert srv.active_profile.name == "full-access"

    assert "create_linode_profile_devices_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_devices_list" in tools_mod.__all__
    assert "linode_profile_devices_list" in registry_names
    assert "linode_profile_devices_list" in srv.registered_tool_names


async def test_profile_apps_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    import linodemcp.tools as tools_mod

    srv = Server(_full_access_config(sample_config))
    registry_names = {entry.name for entry in get_tool_registry()}

    assert srv.active_profile.name == "full-access"

    assert "create_linode_profile_apps_list_tool" in tools_mod.__all__
    assert "handle_linode_profile_apps_list" in tools_mod.__all__
    assert "linode_profile_apps_list" in registry_names
    assert "linode_profile_apps_list" in srv.registered_tool_names


async def test_profile_app_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    import linodemcp.tools as tools_mod

    srv = Server(_full_access_config(sample_config))
    registry_names = {entry.name for entry in get_tool_registry()}

    assert srv.active_profile.name == "full-access"

    assert "create_linode_profile_app_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_app_get" in tools_mod.__all__
    assert "linode_profile_app_get" in registry_names
    assert "linode_profile_app_get" in srv.registered_tool_names


async def test_profile_device_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    import linodemcp.tools as tools_mod

    srv = Server(_full_access_config(sample_config))
    registry_names = {entry.name for entry in get_tool_registry()}

    assert srv.active_profile.name == "full-access"

    assert "create_linode_profile_device_get_tool" in tools_mod.__all__
    assert "handle_linode_profile_device_get" in tools_mod.__all__
    assert "linode_profile_device_get" in registry_names
    assert "linode_profile_device_get" in srv.registered_tool_names


async def test_profile_device_revoke_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    import linodemcp.tools as tools_mod

    srv = Server(_full_access_config(sample_config))
    registry_names = {entry.name for entry in get_tool_registry()}

    assert srv.active_profile.name == "full-access"

    assert "create_linode_profile_device_revoke_tool" in tools_mod.__all__
    assert "handle_linode_profile_device_revoke" in tools_mod.__all__
    assert "linode_profile_device_revoke" in registry_names
    assert "linode_profile_device_revoke" in srv.registered_tool_names


def test_linode_nodebalancer_vpc_configs_list_exported() -> None:
    """NodeBalancer VPC configs list tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_vpc_configs_list_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_vpc_configs_list" in tools_mod.__all__


def test_linode_nodebalancer_vpc_configs_list_registered() -> None:
    """NodeBalancer VPC configs list tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_vpc_configs_list" in entries
    assert entries["linode_nodebalancer_vpc_configs_list"].capability == Capability.Read


def test_linode_nodebalancer_vpc_config_get_exported() -> None:
    """NodeBalancer VPC config get tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_vpc_config_get_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_vpc_config_get" in tools_mod.__all__


def test_linode_nodebalancer_vpc_config_get_registered() -> None:
    """NodeBalancer VPC config get tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_vpc_config_get" in entries
    assert entries["linode_nodebalancer_vpc_config_get"].capability == Capability.Read


def test_linode_nodebalancer_firewalls_update_exported() -> None:
    """NodeBalancer firewall update tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_firewalls_update_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_firewalls_update" in tools_mod.__all__


def test_linode_nodebalancer_firewalls_update_registered() -> None:
    """NodeBalancer firewall update tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_firewalls_update" in entries
    assert (
        entries["linode_nodebalancer_firewalls_update"].capability == Capability.Write
    )


def test_linode_nodebalancer_config_rebuild_exported() -> None:
    """NodeBalancer config rebuild tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_rebuild_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_rebuild" in tools_mod.__all__


def test_linode_nodebalancer_config_rebuild_registered() -> None:
    """NodeBalancer config rebuild tool is registered."""
    from linodemcp.server import get_tool_registry

    # Tool registration is discovered dynamically from linodemcp.tools.__all__.
    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_rebuild" in entries
    assert entries["linode_nodebalancer_config_rebuild"].capability == Capability.Write


def test_linode_nodebalancer_config_delete_exported() -> None:
    """NodeBalancer config delete tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_delete_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_delete" in tools_mod.__all__


def test_linode_nodebalancer_config_delete_registered() -> None:
    """NodeBalancer config delete tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_delete" in entries
    assert entries["linode_nodebalancer_config_delete"].capability == Capability.Destroy


def test_linode_nodebalancer_config_node_create_exported() -> None:
    """NodeBalancer config node create tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_node_create_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_node_create" in tools_mod.__all__


def test_linode_nodebalancer_config_node_create_registered() -> None:
    """NodeBalancer config node create tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_node_create" in entries
    entry = entries["linode_nodebalancer_config_node_create"]
    assert entry.capability == Capability.Write


def test_linode_nodebalancer_config_node_update_exported() -> None:
    """NodeBalancer config node update tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_node_update_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_node_update" in tools_mod.__all__


def test_linode_nodebalancer_config_node_update_registered() -> None:
    """NodeBalancer config node update tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_node_update" in entries
    entry = entries["linode_nodebalancer_config_node_update"]
    assert entry.capability == Capability.Write


def test_linode_nodebalancer_config_node_delete_exported() -> None:
    """NodeBalancer config node delete tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_node_delete_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_node_delete" in tools_mod.__all__


def test_linode_nodebalancer_config_node_delete_registered() -> None:
    """NodeBalancer config node delete tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_node_delete" in entries
    entry = entries["linode_nodebalancer_config_node_delete"]
    assert entry.capability == Capability.Destroy


def test_linode_nodebalancer_config_get_exported() -> None:
    """NodeBalancer config get tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_get_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_get" in tools_mod.__all__


def test_linode_nodebalancer_config_get_registered() -> None:
    """NodeBalancer config get tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_get" in entries
    entry = entries["linode_nodebalancer_config_get"]
    assert entry.capability == Capability.Read


def test_linode_nodebalancer_configs_list_exported() -> None:
    """NodeBalancer configs list tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_configs_list_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_configs_list" in tools_mod.__all__


def test_linode_nodebalancer_configs_list_registered() -> None:
    """NodeBalancer configs list tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_configs_list" in entries
    entry = entries["linode_nodebalancer_configs_list"]
    assert entry.capability == Capability.Read


def test_linode_nodebalancer_config_node_get_exported() -> None:
    """NodeBalancer config node get tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_node_get_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_node_get" in tools_mod.__all__


def test_linode_nodebalancer_config_node_get_registered() -> None:
    """NodeBalancer config node get tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_node_get" in entries
    entry = entries["linode_nodebalancer_config_node_get"]
    assert entry.capability == Capability.Read


def test_linode_nodebalancer_stats_exported() -> None:
    """NodeBalancer stats tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_stats_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_stats" in tools_mod.__all__


def test_linode_nodebalancer_stats_registered() -> None:
    """NodeBalancer stats tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_stats" in entries
    assert entries["linode_nodebalancer_stats"].capability == Capability.Read


def test_linode_nodebalancer_config_update_exported() -> None:
    """NodeBalancer config update tool is exported."""
    import linodemcp.tools as tools_mod

    assert "create_linode_nodebalancer_config_update_tool" in tools_mod.__all__
    assert "handle_linode_nodebalancer_config_update" in tools_mod.__all__


def test_linode_nodebalancer_config_update_registered() -> None:
    """NodeBalancer config update tool is registered."""
    from linodemcp.server import get_tool_registry

    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_nodebalancer_config_update" in entries
    assert entries["linode_nodebalancer_config_update"].capability == Capability.Write


async def test_monitor_services_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor services list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_services_list_tool" in tools_mod.__all__
    assert "handle_linode_monitor_services_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_services_list_tool()
    assert tool.name == "linode_monitor_services_list"
    assert capability is Capability.Read

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_monitor_services_list"].capability is Capability.Read

    srv = Server(_full_access_config(sample_config))
    assert "linode_monitor_services_list" in srv.registered_tool_names


async def test_monitor_services_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor services list dispatch should call the retryable client."""
    srv = Server(_full_access_config(sample_config))
    response_data = {
        "data": [{"label": "Databases", "service_type": "dbaas"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_services.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await srv.dispatch("linode_monitor_services_list", {})

    payload = json.loads(result[0].text)
    assert payload["services"][0]["service_type"] == "dbaas"
    assert payload["count"] == 1
    mock_client.list_monitor_services.assert_awaited_once_with()


async def test_monitor_service_alert_definition_create_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor alert definition create tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_monitor_service_alert_definition_create_tool"
        in tools_mod.__all__
    )
    assert "handle_linode_monitor_service_alert_definition_create" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_monitor_service_alert_definition_create_tool()
    )
    assert tool.name == "linode_monitor_service_alert_definition_create"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_monitor_service_alert_definition_create"].capability
        is Capability.Write
    )

    srv = Server(_full_access_config(sample_config))
    assert "linode_monitor_service_alert_definition_create" in srv.registered_tool_names


async def test_monitor_service_alert_definition_create_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor alert definition create dispatches through the registered tool."""
    response_data = {"id": 67890, "label": "CPU high"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_monitor_service_alert_definition.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_monitor_service_alert_definition_create",
            {
                "service_type": "linode",
                "label": "CPU high",
                "severity": 1,
                "rule_criteria": {"rules": [{"metric": "cpu_usage"}]},
                "trigger_conditions": {"criteria_condition": "ALL"},
                "channel_ids": [10000],
                "confirm": True,
            },
        )

    result_json = json.loads(result[0].text)
    assert result_json["service_type"] == "linode"
    assert result_json["alert_definition"] == response_data
    mock_client.create_monitor_service_alert_definition.assert_awaited_once_with(
        "linode",
        label="CPU high",
        severity=1,
        rule_criteria={"rules": [{"metric": "cpu_usage"}]},
        trigger_conditions={"criteria_condition": "ALL"},
        channel_ids=[10000],
        description=None,
        entity_ids=None,
    )


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"service_type": ""},
        {"service_type": "linode/v4", "confirm": True},
        {"service_type": "linode?x=1", "confirm": True},
        {"service_type": "..", "confirm": True},
        {"service_type": "linode", "confirm": False},
        {"service_type": "linode", "confirm": "true"},
        {"service_type": "linode", "confirm": 1},
        {"service_type": "linode", "label": "", "confirm": True},
        {
            "service_type": "linode",
            "label": "CPU high",
            "severity": True,
            "confirm": True,
        },
    ],
)
async def test_monitor_service_alert_definition_create_rejects_invalid_args(
    sample_config: Config, arguments: dict[str, Any]
) -> None:
    """Monitor alert definition create rejects malformed args before client use."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_monitor_service_alert_definition_create", arguments
        )

    assert "error" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_monitor_service_alert_definition_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor alert definition get tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_monitor_service_alert_definition_get_tool" in tools_mod.__all__
    )
    assert "handle_linode_monitor_service_alert_definition_get" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_monitor_service_alert_definition_get_tool()
    )
    assert tool.name == "linode_monitor_service_alert_definition_get"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_monitor_service_alert_definition_get"].capability
        is Capability.Read
    )

    srv = Server(sample_config)
    assert "linode_monitor_service_alert_definition_get" in srv.registered_tool_names


async def test_monitor_service_alert_definition_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor alert definition get dispatches through the registered tool."""
    response_data = {"id": 12345, "label": "CPU high"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_monitor_service_alert_definition.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_alert_definition_get",
            {"service_type": "linode", "alert_id": 12345},
        )

    result_json = json.loads(result[0].text)
    assert result_json["service_type"] == "linode"
    assert result_json["alert_id"] == 12345
    assert result_json["alert_definition"] == response_data
    mock_client.get_monitor_service_alert_definition.assert_awaited_once_with(
        "linode", 12345
    )


@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"service_type": ""},
        {"service_type": "linode/v4", "alert_id": 12345},
        {"service_type": "linode?x=1", "alert_id": 12345},
        {"service_type": "..", "alert_id": 12345},
        {"service_type": "linode", "alert_id": "12345"},
        {"service_type": "linode", "alert_id": "1/2"},
        {"service_type": "linode", "alert_id": "1?x"},
        {"service_type": "linode", "alert_id": ".."},
        {"service_type": "linode", "alert_id": True},
        {"service_type": "linode", "alert_id": 0},
    ],
)
async def test_monitor_service_alert_definition_get_rejects_invalid_path_params(
    sample_config: Config, arguments: dict[str, Any]
) -> None:
    """Monitor alert definition get rejects malformed path params."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_alert_definition_get", arguments
        )

    assert "error" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_monitor_alert_channels_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor alert channels list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_alert_channels_list_tool" in tools_mod.__all__
    assert "handle_linode_monitor_alert_channels_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_alert_channels_list_tool()
    assert tool.name == "linode_monitor_alert_channels_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_monitor_alert_channels_list"].capability is Capability.Read

    srv = Server(sample_config)
    assert "linode_monitor_alert_channels_list" in srv.registered_tool_names


async def test_monitor_alert_channels_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor alert channels list dispatches through the registered tool."""
    response_data = {"data": [{"id": 10000, "label": "Email Ops"}]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_alert_channels.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_monitor_alert_channels_list", {})

    result_json = json.loads(result[0].text)
    assert result_json["alert_channels"] == response_data["data"]
    assert result_json["count"] == 1
    mock_client.list_monitor_alert_channels.assert_awaited_once_with()


async def test_monitor_alert_definitions_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor alert definitions list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_alert_definitions_list_tool" in tools_mod.__all__
    assert "handle_linode_monitor_alert_definitions_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_alert_definitions_list_tool()
    assert tool.name == "linode_monitor_alert_definitions_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_monitor_alert_definitions_list"].capability is Capability.Read
    )

    srv = Server(sample_config)
    assert "linode_monitor_alert_definitions_list" in srv.registered_tool_names


async def test_monitor_alert_definitions_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor alert definitions list dispatches through the registered tool."""
    response_data = {"data": [{"id": 123, "label": "CPU Usage"}]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_alert_definitions.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_monitor_alert_definitions_list", {})

    result_json = json.loads(result[0].text)
    assert result_json["alert_definitions"] == response_data["data"]
    assert result_json["count"] == 1
    mock_client.list_monitor_alert_definitions.assert_awaited_once_with()


async def test_monitor_service_alert_definitions_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor alert definitions list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_monitor_service_alert_definitions_list_tool" in tools_mod.__all__
    )
    assert "handle_linode_monitor_service_alert_definitions_list" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_monitor_service_alert_definitions_list_tool()
    )
    assert tool.name == "linode_monitor_service_alert_definitions_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_monitor_service_alert_definitions_list"].capability
        is Capability.Read
    )

    srv = Server(sample_config)
    assert "linode_monitor_service_alert_definitions_list" in srv.registered_tool_names


async def test_monitor_service_alert_definitions_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor alert definitions list dispatches through the registered tool."""
    response_data = {
        "data": [{"id": 123, "label": "CPU Usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_service_alert_definitions.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_alert_definitions_list",
            {"service_type": "linode"},
        )

    result_json = json.loads(result[0].text)
    assert result_json["service_type"] == "linode"
    assert result_json["alert_definitions"] == response_data
    mock_client.list_monitor_service_alert_definitions.assert_awaited_once_with(
        "linode"
    )


@pytest.mark.parametrize(
    "service_type", [None, "", "   ", 123, "linode/v4", "linode?x=1", ".."]
)
async def test_monitor_service_alert_definitions_list_rejects_invalid_service_type(
    sample_config: Config, service_type: Any
) -> None:
    """Monitor alert definitions list rejects malformed path params."""
    arguments: dict[str, Any] = {}
    if service_type is not None:
        arguments["service_type"] = service_type

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_alert_definitions_list", arguments
        )

    assert "service_type" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_monitor_dashboards_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor dashboards list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_dashboards_list_tool" in tools_mod.__all__
    assert "handle_linode_monitor_dashboards_list" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_dashboards_list_tool()
    assert tool.name == "linode_monitor_dashboards_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_monitor_dashboards_list"].capability is Capability.Read

    srv = Server(sample_config)
    assert "linode_monitor_dashboards_list" in srv.registered_tool_names


async def test_monitor_dashboards_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor dashboards list dispatches through the registered tool."""
    response_data = {"data": [{"id": 1, "label": "Resource Usage"}]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_dashboards.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch("linode_monitor_dashboards_list", {})

    result_json = json.loads(result[0].text)
    assert result_json["dashboards"] == response_data["data"]
    assert result_json["count"] == 1
    mock_client.list_monitor_dashboards.assert_awaited_once_with()


async def test_monitor_dashboard_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor dashboard get tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_dashboard_get_tool" in tools_mod.__all__
    assert "handle_linode_monitor_dashboard_get" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_dashboard_get_tool()
    assert tool.name == "linode_monitor_dashboard_get"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_monitor_dashboard_get"].capability is Capability.Read

    srv = Server(sample_config)
    assert "linode_monitor_dashboard_get" in srv.registered_tool_names


async def test_monitor_dashboard_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor dashboard get dispatches through the registered tool."""
    response_data = {"id": 12345, "label": "Resource Usage"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_monitor_dashboard.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_dashboard_get", {"dashboard_id": 12345}
        )

    result_json = json.loads(result[0].text)
    assert result_json["dashboard_id"] == 12345
    assert result_json["dashboard"] == response_data
    mock_client.get_monitor_dashboard.assert_awaited_once_with(12345)


@pytest.mark.parametrize(
    "dashboard_id", [None, True, "12345", "1/2", "1?x", "..", 12.9, 0, -1]
)
async def test_monitor_dashboard_get_rejects_invalid_dashboard_id(
    sample_config: Config, dashboard_id: Any
) -> None:
    """Monitor dashboard get rejects malformed path params."""
    arguments: dict[str, Any] = {}
    if dashboard_id is not None:
        arguments["dashboard_id"] = dashboard_id

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_monitor_dashboard_get", arguments)

    assert "dashboard_id" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_monitor_service_metric_definitions_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor metric definitions list tool is exported and registered."""
    from linodemcp import tools as tools_mod

    assert (
        "create_linode_monitor_service_metric_definitions_list_tool"
        in tools_mod.__all__
    )
    assert "handle_linode_monitor_service_metric_definitions_list" in tools_mod.__all__

    tool, capability = (
        tools_mod.create_linode_monitor_service_metric_definitions_list_tool()
    )
    assert tool.name == "linode_monitor_service_metric_definitions_list"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert (
        registry["linode_monitor_service_metric_definitions_list"].capability
        is Capability.Read
    )

    srv = Server(sample_config)
    assert "linode_monitor_service_metric_definitions_list" in srv.registered_tool_names


async def test_monitor_service_metric_definitions_list_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor metric definitions list dispatches through the registered tool."""
    response_data = {
        "data": [{"label": "CPU Usage", "metric": "cpu_usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_monitor_service_metric_definitions.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_metric_definitions_list",
            {"service_type": "linode"},
        )

    result_json = json.loads(result[0].text)
    assert result_json["service_type"] == "linode"
    assert result_json["metric_definitions"] == response_data
    mock_client.list_monitor_service_metric_definitions.assert_awaited_once_with(
        "linode"
    )


@pytest.mark.parametrize(
    "service_type", [None, "", "   ", 123, "linode/v4", "linode?x=1", ".."]
)
async def test_monitor_service_metric_definitions_list_rejects_invalid_service_type(
    sample_config: Config, service_type: Any
) -> None:
    """Monitor metric definitions list rejects malformed path params."""
    arguments: dict[str, Any] = {}
    if service_type is not None:
        arguments["service_type"] = service_type

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_metric_definitions_list", arguments
        )

    assert "service_type" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_monitor_service_metrics_read_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monitor service metrics read tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_monitor_service_metrics_read_tool" in tools_mod.__all__
    assert "handle_linode_monitor_service_metrics_read" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_monitor_service_metrics_read_tool()
    assert tool.name == "linode_monitor_service_metrics_read"
    assert capability is Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_monitor_service_metrics_read"].capability is Capability.Read

    srv = Server(sample_config)
    assert "linode_monitor_service_metrics_read" in srv.registered_tool_names


async def test_monitor_service_metrics_read_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Monitor service metrics read dispatches through the registered tool."""
    response_data = {"data": [{"label": "cpu", "value": 1.0}]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.read_monitor_service_metrics.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_monitor_service_metrics_read",
            {"service_type": "linode"},
        )

    result_json = json.loads(result[0].text)
    assert result_json["service_type"] == "linode"
    assert result_json["metrics"] == response_data
    mock_client.read_monitor_service_metrics.assert_awaited_once_with("linode")


@pytest.mark.parametrize(
    "service_type", [None, "", "   ", 123, "linode/v4", "linode?x=1", ".."]
)
async def test_monitor_service_metrics_read_rejects_invalid_service_type(
    sample_config: Config, service_type: Any
) -> None:
    """Monitor metrics read rejects malformed path params before client creation."""
    arguments: dict[str, Any] = {}
    if service_type is not None:
        arguments["service_type"] = service_type

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_monitor_service_metrics_read", arguments)

    assert "service_type" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_ipv4_assign_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """IPv4 assign tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_ipv4_assign_tool" in tools_mod.__all__
    assert "handle_linode_ipv4_assign" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_ipv4_assign" in srv.registered_tool_names


async def test_ipv4_assign_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """IPv4 assign is callable through server dispatch with confirm=true."""
    assignments = [{"address": "192.0.2.1", "linode_id": 123}]

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.assign_ipv4s.return_value = {}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "confirm": True,
                "region": "us-east",
                "assignments": assignments,
            },
        )

    result_json = json.loads(result[0].text)
    assert result_json["region"] == "us-east"
    assert result_json["assignments"] == assignments
    mock_client.assign_ipv4s.assert_awaited_once_with("us-east", assignments)


async def test_ipv4_assign_rejects_missing_confirm_before_client(
    sample_config: Config,
) -> None:
    """IPv4 assign should reject missing confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "region": "us-east",
                "assignments": [{"address": "192.0.2.1", "linode_id": 123}],
            },
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_ipv4_assign_rejects_false_confirm_before_client(
    sample_config: Config,
) -> None:
    """IPv4 assign should reject false confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "confirm": False,
                "region": "us-east",
                "assignments": [{"address": "192.0.2.1", "linode_id": 123}],
            },
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_ipv4_assign_rejects_string_confirm_before_client(
    sample_config: Config,
) -> None:
    """IPv4 assign should reject string confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "confirm": "true",
                "region": "us-east",
                "assignments": [{"address": "192.0.2.1", "linode_id": 123}],
            },
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_ipv4_assign_rejects_numeric_confirm_before_client(
    sample_config: Config,
) -> None:
    """IPv4 assign should reject numeric confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "confirm": 1,
                "region": "us-east",
                "assignments": [{"address": "192.0.2.1", "linode_id": 123}],
            },
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("region", [None, "", "   ", 123])
async def test_ipv4_assign_rejects_invalid_region(
    sample_config: Config, region: Any
) -> None:
    """IPv4 assign should reject missing or invalid region."""
    arguments: dict[str, Any] = {
        "confirm": True,
        "assignments": [{"address": "192.0.2.1", "linode_id": 123}],
    }
    if region is not None:
        arguments["region"] = region

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_ipv4_assign", arguments)

    assert "region" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("assignments", "expected"),
    [
        ("192.0.2.1", "assignments"),
        ([], "assignments"),
        (["192.0.2.1"], "assignments"),
        ([{"linode_id": 123}], "address"),
        ([{"address": "", "linode_id": 123}], "address"),
        ([{"address": "192.0.2.1"}], "linode_id"),
        ([{"address": "192.0.2.1", "linode_id": "123"}], "linode_id"),
    ],
)
async def test_ipv4_assign_rejects_invalid_assignments(
    sample_config: Config, assignments: Any, expected: str
) -> None:
    """IPv4 assign should reject malformed assignments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_assign",
            {
                "confirm": True,
                "region": "us-east",
                "assignments": assignments,
            },
        )

    assert expected in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_ipv4_share_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """IPv4 share tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_ipv4_share_tool" in tools_mod.__all__
    assert "handle_linode_ipv4_share" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_ipv4_share" in srv.registered_tool_names


async def test_ipv4_share_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """IPv4 share is callable through server dispatch with confirm=true."""
    response_data = {"success": True, "shared": ["192.168.1.1"]}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.share_ipv4s.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_ipv4_share",
            {
                "confirm": True,
                "ips": ["192.168.1.1"],
                "linode_id": 12345,
            },
        )

    result_json = json.loads(result[0].text)
    assert result_json["linode_id"] == 12345
    assert result_json["ips"] == ["192.168.1.1"]
    mock_client.share_ipv4s.assert_awaited_once_with(["192.168.1.1"], 12345)


async def test_ipv4_share_rejects_missing_confirm(
    sample_config: Config,
) -> None:
    """IPv4 share should reject calls without confirm=true."""
    srv = Server(_full_access_config(sample_config))
    result = await srv.dispatch(
        "linode_ipv4_share",
        {
            "ips": ["192.168.1.1"],
            "linode_id": 12345,
        },
    )
    text = result[0].text
    assert "confirm" in text.lower()


async def test_ipv4_share_rejects_false_confirm(
    sample_config: Config,
) -> None:
    """IPv4 share should reject calls with confirm=false."""
    srv = Server(_full_access_config(sample_config))
    result = await srv.dispatch(
        "linode_ipv4_share",
        {
            "confirm": False,
            "ips": ["192.168.1.1"],
            "linode_id": 12345,
        },
    )
    text = result[0].text
    assert "confirm" in text.lower()


async def test_ipv4_share_rejects_missing_ips(
    sample_config: Config,
) -> None:
    """IPv4 share should reject calls without ips."""
    srv = Server(_full_access_config(sample_config))
    result = await srv.dispatch(
        "linode_ipv4_share",
        {
            "confirm": True,
            "linode_id": 12345,
        },
    )
    text = result[0].text
    assert "ips" in text.lower()


async def test_ipv4_share_rejects_missing_linode_id(
    sample_config: Config,
) -> None:
    """IPv4 share should reject calls without linode_id."""
    srv = Server(_full_access_config(sample_config))
    result = await srv.dispatch(
        "linode_ipv4_share",
        {
            "confirm": True,
            "ips": ["192.168.1.1"],
        },
    )
    text = result[0].text
    assert "linode_id" in text.lower()


async def test_networking_ip_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Networking IP update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_networking_ip_update_tool" in tools_mod.__all__
    assert "handle_linode_networking_ip_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_networking_ip_update" in srv.registered_tool_names


async def test_networking_ip_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Networking IP get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_networking_ip_get_tool" in tools_mod.__all__
    assert "handle_linode_networking_ip_get" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_networking_ip_get" in srv.registered_tool_names


async def test_networking_ip_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Networking IP get is callable through server dispatch."""
    response_data = {"address": "198.51.100.5", "rdns": "example.example.com"}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_networking_ip.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_get", {"address": "198.51.100.5"}
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_networking_ip.assert_awaited_once_with("198.51.100.5")


@pytest.mark.parametrize(
    "address",
    ["", 123, "198.51.100.5/32", "198.51.100.5?x=1", ".."],
)
async def test_networking_ip_get_rejects_malformed_address_before_client(
    sample_config: Config, address: Any
) -> None:
    """Networking IP get rejects malformed address values before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_networking_ip_get", {"address": address})

    assert "address" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "address",
    ["", 123, "198.51.100.5/32", "198.51.100.5?x=1", ".."],
)
async def test_networking_ip_update_rejects_malformed_address_before_client(
    sample_config: Config, address: Any
) -> None:
    """Networking IP update rejects malformed address before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_update",
            {"address": address, "rdns": None, "confirm": True},
        )

    assert "address" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("tool_name", "arguments"),
    [
        ("linode_instance_ip_get", {"instance_id": "123", "address": "../bad"}),
        (
            "linode_instance_ip_update",
            {
                "instance_id": "123",
                "address": "198.51.100.5/32",
                "rdns": None,
                "confirm": True,
            },
        ),
        (
            "linode_instance_ip_delete",
            {
                "instance_id": "123",
                "address": "198.51.100.5?x=1",
                "confirm": True,
            },
        ),
    ],
)
async def test_instance_ip_tools_reject_malformed_address_before_client(
    sample_config: Config, tool_name: str, arguments: dict[str, Any]
) -> None:
    """Instance IP tools reject malformed addresses before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(tool_name, arguments)

    assert "address" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_networking_ips_list_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Networking IPs list tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_networking_ips_list_tool" in tools_mod.__all__
    assert "handle_linode_networking_ips_list" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_networking_ips_list" in srv.registered_tool_names


async def test_networking_ips_list_dispatches_happy_path(
    sample_config: Config,
) -> None:
    """Networking IPs list calls client with skip_ipv6_rdns."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_networking_ips.return_value = [
            {"address": "198.51.100.5", "type": "ipv4"}
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ips_list",
            {"skip_ipv6_rdns": True},
        )

    mock_client.list_networking_ips.assert_awaited_once_with(skip_ipv6_rdns=True)
    result_json = json.loads(result[0].text)
    assert result_json["count"] == 1
    assert result_json["ips"][0]["address"] == "198.51.100.5"


async def test_networking_ips_list_returns_error_response_on_client_failure(
    sample_config: Config,
) -> None:
    """Networking IPs list returns a tool error response when the client fails."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_networking_ips.side_effect = NetworkError(
            "ListNetworkingIPs", RuntimeError("timeout")
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch("linode_networking_ips_list", {})

    assert "failed to list networking ips" in result[0].text.lower()
    assert "ListNetworkingIPs" in result[0].text


@pytest.mark.parametrize("skip_ipv6_rdns", ["true", 1, None])
async def test_networking_ips_list_rejects_invalid_skip_ipv6_rdns_before_client(
    sample_config: Config, skip_ipv6_rdns: Any
) -> None:
    """Networking IPs list rejects non-boolean skip_ipv6_rdns values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ips_list",
            {"skip_ipv6_rdns": skip_ipv6_rdns},
        )

    assert "skip_ipv6_rdns" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_networking_ip_allocate_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Networking IP allocate tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_networking_ip_allocate_tool" in tools_mod.__all__
    assert "handle_linode_networking_ip_allocate" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_networking_ip_allocate" in srv.registered_tool_names


async def test_networking_ip_allocate_rejects_missing_confirm_before_client(
    sample_config: Config,
) -> None:
    """Networking IP allocate should reject missing confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {"linode_id": 12345, "type": "ipv4"},
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_networking_ip_allocate_rejects_false_confirm_before_client(
    sample_config: Config,
) -> None:
    """Networking IP allocate should reject false confirm before client creation."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {"linode_id": 12345, "type": "ipv4", "confirm": False},
        )

    assert "confirm" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_networking_ip_allocate_dispatches_happy_path(
    sample_config: Config,
) -> None:
    """Networking IP allocate calls client with validated arguments."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.allocate_networking_ip.return_value = {
            "address": "198.51.100.10",
            "linode_id": 12345,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {
                "linode_id": 12345,
                "type": "ipv4",
                "public": True,
                "confirm": True,
            },
        )

    mock_client.allocate_networking_ip.assert_awaited_once_with(
        12345, ip_type="ipv4", public=True
    )
    result_json = json.loads(result[0].text)
    assert result_json["address"] == "198.51.100.10"


@pytest.mark.parametrize(
    "linode_id",
    [
        (True,),
        ("123",),
        (None,),
    ],
)
async def test_networking_ip_allocate_rejects_invalid_linode_id(
    sample_config: Config, linode_id: Any
) -> None:
    """Networking IP allocate rejects invalid linode_id values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {
                "linode_id": linode_id,
                "type": "ipv4",
                "confirm": True,
            },
        )

    assert "linode_id" in result[0].text.lower()
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    "ip_type",
    [
        ("ipv5",),
        (123,),
        ("",),
    ],
)
async def test_networking_ip_allocate_rejects_invalid_type(
    sample_config: Config, ip_type: Any
) -> None:
    """Networking IP allocate rejects invalid type values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {
                "linode_id": 12345,
                "type": ip_type,
                "confirm": True,
            },
        )

    assert "type" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_networking_ip_allocate_rejects_non_bool_public(
    sample_config: Config,
) -> None:
    """Networking IP allocate rejects non-boolean public values."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(_full_access_config(sample_config))
        result = await srv.dispatch(
            "linode_networking_ip_allocate",
            {
                "linode_id": 12345,
                "type": "ipv4",
                "public": "yes",
                "confirm": True,
            },
        )

    assert "public" in result[0].text.lower()
    mock_client_class.assert_not_called()


async def test_firewall_device_delete_tool_schema_requires_confirm() -> None:
    """Firewall device delete schema requires explicit confirmation."""
    from linodemcp.tools.linode_firewalls_write import (
        create_linode_firewall_device_delete_tool,
    )

    tool, capability = create_linode_firewall_device_delete_tool()

    assert tool.name == "linode_firewall_device_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["required"] == [
        "firewall_id",
        "device_id",
        "confirm",
    ]


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_firewall_device_delete_rejects_missing_or_non_bool_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Confirm guard rejects missing, false, string, and numeric values."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    arguments: dict[str, Any] = {"firewall_id": 12345, "device_id": 456}
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch(
        "linodemcp.tools.linode_firewalls_write.execute_tool", new_callable=AsyncMock
    ) as mock_execute:
        result = await handle_linode_firewall_device_delete(arguments, sample_config)

    mock_execute.assert_not_awaited()
    assert "Error:" in result[0].text
    assert "confirm=true" in result[0].text


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("firewall_id", 0, "firewall_id must be a positive integer"),
        ("device_id", -1, "device_id must be a positive integer"),
        ("firewall_id", True, "firewall_id must be a valid integer"),
        ("firewall_id", "123/../?x=1", "firewall_id must be a valid integer"),
        ("device_id", "456/../?x=1", "device_id must be a valid integer"),
        ("device_id", "..", "device_id must be a valid integer"),
    ],
)
async def test_firewall_device_delete_rejects_invalid_path_params_before_client_call(
    sample_config: Config, field: str, value: object, message: str
) -> None:
    """Invalid firewall device delete path params fail before client calls."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    arguments: dict[str, Any] = {
        "firewall_id": 12345,
        "device_id": 456,
        "confirm": True,
    }
    arguments[field] = value

    with patch(
        "linodemcp.tools.linode_firewalls_write.execute_tool", new_callable=AsyncMock
    ) as mock_execute:
        result = await handle_linode_firewall_device_delete(arguments, sample_config)

    mock_execute.assert_not_awaited()
    assert message in result[0].text


async def test_firewall_device_delete_handler_success(sample_config: Config) -> None:
    """Handler calls the client and reports deleted IDs."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    captured_call: Any = None

    async def fake_execute_tool(
        _cfg: Config,
        _arguments: dict[str, Any],
        _action: str,
        call: Any,
    ) -> list[Any]:
        nonlocal captured_call
        client = AsyncMock()
        captured_call = client.delete_firewall_device
        payload = await call(client)
        return [payload]

    with patch(
        "linodemcp.tools.linode_firewalls_write.execute_tool",
        side_effect=fake_execute_tool,
    ):
        result = cast(
            "list[Any]",
            await handle_linode_firewall_device_delete(
                {"firewall_id": 12345, "device_id": 456, "confirm": True},
                sample_config,
            ),
        )

    assert result == [
        {
            "message": "Firewall device deleted successfully",
            "firewall_id": 12345,
            "device_id": 456,
        }
    ]
    assert captured_call is not None
    captured_call.assert_awaited_once_with(12345, 456)


async def test_firewall_device_delete_dry_run_returns_preview_without_mutating(
    sample_config: Config,
) -> None:
    """dry_run=true must fetch state via GET and never call delete."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall_device.return_value = {
            "id": 456,
            "entity": {"id": 789, "type": "linode", "label": "web-01"},
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_device_delete(
            {"firewall_id": 123, "device_id": 456, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        body = json.loads(result[0].text)
        assert body["dry_run"] is True
        assert body["tool"] == "linode_firewall_device_delete"
        assert body["would_execute"]["method"] == "DELETE"
        assert body["would_execute"]["path"] == "/networking/firewalls/123/devices/456"
        mock_client.get_firewall_device.assert_awaited_once_with(123, 456)
        mock_client.delete_firewall_device.assert_not_called()


async def test_firewall_device_delete_dry_run_does_not_require_confirm(
    sample_config: Config,
) -> None:
    """dry_run path must bypass the confirm gate."""
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_firewall_device.return_value = {"id": 456}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_firewall_device_delete(
            {"firewall_id": 123, "device_id": 456, "dry_run": True},
            sample_config,
        )

        assert len(result) == 1
        assert "confirm=true" not in result[0].text


async def test_firewall_device_delete_dry_run_still_rejects_negative_ids(
    sample_config: Config,
) -> None:
    """Pre-validation guard (positive-int) fires on dry-run too.

    Catches a regression where the dry-run branch bypasses the
    handler's pre-validation guard for negative IDs.
    """
    from linodemcp.tools.linode_firewalls_write import (
        handle_linode_firewall_device_delete,
    )

    result = await handle_linode_firewall_device_delete(
        {"firewall_id": -1, "device_id": 456, "dry_run": True},
        sample_config,
    )

    assert len(result) == 1
    assert "firewall_id must be a positive integer" in result[0].text


async def test_account_user_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Account user update tool is exported and server-registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_account_user_update_tool" in tools_mod.__all__
    assert "handle_linode_account_user_update" in tools_mod.__all__

    tool, capability = tools_mod.create_linode_account_user_update_tool()
    assert tool.name == "linode_account_user_update"
    assert capability is Capability.Write
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "confirm" in tool.inputSchema["required"]

    cfg = dataclasses.replace(
        sample_config,
        active_profile="account-user-write",
        profiles={
            "account-user-write": UserProfileConfig(
                description="account user write",
                allowed_tools=("linode_account_user_update",),
            ),
        },
    )
    srv = Server(cfg)
    assert "linode_account_user_update" in srv.registered_tool_names

    list_tools = srv.mcp.request_handlers[ListToolsRequest]
    result = await list_tools(ListToolsRequest(method="tools/list"))
    list_result = cast("ListToolsResult", result.root)
    assert "linode_account_user_update" in {tool.name for tool in list_result.tools}


async def test_account_user_update_handler_updates_user(
    sample_config: Config,
) -> None:
    """Account user update handler returns the updated user."""
    from linodemcp.tools import handle_linode_account_user_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account_user = AsyncMock(
            return_value={
                "username": "new-user",
                "email": "new@example.com",
            }
        )
        mock_context = AsyncMock()
        mock_context.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_context

        result = await handle_linode_account_user_update(
            {
                "current_username": "old-user",
                "username": "new-user",
                "email": "new@example.com",
                "restricted": False,
                "ssh_keys": ["ssh-rsa AAA"],
                "confirm": True,
            },
            sample_config,
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Account user updated successfully"
    assert payload["user"]["username"] == "new-user"
    mock_client.update_account_user.assert_awaited_once_with(
        "old-user",
        username="new-user",
        email="new@example.com",
        restricted=False,
        ssh_keys=["ssh-rsa AAA"],
    )


async def test_account_user_update_dry_run_encodes_username(
    sample_config: Config,
) -> None:
    """Dry run previews the encoded account user update path."""
    from linodemcp.tools import handle_linode_account_user_update

    result = await handle_linode_account_user_update(
        {
            "current_username": "old-user",
            "email": "new@example.com",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_account_user_update"
    assert payload["would_execute"]["method"] == "PUT"
    assert payload["would_execute"]["path"] == "/account/users/old-user"
    assert payload["would_execute"]["body"] == {"email": "new@example.com"}


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_account_user_update_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Live account user updates require explicit boolean confirm."""
    from linodemcp.tools import handle_linode_account_user_update

    arguments: dict[str, object] = {
        "current_username": "old-user",
        "email": "new@example.com",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_context = AsyncMock()
        mock_context.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_context
        result = await handle_linode_account_user_update(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client.update_account_user.assert_not_called()


@pytest.mark.parametrize(
    "current_username",
    [
        "bad/user",
        "bad?user",
        "bad#user",
        "bad..user",
        " bad ",
        "bad user",
        "bad\nuser",
    ],
)
async def test_account_user_update_rejects_invalid_current_username(
    sample_config: Config, current_username: str
) -> None:
    """Malformed account usernames are rejected before client calls."""
    from linodemcp.tools import handle_linode_account_user_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client
        result = await handle_linode_account_user_update(
            {
                "current_username": current_username,
                "email": "new@example.com",
                "confirm": True,
            },
            sample_config,
        )

    assert "current_username must contain only" in result[0].text
    mock_client.update_account_user.assert_not_called()


@pytest.mark.parametrize(
    "current_username",
    [
        "bad/user",
        "bad?user",
        "bad#user",
        "bad..user",
        " bad ",
        "bad user",
        "bad\nuser",
    ],
)
async def test_account_user_update_dry_run_rejects_invalid_current_username(
    sample_config: Config, current_username: str
) -> None:
    """Dry-run rejects malformed account usernames before preview construction."""
    from linodemcp.tools import handle_linode_account_user_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client_class.return_value = mock_client
        result = await handle_linode_account_user_update(
            {
                "current_username": current_username,
                "email": "new@example.com",
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    assert "current_username must contain only" in result[0].text
    mock_client.update_account_user.assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"current_username": "old-user", "confirm": True},
            "At least one account user field",
        ),
        (
            {"current_username": "old-user", "confirm": True, "unknown": "x"},
            "Unsupported account user field",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "email": False,
            },
            "email must be a string",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "username": 123,
            },
            "username must be a string",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "restricted": "false",
            },
            "restricted must be a boolean",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "ssh_keys": "ssh-rsa AAA",
            },
            "ssh_keys must be a list of strings",
        ),
        (
            {"current_username": "old-user", "confirm": True, "ssh_keys": [1]},
            "ssh_keys must be a list of strings",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "last_login": "2025-01-01T00:00:00",
            },
            "Unsupported account user field",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "password_created": "2025-01-01T00:00:00",
            },
            "Unsupported account user field",
        ),
        (
            {"current_username": "old-user", "confirm": True, "tfa_enabled": True},
            "Unsupported account user field",
        ),
        (
            {
                "current_username": "old-user",
                "confirm": True,
                "verified_phone_number": "+15551234567",
            },
            "Unsupported account user field",
        ),
    ],
)
async def test_account_user_update_rejects_invalid_fields(
    sample_config: Config, arguments: dict[str, object], message: str
) -> None:
    """Account user update validates route body fields before client calls."""
    from linodemcp.tools import handle_linode_account_user_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_context = AsyncMock()
        mock_context.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_context
        result = await handle_linode_account_user_update(arguments, sample_config)

    assert message in result[0].text
    mock_client.update_account_user.assert_not_called()


async def test_account_user_update_client_error_is_reported(
    sample_config: Config,
) -> None:
    """Account user update handler reports client failures."""
    from linodemcp.tools import handle_linode_account_user_update

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_account_user = AsyncMock(
            side_effect=NetworkError("UpdateAccountUser", Exception("boom"))
        )
        mock_context = AsyncMock()
        mock_context.__aenter__.return_value = mock_client
        mock_client_class.return_value = mock_context

        result = await handle_linode_account_user_update(
            {
                "current_username": "old-user",
                "email": "new@example.com",
                "confirm": True,
            },
            sample_config,
        )

    assert "Failed to update Linode account user" in result[0].text


async def test_database_engine_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Database engine get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_database_engine_get_tool" in tools_mod.__all__
    assert "handle_linode_database_engine_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_database_engine_get" in srv.registered_tool_names

    entry = next(
        item
        for item in get_tool_registry()
        if item.name == "linode_database_engine_get"
    )
    assert entry.tool.inputSchema["required"] == ["engine_id"]
    assert entry.tool.inputSchema["properties"]["engine_id"]["type"] == "string"
    assert entry.tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert entry.tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert entry.tool.inputSchema["properties"]["page_size"]["maximum"] == 500


async def test_database_engine_get_dispatches_from_registry(
    sample_config: Config,
) -> None:
    """Database engine get is callable through server dispatch."""
    response_data: dict[str, object] = {
        "id": "mysql/8.0.26",
        "engine": "mysql",
        "version": "8.0.26",
    }

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_database_engine.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        srv = Server(sample_config)
        result = await srv.dispatch(
            "linode_database_engine_get",
            {"engine_id": "mysql/8.0.26", "page": 2, "page_size": 25},
        )

    assert json.loads(result[0].text) == response_data
    mock_client.get_database_engine.assert_awaited_once_with(
        "mysql/8.0.26", page=2, page_size=25
    )


@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        ({}, "engine_id is required"),
        ({"engine_id": 123}, "engine_id must be a string"),
        ({"engine_id": ""}, "engine_id is required"),
        (
            {"engine_id": " mysql/8.0.26"},
            "engine_id must not include leading or trailing whitespace",
        ),
        ({"engine_id": "mysql?8.0.26"}, "engine_id must not contain"),
        ({"engine_id": "mysql#8.0.26"}, "engine_id must not contain"),
        ({"engine_id": ".."}, "engine_id must not contain"),
        ({"engine_id": "mysql"}, "engine_id must use the documented"),
        ({"engine_id": "/mysql/8.0.26"}, "engine_id must use the documented"),
        ({"engine_id": "mysql//8.0.26"}, "engine_id must use the documented"),
        ({"engine_id": "mysql/8.0.26/extra"}, "engine_id must use the documented"),
        ({"engine_id": "mysql/8.0.26$"}, "engine_id must use the documented"),
        (
            {"engine_id": "mysql/8.0.26", "page": "2"},
            "page must be an integer",
        ),
        (
            {"engine_id": "mysql/8.0.26", "page": 0},
            "page must be at least 1",
        ),
        (
            {"engine_id": "mysql/8.0.26", "page_size": 24},
            "page_size must be at least 25",
        ),
        (
            {"engine_id": "mysql/8.0.26", "page_size": 501},
            "page_size must be at most 500",
        ),
        (
            {"engine_id": "mysql/8.0.26", "page_size": True},
            "page_size must be an integer",
        ),
    ],
)
async def test_database_engine_get_rejects_invalid_arguments(
    sample_config: Config, arguments: dict[str, object], expected_error: str
) -> None:
    """Database engine get rejects invalid arguments before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        srv = Server(sample_config)
        result = await srv.dispatch("linode_database_engine_get", arguments)

    assert expected_error in result[0].text
    mock_client_class.assert_not_called()
