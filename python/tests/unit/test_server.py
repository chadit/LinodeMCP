"""Unit tests for MCP server dispatch."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.linode import Profile
from linodemcp.server import Server
from linodemcp.tools import handle_hello, handle_version

if TYPE_CHECKING:
    from linodemcp.config import Config


async def test_server_construction_stores_config(sample_config: Config) -> None:
    """Server stores the config and creates an MCP server instance."""
    srv = Server(sample_config)
    assert srv.config is sample_config
    assert srv.mcp is not None


async def test_server_none_config_raises() -> None:
    """Passing None as config raises ValueError."""
    with pytest.raises(ValueError, match="config cannot be None"):
        Server(None)  # type: ignore[arg-type]


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

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
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

    tool_names = [fn().name for fn in create_funcs]

    # No duplicate tool names
    seen: set[str] = set()
    duplicates: set[str] = set()
    for name in tool_names:
        if name in seen:
            duplicates.add(name)
        seen.add(name)
    assert not duplicates, f"Duplicate tool names: {duplicates}"

    # Collect all handle_* functions from the tools module.
    handle_funcs = {
        name
        for name in dir(tools_mod)
        if name.startswith("handle_")
    }
    # "hello" and "version" handlers don't follow the linode_ pattern
    config_handles = {
        n
        for n in handle_funcs
        if n not in ("handle_hello", "handle_version")
    }

    # Direction 1: Every handler must have a matching tool.
    for handler_name in config_handles:
        tool_name = handler_name.replace("handle_", "", 1)
        assert tool_name in seen, (
            f"Handler {handler_name} has no matching tool"
            f" '{tool_name}'"
        )

    # Direction 2: Every tool (except hello/version) must have a handler.
    non_utility_tools = {
        t for t in seen if t not in ("hello", "version")
    }
    for tool_name in non_utility_tools:
        handler_name = f"handle_{tool_name}"
        assert handler_name in handle_funcs, (
            f"Tool '{tool_name}' has no matching handler"
            f" '{handler_name}'"
        )

    # The number of config-based tools must match the number of
    # config-based handlers exactly (no orphans on either side).
    assert len(non_utility_tools) == len(config_handles), (
        f"Tool/handler count mismatch: {len(non_utility_tools)} tools"
        f" vs {len(config_handles)} handlers"
    )
