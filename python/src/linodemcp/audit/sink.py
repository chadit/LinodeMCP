"""Audit sink protocol and stand-in implementations.

Mirrors ``go/internal/audit/sink.go``. Phase 1b ships only the
``NoopSink`` so the capture middleware has a stable target while
Phase 2 implements the JSONL writer.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Protocol

if TYPE_CHECKING:
    from linodemcp.audit.event import Event


class Sink(Protocol):
    """Consume audit events.

    Implementations MUST NOT mutate the event after ``write`` returns.
    Production sinks (Phase 2 JSONL writer, Phase 3 SQLite) buffer the
    event to a background task; the per-event send is the only
    synchronous cost paid by the dispatcher.
    """

    def write(self, event: Event) -> None:
        """Record an event."""
        ...


class NoopSink:
    """Discards every event.

    Used until Phase 2 lands the JSONL writer. Tests use it to
    exercise the capture middleware without exercising a real sink.
    """

    def write(self, event: Event) -> None:  # noqa: ARG002 - protocol shape
        """Discard the event. No observable side effect."""
        return


class MultiSink:
    """Fan an event out to every child sink in order.

    Used to dual-write to JSONL and SQLite when both are enabled. Each
    child's write is best-effort per its own contract, so a failing
    child (e.g. a SQLite insert error) does not stop the others; the
    JSONL sink stays the durable record.
    """

    def __init__(self, *sinks: Sink) -> None:
        """Build a fan-out over ``sinks``, written in the order given."""
        self._sinks = sinks

    def write(self, event: Event) -> None:
        """Forward the event to every child sink."""
        for sink in self._sinks:
            sink.write(event)


class CapturingSink:
    """Retain every event for test inspection.

    NOT for production use: accumulates without bound. The capture
    middleware tests rely on it to assert event-field population at
    the dispatch boundary.
    """

    def __init__(self) -> None:
        self._events: list[Event] = []

    def write(self, event: Event) -> None:
        """Append the event to the internal buffer.

        Stores the event reference directly; tests that need
        snapshot-style capture (defensive against later mutation by
        the dispatcher) should construct a copy via ``Event.to_dict``
        before the next dispatch.
        """
        self._events.append(event)

    def events(self) -> list[Event]:
        """Return the captured event list.

        The list is the sink's internal buffer; callers must not
        mutate it. Returning the live list avoids a copy in the
        common test path.
        """
        return self._events

    def __len__(self) -> int:
        """Report how many events have been captured."""
        return len(self._events)
