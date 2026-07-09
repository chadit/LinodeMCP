"""Tests for the domain import route."""

from __future__ import annotations

import json
from typing import Any, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_domains_write import (
    create_linode_domain_import_tool,
    handle_linode_domain_import,
)
from linodemcp.version import FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_import_domain_sends_exact_path_and_body() -> None:
    """Low-level client sends POST /domains/import with documented body."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "id": 123,
                "domain": "example.com",
                "type": "master",
                "status": "active",
                "soa_email": "admin@example.com",
                "created": "2026-01-01T00:00:00",
                "updated": "2026-01-01T00:00:00",
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.import_domain("example.com", "ns1.example.net")
    finally:
        await client.close()

    assert result.domain == "example.com"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == "/v4/domains/import"
    assert request.url.query == b""
    assert json.loads(request.content) == {
        "domain": "example.com",
        "remote_nameserver": "ns1.example.net",
    }
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("domain", "remote_nameserver", "message"),
    [
        ("", "ns1.example.net", "domain is required"),
        (" example.com", "ns1.example.net", "domain is required"),
        ("example/com", "ns1.example.net", "label contains invalid character"),
        ("example.com", "", "remote_nameserver is required"),
        ("example.com", " ns1.example.net", "remote_nameserver is required"),
    ],
)
async def test_client_import_domain_validates_inputs_before_request(
    domain: str, remote_nameserver: str, message: str
) -> None:
    """Invalid import body fields are rejected before HTTP requests."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match=message):
            await client.import_domain(domain, remote_nameserver)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_import_domain_does_not_replay_post() -> None:
    """Domain import delegates once and does not use the generic retry wrapper."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    network_error = NetworkError("ImportDomain", httpx.ConnectTimeout("boom"))
    mock_import = AsyncMock(side_effect=network_error)
    cast("Any", retryable.client).import_domain = mock_import

    try:
        with pytest.raises(NetworkError):
            await retryable.import_domain("example.com", "ns1.example.net")
    finally:
        await retryable.close()

    mock_import.assert_awaited_once_with("example.com", "ns1.example.net")


def test_create_linode_domain_import_tool_schema() -> None:
    """Tool schema exposes required body fields plus confirm/dry_run."""
    tool, capability = create_linode_domain_import_tool()

    assert tool.name == "linode_domain_import"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "domain",
        "remote_nameserver",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["domain"]["type"] == "string"
    assert tool.inputSchema["properties"]["remote_nameserver"]["type"] == "string"
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_domain_import_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler imports the domain and returns the full proto domain element."""
    mock_linode_client.post_raw.return_value = {
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
    mock_linode_client.post_raw.assert_awaited_once_with(
        "/domains/import",
        {"domain": "example.com", "remote_nameserver": "ns1.example.net"},
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
    mock_linode_client.import_domain.assert_not_called()


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
    mock_linode_client.import_domain.assert_not_called()


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
    mock_linode_client.import_domain.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_domain_import_reports_client_errors(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Client failures are mapped through the shared tool error path."""
    mock_linode_client.post_raw.side_effect = NetworkError(
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
    mock_linode_client.post_raw.assert_awaited_once()


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
