"""Two-stage parity tests for the second wave of opted-in delete tools.

Covers the CapDestroy tools wired into plan/apply after the first wave: the
database, image share group, firewall device, instance backup/IP/password,
IPv6 range, LKE node/kubeconfig/service-token, object storage bucket/key/SSL,
tag, and VLAN tools. Each case proves the plan/apply round trip: a mode:"plan"
call returns a plan_id and skips the mutation, and a mode:"apply" call with that
plan_id runs the mutation. Identical state on both sides means no drift, so the
apply must execute.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    handle_linode_database_mysql_instance_delete,
    handle_linode_database_postgresql_instance_delete,
    handle_linode_firewall_device_delete,
    handle_linode_image_sharegroup_delete,
    handle_linode_image_sharegroup_token_delete,
    handle_linode_instance_backups_cancel,
    handle_linode_instance_ip_delete,
    handle_linode_instance_password_reset,
    handle_linode_ipv6_range_delete,
    handle_linode_lke_kubeconfig_delete,
    handle_linode_lke_node_delete,
    handle_linode_lke_service_token_delete,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_key_delete,
    handle_linode_object_storage_ssl_delete,
    handle_linode_tag_delete,
    handle_linode_vlan_delete,
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

_REGION = "us-east-1"
_BUCKET = "my-bucket"
_TOKEN_UUID = "11111111-1111-1111-1111-111111111111"

# Each case: (handler, args, fetch_attr, fetch_return, exec_attr, expect).
# fetch_return is the state the GET returns at plan and apply (identical, so no
# drift). expect is a substring the apply's success message must contain.
_CASES = [
    pytest.param(
        handle_linode_database_mysql_instance_delete,
        {"instance_id": 123},
        "route_raw",
        {"id": 123, "label": "db", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "deleted",
        id="database_mysql",
    ),
    pytest.param(
        handle_linode_database_postgresql_instance_delete,
        {"instance_id": 123},
        "route_raw",
        {"id": 123, "label": "pg", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "deleted",
        id="database_postgresql",
    ),
    pytest.param(
        handle_linode_image_sharegroup_delete,
        {"sharegroup_id": 3},
        "route_raw",
        {
            "uuid": "22222222-2222-2222-2222-222222222222",
            "label": "share",
            "updated": "2026-01-01T00:00:00",
        },
        "route_call",
        "removed successfully",
        id="image_sharegroup",
    ),
    pytest.param(
        handle_linode_image_sharegroup_token_delete,
        {"token_uuid": _TOKEN_UUID},
        "route_raw",
        {"uuid": "sg-uuid", "label": "share", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "removed successfully",
        id="image_sharegroup_token",
    ),
    pytest.param(
        handle_linode_instance_backups_cancel,
        {"linode_id": 123},
        "route_raw",
        {"id": 123, "status": "running", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "canceled",
        id="instance_backups_cancel",
    ),
    pytest.param(
        handle_linode_instance_ip_delete,
        {"linode_id": 123, "address": "203.0.113.7"},
        "route_raw",
        {"address": "203.0.113.7", "type": "ipv4", "public": True},
        "route_call",
        "removed from instance",
        id="instance_ip",
    ),
    pytest.param(
        handle_linode_ipv6_range_delete,
        {"range": "2001:db8::/64"},
        "route_raw",
        {"range": "2001:db8::", "region": "us-east", "prefix": 64},
        "route_call",
        "deleted",
        id="ipv6_range",
    ),
    pytest.param(
        handle_linode_lke_node_delete,
        {"cluster_id": 123, "node_id": "node-xyz"},
        "route_raw",
        {"id": "node-xyz", "instance_id": 456, "status": "ready"},
        "route_call",
        "deleted",
        id="lke_node",
    ),
    pytest.param(
        handle_linode_lke_kubeconfig_delete,
        {"cluster_id": 123},
        "route_raw",
        {"id": 123, "label": "lke", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "regenerated",
        id="lke_kubeconfig",
    ),
    pytest.param(
        handle_linode_lke_service_token_delete,
        {"cluster_id": 123},
        "route_raw",
        {"id": 123, "label": "lke", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "deleted",
        id="lke_service_token",
    ),
    pytest.param(
        handle_linode_object_storage_bucket_delete,
        {"region": _REGION, "label": _BUCKET},
        "route_raw",
        {"label": _BUCKET, "region": _REGION, "objects": 0},
        "route_call",
        "removed successfully",
        id="object_storage_bucket",
    ),
    pytest.param(
        handle_linode_object_storage_key_delete,
        {"key_id": 123},
        "route_raw",
        {"id": 123, "label": "ci-key", "access_key": "AK"},
        "route_call",
        "revoked",
        id="object_storage_key",
    ),
    pytest.param(
        handle_linode_object_storage_ssl_delete,
        {"region": _REGION, "label": _BUCKET},
        "route_raw",
        {"ssl": True},
        "route_call",
        "deleted",
        id="object_storage_ssl",
    ),
    pytest.param(
        handle_linode_tag_delete,
        {"tag_label": "prod"},
        "route_raw",
        {"data": [], "page": 1, "pages": 1, "results": 0},
        "route_call",
        "deleted",
        id="account_tag",
    ),
]

# Two-ID delete tools keyed by (outer, inner) integer IDs.
_TWO_ID_CASES = [
    pytest.param(
        handle_linode_firewall_device_delete,
        "firewall_id",
        "device_id",
        "route_raw",
        {"id": 20, "status": "ready", "updated": "2026-01-01T00:00:00"},
        "route_call",
        "removed",
        id="firewall_device",
    ),
]


@pytest.mark.parametrize(
    ("handler", "args", "fetch_attr", "fetch_return", "exec_attr", "expect"),
    _CASES,
)
async def test_plan_then_apply(
    handler: _DeleteHandler,
    args: dict[str, Any],
    fetch_attr: str,
    fetch_return: dict[str, Any],
    exec_attr: str,
    expect: str,
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    getattr(mock_linode_client, fetch_attr).return_value = fetch_return
    execute = getattr(mock_linode_client, exec_attr)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_result = await handler({**args, "mode": "plan"}, sample_config)
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        execute.assert_not_awaited()
        assert await store.length() == 1

        apply_result = await handler(
            {**args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert expect in apply_result[0].text
        execute.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


@pytest.mark.parametrize(
    (
        "handler",
        "outer_key",
        "inner_key",
        "fetch_attr",
        "fetch_return",
        "exec_attr",
        "expect",
    ),
    _TWO_ID_CASES,
)
async def test_two_id_plan_then_apply(
    handler: _DeleteHandler,
    outer_key: str,
    inner_key: str,
    fetch_attr: str,
    fetch_return: dict[str, Any],
    exec_attr: str,
    expect: str,
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    getattr(mock_linode_client, fetch_attr).return_value = fetch_return
    execute = getattr(mock_linode_client, exec_attr)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_args = {outer_key: 10, inner_key: 20, "mode": "plan"}
        plan_result = await handler(plan_args, sample_config)
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        execute.assert_not_awaited()

        apply_args = {
            outer_key: 10,
            inner_key: 20,
            "mode": "apply",
            "plan_id": plan_id,
        }
        apply_result = await handler(apply_args, sample_config)
        assert expect in apply_result[0].text
        execute.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


async def test_vlan_plan_then_apply(
    sample_config: Config,
    mock_linode_client: AsyncMock,
) -> None:
    """VLAN delete resolves its state through the list endpoint (no single GET),
    so it is exercised on its own rather than via the shared single-key table.
    """
    mock_linode_client.route_raw.return_value = {
        "data": [{"region": "us-east", "label": "vl-app", "linodes": []}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    delete = mock_linode_client.route_call

    store = PlanStore()
    token = set_plan_store(store)
    try:
        args = {"region_id": "us-east", "label": "vl-app"}
        plan_result = await handle_linode_vlan_delete(
            {**args, "mode": "plan"}, sample_config
        )
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        delete.assert_not_awaited()

        apply_result = await handle_linode_vlan_delete(
            {**args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "deleted" in apply_result[0].text
        delete.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


async def test_password_reset_plan_then_apply(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """The reset reads its state and runs its write through the same primitive,
    so the plan-time claim is which tool was named rather than whether the
    primitive was touched.
    """
    mock_linode_client.route_raw.return_value = {
        "id": 123,
        "status": "offline",
        "updated": "2026-01-01T00:00:00",
    }
    args = {"linode_id": 123, "root_pass": "Sup3rSecretPass99"}

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_result = await handle_linode_instance_password_reset(
            {**args, "mode": "plan"}, sample_config
        )
        plan_id = json.loads(plan_result[0].text)["plan_id"]
        assert plan_id
        named = [
            c.args[0] for c in mock_linode_client.route_raw.await_args_list if c.args
        ]
        assert named == ["linode_instance_get"]
        assert await store.length() == 1

        apply_result = await handle_linode_instance_password_reset(
            {**args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "reset" in apply_result[0].text
        named = [
            c.args[0] for c in mock_linode_client.route_raw.await_args_list if c.args
        ]
        assert named.count("linode_instance_password_reset") == 1
        assert await store.length() == 0
    finally:
        reset_plan_store(token)
