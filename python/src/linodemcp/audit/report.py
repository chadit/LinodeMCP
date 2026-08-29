"""One named report, run over the same stores the other query operations read.

Mirrors ``go/internal/audit/report.go``. The engine sits here rather than
beside the tool because every decision it makes belongs to the audit subsystem:
which store answers, what a window means, which events a filter admits, and
what a definition the vocabulary will not take does. The tool layer is left
with the two conditions those decisions report.
"""

from __future__ import annotations

import fnmatch
from datetime import datetime, timedelta
from typing import TYPE_CHECKING

from linodemcp.audit.export import MAX_EXPORT_RECORDS, export_events
from linodemcp.audit.reader import RecentQuery
from linodemcp.audit.store import read_with_fallback
from linodemcp.audit.summary import (
    UnknownGroupByColumnError,
    summarize,
    validate_group_by,
)
from linodemcp.config import REPORT_OUTPUT_SUMMARY, parse_duration_seconds
from linodemcp.genlocal import AuditReportResponse

if TYPE_CHECKING:
    from linodemcp.audit.event import Event
    from linodemcp.config import ReportConfig, ReportFilter


class ReportDefinitionError(ValueError):
    """Raised for a report the configuration wrote wrong.

    An offset that is not a duration, a bound that is not a timestamp, a
    group_by column outside the summary vocabulary. It is the caller's own
    condition rather than a failed read, and the two reach the tool layer
    through one call, so the failure has to say which it is.
    """


def report(
    sqlite_path: str,
    jsonl_dir: str,
    name: str,
    definition: ReportConfig,
    now: datetime,
) -> AuditReportResponse:
    """Run one named report over the configured database where one answers and
    over the JSONL log otherwise.

    Raises ReportDefinitionError for a definition the vocabulary will not take
    and whatever the read failed with otherwise. A configured database that will
    not open leaves the log standing behind a warning, which is the store's own
    degrade rule.

    ``now`` is the reference instant a relative window is measured back from. It
    is handed in rather than read here so a pinned clock reaches the window,
    which is what lets a report fixture replay.
    """
    query = _report_query(definition.filter, now)

    events, warnings = read_with_fallback(
        sqlite_path,
        lambda store: export_events(store, jsonl_dir, query),
    )

    matched = [event for event in events if _report_admits(event, definition.filter)]

    return _report_answer(name, definition, matched, warnings)


def _report_answer(
    name: str,
    definition: ReportConfig,
    matched: list[Event],
    warnings: list[str],
) -> AuditReportResponse:
    """The report's own answer, built once with its warnings in whichever
    output the definition names.

    Both outputs fill both list members because the answer declares both, and
    the one the output did not build is answered empty rather than left out.
    """
    if definition.output == REPORT_OUTPUT_SUMMARY:
        return AuditReportResponse(
            name=name,
            output=REPORT_OUTPUT_SUMMARY,
            total_events=len(matched),
            rows=summarize(matched, _report_columns(definition)),
            events=[],
            warnings=warnings,
        )

    if definition.limit > 0 and len(matched) > definition.limit:
        matched = matched[: definition.limit]

    return AuditReportResponse(
        name=name,
        output=definition.output,
        total_events=len(matched),
        rows=[],
        events=list(matched),
        warnings=warnings,
    )


def _report_columns(definition: ReportConfig) -> list[str]:
    """The columns a summary report groups by, refusing one the vocabulary lacks.

    The cause is re-raised under the definition's own condition so the sentence
    says which validation failed, the way the other language's does.
    """
    try:
        return validate_group_by(list(definition.group_by))
    except UnknownGroupByColumnError as exc:
        msg = f"validate report group_by: {exc}"
        raise ReportDefinitionError(msg) from exc


def _report_query(filter_: ReportFilter, now: datetime) -> RecentQuery:
    """Translate the filter members a store query carries into one query.

    include_meta is True because the report grammar decides meta inclusion
    through its own capability filter rather than through the tool-layer
    default, and since_offset wins over the absolute since when both are set.
    """
    return RecentQuery(
        limit=MAX_EXPORT_RECORDS,
        since=_report_since(filter_, now),
        until=_report_bound(filter_.until),
        tool=filter_.tool,
        capability=filter_.capability,
        status=filter_.status,
        include_meta=True,
    )


def _report_since(filter_: ReportFilter, now: datetime) -> datetime | None:
    """The load-time lower bound: the offset measured back from the reference
    instant where the filter names one, else the absolute timestamp, else none.
    """
    if not filter_.since_offset:
        return _report_bound(filter_.since)

    try:
        span = parse_duration_seconds(filter_.since_offset)
    except ValueError as exc:
        raise ReportDefinitionError(str(exc)) from exc

    return now - timedelta(seconds=span)


def _report_bound(value: str) -> datetime | None:
    """Parse one absolute bound, answering None for an absent one.

    The configuration validator reads these first, so a value that fails here
    reached the operation from a file nothing checked.
    """
    if not value:
        return None

    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        raise ReportDefinitionError(str(exc)) from exc


def _report_admits(event: Event, filter_: ReportFilter) -> bool:
    """Whether one event clears every filter member a store query cannot carry.

    The two membership lists, and the environment and profile globs.
    """
    capability = event.tool_capability
    if filter_.capability_in and capability not in filter_.capability_in:
        return False

    if filter_.status_in and event.status not in filter_.status_in:
        return False

    return _glob_admits(filter_.environment, event.environment) and _glob_admits(
        filter_.profile, event.profile
    )


def _glob_admits(pattern: str, value: str) -> bool:
    """Whether one glob admits a value, with an unset glob admitting every one."""
    return not pattern or fnmatch.fnmatchcase(value, pattern)
