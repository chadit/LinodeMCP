"""Tests for Managed service enable route tooling."""

import dataclasses
import json
from typing import Any, cast
from unittest.mock import AsyncMock, patch

from linodemcp.config import BuiltinOverride, Config
from linodemcp.profiles import Capability
from linodemcp.server import Server
from linodemcp.tools.linode_account import (
    create_linode_managed_service_enable_tool,
    handle_linode_managed_service_enable,
)
from linodemcp.version import get_version_info


def _full_access_config(base: Config) -> Config:
    """Return a config that enables full-access built-in tools."""
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={
            "full-access": BuiltinOverride(disabled=False),
        },
    )


def _response_text(result: list[Any]) -> str:
    return cast("str", result[0].text)


def test_create_linode_managed_service_enable_tool() -> None:
    """Test linode_managed_service_enable tool schema."""
    tool, capability = create_linode_managed_service_enable_tool()
    assert tool.name == "linode_managed_service_enable"
    assert capability is Capability.Admin
    assert tool.inputSchema["required"] == ["service_id", "confirm"]
    properties = tool.inputSchema["properties"]
    assert properties["service_id"]["type"] == "integer"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"


async def test_handle_linode_managed_service_enable(sample_config: Config) -> None:
    """Test linode_managed_service_enable tool."""
    response_data: dict[str, Any] = {"id": 429, "label": "web monitor"}
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.enable_managed_service.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        result = await handle_linode_managed_service_enable(
            {"service_id": 429, "confirm": True},
            sample_config,
        )

    assert json.loads(_response_text(result)) == {
        "message": "Managed service enabled successfully",
        "service_id": 429,
    }
    mock_client.enable_managed_service.assert_awaited_once_with(429)


async def test_handle_linode_managed_service_enable_requires_boolean_confirm(
    sample_config: Config,
) -> None:
    """Missing, false, string, and numeric confirm values reject before client call."""
    for value in (None, False, "true", 1):
        arguments: dict[str, Any] = {"service_id": 429}
        if value is not None:
            arguments["confirm"] = value
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_managed_service_enable(
                arguments, sample_config
            )

        assert "confirm=true" in _response_text(result)
        mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_enable_rejects_invalid_service_id(
    sample_config: Config,
) -> None:
    """Malformed service IDs reject before client construction."""
    for value in (0, -1, True, "1/2", "1?x", ".."):
        with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
            result = await handle_linode_managed_service_enable(
                {"service_id": value, "confirm": True},
                sample_config,
            )

        assert "service_id must be a positive integer" in _response_text(result)
        mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_enable_dry_run(
    sample_config: Config,
) -> None:
    """dry_run=true previews the exact POST path and skips the client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_enable(
            {"service_id": 429, "confirm": True, "dry_run": True},
            sample_config,
        )

    payload = json.loads(_response_text(result))
    assert payload["tool"] == "linode_managed_service_enable"
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == "/managed/services/429/enable"
    assert "body" not in payload["would_execute"]
    mock_client_class.assert_not_called()


def test_linode_managed_service_enable_is_registered_and_versioned(
    sample_config: Config,
) -> None:
    """The enable tool is exported, registered, and listed in version metadata."""
    srv = Server(_full_access_config(sample_config))
    assert "linode_managed_service_enable" in srv.registered_tool_names
    assert "linode_managed_service_enable" in get_version_info().features["tools"]
