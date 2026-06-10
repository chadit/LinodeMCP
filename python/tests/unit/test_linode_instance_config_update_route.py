"""Tests for updating Linode instance configuration profiles."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_instances import (
    create_linode_instance_config_interface_add_tool,
    create_linode_instance_config_interface_delete_tool,
    create_linode_instance_config_update_tool,
    create_linode_instance_interface_delete_tool,
    create_linode_instance_interface_get_tool,
    create_linode_instance_interface_list_tool,
    create_linode_instance_interface_settings_get_tool,
    handle_linode_instance_config_interface_add,
    handle_linode_instance_config_interface_delete,
    handle_linode_instance_config_update,
    handle_linode_instance_interface_delete,
    handle_linode_instance_interface_get,
    handle_linode_instance_interface_list,
    handle_linode_instance_interface_settings_get,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


class _FailingRetryableClient(RetryableClient):
    """Retryable client test double that fails if replay retry is used."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.retry_calls = 0

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        del func, args
        self.retry_calls += 1
        raise AssertionError("mutating update must not use replay retry")


@pytest.mark.asyncio
async def test_client_list_instance_interfaces_sends_exact_request() -> None:
    """Low-level client sends GET for the documented instance interfaces path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": [{"id": 789}], "page": 1})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_instance_interfaces(123)
    finally:
        await client.close()

    assert result == {"data": [{"id": 789}], "page": 1}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/interfaces"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_list_instance_interfaces_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.list_instance_interfaces(linode_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_list_instance_interfaces_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ListInstanceInterfaces"):
            await client.list_instance_interfaces(123)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_list_instance_interfaces_uses_retry() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    retryable = cast("Any", client)
    retryable.client.list_instance_interfaces = AsyncMock(
        return_value={"data": [{"id": 789}], "page": 1}
    )
    retryable._execute_with_retry = AsyncMock(
        return_value={"data": [{"id": 789}], "page": 1}
    )

    result = await client.list_instance_interfaces(123)

    assert result == {"data": [{"id": 789}], "page": 1}
    retryable._execute_with_retry.assert_awaited_once_with(
        client.client.list_instance_interfaces, 123
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_interfaces_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.list_instance_interfaces.return_value = {
        "data": [{"id": 789}],
        "page": 1,
    }

    result = await handle_linode_instance_interface_list(
        {"linode_id": 123}, sample_config
    )

    assert json.loads(result[0].text) == {"data": [{"id": 789}], "page": 1}
    mock_linode_client.list_instance_interfaces.assert_awaited_once_with(123)


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_handle_linode_instance_interfaces_list_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_list(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.list_instance_interfaces.assert_not_called()


@pytest.mark.asyncio
async def test_client_get_instance_interface_settings_sends_exact_request() -> None:
    """Low-level client sends GET for the documented interface settings path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"default_route": {"ipv4": True}})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_instance_interface_settings(123)
    finally:
        await client.close()

    assert result == {"default_route": {"ipv4": True}}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/interfaces/settings"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_client_get_instance_interface_settings_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.get_instance_interface_settings(linode_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_get_instance_interface_settings_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="GetInstanceInterfaceSettings"):
            await client.get_instance_interface_settings(123)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_get_instance_interface_settings_uses_retry() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    retryable = cast("Any", client)
    retryable.client.get_instance_interface_settings = AsyncMock(
        return_value={"default_route": {"ipv4": True}}
    )
    retryable._execute_with_retry = AsyncMock(
        return_value={"default_route": {"ipv4": True}}
    )

    result = await client.get_instance_interface_settings(123)

    assert result == {"default_route": {"ipv4": True}}
    retryable._execute_with_retry.assert_awaited_once_with(
        client.client.get_instance_interface_settings, 123
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_settings_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance_interface_settings.return_value = {
        "default_route": {"ipv4": True}
    }

    result = await handle_linode_instance_interface_settings_get(
        {"linode_id": 123}, sample_config
    )

    assert json.loads(result[0].text) == {"default_route": {"ipv4": True}}
    mock_linode_client.get_instance_interface_settings.assert_awaited_once_with(123)


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_handle_linode_instance_interface_settings_get_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_settings_get(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.get_instance_interface_settings.assert_not_called()


def test_create_linode_instance_interface_settings_get_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_settings_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_settings_get"
    assert tool.inputSchema["required"] == ["linode_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1


def test_linode_instance_interface_settings_get_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_settings_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_settings_get"
    assert entry.handle_fn is handle_linode_instance_interface_settings_get
    assert "linode_instance_interface_settings_get" in FEATURE_TOOLS_LIST


def test_create_linode_instance_interfaces_list_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_list"
    assert tool.inputSchema["required"] == ["linode_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1


def test_linode_instance_interfaces_list_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_list"
    assert entry.handle_fn is handle_linode_instance_interface_list
    assert "linode_instance_interface_list" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_get_instance_interface_sends_exact_request() -> None:
    """Low-level client sends GET for the documented instance interface path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 789, "purpose": "public"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_instance_interface(123, 789)
    finally:
        await client.close()

    assert result == {"id": 789, "purpose": "public"}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/interfaces/789"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("linode_id", "interface_id"),
    [
        ("1/2", 789),
        ("1?x=2", 789),
        ("..", 789),
        (0, 789),
        (True, 789),
        (123, "7/8"),
        (123, "7?x=8"),
        (123, ".."),
        (123, 0),
        (123, True),
    ],
)
async def test_client_get_instance_interface_rejects_invalid_ids(
    linode_id: Any, interface_id: Any
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="must be a positive integer"):
            await client.get_instance_interface(linode_id, interface_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_get_instance_interface_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="GetInstanceInterface"):
            await client.get_instance_interface(123, 789)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_get_instance_interface_uses_retry() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    retryable = cast("Any", client)
    retryable.client.get_instance_interface = AsyncMock(
        return_value={"id": 789, "purpose": "public"}
    )
    retryable._execute_with_retry = AsyncMock(
        return_value={"id": 789, "purpose": "public"}
    )

    result = await client.get_instance_interface(123, 789)

    assert result == {"id": 789, "purpose": "public"}
    retryable._execute_with_retry.assert_awaited_once_with(
        client.client.get_instance_interface, 123, 789
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance_interface.return_value = {
        "id": 789,
        "purpose": "public",
    }

    result = await handle_linode_instance_interface_get(
        {"linode_id": 123, "interface_id": 789}, sample_config
    )

    assert json.loads(result[0].text) == {"id": 789, "purpose": "public"}
    mock_linode_client.get_instance_interface.assert_awaited_once_with(123, 789)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": "1/2", "interface_id": 789},
        {"linode_id": "1?x=2", "interface_id": 789},
        {"linode_id": "..", "interface_id": 789},
        {"linode_id": 0, "interface_id": 789},
        {"linode_id": True, "interface_id": 789},
        {"linode_id": 123},
        {"linode_id": 123, "interface_id": "7/8"},
        {"linode_id": 123, "interface_id": "7?x=8"},
        {"linode_id": 123, "interface_id": ".."},
        {"linode_id": 123, "interface_id": 0},
        {"linode_id": 123, "interface_id": True},
    ],
)
async def test_handle_linode_instance_interface_get_rejects_invalid_ids(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    assert "positive integer" in result[0].text
    mock_linode_client.get_instance_interface.assert_not_called()


def test_create_linode_instance_interface_get_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_get"
    assert tool.inputSchema["required"] == ["linode_id", "interface_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["interface_id"]["minimum"] == 1


def test_linode_instance_interface_get_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_get"
    assert entry.handle_fn is handle_linode_instance_interface_get
    assert "linode_instance_interface_get" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_add_config_interface_sends_exact_post() -> None:
    """Low-level client sends POST add config interface."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 789, "purpose": "public"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.add_instance_config_interface(
            123, 456, {"purpose": "vlan", "label": "backend"}
        )
    finally:
        await client.close()

    assert result == {"id": 789, "purpose": "public"}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == "/v4/linode/instances/123/configs/456/interfaces"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {"purpose": "vlan", "label": "backend"}


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_add_config_interface_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.add_instance_config_interface(
                linode_id, 456, {"purpose": "public"}
            )
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("config_id", ["4/5", "4?x=5", "..", 0, False])
async def test_client_add_config_interface_rejects_invalid_config_id(
    config_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="config_id must be a positive integer"):
            await client.add_instance_config_interface(
                123, config_id, {"purpose": "public"}
            )
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_add_config_interface_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="AddInstanceConfigInterface"):
            await client.add_instance_config_interface(123, 456, {"purpose": "public"})
    finally:
        await client.close()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("interface", "message"),
    [
        (None, "interface must be an object"),
        ({}, "purpose must be one of"),
        ({"purpose": "other"}, "purpose must be one of"),
        ({"purpose": "vlan"}, "label is required"),
        ({"purpose": "vpc"}, "subnet_id must be a positive integer"),
        ({"purpose": "vpc", "subnet_id": True}, "subnet_id must be a positive integer"),
        ({"purpose": "public", "subnet_id": 0}, "subnet_id must be a positive integer"),
        (
            {"purpose": "public", "subnet_id": True},
            "subnet_id must be a positive integer",
        ),
        ({"purpose": "vlan", "label": 123}, "label must be a string or null"),
        (
            {"purpose": "vlan", "label": "backend", "ipam_address": 123},
            "ipam_address must be a string or null",
        ),
        ({"purpose": "public", "primary": 1}, "primary must be a boolean"),
        (
            {"purpose": "vpc", "subnet_id": 101, "ip_ranges": "10.0.0.0/24"},
            "ip_ranges must be an array of strings or null",
        ),
        (
            {"purpose": "vpc", "subnet_id": 101, "ip_ranges": ["10.0.0.0/24", 1]},
            "ip_ranges must be an array of strings or null",
        ),
        (
            {"purpose": "vpc", "subnet_id": 101, "ipv4": "bad"},
            "ipv4 must be an object or null",
        ),
        (
            {"purpose": "vpc", "subnet_id": 101, "ipv6": "bad"},
            "ipv6 must be an object or null",
        ),
    ],
)
async def test_client_add_config_interface_rejects_invalid_interface_body(
    interface: Any, message: str
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises((TypeError, ValueError), match=message):
            await client.add_instance_config_interface(123, 456, interface)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_add_instance_config_interface_no_replay() -> None:
    """Mutating add interface delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_add = AsyncMock(
        side_effect=NetworkError("AddInstanceConfigInterface", transient)
    )
    cast("Any", retryable.client).add_instance_config_interface = mock_add

    try:
        with pytest.raises(NetworkError):
            await retryable.add_instance_config_interface(
                123, 456, {"purpose": "public"}
            )
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_add.assert_awaited_once_with(123, 456, {"purpose": "public"})


def test_create_linode_instance_config_interface_add_tool_schema() -> None:
    tool, capability = create_linode_instance_config_interface_add_tool()

    assert tool.name == "linode_instance_config_interface_add"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "linode_id",
        "config_id",
        "purpose",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["purpose"]["enum"] == [
        "public",
        "vlan",
        "vpc",
    ]
    assert tool.inputSchema["properties"]["subnet_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["ip_ranges"]["items"] == {"type": "string"}
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "dry_run" not in tool.inputSchema["required"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_add_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.add_instance_config_interface.return_value = {
        "id": 789,
        "purpose": "public",
    }

    result = await handle_linode_instance_config_interface_add(
        {
            "linode_id": 123,
            "config_id": 456,
            "purpose": "vpc",
            "subnet_id": 101,
            "ipv4": {"vpc": "10.0.0.2", "nat_1_1": "any"},
            "ip_ranges": ["10.0.0.64/26"],
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {"id": 789, "purpose": "public"}
    mock_linode_client.add_instance_config_interface.assert_awaited_once_with(
        123,
        456,
        {
            "purpose": "vpc",
            "subnet_id": 101,
            "ipv4": {"vpc": "10.0.0.2", "nat_1_1": "any"},
            "ip_ranges": ["10.0.0.64/26"],
        },
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_add_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_add(
        {
            "linode_id": 123,
            "config_id": 456,
            "purpose": "vlan",
            "label": "backend",
            "ipam_address": "10.0.0.1/24",
            "primary": False,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_interface_add"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == (
        "/linode/instances/123/configs/456/interfaces"
    )
    assert body["would_execute"]["body"] == {
        "purpose": "vlan",
        "label": "backend",
        "ipam_address": "10.0.0.1/24",
        "primary": False,
    }
    mock_linode_client.add_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_config_interface_add_requires_boolean_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {
        "linode_id": 123,
        "config_id": 456,
        "purpose": "public",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_interface_add(arguments, sample_config)

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.add_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"linode_id": 123, "config_id": 456, "confirm": True}, "purpose"),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "other",
                "confirm": True,
            },
            "purpose",
        ),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "vlan",
                "confirm": True,
            },
            "label is required",
        ),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "vpc",
                "confirm": True,
            },
            "subnet_id is required",
        ),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "vpc",
                "subnet_id": True,
                "confirm": True,
            },
            "subnet_id must be a positive integer",
        ),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "public",
                "primary": "yes",
                "confirm": True,
            },
            "primary must be a boolean",
        ),
        (
            {
                "linode_id": 123,
                "config_id": 456,
                "purpose": "vpc",
                "subnet_id": 101,
                "ip_ranges": ["10.0.0.1/24", 3],
                "confirm": True,
            },
            "ip_ranges must be an array of strings or null",
        ),
    ],
)
async def test_handle_linode_instance_config_interface_add_rejects_invalid_body(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    result = await handle_linode_instance_config_interface_add(arguments, sample_config)

    assert result[0].text.startswith(f"Error: {message}")
    mock_linode_client.add_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "config_id": 456, "purpose": "public", "confirm": True},
        {"linode_id": "1?x=2", "config_id": 456, "purpose": "public", "confirm": True},
        {"linode_id": "..", "config_id": 456, "purpose": "public", "confirm": True},
        {"linode_id": 0, "config_id": 456, "purpose": "public", "confirm": True},
        {"linode_id": 123, "config_id": "4/5", "purpose": "public", "confirm": True},
        {"linode_id": 123, "config_id": "4?x=5", "purpose": "public", "confirm": True},
        {"linode_id": 123, "config_id": "..", "purpose": "public", "confirm": True},
        {"linode_id": 123, "config_id": 0, "purpose": "public", "confirm": True},
    ],
)
async def test_handle_linode_instance_config_interface_add_rejects_invalid_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_add(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.add_instance_config_interface.assert_not_called()


def test_linode_instance_config_interface_add_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_interface_add"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_instance_config_interface_add"
    assert entry.handle_fn is handle_linode_instance_config_interface_add
    assert "linode_instance_config_interface_add" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_update_instance_config_sends_exact_method_path_and_body() -> None:
    """Low-level client sends PUT /linode/instances/{linodeId}/configs/{configId}."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 456, "label": "rescue"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_instance_config(
            123,
            456,
            {
                "comments": "boot rescue kernel",
                "devices": {"sda": {"disk_id": 789}},
                "helpers": {"network": True},
                "interfaces": [{"purpose": "public"}],
                "kernel": "linode/latest-64bit",
                "label": "rescue",
                "memory_limit": 1024,
                "root_device": "/dev/sda",
                "run_level": "default",
                "virt_mode": "paravirt",
            },
        )
    finally:
        await client.close()

    assert result["label"] == "rescue"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == "/v4/linode/instances/123/configs/456"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {
        "comments": "boot rescue kernel",
        "devices": {"sda": {"disk_id": 789}},
        "helpers": {"network": True},
        "interfaces": [{"purpose": "public"}],
        "kernel": "linode/latest-64bit",
        "label": "rescue",
        "memory_limit": 1024,
        "root_device": "/dev/sda",
        "run_level": "default",
        "virt_mode": "paravirt",
    }


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "linode_id",
    ["1/2", "1?x=2", "..", 0, True],
)
async def test_client_update_instance_config_rejects_invalid_linode_id_before_request(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.update_instance_config(linode_id, 456, {})
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "config_id",
    ["4/5", "4?x=5", "..", 0, False],
)
async def test_client_update_instance_config_rejects_invalid_config_id_before_request(
    config_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="config_id must be a positive integer"):
            await client.update_instance_config(123, config_id, {})
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_update_instance_config_no_replay() -> None:
    """Mutating update delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_update = AsyncMock(side_effect=NetworkError("UpdateInstanceConfig", transient))
    cast("Any", retryable.client).update_instance_config = mock_update

    try:
        with pytest.raises(NetworkError):
            await retryable.update_instance_config(123, 456, {"label": "rescue"})
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_update.assert_awaited_once_with(123, 456, {"label": "rescue"})


def test_create_linode_instance_config_update_tool_schema() -> None:
    tool, capability = create_linode_instance_config_update_tool()

    assert tool.name == "linode_instance_config_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["linode_id", "config_id", "confirm"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "dry_run" not in tool.inputSchema["required"]
    assert {"required": ["label"]} in tool.inputSchema["anyOf"]
    assert "id" not in tool.inputSchema["properties"]
    assert "devices" in tool.inputSchema["properties"]
    assert "virt_mode" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_instance_config.return_value = {
        "id": 456,
        "label": "rescue",
    }

    result = await handle_linode_instance_config_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "label": "rescue",
            "devices": {"sda": {"disk_id": 789}},
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {"id": 456, "label": "rescue"}
    mock_linode_client.update_instance_config.assert_awaited_once_with(
        123, 456, {"devices": {"sda": {"disk_id": 789}}, "label": "rescue"}
    )


@pytest.mark.asyncio
async def test_client_update_instance_config_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateInstanceConfig"):
            await client.update_instance_config(123, 456, {"label": "rescue"})
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "label": "updated-config",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_update"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == "/linode/instances/123/configs/456"
    assert body["would_execute"]["body"] == {"label": "updated-config"}
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_config_update_requires_boolean_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {"linode_id": 123, "config_id": 456, "label": "rescue"}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_update(arguments, sample_config)

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_requires_update_field(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(
        {"linode_id": 123, "config_id": 456, "confirm": True}, sample_config
    )

    assert result[0].text == "Error: at least one update field is required"
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "config_id": 456, "label": "rescue", "confirm": True},
        {"linode_id": "1?x=2", "config_id": 456, "label": "rescue", "confirm": True},
        {"linode_id": "..", "config_id": 456, "label": "rescue", "confirm": True},
        {"linode_id": 0, "config_id": 456, "label": "rescue", "confirm": True},
        {"linode_id": 123, "config_id": "4/5", "label": "rescue", "confirm": True},
        {"linode_id": 123, "config_id": "4?x=5", "label": "rescue", "confirm": True},
        {"linode_id": 123, "config_id": "..", "label": "rescue", "confirm": True},
        {"linode_id": 123, "config_id": 0, "label": "rescue", "confirm": True},
    ],
)
async def test_handle_linode_instance_config_update_rejects_invalid_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_instance_config.assert_not_called()


def test_linode_instance_config_update_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_instance_config_update"
    assert entry.handle_fn is handle_linode_instance_config_update
    assert "linode_instance_config_update" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_reorder_interfaces_sends_exact_request() -> None:
    """Low-level client sends the exact reorder request."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"ids": [789, 790]})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.reorder_instance_config_interfaces(123, 456, [789, 790])
    finally:
        await client.close()

    assert result == {"ids": [789, 790]}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "POST"
    assert request.url.path == "/v4/linode/instances/123/configs/456/interfaces/order"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {"ids": [789, 790]}


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_reorder_interfaces_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.reorder_instance_config_interfaces(linode_id, 456, [789])
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("config_id", ["4/5", "4?x=5", "..", 0, False])
async def test_client_reorder_interfaces_rejects_invalid_config_id(
    config_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="config_id must be a positive integer"):
            await client.reorder_instance_config_interfaces(123, config_id, [789])
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("ids", [{"ids": [789]}, [], [0], [True], ["789"]])
async def test_client_reorder_interfaces_rejects_invalid_ids(ids: Any) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="ids must be a non-empty list"):
            await client.reorder_instance_config_interfaces(123, 456, ids)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_reorder_instance_config_interfaces_no_replay() -> None:
    """Mutating reorder delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_reorder = AsyncMock(
        side_effect=NetworkError("ReorderInstanceConfigInterfaces", transient)
    )
    cast("Any", retryable.client).reorder_instance_config_interfaces = mock_reorder

    try:
        with pytest.raises(NetworkError):
            await retryable.reorder_instance_config_interfaces(123, 456, [789, 790])
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_reorder.assert_awaited_once_with(123, 456, [789, 790])


def test_create_linode_instance_config_interfaces_order_tool_schema() -> None:
    from linodemcp.tools.linode_instances import (
        create_linode_instance_config_interface_reorder_tool,
    )

    tool, capability = create_linode_instance_config_interface_reorder_tool()

    assert tool.name == "linode_instance_config_interface_reorder"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["linode_id", "config_id", "ids", "confirm"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["ids"]["type"] == "array"
    assert tool.inputSchema["properties"]["ids"]["items"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interfaces_order_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    mock_linode_client.reorder_instance_config_interfaces.return_value = {
        "ids": [789, 790]
    }

    result = await handle_linode_instance_config_interface_reorder(
        {"linode_id": 123, "config_id": 456, "ids": [789, 790], "confirm": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {"ids": [789, 790]}
    mock_linode_client.reorder_instance_config_interfaces.assert_awaited_once_with(
        123, 456, [789, 790]
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interfaces_order_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    result = await handle_linode_instance_config_interface_reorder(
        {
            "linode_id": 123,
            "config_id": 456,
            "ids": [789, 790],
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_interface_reorder"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "POST"
    assert (
        body["would_execute"]["path"]
        == "/linode/instances/123/configs/456/interfaces/order"
    )
    assert body["would_execute"]["body"] == {"ids": [789, 790]}
    mock_linode_client.reorder_instance_config_interfaces.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_config_interfaces_order_requires_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    arguments: dict[str, Any] = {"linode_id": 123, "config_id": 456, "ids": [789]}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_interface_reorder(
        arguments, sample_config
    )

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.reorder_instance_config_interfaces.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "config_id": 456, "ids": [789], "confirm": True},
        {"linode_id": "1?x=2", "config_id": 456, "ids": [789], "confirm": True},
        {"linode_id": "..", "config_id": 456, "ids": [789], "confirm": True},
        {"linode_id": 0, "config_id": 456, "ids": [789], "confirm": True},
        {"linode_id": 123, "config_id": "4/5", "ids": [789], "confirm": True},
        {"linode_id": 123, "config_id": "4?x=5", "ids": [789], "confirm": True},
        {"linode_id": 123, "config_id": "..", "ids": [789], "confirm": True},
        {"linode_id": 123, "config_id": 0, "ids": [789], "confirm": True},
    ],
)
async def test_handle_config_interfaces_order_rejects_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    result = await handle_linode_instance_config_interface_reorder(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.reorder_instance_config_interfaces.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("ids", [None, [], [0], [True], ["789"]])
async def test_handle_linode_instance_config_interfaces_order_requires_ids(
    ids: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    arguments: dict[str, Any] = {"linode_id": 123, "config_id": 456, "confirm": True}
    if ids is not None:
        arguments["ids"] = ids

    result = await handle_linode_instance_config_interface_reorder(
        arguments, sample_config
    )

    assert result[0].text == "Error: ids must be a non-empty list of positive integers"
    mock_linode_client.reorder_instance_config_interfaces.assert_not_called()


@pytest.mark.asyncio
async def test_client_reorder_instance_config_interfaces_translates_http_errors() -> (
    None
):
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ReorderInstanceConfigInterfaces"):
            await client.reorder_instance_config_interfaces(123, 456, [789])
    finally:
        await client.close()


def test_linode_instance_config_interfaces_order_registered_and_exported() -> None:
    from linodemcp.tools.linode_instances import (
        handle_linode_instance_config_interface_reorder,
    )

    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_interface_reorder"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_instance_config_interface_reorder"
    assert entry.handle_fn is handle_linode_instance_config_interface_reorder
    assert "linode_instance_config_interface_reorder" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_delete_instance_interface_sends_exact_request() -> None:
    """Low-level client sends DELETE for the documented instance interface path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_instance_interface(123, 789)
    finally:
        await client.close()

    assert len(seen) == 1
    request = seen[0]
    assert request.method == "DELETE"
    assert request.url.path == "/v4/linode/instances/123/interfaces/789"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
async def test_client_delete_instance_interface_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="DeleteInstanceInterface"):
            await client.delete_instance_interface(123, 789)
    finally:
        await client.close()


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_delete_instance_interface_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.delete_instance_interface(linode_id, 789)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("interface_id", ["7/8", "7?x=8", "..", 0, True])
async def test_client_delete_instance_interface_rejects_invalid_interface_id(
    interface_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="interface_id must be a positive integer"):
            await client.delete_instance_interface(123, interface_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_delete_instance_interface_no_replay() -> None:
    """Mutating delete delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_delete = AsyncMock(
        side_effect=NetworkError("DeleteInstanceInterface", transient)
    )
    cast("Any", retryable.client).delete_instance_interface = mock_delete

    try:
        with pytest.raises(NetworkError):
            await retryable.delete_instance_interface(123, 789)
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_delete.assert_awaited_once_with(123, 789)


def test_create_linode_instance_interface_delete_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_delete_tool()

    assert tool.name == "linode_instance_interface_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == [
        "linode_id",
        "interface_id",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["interface_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_delete(
        {"linode_id": 123, "interface_id": 789, "confirm": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body == {
        "message": "Linode instance interface 789 deleted from Linode 123",
        "linode_id": 123,
        "interface_id": 789,
    }
    mock_linode_client.delete_instance_interface.assert_awaited_once_with(123, 789)


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_delete_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_delete(
        {
            "linode_id": 123,
            "interface_id": 789,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_interface_delete"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/linode/instances/123/interfaces/789"
    assert "body" not in body["would_execute"]
    mock_linode_client.delete_instance_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_instance_interface_delete_requires_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {"linode_id": 123, "interface_id": 789}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_interface_delete(arguments, sample_config)

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.delete_instance_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "interface_id": 789, "confirm": True},
        {"linode_id": "1?x=2", "interface_id": 789, "confirm": True},
        {"linode_id": "..", "interface_id": 789, "confirm": True},
        {"linode_id": 0, "interface_id": 789, "confirm": True},
        {"linode_id": True, "interface_id": 789, "confirm": True},
        {"linode_id": 123, "interface_id": "7/8", "confirm": True},
        {"linode_id": 123, "interface_id": "7?x=8", "confirm": True},
        {"linode_id": 123, "interface_id": "..", "confirm": True},
        {"linode_id": 123, "interface_id": 0, "confirm": True},
        {"linode_id": 123, "interface_id": True, "confirm": True},
    ],
)
async def test_handle_instance_interface_delete_rejects_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_delete(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.delete_instance_interface.assert_not_called()


def test_linode_instance_interface_delete_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_instance_interface_delete"
    assert entry.handle_fn is handle_linode_instance_interface_delete
    assert "linode_instance_interface_delete" in FEATURE_TOOLS_LIST


@pytest.mark.asyncio
async def test_client_delete_instance_config_interface_sends_exact_request() -> None:
    """Low-level client sends DELETE for the documented config interface path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_instance_config_interface(123, 456, 789)
    finally:
        await client.close()

    assert len(seen) == 1
    request = seen[0]
    assert request.method == "DELETE"
    assert request.url.path == "/v4/linode/instances/123/configs/456/interfaces/789"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
async def test_client_delete_instance_config_interface_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="DeleteInstanceConfigInterface"):
            await client.delete_instance_config_interface(123, 456, 789)
    finally:
        await client.close()


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_delete_config_interface_rejects_invalid_linode_id(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.delete_instance_config_interface(linode_id, 456, 789)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("config_id", ["4/5", "4?x=5", "..", 0, False])
async def test_client_delete_config_interface_rejects_invalid_config_id(
    config_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="config_id must be a positive integer"):
            await client.delete_instance_config_interface(123, config_id, 789)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize("interface_id", ["7/8", "7?x=8", "..", 0, True])
async def test_client_delete_config_interface_rejects_invalid_interface_id(
    interface_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="interface_id must be a positive integer"):
            await client.delete_instance_config_interface(123, 456, interface_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_delete_instance_config_interface_no_replay() -> None:
    """Mutating delete delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_delete = AsyncMock(
        side_effect=NetworkError("DeleteInstanceConfigInterface", transient)
    )
    cast("Any", retryable.client).delete_instance_config_interface = mock_delete

    try:
        with pytest.raises(NetworkError):
            await retryable.delete_instance_config_interface(123, 456, 789)
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_delete.assert_awaited_once_with(123, 456, 789)


def test_create_linode_instance_config_interface_delete_tool_schema() -> None:
    tool, capability = create_linode_instance_config_interface_delete_tool()

    assert tool.name == "linode_instance_config_interface_delete"
    assert capability is Capability.Destroy
    assert tool.inputSchema["required"] == [
        "linode_id",
        "config_id",
        "interface_id",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["interface_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_delete(
        {"linode_id": 123, "config_id": 456, "interface_id": 789, "confirm": True},
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body == {
        "message": "Linode instance config 456 interface 789 deleted from Linode 123",
        "linode_id": 123,
        "config_id": 456,
        "interface_id": 789,
    }
    mock_linode_client.delete_instance_config_interface.assert_awaited_once_with(
        123, 456, 789
    )


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_delete_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_delete(
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": 789,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_interface_delete"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "DELETE"
    assert (
        body["would_execute"]["path"]
        == "/linode/instances/123/configs/456/interfaces/789"
    )
    assert "body" not in body["would_execute"]
    mock_linode_client.delete_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_config_interface_delete_requires_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {
        "linode_id": 123,
        "config_id": 456,
        "interface_id": 789,
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_interface_delete(
        arguments, sample_config
    )

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.delete_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "config_id": 456, "interface_id": 789, "confirm": True},
        {"linode_id": "1?x=2", "config_id": 456, "interface_id": 789, "confirm": True},
        {"linode_id": "..", "config_id": 456, "interface_id": 789, "confirm": True},
        {"linode_id": 0, "config_id": 456, "interface_id": 789, "confirm": True},
        {"linode_id": True, "config_id": 456, "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": "4/5", "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": "4?x=5", "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": "..", "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": 0, "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": False, "interface_id": 789, "confirm": True},
        {"linode_id": 123, "config_id": 456, "interface_id": "7/8", "confirm": True},
        {"linode_id": 123, "config_id": 456, "interface_id": "7?x=8", "confirm": True},
        {"linode_id": 123, "config_id": 456, "interface_id": "..", "confirm": True},
        {"linode_id": 123, "config_id": 456, "interface_id": 0, "confirm": True},
        {"linode_id": 123, "config_id": 456, "interface_id": True, "confirm": True},
    ],
)
async def test_handle_config_interface_delete_rejects_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_delete(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.delete_instance_config_interface.assert_not_called()


def test_linode_instance_config_interface_delete_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_interface_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_instance_config_interface_delete"
    assert entry.handle_fn is handle_linode_instance_config_interface_delete
    assert "linode_instance_config_interface_delete" in FEATURE_TOOLS_LIST
