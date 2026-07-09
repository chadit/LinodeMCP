"""Tests for Linode instance monthly statistics route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import Server, get_tool_registry
from linodemcp.tools.linode_instances import (
    create_linode_instance_stats_month_get_tool,
    handle_linode_instance_stats_month_get,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config


async def test_get_instance_stats_by_year_month_sends_exact_route() -> None:
    """Client sends the documented monthly stats route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"cpu": [[1719792000, 1.25]], "io": {}}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_instance_stats_by_year_month(123, 2024, 7)

    assert result["cpu"] == [[1719792000, 1.25]]
    mock_request.assert_awaited_once_with("GET", "/linode/instances/123/stats/2024/7")

    await client.close()


@pytest.mark.parametrize(
    ("linode_id", "year", "month", "message"),
    [
        (0, 2024, 7, "linode_id"),
        (True, 2024, 7, "linode_id"),
        ("12/3", 2024, 7, "linode_id"),
        (123, 1969, 7, "year"),
        (123, "2024?x=1", 7, "year"),
        (123, 2024, 0, "month"),
        (123, 2024, "..", "month"),
    ],
)
async def test_get_instance_stats_by_year_month_rejects_invalid_path_params(
    linode_id: object, year: object, month: object, message: str
) -> None:
    """Client rejects invalid monthly stats path parameters before requests."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match=message),
    ):
        await client.get_instance_stats_by_year_month(
            cast("Any", linode_id), cast("Any", year), cast("Any", month)
        )

    mock_request.assert_not_called()

    await client.close()


async def test_get_instance_stats_by_year_month_handles_non_object_json() -> None:
    """Client returns an empty mapping for unexpected non-object stats responses."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = []

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_instance_stats_by_year_month(123, 2024, 7)

    assert result == {}
    mock_request.assert_awaited_once_with("GET", "/linode/instances/123/stats/2024/7")

    await client.close()


async def test_get_instance_stats_by_year_month_wraps_http_errors() -> None:
    """Client wraps monthly stats HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_instance_stats_by_year_month(123, 2024, 7)

    assert "GetInstanceStatsByYearMonth" in str(exc_info.value)

    await client.close()


async def test_retryable_get_instance_stats_by_year_month_delegates() -> None:
    """Retryable client delegates monthly stats retrieval."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_get = AsyncMock(return_value={"cpu": []})
    object.__setattr__(client.client, "get_instance_stats_by_year_month", mock_get)

    try:
        result = await client.get_instance_stats_by_year_month(123, 2024, 7)
    finally:
        await client.close()

    assert result == {"cpu": []}
    mock_get.assert_awaited_once_with(123, 2024, 7)


def test_linode_instance_stats_month_get_tool_schema() -> None:
    """Monthly stats tool advertises the proto-generated input schema.

    The handler enforces the year/month ranges; the generated schema carries
    only the proto int32 bounds.
    """
    tool, capability = create_linode_instance_stats_month_get_tool()

    assert tool.name == "linode_instance_stats_month_get"
    assert capability is Capability.Read
    assert tool.inputSchema == schema("linode.mcp.v1.InstanceStatsMonthGetInput")
    assert tool.inputSchema["required"] == ["linode_id", "year", "month"]


async def test_handle_linode_instance_stats_month_get_success(
    sample_config: Config,
) -> None:
    """Handler returns monthly stats from the retryable client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        # The real API nests the graphs under a top-level "data" object; the
        # proto now models that wrapper.
        mock_client.get_instance_stats_by_year_month.return_value = {
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
    mock_client.get_instance_stats_by_year_month.assert_awaited_once_with(123, 2024, 7)


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
