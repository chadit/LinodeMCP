"""Tests for the two-stage asyncio plan store.

Mirrors the coverage of the Go ``store_test.go``: put/get, single-use take,
expiry, sweep, and ceiling eviction, with an injected clock for the
time-dependent cases.
"""

from __future__ import annotations

import asyncio
import contextlib
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

import pytest

from linodemcp.twostage.store import (
    MAX_OUTSTANDING_PLANS,
    PlanEntry,
    PlanExpiredError,
    PlanNotFoundError,
    PlanStore,
)

if TYPE_CHECKING:
    from mcp.types import TextContent

_BASE = datetime(2026, 1, 1, tzinfo=UTC)


async def _noop_apply() -> list[TextContent]:
    return []


def _entry(plan_id: str, planned_at: datetime, ttl_minutes: int = 5) -> PlanEntry:
    return PlanEntry(
        id=plan_id,
        tool="linode_x",
        environment="default",
        args={"id": 1},
        state_hash="sha256:abc",
        planned_at=planned_at,
        expires_at=planned_at + timedelta(minutes=ttl_minutes),
        apply=_noop_apply,
    )


async def test_put_and_get() -> None:
    store = PlanStore(now=lambda: _BASE)
    entry = _entry("plan_a", _BASE)
    await store.put(entry)

    assert await store.get("plan_a") is entry
    assert await store.length() == 1


async def test_get_unknown_raises_not_found() -> None:
    store = PlanStore(now=lambda: _BASE)
    with pytest.raises(PlanNotFoundError):
        await store.get("plan_missing")


async def test_get_expired_raises_expired() -> None:
    clock = {"t": _BASE}
    store = PlanStore(now=lambda: clock["t"])
    await store.put(_entry("plan_a", _BASE))

    clock["t"] = _BASE + timedelta(minutes=10)

    with pytest.raises(PlanExpiredError):
        await store.get("plan_a")


async def test_take_is_single_use() -> None:
    store = PlanStore(now=lambda: _BASE)
    await store.put(_entry("plan_x", _BASE))

    taken = await store.take("plan_x")
    assert taken.id == "plan_x"

    with pytest.raises(PlanNotFoundError):
        await store.take("plan_x")
    assert await store.length() == 0


async def test_take_expired_still_removes() -> None:
    clock = {"t": _BASE}
    store = PlanStore(now=lambda: clock["t"])
    await store.put(_entry("plan_x", _BASE))

    clock["t"] = _BASE + timedelta(minutes=10)

    with pytest.raises(PlanExpiredError):
        await store.take("plan_x")
    assert await store.length() == 0


async def test_remove_is_noop_when_absent() -> None:
    store = PlanStore(now=lambda: _BASE)
    await store.put(_entry("plan_x", _BASE))

    await store.remove("plan_x")
    await store.remove("plan_never_existed")

    assert await store.length() == 0


async def test_sweep_drops_only_expired() -> None:
    clock = {"t": _BASE}
    store = PlanStore(now=lambda: clock["t"])
    await store.put(_entry("plan_short", _BASE, ttl_minutes=5))
    await store.put(_entry("plan_long", _BASE, ttl_minutes=60))

    clock["t"] = _BASE + timedelta(minutes=10)

    assert await store.sweep() == 1
    assert await store.length() == 1
    assert (await store.get("plan_long")).id == "plan_long"


async def test_start_janitor_sweeps_expired() -> None:
    clock = {"t": _BASE}
    store = PlanStore(now=lambda: clock["t"])
    await store.put(_entry("plan_x", _BASE, ttl_minutes=5))

    # Advance past expiry, then let the janitor tick on a tiny interval.
    clock["t"] = _BASE + timedelta(minutes=10)
    task = store.start_janitor(timedelta(milliseconds=1))
    try:
        for _ in range(200):
            if await store.length() == 0:
                break
            await asyncio.sleep(0.005)
        assert await store.length() == 0, "janitor did not sweep the expired plan"
    finally:
        task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await task


async def test_put_evicts_oldest_at_ceiling() -> None:
    store = PlanStore(now=lambda: _BASE)
    for index in range(MAX_OUTSTANDING_PLANS):
        await store.put(
            _entry(
                f"plan_{index:04d}",
                _BASE + timedelta(milliseconds=index),
                ttl_minutes=60,
            )
        )

    assert await store.length() == MAX_OUTSTANDING_PLANS

    await store.put(_entry("plan_newest", _BASE + timedelta(hours=1), ttl_minutes=60))

    assert await store.length() == MAX_OUTSTANDING_PLANS
    with pytest.raises(PlanNotFoundError):
        await store.get("plan_0000")
    assert (await store.get("plan_newest")).id == "plan_newest"
