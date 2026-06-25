"""Tests for Managed service update route tooling."""

import dataclasses
import json
from typing import Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.config import BuiltinOverride, Config
from linodemcp.profiles import Capability
from linodemcp.server import Server
from linodemcp.tools.linode_account import (
    create_linode_managed_service_update_tool,
    handle_linode_managed_service_update,
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


def test_create_linode_managed_service_update_tool() -> None:
    """Test linode_managed_service_update tool schema."""
    tool, capability = create_linode_managed_service_update_tool()
    assert tool.name == "linode_managed_service_update"
    assert capability is Capability.Admin
    assert tool.inputSchema["required"] == ["service_id", "confirm"]
    properties = tool.inputSchema["properties"]
    assert properties["service_id"]["minimum"] == 1
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    assert properties["service_type"]["enum"] == ["url", "tcp"]
    assert properties["timeout"]["minimum"] == 1
    assert properties["timeout"]["maximum"] == 255
    assert properties["credentials"]["items"]["minimum"] == 1
    assert "id" not in properties
    assert "created" not in properties
    assert "status" not in properties
    assert "updated" not in properties


async def test_handle_linode_managed_service_update(sample_config: Config) -> None:
    """Test linode_managed_service_update tool."""
    response_data: dict[str, Any] = {
        "id": 429,
        "label": "web monitor",
        "service_type": "url",
        "timeout": 30,
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_managed_service.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client
        result = await handle_linode_managed_service_update(
            {
                "service_id": 429,
                "address": "https://example.com/health?check=1",
                "body": "ok",
                "consultation_group": "ops",
                "credentials": [9991],
                "label": "web monitor",
                "notes": None,
                "region": "us-east",
                "service_type": "url",
                "timeout": 30,
                "confirm": True,
            },
            sample_config,
        )
    assert json.loads(result[0].text) == response_data
    mock_client.update_managed_service.assert_awaited_once_with(
        429,
        address="https://example.com/health?check=1",
        body="ok",
        consultation_group="ops",
        credentials=[9991],
        label="web monitor",
        notes=None,
        region="us-east",
        service_type="url",
        timeout=30,
    )


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_managed_service_update_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Managed service update requires explicit boolean confirmation."""
    arguments: dict[str, object] = {"service_id": 429, "label": "web monitor"}
    if confirm is not None:
        arguments["confirm"] = confirm
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(arguments, sample_config)
    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("service_id", [0, -1, True, "/", "1?", ".."])
async def test_handle_linode_managed_service_update_rejects_invalid_service_id(
    sample_config: Config, service_id: object
) -> None:
    """Managed service update validates service_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(
            {"service_id": service_id, "label": "web monitor", "confirm": True},
            sample_config,
        )
    assert "service_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "expected"),
    [
        ("address", "", "address must be a non-empty string"),
        ("address", 123, "address must be a non-empty string"),
        ("consultation_group", "", "consultation_group must be a non-empty string"),
        ("label", "", "label must be a non-empty string"),
        ("body", 123, "body must be a string or null"),
        ("notes", 123, "notes must be a string or null"),
        ("region", 123, "region must be a string or null"),
        ("service_type", "udp", "service_type must be one of: tcp, url"),
        ("timeout", 0, "timeout must be an integer from 1 to 255"),
        ("timeout", 256, "timeout must be an integer from 1 to 255"),
        ("timeout", True, "timeout must be an integer from 1 to 255"),
        ("credentials", "9991", "credentials must be an array of positive integers"),
        ("credentials", [0], "credentials must be an array of positive integers"),
        ("credentials", [True], "credentials must be an array of positive integers"),
    ],
)
async def test_handle_linode_managed_service_update_rejects_invalid_fields(
    sample_config: Config, field: str, value: object, expected: str
) -> None:
    """Managed service update validates local inputs before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(
            {"service_id": 429, field: value, "confirm": True}, sample_config
        )
    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_update_rejects_read_only_fields(
    sample_config: Config,
) -> None:
    """Managed service update does not forward read-only fields."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(
            {
                "service_id": 429,
                "id": 429,
                "created": "2024-01-01T00:00:00Z",
                "status": "ok",
                "updated": "2024-01-02T00:00:00Z",
                "confirm": True,
            },
            sample_config,
        )
    assert (
        "Read-only fields are not accepted: created, id, status, updated"
        in result[0].text
    )
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_update_requires_field(
    sample_config: Config,
) -> None:
    """Managed service update requires at least one writable field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(
            {"service_id": 429, "confirm": True}, sample_config
        )
    assert "At least one managed service field is required" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_service_update_dry_run(
    sample_config: Config,
) -> None:
    """Managed service update dry-run previews the request without calling client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_service_update(
            {
                "service_id": 429,
                "label": "web monitor",
                "notes": None,
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )
    data = json.loads(result[0].text)
    assert data["would_execute"]["method"] == "PUT"
    assert data["would_execute"]["path"] == "/managed/services/429"
    assert data["would_execute"]["body"] == {"label": "web monitor", "notes": None}
    mock_client_class.assert_not_called()


def test_linode_managed_service_update_is_registered_and_versioned(
    sample_config: Config,
) -> None:
    """Managed service update is registered and listed in version features."""
    srv = Server(_full_access_config(sample_config))
    assert "linode_managed_service_update" in srv.registered_tool_names
    assert "linode_managed_service_update" in get_version_info().features["tools"]
