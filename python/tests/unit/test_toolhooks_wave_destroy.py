"""The hooks the seven single-id destroys own: the resource each previews and
plans over, and the walks that name what a delete takes with it.

The Go twin is ``wave_destroy_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_DEVICES = {
    "data": [{"id": 5, "entity": {"id": 9, "type": "linode", "label": "web-01"}}]
}
_SUBNETS = [{"id": 3, "label": "app", "linodes": [{"id": 1}, {"id": 2}]}]
_CONFIGS = {"data": [{"id": 4, "protocol": "https", "port": 443}]}

pytestmark = pytest.mark.asyncio


async def test_firewall_walk_names_the_devices() -> None:
    """The resources a firewall guards survive the delete but lose its rules."""
    client = AsyncMock()
    client.list_firewall_devices.return_value = _DEVICES

    details = await toolhooks.linode_firewall_delete_dependency_walk(client, 77, None)

    assert details.get("dependencies") == [
        {
            "kind": "linode",
            "id": 9,
            "label": "web-01",
            "action": "removed",
            "note": "Loses this firewall's rules when the firewall is deleted.",
        }
    ]
    assert details.get("warnings") == [
        "1 resource(s) currently use this firewall and will lose its rules."
    ]


async def test_vpc_walk_counts_detached_interfaces() -> None:
    """The subnets go with the VPC and their Linode interfaces are detached."""
    client = AsyncMock()
    client.list_vpc_subnets.return_value = _SUBNETS

    details = await toolhooks.linode_vpc_delete_dependency_walk(
        client, 77, DeclaredState({})
    )

    assert details.get("dependencies") == [
        {
            "kind": "vpc_subnet",
            "id": 3,
            "label": "app",
            "action": "cascade_deleted",
            "note": "2 attached Linode interface(s)",
        }
    ]
    assert details.get("warnings") == [
        "2 Linode interface(s) across 1 subnet(s) will be detached."
    ]


async def test_vpc_walk_stays_quiet_without_interfaces() -> None:
    """An empty subnet still cascades, but nothing is detached over it."""
    client = AsyncMock()
    client.list_vpc_subnets.return_value = [{"id": 3, "label": "app", "linodes": []}]

    details = await toolhooks.linode_vpc_delete_dependency_walk(
        client, 77, DeclaredState({})
    )

    assert len(details.get("dependencies", [])) == 1
    assert "warnings" not in details


async def test_nodebalancer_walk_names_the_configs() -> None:
    """Each config takes its backend node list with the NodeBalancer."""
    client = AsyncMock()
    client.list_nodebalancer_configs.return_value = _CONFIGS

    details = await toolhooks.linode_nodebalancer_delete_dependency_walk(
        client, 77, None
    )

    assert details.get("dependencies") == [
        {
            "kind": "nodebalancer_config",
            "id": 4,
            "action": "cascade_deleted",
            "note": "https config on port 443",
        }
    ]
    assert details.get("warnings") == [
        "Deleting this NodeBalancer destroys 1 config(s) and their backend node lists."
    ]


@pytest.mark.parametrize(
    ("hook", "reader", "wanted"),
    [
        (
            "linode_firewall_delete_dependency_walk",
            "list_firewall_devices",
            "Could not list firewall devices: ",
        ),
        (
            "linode_vpc_delete_dependency_walk",
            "list_vpc_subnets",
            "Could not list VPC subnets: ",
        ),
        (
            "linode_nodebalancer_delete_dependency_walk",
            "list_nodebalancer_configs",
            "Could not list NodeBalancer configs: ",
        ),
    ],
)
async def test_walks_degrade_to_a_warning(hook: str, reader: str, wanted: str) -> None:
    """Each walk is best-effort, so a fetch it cannot make leaves a warning and a
    previewable answer rather than refusing the preview outright.
    """
    client = AsyncMock()
    getattr(client, reader).side_effect = APIError(500, "boom")

    details = await getattr(toolhooks, hook)(client, 77, None)

    assert "dependencies" not in details
    warnings = details.get("warnings", [])
    assert len(warnings) == 1
    assert warnings[0].startswith(wanted)
