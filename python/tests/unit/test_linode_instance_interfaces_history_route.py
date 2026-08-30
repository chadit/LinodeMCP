"""Tests for Linode instance network interface history route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar

import pytest

from linodemcp.gentools import (
    create_linode_instance_interface_history_list_tool as exported_create_tool,
)
from linodemcp.gentools.instance import (
    create_linode_instance_interface_history_list_tool,
    handle_linode_instance_interface_history_list,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


T = TypeVar("T")


def test_create_linode_instance_interfaces_history_list_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_history_list_tool()

    assert tool.name == "linode_instance_interface_history_list"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["linode_id"]


@pytest.mark.asyncio
async def test_handle_linode_instance_interfaces_history_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.route_raw.return_value = {
        "data": [
            {
                "interface_history_id": 3,
                "interface_id": 221,
                "linode_id": 123,
                "version": 1,
                "interface_data": {"mac_address": "22:00:AB:CD:EF:02"},
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_instance_interface_history_list(
        {"linode_id": 123}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["interface_history"][0]["interface_history_id"] == 3
    assert payload["interface_history"][0]["linode_id"] == 123
    assert payload["interface_history"][0]["interface_data"] == {
        "mac_address": "22:00:AB:CD:EF:02"
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_interface_history_list", 123, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": "1/2"},
        {"linode_id": "1?x=2"},
        {"linode_id": ".."},
        {"linode_id": 0},
        {"linode_id": True},
    ],
)
async def test_handle_linode_instance_interfaces_history_list_rejects_invalid_linode_id(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_history_list(
        arguments, sample_config
    )

    assert result[0].text in (
        "Error: linode_id is required",
        "Error: linode_id must be a positive integer",
    )
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"linode_id": 123, "page": 0},
            "page must be an integer greater than or equal to 1",
        ),
        ({"linode_id": 123, "page": "2"}, "page must be an integer"),
        (
            {"linode_id": 123, "page_size": 24},
            "page_size must be an integer from 25 through 500",
        ),
        (
            {"linode_id": 123, "page_size": 501},
            "page_size must be an integer from 25 through 500",
        ),
        ({"linode_id": 123, "page_size": True}, "page_size must be an integer"),
    ],
)
async def test_handle_linode_instance_interfaces_history_list_rejects_bad_pagination(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    result = await handle_linode_instance_interface_history_list(
        arguments, sample_config
    )

    assert result[0].text == f"Error: {message}"
    mock_linode_client.route_raw.assert_not_called()


def test_linode_instance_interfaces_history_list_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_history_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_history_list"
    assert entry.handle_fn is handle_linode_instance_interface_history_list
    assert exported_create_tool is create_linode_instance_interface_history_list_tool
