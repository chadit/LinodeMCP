"""Offline tests for the proto lint gate.

verify_proto_lint.py runs buf lint over the modules buf.yaml declares. These
tests pin the module read, which is what the gate uses to prove it had a surface
to scan. The subprocess call itself is not exercised; the live tree assertion
below is what says the read works against the real workspace config.
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


gate = _load_script("verify_proto_lint")


def _workspace(tmp_path: Path, body: str, monkeypatch: pytest.MonkeyPatch) -> None:
    (tmp_path / "buf.yaml").write_text(body, encoding="utf-8")
    monkeypatch.setattr(gate, "_ROOT", tmp_path)


def test_every_declared_module_path_is_read(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Each module entry counts, and keys under other sections do not."""
    _workspace(
        tmp_path,
        "version: v2\n"
        "modules:\n"
        "  - path: proto\n"
        "  - path: vendor/proto\n"
        "lint:\n"
        "  use:\n"
        "    - STANDARD\n"
        "  ignore:\n"
        "    - proto/buf\n",
        monkeypatch,
    )

    assert gate._module_paths() == ["proto", "vendor/proto"]


def test_a_workspace_declaring_no_module_reads_nothing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Losing the modules key leaves nothing to lint, which is not a clean run."""
    _workspace(tmp_path, "version: v2\nlint:\n  use:\n    - STANDARD\n", monkeypatch)

    assert gate._module_paths() == []


def test_live_workspace_declares_the_contract_module() -> None:
    """The real buf.yaml names the directory the contract lives in."""
    paths = gate._module_paths()

    assert paths
    assert all((REPO_ROOT / path).is_dir() for path in paths)
    assert any(list((REPO_ROOT / path).rglob("*.proto")) for path in paths)
