"""Tests for the image replicate route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability, Scope, required_scopes
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_images import (
    create_linode_image_replicate_tool,
    handle_linode_image_replicate,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


class _CapturingRetryableClient(RetryableClient):
    """RetryableClient test double that records retry callbacks."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.calls: list[Callable[..., Awaitable[Any]]] = []

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        self.calls.append(func)
        return await func(*args)


@pytest.mark.asyncio
async def test_client_replicate_image_sends_exact_path_and_body() -> None:
    """Low-level client sends POST /images/{imageId}/regions."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={"id": "private/123", "label": "replicated-image"},
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.replicate_image("private/123", ["us-mia", "us-east"])
    finally:
        await client.close()

    assert result["id"] == "private/123"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.raw_path == b"/v4/images/private%2F123/regions"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads((await request.aread()).decode()) == {
        "regions": ["us-mia", "us-east"]
    }


@pytest.mark.asyncio
async def test_client_replicate_image_escapes_image_id() -> None:
    """Low-level client URL-encodes untrusted image IDs at the path boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "private/123"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.replicate_image("private/123?bad", ["us-east"])
    finally:
        await client.close()

    assert seen[0].url.raw_path == b"/v4/images/private%2F123%3Fbad/regions"


@pytest.mark.asyncio
async def test_client_replicate_image_maps_http_error() -> None:
    """Low-level client maps HTTP errors to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ReplicateImage"):
            await client.replicate_image("private/123", ["us-east"])
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_client_replicate_image_delegates_once() -> None:
    """Mutating replicate-image requests are not replayed by retry logic."""
    retryable = _CapturingRetryableClient()
    mock_replicate = AsyncMock(return_value={"id": "private/123"})
    cast("Any", retryable.client).replicate_image = mock_replicate

    try:
        result = await retryable.replicate_image("private/123", ["us-east"])
    finally:
        await retryable.close()

    assert result == {"id": "private/123"}
    assert retryable.calls == []
    mock_replicate.assert_awaited_once_with("private/123", ["us-east"])


def test_create_linode_image_replicate_tool_schema() -> None:
    """Replicate tool schema exposes image_id, regions, confirm, and dry_run."""
    tool, capability = create_linode_image_replicate_tool()

    assert tool.name == "linode_image_replicate"
    assert capability is Capability.Write
    schema = tool.inputSchema
    assert schema["required"] == ["image_id", "regions", "confirm"]
    assert schema["properties"]["image_id"]["type"] == "string"
    assert schema["properties"]["regions"]["type"] == "array"
    assert schema["properties"]["regions"]["items"] == {"type": "string"}
    assert schema["properties"]["confirm"]["type"] == "boolean"
    assert schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_image_replicate_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler replicates an image with the documented body fields."""
    mock_linode_client.replicate_image.return_value = {
        "id": "private/123",
        "label": "replicated-image",
    }

    result = await handle_linode_image_replicate(
        {"image_id": "private/123", "regions": ["us-mia", "us-east"], "confirm": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image 'private/123' replicated successfully",
        "image": {
            "id": "private/123",
            "label": "replicated-image",
            "description": "",
            "type": "",
            "vendor": "",
            "status": "",
            "created": "",
            "created_by": "",
            "capabilities": [],
            "tags": [],
            "size": 0,
            "is_public": False,
            "deprecated": False,
        },
    }
    mock_linode_client.replicate_image.assert_awaited_once_with(
        "private/123", ["us-mia", "us-east"]
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_image_replicate_requires_literal_confirm(
    confirm_value: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Missing, false, string, and numeric confirm values stop before client calls."""
    arguments: dict[str, Any] = {"image_id": "private/123", "regions": ["us-east"]}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_image_replicate(arguments, sample_config)

    assert result[0].text == (
        "Error: This replicates an image to regions. Set confirm=true to proceed."
    )
    mock_linode_client.replicate_image.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"regions": ["us-east"], "confirm": True},
            "image_id must be a non-empty string",
        ),
        (
            {"image_id": "", "regions": ["us-east"], "confirm": True},
            "image_id must be a non-empty string",
        ),
        (
            {"image_id": "private/../123", "regions": ["us-east"], "confirm": True},
            "image_id must not contain traversal or query separators",
        ),
        (
            {"image_id": "private/123?bad", "regions": ["us-east"], "confirm": True},
            "image_id must not contain traversal or query separators",
        ),
        (
            {"image_id": "private/123", "confirm": True},
            "regions must be a non-empty list of region slugs",
        ),
        (
            {"image_id": "private/123", "regions": [], "confirm": True},
            "regions must be a non-empty list of region slugs",
        ),
        (
            {"image_id": "private/123", "regions": [""], "confirm": True},
            "regions must contain non-empty strings",
        ),
        (
            {"image_id": "private/123", "regions": ["us/east"], "confirm": True},
            "regions must not contain path or query separators",
        ),
    ],
)
async def test_handle_linode_image_replicate_rejects_invalid_inputs(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler validates path and body inputs before client calls."""
    result = await handle_linode_image_replicate(arguments, sample_config)

    assert result[0].text == f"Error: {message}"
    mock_linode_client.replicate_image.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_image_replicate_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry-run reports exact POST path and body without calling the client."""
    result = await handle_linode_image_replicate(
        {
            "image_id": "private/123",
            "regions": ["us-east"],
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["tool"] == "linode_image_replicate"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/images/private%2F123/regions",
        "body": {"regions": ["us-east"]},
    }
    mock_linode_client.replicate_image.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_image_replicate_reports_client_errors(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Client failures are mapped through the shared tool error path."""
    mock_linode_client.replicate_image.side_effect = NetworkError(
        "ReplicateImage", httpx.ConnectError("temporary")
    )

    result = await handle_linode_image_replicate(
        {"image_id": "private/123", "regions": ["us-east"], "confirm": True},
        sample_config,
    )

    assert result[0].text.startswith("Failed to replicate Linode image")


def test_linode_image_replicate_registered() -> None:
    """Dynamic registry exports the replicate tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_replicate"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_image_replicate"
    assert entry.handle_fn is handle_linode_image_replicate


def test_linode_image_replicate_scopes_to_images_write() -> None:
    """Profile scope mapping keeps image replication in the Images write category."""
    scopes = required_scopes("linode_image_replicate", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_image_replicate_in_feature_tools() -> None:
    """Version metadata advertises the replicate tool."""
    assert "linode_image_replicate" in FEATURE_TOOLS_LIST.split(",")
