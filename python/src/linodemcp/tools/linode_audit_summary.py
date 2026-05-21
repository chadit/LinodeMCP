"""Phase 3d audit summary query tool.

``linode_audit_summary`` counts audit events grouped by tool/status
(or other allowlisted columns) over a time window. CapMeta, so every
profile can read it.

Reads the SQLite store when its path is installed via
:func:`set_audit_sqlite_path` (main wires this when the SQLite sink is
enabled), falling back to the JSONL log otherwise. The Python tool
factory takes no config, so the path arrives through this module
bridge rather than a constructor argument; the counts are identical
either way.

Mirrors ``go/internal/tools/linode_audit_summary.go``.
"""

from __future__ import annotations

import json
from dataclasses import asdict
from datetime import datetime
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.audit import (
    UnknownGroupByColumnError,
    load_window,
    resolve_default_audit_dir,
    summarize,
    validate_group_by,
)
from linodemcp.profiles import Capability

# Module bridge for the SQLite path. main installs the path when the
# SQLite sink is enabled; empty string (the default) selects the JSONL
# fallback in load_window.
_audit_sqlite_path: str = ""

_ARG_SINCE = "since"
_ARG_GROUP_BY = "group_by"
_ARG_INCLUDE_META = "include_meta"


def set_audit_sqlite_path(path: str) -> None:
    """Install the SQLite database path the summary tool should read.

    Pass an empty string (the default) to use the JSONL fallback.
    """
    global _audit_sqlite_path  # noqa: PLW0603 - process-wide bridge
    _audit_sqlite_path = path


def audit_sqlite_path() -> str:
    """Return the installed SQLite path, or empty string for the JSONL
    fallback. Sibling audit query tools (summary, health, export) share
    this single bridge rather than each holding their own.
    """
    return _audit_sqlite_path


def create_linode_audit_summary_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_audit_summary`` MCP tool definition."""
    return (
        Tool(
            name="linode_audit_summary",
            description=(
                "Count audit events grouped by tool and status (or other "
                "columns) over a time window. Reads SQLite when enabled, else "
                "the JSONL log."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_SINCE: {
                        "type": "string",
                        "description": (
                            "Only count events at or after this RFC 3339 timestamp."
                        ),
                    },
                    _ARG_GROUP_BY: {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": (
                            "Columns to group by. Allowed: tool, status, "
                            "capability, profile, environment. Defaults to "
                            "[tool, status]."
                        ),
                    },
                    _ARG_INCLUDE_META: {
                        "type": "boolean",
                        "description": (
                            "Include audit/profile meta-tool events. Default false."
                        ),
                    },
                },
            },
        ),
        Capability.Meta,
    )


async def handle_linode_audit_summary(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Aggregate audit events and return the count table as JSON.

    A malformed since timestamp or an unknown group_by column returns an
    error message rather than silently dropping the filter.
    """
    try:
        since = _parse_optional_time(arguments.get(_ARG_SINCE, ""))
        group_by = validate_group_by(arguments.get(_ARG_GROUP_BY))
    except (ValueError, UnknownGroupByColumnError) as exc:
        return [TextContent(type="text", text=str(exc))]

    include_meta = bool(arguments.get(_ARG_INCLUDE_META, False))

    events = load_window(
        _audit_sqlite_path,
        resolve_default_audit_dir(),
        since,
        include_meta,
    )
    rows = summarize(events, group_by)

    payload = {
        "total_events": len(events),
        "rows": [asdict(row) for row in rows],
    }
    return [TextContent(type="text", text=json.dumps(payload))]


def _parse_optional_time(value: str) -> datetime | None:
    """Parse an RFC 3339 timestamp, or None for an empty value."""
    if not value:
        return None

    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        msg = f"invalid 'since' timestamp: expected RFC 3339, got {value!r}: {exc}"
        raise ValueError(msg) from exc
