"""Offline tests for the Go analyzer gate.

verify_go_analyzers.py runs gopls over the Go trees the language registry
names. These tests pin the two halves that decide what gets scanned: which
registered languages are Go trees, and which files under one count as sources.
The subprocess call itself is not exercised; the live tree assertion below is
what says the scope resolves against the real repository.
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


gate = _load_script("verify_go_analyzers")


def _registry(tmp_path: Path, body: str) -> Path:
    registry = tmp_path / "languages.txt"
    registry.write_text(body, encoding="utf-8")
    return registry


def test_only_registered_directories_holding_a_go_mod_are_scanned(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A registered language is a Go tree when its directory carries a go.mod."""
    (tmp_path / "one").mkdir()
    (tmp_path / "one" / "go.mod").write_text("module one\n", encoding="utf-8")
    (tmp_path / "two").mkdir()
    monkeypatch.setattr(gate, "_ROOT", tmp_path)
    monkeypatch.setattr(
        gate,
        "_LANGUAGES",
        _registry(
            tmp_path,
            "# a comment\n\nfirst\tone\tdump one\nsecond\ttwo\tdump two\n",
        ),
    )

    assert gate._go_trees() == [tmp_path / "one"]


def test_a_registry_naming_no_go_tree_scans_nothing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Losing the Go row leaves nothing to scan, which is not a clean run."""
    (tmp_path / "two").mkdir()
    monkeypatch.setattr(gate, "_ROOT", tmp_path)
    registry = _registry(tmp_path, "second\ttwo\tdump two\n")
    monkeypatch.setattr(gate, "_LANGUAGES", registry)

    assert gate._go_trees() == []


def test_sources_are_every_go_file_under_the_tree(tmp_path: Path) -> None:
    """Nested and generated files count; anything not .go does not."""
    (tmp_path / "nested").mkdir()
    (tmp_path / "top.go").write_text("package main\n", encoding="utf-8")
    (tmp_path / "nested" / "deep.go").write_text("package nested\n", encoding="utf-8")
    (tmp_path / "notes.md").write_text("not go\n", encoding="utf-8")

    want = [tmp_path / "nested" / "deep.go", tmp_path / "top.go"]

    assert gate._sources(tmp_path) == want


def test_live_registry_resolves_to_the_repository_go_module() -> None:
    """The real registry names a Go tree, the module that holds the emitter."""
    trees = gate._go_trees()

    assert trees
    assert all((tree / "go.mod").is_file() for tree in trees)
    assert any((tree / "cmd" / "toolgen").is_dir() for tree in trees)
