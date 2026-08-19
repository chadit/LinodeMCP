"""The hooks the multi-slot generated destroys own: the state each previews and
plans over, and the walk that names what the removal takes with it.

The Go twin is ``slot_destroy_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_GROUP = DeclaredState(
    {
        "id": 123,
        "label": "pg-rack",
        "region": "us-east",
        "members": [{"linode_id": 11}, {"linode_id": 22}],
    }
)
_SHARE_GROUP = {"uuid": "22222222-2222-2222-2222-222222222222", "label": "share-team"}
_ACL = {"acl": {"enabled": True, "addresses": {"ipv4": ["10.0.0.1/32"]}}}
_POOL = DeclaredState(
    {
        "id": 7,
        "type": "g6-standard-2",
        "count": 2,
        "nodes": [
            {"id": "node-a", "instance_id": 11, "status": "ready"},
            {"id": "node-b", "instance_id": 22, "status": "ready"},
        ],
    }
)
_NODE = DeclaredState({"id": "node-a", "instance_id": 11, "status": "ready"})
_TAGGED_PAGE = {
    "data": [
        {"type": "linode", "data": {"id": 11, "label": "web-01"}},
        {"data": {"id": 22}},
    ],
    "page": 1,
    "pages": 2,
    "results": 9,
}
_INSTANCE = {"id": 5, "label": "web-01", "image": "linode/ubuntu24.04"}
_DISKS = [{"id": 1, "label": "boot", "size": 25600, "filesystem": "ext4"}]

pytestmark = pytest.mark.asyncio


async def test_placement_group_delete_walk_detaches_every_member() -> None:
    """The instances survive the group, so each is detached rather than deleted
    and the warning counts them."""
    details = await toolhooks.linode_placement_group_delete_dependency_walk(
        AsyncMock(), 123, _GROUP
    )

    assert [dep["id"] for dep in details.get("dependencies", [])] == [11, 22]
    assert all(dep["action"] == "detached" for dep in details.get("dependencies", []))
    assert "detaches 2 Linode(s)" in details.get("warnings", [])[0]


@pytest.mark.parametrize("state", [DeclaredState({"members": []}), DeclaredState({})])
async def test_placement_group_delete_walk_says_nothing_without_members(
    state: Any,
) -> None:
    """A group with no members, and one whose read answered without the member,
    both report the route alone rather than guessing at dependencies."""
    details = await toolhooks.linode_placement_group_delete_dependency_walk(
        AsyncMock(), 123, state
    )

    assert details == {}


async def test_lke_pool_delete_walk_names_every_backing_linode() -> None:
    """Each node's Linode goes with the pool, and the warning counts them
    because the running workloads are what a caller cannot put back."""
    details = await toolhooks.linode_lke_pool_delete_dependency_walk(
        AsyncMock(), 123, 7, _POOL
    )

    assert [dep["label"] for dep in details.get("dependencies", [])] == [
        "node-a",
        "node-b",
    ]
    assert all(
        dep["action"] == "cascade_deleted" for dep in details.get("dependencies", [])
    )
    assert "destroys 2 node(s)" in details.get("warnings", [])[0]


@pytest.mark.parametrize(
    "state", [DeclaredState({"id": 7, "count": 0, "nodes": []}), DeclaredState({})]
)
async def test_lke_pool_delete_walk_says_nothing_without_nodes(state: Any) -> None:
    """A pool holding no nodes destroys no workload, and one whose read answered
    without the members reports the route alone."""
    details = await toolhooks.linode_lke_pool_delete_dependency_walk(
        AsyncMock(), 123, 7, state
    )

    assert details == {}


async def test_lke_node_delete_walk_names_the_backing_linode() -> None:
    """The Linode goes with the node, and the pool effect is said either way."""
    details = await toolhooks.linode_lke_node_delete_dependency_walk(
        AsyncMock(), 123, "node-a", _NODE
    )

    assert details.get("dependencies", [])[0]["id"] == 11
    assert "pool node count" in details.get("warnings", [])[0]


async def test_lke_node_delete_walk_still_reports_an_unbacked_node() -> None:
    """A node with no Linode yet cascades to nothing, and the pool effect holds."""
    details = await toolhooks.linode_lke_node_delete_dependency_walk(
        AsyncMock(), 123, "node-a", DeclaredState({"id": "node-a", "instance_id": 0})
    )

    assert "dependencies" not in details
    assert len(details.get("warnings", [])) == 1


async def test_lke_node_delete_walk_reports_a_node_missing_its_members() -> None:
    """A read that answered without the members reports the pool effect alone,
    which is the only thing left to say about it."""
    details = await toolhooks.linode_lke_node_delete_dependency_walk(
        AsyncMock(), 123, "node-a", DeclaredState({})
    )

    assert "dependencies" not in details
    assert len(details.get("warnings", [])) == 1


async def test_tag_delete_fetch_state_lists_the_tagged_objects() -> None:
    """The page is what the preview reports and what the walk names one by one."""
    client = AsyncMock()
    client.list_tagged_objects.return_value = _TAGGED_PAGE

    assert await toolhooks.linode_tag_delete_fetch_state(client, "prod") == _TAGGED_PAGE
    client.list_tagged_objects.assert_awaited_once_with("prod")


async def test_tag_delete_walk_counts_beyond_the_first_page() -> None:
    """The itemized list is the page and the count is the envelope's total, so a
    truncated first page cannot understate the blast radius."""
    details = await toolhooks.linode_tag_delete_dependency_walk(
        AsyncMock(), "prod", _TAGGED_PAGE
    )

    # The second object declares no type, so it falls back to the generic kind.
    assert [dep["kind"] for dep in details.get("dependencies", [])] == [
        "linode",
        "resource",
    ]
    assert "from 9 tagged object(s)" in details.get("warnings", [])[0]
    assert "first 2 tagged object(s)" in details.get("warnings", [])[1]


@pytest.mark.parametrize(
    "state",
    [{"data": [], "results": 0}, {"data": "not a list"}, "not a page", None],
)
async def test_tag_delete_walk_says_nothing_for_an_untagged_label(state: Any) -> None:
    """A tag on nothing removes nothing, whatever shape the state arrived in."""
    details = await toolhooks.linode_tag_delete_dependency_walk(
        AsyncMock(), "prod", state
    )

    assert details == {}


async def test_instance_rebuild_walk_names_every_disk_and_the_image() -> None:
    """A rebuild erases the disks and replaces the image, and both are said."""
    client = AsyncMock()
    client.list_instance_disks.return_value = _DISKS

    details = await toolhooks.linode_instance_rebuild_dependency_walk(
        client, 5, DeclaredState(_INSTANCE)
    )

    assert details.get("side_effects", []) == [
        'Disk "boot" (25600 MB, ext4) is erased and recreated from the new image.'
    ]
    assert '"linode/ubuntu24.04"' in details.get("warnings", [])[0]


async def test_instance_rebuild_walk_falls_back_without_an_image() -> None:
    """An instance carrying no image still gets the data-loss warning."""
    client = AsyncMock()
    client.list_instance_disks.return_value = []

    details = await toolhooks.linode_instance_rebuild_dependency_walk(
        client, 5, DeclaredState({})
    )

    assert details == {
        "warnings": [
            "Rebuild destroys all data on the instance and resets the root password."
        ]
    }


async def test_instance_rebuild_walk_warns_on_a_failed_disk_list() -> None:
    """The preview is still worth having without the disk picture, so a failed
    list is a warning rather than a refusal."""
    client = AsyncMock()
    client.list_instance_disks.side_effect = APIError(500, "boom")

    details = await toolhooks.linode_instance_rebuild_dependency_walk(
        client, 5, DeclaredState(_INSTANCE)
    )

    assert "side_effects" not in details
    assert "Could not list instance disks" in details.get("warnings", [])[0]
    assert '"linode/ubuntu24.04"' in details.get("warnings", [])[1]
