"""64-bit widening behavior of the canonical proto serializer.

The proto3 JSON mapping emits int64/uint64 fields as quoted strings; the
canonical serializer converts them back to JSON numbers so a number-typed
field is always a JSON number. Mirrors ``go/internal/tools/proto_int64_test.go``
so both languages pin the same contract.
"""

from __future__ import annotations

from typing import Any

from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.tools.proto_response import serialize_api_response


def test_int64_fields_serialize_as_numbers() -> None:
    """64-bit counters emit as ints, including inside a nested message."""
    payload: dict[str, Any] = {
        "jsonl_path": "/var/log/linodemcp/audit",
        "active_log_exists": True,
        "rotated_file_count": 0,
        "oldest_rotated_date": "",
        "disk_bytes": 40960,
        "dropped_events": 0,
        "sqlite": {
            "path": "/var/log/linodemcp/audit.db",
            "event_count": 1200,
            "oldest_event_unix_ns": 1782734400000000000,
            "db_bytes": 262144,
        },
    }

    out = serialize_api_response(payload, audit_pb2.AuditHealthResponse())

    assert out["disk_bytes"] == 40960
    assert isinstance(out["disk_bytes"], int)
    assert out["dropped_events"] == 0
    assert isinstance(out["dropped_events"], int)
    sqlite = out["sqlite"]
    assert sqlite["event_count"] == 1200
    assert isinstance(sqlite["event_count"], int)
    # The nanosecond timestamp is the value most exposed to precision loss;
    # int round-tripping keeps every digit.
    assert sqlite["oldest_event_unix_ns"] == 1782734400000000000
    assert isinstance(sqlite["oldest_event_unix_ns"], int)


def test_struct_digit_strings_stay_strings() -> None:
    """The walk is descriptor-driven: free-form Struct args keep strings.

    A redacted tool argument that happens to share a 64-bit field's name
    (disk_bytes) and carry a digit-only string value must not be widened.
    """
    event: dict[str, Any] = {
        "ts": "2026-06-29T12:00:00.5Z",
        "ts_unix_ns": 1782734400500000000,
        "event_id": "01JYyyyyyyyyyyyyyyyyyyyyyy",
        "tool": "linode_instance_list",
        "tool_capability": "read",
        "environment": "",
        "profile": "",
        "mode": "normal",
        "args": {"disk_bytes": "40960"},
        "args_redacted": [],
        "status": "success",
        "latency_ms": 250,
        "result_summary": "",
        "linodemcp_version": "",
        "session_id": "",
        "credential_generation": 2,
    }

    out = serialize_api_response(event, audit_pb2.AuditEvent())

    assert out["ts_unix_ns"] == 1782734400500000000
    assert isinstance(out["ts_unix_ns"], int)
    assert out["latency_ms"] == 250
    assert isinstance(out["latency_ms"], int)
    assert out["credential_generation"] == 2
    assert isinstance(out["credential_generation"], int)
    assert out["args"]["disk_bytes"] == "40960"
    assert isinstance(out["args"]["disk_bytes"], str)
