"""64-bit widening behavior of the canonical proto serializer.

The proto3 JSON mapping emits int64/uint64 fields as quoted strings; the
canonical serializer converts them back to JSON numbers so a number-typed
field is always a JSON number. Mirrors ``go/internal/tools/proto_int64_test.go``
so both languages pin the same contract.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from google.protobuf import descriptor_pb2, descriptor_pool, message_factory

from linodemcp.genpb.linode.mcp.v1 import audit_pb2
from linodemcp.tools.proto_response import serialize_api_response

if TYPE_CHECKING:
    from google.protobuf.message import Message

_WIDEN_PACKAGE = "linodemcp.test.widen"


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


def _widen_case_message() -> Message:
    """Build a message carrying a repeated int64 and a map<string, int64>.

    The proto contract has neither shape today (every 64-bit field in it is
    singular), so the serializer's repeated and map widening arms have no
    fixture to drive them. Declaring the shapes here at runtime, in a private
    pool that never touches the real registry, drives the public serializer
    down those arms and pins the behavior for the day the contract grows such a
    field. Go's walk reaches the same two positions through its list and
    map-value contexts.
    """
    file_proto = descriptor_pb2.FileDescriptorProto()
    file_proto.name = "linodemcp_test/widen.proto"
    file_proto.package = _WIDEN_PACKAGE
    file_proto.syntax = "proto3"

    message_proto = file_proto.message_type.add()
    message_proto.name = "WidenCase"

    counters = message_proto.field.add()
    counters.name = "counters"
    counters.number = 1
    counters.label = descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    counters.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT64

    entry_proto = message_proto.nested_type.add()
    entry_proto.name = "TotalsEntry"
    entry_proto.options.map_entry = True
    entry_key = entry_proto.field.add()
    entry_key.name = "key"
    entry_key.number = 1
    entry_key.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    entry_key.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    entry_value = entry_proto.field.add()
    entry_value.name = "value"
    entry_value.number = 2
    entry_value.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    entry_value.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT64

    totals = message_proto.field.add()
    totals.name = "totals"
    totals.number = 2
    totals.label = descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    totals.type = descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE
    totals.type_name = f".{_WIDEN_PACKAGE}.WidenCase.TotalsEntry"

    pool = descriptor_pool.DescriptorPool()
    pool.Add(file_proto)
    descriptor = pool.FindMessageTypeByName(f"{_WIDEN_PACKAGE}.WidenCase")

    return message_factory.GetMessageClass(descriptor)()


def test_repeated_and_map_int64_fields_serialize_as_numbers() -> None:
    """Repeated and map-valued 64-bit fields widen like singular ones."""
    payload: dict[str, Any] = {
        "counters": [1, -2, 9007199254740993],
        "totals": {"hits": 5, "misses": -7},
    }

    out = serialize_api_response(payload, _widen_case_message())

    assert out["counters"] == [1, -2, 9007199254740993]
    assert all(isinstance(item, int) for item in out["counters"])
    assert out["totals"] == {"hits": 5, "misses": -7}
    assert all(isinstance(item, int) for item in out["totals"].values())
