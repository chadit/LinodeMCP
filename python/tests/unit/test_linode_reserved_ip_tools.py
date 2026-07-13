"""Tool, schema, registration, and profile coverage for reserved IPv4 routes."""

from __future__ import annotations

import dataclasses
import json
from typing import TYPE_CHECKING, Any, cast

import pytest

from linodemcp.config import BuiltinOverride
from linodemcp.linode import NetworkError
from linodemcp.profiles import Capability, Scope, required_scopes
from linodemcp.profiles.builtin import categories
from linodemcp.server import Server, get_tool_registry
from linodemcp.tools.linode_reserved_ips import (
    create_linode_networking_reserved_ip_create_tool,
    create_linode_networking_reserved_ip_delete_tool,
    create_linode_networking_reserved_ip_get_tool,
    create_linode_networking_reserved_ip_list_tool,
    create_linode_networking_reserved_ip_type_list_tool,
    create_linode_networking_reserved_ip_update_tool,
    handle_linode_networking_reserved_ip_create,
    handle_linode_networking_reserved_ip_delete,
    handle_linode_networking_reserved_ip_get,
    handle_linode_networking_reserved_ip_list,
    handle_linode_networking_reserved_ip_type_list,
    handle_linode_networking_reserved_ip_update,
)
from linodemcp.version import get_version_info

if TYPE_CHECKING:
    from collections.abc import Callable
    from unittest.mock import AsyncMock

    from mcp.types import TextContent, Tool

    from linodemcp.config import Config


RESERVED_IP = {
    "address": "192.0.2.10",
    "assigned_entity": {
        "id": 1234,
        "label": "web-01",
        "type": "linode",
        "url": "/v4/linode/instances/1234",
    },
    "gateway": "192.0.2.1",
    "interface_id": 456,
    "linode_id": 1234,
    "prefix": 24,
    "public": True,
    "rdns": None,
    "region": "us-east",
    "reserved": True,
    "subnet_mask": "255.255.255.0",
    "tags": ["prod", "web"],
    "type": "ipv4",
    "vpc_nat_1_1": {"address": "192.168.0.42", "subnet_id": 101, "vpc_id": 111},
}


def _body(result: list[TextContent]) -> dict[str, Any]:
    return cast("dict[str, Any]", json.loads(result[0].text))


def test_reserved_ip_tool_schemas_and_capabilities() -> None:
    """All tools expose proto schemas and mutations require boolean confirmation."""
    factories: list[tuple[Callable[[], tuple[Tool, Capability]], Capability]] = [
        (create_linode_networking_reserved_ip_create_tool, Capability.Write),
        (create_linode_networking_reserved_ip_delete_tool, Capability.Destroy),
        (create_linode_networking_reserved_ip_get_tool, Capability.Read),
        (create_linode_networking_reserved_ip_list_tool, Capability.Read),
        (create_linode_networking_reserved_ip_type_list_tool, Capability.Read),
        (create_linode_networking_reserved_ip_update_tool, Capability.Write),
    ]
    for factory, expected_capability in factories:
        tool, capability = factory()
        assert capability == expected_capability
        assert tool.inputSchema["additionalProperties"] is False
        if capability in {Capability.Write, Capability.Destroy}:
            assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
            assert "confirm" in tool.inputSchema["required"]
            assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
        else:
            assert "confirm" not in tool.inputSchema["properties"]


async def test_reserved_ip_create_passes_body_and_preserves_response(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Reserve forwards required/optional body fields and preserves nested data."""
    mock_linode_client.create_reserved_ip.return_value = RESERVED_IP
    result = await handle_linode_networking_reserved_ip_create(
        {"region": "us-east", "tags": ["prod", "web"], "confirm": True},
        sample_config,
    )
    body = _body(result)
    assert body == RESERVED_IP
    assert result[0].text == json.dumps(RESERVED_IP, indent=2)
    assert type(body["assigned_entity"]["id"]) is int
    assert type(body["interface_id"]) is int
    assert type(body["linode_id"]) is int
    assert type(body["vpc_nat_1_1"]["subnet_id"]) is int
    assert type(body["vpc_nat_1_1"]["vpc_id"]) is int
    mock_linode_client.create_reserved_ip.assert_awaited_once_with(
        "us-east", ["prod", "web"]
    )


async def test_reserved_ip_list_passes_pagination_and_preserves_nulls(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """List uses repository pagination and retains documented nullable fields."""
    unassigned = {
        **RESERVED_IP,
        "assigned_entity": None,
        "gateway": None,
        "interface_id": None,
        "linode_id": None,
        "rdns": None,
        "vpc_nat_1_1": None,
    }
    mock_linode_client.list_reserved_ips.return_value = {
        "data": [unassigned],
        "page": 2,
        "pages": 2,
        "results": 1,
    }
    result = await handle_linode_networking_reserved_ip_list(
        {"page": 2, "page_size": 50}, sample_config
    )
    body = _body(result)
    assert body["count"] == 1
    assert body["reserved_ips"][0]["reserved"] is True
    assert body["reserved_ips"][0]["assigned_entity"] is None
    assert body["reserved_ips"][0]["gateway"] is None
    assert body["reserved_ips"][0]["interface_id"] is None
    assert body["reserved_ips"][0]["linode_id"] is None
    assert body["reserved_ips"][0]["rdns"] is None
    assert body["reserved_ips"][0]["vpc_nat_1_1"] is None
    mock_linode_client.list_reserved_ips.assert_awaited_once_with(page=2, page_size=50)


async def test_reserved_ip_get_preserves_vpc_nat_shape(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Get passes through assigned entity, tags, and the VPC NAT object."""
    mock_linode_client.get_reserved_ip.return_value = RESERVED_IP
    result = await handle_linode_networking_reserved_ip_get(
        {"address": "192.0.2.10"}, sample_config
    )
    body = _body(result)
    assert body["assigned_entity"] == RESERVED_IP["assigned_entity"]
    assert body["tags"] == ["prod", "web"]
    assert body["vpc_nat_1_1"] == {
        "address": "192.168.0.42",
        "subnet_id": 101,
        "vpc_id": 111,
    }
    mock_linode_client.get_reserved_ip.assert_awaited_once_with("192.0.2.10")


async def test_reserved_ip_update_replaces_tags(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Update sends the full replacement tag list and returns the API object."""
    updated = {**RESERVED_IP, "tags": []}
    mock_linode_client.update_reserved_ip.return_value = updated
    result = await handle_linode_networking_reserved_ip_update(
        {"address": "192.0.2.10", "tags": [], "confirm": True}, sample_config
    )
    assert _body(result)["tags"] == []
    mock_linode_client.update_reserved_ip.assert_awaited_once_with("192.0.2.10", [])


async def test_reserved_ip_type_list_preserves_pricing(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Pricing includes nullable monthly price, region overrides, and transfer."""
    price_type = {
        "id": "reserved-ipv4",
        "label": "Reserved IPv4",
        "price": {"hourly": 0.0068, "monthly": None},
        "region_prices": [{"id": "id-cgk", "hourly": 0.008, "monthly": 5.0}],
        "transfer": 0,
    }
    mock_linode_client.list_reserved_ip_types.return_value = {
        "data": [price_type],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    result = await handle_linode_networking_reserved_ip_type_list({}, sample_config)
    assert _body(result) == {"count": 1, "reserved_ip_types": [price_type]}
    mock_linode_client.list_reserved_ip_types.assert_awaited_once_with()


async def test_reserved_ip_delete_handles_empty_success(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """An empty successful API response produces a deterministic confirmation."""
    mock_linode_client.delete_reserved_ip.return_value = None
    result = await handle_linode_networking_reserved_ip_delete(
        {"address": "192.0.2.10", "confirm": True}, sample_config
    )
    assert _body(result) == {
        "message": "Reserved IP 192.0.2.10 unreserved successfully",
        "address": "192.0.2.10",
    }
    mock_linode_client.delete_reserved_ip.assert_awaited_once_with("192.0.2.10")


@pytest.mark.parametrize(
    ("handler", "arguments", "message", "client_method"),
    [
        (
            handle_linode_networking_reserved_ip_create,
            {"confirm": True},
            "region is required",
            "create_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_update,
            {"address": "192.0.2.10", "confirm": True},
            "tags is required",
            "update_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_update,
            {"address": "192.0.2.10", "tags": "prod", "confirm": True},
            "tags must be a list of strings",
            "update_reserved_ip",
        ),
    ],
)
async def test_reserved_ip_tools_validate_required_body_fields(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[[dict[str, Any], Config], Any],
    arguments: dict[str, Any],
    message: str,
    client_method: str,
) -> None:
    """Missing or malformed documented body fields fail before client use."""
    result = await handler(arguments, sample_config)
    assert message in result[0].text
    getattr(mock_linode_client, client_method).assert_not_called()


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": "first"}, "page must be an integer"),
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_reserved_ip_list_validates_pagination(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    arguments: dict[str, Any],
    message: str,
) -> None:
    """Invalid page controls are rejected before listing."""
    result = await handle_linode_networking_reserved_ip_list(arguments, sample_config)
    assert message in result[0].text
    mock_linode_client.list_reserved_ips.assert_not_called()


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
@pytest.mark.parametrize(
    ("handler", "arguments", "client_method"),
    [
        (
            handle_linode_networking_reserved_ip_create,
            {"region": "us-east"},
            "create_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_update,
            {"address": "192.0.2.10", "tags": ["prod"]},
            "update_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_delete,
            {"address": "192.0.2.10"},
            "delete_reserved_ip",
        ),
    ],
)
async def test_reserved_ip_mutations_require_strict_boolean_confirm(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    confirm: object,
    handler: Callable[[dict[str, Any], Config], Any],
    arguments: dict[str, Any],
    client_method: str,
) -> None:
    """Missing, false, string, and numeric confirmations fail before client use."""
    call_arguments = dict(arguments)
    if confirm is not None:
        call_arguments["confirm"] = confirm
    result = await handler(call_arguments, sample_config)
    assert "confirm=true" in result[0].text
    getattr(mock_linode_client, client_method).assert_not_called()


@pytest.mark.parametrize("address", ["192.0.2.10/other", "192.0.2.10?x=1", ".."])
@pytest.mark.parametrize(
    ("handler", "extra", "client_method"),
    [
        (handle_linode_networking_reserved_ip_get, {}, "get_reserved_ip"),
        (
            handle_linode_networking_reserved_ip_update,
            {"tags": ["prod"], "confirm": True},
            "update_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_delete,
            {"confirm": True},
            "delete_reserved_ip",
        ),
    ],
)
async def test_reserved_ip_tools_reject_malformed_address_before_client(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    address: str,
    handler: Callable[[dict[str, Any], Config], Any],
    extra: dict[str, Any],
    client_method: str,
) -> None:
    """Separators, query syntax, and traversal values never reach the client."""
    result = await handler({"address": address, **extra}, sample_config)
    assert "valid IPv4 address" in result[0].text
    getattr(mock_linode_client, client_method).assert_not_called()


async def test_reserved_ip_create_dry_run_previews_without_client(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Reserve dry-run previews the exact request and does not require confirm."""
    result = await handle_linode_networking_reserved_ip_create(
        {"region": "us-east", "tags": ["prod"], "dry_run": True}, sample_config
    )
    body = _body(result)
    assert body["would_execute"] == {
        "method": "POST",
        "path": "/networking/reserved/ips",
        "body": {"region": "us-east", "tags": ["prod"]},
    }
    mock_linode_client.create_reserved_ip.assert_not_called()


@pytest.mark.parametrize(
    ("handler", "arguments", "method", "request_body", "mutation"),
    [
        (
            handle_linode_networking_reserved_ip_update,
            {"address": "192.0.2.10", "tags": ["prod"], "dry_run": True},
            "PUT",
            {"tags": ["prod"]},
            "update_reserved_ip",
        ),
        (
            handle_linode_networking_reserved_ip_delete,
            {"address": "192.0.2.10", "dry_run": True},
            "DELETE",
            None,
            "delete_reserved_ip",
        ),
    ],
)
async def test_reserved_ip_existing_resource_dry_runs_fetch_without_mutating(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[[dict[str, Any], Config], Any],
    arguments: dict[str, Any],
    method: str,
    request_body: dict[str, Any] | None,
    mutation: str,
) -> None:
    """Update and delete dry-runs fetch current state and never mutate."""
    mock_linode_client.get_reserved_ip.return_value = RESERVED_IP
    result = await handler(arguments, sample_config)
    body = _body(result)
    assert body["would_execute"]["method"] == method
    if request_body is None:
        assert "body" not in body["would_execute"]
    else:
        assert body["would_execute"]["body"] == request_body
    assert body["current_state"]["address"] == "192.0.2.10"
    mock_linode_client.get_reserved_ip.assert_awaited_once_with("192.0.2.10")
    getattr(mock_linode_client, mutation).assert_not_called()


async def test_reserved_ip_tool_maps_client_error(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """Client failures use the shared tool error mapping."""
    mock_linode_client.get_reserved_ip.side_effect = NetworkError(
        "GetReservedIP", RuntimeError("boom")
    )
    result = await handle_linode_networking_reserved_ip_get(
        {"address": "192.0.2.10"}, sample_config
    )
    assert result[0].text.startswith("Failed to get reserved IPv4 address:")


def test_reserved_ip_tools_are_exported_registered_and_profiled(
    sample_config: Config,
) -> None:
    """Exports drive registry discovery and networking built-ins include the family."""
    from linodemcp import tools as tools_module

    names = {
        "linode_networking_reserved_ip_create",
        "linode_networking_reserved_ip_delete",
        "linode_networking_reserved_ip_get",
        "linode_networking_reserved_ip_list",
        "linode_networking_reserved_ip_type_list",
        "linode_networking_reserved_ip_update",
    }
    registry_names = {entry.name for entry in get_tool_registry()}
    assert names <= registry_names
    for name in names:
        assert f"create_{name}_tool" in tools_module.__all__
        assert f"handle_{name}" in tools_module.__all__
        assert categories(name) == ["networking"]

    default_server = Server(sample_config)
    assert {
        "linode_networking_reserved_ip_get",
        "linode_networking_reserved_ip_list",
        "linode_networking_reserved_ip_type_list",
    } <= default_server.registered_tool_names

    full_access = dataclasses.replace(
        sample_config,
        active_profile="full-access",
        profiles_builtin_overrides={"full-access": BuiltinOverride(disabled=False)},
    )
    assert names <= Server(full_access).registered_tool_names

    features = set(get_version_info().features["tools"].split(","))
    assert names <= features


@pytest.mark.parametrize(
    ("tool_name", "capability", "expected"),
    [
        (
            "linode_networking_reserved_ip_get",
            Capability.Read,
            Scope.ReservedIPsReadOnly,
        ),
        (
            "linode_networking_reserved_ip_list",
            Capability.Read,
            Scope.IPsReadOnly,
        ),
        (
            "linode_networking_reserved_ip_type_list",
            Capability.Read,
            Scope.ReservedIPsReadOnly,
        ),
        (
            "linode_networking_reserved_ip_create",
            Capability.Write,
            Scope.IPsReadWrite,
        ),
        (
            "linode_networking_reserved_ip_update",
            Capability.Write,
            Scope.ReservedIPsReadWrite,
        ),
        (
            "linode_networking_reserved_ip_delete",
            Capability.Destroy,
            Scope.ReservedIPsReadWrite,
        ),
    ],
)
def test_reserved_ip_tools_require_documented_token_scopes(
    tool_name: str, capability: Capability, expected: Scope
) -> None:
    """Reserved-IP tools declare each endpoint's documented token scope."""
    assert required_scopes(tool_name, capability) == [expected]
