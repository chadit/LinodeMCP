"""Tests for the LKE tier version get route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    create_linode_lke_tier_version_get_tool,
    handle_linode_lke_tier_version_get,
)
from linodemcp.profiles import Capability
from linodemcp.server import Server, get_tool_registry

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


def test_linode_lke_tier_version_get_tool_schema() -> None:
    """Tool schema exposes tier and version path params."""
    tool, capability = create_linode_lke_tier_version_get_tool()

    assert tool.name == "linode_lke_tier_version_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["tier", "version"]
    assert tool.input_schema["properties"]["tier"]["type"] == "string"
    assert tool.input_schema["properties"]["version"]["type"] == "string"


async def test_handle_linode_lke_tier_version_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns LKE tier version details."""
    mock_linode_client.route_raw.return_value = {
        "id": "1.31",
        "tier": "standard",
    }

    result = await handle_linode_lke_tier_version_get(
        {"tier": "standard", "version": "1.31"}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {"id": "1.31", "tier": "standard"}
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_lke_tier_version_get", "standard", "1.31"
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "tier"),
        ({"tier": "", "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": ""}, "version"),
        ({"tier": "standard/x", "version": "1.31"}, "tier"),
        ({"tier": "standard?x=1", "version": "1.31"}, "tier"),
        ({"tier": "..", "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": "1/31"}, "version"),
        ({"tier": "standard", "version": "1.31?x=1"}, "version"),
        ({"tier": "standard", "version": ".."}, "version"),
        ({"tier": 123, "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": 1.31}, "version"),
    ],
)
async def test_handle_linode_lke_tier_version_get_rejects_invalid_path_params(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before client calls."""
    result = await handle_linode_lke_tier_version_get(arguments, sample_config)

    assert message in result[0].text
    mock_linode_client.route_raw.assert_not_called()


def test_linode_lke_tier_version_get_exported_and_registered(
    sample_config: Any,
) -> None:
    """Tier version get is exported and registered."""
    registry_names = {entry.name for entry in get_tool_registry()}
    assert "linode_lke_tier_version_get" in registry_names

    srv = Server(sample_config)
    assert "linode_lke_tier_version_get" in srv.registered_tool_names
