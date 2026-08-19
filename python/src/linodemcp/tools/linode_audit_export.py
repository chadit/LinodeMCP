"""Phase 3f audit export tool.

``linode_audit_export`` dumps a filtered window of audit events to a
temp file in JSON, CSV, or NDJSON and returns the path. CapMeta, so
every profile can read it.

The SQLite path arrives through the shared module bridge in
:mod:`linodemcp.tools.linode_audit_summary` (the same path the summary
and health tools read), falling back to the JSONL log when unset.

Mirrors ``go/internal/tools/linode_audit_export.go``.
"""

from __future__ import annotations

import json
import tempfile
from typing import Any

from mcp.types import TextContent

from linodemcp.audit import (
    DEFAULT_EXPORT_MAX_RECORDS,
    MAX_EXPORT_RECORDS,
    RecentQuery,
    encode_events,
    export_events,
    resolve_default_audit_dir,
)
from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.tools.helpers import error_response, parse_optional_time
from linodemcp.tools.linode_audit_summary import audit_sqlite_path
from linodemcp.tools.proto_response import serialize_api_response

_ARG_FORMAT = "format"
_ARG_SINCE = "since"
_ARG_UNTIL = "until"
_ARG_TOOL = "tool"
_ARG_MAX_RECORDS = "max_records"
_ARG_INCLUDE_META = "include_meta"


def audit_export_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Export the filtered window to a temp file and return its path.

    The format's legal set is the contract's rule, so by here the name is one
    the encoder knows. A malformed since/until timestamp answers as a
    tool-result error, the way the Go hook's does.
    """
    export_format = arguments.get(_ARG_FORMAT, "")

    try:
        query = _build_export_query(arguments)
    except ValueError as exc:
        return error_response(str(exc))

    events = export_events(audit_sqlite_path(), resolve_default_audit_dir(), query)

    body = encode_events(events, export_format)

    try:
        path = _write_export_file(body, export_format)
    except OSError as exc:
        return error_response(f"failed to write export file: {exc}")

    payload = {
        "path": path,
        "format": export_format,
        "record_count": len(events),
    }
    result = serialize_api_response(payload, audit_pb2.AuditExportResponse())
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def _build_export_query(arguments: dict[str, Any]) -> RecentQuery:
    """Translate request arguments into a RecentQuery whose limit carries
    the resolved max_records cap. Raises ValueError on a bad timestamp.
    """
    return RecentQuery(
        limit=_resolve_max_records(arguments.get(_ARG_MAX_RECORDS)),
        since=parse_optional_time(arguments.get(_ARG_SINCE, "")),
        until=parse_optional_time(arguments.get(_ARG_UNTIL, "")),
        tool=arguments.get(_ARG_TOOL, ""),
        include_meta=bool(arguments.get(_ARG_INCLUDE_META, False)),
    )


def _resolve_max_records(requested: object) -> int:
    """Apply the default and hard cap to a requested max_records value.
    A missing, non-integer, or non-positive value uses the default.
    """
    if not isinstance(requested, int) or isinstance(requested, bool) or requested <= 0:
        return DEFAULT_EXPORT_MAX_RECORDS

    return min(requested, MAX_EXPORT_RECORDS)


def _write_export_file(body: str, export_format: str) -> str:
    """Write the encoded body to a temp file named with the format
    extension and return its path. The file is left in place for the
    user to read.
    """
    with tempfile.NamedTemporaryFile(
        mode="w",
        prefix="linode-audit-export-",
        suffix=f".{export_format}",
        delete=False,
        encoding="utf-8",
    ) as handle:
        handle.write(body)
        return handle.name
