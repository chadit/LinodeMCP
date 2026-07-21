"""Offline tests for the tool-parity gate's scope comparison.

verify_tool_parity.py diffs each tool's required OAuth scopes across
languages (the dumpers emit a "scopes" field). These tests pin the
normalization, the divergence line, the pass case, and the went-blind
guard; the full gate runs live in `make check`.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_tool_parity")


def _record(scopes: list[str]) -> dict[str, object]:
    return {
        "name": "linode_example_list",
        "capability": "Read",
        "params": {"page": "number"},
        "required": [],
        "scopes": scopes,
    }


def test_normalize_sorts_scopes_and_defaults_missing_to_empty() -> None:
    sorted_rec = gate._normalize(_record(["b:read_only", "a:read_only"]))
    legacy_rec = gate._normalize(
        {"name": "x", "capability": "Read", "params": {}, "required": []}
    )

    assert sorted_rec["scopes"] == ["a:read_only", "b:read_only"]
    assert legacy_rec["scopes"] == []


def test_scope_divergence_produces_one_named_line() -> None:
    ref = gate._normalize(_record(["ips:read_only"]))
    other = gate._normalize(_record(["reserved-ips:read_only"]))

    problems = gate._compare_one("linode_example_list", "go", ref, "python", other)

    assert problems == [
        "linode_example_list: scopes go=['ips:read_only']"
        " python=['reserved-ips:read_only']"
    ]


def test_identical_scopes_produce_no_divergence() -> None:
    ref = gate._normalize(_record(["ips:read_only"]))
    other = gate._normalize(_record(["ips:read_only"]))

    assert gate._compare_one("linode_example_list", "go", ref, "python", other) == []


def test_surface_without_any_scopes_fails_loudly() -> None:
    """A dumper that stops emitting scopes must fail, not compare empty."""
    blind = {"go": {"tool_a": gate._normalize(_record([]))}}

    with pytest.raises(SystemExit, match="no scopes"):
        gate._require_scope_signal(blind)


def test_surface_with_scope_signal_passes() -> None:
    surfaces = {
        "go": {
            "tool_a": gate._normalize(_record([])),
            "tool_b": gate._normalize(_record(["linodes:read_only"])),
        }
    }

    gate._require_scope_signal(surfaces)
