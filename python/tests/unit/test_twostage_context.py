"""Tests for the two-stage plan-store ContextVar accessor."""

from __future__ import annotations

from linodemcp.twostage import (
    plan_store_from_context,
    reset_plan_store,
    set_plan_store,
)
from linodemcp.twostage.store import PlanStore


def test_plan_store_context_roundtrip() -> None:
    assert plan_store_from_context() is None

    store = PlanStore()
    token = set_plan_store(store)
    try:
        assert plan_store_from_context() is store
    finally:
        reset_plan_store(token)

    assert plan_store_from_context() is None
