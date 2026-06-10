"""Tests for the per-resource-type hash-ignore lists.

Mirrors the Go ``hash_ignore_test.go``. A regression here (a dropped or
renamed cosmetic field) would otherwise only surface as a false drift refusal
at apply time, so assert the lists directly.
"""

from __future__ import annotations

import pytest

from linodemcp.twostage.hash_ignore import hash_ignore_fields

_UPDATED = "updated"
_CREATED = "created"

_CASES = [
    ("Instance", [_UPDATED, "last_seen_ipv4", "watchdog_enabled", "host_uuid"]),
    ("Volume", [_UPDATED, "last_seen_ipv4"]),
    ("LKECluster", [_UPDATED, _CREATED]),
    ("Firewall", [_UPDATED]),
    ("NodeBalancer", [_UPDATED, "transfer"]),
    ("VPC", [_UPDATED]),
    ("Domain", [_UPDATED]),
    ("StackScript", [_UPDATED, "deployments_total"]),
    ("Disk", [_UPDATED]),
    ("VPCSubnet", [_UPDATED]),
    ("DomainRecord", [_UPDATED]),
    ("LKENodePool", ["nodes"]),
    ("DatabaseInstance", [_UPDATED]),
    ("ImageShareGroup", [_UPDATED]),
    ("ImageShareGroupToken", [_UPDATED]),
    ("FirewallDevice", [_UPDATED]),
    ("LKEKubeconfig", [_UPDATED, _CREATED]),
    ("LKEServiceToken", [_UPDATED, _CREATED]),
]


@pytest.mark.parametrize(("resource_type", "must_contain"), _CASES)
def test_hash_ignore_fields(resource_type: str, must_contain: list[str]) -> None:
    fields = hash_ignore_fields(resource_type)
    for field in must_contain:
        assert field in fields, f"{resource_type} missing {field!r}: {fields}"


def test_hash_ignore_fields_unknown_returns_empty() -> None:
    # An unmapped type hashes its whole state: the safe conservative default.
    assert hash_ignore_fields("NoSuchResource") == []
