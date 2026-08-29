"""The report engine's own tests.

Mirrors ``go/internal/audit/report_test.go``. They live beside the engine
because the engine moved into ``linodemcp.audit``, and they reach the two
conditions no config file can produce: the offset and the bounds are validated
at load time, so a definition the vocabulary will not take arrives here only
from a config nothing checked.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    ReportDefinitionError,
    Status,
    event_timestamp,
    report,
)
from linodemcp.config import (
    REPORT_OUTPUT_LIST,
    REPORT_OUTPUT_SUMMARY,
    ReportConfig,
    ReportFilter,
)
from linodemcp.genlocal import (
    record_audit_event,
)

if TYPE_CHECKING:
    from pathlib import Path

_REPORT_NAME = "window"
_LIST_TOOL = "linode_instance_list"
_CREATE_TOOL = "linode_instance_create"
_DELETE_TOOL = "linode_instance_delete"
_META_TOOL = "linode_audit_recent"

# Reaches back past the oldest seeded event, so a report carrying it takes all
# four unless one of its own filters drops one.
_WHOLE_WINDOW = "4h"

# The reference instant a relative window is measured back from, standing one
# hour after the newest seeded event.
_CLOCK = datetime(2026, 5, 19, 13, 0, 0, tzinfo=UTC)


def _event(tool: str, capability: Capability, status: Status, hour: int) -> Event:
    """One seeded event at a distinct hour, carrying the environment and profile
    the post-load globs read.
    """
    ts = datetime(2026, 5, 19, hour, 0, 0, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{tool}_{hour}",
        tool=tool,
        tool_capability=capability,
        environment="production",
        profile="full-access",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=status,
        latency_ms=1,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="sess-audit-report",
        credential_generation=0,
    )


@pytest.fixture
def seeded_dir(tmp_path: Path) -> Path:
    """Four events in a fresh JSONL log.

    Three an hour apart and a fourth that is a meta event, which every other
    reader defaults out and the report asks for.
    """
    events = [
        _event(_LIST_TOOL, Capability.READ, Status.SUCCESS, 10),
        _event(_CREATE_TOOL, Capability.WRITE, Status.SUCCESS, 11),
        _event(_DELETE_TOOL, Capability.DESTROY, Status.ERROR, 12),
        _event(_META_TOOL, Capability.META, Status.SUCCESS, 12),
    ]
    lines = [record_audit_event(event, "", "") for event in events]
    (tmp_path / "audit.log").write_text("\n".join(lines) + "\n", encoding="utf-8")

    return tmp_path


def _summary(filter_: ReportFilter) -> ReportConfig:
    """A report grouping the seeded window by status."""
    return ReportConfig(
        filter=filter_, output=REPORT_OUTPUT_SUMMARY, group_by=["status"]
    )


def _listing(filter_: ReportFilter, limit: int) -> ReportConfig:
    """A report listing the seeded window."""
    return ReportConfig(filter=filter_, output=REPORT_OUTPUT_LIST, limit=limit)


def test_report_measures_a_relative_window_from_the_instant_it_is_handed(
    seeded_dir: Path,
) -> None:
    """The clock reaches the window rather than being read inside the engine.

    The same store and the same report answer differently under two instants,
    which is the property LOCAL_AMBIENT_CLOCK exists to keep a tenth language
    from losing.
    """
    definition = _summary(ReportFilter(since_offset=_WHOLE_WINDOW))

    inside = report("", str(seeded_dir), _REPORT_NAME, definition, _CLOCK)
    assert inside.total_events == 4

    long_after = _CLOCK.replace(year=_CLOCK.year + 1)

    outside = report("", str(seeded_dir), _REPORT_NAME, definition, long_after)
    assert outside.total_events == 0


def test_report_summarizes_into_the_buckets_its_definition_names(
    seeded_dir: Path,
) -> None:
    """The rows carry the grouped counts and the event list stays empty."""
    answer = report(
        "",
        str(seeded_dir),
        _REPORT_NAME,
        _summary(ReportFilter(since_offset=_WHOLE_WINDOW)),
        _CLOCK,
    )

    assert answer.name == _REPORT_NAME
    assert answer.output == REPORT_OUTPUT_SUMMARY
    assert len(answer.rows) == 2
    assert answer.events == []


def test_report_lists_under_its_own_limit(seeded_dir: Path) -> None:
    """The truncation, which is the only place the limit is read."""
    answer = report(
        "",
        str(seeded_dir),
        _REPORT_NAME,
        _listing(ReportFilter(since_offset=_WHOLE_WINDOW), 1),
        _CLOCK,
    )

    assert answer.total_events == 1
    assert len(answer.events) == 1
    assert answer.rows == []


def test_report_bounds_an_absolute_window_at_both_ends(seeded_dir: Path) -> None:
    """Two timestamps rather than an offset, which the clock never touches."""
    answer = report(
        "",
        str(seeded_dir),
        _REPORT_NAME,
        _summary(
            ReportFilter(since="2026-05-19T10:30:00Z", until="2026-05-19T11:30:00Z")
        ),
        _CLOCK,
    )

    assert answer.total_events == 1


@pytest.mark.parametrize(
    ("case", "filter_", "want"),
    [
        (
            "a capability list",
            ReportFilter(
                since_offset=_WHOLE_WINDOW, capability_in=["write", "destroy"]
            ),
            2,
        ),
        (
            "a status list",
            ReportFilter(since_offset=_WHOLE_WINDOW, status_in=["error"]),
            1,
        ),
        (
            "an environment glob",
            ReportFilter(since_offset=_WHOLE_WINDOW, environment="prod*"),
            4,
        ),
        (
            "a profile glob nothing matches",
            ReportFilter(since_offset=_WHOLE_WINDOW, profile="no-such-*"),
            0,
        ),
    ],
)
def test_report_keeps_only_the_events_its_post_filters_admit(
    seeded_dir: Path, case: str, filter_: ReportFilter, want: int
) -> None:
    """The four filter members no store query carries, each narrowing the same
    seeded window.
    """
    answer = report("", str(seeded_dir), _REPORT_NAME, _summary(filter_), _CLOCK)

    assert answer.total_events == want, case


@pytest.mark.parametrize(
    ("case", "definition"),
    [
        (
            "an offset that is not a duration",
            _summary(ReportFilter(since_offset="two hours")),
        ),
        ("a since that is not a timestamp", _summary(ReportFilter(since="yesterday"))),
        ("an until that is not a timestamp", _summary(ReportFilter(until="tomorrow"))),
        (
            "a group_by column outside the summary vocabulary",
            ReportConfig(output=REPORT_OUTPUT_SUMMARY, group_by=["nonsense"]),
        ),
    ],
)
def test_report_refuses_a_definition_the_vocabulary_will_not_take(
    seeded_dir: Path, case: str, definition: ReportConfig
) -> None:
    """The four ways a report file can be written wrong.

    The configuration validator reads the offset and the bounds first, so these
    reach the engine only from a definition nothing checked. That is what makes
    them the engine's own condition rather than a failed read.
    """
    with pytest.raises(ReportDefinitionError):
        report("", str(seeded_dir), _REPORT_NAME, definition, _CLOCK)

    assert case


def test_report_keeps_the_cause_out_of_the_conditions_words(seeded_dir: Path) -> None:
    """The sentence is the cause's own, so the tool layer's wording is not
    doubled with the condition's.
    """
    definition = ReportConfig(output=REPORT_OUTPUT_SUMMARY, group_by=["nonsense"])

    with pytest.raises(ReportDefinitionError) as raised:
        report("", str(seeded_dir), _REPORT_NAME, definition, _CLOCK)

    assert str(raised.value) == (
        'validate report group_by: audit: unknown group_by column: "nonsense"'
    )


def test_report_answers_a_read_failure_rather_than_a_definition_one(
    tmp_path: Path,
) -> None:
    """A store nothing can read is not the definition's fault, and the tool layer
    words the two differently.
    """
    sealed = tmp_path / "sealed"
    sealed.mkdir()
    sealed.chmod(0o000)

    try:
        with pytest.raises(PermissionError) as raised:
            report(
                "",
                str(sealed),
                _REPORT_NAME,
                _summary(ReportFilter(since_offset=_WHOLE_WINDOW)),
                _CLOCK,
            )

        assert not isinstance(raised.value, ReportDefinitionError)
    finally:
        sealed.chmod(0o750)


def test_report_carries_the_warning_when_it_falls_back_to_the_log(
    seeded_dir: Path,
) -> None:
    """The store fallback through the report, which is the answer member the
    engine builds once with everything else rather than assigning in afterwards.
    """
    store = seeded_dir / "audit.db"
    store.write_text("this is not a sqlite database", encoding="utf-8")

    answer = report(
        str(store),
        str(seeded_dir),
        _REPORT_NAME,
        _summary(ReportFilter(since_offset=_WHOLE_WINDOW)),
        _CLOCK,
    )

    assert len(answer.warnings) == 1
    assert answer.total_events == 4

    # The list output builds its own answer, so it carries the warning through a
    # second constructor the summary case never reaches.
    listed = report(
        str(store),
        str(seeded_dir),
        _REPORT_NAME,
        _listing(ReportFilter(since_offset=_WHOLE_WINDOW), 0),
        _CLOCK,
    )

    assert len(listed.warnings) == 1


def test_report_reads_the_offset_as_the_span_it_names(seeded_dir: Path) -> None:
    """A narrower offset takes fewer events, which is what says the duration is
    parsed rather than ignored: 90 minutes back from the reference instant
    reaches the two events at noon and leaves the two earlier ones out.
    """
    narrow = report(
        "",
        str(seeded_dir),
        _REPORT_NAME,
        _summary(ReportFilter(since_offset="90m")),
        _CLOCK,
    )

    assert narrow.total_events == 2
    assert _CLOCK - timedelta(minutes=90) == datetime(
        2026, 5, 19, 11, 30, 0, tzinfo=UTC
    )


def test_report_counts_the_meta_events_the_other_readers_leave_out(
    seeded_dir: Path,
) -> None:
    """The one query member the report sets for itself rather than taking from a
    caller.

    The report grammar decides meta inclusion through its own capability filter,
    so its load query always asks for meta events; every other reader defaults
    them out. Nothing else in either net reaches that, which the mutation run
    said before this case existed.
    """
    answer = report(
        "",
        str(seeded_dir),
        _REPORT_NAME,
        _summary(
            ReportFilter(since_offset=_WHOLE_WINDOW, capability=Capability.META.value)
        ),
        _CLOCK,
    )

    assert answer.total_events == 1
