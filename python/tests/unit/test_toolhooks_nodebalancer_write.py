"""The hooks the NodeBalancer write tools own.

The Go twin is ``nodebalancer_write_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.linode import NodeBalancer, Transfer

if TYPE_CHECKING:
    from linodemcp.config import Config

_NB_PATH = "/nodebalancers/789"
_CONFIGS_PATH = "/nodebalancers/5/configs"
_CONFIG_PATH = "/nodebalancers/5/configs/7"
_REBUILD_PATH = "/nodebalancers/5/configs/7/rebuild"
_NODE_PATH = "/nodebalancers/5/configs/7/nodes/9"
_CONFIG = {"id": 7, "port": 80, "protocol": "http", "nodebalancer_id": 5}
_NODE = {"id": 9, "label": "web-1", "weight": 50}
_BILLING_WARNING = "Billing for the NodeBalancer starts immediately on creation."


def _stub_client() -> Any:
    """Build the async client stub every preview below fetches through."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_config_create_preview_reads_the_existing_configs(
    sample_config: Config,
) -> None:
    """The list fetch is what tells a caller which ports are already taken."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_nodebalancer_configs.return_value = {
            "data": [_CONFIG],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_config_create_preview(
            sample_config,
            {"nodebalancer_id": 5, "port": 443, "dry_run": True},
            "POST",
            _CONFIGS_PATH,
            {"port": 443},
        )

        mock_client.list_nodebalancer_configs.assert_awaited_once_with(5)

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["would_execute"]["path"] == _CONFIGS_PATH
    assert preview["current_state"] == [_CONFIG]


async def test_config_create_preview_reports_an_empty_page_as_a_list(
    sample_config: Config,
) -> None:
    """An empty page reads as [] rather than as a missing state."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_nodebalancer_configs.return_value = {"data": []}
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_config_create_preview(
            sample_config,
            {"nodebalancer_id": 5, "dry_run": True},
            "POST",
            _CONFIGS_PATH,
            None,
        )

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["current_state"] == []


async def test_config_create_preview_reports_an_unreadable_list(
    sample_config: Config,
) -> None:
    """A failed read is a tool error, not a preview naming no starting point."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_nodebalancer_configs.side_effect = RuntimeError(
            "configs not found"
        )
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_config_create_preview(
            sample_config,
            {"nodebalancer_id": 5, "dry_run": True},
            "POST",
            _CONFIGS_PATH,
            None,
        )

    assert "configs not found" in result[0].text


@pytest.mark.parametrize(
    ("arguments", "effects"),
    [
        ({"label": "after"}, ['Label changes from "lb-1" to "after".']),
        ({"label": "lb-1"}, ['Label is set to "lb-1".']),
        (
            {"client_conn_throttle": 5},
            ["Connection throttle is set to 5 connections per second per client IP."],
        ),
        (
            {"client_conn_throttle": 0},
            ["Connection throttle is set to 0 connections per second per client IP."],
        ),
        ({"tags": ["prod"]}, []),
    ],
)
async def test_update_preview_names_the_change(
    sample_config: Config, arguments: dict[str, Any], effects: list[str]
) -> None:
    """The preview reads the NodeBalancer and names what the change replaces."""
    balancer = NodeBalancer(
        id=789,
        label="lb-1",
        region="us-east",
        hostname="nb-789.linode.com",
        ipv4="",
        ipv6="",
        client_conn_throttle=0,
        transfer=Transfer(in_=0, out=0, total=0),
        tags=[],
        created="",
        updated="",
    )

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.get_nodebalancer.return_value = balancer
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_update_preview(
            sample_config,
            {"nodebalancer_id": 789, "dry_run": True, **arguments},
            "PUT",
            _NB_PATH,
            None,
        )

        mock_client.get_nodebalancer.assert_awaited_once_with(789)

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["side_effects"] == effects


async def test_update_preview_reports_an_unreadable_balancer(
    sample_config: Config,
) -> None:
    """A failed read is a tool error, not a preview diffing against nothing."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.get_nodebalancer.side_effect = RuntimeError("nodebalancer missing")
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_update_preview(
            sample_config,
            {"nodebalancer_id": 789, "label": "after", "dry_run": True},
            "PUT",
            _NB_PATH,
            None,
        )

    assert "nodebalancer missing" in result[0].text
