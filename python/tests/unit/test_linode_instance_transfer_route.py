"""Tests for getting monthly transfer stats for a Linode instance."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar

import pytest

from linodemcp.gentools import (
    create_linode_instance_transfer_month_get_tool,
    handle_linode_instance_transfer_month_get,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

T = TypeVar("T")


def test_create_linode_instance_transfer_month_get_tool_schema() -> None:
    tool, capability = create_linode_instance_transfer_month_get_tool()

    assert tool.name == "linode_instance_transfer_month_get"
    assert capability is Capability.Read
    assert tool.input_schema == schema("linode.mcp.v1.InstanceTransferMonthGetInput")
    assert tool.input_schema["required"] == ["linode_id", "year", "month"]
    for field in ("linode_id", "year", "month"):
        assert tool.input_schema["properties"][field]["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_instance_transfer_month_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    # The month endpoint returns bytes_in/bytes_out/bytes_total (a distinct shape
    # from the current-month billable/quota/used); int64 byte counts stay JSON
    # numbers under the canonical serializer.
    mock_linode_client.route_raw.return_value = {
        "bytes_in": 30471077120,
        "bytes_out": 22956600198,
        "bytes_total": 53427677318,
    }

    result = await handle_linode_instance_transfer_month_get(
        {"linode_id": 123, "year": 2024, "month": 5}, sample_config
    )

    assert json.loads(result[0].text) == {
        "bytes_in": 30471077120,
        "bytes_out": 22956600198,
        "bytes_total": 53427677318,
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_instance_transfer_month_get", 123, 2024, 5
    )


def test_linode_instance_transfer_month_get_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_transfer_month_get"]
    assert entry.capability is Capability.Read
    assert entry.handle_fn is handle_linode_instance_transfer_month_get
