"""Explicit-null restoration in the canonical proto serializer.

The serializer emits no member for an unset optional scalar or an absent
sub-message, so a documented ``"gateway": null`` comes back missing. Mirrors
``go/internal/tools/proto_response_nulls_test.go`` so both languages pin the
same contract.
"""

from __future__ import annotations

from typing import Any

from linodemcp.genpb.linode.mcp.v1 import ip_pb2
from linodemcp.tools.proto_response import (
    restore_explicit_nulls,
    serialize_api_response,
)

# The six members Linode documents as nullable on a reserved address, which is
# the shape the restoration exists for.
_NULLABLE = (
    "assigned_entity",
    "gateway",
    "interface_id",
    "linode_id",
    "rdns",
    "vpc_nat_1_1",
)

_FULL_BODY: dict[str, Any] = {
    "address": "192.0.2.10",
    "assigned_entity": None,
    "gateway": None,
    "interface_id": None,
    "linode_id": None,
    "prefix": 24,
    "public": True,
    "rdns": None,
    "region": "us-east",
    "reserved": True,
    "subnet_mask": "255.255.255.0",
    "tags": [],
    "type": "ipv4",
    "vpc_nat_1_1": None,
}


def _serialize(raw: dict[str, Any]) -> tuple[dict[str, Any], Any]:
    """Decode a body the way a routed call does, answering it and its message."""
    message = ip_pb2.ReservedIPAddress()
    return serialize_api_response(raw, message), message


def test_restores_every_explicit_null_the_body_carried() -> None:
    """Every documented null comes back, in the order the message declares."""
    serialized, message = _serialize(_FULL_BODY)
    restored = restore_explicit_nulls(
        _FULL_BODY, serialized, _NULLABLE, message.DESCRIPTOR
    )

    assert list(restored) == [
        "address",
        "assigned_entity",
        "gateway",
        "interface_id",
        "linode_id",
        "prefix",
        "public",
        "rdns",
        "region",
        "reserved",
        "subnet_mask",
        "tags",
        "type",
        "vpc_nat_1_1",
    ]
    assert all(restored[name] is None for name in _NULLABLE)


def test_restores_only_the_nulls_the_api_sent() -> None:
    """A key the API never mentioned is never invented."""
    raw: dict[str, Any] = {
        "address": "192.0.2.10",
        "gateway": None,
        "region": "us-east",
    }
    serialized, message = _serialize(raw)
    restored = restore_explicit_nulls(raw, serialized, _NULLABLE, message.DESCRIPTOR)

    assert restored["gateway"] is None
    for name in ("assigned_entity", "interface_id", "linode_id", "rdns", "vpc_nat_1_1"):
        assert name not in restored


def test_restoring_nothing_leaves_the_serialized_answer_alone() -> None:
    """A tool declaring no field answers exactly what the serializer built."""
    raw: dict[str, Any] = {"address": "192.0.2.10", "gateway": None}
    serialized, message = _serialize(raw)

    assert (
        restore_explicit_nulls(raw, dict(serialized), (), message.DESCRIPTOR)
        == serialized
    )


def test_a_body_without_the_key_restores_nothing() -> None:
    """An absent key is not a null, so nothing is written for it."""
    raw: dict[str, Any] = {"address": "192.0.2.10", "region": "us-east"}
    serialized, message = _serialize(raw)

    assert (
        restore_explicit_nulls(raw, dict(serialized), _NULLABLE, message.DESCRIPTOR)
        == serialized
    )
