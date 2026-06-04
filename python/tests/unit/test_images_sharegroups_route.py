"""Tests for the image share groups route."""

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
    create_linode_image_sharegroup_create_tool,
    create_linode_images_sharegroup_image_delete_tool,
    create_linode_images_sharegroup_image_update_tool,
    create_linode_images_sharegroup_images_add_tool,
    create_linode_images_sharegroup_images_list_tool,
    create_linode_images_sharegroup_member_token_delete_tool,
    create_linode_images_sharegroup_member_token_get_tool,
    create_linode_images_sharegroup_member_token_update_tool,
    create_linode_images_sharegroup_members_add_tool,
    create_linode_images_sharegroup_members_list_tool,
    create_linode_images_sharegroups_list_tool,
    create_linode_images_sharegroups_token_delete_tool,
    create_linode_images_sharegroups_token_get_tool,
    create_linode_images_sharegroups_token_sharegroup_get_tool,
    create_linode_images_sharegroups_token_sharegroup_images_list_tool,
    create_linode_images_sharegroups_token_update_tool,
    create_linode_images_sharegroups_tokens_list_tool,
    handle_linode_image_sharegroup_create,
    handle_linode_images_sharegroup_image_delete,
    handle_linode_images_sharegroup_image_update,
    handle_linode_images_sharegroup_images_add,
    handle_linode_images_sharegroup_images_list,
    handle_linode_images_sharegroup_member_token_delete,
    handle_linode_images_sharegroup_member_token_get,
    handle_linode_images_sharegroup_member_token_update,
    handle_linode_images_sharegroup_members_add,
    handle_linode_images_sharegroup_members_list,
    handle_linode_images_sharegroups_list,
    handle_linode_images_sharegroups_token_delete,
    handle_linode_images_sharegroups_token_get,
    handle_linode_images_sharegroups_token_sharegroup_get,
    handle_linode_images_sharegroups_token_sharegroup_images_list,
    handle_linode_images_sharegroups_token_update,
    handle_linode_images_sharegroups_tokens_list,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")

INVALID_ADD_IMAGE_SHAREGROUP_IMAGES_CASES: list[tuple[object, str]] = [
    (None, "images must be a non-empty list of image objects"),
    ([], "images must be a non-empty list of image objects"),
    ("private/ubuntu", "images must be a non-empty list of image objects"),
    (["private/ubuntu"], "images must contain objects"),
    ([{}], "images[].id must be a non-empty string"),
    ([{"id": ""}], "images[].id must be a non-empty string"),
    ([{"id": "private/ubuntu", "label": 1}], "images[].label must be a string"),
    (
        [{"id": "private/ubuntu", "description": 1}],
        "images[].description must be a string",
    ),
]


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
async def test_client_list_image_sharegroups_sends_exact_path_and_query() -> None:
    """Low-level client sends GET /images/sharegroups with pagination."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"id": "share-1", "label": "shared images"}],
                "page": 2,
                "pages": 3,
                "results": 7,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroups(page=2, page_size=50)
    finally:
        await client.close()

    assert result["data"][0]["id"] == "share-1"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/images/sharegroups"
    assert request.url.query == b"page=2&page_size=50"
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_member_token_accepts_no_content() -> None:
    """Low-level client accepts 204 No Content delete responses."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(204)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_member_token(
            "22222222-2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        )
    finally:
        await client.close()

    assert len(seen) == 1


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("kwargs", "message"),
    [
        ({"page": 0}, "page must be an integer at least 1"),
        ({"page": "2"}, "page must be an integer at least 1"),
        ({"page": True}, "page must be an integer at least 1"),
        ({"page_size": 24}, "page_size must be an integer between 25 and 500"),
        ({"page_size": 501}, "page_size must be an integer between 25 and 500"),
        ({"page_size": "50"}, "page_size must be an integer between 25 and 500"),
        ({"page_size": False}, "page_size must be an integer between 25 and 500"),
    ],
)
async def test_client_list_image_sharegroups_validates_pagination_before_request(
    kwargs: dict[str, Any], message: str
) -> None:
    """Invalid pagination is rejected locally before an HTTP request."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={"data": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match=message):
            await client.list_image_sharegroups(**kwargs)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_list_image_sharegroups_uses_read_retry() -> None:
    """Read-only image share groups list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_image_sharegroups = mock_list

    try:
        result = await retryable.list_image_sharegroups(page=1, page_size=25)
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(page=1, page_size=25)


def test_create_linode_images_sharegroups_list_tool_schema() -> None:
    """Tool schema exposes the documented pagination params."""
    tool, capability = create_linode_images_sharegroups_list_tool()

    assert tool.name == "linode_images_sharegroups_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share groups and pagination metadata."""
    mock_linode_client.list_image_sharegroups.return_value = {
        "data": [{"id": "share-1", "label": "shared images"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_images_sharegroups_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share groups listed",
        "count": 1,
        "sharegroups": [{"id": "share-1", "label": "shared images"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }
    mock_linode_client.list_image_sharegroups.assert_awaited_once_with(
        page=2, page_size=50
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"page": 0},
        {"page": "2"},
        {"page": True},
        {"page_size": 24},
        {"page_size": 501},
        {"page_size": "50"},
        {"page_size": False},
    ],
)
async def test_handle_linode_images_sharegroups_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid pagination before creating a client call."""
    result = await handle_linode_images_sharegroups_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_image_sharegroups.assert_not_called()


def test_linode_images_sharegroups_list_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_list"
    assert entry.handle_fn is handle_linode_images_sharegroups_list


def test_linode_images_sharegroups_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroups_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


@pytest.mark.asyncio
async def test_client_create_image_sharegroup_sends_exact_body() -> None:
    """Low-level client sends POST /images/sharegroups with documented body."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        body = json.loads(request.content)
        assert body == {
            "label": "shared images",
            "description": "team image pool",
            "images": [
                {
                    "id": "private/7",
                    "label": "Linux Debian",
                    "description": "Official Debian Linux image",
                }
            ],
        }
        return httpx.Response(
            200,
            json={
                "id": 1,
                "uuid": "123e4567-e89b-12d3-a456-426614174000",
                "label": "shared images",
                "description": "team image pool",
                "images_count": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.create_image_sharegroup(
            label="shared images",
            description="team image pool",
            images=[
                {
                    "id": "private/7",
                    "label": "Linux Debian",
                    "description": "Official Debian Linux image",
                }
            ],
        )
    finally:
        await client.close()

    assert result["label"] == "shared images"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == "/v4/images/sharegroups"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_create_image_sharegroup_maps_http_error() -> None:
    """Low-level client maps HTTP errors to a NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="CreateImageShareGroup"):
            await client.create_image_sharegroup(label="shared images")
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_client_create_image_sharegroup_does_not_retry() -> None:
    """Mutating image share group create delegates once without retry replay."""
    retryable = _CapturingRetryableClient()
    mock_create = AsyncMock(return_value={"label": "shared images"})
    cast("Any", retryable.client).create_image_sharegroup = mock_create

    try:
        result = await retryable.create_image_sharegroup(label="shared images")
    finally:
        await retryable.close()

    assert result["label"] == "shared images"
    assert retryable.calls == []
    mock_create.assert_awaited_once_with(
        label="shared images", description=None, images=None
    )


def test_create_linode_image_sharegroup_create_tool_schema() -> None:
    """Create tool schema exposes label, images, confirm, and dry_run."""
    tool, capability = create_linode_image_sharegroup_create_tool()

    assert tool.name == "linode_image_sharegroup_create"
    assert capability is Capability.Write
    schema = tool.inputSchema
    assert schema["required"] == ["label", "confirm"]
    assert schema["properties"]["label"]["type"] == "string"
    assert schema["properties"]["images"]["items"]["required"] == ["id"]
    assert schema["properties"]["confirm"]["type"] == "boolean"
    assert schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_image_sharegroup_create_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler creates a share group with validated documented body fields."""
    mock_linode_client.create_image_sharegroup.return_value = {
        "id": 1,
        "label": "shared images",
        "images_count": 1,
    }

    result = await handle_linode_image_sharegroup_create(
        {
            "label": "shared images",
            "description": "team image pool",
            "images": [{"id": "private/7", "label": "Linux Debian"}],
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group 'shared images' created",
        "sharegroup": {"id": 1, "label": "shared images", "images_count": 1},
    }
    mock_linode_client.create_image_sharegroup.assert_awaited_once_with(
        label="shared images",
        description="team image pool",
        images=[{"id": "private/7", "label": "Linux Debian"}],
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_image_sharegroup_create_requires_literal_confirm(
    confirm_value: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Missing, false, string, and numeric confirm values stop before client calls."""
    arguments = {"label": "shared images"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_image_sharegroup_create(arguments, sample_config)

    assert result[0].text == (
        "Error: This creates an image share group. Set confirm=true to proceed."
    )
    mock_linode_client.create_image_sharegroup.assert_not_called()


IMAGE_SHAREGROUP_CREATE_INVALID_PAYLOAD_CASES: list[tuple[dict[str, Any], str]] = [
    ({"confirm": True}, "label must be a non-empty string"),
    ({"confirm": True, "label": ""}, "label must be a non-empty string"),
    ({"confirm": True, "label": "   "}, "label must be a non-empty string"),
    (
        {"confirm": True, "label": "shared images", "description": 7},
        "description must be a string",
    ),
    (
        {"confirm": True, "label": "shared images", "images": "private/7"},
        "images must be a list of image objects",
    ),
    (
        {"confirm": True, "label": "shared images", "images": ["private/7"]},
        "images must contain objects",
    ),
    (
        {"confirm": True, "label": "shared images", "images": [{}]},
        "images[].id must be a non-empty string",
    ),
    (
        {
            "confirm": True,
            "label": "shared images",
            "images": [{"id": "private/7", "label": 7}],
        },
        "images[].label must be a string",
    ),
    (
        {
            "confirm": True,
            "label": "shared images",
            "images": [{"id": "private/7", "description": 7}],
        },
        "images[].description must be a string",
    ),
]


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"), IMAGE_SHAREGROUP_CREATE_INVALID_PAYLOAD_CASES
)
async def test_handle_linode_image_sharegroup_create_rejects_invalid_payload(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler validates body shape before client calls."""
    result = await handle_linode_image_sharegroup_create(arguments, sample_config)

    assert result[0].text == f"Error: {message}"
    mock_linode_client.create_image_sharegroup.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_image_sharegroup_create_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry-run reports exact POST path and body without calling the client."""
    result = await handle_linode_image_sharegroup_create(
        {
            "label": "shared images",
            "description": "team image pool",
            "images": [{"id": "private/7"}],
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["tool"] == "linode_image_sharegroup_create"
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/images/sharegroups",
        "body": {
            "label": "shared images",
            "description": "team image pool",
            "images": [{"id": "private/7"}],
        },
    }
    mock_linode_client.create_image_sharegroup.assert_not_called()


def test_linode_image_sharegroup_create_registered() -> None:
    """Dynamic registry exports the create tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_create"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_image_sharegroup_create"
    assert entry.handle_fn is handle_linode_image_sharegroup_create


def test_linode_image_sharegroup_create_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the create route in the Images write category."""
    scopes = required_scopes("linode_image_sharegroup_create", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_tokens_sends_exact_path() -> None:
    """Low-level client sends GET /images/sharegroups/tokens with no query/body."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [
                    {
                        "id": "sharegroup-record-1",
                        "created": "2026-01-01T00:00:00",
                    }
                ],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroup_tokens()
    finally:
        await client.close()

    assert result["data"][0]["id"] == "sharegroup-record-1"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/images/sharegroups/tokens"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_retryable_client_list_image_sharegroup_tokens_uses_read_retry() -> None:
    """Read-only image share group tokens list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_image_sharegroup_tokens = mock_list

    try:
        result = await retryable.list_image_sharegroup_tokens()
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with()


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_token_sends_exact_encoded_path() -> None:
    """Low-level client sends GET /images/sharegroups/tokens/{tokenUuid}."""
    seen: list[httpx.Request] = []
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "id": "sharegroup-record-1",
                "token_uuid": token_uuid,
                "created": "2026-01-01T00:00:00",
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_image_sharegroup_token(token_uuid)
    finally:
        await client.close()

    assert result["token_uuid"] == token_uuid
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == f"/v4/images/sharegroups/tokens/{token_uuid}"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_token_encodes_path_param() -> None:
    """Low-level client URL-encodes token_uuid at the path boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "encoded"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.get_image_sharegroup_token("token/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/tokens/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
async def test_retryable_client_get_image_sharegroup_token_uses_read_retry() -> None:
    """Read-only image share group token get goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_get = AsyncMock(return_value={"id": "sharegroup-record-1"})
    cast("Any", retryable.client).get_image_sharegroup_token = mock_get

    try:
        result = await retryable.get_image_sharegroup_token(token_uuid)
    finally:
        await retryable.close()

    assert result["id"] == "sharegroup-record-1"
    assert len(retryable.calls) == 1
    mock_get.assert_awaited_once_with(token_uuid)


def test_create_linode_images_sharegroups_token_get_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = create_linode_images_sharegroups_token_get_tool()

    assert tool.name == "linode_images_sharegroups_token_get"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "token_uuid"}
    assert tool.inputSchema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns a single image share group token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.get_image_sharegroup_token.return_value = {
        "id": "sharegroup-record-1",
        "token_uuid": token_uuid,
        "created": "2026-01-01T00:00:00",
    }

    result = await handle_linode_images_sharegroups_token_get(
        {"token_uuid": f" {token_uuid} "}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group token retrieved",
        "token": {
            "id": "sharegroup-record-1",
            "token_uuid": token_uuid,
            "created": "2026-01-01T00:00:00",
        },
    }
    mock_linode_client.get_image_sharegroup_token.assert_awaited_once_with(token_uuid)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_linode_images_sharegroups_token_get_rejects_invalid_token_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_images_sharegroups_token_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.get_image_sharegroup_token.assert_not_called()


def test_linode_images_sharegroups_token_get_registered() -> None:
    """Dynamic registry exports the token get tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_token_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_token_get"
    assert entry.handle_fn is handle_linode_images_sharegroups_token_get


def test_linode_images_sharegroups_token_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroups_token_get", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroups_token_get_in_version_features() -> None:
    """Version metadata advertises the token get tool."""
    assert "linode_images_sharegroups_token_get" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_by_token_sends_exact_encoded_path() -> None:
    """Low-level client sends GET /images/sharegroups/tokens/{tokenUuid}/sharegroup."""
    seen: list[httpx.Request] = []
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={"uuid": "22222222-2222-4222-8222-222222222222"},
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_image_sharegroup_by_token(token_uuid)
    finally:
        await client.close()

    assert result["uuid"] == "22222222-2222-4222-8222-222222222222"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == f"/v4/images/sharegroups/tokens/{token_uuid}/sharegroup"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_by_token_encodes_path_param() -> None:
    """Low-level client URL-encodes token_uuid before appending /sharegroup."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"uuid": "encoded"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.get_image_sharegroup_by_token("token/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/tokens/token%2Fwith%3Fseparator/sharegroup"
    )


@pytest.mark.asyncio
async def test_retryable_client_get_image_sharegroup_by_token_uses_read_retry() -> None:
    """Read-only share group by token get goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_get = AsyncMock(return_value={"uuid": "sharegroup-1"})
    cast("Any", retryable.client).get_image_sharegroup_by_token = mock_get

    try:
        result = await retryable.get_image_sharegroup_by_token(token_uuid)
    finally:
        await retryable.close()

    assert result["uuid"] == "sharegroup-1"
    assert len(retryable.calls) == 1
    mock_get.assert_awaited_once_with(token_uuid)


def test_create_linode_images_sharegroups_token_sharegroup_get_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = create_linode_images_sharegroups_token_sharegroup_get_tool()

    assert tool.name == "linode_images_sharegroups_token_sharegroup_get"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "token_uuid"}
    assert tool.inputSchema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_sharegroup_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns the share group associated with a token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.get_image_sharegroup_by_token.return_value = {
        "uuid": "22222222-2222-4222-8222-222222222222",
        "label": "shared-images",
    }

    result = await handle_linode_images_sharegroups_token_sharegroup_get(
        {"token_uuid": f" {token_uuid} "}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group retrieved",
        "sharegroup": {
            "uuid": "22222222-2222-4222-8222-222222222222",
            "label": "shared-images",
        },
    }
    mock_linode_client.get_image_sharegroup_by_token.assert_awaited_once_with(
        token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_token_sharegroup_get_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_images_sharegroups_token_sharegroup_get(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.get_image_sharegroup_by_token.assert_not_called()


def test_linode_images_sharegroups_token_sharegroup_get_registered() -> None:
    """Dynamic registry exports the share group by token tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_token_sharegroup_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_token_sharegroup_get"
    assert entry.handle_fn is handle_linode_images_sharegroups_token_sharegroup_get


def test_linode_images_sharegroups_token_sharegroup_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes(
        "linode_images_sharegroups_token_sharegroup_get", Capability.Read
    )

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroups_token_sharegroup_get_in_version_features() -> None:
    """Version metadata advertises the share group by token tool."""
    assert "linode_images_sharegroups_token_sharegroup_get" in FEATURE_TOOLS_LIST.split(
        ","
    )


@pytest.mark.asyncio
async def test_client_list_images_by_token_sends_exact_encoded_path() -> None:
    """Low-level client sends GET images-by-token route."""
    seen: list[httpx.Request] = []
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroup_images_by_token(token_uuid)
    finally:
        await client.close()

    assert result["data"][0]["id"] == "private/ubuntu"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == (
        f"/v4/images/sharegroups/tokens/{token_uuid}/sharegroup/images"
    )
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_list_images_by_token_encodes_path_param() -> None:
    """Low-level client URL-encodes token_uuid before appending /sharegroup/images."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.list_image_sharegroup_images_by_token("token/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/tokens/token%2Fwith%3Fseparator/sharegroup/images"
    )


@pytest.mark.asyncio
async def test_retryable_list_images_by_token_uses_read_retry() -> None:
    """Read-only images by token list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_list = AsyncMock(return_value={"data": [{"id": "private/ubuntu"}]})
    cast("Any", retryable.client).list_image_sharegroup_images_by_token = mock_list

    try:
        result = await retryable.list_image_sharegroup_images_by_token(token_uuid)
    finally:
        await retryable.close()

    assert result["data"][0]["id"] == "private/ubuntu"
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(token_uuid)


def test_create_token_sharegroup_images_list_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = (
        create_linode_images_sharegroups_token_sharegroup_images_list_tool()
    )

    assert tool.name == "linode_images_sharegroups_token_sharegroup_images_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment", "token_uuid"}
    assert tool.inputSchema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_sharegroup_images_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns images associated with a share group token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.list_image_sharegroup_images_by_token.return_value = {
        "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_images_sharegroups_token_sharegroup_images_list(
        {"token_uuid": f" {token_uuid} "}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group images retrieved",
        "count": 1,
        "images": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_linode_client.list_image_sharegroup_images_by_token.assert_awaited_once_with(
        token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_token_sharegroup_images_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_images_sharegroups_token_sharegroup_images_list(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_image_sharegroup_images_by_token.assert_not_called()


def test_linode_images_sharegroups_token_sharegroup_images_list_registered() -> None:
    """Dynamic registry exports the images by token tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_token_sharegroup_images_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_token_sharegroup_images_list"
    assert (
        entry.handle_fn is handle_linode_images_sharegroups_token_sharegroup_images_list
    )


def test_token_sharegroup_images_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes(
        "linode_images_sharegroups_token_sharegroup_images_list", Capability.Read
    )

    assert scopes == [Scope.ImagesReadOnly]


def test_token_sharegroup_images_list_in_version_features() -> None:
    """Version metadata advertises the images by token tool."""
    features = FEATURE_TOOLS_LIST.split(",")

    assert "linode_images_sharegroups_token_sharegroup_images_list" in features


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_members_sends_exact_encoded_path() -> None:
    """Low-level client sends GET /images/sharegroups/{sharegroupId}/members."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"username": "alice", "status": "accepted"}],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroup_members(sharegroup_id)
    finally:
        await client.close()

    assert result["data"][0]["username"] == "alice"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == f"/v4/images/sharegroups/{sharegroup_id}/members"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_members_encodes_path_param() -> None:
    """Low-level client URL-encodes sharegroup_id before appending /members."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.list_image_sharegroup_members("sharegroup/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator/members"
    )


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_members_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ListImageSharegroupMembers"):
            await client.list_image_sharegroup_members(
                "22222222-2222-4222-8222-222222222222"
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_list_image_sharegroup_members_uses_read_retry() -> None:
    """Read-only members by share group list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_list = AsyncMock(return_value={"data": [{"username": "alice"}]})
    cast("Any", retryable.client).list_image_sharegroup_members = mock_list

    try:
        result = await retryable.list_image_sharegroup_members(sharegroup_id)
    finally:
        await retryable.close()

    assert result["data"][0]["username"] == "alice"
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(sharegroup_id)


def test_create_linode_images_sharegroup_members_list_tool_schema() -> None:
    """Tool schema requires the documented sharegroup UUID path param."""
    tool, capability = create_linode_images_sharegroup_members_list_tool()

    assert tool.name == "linode_images_sharegroup_members_list"
    assert capability is Capability.Read
    sharegroup_id_schema = tool.inputSchema["properties"]["sharegroup_id"]
    assert set(tool.inputSchema["properties"]) == {"environment", "sharegroup_id"}
    assert tool.inputSchema["required"] == ["sharegroup_id"]
    assert "[0-9a-fA-F]{8}" in sharegroup_id_schema["pattern"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns members associated with a share group."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_linode_client.list_image_sharegroup_members.return_value = {
        "data": [{"username": "alice", "status": "accepted"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_images_sharegroup_members_list(
        {"sharegroup_id": f" {sharegroup_id} "}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group members retrieved",
        "count": 1,
        "members": [{"username": "alice", "status": "accepted"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_linode_client.list_image_sharegroup_members.assert_awaited_once_with(
        sharegroup_id
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_list_defaults_missing_pagination(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler supplies stable pagination defaults for sparse API responses."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_linode_client.list_image_sharegroup_members.return_value = {}

    result = await handle_linode_images_sharegroup_members_list(
        {"sharegroup_id": sharegroup_id}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group members retrieved",
        "count": 0,
        "members": [],
        "page": 1,
        "pages": 1,
        "results": 0,
    }
    mock_linode_client.list_image_sharegroup_members.assert_awaited_once_with(
        sharegroup_id
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 123},
    ],
)
async def test_handle_linode_images_sharegroup_members_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    result = await handle_linode_images_sharegroup_members_list(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_image_sharegroup_members.assert_not_called()


def test_linode_images_sharegroup_members_list_registered() -> None:
    """Dynamic registry exports the members by share group tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_members_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroup_members_list"
    assert entry.handle_fn is handle_linode_images_sharegroup_members_list


def test_linode_images_sharegroup_members_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroup_members_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroup_members_list_in_version_features() -> None:
    """Version metadata advertises the members by share group tool."""
    assert "linode_images_sharegroup_members_list" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_member_token_sends_exact_encoded_path() -> (
    None
):
    """Low-level client sends GET /images/sharegroups/{id}/members/{tokenUuid}."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "id": "member-token-record-1",
                "token_uuid": token_uuid,
                "created": "2026-01-01T00:00:00",
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_image_sharegroup_member_token(
            sharegroup_id, token_uuid
        )
    finally:
        await client.close()

    assert result["token_uuid"] == token_uuid
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == (
        f"/v4/images/sharegroups/{sharegroup_id}/members/{token_uuid}"
    )
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_member_token_encodes_path_params() -> None:
    """Low-level client URL-encodes both path params at the boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "encoded"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.get_image_sharegroup_member_token(
            "sharegroup/with?separator", "token/with?separator"
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator"
        b"/members/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
async def test_client_get_image_sharegroup_member_token_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="GetImageSharegroupMemberToken"):
            await client.get_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_get_image_sharegroup_member_token_uses_read_retry() -> None:
    """Read-only member token get goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_get = AsyncMock(return_value={"id": "member-token-record-1"})
    cast("Any", retryable.client).get_image_sharegroup_member_token = mock_get

    try:
        result = await retryable.get_image_sharegroup_member_token(
            sharegroup_id, token_uuid
        )
    finally:
        await retryable.close()

    assert result["id"] == "member-token-record-1"
    assert len(retryable.calls) == 1
    mock_get.assert_awaited_once_with(sharegroup_id, token_uuid)


def test_create_linode_images_sharegroup_member_token_get_tool_schema() -> None:
    """Tool schema requires both documented path params."""
    tool, capability = create_linode_images_sharegroup_member_token_get_tool()

    assert tool.name == "linode_images_sharegroup_member_token_get"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {
        "environment",
        "sharegroup_id",
        "token_uuid",
    }
    assert tool.inputSchema["required"] == ["sharegroup_id", "token_uuid"]
    sharegroup_schema = tool.inputSchema["properties"]["sharegroup_id"]
    token_schema = tool.inputSchema["properties"]["token_uuid"]
    assert "[0-9a-fA-F]{8}" in sharegroup_schema["pattern"]
    assert "[0-9a-fA-F]{8}" in token_schema["pattern"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns a membership token associated with a share group."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.get_image_sharegroup_member_token.return_value = {
        "id": "member-token-record-1",
        "token_uuid": token_uuid,
        "created": "2026-01-01T00:00:00",
    }

    result = await handle_linode_images_sharegroup_member_token_get(
        {"sharegroup_id": f" {sharegroup_id} ", "token_uuid": f" {token_uuid} "},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group member token retrieved",
        "token": {
            "id": "member-token-record-1",
            "token_uuid": token_uuid,
            "created": "2026-01-01T00:00:00",
        },
    }
    mock_linode_client.get_image_sharegroup_member_token.assert_awaited_once_with(
        sharegroup_id, token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("sharegroup_id", "token_uuid"),
    [
        (None, "11111111-1111-4111-8111-111111111111"),
        ("", "11111111-1111-4111-8111-111111111111"),
        ("not-a-uuid", "11111111-1111-4111-8111-111111111111"),
        (
            "22222222/2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        (
            "22222222?2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        ("..", "11111111-1111-4111-8111-111111111111"),
        (123, "11111111-1111-4111-8111-111111111111"),
        ("22222222-2222-4222-8222-222222222222", None),
        ("22222222-2222-4222-8222-222222222222", ""),
        ("22222222-2222-4222-8222-222222222222", "not-a-uuid"),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111/1111-4111-8111-111111111111",
        ),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111?1111-4111-8111-111111111111",
        ),
        ("22222222-2222-4222-8222-222222222222", ".."),
        ("22222222-2222-4222-8222-222222222222", 123),
    ],
)
async def test_member_token_get_rejects_invalid_path_params(
    sharegroup_id: Any,
    token_uuid: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before the client call."""
    result = await handle_linode_images_sharegroup_member_token_get(
        {"sharegroup_id": sharegroup_id, "token_uuid": token_uuid}, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.get_image_sharegroup_member_token.assert_not_called()


def test_linode_images_sharegroup_member_token_get_registered() -> None:
    """Dynamic registry exports the member token get tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_member_token_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroup_member_token_get"
    assert entry.handle_fn is handle_linode_images_sharegroup_member_token_get


def test_linode_images_sharegroup_member_token_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes(
        "linode_images_sharegroup_member_token_get", Capability.Read
    )

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroup_member_token_get_in_version_features() -> None:
    """Version metadata advertises the member token get tool."""
    assert "linode_images_sharegroup_member_token_get" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_member_token_sends_exact_path_body() -> (
    None
):
    """Low-level client sends PUT /images/sharegroups/{id}/members/{tokenUuid}."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "id": "member-token-record-1",
                "token_uuid": token_uuid,
                "label": "renamed-member",
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_image_sharegroup_member_token(
            sharegroup_id, token_uuid, label="renamed-member"
        )
    finally:
        await client.close()

    assert result["label"] == "renamed-member"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == (
        f"/v4/images/sharegroups/{sharegroup_id}/members/{token_uuid}"
    )
    assert request.url.query == b""
    assert json.loads((await request.aread()).decode()) == {"label": "renamed-member"}
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_member_token_encodes_path_params() -> (
    None
):
    """Low-level client URL-encodes both path params at the boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "encoded", "label": "renamed"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.update_image_sharegroup_member_token(
            "sharegroup/with?separator", "token/with?separator", label="renamed"
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator"
        b"/members/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_label", ["", "   "])
async def test_client_update_image_sharegroup_member_token_rejects_blank_label(
    bad_label: str,
) -> None:
    """Low-level client rejects blank labels before request creation."""
    calls = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal calls
        calls += 1
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="label must be a non-empty string"):
            await client.update_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
                label=bad_label,
            )
    finally:
        await client.close()

    assert calls == 0


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_member_token_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateImageSharegroupMemberToken"):
            await client.update_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
                label="renamed-member",
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_member_token_update_delegates_once() -> None:
    """Retryable update wrapper should not replay member token updates after errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_update = AsyncMock(side_effect=httpx.HTTPError("temporary"))
    cast("Any", retryable.client).update_image_sharegroup_member_token = mock_update

    try:
        with pytest.raises(httpx.HTTPError):
            await retryable.update_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
                label="renamed-member",
            )
    finally:
        await retryable.close()

    mock_update.assert_awaited_once_with(
        "22222222-2222-4222-8222-222222222222",
        "11111111-1111-4111-8111-111111111111",
        label="renamed-member",
    )


def test_create_linode_images_sharegroup_member_token_update_tool_schema() -> None:
    """Tool schema requires both path params, label, and confirm."""
    tool, capability = create_linode_images_sharegroup_member_token_update_tool()

    assert tool.name == "linode_images_sharegroup_member_token_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "sharegroup_id",
        "token_uuid",
        "label",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert (
        "[0-9a-fA-F]{8}" in tool.inputSchema["properties"]["sharegroup_id"]["pattern"]
    )
    assert "[0-9a-fA-F]{8}" in tool.inputSchema["properties"]["token_uuid"]["pattern"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler updates a member token label through the client."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.update_image_sharegroup_member_token.return_value = {
        "id": "member-token-record-1",
        "token_uuid": token_uuid,
        "label": "renamed-member",
    }

    result = await handle_linode_images_sharegroup_member_token_update(
        {
            "sharegroup_id": f" {sharegroup_id} ",
            "token_uuid": f" {token_uuid} ",
            "label": " renamed-member ",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group member token updated",
        "token": {
            "id": "member-token-record-1",
            "token_uuid": token_uuid,
            "label": "renamed-member",
        },
    }
    mock_linode_client.update_image_sharegroup_member_token.assert_awaited_once_with(
        sharegroup_id, token_uuid, label="renamed-member"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_member_token_update_requires_true_confirm(
    bad_confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects non-true confirm values before the client call."""
    arguments: dict[str, Any] = {
        "sharegroup_id": "22222222-2222-4222-8222-222222222222",
        "token_uuid": "11111111-1111-4111-8111-111111111111",
        "label": "renamed-member",
    }
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    result = await handle_linode_images_sharegroup_member_token_update(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    assert "confirm=true" in result[0].text
    mock_linode_client.update_image_sharegroup_member_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("sharegroup_id", "token_uuid"),
    [
        (None, "11111111-1111-4111-8111-111111111111"),
        ("", "11111111-1111-4111-8111-111111111111"),
        ("not-a-uuid", "11111111-1111-4111-8111-111111111111"),
        (
            "22222222/2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        (
            "22222222?2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        ("..", "11111111-1111-4111-8111-111111111111"),
        (123, "11111111-1111-4111-8111-111111111111"),
        ("22222222-2222-4222-8222-222222222222", None),
        ("22222222-2222-4222-8222-222222222222", ""),
        ("22222222-2222-4222-8222-222222222222", "not-a-uuid"),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111/1111-4111-8111-111111111111",
        ),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111?1111-4111-8111-111111111111",
        ),
        ("22222222-2222-4222-8222-222222222222", ".."),
        ("22222222-2222-4222-8222-222222222222", 123),
    ],
)
async def test_member_token_update_rejects_invalid_path_params(
    sharegroup_id: Any,
    token_uuid: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before the client call."""
    result = await handle_linode_images_sharegroup_member_token_update(
        {
            "sharegroup_id": sharegroup_id,
            "token_uuid": token_uuid,
            "label": "renamed-member",
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_image_sharegroup_member_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_label", [None, "", "   ", 123, True])
async def test_member_token_update_rejects_invalid_label(
    bad_label: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler requires a non-empty label before the client call."""
    result = await handle_linode_images_sharegroup_member_token_update(
        {
            "sharegroup_id": "22222222-2222-4222-8222-222222222222",
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "label": bad_label,
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text.startswith("Error: ")
    assert "label" in result[0].text
    mock_linode_client.update_image_sharegroup_member_token.assert_not_called()


@pytest.mark.asyncio
async def test_image_sharegroup_member_token_update_dry_run_requires_confirm(
    sample_config: Any,
) -> None:
    """Dry-run still requires confirm because the tool schema requires it."""
    result = await handle_linode_images_sharegroup_member_token_update(
        {
            "sharegroup_id": "22222222-2222-4222-8222-222222222222",
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "label": "renamed-member",
            "dry_run": True,
        },
        sample_config,
    )

    assert "confirm=true" in result[0].text


@pytest.mark.asyncio
async def test_image_sharegroup_member_token_update_dry_run_returns_encoded_preview(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """dry_run=true previews member token update without calling the client."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroup_member_token_update(
        {
            "sharegroup_id": sharegroup_id,
            "token_uuid": token_uuid,
            "label": "renamed-member",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_images_sharegroup_member_token_update"
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == (
        f"/images/sharegroups/{sharegroup_id}/members/{token_uuid}"
    )
    assert body["would_execute"]["body"] == {"label": "renamed-member"}
    mock_linode_client.update_image_sharegroup_member_token.assert_not_called()


def test_linode_images_sharegroup_member_token_update_registered() -> None:
    """Dynamic registry exports the member token update tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_member_token_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_images_sharegroup_member_token_update"
    assert entry.handle_fn is handle_linode_images_sharegroup_member_token_update


def test_linode_images_sharegroup_member_token_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes(
        "linode_images_sharegroup_member_token_update", Capability.Write
    )

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_member_token_update_in_version_features() -> None:
    """Version metadata advertises the member token update tool."""
    assert "linode_images_sharegroup_member_token_update" in FEATURE_TOOLS_LIST.split(
        ","
    )


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_member_token_sends_exact_path() -> None:
    """Low-level client sends DELETE /images/sharegroups/{id}/members/{tokenUuid}."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_member_token(sharegroup_id, token_uuid)
    finally:
        await client.close()

    assert len(seen) == 1
    request = seen[0]
    assert request.method == "DELETE"
    assert request.url.path == (
        f"/v4/images/sharegroups/{sharegroup_id}/members/{token_uuid}"
    )
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_member_token_encodes_path_params() -> (
    None
):
    """Low-level client URL-encodes both path params at the boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_member_token(
            "sharegroup/with?separator", "token/with?separator"
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator"
        b"/members/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_member_token_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="DeleteImageSharegroupMemberToken"):
            await client.delete_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_member_token_delete_delegates_once() -> None:
    """Destructive member token revoke should not replay after errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_delete = AsyncMock(side_effect=httpx.HTTPError("temporary"))
    cast("Any", retryable.client).delete_image_sharegroup_member_token = mock_delete

    try:
        with pytest.raises(httpx.HTTPError):
            await retryable.delete_image_sharegroup_member_token(
                "22222222-2222-4222-8222-222222222222",
                "11111111-1111-4111-8111-111111111111",
            )
    finally:
        await retryable.close()

    mock_delete.assert_awaited_once_with(
        "22222222-2222-4222-8222-222222222222",
        "11111111-1111-4111-8111-111111111111",
    )


def test_create_linode_images_sharegroup_member_token_delete_tool_schema() -> None:
    """Tool schema requires both path params, confirm, and dry_run."""
    tool, capability = create_linode_images_sharegroup_member_token_delete_tool()

    assert tool.name == "linode_images_sharegroup_member_token_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == [
        "sharegroup_id",
        "token_uuid",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert (
        "[0-9a-fA-F]{8}" in tool.inputSchema["properties"]["sharegroup_id"]["pattern"]
    )
    assert "[0-9a-fA-F]{8}" in tool.inputSchema["properties"]["token_uuid"]["pattern"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler revokes a member token through the client."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroup_member_token_delete(
        {
            "sharegroup_id": f" {sharegroup_id} ",
            "token_uuid": f" {token_uuid} ",
            "confirm": True,
        },
        sample_config,
    )

    assert json.loads(result[0].text) == {
        "message": "Image share group member token revoked"
    }
    mock_linode_client.delete_image_sharegroup_member_token.assert_awaited_once_with(
        sharegroup_id, token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_member_token_delete_requires_true_confirm(
    bad_confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects non-true confirm values before the client call."""
    arguments: dict[str, Any] = {
        "sharegroup_id": "22222222-2222-4222-8222-222222222222",
        "token_uuid": "11111111-1111-4111-8111-111111111111",
    }
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    result = await handle_linode_images_sharegroup_member_token_delete(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    assert "confirm=true" in result[0].text
    mock_linode_client.delete_image_sharegroup_member_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("sharegroup_id", "token_uuid"),
    [
        (None, "11111111-1111-4111-8111-111111111111"),
        ("", "11111111-1111-4111-8111-111111111111"),
        ("not-a-uuid", "11111111-1111-4111-8111-111111111111"),
        (
            "22222222/2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        (
            "22222222?2222-4222-8222-222222222222",
            "11111111-1111-4111-8111-111111111111",
        ),
        ("..", "11111111-1111-4111-8111-111111111111"),
        (123, "11111111-1111-4111-8111-111111111111"),
        ("22222222-2222-4222-8222-222222222222", None),
        ("22222222-2222-4222-8222-222222222222", ""),
        ("22222222-2222-4222-8222-222222222222", "not-a-uuid"),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111/1111-4111-8111-111111111111",
        ),
        (
            "22222222-2222-4222-8222-222222222222",
            "11111111?1111-4111-8111-111111111111",
        ),
        ("22222222-2222-4222-8222-222222222222", ".."),
        ("22222222-2222-4222-8222-222222222222", 123),
    ],
)
async def test_member_token_delete_rejects_invalid_path_params(
    sharegroup_id: Any,
    token_uuid: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before the client call."""
    result = await handle_linode_images_sharegroup_member_token_delete(
        {
            "sharegroup_id": sharegroup_id,
            "token_uuid": token_uuid,
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.delete_image_sharegroup_member_token.assert_not_called()


@pytest.mark.asyncio
async def test_image_sharegroup_member_token_delete_dry_run_requires_confirm(
    sample_config: Any,
) -> None:
    """Dry-run still requires confirm because the tool schema requires it."""
    result = await handle_linode_images_sharegroup_member_token_delete(
        {
            "sharegroup_id": "22222222-2222-4222-8222-222222222222",
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "dry_run": True,
        },
        sample_config,
    )

    assert "confirm=true" in result[0].text


@pytest.mark.asyncio
async def test_image_sharegroup_member_token_delete_dry_run_returns_encoded_preview(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """dry_run=true previews member token revoke without calling the client."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroup_member_token_delete(
        {
            "sharegroup_id": sharegroup_id,
            "token_uuid": token_uuid,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_images_sharegroup_member_token_delete"
    assert body["would_execute"] == {
        "method": "DELETE",
        "path": f"/images/sharegroups/{sharegroup_id}/members/{token_uuid}",
    }
    mock_linode_client.delete_image_sharegroup_member_token.assert_not_called()


def test_linode_images_sharegroup_member_token_delete_registered() -> None:
    """Dynamic registry exports the member token delete tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_member_token_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_images_sharegroup_member_token_delete"
    assert entry.handle_fn is handle_linode_images_sharegroup_member_token_delete


def test_linode_images_sharegroup_member_token_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes(
        "linode_images_sharegroup_member_token_delete", Capability.Destroy
    )

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_member_token_delete_in_version_features() -> None:
    """Version metadata advertises the member token delete tool."""
    assert "linode_images_sharegroup_member_token_delete" in FEATURE_TOOLS_LIST.split(
        ","
    )


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_images_sends_exact_encoded_path() -> None:
    """Low-level client sends GET /images/sharegroups/{sharegroupId}/images."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroup_images(sharegroup_id)
    finally:
        await client.close()

    assert result["data"][0]["id"] == "private/ubuntu"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == f"/v4/images/sharegroups/{sharegroup_id}/images"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_images_encodes_path_param() -> None:
    """Low-level client URL-encodes sharegroup_id before appending /images."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.list_image_sharegroup_images("sharegroup/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator/images"
    )


@pytest.mark.asyncio
async def test_retryable_list_image_sharegroup_images_uses_read_retry() -> None:
    """Read-only images by share group list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_list = AsyncMock(return_value={"data": [{"id": "private/ubuntu"}]})
    cast("Any", retryable.client).list_image_sharegroup_images = mock_list

    try:
        result = await retryable.list_image_sharegroup_images(sharegroup_id)
    finally:
        await retryable.close()

    assert result["data"][0]["id"] == "private/ubuntu"
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(sharegroup_id)


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_image_sends_exact_path() -> None:
    """Low-level client sends DELETE to the documented share-group image path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_image("123", "456")
    finally:
        await client.close()

    assert len(seen) == 1
    request = seen[0]
    assert request.method == "DELETE"
    assert request.url.raw_path == b"/v4/images/sharegroups/123/images/456"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_image_encodes_path_params() -> None:
    """Low-level client URL-encodes both path params at the boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_image("12/../?x=1", "34?y=2")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/12%2F..%2F%3Fx%3D1/images/34%3Fy%3D2"
    )


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_image_maps_http_error() -> None:
    """Low-level client maps HTTP failures to NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="DeleteImageSharegroupImage"):
            await client.delete_image_sharegroup_image("123", "456")
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_delete_image_sharegroup_image_delegates_once() -> None:
    """Destructive image revocation delegates once without retry replay."""
    retryable = _CapturingRetryableClient()
    mock_delete = AsyncMock(return_value=None)
    cast("Any", retryable.client).delete_image_sharegroup_image = mock_delete

    try:
        await retryable.delete_image_sharegroup_image("123", "456")
    finally:
        await retryable.close()

    assert retryable.calls == []
    mock_delete.assert_awaited_once_with("123", "456")


def test_create_linode_images_sharegroup_image_delete_tool_schema() -> None:
    """Delete-image tool schema exposes path params, confirm, and dry_run."""
    tool, capability = create_linode_images_sharegroup_image_delete_tool()

    assert tool.name == "linode_images_sharegroup_image_delete"
    assert capability is Capability.Destroy
    schema = tool.inputSchema
    assert schema["required"] == ["sharegroup_id", "image_id", "confirm"]
    assert schema["properties"]["sharegroup_id"]["type"] == "integer"
    assert schema["properties"]["sharegroup_id"]["minimum"] == 1
    assert schema["properties"]["image_id"]["type"] == "integer"
    assert schema["properties"]["image_id"]["minimum"] == 1
    assert schema["properties"]["confirm"]["type"] == "boolean"
    assert schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler revokes one shared image and returns a success message."""
    result = await handle_linode_images_sharegroup_image_delete(
        {"sharegroup_id": 123, "image_id": 456, "confirm": True},
        sample_config,
    )

    assert json.loads(result[0].text) == {"message": "Shared image access revoked"}
    mock_linode_client.delete_image_sharegroup_image.assert_awaited_once_with(
        "123", "456"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroup_image_delete_rejects_non_true_confirm(
    sample_config: Any, mock_linode_client: AsyncMock, confirm: object
) -> None:
    """Handler requires literal confirm=True before client calls."""
    arguments: dict[str, object] = {"sharegroup_id": 123, "image_id": 456}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_images_sharegroup_image_delete(
        arguments, sample_config
    )

    assert "Set confirm=true" in result[0].text
    mock_linode_client.delete_image_sharegroup_image.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        (
            {"image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": "", "image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 0, "image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": True, "image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 123, "confirm": True},
            "image_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 123, "image_id": "456", "confirm": True},
            "image_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 123, "image_id": 0, "confirm": True},
            "image_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 123, "image_id": False, "confirm": True},
            "image_id must be a positive integer",
        ),
    ],
)
async def test_handle_linode_images_sharegroup_image_delete_rejects_invalid_path_params(
    sample_config: Any,
    mock_linode_client: AsyncMock,
    arguments: dict[str, object],
    expected_error: str,
) -> None:
    """Handler rejects malformed path params before client calls."""
    result = await handle_linode_images_sharegroup_image_delete(
        arguments, sample_config
    )

    assert expected_error in result[0].text
    mock_linode_client.delete_image_sharegroup_image.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_delete_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry run previews the destructive request without a client call."""
    result = await handle_linode_images_sharegroup_image_delete(
        {"sharegroup_id": 123, "image_id": 456, "confirm": True, "dry_run": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_images_sharegroup_image_delete"
    assert payload["would_execute"] == {
        "method": "DELETE",
        "path": "/images/sharegroups/123/images/456",
    }
    mock_linode_client.delete_image_sharegroup_image.assert_not_called()


def test_linode_images_sharegroup_image_delete_registered() -> None:
    """Dynamic registry exports the delete-image tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_image_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_images_sharegroup_image_delete"
    assert entry.handle_fn is handle_linode_images_sharegroup_image_delete


def test_linode_images_sharegroup_image_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes(
        "linode_images_sharegroup_image_delete", Capability.Destroy
    )

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_image_delete_in_version_features() -> None:
    """Version metadata advertises the delete-image tool."""
    assert "linode_images_sharegroup_image_delete" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_add_image_sharegroup_images_sends_exact_path_and_body() -> None:
    """Low-level client sends POST /images/sharegroups/{sharegroupId}/images."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    images = [
        {"id": "private/ubuntu", "label": "Private Ubuntu", "description": "Ubuntu"}
    ]

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"images": images})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.add_image_sharegroup_images(sharegroup_id, images)
    finally:
        await client.close()

    assert result == {"images": images}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == f"/v4/images/sharegroups/{sharegroup_id}/images"
    assert request.url.query == b""
    assert json.loads((await request.aread()).decode()) == {"images": images}
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_add_image_sharegroup_images_encodes_path_param() -> None:
    """Low-level client URL-encodes sharegroup_id before appending /images."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"images": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.add_image_sharegroup_images(
            "sharegroup/with?separator", [{"id": "private/ubuntu"}]
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator/images"
    )


@pytest.mark.asyncio
async def test_client_add_image_sharegroup_images_rejects_empty_images() -> None:
    """Low-level client rejects an empty required body before the request."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="images must be a non-empty"):
            await client.add_image_sharegroup_images(
                "22222222-2222-4222-8222-222222222222", []
            )
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_add_image_sharegroup_images_maps_http_error() -> None:
    """Low-level client maps HTTP transport failures."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary failure", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="AddImageSharegroupImages"):
            await client.add_image_sharegroup_images(
                "22222222-2222-4222-8222-222222222222",
                [{"id": "private/ubuntu"}],
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_add_image_sharegroup_images_delegates_once() -> None:
    """Mutating add-images route delegates once without retry replay."""
    retryable = _CapturingRetryableClient()
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    images = [{"id": "private/ubuntu"}]
    mock_add = AsyncMock(return_value={"images": images})
    cast("Any", retryable.client).add_image_sharegroup_images = mock_add

    try:
        result = await retryable.add_image_sharegroup_images(sharegroup_id, images)
    finally:
        await retryable.close()

    assert result == {"images": images}
    assert retryable.calls == []
    mock_add.assert_awaited_once_with(sharegroup_id, images)


def test_create_linode_images_sharegroup_images_add_tool_schema() -> None:
    """Tool schema requires UUID path, images body, and confirm."""
    tool, capability = create_linode_images_sharegroup_images_add_tool()

    assert tool.name == "linode_images_sharegroup_images_add"
    assert capability is Capability.Write
    assert set(tool.inputSchema["properties"]) == {
        "environment",
        "sharegroup_id",
        "images",
        "confirm",
        "dry_run",
    }
    assert tool.inputSchema["required"] == ["sharegroup_id", "images", "confirm"]
    assert tool.inputSchema["properties"]["images"]["minItems"] == 1
    assert (
        "[0-9a-fA-F]{8}" in tool.inputSchema["properties"]["sharegroup_id"]["pattern"]
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_images_add_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler adds images to a share group."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    images = [{"id": "private/ubuntu", "label": "Private Ubuntu"}]
    mock_linode_client.add_image_sharegroup_images.return_value = {"images": images}

    result = await handle_linode_images_sharegroup_images_add(
        {"sharegroup_id": f" {sharegroup_id} ", "images": images, "confirm": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Images added to image share group",
        "result": {"images": images},
    }
    mock_linode_client.add_image_sharegroup_images.assert_awaited_once_with(
        sharegroup_id, images
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroup_images_add_requires_literal_confirm(
    confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects omitted and non-literal confirm before the client call."""
    arguments: dict[str, Any] = {
        "sharegroup_id": "22222222-2222-4222-8222-222222222222",
        "images": [{"id": "private/ubuntu"}],
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_images_sharegroup_images_add(arguments, sample_config)

    assert result[0].text.startswith("Error: This adds images")
    mock_linode_client.add_image_sharegroup_images.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 123},
    ],
)
async def test_handle_linode_images_sharegroup_images_add_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    arguments = {**arguments, "images": [{"id": "private/ubuntu"}], "confirm": True}

    result = await handle_linode_images_sharegroup_images_add(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.add_image_sharegroup_images.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("images", "message"), INVALID_ADD_IMAGE_SHAREGROUP_IMAGES_CASES
)
async def test_handle_linode_images_sharegroup_images_add_rejects_invalid_body(
    images: object, message: str, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid images payloads before confirm/client calls."""
    result = await handle_linode_images_sharegroup_images_add(
        {
            "sharegroup_id": "22222222-2222-4222-8222-222222222222",
            "images": images,
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text == f"Error: {message}"
    mock_linode_client.add_image_sharegroup_images.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_images_add_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry run previews the encoded mutating request without client call."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    images = [{"id": "private/ubuntu"}]

    result = await handle_linode_images_sharegroup_images_add(
        {
            "sharegroup_id": sharegroup_id,
            "images": images,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_images_sharegroup_images_add"
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == (
        f"/images/sharegroups/{sharegroup_id}/images"
    )
    assert payload["would_execute"]["body"] == {"images": images}
    mock_linode_client.add_image_sharegroup_images.assert_not_called()


def test_linode_images_sharegroup_images_add_registered() -> None:
    """Dynamic registry exports the add-images tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_images_add"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_images_sharegroup_images_add"
    assert entry.handle_fn is handle_linode_images_sharegroup_images_add


def test_linode_images_sharegroup_images_add_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes("linode_images_sharegroup_images_add", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_images_add_in_version_features() -> None:
    """Version metadata advertises the add-images tool."""
    assert "linode_images_sharegroup_images_add" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_add_members_exact_path_and_body() -> None:
    """Low-level client sends POST /images/sharegroups/{sharegroupId}/members."""
    seen: list[httpx.Request] = []
    sharegroup_id = "22222222-2222-4222-8222-222222222222"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"member": {"label": "team-a"}})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.add_members_to_image_sharegroup(
            sharegroup_id, label="team-a", token="share-token"
        )
    finally:
        await client.close()

    assert result == {"member": {"label": "team-a"}}
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == f"/v4/images/sharegroups/{sharegroup_id}/members"
    assert request.url.query == b""
    assert json.loads((await request.aread()).decode()) == {
        "label": "team-a",
        "token": "share-token",
    }
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_add_members_to_image_sharegroup_encodes_path_param() -> None:
    """Low-level client URL-encodes sharegroup_id before appending /members."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.add_members_to_image_sharegroup(
            "sharegroup/with?separator", label="team-a", token="share-token"
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/sharegroup%2Fwith%3Fseparator/members"
    )


@pytest.mark.asyncio
async def test_client_add_members_rejects_empty_body_fields() -> None:
    """Low-level client rejects empty required body fields before the request."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="label must be a non-empty string"):
            await client.add_members_to_image_sharegroup(
                "22222222-2222-4222-8222-222222222222", label="", token="share-token"
            )
        with pytest.raises(ValueError, match="token must be a non-empty string"):
            await client.add_members_to_image_sharegroup(
                "22222222-2222-4222-8222-222222222222", label="team-a", token=""
            )
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_add_members_to_image_sharegroup_maps_http_error() -> None:
    """Low-level client maps HTTP transport failures."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary failure", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="AddMembersToImageSharegroup"):
            await client.add_members_to_image_sharegroup(
                "22222222-2222-4222-8222-222222222222",
                label="team-a",
                token="share-token",
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_add_members_to_image_sharegroup_delegates_once() -> None:
    """Mutating add-members route delegates once without retry replay."""
    retryable = _CapturingRetryableClient()
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_add = AsyncMock(return_value={"member": {"label": "team-a"}})
    cast("Any", retryable.client).add_members_to_image_sharegroup = mock_add

    try:
        result = await retryable.add_members_to_image_sharegroup(
            sharegroup_id, label="team-a", token="share-token"
        )
    finally:
        await retryable.close()

    assert result == {"member": {"label": "team-a"}}
    assert retryable.calls == []
    mock_add.assert_awaited_once_with(
        sharegroup_id, label="team-a", token="share-token"
    )


def test_create_linode_images_sharegroup_members_add_tool_schema() -> None:
    """Tool schema requires UUID path, label/token body, and confirm."""
    tool, capability = create_linode_images_sharegroup_members_add_tool()

    assert tool.name == "linode_images_sharegroup_members_add"
    assert capability is Capability.Write
    assert set(tool.inputSchema["properties"]) == {
        "environment",
        "sharegroup_id",
        "label",
        "token",
        "confirm",
        "dry_run",
    }
    assert tool.inputSchema["required"] == [
        "sharegroup_id",
        "label",
        "token",
        "confirm",
    ]
    sharegroup_pattern = tool.inputSchema["properties"]["sharegroup_id"]["pattern"]
    assert "[0-9a-fA-F]{8}" in sharegroup_pattern


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_add_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler adds members to a share group."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_linode_client.add_members_to_image_sharegroup.return_value = {
        "member": {"label": "team-a"}
    }

    result = await handle_linode_images_sharegroup_members_add(
        {
            "sharegroup_id": f" {sharegroup_id} ",
            "label": "team-a",
            "token": "share-token",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Members added to image share group",
        "result": {"member": {"label": "team-a"}},
    }
    mock_linode_client.add_members_to_image_sharegroup.assert_awaited_once_with(
        sharegroup_id, label="team-a", token="share-token"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroup_members_add_requires_literal_confirm(
    confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects omitted and non-literal confirm before the client call."""
    arguments: dict[str, Any] = {
        "sharegroup_id": "22222222-2222-4222-8222-222222222222",
        "label": "team-a",
        "token": "share-token",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_images_sharegroup_members_add(arguments, sample_config)

    assert result[0].text.startswith("Error: This adds members")
    mock_linode_client.add_members_to_image_sharegroup.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 123},
    ],
)
async def test_handle_linode_images_sharegroup_members_add_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    arguments = {
        **arguments,
        "label": "team-a",
        "token": "share-token",
        "confirm": True,
    }

    result = await handle_linode_images_sharegroup_members_add(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.add_members_to_image_sharegroup.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"token": "share-token"}, "label must be a non-empty string"),
        ({"label": "", "token": "share-token"}, "label must be a non-empty string"),
        ({"label": 1, "token": "share-token"}, "label must be a non-empty string"),
        ({"label": "team-a"}, "token must be a non-empty string"),
        ({"label": "team-a", "token": ""}, "token must be a non-empty string"),
        ({"label": "team-a", "token": 1}, "token must be a non-empty string"),
    ],
)
async def test_handle_linode_images_sharegroup_members_add_rejects_invalid_body(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects invalid label/token body fields before client calls."""
    result = await handle_linode_images_sharegroup_members_add(
        {
            "sharegroup_id": "22222222-2222-4222-8222-222222222222",
            "confirm": True,
            **arguments,
        },
        sample_config,
    )

    assert result[0].text == f"Error: {message}"
    mock_linode_client.add_members_to_image_sharegroup.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_add_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry run previews the encoded mutating request without client call."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"

    result = await handle_linode_images_sharegroup_members_add(
        {
            "sharegroup_id": sharegroup_id,
            "label": "team-a",
            "token": "share-token",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_images_sharegroup_members_add"
    assert payload["would_execute"]["method"] == "POST"
    assert payload["would_execute"]["path"] == (
        f"/images/sharegroups/{sharegroup_id}/members"
    )
    assert payload["would_execute"]["body"] == {
        "label": "team-a",
        "token": "share-token",
    }
    mock_linode_client.add_members_to_image_sharegroup.assert_not_called()


def test_linode_images_sharegroup_members_add_registered() -> None:
    """Dynamic registry exports the add-members tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_members_add"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_images_sharegroup_members_add"
    assert entry.handle_fn is handle_linode_images_sharegroup_members_add


def test_linode_images_sharegroup_members_add_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes("linode_images_sharegroup_members_add", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_members_add_in_version_features() -> None:
    """Version metadata advertises the add-members tool."""
    assert "linode_images_sharegroup_members_add" in FEATURE_TOOLS_LIST.split(",")


def test_create_linode_images_sharegroup_images_list_tool_schema() -> None:
    """Tool schema requires the documented sharegroup UUID path param."""
    tool, capability = create_linode_images_sharegroup_images_list_tool()

    assert tool.name == "linode_images_sharegroup_images_list"
    assert capability is Capability.Read
    sharegroup_id_schema = tool.inputSchema["properties"]["sharegroup_id"]
    assert set(tool.inputSchema["properties"]) == {"environment", "sharegroup_id"}
    assert tool.inputSchema["required"] == ["sharegroup_id"]
    assert "[0-9a-fA-F]{8}" in sharegroup_id_schema["pattern"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_images_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns images associated with a share group."""
    sharegroup_id = "22222222-2222-4222-8222-222222222222"
    mock_linode_client.list_image_sharegroup_images.return_value = {
        "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_images_sharegroup_images_list(
        {"sharegroup_id": f" {sharegroup_id} "}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group images retrieved",
        "count": 1,
        "images": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_linode_client.list_image_sharegroup_images.assert_awaited_once_with(
        sharegroup_id
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 123},
    ],
)
async def test_handle_linode_images_sharegroup_images_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    result = await handle_linode_images_sharegroup_images_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_image_sharegroup_images.assert_not_called()


def test_linode_images_sharegroup_images_list_registered() -> None:
    """Dynamic registry exports the images by share group tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_images_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroup_images_list"
    assert entry.handle_fn is handle_linode_images_sharegroup_images_list


def test_linode_images_sharegroup_images_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroup_images_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroup_images_list_in_version_features() -> None:
    """Version metadata advertises the images by share group tool."""
    assert "linode_images_sharegroup_images_list" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_token_sends_exact_path_and_body() -> None:
    """Low-level client sends PUT /images/sharegroups/tokens/{tokenUuid}."""
    seen: list[httpx.Request] = []
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "id": "sharegroup-record-1",
                "token_uuid": token_uuid,
                "label": "renamed-token",
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_image_sharegroup_token(
            token_uuid=token_uuid, label="renamed-token"
        )
    finally:
        await client.close()

    assert result["label"] == "renamed-token"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == f"/v4/images/sharegroups/tokens/{token_uuid}"
    assert request.url.query == b""
    assert json.loads((await request.aread()).decode()) == {"label": "renamed-token"}
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_token_encodes_path_param() -> None:
    """Low-level client URL-encodes token_uuid at the path boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "encoded", "label": "renamed"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.update_image_sharegroup_token(
            token_uuid="token/with?separator", label="renamed"
        )
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/tokens/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
async def test_retryable_client_update_image_sharegroup_token_delegates_once() -> None:
    """Retryable update wrapper should not replay token updates after errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_update = AsyncMock(side_effect=httpx.HTTPError("temporary"))
    cast("Any", retryable.client).update_image_sharegroup_token = mock_update

    try:
        with pytest.raises(httpx.HTTPError):
            await retryable.update_image_sharegroup_token(
                token_uuid="11111111-1111-4111-8111-111111111111",
                label="renamed-token",
            )
    finally:
        await retryable.close()

    mock_update.assert_awaited_once_with(
        token_uuid="11111111-1111-4111-8111-111111111111",
        label="renamed-token",
    )


def test_create_linode_images_sharegroups_token_update_tool_schema() -> None:
    """Tool schema requires token UUID, label, and confirm."""
    tool, capability = create_linode_images_sharegroups_token_update_tool()

    assert tool.name == "linode_images_sharegroups_token_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["token_uuid", "label", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler updates a token label through the client."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.update_image_sharegroup_token.return_value = {
        "id": "sharegroup-record-1",
        "token_uuid": token_uuid,
        "label": "renamed-token",
    }

    result = await handle_linode_images_sharegroups_token_update(
        {"token_uuid": f" {token_uuid} ", "label": " renamed-token ", "confirm": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group token updated",
        "token": {
            "id": "sharegroup-record-1",
            "token_uuid": token_uuid,
            "label": "renamed-token",
        },
    }
    mock_linode_client.update_image_sharegroup_token.assert_awaited_once_with(
        token_uuid=token_uuid, label="renamed-token"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroups_token_update_requires_true_confirm(
    bad_confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects non-true confirm values before the client call."""
    arguments: dict[str, Any] = {
        "token_uuid": "11111111-1111-4111-8111-111111111111",
        "label": "renamed-token",
    }
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    result = await handle_linode_images_sharegroups_token_update(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    assert "confirm=true" in result[0].text
    mock_linode_client.update_image_sharegroup_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "bad_uuid",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_linode_images_sharegroups_token_update_rejects_invalid_token_uuid(
    bad_uuid: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    arguments = {"label": "renamed-token", "confirm": True, **bad_uuid}

    result = await handle_linode_images_sharegroups_token_update(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_image_sharegroup_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_label", [None, "", "   ", 123, True])
async def test_handle_linode_images_sharegroups_token_update_rejects_invalid_label(
    bad_label: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler requires a non-empty label before the client call."""
    result = await handle_linode_images_sharegroups_token_update(
        {
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "label": bad_label,
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text.startswith("Error: ")
    assert "label" in result[0].text
    mock_linode_client.update_image_sharegroup_token.assert_not_called()


@pytest.mark.asyncio
async def test_image_sharegroup_token_update_dry_run_requires_confirm(
    sample_config: Any,
) -> None:
    """Dry-run still requires confirm because the tool schema requires it."""
    result = await handle_linode_images_sharegroups_token_update(
        {
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "label": "renamed-token",
            "dry_run": True,
        },
        sample_config,
    )

    assert "confirm=true" in result[0].text


@pytest.mark.asyncio
async def test_image_sharegroup_token_update_dry_run_returns_encoded_preview(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """dry_run=true previews token update without calling the client."""
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroups_token_update(
        {
            "token_uuid": token_uuid,
            "label": "renamed-token",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_images_sharegroups_token_update"
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == f"/images/sharegroups/tokens/{token_uuid}"
    assert body["would_execute"]["body"] == {"label": "renamed-token"}
    mock_linode_client.update_image_sharegroup_token.assert_not_called()


def test_linode_images_sharegroups_token_update_registered() -> None:
    """Dynamic registry exports the token update tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_token_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_images_sharegroups_token_update"
    assert entry.handle_fn is handle_linode_images_sharegroups_token_update


def test_linode_images_sharegroups_token_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes("linode_images_sharegroups_token_update", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroups_token_update_in_version_features() -> None:
    """Version metadata advertises the token update tool."""
    assert "linode_images_sharegroups_token_update" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_token_sends_exact_path() -> None:
    """Low-level client sends DELETE /images/sharegroups/tokens/{tokenUuid}."""
    seen: list[httpx.Request] = []
    token_uuid = "11111111-1111-4111-8111-111111111111"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_token(token_uuid=token_uuid)
    finally:
        await client.close()

    assert len(seen) == 1
    request = seen[0]
    assert request.method == "DELETE"
    assert request.url.path == f"/v4/images/sharegroups/tokens/{token_uuid}"
    assert request.url.query == b""
    assert (await request.aread()) == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_client_delete_image_sharegroup_token_encodes_path_param() -> None:
    """Low-level client URL-encodes token_uuid at the path boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_image_sharegroup_token(token_uuid="token/with?separator")
    finally:
        await client.close()

    assert seen[0].url.raw_path == (
        b"/v4/images/sharegroups/tokens/token%2Fwith%3Fseparator"
    )


@pytest.mark.asyncio
async def test_retryable_client_delete_image_sharegroup_token_delegates_once() -> None:
    """Retryable delete wrapper should not replay deletes after errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_delete = AsyncMock(side_effect=httpx.HTTPError("temporary"))
    cast("Any", retryable.client).delete_image_sharegroup_token = mock_delete

    try:
        with pytest.raises(httpx.HTTPError):
            await retryable.delete_image_sharegroup_token(
                token_uuid="11111111-1111-4111-8111-111111111111"
            )
    finally:
        await retryable.close()

    mock_delete.assert_awaited_once_with(
        token_uuid="11111111-1111-4111-8111-111111111111"
    )


def test_create_linode_images_sharegroups_token_delete_tool_schema() -> None:
    """Tool schema requires token UUID and confirm."""
    tool, capability = create_linode_images_sharegroups_token_delete_tool()

    assert tool.name == "linode_images_sharegroups_token_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == ["token_uuid", "confirm"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler deletes a token through the client."""
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroups_token_delete(
        {"token_uuid": f" {token_uuid} ", "confirm": True}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {"message": "Image share group token deleted"}
    mock_linode_client.delete_image_sharegroup_token.assert_awaited_once_with(
        token_uuid=token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_handle_linode_images_sharegroups_token_delete_requires_true_confirm(
    bad_confirm: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects non-true confirm values before the client call."""
    arguments: dict[str, Any] = {
        "token_uuid": "11111111-1111-4111-8111-111111111111",
    }
    if bad_confirm is not None:
        arguments["confirm"] = bad_confirm

    result = await handle_linode_images_sharegroups_token_delete(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    assert "confirm=true" in result[0].text
    mock_linode_client.delete_image_sharegroup_token.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "bad_uuid",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_linode_images_sharegroups_token_delete_rejects_invalid_token_uuid(
    bad_uuid: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    arguments = {"confirm": True, **bad_uuid}

    result = await handle_linode_images_sharegroups_token_delete(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.delete_image_sharegroup_token.assert_not_called()


@pytest.mark.asyncio
async def test_image_sharegroup_token_delete_dry_run_requires_confirm(
    sample_config: Any,
) -> None:
    """Dry-run still requires confirm because the tool schema requires it."""
    result = await handle_linode_images_sharegroups_token_delete(
        {
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "dry_run": True,
        },
        sample_config,
    )

    assert "confirm=true" in result[0].text


@pytest.mark.asyncio
async def test_image_sharegroup_token_delete_dry_run_returns_encoded_preview(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """dry_run=true previews token delete without calling the client."""
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_images_sharegroups_token_delete(
        {
            "token_uuid": token_uuid,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["tool"] == "linode_images_sharegroups_token_delete"
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == f"/images/sharegroups/tokens/{token_uuid}"
    mock_linode_client.delete_image_sharegroup_token.assert_not_called()


def test_linode_images_sharegroups_token_delete_registered() -> None:
    """Dynamic registry exports the token delete tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_token_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_images_sharegroups_token_delete"
    assert entry.handle_fn is handle_linode_images_sharegroups_token_delete


def test_linode_images_sharegroups_token_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes(
        "linode_images_sharegroups_token_delete", Capability.Destroy
    )

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroups_token_delete_in_version_features() -> None:
    """Version metadata advertises the token delete tool."""
    assert "linode_images_sharegroups_token_delete" in FEATURE_TOOLS_LIST.split(",")


def test_create_linode_images_sharegroups_tokens_list_tool_schema() -> None:
    """Tool schema exposes only the documented environment argument."""
    tool, capability = create_linode_images_sharegroups_tokens_list_tool()

    assert tool.name == "linode_images_sharegroups_tokens_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment"}
    assert "required" not in tool.inputSchema


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_tokens_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share group tokens and pagination metadata."""
    mock_linode_client.list_image_sharegroup_tokens.return_value = {
        "data": [
            {
                "id": "sharegroup-record-1",
                "created": "2026-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_images_sharegroups_tokens_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group tokens listed",
        "count": 1,
        "tokens": [
            {
                "id": "sharegroup-record-1",
                "created": "2026-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_linode_client.list_image_sharegroup_tokens.assert_awaited_once_with()


def test_linode_images_sharegroups_tokens_list_registered() -> None:
    """Dynamic registry exports the token list tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_tokens_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_tokens_list"
    assert entry.handle_fn is handle_linode_images_sharegroups_tokens_list


def test_linode_images_sharegroups_tokens_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroups_tokens_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroups_tokens_list_in_version_features() -> None:
    """Version metadata advertises the token list tool."""
    assert "linode_images_sharegroups_tokens_list" in FEATURE_TOOLS_LIST.split(",")


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_image_sends_exact_path_and_body() -> None:
    """Low-level client sends PUT to the documented shared-image route."""
    seen: list[httpx.Request] = []
    sharegroup_id = "123"
    image_id = "1234"

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": str(image_id), "label": "new-label"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_image_sharegroup_image(
            sharegroup_id, image_id, label="new-label", description="new description"
        )
    finally:
        await client.close()

    assert result == {"id": str(image_id), "label": "new-label"}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.raw_path.decode() == ("/v4/images/sharegroups/123/images/1234")
    assert json.loads(request.content) == {
        "label": "new-label",
        "description": "new description",
    }


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_image_encodes_path_params() -> None:
    """Low-level client URL-encodes both path params at the boundary."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "shared-image"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.update_image_sharegroup_image(
            "group/one", "image?one", label="new-label"
        )
    finally:
        await client.close()

    assert (
        seen[0].url.raw_path.decode()
        == "/v4/images/sharegroups/group%2Fone/images/image%3Fone"
    )


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_image_rejects_empty_body() -> None:
    """Empty update bodies are rejected before HTTP."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(
            ValueError, match="at least one of label or description must be provided"
        ):
            await client.update_image_sharegroup_image("sharegroup", "image")
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_image_rejects_empty_strings() -> None:
    """Low-level client rejects weak body fields before HTTP."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="label must be a non-empty string"):
            await client.update_image_sharegroup_image("123", "1234", label=" ")
        with pytest.raises(ValueError, match="description must be a non-empty string"):
            await client.update_image_sharegroup_image("123", "1234", description="")
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_update_image_sharegroup_image_wraps_http_errors() -> None:
    """HTTP transport failures are mapped to route-specific NetworkError."""

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("temporary failure", request=request)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateImageSharegroupImage"):
            await client.update_image_sharegroup_image("123", "1234", label="x")
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_client_update_image_sharegroup_image_delegates_once() -> None:
    """Mutating shared-image update does not use generic retry replay."""
    retryable = _CapturingRetryableClient()
    mock_update = AsyncMock(return_value={"id": "shared-image"})
    cast("Any", retryable.client).update_image_sharegroup_image = mock_update

    try:
        result = await retryable.update_image_sharegroup_image(
            "sharegroup", "image", label="new-label"
        )
    finally:
        await retryable.close()

    assert result == {"id": "shared-image"}
    assert retryable.calls == []
    mock_update.assert_awaited_once_with(
        "sharegroup", "image", label="new-label", description=None
    )


def test_create_linode_images_sharegroup_image_update_tool_schema() -> None:
    """Tool schema exposes both path params, body fields, confirm, and dry_run."""
    tool, capability = create_linode_images_sharegroup_image_update_tool()

    assert tool.name == "linode_images_sharegroup_image_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["sharegroup_id", "image_id", "confirm"]
    assert tool.inputSchema["properties"]["sharegroup_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["image_id"]["minimum"] == 1
    assert "dry_run" in tool.inputSchema["properties"]
    assert "label" in tool.inputSchema["properties"]
    assert "description" in tool.inputSchema["properties"]
    assert "anyOf" in tool.inputSchema


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler updates one shared image and returns the response."""
    sharegroup_id = 123
    image_id = 1234
    mock_linode_client.update_image_sharegroup_image.return_value = {
        "id": str(image_id),
        "label": "new-label",
    }

    result = await handle_linode_images_sharegroup_image_update(
        {
            "sharegroup_id": sharegroup_id,
            "image_id": image_id,
            "label": " new-label ",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Shared image updated",
        "image": {"id": str(image_id), "label": "new-label"},
    }
    mock_linode_client.update_image_sharegroup_image.assert_awaited_once_with(
        str(sharegroup_id), str(image_id), label="new-label", description=None
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_update_description_only_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler accepts a description-only update body."""
    mock_linode_client.update_image_sharegroup_image.return_value = {
        "id": "1234",
        "description": "new description",
    }

    result = await handle_linode_images_sharegroup_image_update(
        {
            "sharegroup_id": 123,
            "image_id": 1234,
            "description": " new description ",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Shared image updated",
        "image": {"id": "1234", "description": "new description"},
    }
    mock_linode_client.update_image_sharegroup_image.assert_awaited_once_with(
        "123", "1234", label=None, description="new description"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_images_sharegroup_image_update_requires_literal_confirm(
    confirm_value: object, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Missing, false, string, and numeric confirm are rejected before client call."""
    arguments: dict[str, object] = {
        "sharegroup_id": 123,
        "image_id": 1234,
        "label": "new-label",
    }
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_images_sharegroup_image_update(
        arguments, sample_config
    )

    assert (
        result[0].text
        == "Error: This updates a shared image. Set confirm=true to proceed."
    )
    mock_linode_client.update_image_sharegroup_image.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"sharegroup_id": 0, "image_id": 1234},
        {"sharegroup_id": "123", "image_id": 1234},
        {"sharegroup_id": True, "image_id": 1234},
        {"sharegroup_id": 123, "image_id": 0},
        {"sharegroup_id": 123, "image_id": "1234"},
        {"sharegroup_id": 123, "image_id": True},
        {"sharegroup_id": "123/4", "image_id": 1234},
        {"sharegroup_id": "123?4", "image_id": 1234},
        {"sharegroup_id": "..", "image_id": 1234},
        {"sharegroup_id": 123, "image_id": "123/4"},
        {"sharegroup_id": 123, "image_id": "123?4"},
        {"sharegroup_id": 123, "image_id": ".."},
    ],
)
async def test_handle_linode_images_sharegroup_image_update_rejects_invalid_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed numeric share group and image IDs."""
    result = await handle_linode_images_sharegroup_image_update(
        {**arguments, "label": "new-label", "confirm": True}, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_image_sharegroup_image.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "at least one of label or description must be provided"),
        ({"label": ""}, "label must be a non-empty string when provided"),
        ({"label": 1}, "label must be a non-empty string when provided"),
        ({"description": ""}, "description must be a non-empty string when provided"),
        (
            {"description": False},
            "description must be a non-empty string when provided",
        ),
    ],
)
async def test_handle_linode_images_sharegroup_image_update_rejects_invalid_body(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects invalid body fields before confirm/client calls."""
    result = await handle_linode_images_sharegroup_image_update(
        {
            "sharegroup_id": 123,
            "image_id": 1234,
            **arguments,
            "confirm": True,
        },
        sample_config,
    )

    assert result[0].text == f"Error: {message}"
    mock_linode_client.update_image_sharegroup_image.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_update_dry_run(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry run previews the encoded mutating request without client call."""
    sharegroup_id = 123
    image_id = 1234

    result = await handle_linode_images_sharegroup_image_update(
        {
            "sharegroup_id": sharegroup_id,
            "image_id": image_id,
            "description": "new description",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["tool"] == "linode_images_sharegroup_image_update"
    assert payload["would_execute"]["method"] == "PUT"
    assert payload["would_execute"]["path"] == ("/images/sharegroups/123/images/1234")
    assert payload["would_execute"]["body"] == {"description": "new description"}
    mock_linode_client.update_image_sharegroup_image.assert_not_called()


def test_linode_images_sharegroup_image_update_registered() -> None:
    """Dynamic registry exports the shared-image update tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroup_image_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_images_sharegroup_image_update"
    assert entry.handle_fn is handle_linode_images_sharegroup_image_update


def test_linode_images_sharegroup_image_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = required_scopes("linode_images_sharegroup_image_update", Capability.Write)

    assert scopes == [Scope.ImagesReadWrite]


def test_linode_images_sharegroup_image_update_in_version_features() -> None:
    """Version metadata advertises the shared-image update tool."""
    assert "linode_images_sharegroup_image_update" in FEATURE_TOOLS_LIST.split(",")
