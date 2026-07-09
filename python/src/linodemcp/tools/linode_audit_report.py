"""Phase 4b audit report tool.

``linode_audit_report`` runs a user-defined named report from
``audit.reports`` against the active event store. CapMeta, so every
profile can read it. The report definition is resolved from the
installed config snapshot at call time, so editing the report file
takes effect on the next call.

The SQLite path arrives through the shared module bridge in
:mod:`linodemcp.tools.linode_audit_summary`. The reports map arrives
through this module's own ``set_audit_reports`` bridge (main installs
both at startup).

Mirrors ``go/internal/tools/linode_audit_report.go``.
"""

from __future__ import annotations

import fnmatch
import json
from dataclasses import asdict
from datetime import UTC, datetime, timedelta
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.audit import (
    MAX_EXPORT_RECORDS,
    Event,
    RecentQuery,
    export_events,
    resolve_default_audit_dir,
    summarize,
    validate_group_by,
)
from linodemcp.config import (
    REPORT_OUTPUT_LIST,
    REPORT_OUTPUT_SUMMARY,
    ReportConfig,
    ReportFilter,
    parse_duration_seconds,
)
from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.linode_audit_summary import audit_sqlite_path
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema

# Module bridge for the reports map. main installs the catalog from
# the loaded config; an unset bridge returns an empty dict (no reports
# defined).
_audit_reports: dict[str, ReportConfig] = {}


def set_audit_reports(reports: dict[str, ReportConfig]) -> None:
    """Install the named-report catalog the report tool should resolve
    against. Called from main once the config is loaded.
    """
    global _audit_reports  # noqa: PLW0603 - process-wide bridge
    _audit_reports = reports


def audit_reports() -> dict[str, ReportConfig]:
    """Return the installed report catalog, or an empty dict if unset."""
    return _audit_reports


def create_linode_audit_report_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_audit_report`` MCP tool definition."""
    return (
        Tool(
            name="linode_audit_report",
            description=(
                "Run a named custom audit report from config (audit.reports). "
                "Reads SQLite when enabled, else the JSONL log. Returns a "
                "summary of counts or a list of matching events depending on "
                "the report's output mode."
            ),
            inputSchema=schema("linode.mcp.v1.AuditReportInput"),
        ),
        Capability.Meta,
    )


async def handle_linode_audit_report(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Resolve the named report, run it, and return summary or list output."""
    name = str(arguments.get("name", ""))
    if not name:
        return [TextContent(type="text", text="report name is required")]

    report = audit_reports().get(name)
    if report is None:
        return [TextContent(type="text", text=f"unknown report: {name!r}")]

    try:
        payload = _run_report(name, report, datetime.now(UTC))
    except ValueError as exc:
        return [TextContent(type="text", text=str(exc))]

    result = serialize_api_response(payload, audit_pb2.AuditReportResponse())
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def _run_report(name: str, report: ReportConfig, now: datetime) -> dict[str, Any]:
    """Load events for the report, apply the post-filter (the _in lists
    and environment/profile globs RecentQuery doesn't carry), and emit
    summary or list output. Raises ValueError on a bad filter.
    """
    query = _build_report_load_query(report.filter, now)
    events = export_events(audit_sqlite_path(), resolve_default_audit_dir(), query)

    predicate = _compile_report_predicate(report.filter)
    filtered = [event for event in events if predicate(event)]

    if report.output == REPORT_OUTPUT_SUMMARY:
        group_by = validate_group_by(list(report.group_by))
        rows = summarize(filtered, group_by)
        return {
            "name": name,
            "output": REPORT_OUTPUT_SUMMARY,
            "total_events": len(filtered),
            "rows": [asdict(row) for row in rows],
        }

    # list output
    if report.limit > 0 and len(filtered) > report.limit:
        filtered = filtered[: report.limit]

    return {
        "name": name,
        "output": REPORT_OUTPUT_LIST,
        "total_events": len(filtered),
        "events": [event.to_dict() for event in filtered],
    }


def _build_report_load_query(filter_: ReportFilter, now: datetime) -> RecentQuery:
    """Translate the filter fields RecentQuery carries (tool glob, scalar
    capability/status, since/until) into a load-time query. include_meta
    is True: the report grammar controls meta inclusion explicitly via
    the capability filter, not the tool-layer default. since_offset
    takes precedence over the absolute since when both are set.
    """
    return RecentQuery(
        limit=MAX_EXPORT_RECORDS,
        since=_resolve_report_since(filter_, now),
        until=_resolve_report_until(filter_),
        tool=filter_.tool,
        capability=filter_.capability,
        status=filter_.status,
        include_meta=True,
    )


def _resolve_report_since(filter_: ReportFilter, now: datetime) -> datetime | None:
    """since_offset (now - duration) when set, else the absolute since,
    else None (no lower bound). The 4a validator already caught bad
    values, so parse errors here would be unexpected.
    """
    if filter_.since_offset:
        seconds = parse_duration_seconds(filter_.since_offset)
        return now - timedelta(seconds=seconds)

    if filter_.since:
        return datetime.fromisoformat(filter_.since)

    return None


def _resolve_report_until(filter_: ReportFilter) -> datetime | None:
    """Parse the absolute until bound, returning None when absent."""
    if not filter_.until:
        return None

    return datetime.fromisoformat(filter_.until)


def _compile_report_predicate(filter_: ReportFilter) -> Any:
    """Per-event predicate for the filter fields RecentQuery does not
    carry: the _in lists for capability and status, plus the
    environment and profile globs.
    """

    def matches(event: Event) -> bool:
        cap_value = event.tool_capability.value
        if filter_.capability_in and cap_value not in filter_.capability_in:
            return False

        if filter_.status_in and event.status.value not in filter_.status_in:
            return False

        if filter_.environment and not fnmatch.fnmatchcase(
            event.environment, filter_.environment
        ):
            return False

        return not (
            filter_.profile and not fnmatch.fnmatchcase(event.profile, filter_.profile)
        )

    return matches
