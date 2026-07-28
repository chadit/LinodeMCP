"""Focused tests for the behavior gate's malformed-response rule.

A mutating fixture that decodes an API response body must also prove the tool
rejects a malformed one, since decoding is hand-written per language. These
pin the predicate: what counts as decoding, what counts as proof, and which
fixtures the rule leaves alone. Synthetic fixtures throughout, so the rule is
tested independently of whatever the real testdata tree currently holds.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING, cast

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"

_TOOL = "linode_widget_create"

# A case that hands the tool a populated object to decode, which is what puts
# a fixture in scope for the rule.
_DECODE_CASE: dict[str, object] = {
    "name": "returns the created widget",
    "args": {"confirm": True, "label": "widget"},
    "api_response": {"id": 1, "label": "widget"},
    "expect_result": {"id": 1, "label": "widget"},
}


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_behavior")


def _missing(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capability: str,
    cases: list[dict[str, object]],
) -> set[str]:
    """Run the rule over one synthetic fixture tagged with one capability."""
    capabilities = tmp_path / "tools-capabilities.txt"
    capabilities.write_text(
        f"# synthetic capability tags\n{_TOOL}\t{capability}\n", encoding="utf-8"
    )
    monkeypatch.setattr(gate, "_CAPABILITIES", capabilities)

    return cast("set[str]", gate._missing_shape_rejection({_TOOL: cases}))


def test_decoding_fixture_without_a_malformed_body_case_is_reported(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Decoding a body without proving the rejection is the gap the rule exists for."""
    assert _missing(tmp_path, monkeypatch, "Write", [_DECODE_CASE]) == {_TOOL}


def test_non_object_api_response_satisfies_the_rule(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A JSON array where an object belongs is a shape the tool must refuse."""
    rejection: dict[str, object] = {
        "name": "rejects top-level array",
        "api_response": [],
        "expect_api_error": "create widget",
    }

    assert _missing(tmp_path, monkeypatch, "Write", [_DECODE_CASE, rejection]) == set()


def test_null_api_response_satisfies_the_rule(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """JSON null and an absent field both read as None, so membership has to decide."""
    rejection: dict[str, object] = {
        "name": "rejects top-level null",
        "api_response": None,
        "expect_api_error": "create widget",
    }

    assert _missing(tmp_path, monkeypatch, "Write", [_DECODE_CASE, rejection]) == set()


def test_api_response_raw_satisfies_the_rule(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Raw bytes are the only way to serve a truncated or empty body."""
    rejection: dict[str, object] = {
        "name": "rejects malformed success body",
        "api_response_raw": '{"id":',
        "expect_api_error": "create widget",
    }

    assert _missing(tmp_path, monkeypatch, "Write", [_DECODE_CASE, rejection]) == set()


def test_malformed_body_without_an_assertion_does_not_satisfy_the_rule(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Serving the bad body proves nothing until a case pins what comes back."""
    served: dict[str, object] = {
        "name": "serves an empty body",
        "api_response_raw": "",
        "expect_result": {},
    }

    assert _missing(tmp_path, monkeypatch, "Write", [_DECODE_CASE, served]) == {_TOOL}


def test_fixture_that_never_decodes_a_body_is_not_required_to_reject_one(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Validation and request pinning alone leave no decode path to attack."""
    cases: list[dict[str, object]] = [
        {"name": "requires label", "expect_error": "label is required"},
        {
            "name": "sends the documented body",
            "args": {"confirm": True, "label": "widget"},
            "api_response": {},
            "expect_request": {"method": "POST", "path": "/widgets"},
        },
    ]

    assert _missing(tmp_path, monkeypatch, "Write", cases) == set()


def test_read_capability_tool_is_not_required_to_reject_a_malformed_body(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Scope is the mutating tiers; readers are out of this ratchet's reach."""
    assert _missing(tmp_path, monkeypatch, "Read", [_DECODE_CASE]) == set()


def test_destroy_capability_tool_is_in_scope(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Destroy decodes response bodies too, so it carries the same requirement."""
    assert _missing(tmp_path, monkeypatch, "Destroy", [_DECODE_CASE]) == {_TOOL}


def test_update_baseline_writes_the_response_shape_ratchet(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """--update-baseline has to rewrite the new file alongside the older two."""
    paths = {
        "_BASELINE": tmp_path / "behavior-baseline.txt",
        "_DRYRUN_BASELINE": tmp_path / "behavior-dryrun-baseline.txt",
        "_SHAPE_BASELINE": tmp_path / "behavior-response-shape-baseline.txt",
    }
    for attr, path in paths.items():
        monkeypatch.setattr(gate, attr, path)

    assert gate._update_baselines(set(), set(), {_TOOL}) == 0

    written = paths["_SHAPE_BASELINE"].read_text(encoding="utf-8")
    assert written.startswith("#")
    assert "verify_behavior.py --update-baseline" in written
    assert [
        line for line in written.splitlines() if line and not line.startswith("#")
    ] == [_TOOL]


def test_response_shape_baseline_is_covered_by_the_growth_guard() -> None:
    """An unguarded ratchet could grow silently, which defeats the ratchet."""
    guard = _load_script("verify_baseline_direction")
    contracts = REPO_ROOT / "docs" / "contracts"
    guarded = {path.name for path in guard._guarded_baselines(contracts)}

    assert gate._SHAPE_BASELINE.name in guarded
