"""The hooks the eleven cross-family destroys own: the resource each previews
and plans over, and the subnet walk that names what a delete detaches.

The Go twin is ``cross_destroy_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_SHARE_GROUP = {"id": 5, "label": "share-team"}
_SUBNET = DeclaredState(
    {
        "id": 8,
        "label": "subnet-a",
        "linodes": [{"id": 11, "interfaces": [{"id": 1}]}],
    }
)
_DETACH_WARNING = (
    '1 Linode(s) have interfaces in subnet "subnet-a" '
    '(VPC "{label}") and will be detached.'
)
_VLAN = {"label": "vl-app", "region": "us-east", "linodes": [11]}

pytestmark = pytest.mark.asyncio


async def test_vlan_fetch_state_filters_the_list() -> None:
    """VLANs expose only a list endpoint, so the read lists and filters."""
    client = AsyncMock()
    client.list_vlans.return_value = [
        {"label": "other", "region": "us-east", "linodes": []},
        _VLAN,
    ]

    state = await toolhooks.linode_vlan_delete_fetch_state(client, "us-east", "vl-app")

    assert state == _VLAN


async def test_vlan_fetch_state_asks_for_the_largest_page() -> None:
    """The lookup asks for the endpoint's largest page. On the API default page
    a VLAN past position 100 would read as not found here while Go, which has
    always sent the explicit size, previewed it.
    """
    client = AsyncMock()
    client.list_vlans.return_value = [_VLAN]

    await toolhooks.linode_vlan_delete_fetch_state(client, "us-east", "vl-app")

    client.list_vlans.assert_awaited_once_with(page=1, page_size=500)


async def test_vlan_fetch_state_reports_a_missing_vlan() -> None:
    """A VLAN the list does not carry is reported rather than answered as an
    empty resource, since the preview would otherwise describe a delete against
    something that is not there.
    """
    client = AsyncMock()
    client.list_vlans.return_value = [{"label": "other", "region": "us-east"}]

    with pytest.raises(ValueError, match="VLAN not found: vl-app in region us-east"):
        await toolhooks.linode_vlan_delete_fetch_state(client, "us-east", "vl-app")


async def test_subnet_walk_names_the_detached_linodes() -> None:
    """The fetched subnet carries the Linodes with interfaces in it, each
    detached rather than deleted, and the warning names the parent VPC.
    """
    client = AsyncMock()
    client.get_vpc.return_value = {"id": 77, "label": "prod-vpc"}

    details = await toolhooks.linode_vpc_subnet_delete_dependency_walk(
        client, 77, 8, _SUBNET
    )

    assert details.get("dependencies") == [
        {
            "kind": "instance",
            "id": 11,
            "action": "detached",
            "note": "1 interface(s) in this subnet are detached.",
        }
    ]
    assert details.get("warnings") == [_DETACH_WARNING.format(label="prod-vpc")]


async def test_subnet_walk_leaves_the_vpc_label_empty() -> None:
    """The VPC read is best-effort, so a failed one still reports the detached
    Linodes rather than refusing the preview.
    """
    client = AsyncMock()
    client.get_vpc.side_effect = APIError(500, "boom")

    details = await toolhooks.linode_vpc_subnet_delete_dependency_walk(
        client, 77, 8, _SUBNET
    )

    assert details.get("warnings") == [_DETACH_WARNING.format(label="")]


async def test_subnet_walk_says_nothing_without_linodes() -> None:
    """A subnet nothing is attached to takes nothing with it, so the walk makes
    no VPC read to label a warning it would not write.
    """
    client = AsyncMock()

    details = await toolhooks.linode_vpc_subnet_delete_dependency_walk(
        client, 77, 8, DeclaredState({"id": 8, "label": "subnet-a", "linodes": []})
    )

    assert details == {}
    client.get_vpc.assert_not_awaited()


async def test_subnet_walk_reports_a_subnet_carrying_no_linodes_member() -> None:
    """A state the API answered without the member reports no dependencies,
    which is what an unattached subnet means. A state the declared fetch did not
    produce never reaches here: the generated call refuses it first.
    """
    client = AsyncMock()

    details = await toolhooks.linode_vpc_subnet_delete_dependency_walk(
        client, 77, 8, DeclaredState({"id": 8, "label": "subnet-a"})
    )

    assert details == {}
