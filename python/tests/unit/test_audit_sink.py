"""Audit sink tests.

Mirrors ``go/internal/audit/sink_test.go``. Tests define behavior
contracts (noop discards, capturing retains insertion order, length
accessor, non-None empty default).
"""

from __future__ import annotations

from datetime import UTC, datetime

from linodemcp.audit import (
    Capability,
    CapturingSink,
    Event,
    Mode,
    MultiSink,
    NoopSink,
    Sink,
    Status,
)


def _event(tool: str) -> Event:
    """Build a minimal event for sink testing."""
    return Event(
        ts=datetime.now(UTC),
        ts_unix_ns=0,
        event_id="evt_test",
        tool=tool,
        tool_capability=Capability.META,
        environment="",
        profile="",
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="",
        session_id="",
        credential_generation=0,
    )


def test_noop_sink_satisfies_protocol() -> None:
    """NoopSink fits the Sink protocol and accepts events without effect."""
    sink: Sink = NoopSink()

    evt = _event("test_tool")
    sink.write(evt)
    sink.write(evt)

    # No observable contract beyond not raising; the only check is
    # the input is unchanged.
    assert evt.tool == "test_tool"


def test_capturing_sink_retains_write_order() -> None:
    """Insertion order preserved across writes."""
    sink = CapturingSink()

    sink.write(_event("first"))
    sink.write(_event("second"))
    sink.write(_event("third"))

    events = sink.events()
    assert len(events) == 3
    assert events[0].tool == "first"
    assert events[1].tool == "second"
    assert events[2].tool == "third"


def test_capturing_sink_len_reports_count() -> None:
    """__len__ tracks the buffer size."""
    sink = CapturingSink()
    assert len(sink) == 0

    sink.write(_event("one"))
    assert len(sink) == 1


def test_new_capturing_sink_starts_empty() -> None:
    """events() returns an empty list, not None."""
    sink = CapturingSink()

    assert sink.events() == []
    assert sink.events() is not None


def test_multi_sink_fans_out_to_every_child() -> None:
    """The fan-out delivers each event to all child sinks."""
    first = CapturingSink()
    second = CapturingSink()
    multi = MultiSink(first, second)

    multi.write(_event("fanned_out"))

    assert len(first) == 1
    assert len(second) == 1
    assert first.events()[0].tool == "fanned_out"
    assert second.events()[0].tool == "fanned_out"


def test_multi_sink_empty_is_noop() -> None:
    """A fan-out with no children does not raise."""
    multi = MultiSink()
    multi.write(_event("nowhere"))
