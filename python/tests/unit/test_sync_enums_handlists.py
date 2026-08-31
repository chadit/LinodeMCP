"""Offline tests for the non-enum half of the sync gate (verify_sync_enums.py).

The gate diffs the validation value-sets against the live API spec. Each lives
on the contract: the ACL set and the membership rules as CEL alternations, the
placement group type as its field's reader_values, and the device slots as the
key vocabulary of the walk over the devices argument. These tests drive the real
CLI with a crafted `--spec` so no network is needed, and assert the drift logic:
the read-only `custom` ACL value is excluded, an integer set reads as the
strings the spec side carries, and a renamed declaration trips the gate rather
than diffing an empty set.
"""

from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT = REPO_ROOT / "scripts" / "verify_sync_enums.py"


def _enum_body(field: str, values: list[Any]) -> dict[str, Any]:
    """One endpoint declaring a single request-body enum.

    The membership-rule endpoints below are seven copies of this one shape, so
    they are built rather than written out; the ACL and device-slot endpoints
    keep their literals because they carry a second shape each.
    """
    return {
        "requestBody": {
            "content": {
                "application/json": {
                    "schema": {"properties": {field: {"enum": values}}}
                }
            }
        }
    }


# Minimal OpenAPI spec carrying only the hand-list endpoints. The create
# endpoint lists 4 canned ACLs; the access endpoint adds the read-only "custom"
# (schema-reuse with the GET response) that input validation must never accept.
MIN_SPEC: dict[str, Any] = {
    "info": {"version": "test"},
    "paths": {
        "/object-storage/buckets": {
            "post": {
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "properties": {
                                    "acl": {
                                        "enum": [
                                            "private",
                                            "public-read",
                                            "authenticated-read",
                                            "public-read-write",
                                        ]
                                    }
                                }
                            }
                        }
                    }
                }
            }
        },
        "/object-storage/buckets/{regionId}/{bucket}/access": {
            "put": {
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "properties": {
                                    "acl": {
                                        "enum": [
                                            "private",
                                            "public-read",
                                            "authenticated-read",
                                            "public-read-write",
                                            "custom",
                                        ]
                                    }
                                }
                            }
                        }
                    }
                }
            }
        },
        "/placement/groups": {
            "post": {
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "properties": {
                                    "placement_group_type": {
                                        "enum": ["anti_affinity:local"]
                                    }
                                }
                            }
                        }
                    }
                }
            }
        },
        "/linode/instances/{linodeId}/configs": {
            "post": {
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "properties": {
                                    "devices": {
                                        "type": "object",
                                        "properties": {
                                            slot: {}
                                            for slot in (
                                                "sda",
                                                "sdb",
                                                "sdc",
                                                "sdd",
                                                "sde",
                                                "sdf",
                                                "sdg",
                                                "sdh",
                                            )
                                        },
                                    }
                                }
                            }
                        }
                    }
                }
            }
        },
        "/monitor/services/{serviceType}/alert-definitions": {
            "post": _enum_body("severity", [0, 1, 2, 3])
        },
        "/databases/mysql/instances": {"post": _enum_body("cluster_size", [1, 2, 3])},
        "/databases/postgresql/instances": {
            "post": _enum_body("cluster_size", [1, 2, 3])
        },
        "/networking/ipv6/ranges": {"post": _enum_body("prefix_length", [56, 64])},
        "/support/tickets": {"post": _enum_body("severity", [1, 2, 3])},
        "/domains": {"post": _enum_body("status", ["active", "disabled"])},
        "/longview/plan": {
            "put": _enum_body(
                "longview_subscription",
                ["longview-3", "longview-10", "longview-40", "longview-100"],
            )
        },
    },
}

_CANNED_ACLS = ["private", "public-read", "authenticated-read", "public-read-write"]
_ALL_SLOTS = ["sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"]


def _gate() -> ModuleType:
    """Import the gate script by path, since scripts/ is not an installed package."""
    spec = importlib.util.spec_from_file_location("verify_sync_enums", SCRIPT)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _run(tmp_path: Path) -> str:
    """Run the gate offline against the crafted spec; return its output."""
    spec_file = tmp_path / "spec.json"
    spec_file.write_text(json.dumps(MIN_SPEC), encoding="utf-8")

    proc = subprocess.run(  # noqa: S603 - fixed argv, our own script, no shell
        [sys.executable, str(SCRIPT), "--spec", str(spec_file)],
        capture_output=True,
        text=True,
        check=False,
    )
    return proc.stdout + proc.stderr


def test_contract_sets_match_the_spec(tmp_path: Path) -> None:
    """Every declared vocabulary matches the spec, `custom` excluded.

    One assertion per map key, so an entry whose spec side stops resolving is
    named here instead of hiding behind a passing sibling.
    """
    output = _run(tmp_path)

    assert "custom" not in output
    for key in _gate().HAND_LIST_SPEC_MAP:
        assert f"{key}:" not in output


def test_integer_rule_declares_its_set() -> None:
    """An integer alternation reads as the strings the spec side carries."""
    assert _gate().proto_cel_values("ipv6_range_create.prefix_length.known") == {
        "56",
        "64",
    }


def test_every_severity_rule_declares_the_same_set() -> None:
    """The three tools the alert-definition entry names agree on the set.

    The entry diffs all three against one spec side, so a rule that drifts on
    its own has to be visible here.
    """
    gate = _gate()
    declared = {
        rule: gate.proto_cel_values(rule)
        for rule in gate.HAND_LIST_SPEC_MAP["alert_definition_severity"]["cel"]
    }
    assert declared == {rule: {"0", "1", "2", "3"} for rule in declared}


def test_a_member_that_is_not_a_literal_is_tripwire() -> None:
    """A member this reader cannot render leaves the set unread, not halved."""
    with pytest.raises(ValueError, match="is not a literal"):
        _gate()._alternation_members("some_tool.field.known", "'a', this.b")


def test_placement_type_reads_its_reader_values() -> None:
    """The contract field the placement vocabulary moved onto declares it."""
    assert _gate().proto_reader_values("placement_group_type") == {
        "anti_affinity:local"
    }


def test_renamed_reader_values_field_is_tripwire() -> None:
    """A renamed field yields no set, which must raise rather than diff nothing."""
    with pytest.raises(ValueError, match="no contract field declares reader_values"):
        _gate().proto_reader_values("placement_group_kind")


def test_acl_rule_declares_the_canned_set() -> None:
    """The contract rule the ACL set moved onto declares exactly the 4 values."""
    assert _gate().proto_cel_values(
        "object_storage_bucket_access_allow.acl.known"
    ) == set(_CANNED_ACLS)


def test_renamed_acl_rule_is_tripwire() -> None:
    """A renamed rule yields no set, which must raise rather than diff nothing."""
    with pytest.raises(ValueError, match="no rule declares this id"):
        _gate().proto_cel_values("object_storage_bucket_access_allow.acl.renamed")


def test_device_slots_read_their_walk_vocabulary() -> None:
    """The walk the slot set moved onto declares exactly the eight slots."""
    assert _gate().proto_walk_keys("devices") == set(_ALL_SLOTS)


def test_renamed_walked_argument_is_tripwire() -> None:
    """A renamed argument yields no set, which must raise rather than diff nothing."""
    with pytest.raises(ValueError, match="no object_walk declares a key vocabulary"):
        _gate().proto_walk_keys("device_slots")


def test_a_dropped_slot_is_drift(tmp_path: Path) -> None:
    """A slot the contract stops declaring is reported against the spec.

    Driven through the diff rather than the CLI, since the contract is the
    shipped proto and this is about what the comparison does with a short set.
    """
    diffs = _gate()._object_walk_diffs("config_device_slot", "devices", set(_ALL_SLOTS))
    assert diffs == []

    short = _gate()._object_walk_diffs("config_device_slot", "helpers", set(_ALL_SLOTS))
    assert short
    assert "sdh" in short[0]
