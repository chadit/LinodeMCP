"""Canonical proto serialization for tool output.

Proto-backed read tools decode the raw Linode API response into the generated
proto message and serialize it here, so their output is structurally identical
to the Go implementation's MarshalProtoToolResponse (protojson UseProtoNames +
EmitDefaultValues). The cross-language conformance gate
(tests/unit/test_proto_conformance.py) asserts against proto_to_canonical_dict,
so handlers and the gate share one serialization path. Comparison with Go is
structural: protojson and MessageToJson differ only in whitespace.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

from google.protobuf import json_format, struct_pb2
from google.protobuf.descriptor import FieldDescriptor

if TYPE_CHECKING:
    from collections.abc import Callable

    from google.protobuf.message import Message

# The five 64-bit integer field types the proto3 JSON mapping emits as quoted
# strings; the widening pass below converts them back to JSON numbers.
_INT64_FIELD_TYPES = frozenset(
    {
        FieldDescriptor.TYPE_INT64,
        FieldDescriptor.TYPE_UINT64,
        FieldDescriptor.TYPE_SINT64,
        FieldDescriptor.TYPE_FIXED64,
        FieldDescriptor.TYPE_SFIXED64,
    }
)

# Well-known types whose JSON form is free-form user data. Digit-only strings
# inside them are genuine strings, so the widening pass never descends into
# these subtrees.
_FREEFORM_MESSAGES = frozenset(
    {
        "google.protobuf.Struct",
        "google.protobuf.Value",
        "google.protobuf.ListValue",
    }
)


def raw_str(raw: Any, key: str) -> str:
    """Read a string field from a raw API response, "" when absent or non-dict.

    Write handlers build their message string from the POST/PUT response. The
    raw body is typed Any (it comes off httpx.json()), so this narrows it to a
    dict and coerces the field to str to keep the message-building code typed.
    """
    if isinstance(raw, dict):
        value = cast("dict[str, Any]", raw).get(key)
        if isinstance(value, str):
            return value
    return ""


def raw_int(raw: Any, key: str) -> int:
    """Read an int field from a raw API response, 0 when absent or non-dict."""
    if isinstance(raw, dict):
        value = cast("dict[str, Any]", raw).get(key)
        if isinstance(value, int):
            return value
    return 0


def _is_freeform(descriptor: Any) -> bool:
    """Report whether descriptor is a free-form well-known type.

    Typed Any because the runtime descriptor comes from the C (upb) backend,
    which pyright cannot unify with the pure-Python Descriptor stub.
    """
    return bool(descriptor.full_name in _FREEFORM_MESSAGES)


def _widen_scalar(value: Any) -> Any:
    """Convert a protojson 64-bit string back to an int, pass others through."""
    return int(value) if isinstance(value, str) else value


def _widen_message_field(node: dict[str, Any], field: Any, value: Any) -> None:
    """Widen 64-bit values under a message-typed field (map, repeated, or one)."""
    message_type = field.message_type
    if message_type.GetOptions().map_entry:
        value_field = message_type.fields_by_name["value"]
        if not isinstance(value, dict):
            return
        typed_map = cast("dict[str, Any]", value)
        if value_field.type in _INT64_FIELD_TYPES:
            node[field.name] = {
                key: _widen_scalar(item) for key, item in typed_map.items()
            }
        elif value_field.type == FieldDescriptor.TYPE_MESSAGE and not _is_freeform(
            value_field.message_type
        ):
            for item in typed_map.values():
                _widen_int64_fields(item, value_field.message_type)
    elif _is_freeform(message_type):
        return
    elif isinstance(value, list):
        # cast to list[object], not list[Any]: mypy narrows the Any to
        # list[Any] already (so that cast would be redundant) while pyright
        # still needs a concrete element type.
        for item in cast("list[object]", value):
            _widen_int64_fields(item, message_type)
    else:
        _widen_int64_fields(value, message_type)


def _widen_int64_fields(node: Any, descriptor: Any) -> None:
    """Convert 64-bit integer fields from protojson strings to JSON numbers.

    The proto3 JSON mapping quotes int64/uint64 so JavaScript consumers never
    round values past 2^53, but this project's contract wants numbers to be
    numbers: JSON carries arbitrary-precision integers, protojson parsers
    accept both forms on decode, and the realistic values here (byte counters,
    latencies, counts) sit far below 2^53. The walk is descriptor-driven so
    digit strings inside free-form Struct/Value subtrees (redacted audit args,
    dashboard widgets) stay strings. Mirrors Go's widenInt64JSON.
    """
    if not isinstance(node, dict):
        return
    typed_node = cast("dict[str, Any]", node)
    for field in descriptor.fields:
        if field.name not in typed_node:
            continue
        value = typed_node[field.name]
        if field.type == FieldDescriptor.TYPE_MESSAGE:
            _widen_message_field(typed_node, field, value)
        elif field.type in _INT64_FIELD_TYPES:
            if isinstance(value, list):
                typed_node[field.name] = [
                    _widen_scalar(item) for item in cast("list[object]", value)
                ]
            else:
                typed_node[field.name] = _widen_scalar(value)


def proto_to_canonical_dict(message: Message) -> dict[str, Any]:
    """Serialize message to its canonical output dict.

    snake_case field names, default values emitted (empty repeated -> [],
    implicit-presence scalars -> their zero value), explicit-presence (optional)
    fields omitted when unset, 64-bit integer fields as JSON numbers (see
    _widen_int64_fields). Matches Go's MarshalProtoJSON.
    """
    text = json_format.MessageToJson(
        message,
        preserving_proto_field_name=True,
        always_print_fields_with_no_presence=True,
        indent=2,
    )
    result: dict[str, Any] = json.loads(text)
    if not _is_freeform(message.DESCRIPTOR):
        _widen_int64_fields(result, message.DESCRIPTOR)
    return result


def serialize_api_response(raw: dict[str, Any], message: Message) -> dict[str, Any]:
    """Decode a raw API response into message and return its canonical output.

    Unknown fields (the API returns more than the proto models) are ignored, the
    same way Go's protojson DiscardUnknown decode does.
    """
    json_format.ParseDict(raw, message, ignore_unknown_fields=True)
    return proto_to_canonical_dict(message)


def _sorted_deep(value: Any) -> Any:
    """Sort dict keys recursively at every nesting level.

    Python protobuf map iteration order (which backs Struct) is not guaranteed,
    so relying on MessageToJson's emission order makes Struct output flip
    between runs. Go's protojson sorts map keys deterministically; this makes
    the Python side match by construction.
    """
    if isinstance(value, dict):
        typed = cast("dict[str, Any]", value)
        return {key: _sorted_deep(typed[key]) for key in sorted(typed)}
    if isinstance(value, list):
        return [_sorted_deep(item) for item in cast("list[object]", value)]
    return value


def serialize_struct_response(raw: dict[str, Any]) -> dict[str, Any]:
    """Serialize a free-form API object through a bare google.protobuf.Struct.

    Read tools whose response is an open-ended object (engine config
    descriptors, profile preferences, managed stats) use this so both languages
    emit the same key-sorted object: keys are sorted recursively at every
    nesting level, matching Go's structpb + protojson map ordering. Mirrors
    Go's MarshalStructToolResponse.
    """
    result = _sorted_deep(serialize_api_response(raw, struct_pb2.Struct()))
    return cast("dict[str, Any]", result)


def serialize_preview_envelope(raw: dict[str, Any], message: Message) -> dict[str, Any]:
    """Serialize a dry-run or plan envelope and sort its free-form subtrees.

    The envelope's typed fields serialize in field order like any proto message,
    but current_state and would_execute.body are google.protobuf.Value subtrees
    whose object keys Python's MessageToJson leaves in insertion order; Go's
    protojson sorts them. Sorting just those two subtrees makes both languages
    emit the same key order without disturbing the top-level field order. Mirrors
    the Go builders, which route current_state and body through structpb.Value so
    protojson sorts them there.
    """
    result = serialize_api_response(raw, message)
    if "current_state" in result:
        result["current_state"] = _sorted_deep(result["current_state"])
    would = result.get("would_execute")
    if isinstance(would, dict):
        would_dict = cast("dict[str, Any]", would)
        if "body" in would_dict:
            would_dict["body"] = _sorted_deep(would_dict["body"])
    return result


def serialize_list_response(
    raw: Any,
    key: str,
    message: Message,
    *,
    filter_value: str | None = None,
    item_filter: Callable[[dict[str, Any]], bool] | None = None,
) -> dict[str, Any]:
    """Build the project list envelope from a raw Linode page and serialize it.

    Linode list endpoints return {data, page, pages, results}; the project's list
    contract is {count, <key>, filter?} with proto-canonical elements. This pulls
    the data[] page, optionally filters it client-side, wraps it as count + key
    (+ filter echo), then routes the whole thing through the proto *ListResponse
    message so the output matches Go's MarshalProtoToolResponse element-for-element.

    message must be the proto list-response message whose repeated field is named
    key (e.g. InstanceListResponse with key "instances").
    """
    page: dict[str, Any] = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
    data_value = page.get("data", [])
    data_list: list[object] = (
        cast("list[object]", data_value) if isinstance(data_value, list) else []
    )
    items: list[dict[str, Any]] = [
        cast("dict[str, Any]", item) for item in data_list if isinstance(item, dict)
    ]

    if item_filter is not None:
        items = [item for item in items if item_filter(item)]

    wrapper: dict[str, Any] = {"count": len(items), key: items}
    if filter_value:
        wrapper["filter"] = filter_value

    return serialize_api_response(wrapper, message)
