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

from google.protobuf import json_format

if TYPE_CHECKING:
    from collections.abc import Callable

    from google.protobuf.message import Message


def proto_to_canonical_dict(message: Message) -> dict[str, Any]:
    """Serialize message to its canonical output dict.

    snake_case field names, default values emitted (empty repeated -> [],
    implicit-presence scalars -> their zero value), explicit-presence (optional)
    fields omitted when unset. Matches Go's protojson options.
    """
    text = json_format.MessageToJson(
        message,
        preserving_proto_field_name=True,
        always_print_fields_with_no_presence=True,
        indent=2,
    )
    result: dict[str, Any] = json.loads(text)
    return result


def serialize_api_response(raw: dict[str, Any], message: Message) -> dict[str, Any]:
    """Decode a raw API response into message and return its canonical output.

    Unknown fields (the API returns more than the proto models) are ignored, the
    same way Go's protojson DiscardUnknown decode does.
    """
    json_format.ParseDict(raw, message, ignore_unknown_fields=True)
    return proto_to_canonical_dict(message)


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
