"""Tests for Linode instance monthly statistics route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.gentools import (
    create_linode_instance_stats_month_get_tool,
    handle_linode_instance_stats_month_get,
)
from linodemcp.profiles import Capability
from linodemcp.server import Server, get_tool_registry
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config


def test_linode_instance_stats_month_get_tool_schema() -> None:
    """Monthly stats tool advertises the proto-generated input schema.

    The handler enforces the year/month ranges; the generated schema carries
    only the proto int32 bounds.
    """
    tool, capability = create_linode_instance_stats_month_get_tool()

    assert tool.name == "linode_instance_stats_month_get"
    assert capability is Capability.Read
    assert tool.input_schema == schema("linode.mcp.v1.InstanceStatsMonthGetInput")
    assert tool.input_schema["required"] == ["linode_id", "year", "month"]


async def test_handle_linode_instance_stats_month_get_success(
    sample_config: Config,
) -> None:
    """Handler returns monthly stats from the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        # The real API nests the graphs under a top-level "data" object; the
        # proto now models that wrapper.
        mock_client.route_raw.return_value = {
            "title": "linode123 stats",
            "data": {"cpu": [[1719792000, 1.25]], "io": {}},
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_stats_month_get(
            {"linode_id": 123, "year": 2024, "month": 7}, sample_config
        )

    payload: dict[str, Any] = json.loads(result[0].text)
    assert payload["title"] == "linode123 stats"
    assert payload["data"]["cpu"][0][1] == 1.25
    mock_client.route_raw.assert_awaited_once_with(
        "linode_instance_stats_month_get", 123, 2024, 7
    )


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"linode_id": "123", "year": 2024, "month": 7}, "linode_id"),
        ({"linode_id": 123, "year": "2024", "month": 7}, "year"),
        ({"linode_id": 123, "year": 2024, "month": "7"}, "month"),
        ({"linode_id": 123, "year": 2024, "month": 13}, "month"),
        ({"linode_id": 123, "year": 1969, "month": 7}, "year"),
        ({"linode_id": "/", "year": 2024, "month": 7}, "linode_id"),
        ({"linode_id": "?", "year": 2024, "month": 7}, "linode_id"),
        ({"linode_id": "..", "year": 2024, "month": 7}, "linode_id"),
        ({"linode_id": 123, "year": "/", "month": 7}, "year"),
        ({"linode_id": 123, "year": "?", "month": 7}, "year"),
        ({"linode_id": 123, "year": "..", "month": 7}, "year"),
        ({"linode_id": 123, "year": 2024, "month": "/"}, "month"),
        ({"linode_id": 123, "year": 2024, "month": "?"}, "month"),
        ({"linode_id": 123, "year": 2024, "month": ".."}, "month"),
        # Omitted year/month parse to None, so the explicit None guards reject
        # them rather than the value validators.
        ({"linode_id": 123, "month": 7}, "year must be an integer"),
        ({"linode_id": 123, "year": 2024}, "month must be an integer"),
    ],
)
async def test_handle_linode_instance_stats_month_get_rejects_invalid_path_params(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    """Handler rejects malformed path params before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_instance_stats_month_get(arguments, sample_config)

    assert message in result[0].text
    mock_client_class.assert_not_called()


def test_linode_instance_stats_month_get_exported_and_registered(
    sample_config: Config,
) -> None:
    """Monthly stats tool is exported and registered."""
    registry_names = {entry.name for entry in get_tool_registry()}
    assert "linode_instance_stats_month_get" in registry_names

    srv = Server(sample_config)
    assert "linode_instance_stats_month_get" in srv.registered_tool_names
