"""Tests for updating Linode instance configuration profiles."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar

import pytest

from linodemcp.gentools import (
    create_linode_instance_config_interface_delete_tool,
    create_linode_instance_interface_delete_tool,
    create_linode_instance_interface_get_tool,
    create_linode_instance_interface_list_tool,
    create_linode_instance_interface_settings_get_tool,
    handle_linode_instance_config_interface_delete,
    handle_linode_instance_interface_delete,
    handle_linode_instance_interface_get,
    handle_linode_instance_interface_list,
    handle_linode_instance_interface_settings_get,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

T = TypeVar("T")


@pytest.mark.asyncio
async def test_handle_linode_instance_interfaces_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    # The current-generation interfaces endpoint wraps under "interfaces", not a
    # {data} page envelope; the handler unwraps it and emits the proto list shape.
    mock_linode_client.route_raw.return_value = {
        "interfaces": [
            {
                "id": 789,
                "mac_address": "22:00:AB:CD:EF:01",
                "default_route": {"ipv4": True, "ipv6": False},
                "vpc": {
                    "subnet_id": 555,
                    "ipv4": {"addresses": [{"address": "10.0.0.5"}]},
                },
            },
        ],
    }

    result = await handle_linode_instance_interface_list(
        {"linode_id": 123}, sample_config
    )

    body = json.loads(result[0].text)
    assert body["count"] == 1
    interface = body["interfaces"][0]
    assert interface["id"] == 789
    assert interface["mac_address"] == "22:00:AB:CD:EF:01"
    assert interface["default_route"]["ipv4"] is True
    assert interface["vpc"]["subnet_id"] == 555
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_interface_list", 123, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "api_response",
    [{}, {"interfaces": None}, {"unrelated": True}],
    ids=["missing", "null", "extra-field"],
)
async def test_handle_linode_instance_interfaces_list_accepts_empty_shapes(
    api_response: dict[str, Any],
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    mock_linode_client.route_raw.return_value = api_response

    result = await handle_linode_instance_interface_list(
        {"linode_id": 123}, sample_config
    )

    assert json.loads(result[0].text) == {"count": 0, "interfaces": []}


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "interfaces",
    [{}, "", 0, False],
    ids=["object", "string", "number", "boolean"],
)
async def test_handle_linode_instance_interfaces_list_rejects_falsey_non_arrays(
    interfaces: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    mock_linode_client.route_raw.return_value = {"interfaces": interfaces}

    result = await handle_linode_instance_interface_list(
        {"linode_id": 123}, sample_config
    )

    assert result[0].text.startswith(
        "Failed to retrieve items: list response data must be an array"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "api_response",
    [[], None, "", 0, False],
    ids=["array", "null", "string", "number", "boolean"],
)
async def test_handle_linode_instance_interfaces_list_rejects_non_object_response(
    api_response: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    mock_linode_client.route_raw.return_value = api_response

    result = await handle_linode_instance_interface_list(
        {"linode_id": 123}, sample_config
    )

    assert result[0].text.startswith(
        "Failed to retrieve items: list response must be an object"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_handle_linode_instance_interfaces_list_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_list(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_settings_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.route_raw.return_value = {
        "default_route": {"ipv4_interface_id": 1001}
    }

    result = await handle_linode_instance_interface_settings_get(
        {"linode_id": 123}, sample_config
    )

    assert json.loads(result[0].text) == {"default_route": {"ipv4_interface_id": 1001}}
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_interface_settings_get", 123
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_handle_linode_instance_interface_settings_get_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_settings_get(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.route_raw.assert_not_called()


def test_create_linode_instance_interface_settings_get_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_settings_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_settings_get"
    assert tool.input_schema["required"] == ["linode_id"]
    assert "linode_id" in tool.input_schema["properties"]


def test_linode_instance_interface_settings_get_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_settings_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_settings_get"
    assert entry.handle_fn is handle_linode_instance_interface_settings_get


def test_create_linode_instance_interfaces_list_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_list_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_list"
    assert tool.input_schema["required"] == ["linode_id"]


def test_linode_instance_interfaces_list_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_list"
    assert entry.handle_fn is handle_linode_instance_interface_list


@pytest.mark.asyncio
async def test_handle_linode_instance_interface_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.route_raw.return_value = {
        "id": 789,
        "mac_address": "22:00:AB:CD:EF:02",
        "vlan": {"vlan_label": "vlan-test", "ipam_address": "10.0.0.1/24"},
    }

    result = await handle_linode_instance_interface_get(
        {"linode_id": 123, "interface_id": 789}, sample_config
    )

    assert json.loads(result[0].text) == {
        "id": 789,
        "vlan": {"vlan_label": "vlan-test", "ipam_address": "10.0.0.1/24"},
        "mac_address": "22:00:AB:CD:EF:02",
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_interface_get", 123, 789
    )


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
    assert "positive integer" in result[0].text or "is required" in result[0].text
    mock_linode_client.route_raw.assert_not_called()


def test_create_linode_instance_interface_get_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_get_tool()

    assert capability is Capability.Read
    assert tool.name == "linode_instance_interface_get"
    assert set(tool.input_schema["required"]) == {"linode_id", "interface_id"}
    assert "linode_id" in tool.input_schema["properties"]
    assert "interface_id" in tool.input_schema["properties"]


def test_linode_instance_interface_get_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_get"
    assert entry.handle_fn is handle_linode_instance_interface_get


def test_create_linode_instance_interface_delete_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_delete_tool()

    assert tool.name == "linode_instance_interface_delete"
    assert capability is Capability.Destroy
    assert tool.input_schema["required"] == [
        "linode_id",
        "interface_id",
        "confirm",
    ]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.input_schema["properties"]


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
        "message": "Interface 789 deleted from instance 123 successfully",
        "linode_id": 123,
        "interface_id": 789,
    }
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_instance_interface_delete", 123, 789, retry=False
    )


def test_linode_instance_interface_delete_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_instance_interface_delete"
    assert entry.handle_fn is handle_linode_instance_interface_delete


def test_create_linode_instance_config_interface_delete_tool_schema() -> None:
    tool, capability = create_linode_instance_config_interface_delete_tool()

    assert tool.name == "linode_instance_config_interface_delete"
    assert capability is Capability.Destroy
    assert tool.input_schema["required"] == [
        "linode_id",
        "config_id",
        "interface_id",
        "confirm",
    ]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.input_schema["properties"]


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
        "message": (
            "Configuration profile interface 789 removed from config 456"
            " on instance 123"
        ),
        "linode_id": 123,
        "config_id": 456,
        "interface_id": 789,
    }
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_instance_config_interface_delete", 123, 456, 789, retry=False
    )


def test_linode_instance_config_interface_delete_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_interface_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_instance_config_interface_delete"
    assert entry.handle_fn is handle_linode_instance_config_interface_delete
