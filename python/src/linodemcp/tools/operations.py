"""The local operations that read no ambient state.

An operation reading ambient state is served through the value that carries it,
which for the profile builder is ``BuilderState``. The rest are served from this
module, and the generated arm is handed the function itself rather than an
object to call a method on.

Each takes the values its declaration names and answers the shape it declares,
so none of them can tell which tool called it.
"""

from __future__ import annotations

import sqlite3
from typing import TYPE_CHECKING

from linodemcp.audit import (
    RecentQuery,
    ReportDefinitionError,
    UnknownGroupByColumnError,
    export_events,
    export_to_file,
    health,
    read_recent,
    read_with_fallback,
    resolve_default_audit_dir,
    resolve_max_records,
    resolve_sqlite_path,
    summary_over,
    validate_group_by,
)
from linodemcp.audit import (
    report as run_report,
)
from linodemcp.genlocal import (
    AuditExportResponse,
    AuditHealthResponse,
    AuditRecentResponse,
    AuditReportResponse,
    AuditSummaryResponse,
    LocalInputRejectedError,
    LocalNotFoundError,
    LocalReadFailedError,
    LocalWriteFailedError,
    VersionResponse,
)
from linodemcp.version import get_version_info

if TYPE_CHECKING:
    from datetime import datetime

    from linodemcp.config import Config

# What a read can answer with. Go's audit readers report every failure through
# one error return, so each of these maps onto the one read condition rather
# than escaping the handler.
_READ_FAILURES = (OSError, sqlite3.Error, ValueError)

# Machine names Python's platform.machine() reports mapped to Go's GOARCH so the
# version output's platform value matches the Go server on the same host.
_ARCH_ALIASES = {
    "x86_64": "amd64",
    "aarch64": "arm64",
    "i386": "386",
    "i686": "386",
}


def _normalized_platform(raw: str) -> str:
    """Normalize a "system/machine" string to Go's runtime.GOOS/GOARCH naming.

    Python reports "Darwin/arm64" or "Linux/x86_64"; Go reports "darwin/arm64"
    or "linux/amd64". Lowercasing the OS and aliasing the common arch names lines
    the two servers' version output up on the same host.
    """
    os_name, _, arch = raw.partition("/")
    return f"{os_name.lower()}/{_ARCH_ALIASES.get(arch, arch)}"


def build_info() -> VersionResponse:
    """The build information the binary reports about itself.

    The one construction of the version envelope: the tool's operation and the
    CLI ``version`` verb both project this answer, so every path in every
    language emits the field set version.proto pins (the shape the
    testdata/conformance/version_response.json fixture locks).
    """
    info = get_version_info()
    return VersionResponse(
        version=info.version,
        api_version=info.api_version,
        build_date=info.build_date,
        commit=info.git_commit,
        platform=_normalized_platform(info.platform),
    )


def audit_health(cfg: Config) -> AuditHealthResponse:
    """Answer the audit stores' own health.

    Raises LocalReadFailedError only where nothing readable answers: a
    configured database that will not open leaves the JSONL half standing behind
    a warning, which is the store's own degrade rule.
    """
    try:
        return health(resolve_sqlite_path(cfg), resolve_default_audit_dir())
    except _READ_FAILURES as exc:
        raise LocalReadFailedError(str(exc)) from exc


def audit_recent(
    limit: int,
    since: datetime | None,
    until: datetime | None,
    tool: str,
    capability: str,
    status: str,
    include_meta: bool,
) -> AuditRecentResponse:
    """Answer the events the JSONL log holds, newest first, through whichever
    filters the call named.

    No store and no cancellation: this operation reads the log alone, and the
    reader behind it takes neither.
    """
    try:
        events = read_recent(
            resolve_default_audit_dir(),
            RecentQuery(
                limit=limit,
                since=since,
                until=until,
                tool=tool,
                capability=capability,
                status=status,
                include_meta=include_meta,
            ),
        )
    except _READ_FAILURES as exc:
        raise LocalReadFailedError(str(exc)) from exc

    return AuditRecentResponse(count=len(events), events=list(events))


def audit_summary(
    cfg: Config,
    since: datetime | None,
    group_by: list[str],
    include_meta: bool,
) -> AuditSummaryResponse:
    """Count the events in a window into the buckets the call named.

    Raises LocalInputRejectedError for a column outside the vocabulary, which is
    answered before any store is opened, and LocalReadFailedError where nothing
    readable answers.
    """
    try:
        columns = validate_group_by(group_by)
    except UnknownGroupByColumnError as exc:
        raise LocalInputRejectedError(str(exc)) from exc

    try:
        return summary_over(
            resolve_sqlite_path(cfg),
            resolve_default_audit_dir(),
            since,
            columns,
            include_meta,
        )
    except _READ_FAILURES as exc:
        raise LocalReadFailedError(str(exc)) from exc


def audit_report(cfg: Config, clock: datetime, report: str) -> AuditReportResponse:
    """Run one named report from the audit configuration.

    The definition is resolved at call time rather than captured, so editing the
    report file takes effect on the next call. Raises LocalNotFoundError for a
    name the configuration does not declare, LocalInputRejectedError for a
    definition the vocabulary will not take, and LocalReadFailedError where
    nothing readable answers.
    """
    # run_report is audit.report under an alias: the declaration names this
    # operation's input report, and the conformance walk holds the parameter to
    # that name.
    definition = cfg.audit.reports.get(report)
    if definition is None:
        # No cause: the tool's own sentence names the report it was asked for,
        # which is the only thing there is to say about a name nothing declares.
        raise LocalNotFoundError

    try:
        return run_report(
            resolve_sqlite_path(cfg),
            resolve_default_audit_dir(),
            report,
            definition,
            clock,
        )
    except ReportDefinitionError as exc:
        raise LocalInputRejectedError(str(exc)) from exc
    except _READ_FAILURES as exc:
        raise LocalReadFailedError(str(exc)) from exc


def audit_export(
    cfg: Config,
    format_argument: str,
    since: datetime | None,
    until: datetime | None,
    tool: str,
    max_records: int,
    include_meta: bool,
) -> AuditExportResponse:
    """Write a window of events to a file and answer where it landed.

    Raises LocalReadFailedError where no store answers and
    LocalWriteFailedError where the file cannot be written. The record cap is
    applied by the subsystem so an unbounded range never reaches memory.
    """
    try:
        events, warnings = read_with_fallback(
            resolve_sqlite_path(cfg),
            lambda store: export_events(
                store,
                resolve_default_audit_dir(),
                RecentQuery(
                    limit=resolve_max_records(max_records),
                    since=since,
                    until=until,
                    tool=tool,
                    include_meta=include_meta,
                ),
            ),
        )
    except _READ_FAILURES as exc:
        raise LocalReadFailedError(str(exc)) from exc

    try:
        path = export_to_file(events, format_argument)
    except (OSError, ValueError) as exc:
        raise LocalWriteFailedError(str(exc)) from exc

    return AuditExportResponse(
        path=path,
        format=format_argument,
        record_count=len(events),
        warnings=warnings,
    )
