"""Focused tests for the tracking-issue state gate."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"

CLOSED_ISSUE = "https://github.com/chadit/LinodeMCP-Issue/issues/1038"
OPEN_ISSUE = "https://github.com/chadit/LinodeMCP-Issue/issues/1064"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_tracking_issues")


def _baseline(tmp_path: Path, name: str, lines: list[str]) -> Path:
    path = tmp_path / name
    path.write_text("# header\n" + "\n".join(lines) + "\n", encoding="utf-8")
    return path


def test_collects_every_entry_citing_one_issue(tmp_path: Path) -> None:
    """A closed issue has to name every entry that moves with it.

    Reporting per line would understate the fix; issue 1038 was cited by twenty
    entries across eight files.
    """
    first = _baseline(
        tmp_path,
        "a-baseline.txt",
        [f"tool_one  # accepted 2026-07-16 {CLOSED_ISSUE}"],
    )
    second = _baseline(
        tmp_path,
        "b-baseline.txt",
        [
            f"tool_two  # accepted 2026-07-16 {CLOSED_ISSUE}",
            f"tool_three  # accepted 2026-07-16 {OPEN_ISSUE}",
        ],
    )

    citations = gate.cited_issues([first, second])

    assert citations[CLOSED_ISSUE] == [
        "a-baseline.txt: tool_one",
        "b-baseline.txt: tool_two",
    ]
    assert citations[OPEN_ISSUE] == ["b-baseline.txt: tool_three"]


def test_reports_a_closed_issue_with_its_entries() -> None:
    """The report leads with the dead issue, then what it was holding."""
    citations = {
        CLOSED_ISSUE: ["scope-sync-baseline.txt: one", "behavior-baseline.txt: two"],
        OPEN_ISSUE: ["list-envelope-baseline.txt: three"],
    }

    reported = gate.closed_citations(
        citations, {CLOSED_ISSUE: "closed", OPEN_ISSUE: "open"}
    )

    assert reported == [
        f"{CLOSED_ISSUE} is closed",
        "    behavior-baseline.txt: two",
        "    scope-sync-baseline.txt: one",
    ]


def test_an_unresolvable_issue_is_not_assumed_open() -> None:
    """A deleted or renumbered issue is as ownerless as a closed one."""
    citations = {CLOSED_ISSUE: ["a-baseline.txt: one"]}

    assert gate.closed_citations(citations, {}) == [
        f"{CLOSED_ISSUE} is unknown",
        "    a-baseline.txt: one",
    ]


def test_an_open_issue_passes() -> None:
    """The gate only fires on promises that lost their owner."""
    citations = {OPEN_ISSUE: ["a-baseline.txt: one"]}

    assert gate.closed_citations(citations, {OPEN_ISSUE: "open"}) == []


def test_snapshots_are_not_walked(tmp_path: Path) -> None:
    """Snapshot baselines record upstream state and carry no acceptances."""
    _baseline(tmp_path, "api-pagination-baseline.txt", ["route  # not an acceptance"])
    _baseline(
        tmp_path, "real-baseline.txt", [f"tool  # accepted 2026-07-16 {OPEN_ISSUE}"]
    )

    names = {path.name for path in gate.guarded_files(tmp_path)}

    assert "real-baseline.txt" in names
    assert "api-pagination-baseline.txt" not in names


def test_exemption_files_are_walked(tmp_path: Path) -> None:
    """Exemptions carry the same dated annotation, so they are checked too."""
    (tmp_path / "scope-sync-exempt.txt").write_text(
        f"entry\treason  # accepted 2026-07-25 {OPEN_ISSUE}\n", encoding="utf-8"
    )

    names = {path.name for path in gate.guarded_files(tmp_path)}

    assert "scope-sync-exempt.txt" in names


def test_a_closed_issue_fails_the_gate(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """End to end: a recorded closed state exits non-zero and names the issue.

    This is the case the shape-only URL check passed for five days.
    """
    contracts = tmp_path / "contracts"
    contracts.mkdir()
    _baseline(
        contracts, "thing-baseline.txt", [f"tool  # accepted 2026-07-16 {CLOSED_ISSUE}"]
    )
    monkeypatch.setattr(gate, "_CONTRACTS", contracts)

    states = tmp_path / "states.json"
    states.write_text(json.dumps({CLOSED_ISSUE: "closed"}), encoding="utf-8")

    assert gate.main(["--states", str(states)]) == 1
    assert CLOSED_ISSUE in capsys.readouterr().err


def test_the_real_contracts_cite_only_open_issues() -> None:
    """Every acceptance in the repo points at a live promise.

    Recorded states keep this offline; the gate resolves them for real on the
    sync schedule.
    """
    contracts = REPO_ROOT / "docs" / "contracts"
    citations = gate.cited_issues(gate.guarded_files(contracts))

    assert gate.closed_citations(citations, dict.fromkeys(citations, "open")) == []
