"""Two-stage parity tests for the non-instance opted-in delete tools.

Covers volume, LKE cluster, firewall, NodeBalancer, VPC, and domain. The
instance test carries the full refusal matrix (drift, expiry, unknown,
args-mismatch); these prove each other opted-in delete tool also runs the
plan/apply flow and honors its per-type HashIgnore list, so a cosmetic
timestamp bump does not refuse the apply.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.tools.linode_domain_records import handle_linode_domain_record_delete
from linodemcp.tools.linode_domains_write import handle_linode_domain_delete
from linodemcp.tools.linode_firewalls_write import handle_linode_firewall_delete
from linodemcp.tools.linode_images import handle_linode_image_delete
from linodemcp.tools.linode_instance_disks import handle_linode_instance_disk_delete
from linodemcp.tools.linode_lke_write import (
    handle_linode_lke_cluster_delete,
    handle_linode_lke_pool_delete,
)
from linodemcp.tools.linode_nodebalancers_write import (
    handle_linode_nodebalancer_delete,
)
from linodemcp.tools.linode_placement_groups_write import (
    handle_linode_placement_group_delete,
)
from linodemcp.tools.linode_sshkeys_write import handle_linode_sshkey_delete
from linodemcp.tools.linode_stackscripts import handle_linode_stackscript_delete
from linodemcp.tools.linode_volumes_write import handle_linode_volume_delete
from linodemcp.tools.linode_vpc_write import (
    handle_linode_vpc_delete,
    handle_linode_vpc_subnet_delete,
)
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable
    from unittest.mock import AsyncMock

    from mcp.types import TextContent

    from linodemcp.config import Config

    type _DeleteHandler = Callable[
        [dict[str, Any], Config], Awaitable[list[TextContent]]
    ]

_VOLUME = pytest.param(
    handle_linode_volume_delete,
    "volume_id",
    123,
    "get_volume",
    "delete_volume",
    id="volume",
)
_LKE = pytest.param(
    handle_linode_lke_cluster_delete,
    "cluster_id",
    123,
    "get_lke_cluster",
    "delete_lke_cluster",
    id="lke_cluster",
)
_FIREWALL = pytest.param(
    handle_linode_firewall_delete,
    "firewall_id",
    123,
    "get_firewall",
    "delete_firewall",
    id="firewall",
)
_NODEBALANCER = pytest.param(
    handle_linode_nodebalancer_delete,
    "nodebalancer_id",
    123,
    "get_nodebalancer",
    "delete_nodebalancer",
    id="nodebalancer",
)
_VPC = pytest.param(
    handle_linode_vpc_delete, "vpc_id", 123, "get_vpc", "delete_vpc", id="vpc"
)
_DOMAIN = pytest.param(
    handle_linode_domain_delete,
    "domain_id",
    123,
    "get_domain",
    "delete_domain",
    id="domain",
)
_STACKSCRIPT = pytest.param(
    handle_linode_stackscript_delete,
    "stackscript_id",
    123,
    "get_stackscript",
    "delete_stackscript",
    id="stackscript",
)
_SSHKEY = pytest.param(
    handle_linode_sshkey_delete,
    "ssh_key_id",
    123,
    "get_ssh_key",
    "delete_ssh_key",
    id="sshkey",
)
_PLACEMENT = pytest.param(
    handle_linode_placement_group_delete,
    "group_id",
    123,
    "get_placement_group",
    "delete_placement_group",
    id="placement_group",
)
_IMAGE = pytest.param(
    handle_linode_image_delete,
    "image_id",
    "private/123",
    "get_image",
    "delete_image",
    id="image",
)

# Every opted-in delete tool: proves the plan/apply round trip works.
_ALL_CASES = [
    _VOLUME,
    _LKE,
    _FIREWALL,
    _NODEBALANCER,
    _VPC,
    _DOMAIN,
    _STACKSCRIPT,
    _SSHKEY,
    _PLACEMENT,
    _IMAGE,
]

# Subset whose HashIgnore list strips "updated"; only these can prove a
# cosmetic timestamp bump is ignored. Image, SSH key, and placement group have
# no cosmetic field, so a real "updated" change is genuine drift for them.
_COSMETIC_CASES = [
    _VOLUME,
    _LKE,
    _FIREWALL,
    _NODEBALANCER,
    _VPC,
    _DOMAIN,
    _STACKSCRIPT,
]


def _state(updated: str) -> dict[str, Any]:
    return {"id": 123, "status": "active", "updated": updated}


def _stub_walk_calls(client: AsyncMock) -> None:
    """Stub the sub-fetches the firewall and NodeBalancer plan-time dependency
    walks make so they return empty device/config lists. Harmless for the other
    parametrized cases, whose walks read straight from the fetched state.
    """
    client.list_firewall_devices.return_value = {"data": []}
    client.list_nodebalancer_configs.return_value = {"data": []}


@pytest.mark.parametrize(
    ("handler", "id_key", "id_val", "fetch_attr", "delete_attr"), _ALL_CASES
)
async def test_plan_then_apply(
    handler: _DeleteHandler,
    id_key: str,
    id_val: object,
    fetch_attr: str,
    delete_attr: str,
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    getattr(mock_linode_client, fetch_attr).return_value = _state("2026-01-01T00:00:00")
    _stub_walk_calls(mock_linode_client)
    delete = getattr(mock_linode_client, delete_attr)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_result = await handler({id_key: id_val, "mode": "plan"}, sample_config)
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        delete.assert_not_awaited()
        assert await store.length() == 1

        apply_result = await handler(
            {id_key: id_val, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        apply_text = apply_result[0].text
        assert "deleted" in apply_text or "removed" in apply_text
        delete.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


@pytest.mark.parametrize(
    ("handler", "id_key", "id_val", "fetch_attr", "delete_attr"), _COSMETIC_CASES
)
async def test_apply_ignores_cosmetic_drift(
    handler: _DeleteHandler,
    id_key: str,
    id_val: object,
    fetch_attr: str,
    delete_attr: str,
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    fetch = getattr(mock_linode_client, fetch_attr)
    fetch.return_value = _state("2026-01-01T00:00:00")
    _stub_walk_calls(mock_linode_client)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_result = await handler({id_key: id_val, "mode": "plan"}, sample_config)
        plan_id = json.loads(plan_result[0].text)["plan_id"]

        # Only the cosmetic "updated" timestamp moves; this must not refuse.
        fetch.return_value = _state("2026-09-09T09:09:09")

        apply_result = await handler(
            {id_key: id_val, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        apply_text = apply_result[0].text
        assert "deleted" in apply_text or "removed" in apply_text
        getattr(mock_linode_client, delete_attr).assert_awaited_once()
    finally:
        reset_plan_store(token)


# Two-ID delete tools: (outer, inner) ID pair, plus the per-type cosmetic field
# and the value it drifts to (a list for LKE pools, a timestamp otherwise).
_TWO_ID_CASES = [
    pytest.param(
        handle_linode_instance_disk_delete,
        "linode_id",
        "disk_id",
        "get_instance_disk",
        "delete_instance_disk",
        "updated",
        "2026-09-09T09:09:09",
        id="instance_disk",
    ),
    pytest.param(
        handle_linode_vpc_subnet_delete,
        "vpc_id",
        "subnet_id",
        "get_vpc_subnet",
        "delete_vpc_subnet",
        "updated",
        "2026-09-09T09:09:09",
        id="vpc_subnet",
    ),
    pytest.param(
        handle_linode_domain_record_delete,
        "domain_id",
        "record_id",
        "get_domain_record",
        "delete_domain_record",
        "updated",
        "2026-09-09T09:09:09",
        id="domain_record",
    ),
    pytest.param(
        handle_linode_lke_pool_delete,
        "cluster_id",
        "pool_id",
        "get_lke_node_pool",
        "delete_lke_node_pool",
        "nodes",
        [{"status": "ready"}],
        id="lke_pool",
    ),
]


@pytest.mark.parametrize(
    (
        "handler",
        "outer_key",
        "inner_key",
        "fetch_attr",
        "delete_attr",
        "cosmetic",
        "drift_val",
    ),
    _TWO_ID_CASES,
)
async def test_two_id_plan_then_apply(
    handler: _DeleteHandler,
    outer_key: str,
    inner_key: str,
    fetch_attr: str,
    delete_attr: str,
    cosmetic: str,
    drift_val: object,
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    getattr(mock_linode_client, fetch_attr).return_value = _state("2026-01-01T00:00:00")
    delete = getattr(mock_linode_client, delete_attr)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_args = {outer_key: 10, inner_key: 20, "mode": "plan"}
        plan_result = await handler(plan_args, sample_config)
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        delete.assert_not_awaited()

        # Bump only the per-type cosmetic field; the apply must still execute.
        getattr(mock_linode_client, fetch_attr).return_value = {
            **_state("2026-01-01T00:00:00"),
            cosmetic: drift_val,
        }

        apply_args = {
            outer_key: 10,
            inner_key: 20,
            "mode": "apply",
            "plan_id": plan_id,
        }
        apply_result = await handler(apply_args, sample_config)
        apply_text = apply_result[0].text
        assert "deleted" in apply_text or "removed" in apply_text
        delete.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)
