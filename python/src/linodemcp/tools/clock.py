"""Ambient reference clock for the audit report tool.

Python tool handlers take ``(arguments, config)`` with no context parameter, so
the reference clock travels through a ``ContextVar`` the way the builder state
does (``linodemcp.tools.builderstate``). Go carries the same seam on the call
context (``WithClock`` in ``go/internal/tools/clock.go``).
"""

from __future__ import annotations

from contextvars import ContextVar
from datetime import UTC, datetime
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable
    from contextvars import Token

_CLOCK: ContextVar[Callable[[], datetime] | None] = ContextVar(
    "reference_clock", default=None
)


def set_clock(
    clock: Callable[[], datetime] | None,
) -> Token[Callable[[], datetime] | None]:
    """Publish the reference clock, returning a resetting token.

    The cross-language behavior fixtures pin a report whose window is relative,
    and a wall-clock window has no reproducible answer.
    """
    return _CLOCK.set(clock)


def reset_clock(token: Token[Callable[[], datetime] | None]) -> None:
    """Restore the clock the matching set_clock call replaced."""
    _CLOCK.reset(token)


def now() -> datetime:
    """Return the published reference time, or the wall clock when unset."""
    clock = _CLOCK.get()
    if clock is None:
        return datetime.now(UTC)

    return clock()
