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

from linodemcp.gentools import (
    handle_linode_domain_delete,
    handle_linode_domain_record_delete,
    handle_linode_firewall_delete,
    handle_linode_image_delete,
    handle_linode_instance_disk_delete,
    handle_linode_lke_cluster_delete,
    handle_linode_lke_pool_delete,
    handle_linode_nodebalancer_delete,
    handle_linode_placement_group_delete,
    handle_linode_sshkey_delete,
    handle_linode_stackscript_delete,
    handle_linode_volume_delete,
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

# The volume delete is generated, so its removal goes through the routed
# primitive rather than a typed client method.
_VOLUME = pytest.param(
    handle_linode_volume_delete,
    "volume_id",
    123,
    "route_raw",
    "route_call",
    id="volume",
)
_LKE = pytest.param(
    handle_linode_lke_cluster_delete,
    "cluster_id",
    123,
    "route_raw",
    "route_call",
    id="lke_cluster",
)
_FIREWALL = pytest.param(
    handle_linode_firewall_delete,
    "firewall_id",
    123,
    "route_raw",
    "route_call",
    id="firewall",
)
_NODEBALANCER = pytest.param(
    handle_linode_nodebalancer_delete,
    "nodebalancer_id",
    123,
    "route_raw",
    "route_call",
    id="nodebalancer",
)
_VPC = pytest.param(
    handle_linode_vpc_delete, "vpc_id", 123, "route_raw", "route_call", id="vpc"
)
# The generated destroys remove their resource through the routed
# primitive rather than a typed client method, so route_call is what a
# completed apply awaits for them.
_DOMAIN = pytest.param(
    handle_linode_domain_delete,
    "domain_id",
    123,
    "route_raw",
    "route_call",
    id="domain",
)
_STACKSCRIPT = pytest.param(
    handle_linode_stackscript_delete,
    "stackscript_id",
    123,
    "route_raw",
    "route_call",
    id="stackscript",
)
_SSHKEY = pytest.param(
    handle_linode_sshkey_delete,
    "ssh_key_id",
    123,
    "route_raw",
    "route_call",
    id="sshkey",
)
_PLACEMENT = pytest.param(
    handle_linode_placement_group_delete,
    "group_id",
    123,
    "route_raw",
    "route_call",
    id="placement_group",
)
_IMAGE = pytest.param(
    handle_linode_image_delete,
    "image_id",
    "private/123",
    "route_raw",
    "route_call",
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


def _state(updated: str, resource_id: object = 123) -> dict[str, Any]:
    """The fetched resource a plan hashes. resource_id is the case's own, since
    an image is addressed by a string where every other resource takes an int.
    """
    return {"id": resource_id, "status": "active", "updated": updated}


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
    getattr(mock_linode_client, fetch_attr).return_value = _state(
        "2026-01-01T00:00:00", id_val
    )
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
        "route_raw",
        "route_call",
        "updated",
        "2026-09-09T09:09:09",
        id="instance_disk",
    ),
    pytest.param(
        handle_linode_vpc_subnet_delete,
        "vpc_id",
        "subnet_id",
        "route_raw",
        "route_call",
        "updated",
        "2026-09-09T09:09:09",
        id="vpc_subnet",
    ),
    pytest.param(
        handle_linode_domain_record_delete,
        "domain_id",
        "record_id",
        "route_raw",
        "route_call",
        "updated",
        "2026-09-09T09:09:09",
        id="domain_record",
    ),
    pytest.param(
        handle_linode_lke_pool_delete,
        "cluster_id",
        "pool_id",
        "route_raw",
        "route_call",
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
