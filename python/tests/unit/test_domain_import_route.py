"""Tests for the domain import route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import httpx
import pytest

from linodemcp.gentools import (
    create_linode_domain_import_tool,
    handle_linode_domain_import,
)
from linodemcp.linode import NetworkError
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


def test_create_linode_domain_import_tool_schema() -> None:
    """Tool schema exposes required body fields plus confirm/dry_run."""
    tool, capability = create_linode_domain_import_tool()

    assert tool.name == "linode_domain_import"
    assert capability is Capability.Write
    assert tool.input_schema["required"] == [
        "domain",
        "remote_nameserver",
        "confirm",
    ]
    assert tool.input_schema["properties"]["domain"]["type"] == "string"
    assert tool.input_schema["properties"]["remote_nameserver"]["type"] == "string"
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_domain_import_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler imports the domain and returns the full proto domain element."""
    mock_linode_client.route_raw.return_value = {
        "id": 123,
        "domain": "example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
    }

    result = await handle_linode_domain_import(
        {
            "domain": "example.com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Domain 'example.com' (ID: 123) imported successfully"
    assert payload["domain"]["domain"] == "example.com"
    assert payload["domain"]["soa_email"] == "admin@example.com"
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_domain_import",
        body={"domain": "example.com", "remote_nameserver": "ns1.example.net"},
        retry=False,
    )


@pytest.mark.asyncio
async def test_handle_linode_domain_import_dry_run_requires_confirm_and_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry-run previews the documented request without importing."""
    result = await handle_linode_domain_import(
        {
            "domain": "example.com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/domains/import",
        "body": {"domain": "example.com", "remote_nameserver": "ns1.example.net"},
    }
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "confirm_value",
    [None, False, "true", 1],
)
async def test_handle_linode_domain_import_rejects_non_true_confirm(
    confirm_value: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Missing/false/string/numeric confirm is rejected before client calls."""
    arguments: dict[str, Any] = {
        "domain": "example.com",
        "remote_nameserver": "ns1.example.net",
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_domain_import(arguments, sample_config)

    assert result[0].text.startswith("Error: This imports a DNS domain")
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"remote_nameserver": "ns1.example.net", "confirm": True},
        {"domain": "", "remote_nameserver": "ns1.example.net", "confirm": True},
        {"domain": 123, "remote_nameserver": "ns1.example.net", "confirm": True},
        {
            "domain": " example.com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        {
            "domain": "example.com ",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        {
            "domain": "example/com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        {"domain": "example.com", "confirm": True},
        {"domain": "example.com", "remote_nameserver": "", "confirm": True},
        {"domain": "example.com", "remote_nameserver": 123, "confirm": True},
        {
            "domain": "example.com",
            "remote_nameserver": " ns1.example.net",
            "confirm": True,
        },
        {
            "domain": "example.com",
            "remote_nameserver": "ns1.example.net ",
            "confirm": True,
        },
    ],
)
async def test_handle_linode_domain_import_rejects_invalid_body_fields(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Required body fields are rejected before dry-run/confirm/client calls."""
    result = await handle_linode_domain_import(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_domain_import_reports_client_errors(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Client failures are mapped through the shared tool error path."""
    mock_linode_client.route_raw.side_effect = NetworkError(
        "ImportDomain", httpx.ConnectTimeout("boom")
    )

    result = await handle_linode_domain_import(
        {
            "domain": "example.com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text.startswith("Failed to import domain: ")
    mock_linode_client.route_raw.assert_awaited_once()


def test_linode_domain_import_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_domain_import"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_domain_import"
    assert entry.handle_fn is handle_linode_domain_import


def test_linode_domain_import_in_version_features() -> None:
    """Version metadata advertises the import tool."""
    assert "linode_domain_import" in FEATURE_TOOLS_LIST.split(",")
