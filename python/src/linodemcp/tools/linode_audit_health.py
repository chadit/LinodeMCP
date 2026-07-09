"""Phase 3e audit health query tool.

``linode_audit_health`` reports the audit subsystem's own status: the
JSONL log path and disk footprint, rotated-file count and oldest date,
and (when the SQLite sink is enabled) row count, oldest event, and
database size. CapMeta, so every profile can read it. Takes no input.

The SQLite path arrives through the shared module bridge in
:mod:`linodemcp.tools.linode_audit_summary` (main installs it when the
sink is enabled), the same path the summary tool reads.

Mirrors ``go/internal/tools/linode_audit_health.go``.
"""

from __future__ import annotations

import json
from dataclasses import asdict
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.audit import collect_health, resolve_default_audit_dir
from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.linode_audit_summary import audit_sqlite_path
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema


def create_linode_audit_health_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_audit_health`` MCP tool definition."""
    return (
        Tool(
            name="linode_audit_health",
            description=(
                "Report the audit subsystem's status: log path and disk "
                "usage, rotated-file count and oldest date, and (when the "
                "SQLite sink is enabled) row count, oldest event, and "
                "database size."
            ),
            inputSchema=schema("linode.mcp.v1.AuditHealthInput"),
        ),
        Capability.Meta,
    )


async def handle_linode_audit_health(
    _arguments: dict[str, Any],
) -> list[TextContent]:
    """Collect audit subsystem status and return it as proto-canonical JSON."""
    report = collect_health(audit_sqlite_path(), resolve_default_audit_dir())
    result = serialize_api_response(asdict(report), audit_pb2.AuditHealthResponse())
    return [TextContent(type="text", text=json.dumps(result, indent=2))]
