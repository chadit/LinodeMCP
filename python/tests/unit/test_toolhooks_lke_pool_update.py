"""The preview hook linode_lke_pool_update owns.

The pool read is what the resize sentence is diffed against: the count a resize
starts from is the one number the arguments cannot supply. The Go twin is
``lke_pool_preview_test.go`` in ``go/internal/toolhooks``, and every sentence
below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config

_PATH = "/lke/clusters/123/pools/10"
_IDS = {"cluster_id": 123, "pool_id": 10}


def _patched_client(pool: dict[str, Any]) -> Any:
    """Answer the node-pool read the preview makes."""
    mock_client = AsyncMock()
    mock_client.get_lke_node_pool.return_value = pool
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    return mock_client


@pytest.mark.parametrize(
    ("pool", "arguments", "effects"),
    [
        (
            {"id": 10, "cluster_id": 123, "count": 2},
            {"count": 5},
            ["Node pool resizes from 2 to 5 node(s)."],
        ),
        (
            {"id": 10, "cluster_id": 123, "count": 0},
            {"count": 5},
            ["Node pool is set to 5 node(s)."],
        ),
        (
            {"id": 10, "cluster_id": 123, "count": 5},
            {"count": 5},
            ["Node pool is set to 5 node(s)."],
        ),
        (
            {"id": 10, "cluster_id": 123, "count": 2},
            {"count": None},
            ["Node pool resizes from 2 to 0 node(s)."],
        ),
        (
            {"id": 10, "cluster_id": 123, "count": 2},
            {"count": 5, "autoscaler": {"enabled": True}},
            [
                "Node pool resizes from 2 to 5 node(s).",
                "The pool autoscaler configuration is updated.",
            ],
        ),
        (
            {"id": 10, "cluster_id": 123, "count": 2},
            {"autoscaler": {}},
            ["The pool autoscaler configuration is updated."],
        ),
        ({"id": 10, "cluster_id": 123, "count": 2}, {"tags": ["web"]}, []),
    ],
)
async def test_lke_pool_update_preview_names_the_count_change(
    sample_config: Config,
    pool: dict[str, Any],
    arguments: dict[str, Any],
    effects: list[str],
) -> None:
    """The preview reads the pool and names the count it starts from."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _patched_client(pool)
        mock_cls.return_value = mock_client

        result: list[TextContent] = await toolhooks.linode_lke_pool_update_preview(
            sample_config,
            {**_IDS, **arguments, "dry_run": True},
            "PUT",
            _PATH,
            dict(arguments),
        )

        mock_client.get_lke_node_pool.assert_awaited_once_with(123, 10)

    body: dict[str, Any] = json.loads(result[0].text)

    assert body["dry_run"] is True
    assert body["tool"] == "linode_lke_pool_update"
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == _PATH
    assert body["current_state"] == pool
    assert body.get("side_effects", []) == effects
