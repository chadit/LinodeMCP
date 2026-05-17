"""Unit tests for MCP server dispatch."""

from __future__ import annotations

import asyncio
import dataclasses
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.config import BuiltinOverride, UserProfileConfig
from linodemcp.linode import Profile
from linodemcp.profiles import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
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
