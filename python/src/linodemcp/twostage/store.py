"""In-memory two-stage plan store.

Mirrors ``go/internal/twostage/store.go``. Plans live only in process
memory; a restart drops them all. A janitor task reaps expired plans on an
interval, and ``put`` enforces a hard ceiling by evicting the oldest plan.
Concurrency is guarded by an ``asyncio.Lock`` since the server dispatches
tool calls on the event loop.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable
    from datetime import timedelta

    from mcp.types import TextContent

# Once the store holds this many plans, the next put evicts the oldest by
# planned_at to make room. A later apply against an evicted plan looks the same
# as an unknown id.
MAX_OUTSTANDING_PLANS = 1000


class PlanNotFoundError(Exception):
    """Raised when no plan with the given id exists (or it was consumed)."""


class PlanExpiredError(Exception):
    """Raised when a plan exists but its TTL has elapsed."""


@dataclass
class PlanEntry:
    """One outstanding plan held in process memory."""

    id: str
    tool: str
    environment: str
    args: dict[str, Any]
    state_hash: str
    planned_at: datetime
    expires_at: datetime
    apply: Callable[[], Awaitable[list[TextContent]]]
    # Normalized top-level field map of the planned state (hash-ignore fields
    # already stripped). On a drift refusal the apply path diffs it against the
    # re-fetched state to name the changed fields. None when the state did not
    # serialize to a JSON object. Mirrors the Go PlanEntry.StateFields.
    state_fields: dict[str, Any] | None = None


def _wall_clock() -> datetime:
    return datetime.now(UTC)


def _empty_plans() -> dict[str, PlanEntry]:
    return {}


@dataclass
class PlanStore:
    """Holds outstanding plans, guarded by an asyncio lock."""

    now: Callable[[], datetime] = _wall_clock
    _plans: dict[str, PlanEntry] = field(default_factory=_empty_plans)
    _lock: asyncio.Lock = field(default_factory=asyncio.Lock)

    async def put(self, entry: PlanEntry) -> None:
        """Store a plan, evicting the oldest entry first when at the ceiling."""
        async with self._lock:
            if len(self._plans) >= MAX_OUTSTANDING_PLANS:
                self._evict_oldest_locked()
            self._plans[entry.id] = entry

    async def get(self, plan_id: str) -> PlanEntry:
        """Return a plan without consuming it.

        Raises PlanNotFoundError for an unknown id and PlanExpiredError when
        the TTL has elapsed.
        """
        async with self._lock:
            entry = self._plans.get(plan_id)
            if entry is None:
                raise PlanNotFoundError(plan_id)
            if self.now() > entry.expires_at:
                raise PlanExpiredError(plan_id)
            return entry

    async def take(self, plan_id: str) -> PlanEntry:
        """Return a plan and remove it atomically (single-use)."""
        async with self._lock:
            entry = self._plans.pop(plan_id, None)
            if entry is None:
                raise PlanNotFoundError(plan_id)
            if self.now() > entry.expires_at:
                raise PlanExpiredError(plan_id)
            return entry

    async def remove(self, plan_id: str) -> None:
        """Drop a plan by id. A no-op when the id is absent."""
        async with self._lock:
            self._plans.pop(plan_id, None)

    async def sweep(self) -> int:
        """Drop every expired plan, returning how many were removed."""
        async with self._lock:
            now = self.now()
            expired = [
                pid for pid, entry in self._plans.items() if now > entry.expires_at
            ]
            for pid in expired:
                del self._plans[pid]
            return len(expired)

    async def length(self) -> int:
        """Return the number of outstanding plans."""
        async with self._lock:
            return len(self._plans)

    def start_janitor(self, interval: timedelta) -> asyncio.Task[None]:
        """Launch a background task that sweeps expired plans on an interval."""

        async def _run() -> None:
            while True:
                await asyncio.sleep(interval.total_seconds())
                await self.sweep()

        return asyncio.create_task(_run())

    def _evict_oldest_locked(self) -> None:
        if not self._plans:
            return
        oldest = min(self._plans.values(), key=lambda entry: entry.planned_at)
        del self._plans[oldest.id]
