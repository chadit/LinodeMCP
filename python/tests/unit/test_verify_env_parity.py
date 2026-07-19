"""Offline tests for the env-var parity gate.

verify_env_parity.py pins every language's env-read surface to
docs/contracts/env-vars.txt. These tests cover the extraction hazards that
matter (wrapped calls, test-file and genpb exclusion), each failure
direction, and the live surface itself.
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


gate = _load_script("verify_env_parity")


def test_go_extraction_handles_wrapped_calls_and_skips_tests(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A lookup wrapped across lines still counts; _test.go never does."""
    (tmp_path / "config.go").write_text(
        'v := os.Getenv(\n    "LINODEMCP_WRAPPED",\n)\n'
        'w := os.Getenv("LINODEMCP_PLAIN")\n',
        encoding="utf-8",
    )
    (tmp_path / "config_test.go").write_text(
        'os.Getenv("LINODEMCP_TEST_ONLY")\n', encoding="utf-8"
    )
    monkeypatch.setattr(gate, "_GO_ROOTS", (tmp_path,))

    assert gate.go_env_reads() == {"LINODEMCP_WRAPPED", "LINODEMCP_PLAIN"}


def test_python_extraction_covers_environ_forms_and_skips_genpb(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    (tmp_path / "config.py").write_text(
        'a = os.getenv(\n    "LINODEMCP_WRAPPED", "default"\n)\n'
        'b = os.environ.get("LINODEMCP_GET")\n'
        'c = os.environ["LINODEMCP_INDEX"]\n',
        encoding="utf-8",
    )
    genpb = tmp_path / "genpb"
    genpb.mkdir()
    (genpb / "gen.py").write_text(
        'os.getenv("LINODEMCP_GENERATED")\n', encoding="utf-8"
    )
    monkeypatch.setattr(gate, "_PYTHON_ROOT", tmp_path)

    assert gate.python_env_reads() == {
        "LINODEMCP_WRAPPED",
        "LINODEMCP_GET",
        "LINODEMCP_INDEX",
    }


def _no_reads() -> set[str]:
    return set()


def test_violations_report_both_directions_per_language(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An uncontracted read and an unimplemented contract entry both fire."""
    monkeypatch.setattr(gate, "contract_vars", lambda: {"LINODEMCP_SHARED"})
    monkeypatch.setattr(
        gate, "go_env_reads", lambda: {"LINODEMCP_SHARED", "LINODEMCP_GO_ONLY"}
    )
    monkeypatch.setattr(gate, "python_env_reads", _no_reads)

    violations = gate.env_violations()

    assert violations["go"] == (["LINODEMCP_GO_ONLY"], [])
    assert violations["python"] == ([], ["LINODEMCP_SHARED"])


def test_live_surface_matches_contract_in_every_language() -> None:
    """The gate itself as a test: both languages read exactly the contract.

    This is what keeps a one-sided env override (the way the observability
    overrides once drifted Go-only) from landing again.
    """
    assert gate.env_violations() == {}
