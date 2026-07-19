"""Offline tests for the metrics-surface parity gate.

verify_metrics_surface.py diffs instrument names and record-call attribute
keys between the two languages' observability sources. These tests pin the
extraction shapes, each failure direction, the extraction-went-blind guard,
and the live surface.
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


gate = _load_script("verify_metrics_surface")


def test_go_extraction_reads_instruments_and_attributes(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    metrics_go = tmp_path / "metrics.go"
    metrics_go.write_text(
        'meter.Int64Counter(\n    "linodemcp.requests.total",\n)\n'
        "attrs := []attribute.KeyValue{\n"
        '    attribute.String("tool", tool),\n'
        '    attribute.Int("status_code", code),\n'
        "}\n",
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_GO_METRICS", metrics_go)

    instruments, attributes = gate.go_metrics_surface()

    assert instruments == {"linodemcp.requests.total"}
    assert attributes == {"tool", "status_code"}


def test_python_extraction_scopes_attributes_to_record_calls(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Only add/record dict literals count; other dicts in the module do not."""
    module = tmp_path / "__init__.py"
    module.write_text(
        'meter.create_counter(\n    "linodemcp.requests.total",\n)\n'
        'level_map = {"debug": 10, "info": 20}\n'
        "self._requests_total.add(\n"
        '    1, {"tool": tool, "status": status}\n'
        ")\n"
        "self._request_duration.record(\n"
        '    seconds, {"method": "execute"}\n'
        ")\n",
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_PY_OBSERVABILITY", module)

    instruments, attributes = gate.python_metrics_surface()

    assert instruments == {"linodemcp.requests.total"}
    assert attributes == {"tool", "status", "method"}


def test_violations_name_the_one_sided_language(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        gate,
        "go_metrics_surface",
        lambda: ({"linodemcp.requests.total", "linodemcp.go.only"}, {"tool"}),
    )
    monkeypatch.setattr(
        gate,
        "python_metrics_surface",
        lambda: ({"linodemcp.requests.total"}, {"tool", "status"}),
    )

    problems = gate.metrics_violations()

    assert "instrument linodemcp.go.only exists in go only" in problems
    assert "attribute key status exists in python only" in problems


def _blind_go_surface() -> tuple[set[str], set[str]]:
    return set(), {"tool"}


def test_blind_extraction_is_a_failure_not_a_pass(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(gate, "go_metrics_surface", _blind_go_surface)
    monkeypatch.setattr(
        gate,
        "python_metrics_surface",
        lambda: ({"linodemcp.requests.total"}, {"tool"}),
    )

    problems = gate.metrics_violations()

    assert any("extraction found nothing for go" in p for p in problems)


def test_live_metrics_surfaces_match() -> None:
    """The gate itself as a test: one telemetry surface across languages."""
    assert gate.metrics_violations() == []
