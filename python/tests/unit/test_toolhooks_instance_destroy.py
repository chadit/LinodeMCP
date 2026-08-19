"""The hooks the two generated instance destroys own: the state each previews
and plans over, and the walk that names what a Linode delete takes with it.

The Go twin is ``instance_destroy_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError
from linodemcp.tools.declared_state import DeclaredState

_INSTANCE = {
    "id": 123,
    "label": "web-01",
    "status": "running",
    "type": "g6-standard-2",
}
_VOLUMES = {"data": [{"id": 11, "label": "data-vol", "size": 50}]}
_IPS = {"ipv4": {"public": [{"address": "203.0.113.10"}]}}
_FIREWALLS = {"data": [{"id": 42, "label": "web-fw"}]}
_RUNNING_WARNING = (
    "Instance is currently running. Delete will not pause for a graceful shutdown."
)

pytestmark = pytest.mark.asyncio


def _walking_client(monthly: float = 20.0) -> AsyncMock:
    """A client whose every walk sub-fetch answers, which is the branch that
    fills the dependency list.
    """
    client = AsyncMock()
    client.list_instance_volumes.return_value = _VOLUMES
    client.list_instance_ips.return_value = _IPS
    client.list_instance_firewalls.return_value = _FIREWALLS
    client.get_type.return_value = type(
        "InstanceType", (), {"price": type("Price", (), {"monthly": monthly})()}
    )()
    return client


async def test_dependency_walk_names_every_dependent() -> None:
    """A delete detaches volumes, releases public addresses, and drops firewall
    attachments, none of which are visible from the request alone.
    """
    client = _walking_client()

    details = await toolhooks.linode_instance_delete_dependency_walk(
        client, 123, DeclaredState(_INSTANCE)
    )

    assert [dep["kind"] for dep in details.get("dependencies", [])] == [
        "volume",
        "public_ip",
        "firewall",
    ]
    assert [dep["action"] for dep in details.get("dependencies", [])] == [
        "detached",
        "released",
        "removed",
    ]
    assert details.get("billing_delta", {}).get("monthly_change_usd") == "-20.00"
    assert details.get("warnings") == [_RUNNING_WARNING]


async def test_dependency_walk_warns_per_failed_fetch() -> None:
    """Each sub-list is best-effort, so three failed fetches leave three
    warnings and a previewable answer rather than one error.
    """
    client = AsyncMock()
    client.list_instance_volumes.side_effect = APIError(500, "volumes down")
    client.list_instance_ips.side_effect = APIError(500, "ips down")
    client.list_instance_firewalls.side_effect = APIError(500, "firewalls down")
    client.get_type.side_effect = APIError(500, "pricing down")

    details = await toolhooks.linode_instance_delete_dependency_walk(
        client, 123, DeclaredState({**_INSTANCE, "status": "offline"})
    )

    assert "dependencies" not in details
    assert [warning.split(":")[0] for warning in details.get("warnings", [])] == [
        "Could not list attached volumes",
        "Could not list IP addresses",
        "Could not list firewalls",
    ]
    assert details.get("billing_delta") == {
        "monthly_change_usd": "unknown",
        "note": "Could not fetch type pricing for the estimate.",
    }


async def test_dependency_walk_reports_no_type_as_unknown() -> None:
    """A Linode whose state carries no type is priced as unknown without a note:
    nothing failed, there was just nothing to price.
    """
    client = _walking_client()

    details = await toolhooks.linode_instance_delete_dependency_walk(
        client, 123, DeclaredState({"id": 123, "label": "web-01"})
    )

    assert details.get("billing_delta") == {"monthly_change_usd": "unknown"}
    assert "warnings" not in details


async def test_dependency_walk_skips_addressless_ips() -> None:
    """A Linode with no IPv4 block reports no released addresses rather than an
    empty entry.
    """
    client = _walking_client()
    client.list_instance_ips.return_value = {}

    details = await toolhooks.linode_instance_delete_dependency_walk(
        client, 123, DeclaredState(_INSTANCE)
    )

    assert [dep["kind"] for dep in details.get("dependencies", [])] == [
        "volume",
        "firewall",
    ]
