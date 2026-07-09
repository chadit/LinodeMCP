"""Offline tests for the hand-list half of the sync gate (verify_sync_enums.py).

The gate diffs the hand-maintained validation lists (bucket ACL, placement group
type, config device slots) against the live API spec. These tests drive the real
CLI with a crafted `--spec` and `--go-lists` so no network or `go` toolchain is
needed, and assert the drift logic: the read-only `custom` ACL value is excluded,
dropped values are flagged, and a missing/renamed Go symbol trips the gate.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT = REPO_ROOT / "scripts" / "verify_sync_enums.py"

# Minimal OpenAPI spec carrying only the three hand-list endpoints. The create
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
    },
}

_CANNED_ACLS = ["private", "public-read", "authenticated-read", "public-read-write"]
_ALL_SLOTS = ["sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"]


def _run(tmp_path: Path, go_lists: dict[str, list[str]]) -> str:
    """Run the gate offline against the crafted spec and go-lists; return output."""
    spec_file = tmp_path / "spec.json"
    spec_file.write_text(json.dumps(MIN_SPEC), encoding="utf-8")
    go_file = tmp_path / "go.json"
    go_file.write_text(json.dumps(go_lists), encoding="utf-8")

    proc = subprocess.run(  # noqa: S603 - fixed argv, our own script, no shell
        [
            sys.executable,
            str(SCRIPT),
            "--spec",
            str(spec_file),
            "--go-lists",
            str(go_file),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    return proc.stdout + proc.stderr


def test_custom_acl_excluded_no_drift(tmp_path: Path) -> None:
    """A 4-value ACL hand-list matches the spec: `custom` must not read as drift."""
    output = _run(
        tmp_path,
        {
            "bucket_acl": _CANNED_ACLS,
            "placement_group_type": ["anti_affinity:local"],
            "config_device_slot": _ALL_SLOTS,
        },
    )
    assert "bucket_acl:" not in output
    assert "custom" not in output
    assert "placement_group_type:" not in output


def test_dropped_acl_value_flagged(tmp_path: Path) -> None:
    """Dropping a canned ACL from the Go hand-list is reported against the spec."""
    output = _run(
        tmp_path,
        {
            "bucket_acl": ["private", "public-read", "authenticated-read"],
            "placement_group_type": ["anti_affinity:local"],
            "config_device_slot": _ALL_SLOTS,
        },
    )
    assert "bucket_acl: go hand-list missing API value(s)" in output
    assert "public-read-write" in output


def test_missing_device_slot_flagged(tmp_path: Path) -> None:
    """A dropped slot drifts: slots come from the `devices` object property names."""
    output = _run(
        tmp_path,
        {
            "bucket_acl": _CANNED_ACLS,
            "placement_group_type": ["anti_affinity:local"],
            "config_device_slot": ["sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg"],
        },
    )
    assert "config_device_slot: go hand-list missing API value(s)" in output
    assert "sdh" in output


def test_missing_go_key_is_tripwire(tmp_path: Path) -> None:
    """A renamed/removed Go symbol yields no key, which must fail loudly."""
    output = _run(
        tmp_path,
        {
            "bucket_acl": _CANNED_ACLS,
            "placement_group_type": ["anti_affinity:local"],
        },
    )
    assert "config_device_slot: go hand-list empty or missing" in output
