"""Phase 2c recent-events query tool.

``linode_audit_recent`` returns the most recent audit events from the
on-disk JSONL log, newest first, with optional filters. Carries
``Capability.Meta`` so every profile (including read-only) can read
it: inspecting what the assistant did should never need write access.

Mirrors ``go/internal/tools/linode_audit_recent.go``.
"""

from __future__ import annotations

import json
from datetime import datetime
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.audit import RecentQuery, read_recent, resolve_default_audit_dir
from linodemcp.profiles import Capability

# Argument-key constants shared by the schema and the handler so the
# two can't drift.
_ARG_LIMIT = "limit"
_ARG_SINCE = "since"
_ARG_UNTIL = "until"
_ARG_TOOL = "tool"
_ARG_CAPABILITY = "capability"
_ARG_STATUS = "status"
_ARG_INCLUDE_META = "include_meta"


def create_linode_audit_recent_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_audit_recent`` MCP tool definition."""
    return (
        Tool(
            name="linode_audit_recent",
            description=(
                "Return the most recent audit events (what tools were called, "
                "with what outcome), newest first. Reads the on-disk JSONL "
                "audit log. Optional filters: limit, since, until, tool "
                "(glob), capability, status, include_meta."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_LIMIT: {
                        "type": "integer",
                        "description": (
                            "Max events to return. Default 20, capped at 200."
                        ),
                    },
                    _ARG_SINCE: {
                        "type": "string",
                        "description": (
                            "Only events at or after this RFC 3339 timestamp "
                            "(e.g. 2026-05-19T00:00:00Z)."
                        ),
                    },
                    _ARG_UNTIL: {
                        "type": "string",
                        "description": (
                            "Only events at or before this RFC 3339 timestamp."
                        ),
                    },
                    _ARG_TOOL: {
                        "type": "string",
                        "description": (
                            "Only events whose tool name matches this glob "
                            '(e.g. "linode_instance_*").'
                        ),
                    },
                    _ARG_CAPABILITY: {
                        "type": "string",
                        "description": (
                            "Only events with this capability: read, write, "
                            "destroy, admin, or meta."
                        ),
                    },
                    _ARG_STATUS: {
                        "type": "string",
                        "description": (
                            "Only events with this status: success, error, or refused."
                        ),
                    },
                    _ARG_INCLUDE_META: {
                        "type": "boolean",
                        "description": (
                            "Include audit/profile meta-tool events. Default "
                            "false (they are noise for activity review)."
                        ),
                    },
                },
            },
        ),
        Capability.Meta,
    )


async def handle_linode_audit_recent(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Read recent audit events and return them as a JSON envelope.

    The response is ``{"count": N, "events": [...]}`` with events
    newest-first. A malformed since/until timestamp returns an error
    message rather than silently dropping the filter.
    """
    try:
        query = _build_recent_query(arguments)
    except ValueError as exc:
        return [TextContent(type="text", text=str(exc))]

    events = read_recent(resolve_default_audit_dir(), query)
    payload = {
        "count": len(events),
        "events": [event.to_dict() for event in events],
    }

    return [TextContent(type="text", text=json.dumps(payload))]


def _build_recent_query(arguments: dict[str, Any]) -> RecentQuery:
    """Translate request arguments into a RecentQuery.

    Raises ``ValueError`` with a parameter-naming message for a
    malformed since/until timestamp.
    """
    return RecentQuery(
        limit=int(arguments.get(_ARG_LIMIT, 0) or 0),
        since=_parse_optional_time(_ARG_SINCE, arguments.get(_ARG_SINCE, "")),
        until=_parse_optional_time(_ARG_UNTIL, arguments.get(_ARG_UNTIL, "")),
        tool=str(arguments.get(_ARG_TOOL, "")),
        capability=str(arguments.get(_ARG_CAPABILITY, "")),
        status=str(arguments.get(_ARG_STATUS, "")),
        include_meta=bool(arguments.get(_ARG_INCLUDE_META, False)),
    )


def _parse_optional_time(param: str, value: str) -> datetime | None:
    """Parse an RFC 3339 timestamp, or None for an empty value.

    Raises ``ValueError`` naming ``param`` for a non-empty but
    unparseable value.
    """
    if not value:
        return None

    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        msg = f"invalid '{param}' timestamp: expected RFC 3339, got {value!r}: {exc}"
        raise ValueError(msg) from exc
