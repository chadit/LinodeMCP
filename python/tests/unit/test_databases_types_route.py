"""Tests for the Managed Databases types route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar

import pytest

from linodemcp.gentools import (
    create_linode_database_type_get_tool,
    create_linode_database_type_list_tool,
    handle_linode_database_engine_list,
    handle_linode_database_type_get,
    handle_linode_database_type_list,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


T = TypeVar("T")


def test_create_linode_database_type_get_tool_schema() -> None:
    """Tool schema exposes the documented type ID and pagination params."""
    tool, capability = create_linode_database_type_get_tool()

    assert tool.name == "linode_database_type_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["type_id"]
    assert "type_id" in tool.input_schema["properties"]
    assert "page" in tool.input_schema["properties"]
    assert "page_size" in tool.input_schema["properties"]


def test_create_linode_databases_types_list_tool_schema() -> None:
    """Tool schema exposes the documented pagination params."""
    tool, capability = create_linode_database_type_list_tool()

    assert tool.name == "linode_database_type_list"
    assert capability is Capability.Read
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_database_type_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns database type details."""
    mock_linode_client.route_raw.return_value = {
        "id": "g6-dedicated-2",
        "label": "Dedicated 4GB",
        "engines": {"mysql": [], "postgresql": []},
    }

    result = await handle_linode_database_type_get(
        {"type_id": "g6-dedicated-2", "page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["id"] == "g6-dedicated-2"
    assert payload["label"] == "Dedicated 4GB"
    assert payload["engines"] == {"mysql": [], "postgresql": []}
    assert payload["deprecated"] is False
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_database_type_get", "g6-dedicated-2", query="page=2&page_size=50"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"type_id": ""},
        {"type_id": " g6-dedicated-2"},
        {"type_id": 123},
        {"type_id": "g6/dedicated-2"},
        {"type_id": "g6?dedicated-2"},
        {"type_id": "g6#dedicated-2"},
        {"type_id": ".."},
        {"type_id": "g6-dedicated-2", "page": 0},
        {"type_id": "g6-dedicated-2", "page_size": 501},
    ],
)
async def test_handle_linode_database_type_get_rejects_invalid_inputs(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid path/pagination inputs before a client call."""
    result = await handle_linode_database_type_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_databases_types_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns the proto-canonical database type list envelope."""
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "g6-dedicated-2", "label": "Dedicated 4GB"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_database_type_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "database_types": [
            {
                "id": "g6-dedicated-2",
                "label": "Dedicated 4GB",
                "class": "",
                "disk": 0,
                "memory": 0,
                "vcpus": 0,
                "deprecated": False,
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_database_type_list", query="page=2&page_size=50"
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
async def test_handle_linode_databases_types_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid pagination before creating a client call."""
    result = await handle_linode_database_type_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_databases_types_list_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    type_entry = entries["linode_database_type_get"]
    assert type_entry.capability is Capability.Read
    assert type_entry.tool.name == "linode_database_type_get"
    assert type_entry.handle_fn is handle_linode_database_type_get

    entry = entries["linode_database_type_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_database_type_list"
    assert entry.handle_fn is handle_linode_database_type_list


@pytest.mark.asyncio
async def test_handle_linode_databases_engines_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns the proto-canonical database engine list envelope."""
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "mysql/8.0.26", "engine": "mysql", "version": "8.0.26"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_database_engine_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "database_engines": [
            {"id": "mysql/8.0.26", "engine": "mysql", "version": "8.0.26"}
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_database_engine_list", query="page=2&page_size=50"
    )


@pytest.mark.asyncio
async def test_handle_linode_databases_engines_list_empty(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler emits count 0 and an empty list when the page has no engines."""
    mock_linode_client.route_raw.return_value = {
        "data": [],
        "page": 1,
        "pages": 1,
        "results": 0,
    }

    result = await handle_linode_database_engine_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {"count": 0, "database_engines": []}
