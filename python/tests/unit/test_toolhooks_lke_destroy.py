"""The hooks the four generated LKE destroys own: the state each previews and
plans over, and the walk that counts what a cluster delete cascades to.

The Go twin is ``lke_destroy_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_CLUSTER = {
    "id": 123,
    "label": "lke-prod",
    "region": "us-east",
    "k8s_version": "1.29",
    "status": "ready",
}
_POOLS = [
    {"id": 7, "type": "g6-standard-2", "count": 3},
    {"id": 8, "type": "g6-standard-4", "count": 2},
]
_NODE_WARNING = (
    "Deleting this cluster destroys 2 node pool(s) and 5 node(s);"
    " running workloads are lost."
)

pytestmark = pytest.mark.asyncio


async def test_dependency_walk_counts_the_nodes_it_destroys() -> None:
    """The pools cascade and the nodes under them carry the running workloads,
    which is the part of the delete a caller cannot put back."""
    client = AsyncMock()
    client.list_lke_node_pools.return_value = _POOLS

    details = await toolhooks.linode_lke_cluster_delete_dependency_walk(
        client, 123, DeclaredState({"id": 123})
    )

    assert details.get("dependencies") == [
        {
            "kind": "node_pool",
            "id": 7,
            "action": "cascade_deleted",
            "note": "3 node(s) of type g6-standard-2",
        },
        {
            "kind": "node_pool",
            "id": 8,
            "action": "cascade_deleted",
            "note": "2 node(s) of type g6-standard-4",
        },
    ]
    assert details.get("warnings") == [_NODE_WARNING]


@pytest.mark.parametrize(
    ("pools", "want_dependencies"),
    [
        ([{"id": 9, "type": "g6-standard-1", "count": 0}], 1),
        ([], 0),
    ],
    ids=["pool holding no nodes", "no pools at all"],
)
async def test_dependency_walk_leaves_out_an_empty_node_count(
    pools: list[dict[str, Any]], want_dependencies: int
) -> None:
    """A pool with no nodes still cascades, so it is listed, but there is no
    workload to warn about and the sentence that names one stays off."""
    client = AsyncMock()
    client.list_lke_node_pools.return_value = pools

    details = await toolhooks.linode_lke_cluster_delete_dependency_walk(
        client, 123, DeclaredState({"id": 123})
    )

    assert len(details.get("dependencies", [])) == want_dependencies
    assert "warnings" not in details


async def test_dependency_walk_degrades_to_a_warning() -> None:
    """A preview without the pool picture is still worth more to a caller
    deciding whether to proceed than no preview at all."""
    client = AsyncMock()
    client.list_lke_node_pools.side_effect = APIError(500, "boom")

    details = await toolhooks.linode_lke_cluster_delete_dependency_walk(
        client, 123, DeclaredState({"id": 123})
    )

    assert "dependencies" not in details
    assert details.get("warnings") == [
        "Could not list node pools: Linode API error (status 500): boom"
    ]
