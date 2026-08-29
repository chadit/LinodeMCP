"""Tests for the gates that keep their debt class at zero with no baseline file.

Each of these gates used to carry a ratchet that had emptied. An empty ratchet
still said one useful thing: it proved the gate had a surface to look at, since
a scan that stopped covering anything would have made every accepted entry read
as fixed. Without the file, "no findings" and "nothing scanned" print the same,
so every hard gate declares what it measured and fails on zero.

These pin both halves for the gates that have no test file of their own: a
synthetic straggler fails, and a classifier that resolves nothing fails too.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from collections.abc import Callable
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"
CONTRACTS = REPO_ROOT / "docs" / "contracts"

_TOOL = "linode_widget_create"

# Each surface of the generated-form gate, with the status its classifiers
# report for a tool that is done. A tool at any other status is a straggler.
_FORM_SURFACES = (
    ("write", "proto"),
    ("read", "proto"),
    ("input", "generated"),
    ("meta", "proto"),
)


def _fixed_dump(status: str) -> Callable[[str], dict[str, str]]:
    """A classifier stand-in reporting one tool at *status* on any surface."""

    def dump(_surface: str) -> dict[str, str]:
        return {_TOOL: status}

    return dump


def _empty_dump(_surface: str) -> dict[str, str]:
    """A classifier stand-in that resolves nothing on any surface."""
    return {}


# The baselines these gates used to carry. Nothing reads them now, so a file
# reappearing here means someone recorded a finding instead of fixing it.
_RETIRED_BASELINES = (
    "write-proto-baseline.txt",
    "write-proto-fixture-baseline.txt",
    "read-proto-baseline.txt",
    "input-proto-baseline.txt",
    "meta-proto-baseline.txt",
    "message-parity-baseline.txt",
    "behavior-baseline.txt",
    "behavior-response-shape-baseline.txt",
    "list-envelope-baseline.txt",
    "pagination-baseline.txt",
    "response-shape-baseline.txt",
    "route-evidence-baseline.txt",
)


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


hardgate = _load_script("_hardgate")


def test_measured_passes_a_scan_that_covered_something() -> None:
    """The normal path says nothing and gets out of the way."""
    assert hardgate.measured("the widget scan", 1) is None


def test_measured_fails_a_scan_that_covered_nothing() -> None:
    """Zero is the failure this helper exists for, and it names the scan."""
    with pytest.raises(SystemExit, match="the widget scan covered nothing"):
        hardgate.measured("the widget scan", 0)


def test_report_is_quiet_and_clean_with_no_findings() -> None:
    """A clean gate prints its own summary line, not this helper's."""
    assert hardgate.report("widgets that drifted", [], "fix them") == 0


def test_report_names_every_finding_and_fails(
    capsys: pytest.CaptureFixture[str],
) -> None:
    """There is no accepted subset, so every finding is printed and fails."""
    code = hardgate.report("widgets that drifted", ["a", "b"], "fix them")
    printed = capsys.readouterr().out

    assert code == 1
    assert "widgets that drifted (2)" in printed
    assert "  a" in printed
    assert "  b" in printed
    assert "fix them" in printed


@pytest.mark.parametrize(("surface", "done"), _FORM_SURFACES)
def test_generated_form_fails_on_a_straggler(
    surface: str, done: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    """One side hand-written is the whole point of the merged gate."""
    gate = _load_script("verify_generated_form")
    monkeypatch.setattr(gate, "_dump_go", _fixed_dump(done))
    monkeypatch.setattr(gate, "_dump_python", _fixed_dump("legacy"))

    assert gate.check_surface(surface) == 1


@pytest.mark.parametrize(("surface", "done"), _FORM_SURFACES)
def test_generated_form_passes_when_both_sides_are_generated(
    surface: str, done: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The converted state is the one that must stay green."""
    gate = _load_script("verify_generated_form")
    monkeypatch.setattr(gate, "_dump_go", _fixed_dump(done))
    monkeypatch.setattr(gate, "_dump_python", _fixed_dump(done))

    assert gate.check_surface(surface) == 0


@pytest.mark.parametrize(("surface", "done"), _FORM_SURFACES)
def test_generated_form_fails_when_a_classifier_resolves_nothing(
    surface: str, done: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A classifier that stops finding handlers must not read as converted."""
    gate = _load_script("verify_generated_form")
    monkeypatch.setattr(gate, "_dump_go", _empty_dump)
    monkeypatch.setattr(gate, "_dump_python", _fixed_dump(done))

    with pytest.raises(SystemExit, match="covered nothing"):
        gate.check_surface(surface)


def test_generated_form_main_walks_every_surface_and_the_fixtures(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A green main means all four surfaces and the fixture check ran.

    The fixture half runs against the real proto tree, so this also pins that
    every checked-in *WriteResponse is registered in the conformance corpus.
    """
    gate = _load_script("verify_generated_form")
    seen: list[str] = []

    def go_dump(surface: str) -> dict[str, str]:
        seen.append(surface)
        return {_TOOL: str(gate._SURFACES[surface][0])}

    def py_dump(surface: str) -> dict[str, str]:
        return {_TOOL: str(gate._SURFACES[surface][0])}

    monkeypatch.setattr(gate, "_dump_go", go_dump)
    monkeypatch.setattr(gate, "_dump_python", py_dump)

    assert gate.main() == 0
    assert seen == ["write", "read", "meta", "input"]


def _messages_gate(
    monkeypatch: pytest.MonkeyPatch, go: dict[str, str], py: dict[str, str]
) -> ModuleType:
    """The message gate with both extractors replaced by fixed maps."""
    gate = _load_script("verify_messages")

    def extract(script: str, *_args: str) -> dict[str, str]:
        return go if "_go" in script else py

    monkeypatch.setattr(gate, "_extract", extract)
    return gate


def test_message_gate_fails_on_a_divergence(monkeypatch: pytest.MonkeyPatch) -> None:
    """Two languages showing different words before one mutation fails."""
    gate = _messages_gate(
        monkeypatch, {_TOOL: "This will create a widget."}, {_TOOL: "Creates a widget."}
    )

    assert gate.main() == 1


def test_message_gate_passes_when_the_text_agrees(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Matching text over a non-empty intersection is the green case."""
    same = "This will create a widget."
    gate = _messages_gate(monkeypatch, {_TOOL: same}, {_TOOL: same})

    assert gate.main() == 0


def test_message_gate_fails_when_the_extractors_share_no_tools(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An empty intersection has no divergence to find, which is not agreement.

    This is the shape that fires when a scanned tree empties out, which is how
    the gate caught its own subject moving from the hand-written trees to the
    generated ones.
    """
    gate = _messages_gate(monkeypatch, {_TOOL: "This will create a widget."}, {})

    with pytest.raises(SystemExit, match="covered nothing"):
        gate.main()


def test_parity_todo_fails_when_a_ratchet_it_reads_is_gone(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A missing file would read as no work owed, which is the wrong answer.

    Deleting a ratchet is how a gate goes hard, so it has to come with the line
    in the report that stopped reading it.
    """
    report = _load_script("parity_todo")
    monkeypatch.setattr(report, "_CONTRACTS", tmp_path)

    with pytest.raises(SystemExit, match=r"tool-parity-baseline\.txt"):
        report.main()


def test_parity_todo_names_every_hard_gate() -> None:
    """The report has to say where the work it no longer lists went."""
    report = _load_script("parity_todo")
    named = " ".join(gate for gate, _ in report._HARD_GATES)

    for gate in (
        "generated-form",
        "behavior",
        "messages",
        "pagination",
        "route-evidence",
    ):
        assert gate in named


@pytest.mark.parametrize("name", _RETIRED_BASELINES)
def test_a_hardened_gate_keeps_no_baseline_file(name: str) -> None:
    """A file back under a hard gate's name is a finding someone recorded."""
    assert not (CONTRACTS / name).exists()
