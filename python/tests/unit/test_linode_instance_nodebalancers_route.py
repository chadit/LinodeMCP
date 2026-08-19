"""Tests for listing NodeBalancers assigned to a Linode instance."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar

import pytest

from linodemcp.gentools import (
    create_linode_instance_nodebalancer_list_tool,
    handle_linode_instance_nodebalancer_list,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.toolschemas import schema
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

T = TypeVar("T")


def test_create_linode_instance_nodebalancers_list_tool_schema() -> None:
    tool, capability = create_linode_instance_nodebalancer_list_tool()

    assert tool.name == "linode_instance_nodebalancer_list"
    assert capability is Capability.Read
    assert tool.input_schema == schema("linode.mcp.v1.InstanceNodeBalancerListInput")
    assert tool.input_schema["required"] == ["linode_id"]
    assert tool.input_schema["properties"]["linode_id"]["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_instance_nodebalancers_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": 456, "label": "nb-1"}],
        "page": 1,
    }

    result = await handle_linode_instance_nodebalancer_list(
        {"linode_id": 123}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert "filter" not in payload
    assert payload["nodebalancers"][0]["id"] == 456
    assert payload["nodebalancers"][0]["label"] == "nb-1"
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_nodebalancer_list", 123, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_handle_linode_instance_nodebalancers_list_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_nodebalancer_list(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_instance_nodebalancers_list_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_nodebalancer_list"]
    assert entry.capability is Capability.Read
    assert entry.handle_fn is handle_linode_instance_nodebalancer_list


def test_linode_instance_nodebalancers_list_in_version_features() -> None:
    assert "linode_instance_nodebalancer_list" in FEATURE_TOOLS_LIST.split(",")
