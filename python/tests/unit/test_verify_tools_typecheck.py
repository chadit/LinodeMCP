"""Offline tests for the tool-project type-check gate.

verify_tools_typecheck.py runs mypy over each project under tools/ at the Python
target that project declares. These tests pin the two halves that decide whether
the gate measured anything: which directories count as projects, and which
target each one is checked against. The mypy call itself is not exercised; the
live tree assertion below is what says discovery and version reading work
against the real repository.
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


gate = _load_script("verify_tools_typecheck")


def _project(root: Path, name: str, requires: str | None) -> Path:
    """Write one tool project, with or without a declared Python floor."""
    project = root / name
    project.mkdir(parents=True)
    body = '[project]\nname = "x"\n'
    if requires is not None:
        body += f'requires-python = "{requires}"\n'
    (project / "pyproject.toml").write_text(body, encoding="utf-8")
    return project


def test_a_directory_counts_only_when_it_declares_a_project(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The pyproject is what makes a directory a project, not the directory."""
    _project(tmp_path, "declared", ">=3.14.6")
    (tmp_path / "loose").mkdir()
    (tmp_path / "loose" / "stray.py").write_text("x = 1\n", encoding="utf-8")
    monkeypatch.setattr(gate, "_TOOLS", tmp_path)

    assert gate._projects() == [tmp_path / "declared"]


def test_a_missing_tools_tree_finds_no_project(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A renamed tools/ finds nothing, which the gate must not read as clean."""
    monkeypatch.setattr(gate, "_TOOLS", tmp_path / "gone")

    assert gate._projects() == []


def test_the_target_comes_from_the_declared_floor(tmp_path: Path) -> None:
    """A patch-level floor still yields the X.Y target mypy takes."""
    project = _project(tmp_path, "declared", ">=3.14.6")

    assert gate._target_version(project) == "3.14"


def test_a_project_declaring_no_floor_has_no_target(tmp_path: Path) -> None:
    """No requires-python means no version to check against, and the gate says so."""
    project = _project(tmp_path, "undeclared", None)

    assert gate._target_version(project) is None


def test_live_tools_tree_declares_a_project_above_the_shipped_target() -> None:
    """The real comparator is checked at 3.14, not the package's 3.13.

    This is the drift the gate exists to prevent: mypy aimed at 3.13 stops at
    the comparator's unparenthesized except-tuple and checks nothing in it.
    """
    projects = gate._projects()

    assert projects
    targets = {gate._target_version(project) for project in projects}
    assert targets == {"3.14"}
