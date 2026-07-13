"""Reference-impl tests for two-stage on linode_instance_delete.

Mirrors the Go ``twostage_destroy_test.go``: drives the real handler with a
published plan store (ContextVar) and a mocked RetryableClient, covering the
plan -> apply happy path plus drift, expiry, unknown, and args-mismatch
refusals.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.tools.linode_instance_write import handle_linode_instance_delete
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

    from linodemcp.config import Config


@pytest.fixture(autouse=True)
def stub_instance_walk(mock_linode_client: AsyncMock) -> None:
    """The plan-time dependency walk lists volumes and instance IPs. Stub both
    to empty so the walk runs cleanly; production fetches them from the API.
    """
    mock_linode_client.list_volumes.return_value = []
    mock_linode_client.list_instance_ips.return_value = {"ipv4": {"public": []}}


def _instance_state(status: str = "running") -> dict[str, Any]:
    return {"id": 123, "label": "web-prod-01", "status": status}


async def _make_plan(cfg: Config) -> str:
    result = await handle_linode_instance_delete(
        {"instance_id": 123, "mode": "plan"}, cfg
    )
    body = json.loads(result[0].text)
    plan_id = body["plan_id"]
    assert isinstance(plan_id, str)
    assert plan_id
    return plan_id


async def test_plan_then_apply(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = _instance_state()

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        body = json.loads(plan_result[0].text)
        plan_id = body["plan_id"]
        assert body["would_execute"]["method"] == "DELETE"
        mock_linode_client.delete_instance.assert_not_awaited()
        assert await store.length() == 1

        apply_result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "removed successfully" in apply_result[0].text
        mock_linode_client.delete_instance.assert_awaited_once()
        assert await store.length() == 0

        again = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "PLAN_NOT_FOUND" in again[0].text
    finally:
        reset_plan_store(token)


async def test_apply_drift(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = _instance_state("running")

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_id = await _make_plan(sample_config)

        mock_linode_client.get_instance.return_value = _instance_state("offline")

        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        drift_text = result[0].text
        assert "PLAN_DRIFT_DETECTED" in drift_text
        # Only status moved (running -> offline); the refusal must name it.
        assert "changed fields: status" in drift_text
        mock_linode_client.delete_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)


async def test_apply_ignores_cosmetic_drift(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    state = _instance_state()
    state["updated"] = "2026-06-01T00:00:00"
    mock_linode_client.get_instance.return_value = state

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_id = await _make_plan(sample_config)

        drifted = _instance_state()
        drifted["updated"] = "2026-06-08T12:34:56"
        mock_linode_client.get_instance.return_value = drifted

        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "removed successfully" in result[0].text
        mock_linode_client.delete_instance.assert_awaited_once()
    finally:
        reset_plan_store(token)


async def test_apply_unknown_plan(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = _instance_state()

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": "plan_missing"},
            sample_config,
        )
        assert "PLAN_NOT_FOUND" in result[0].text
        mock_linode_client.delete_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)


async def test_apply_expired(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = _instance_state()

    current = datetime.now(UTC)
    store = PlanStore(now=lambda: current)
    token = set_plan_store(store)
    try:
        plan_id = await _make_plan(sample_config)

        current = datetime.now(UTC) + timedelta(minutes=10)

        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "PLAN_EXPIRED" in result[0].text
        mock_linode_client.delete_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)


async def test_apply_args_mismatch(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = _instance_state()

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan_id = await _make_plan(sample_config)

        result = await handle_linode_instance_delete(
            {"instance_id": 999, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "PLAN_ARGS_MISMATCH" in result[0].text
        mock_linode_client.delete_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)


async def test_plan_fetch_error_returns_error_and_stores_no_plan(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    # A failed state fetch during plan must surface an error and leave nothing
    # to apply. ValueError is one of the fetch errors the plan path catches.
    mock_linode_client.get_instance.side_effect = ValueError("boom")

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        assert "Failed to fetch state for plan" in result[0].text
        assert await store.length() == 0
        mock_linode_client.delete_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)


async def test_plan_includes_dependency_walk(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    # A plan reads like a dry-run preview: the body carries the dependency
    # walk's output, including distinct ephemeral and reserved IP effects, not
    # just the state hash.
    mock_linode_client.get_instance.return_value = _instance_state()
    mock_linode_client.list_instance_ips.return_value = {
        "ipv4": {
            "public": [{"address": "192.0.2.7"}],
            "reserved": [{"address": "198.51.100.8"}],
        }
    }

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        body = json.loads(result[0].text)
        deps = body["dependencies"]
        ip_deps = {dep["label"]: dep for dep in deps if dep["kind"] == "public_ip"}
        assert ip_deps["192.0.2.7"]["action"] == "released"
        assert ip_deps["198.51.100.8"]["action"] == "detached"
        assert "reservation and billing continue" in ip_deps["198.51.100.8"]["note"]
        assert body["warnings"]
    finally:
        reset_plan_store(token)
