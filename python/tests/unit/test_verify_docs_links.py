"""Offline tests for the docs link gate.

verify_docs_links.py walks internal link targets in README.md and docs/.
These tests pin the target classification (external skipped, anchor
stripped, relative resolution from the linking file) and the live tree.
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


gate = _load_script("verify_docs_links")


def test_broken_and_healthy_links_classified(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Anchors strip, externals skip, and a dead relative target reports."""
    (tmp_path / "README.md").write_text(
        "[ok](docs/real.md) [anchored](docs/real.md#section)\n"
        "[external](https://example.com/missing) [dead](docs/gone.md)\n",
        encoding="utf-8",
    )
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "real.md").write_text(
        "[up](../README.md) [dead-too](nested/nowhere.md)\n", encoding="utf-8"
    )
    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)

    assert gate.broken_links() == [
        "README.md: docs/gone.md",
        "docs/real.md: nested/nowhere.md",
    ]


def test_live_docs_have_no_dead_internal_links() -> None:
    """The gate itself as a test: the shipped docs resolve everywhere."""
    assert gate.broken_links() == []
