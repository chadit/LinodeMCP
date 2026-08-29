"""The ambient reference clock, at the seam the generated handler reads it from.

Mirrors ``go/internal/tools/clock_test.go``. Go held its seam with two cases
from the start and this language held its own only through the behavior corpus,
which the B9 mutation run measured: the defect that stops the seam reading what
a caller published was caught there and by nothing else here.

LOCAL_AMBIENT_CLOCK is what makes the seam a declared reading rather than a
per-language accident, so each registered language owes it a case of its own.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    Status,
    event_timestamp,
)
from linodemcp.config import (
    REPORT_OUTPUT_SUMMARY,
    AuditConfig,
    Config,
    ReportConfig,
    ReportFilter,
)
from linodemcp.genlocal import (
    record_audit_event,
)
from linodemcp.gentools import handle_linode_audit_report
from linodemcp.tools.clock import reset_clock, set_clock

if TYPE_CHECKING:
    from collections.abc import Iterator
    from pathlib import Path

# The report every case runs: a relative window, so its answer is measured back
# from whatever instant the seam publishes.
_WINDOW_REPORT = "window"

# The seeded events all land within a minute of each other, so a two-minute
# window either takes all four or none.
_SEEDED = datetime(2026, 5, 20, 0, 0, 0, tzinfo=UTC)


def _event(tool: str, capability: Capability, second: int) -> Event:
    """One event at a distinct second inside the seeded minute."""
    ts = _SEEDED.replace(second=second)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool=tool,
        tool_capability=capability,
        environment="default",
        profile="full-access",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS.value,
        latency_ms=1,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="sess-clock",
        credential_generation=0,
    )


@pytest.fixture
def clock_config(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Config:
    """Four seeded events and a config holding one relative-window report."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)

    events = [
        _event("linode_instance_list", Capability.READ, 1),
        _event("linode_instance_boot", Capability.WRITE, 2),
        _event("linode_volume_delete", Capability.DESTROY, 3),
        _event("linode_instance_delete", Capability.DESTROY, 4),
    ]
    body = "".join(record_audit_event(event, "", "") + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    return Config(
        audit=AuditConfig(
            reports={
                _WINDOW_REPORT: ReportConfig(
                    filter=ReportFilter(since_offset="2m"),
                    output=REPORT_OUTPUT_SUMMARY,
                )
            }
        )
    )


@pytest.fixture
def published_clock() -> Iterator[list[datetime]]:
    """A published reference instant the case moves, restored on the way out.

    A one-element list so a case can set the instant after the seam is already
    published, which is what keeps the reset paired with the set.
    """
    instant = [_SEEDED]
    token = set_clock(lambda: instant[0])
    try:
        yield instant
    finally:
        reset_clock(token)


async def _events_in_window(cfg: Config) -> int:
    """How many events the relative-window report answers under the seam."""
    result = await handle_linode_audit_report({"name": _WINDOW_REPORT}, cfg)
    answer = json.loads(result[0].text)

    return int(answer["total_events"])


async def test_the_published_clock_moves_the_relative_window(
    clock_config: Config, published_clock: list[datetime]
) -> None:
    """The published instant is what a relative window is measured back from.

    The same store and the same report answer differently under two instants,
    one standing just after the events and one standing long after them.
    """
    published_clock[0] = _SEEDED.replace(minute=1)
    assert await _events_in_window(clock_config) == 4

    published_clock[0] = _SEEDED.replace(year=2027, month=1, day=1)
    assert await _events_in_window(clock_config) == 0


async def test_no_published_clock_leaves_the_call_on_the_wall_clock(
    clock_config: Config,
) -> None:
    """With nothing published the window is measured from the wall clock.

    The seeded events sit at a fixed date in the past, so a two-minute window
    measured from now reaches none of them. Asserting the count rather than the
    instant keeps the case off whatever today reads.
    """
    assert await _events_in_window(clock_config) == 0
