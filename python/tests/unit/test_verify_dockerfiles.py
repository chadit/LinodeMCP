"""Offline tests for the Dockerfile gate.

verify_dockerfiles.py runs droast over the Dockerfiles git reports. These tests
pin the two halves that decide what the gate says: which paths count as a
Dockerfile, and which of droast's report lines are the errors that failed it.
The subprocess call itself is not exercised; the live tree assertion below is
what says discovery works against the real repository.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from types import SimpleNamespace
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"

DF021 = "ERROR [DF021]  Executing a remotely downloaded script without verifying it"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_dockerfiles")


def test_discovery_takes_both_dockerfile_spellings(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A plain Dockerfile and a prefixed one count; a lookalike name does not."""
    for name in ("ci/Dockerfile", "build/dev.Dockerfile", "docs/Dockerfile.md"):
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("FROM scratch\n", encoding="utf-8")
    listed = "ci/Dockerfile\nbuild/dev.Dockerfile\ndocs/Dockerfile.md\n"

    def fake_run(*_args: object, **_kwargs: object) -> SimpleNamespace:
        """Stand in for the git listing, so the test needs no repository."""
        return SimpleNamespace(stdout=listed)

    monkeypatch.setattr(gate, "_ROOT", tmp_path)
    monkeypatch.setattr(gate.subprocess, "run", fake_run)

    assert gate._dockerfiles() == ["build/dev.Dockerfile", "ci/Dockerfile"]


def test_only_errors_are_reported_and_each_carries_its_location() -> None:
    """Warnings and info lines are dropped; an error folds onto its own location."""
    report = [
        "  INFO  [DF012]  No HEALTHCHECK defined",
        "             at ci/Dockerfile",
        "  WARN  [DF001]  'golang:latest' uses an unpinned image tag",
        "             at ci/Dockerfile:11:1",
        f"  {DF021}",
        "             at ci/Dockerfile:30:5",
        "  Summary: 1 error(s), 1 warning(s), 1 info(s)",
    ]

    want = "ci/Dockerfile:30:5: [DF021] Executing a remotely downloaded script"

    assert gate._errors(report) == [f"{want} without verifying it"]


def test_a_clean_report_names_nothing() -> None:
    """Nothing at error severity means the gate has no finding to print."""
    report = [
        "  INFO  [DF022]  No EXPOSE instruction",
        "             at go/Dockerfile",
        "  - Linted 3 file(s), 1 total finding(s)",
    ]

    assert gate._errors(report) == []


def test_live_tree_carries_the_dockerfiles_the_repository_builds() -> None:
    """Discovery finds the real images, the CI mirror among them."""
    found = gate._dockerfiles()

    assert "ci/Dockerfile" in found
    assert all((REPO_ROOT / path).is_file() for path in found)
