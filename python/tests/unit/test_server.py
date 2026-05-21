"""Unit tests for MCP server dispatch."""

from __future__ import annotations

import asyncio
import dataclasses
import json
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.config import BuiltinOverride, UserProfileConfig
from linodemcp.linode import NetworkError, Profile
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


async def test_domain_record_get_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Domain record get tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_domain_record_get_tool" in tools_mod.__all__
    assert "handle_linode_domain_record_get" in tools_mod.__all__

    srv = Server(sample_config)
    assert "linode_domain_record_get" in srv.registered_tool_names


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


async def test_firewall_rules_update_tool_is_exported_and_registered(
    sample_config: Config,
) -> None:
    """Firewall rules update tool should be exported and registered."""
    from linodemcp import tools as tools_mod

    assert "create_linode_firewall_rules_update_tool" in tools_mod.__all__
    assert "handle_linode_firewall_rules_update" in tools_mod.__all__

    srv = Server(_full_access_config(sample_config))
    assert "linode_firewall_rules_update" in srv.registered_tool_names


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
