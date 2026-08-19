"""The hooks the sub-resource destroys own: the state each previews and plans
over, and the two walks that name what a removal takes with it.

These are the removals addressed by more than the resource's own id, plus the
password reset, whose only cost is downtime. The Go twin is
``instance_destroy_test.go`` and its family siblings in ``go/internal/toolhooks``,
and every sentence below is one both languages answer.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_CONFIG = {"id": 6, "label": "boot-config"}
_INTERFACE = {"id": 789, "purpose": "public"}
_DISK = {"id": 10, "label": "boot", "size": 25600}
_ADDRESS = "203.0.113.7"
_IP = {"address": _ADDRESS, "type": "ipv4", "public": True}
_CLUSTER = {"id": 123, "label": "prod", "k8s_version": "1.31"}
_POOL = {"id": 7, "type": "g6-standard-2", "count": 2}
_NODE = {"id": "node-a", "instance_id": 11, "status": "ready"}
_NB_CONFIG = {"id": 6, "port": 80, "protocol": "http"}
_NB_NODE = {"id": 9, "label": "backend-1", "address": "192.168.1.5:80"}
_RECORD = {"id": 7, "type": "A", "name": "www"}
_RUNNING = {"id": 5, "label": "web-01", "status": "running"}
_OFFLINE = {"id": 5, "label": "web-01", "status": "offline"}

_CONFIG_PAGE = {
    "data": [
        {"id": 6, "label": "boot-config", "devices": {"sda": {"disk_id": 10}}},
        {"id": 7, "label": "rescue", "devices": {"sda": {"volume_id": 3}}},
    ],
    "page": 1,
    "pages": 1,
    "results": 2,
}
_NB_NODE_PAGE = {
    "data": [
        {"id": 9, "label": "backend-1", "address": "192.168.1.5:80", "mode": "accept"},
    ],
    "page": 1,
    "pages": 1,
    "results": 1,
}


async def test_instance_disk_delete_walk_names_the_configs_pointing_at_it() -> None:
    """Only the profile whose device slot holds the disk is named."""
    client = AsyncMock()
    client.list_instance_configs.return_value = _CONFIG_PAGE

    details = await toolhooks.linode_instance_disk_delete_dependency_walk(
        client, 123, 10, None
    )

    assert details.get("dependencies", []) == [
        {
            "kind": "instance_config",
            "id": 6,
            "label": "boot-config",
            "action": "removed",
            "note": (
                "References this disk; its device slot is cleared when the "
                "disk is deleted."
            ),
        }
    ]
    assert details.get("warnings", []) == [
        "Config profiles reference this disk; deleting it leaves those slots empty."
    ]


async def test_instance_disk_delete_walk_says_nothing_for_an_unreferenced_disk() -> (
    None
):
    """A disk no profile points at leaves the preview with nothing to add."""
    client = AsyncMock()
    client.list_instance_configs.return_value = _CONFIG_PAGE

    details = await toolhooks.linode_instance_disk_delete_dependency_walk(
        client, 123, 99, None
    )

    assert details == {}


@pytest.mark.parametrize(
    "config",
    [
        {"id": 6, "label": "boot-config"},
        {"id": 6, "label": "boot-config", "devices": "sda"},
    ],
)
async def test_instance_disk_delete_walk_reads_past_a_deviceless_config(
    config: dict[str, Any],
) -> None:
    """A profile carrying no device map, or one that is not a map at all,
    references nothing rather than failing the preview."""
    client = AsyncMock()
    client.list_instance_configs.return_value = {"data": [config]}

    details = await toolhooks.linode_instance_disk_delete_dependency_walk(
        client, 123, 10, None
    )

    assert details == {}


async def test_instance_disk_delete_walk_warns_on_a_failed_config_list() -> None:
    """The preview is still worth having without the profile picture."""
    client = AsyncMock()
    client.list_instance_configs.side_effect = APIError(500, "boom")

    details = await toolhooks.linode_instance_disk_delete_dependency_walk(
        client, 123, 10, None
    )

    assert "Could not list instance configs" in details.get("warnings", [])[0]


async def test_nodebalancer_config_delete_walk_names_every_backend_node() -> None:
    """The config owns its backends, so each one leaves the rotation with it."""
    client = AsyncMock()
    client.list_nodebalancer_config_nodes.return_value = _NB_NODE_PAGE

    details = await toolhooks.linode_nodebalancer_config_delete_dependency_walk(
        client, 5, 6, None
    )

    assert details.get("dependencies", []) == [
        {
            "kind": "nodebalancer_node",
            "id": 9,
            "label": "backend-1",
            "action": "cascade_deleted",
            "note": "backend 192.168.1.5:80 (accept)",
        }
    ]
    assert details.get("warnings", []) == [
        "Deleting this config removes 1 backend node(s) from the rotation."
    ]


async def test_nodebalancer_config_delete_walk_says_nothing_without_backends() -> None:
    """A config with an empty rotation has nothing to report."""
    client = AsyncMock()
    client.list_nodebalancer_config_nodes.return_value = {"data": []}

    details = await toolhooks.linode_nodebalancer_config_delete_dependency_walk(
        client, 5, 6, None
    )

    assert details == {}


async def test_nodebalancer_config_delete_walk_warns_on_a_failed_node_list() -> None:
    """A failed list degrades to a warning rather than refusing the preview."""
    client = AsyncMock()
    client.list_nodebalancer_config_nodes.side_effect = APIError(500, "boom")

    details = await toolhooks.linode_nodebalancer_config_delete_dependency_walk(
        client, 5, 6, None
    )

    assert "Could not list config backend nodes" in details.get("warnings", [])[0]


async def test_password_reset_walk_warns_while_the_instance_is_running() -> None:
    """A running Linode loses service while the reset cycles it."""
    client = AsyncMock()

    details = await toolhooks.linode_instance_password_reset_dependency_walk(
        client, 5, DeclaredState(_RUNNING)
    )

    assert details.get("side_effects", []) == [
        "The instance is powered down and rebooted to apply the new root password."
    ]
    assert details.get("warnings", []) == [
        (
            "Instance is currently running; the reset shuts it down and "
            "reboots it, causing downtime."
        )
    ]


async def test_password_reset_walk_names_the_reboot_on_a_stopped_instance() -> None:
    """The cycle happens either way, so the side effect is reported alone."""
    client = AsyncMock()

    details = await toolhooks.linode_instance_password_reset_dependency_walk(
        client, 5, DeclaredState(_OFFLINE)
    )

    assert details == {
        "side_effects": [
            "The instance is powered down and rebooted to apply the new root password."
        ]
    }
