"""Ambient plan-store access for the two-stage flow.

Python tool handlers take ``(arguments, config)`` with no context parameter,
so the server publishes the active plan store through a ``ContextVar`` that a
two-stage-aware handler reads. This mirrors the Go side's context injection
(``WithPlanStore`` / ``PlanStoreFromContext`` in ``internal/tools``).
"""

from __future__ import annotations

from contextvars import ContextVar
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from contextvars import Token

    from linodemcp.twostage.store import PlanStore

_PLAN_STORE: ContextVar[PlanStore | None] = ContextVar(
    "twostage_plan_store", default=None
)


def set_plan_store(store: PlanStore | None) -> Token[PlanStore | None]:
    """Publish the active plan store, returning a token that resets it."""
    return _PLAN_STORE.set(store)


def reset_plan_store(token: Token[PlanStore | None]) -> None:
    """Restore the plan store the matching set_plan_store call replaced."""
    _PLAN_STORE.reset(token)


def plan_store_from_context() -> PlanStore | None:
    """Return the plan store the server published, or None when unset.

    A None store means two-stage is not wired (for example a unit test that
    calls a handler directly), so the handler uses its single-step path.
    """
    return _PLAN_STORE.get()
