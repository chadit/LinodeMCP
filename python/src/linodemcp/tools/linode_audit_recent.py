"""Phase 2c recent-events query tool.

``linode_audit_recent`` returns the most recent audit events from the
on-disk JSONL log, newest first, with optional filters. Carries
``Capability.Meta`` so every profile (including read-only) can read
it: inspecting what the assistant did should never need write access.

Mirrors ``go/internal/tools/linode_audit_recent.go``.
"""

from __future__ import annotations

import json
from typing import Any

from mcp.types import TextContent

from linodemcp.audit import RecentQuery, read_recent, resolve_default_audit_dir
from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.tools.helpers import error_response, parse_optional_time
from linodemcp.tools.proto_response import serialize_api_response

# Argument-key constants shared by the schema and the handler so the
# two can't drift.
_ARG_LIMIT = "limit"
_ARG_SINCE = "since"
_ARG_UNTIL = "until"
_ARG_TOOL = "tool"
_ARG_CAPABILITY = "capability"
_ARG_STATUS = "status"
_ARG_INCLUDE_META = "include_meta"


def audit_recent_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Read recent audit events and return them as a JSON envelope.

    The response is ``{"count": N, "events": [...]}`` with events
    newest-first. A malformed since/until timestamp answers as a tool-result
    error, the way the Go hook's does.
    """
    try:
        query = _build_recent_query(arguments)
    except ValueError as exc:
        return error_response(str(exc))

    events = read_recent(resolve_default_audit_dir(), query)

    payload = {
        "count": len(events),
        "events": [event.to_dict() for event in events],
    }
    result = serialize_api_response(payload, audit_pb2.AuditRecentResponse())

    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def _build_recent_query(arguments: dict[str, Any]) -> RecentQuery:
    """Translate request arguments into a RecentQuery.

    Raises ``ValueError`` with a parameter-naming message for a
    malformed since/until timestamp.
    """
    return RecentQuery(
        limit=int(arguments.get(_ARG_LIMIT, 0) or 0),
        since=parse_optional_time(arguments.get(_ARG_SINCE, ""), _ARG_SINCE),
        until=parse_optional_time(arguments.get(_ARG_UNTIL, ""), _ARG_UNTIL),
        tool=str(arguments.get(_ARG_TOOL, "")),
        capability=str(arguments.get(_ARG_CAPABILITY, "")),
        status=str(arguments.get(_ARG_STATUS, "")),
        include_meta=bool(arguments.get(_ARG_INCLUDE_META, False)),
    )
