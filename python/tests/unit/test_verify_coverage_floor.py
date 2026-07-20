"""Offline tests for the coverage-floor gate.

verify_coverage_floor.py pins each registered language's total unit-test
coverage to the floors contract. These tests pin the contract parsing, the
Go profile arithmetic (incl. the generated-code exclusions), each failure
direction, and the live contract-vs-registry-vs-pyproject agreement.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

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


gate = _load_script("verify_coverage_floor")

_PROFILE_TEXT = (
    "mode: atomic\n"
    "github.com/chadit/LinodeMCP/go/internal/foo/bar.go:10.2,12.16 2 1\n"
    "github.com/chadit/LinodeMCP/go/internal/foo/bar.go:14.2,15.10 2 0\n"
    "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1/x.pb.go"
    ":5.1,900.2 400 0\n"
    "github.com/chadit/LinodeMCP/go/cmd/linodemcp/main.go:8.1,20.2 6 0\n"
)


def _write_go_fixtures(tmp_path: Path) -> tuple[Path, Path]:
    profile = tmp_path / "coverage.out"
    profile.write_text(_PROFILE_TEXT, encoding="utf-8")
    go_mod = tmp_path / "go.mod"
    go_mod.write_text(
        "module github.com/chadit/LinodeMCP/go\n\ngo 1.26\n", encoding="utf-8"
    )
    return profile, go_mod


def test_read_floors_skips_comments_and_parses_values(tmp_path: Path) -> None:
    contract = tmp_path / "coverage-floors.txt"
    contract.write_text("# header comment\n\ngo 68.0\npython 85\n", encoding="utf-8")

    assert gate.read_floors(contract) == {"go": 68.0, "python": 85.0}


def test_registered_languages_reads_first_tab_field(tmp_path: Path) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text(
        "# registry\ngo\tgo\tgo run ./cmd/parity-dump\n"
        "python\tpython\t.venv/bin/python -m linodemcp.parity_dump\n",
        encoding="utf-8",
    )

    assert gate.registered_languages(registry) == {"go", "python"}


def test_go_percent_excludes_generated_and_cmd(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """genpb's 400 dead statements and the cmd main must not drag the total."""
    profile, go_mod = _write_go_fixtures(tmp_path)
    monkeypatch.setattr(gate, "_GO_PROFILE", profile)
    monkeypatch.setattr(gate, "_GO_MOD", go_mod)

    assert gate.go_coverage_percent() == 50.0


def test_floor_violation_when_go_measures_below_floor(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    profile, go_mod = _write_go_fixtures(tmp_path)
    monkeypatch.setattr(gate, "_GO_PROFILE", profile)
    monkeypatch.setattr(gate, "_GO_MOD", go_mod)

    problems = gate._check_go(90.0)

    assert len(problems) == 1
    assert "below the contracted floor" in problems[0]


def test_missing_profile_is_a_failure_with_remedy(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(gate, "_GO_PROFILE", tmp_path / "absent.out")

    problems = gate._check_go(68.0)

    assert len(problems) == 1
    assert "make go-test" in problems[0]


def test_python_arm_flags_pyproject_contract_drift(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    pyproject = tmp_path / "pyproject.toml"
    pyproject.write_text('addopts = ["--cov-fail-under=80"]\n', encoding="utf-8")
    monkeypatch.setattr(gate, "_PYPROJECT", pyproject)

    problems = gate._check_python(85.0)

    assert len(problems) == 1
    assert "mismatch" in problems[0]


def test_registered_language_without_floor_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    contract = tmp_path / "coverage-floors.txt"
    contract.write_text("go 68.0\n", encoding="utf-8")
    registry = tmp_path / "languages.txt"
    registry.write_text("go\tgo\tdump\npython\tpython\tdump\n", encoding="utf-8")
    monkeypatch.setattr(gate, "_FLOORS", contract)
    monkeypatch.setattr(gate, "_LANGUAGES", registry)

    problems = gate.floor_violations()

    assert len(problems) == 1
    assert "python" in problems[0]
    assert "no floor" in problems[0]


def test_floor_without_enforcement_arm_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A future language needs gate support, not just a contract line."""
    contract = tmp_path / "coverage-floors.txt"
    contract.write_text("typescript 70\n", encoding="utf-8")
    registry = tmp_path / "languages.txt"
    registry.write_text("typescript\tts\tdump\n", encoding="utf-8")
    monkeypatch.setattr(gate, "_FLOORS", contract)
    monkeypatch.setattr(gate, "_LANGUAGES", registry)

    problems = gate.floor_violations()

    assert len(problems) == 1
    assert "no coverage enforcement implemented for typescript" in problems[0]


def test_live_contract_matches_registry_and_pyproject() -> None:
    """The committed contract, registry, and pyproject floor agree."""
    floors = gate.read_floors(REPO_ROOT / "docs" / "contracts" / "coverage-floors.txt")
    registry = gate.registered_languages(
        REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )

    assert set(floors) == registry
    assert gate.pyproject_fail_under() == floors["python"]
