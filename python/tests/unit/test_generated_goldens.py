"""Offline tests for the generated-output byte-identity harness.

scripts/generated_goldens.py freezes a sha256 per generated file and reports
per-file drift against that manifest. The single-emitter refactors have no
acceptance other than this comparison, so a harness that reported "no drift"
over an empty or mis-rooted cohort would pass every stage while checking
nothing. These tests build throwaway trees and pin what it reports.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from types import ModuleType

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


goldens = _load_script("generated_goldens")


def _tree(root: Path) -> None:
    """A stand-in for the emitted cohorts: one tree, one single-file cohort."""
    emitted = root / "emitted"
    (emitted / "nested").mkdir(parents=True)
    (emitted / "one.gen.go").write_text("alpha\n", encoding="utf-8")
    (emitted / "nested" / "two.gen.go").write_text("beta\n", encoding="utf-8")
    (root / "manifest.txt").write_text("tool\n", encoding="utf-8")


COHORTS = ("emitted", "manifest.txt")


def test_build_manifest_covers_tree_and_single_file_cohorts(tmp_path: Path) -> None:
    _tree(tmp_path)

    entries = goldens.build_manifest(tmp_path, COHORTS)

    assert sorted(entries) == [
        "emitted/nested/two.gen.go",
        "emitted/one.gen.go",
        "manifest.txt",
    ]


def test_build_manifest_skips_interpreter_caches(tmp_path: Path) -> None:
    """__pycache__ sits inside the Python tool tree and is not emitted output."""
    _tree(tmp_path)
    cache = tmp_path / "emitted" / "__pycache__"
    cache.mkdir()
    (cache / "one.cpython-314.pyc").write_bytes(b"\x00compiled")

    assert "emitted/__pycache__/one.cpython-314.pyc" not in goldens.build_manifest(
        tmp_path, COHORTS
    )


def test_build_manifest_reports_nothing_for_a_cohort_that_does_not_exist(
    tmp_path: Path,
) -> None:
    assert goldens.build_manifest(tmp_path, ("gone",)) == {}


def test_manifest_round_trips_through_its_stored_form(tmp_path: Path) -> None:
    _tree(tmp_path)
    entries = goldens.build_manifest(tmp_path, COHORTS)

    assert goldens.parse_manifest(goldens.format_manifest(entries)) == entries


def test_format_manifest_is_stable_across_freezes(tmp_path: Path) -> None:
    """Two freezes of one tree agree byte for byte, so a manifest diff means drift."""
    _tree(tmp_path)

    first = goldens.format_manifest(goldens.build_manifest(tmp_path, COHORTS))
    second = goldens.format_manifest(goldens.build_manifest(tmp_path, COHORTS))

    assert first == second


def test_parse_manifest_refuses_a_line_that_is_not_hash_and_path() -> None:
    with pytest.raises(goldens.ManifestError, match="line 2"):
        goldens.parse_manifest("# header\nnot-a-manifest-line\n")


def test_drift_names_every_added_removed_and_changed_file() -> None:
    stored = {"kept": "aa", "gone": "bb", "edited": "cc"}
    current = {"kept": "aa", "edited": "dd", "fresh": "ee"}

    assert goldens.drift(stored, current) == {
        "added": ["fresh"],
        "removed": ["gone"],
        "changed": ["edited"],
    }


def test_compare_passes_on_an_untouched_tree_and_fails_on_one_changed_byte(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    _tree(tmp_path)
    manifest = tmp_path / "frozen.sha256"
    goldens.freeze(tmp_path, manifest, COHORTS)

    assert goldens.compare(tmp_path, manifest, COHORTS) == 0

    (tmp_path / "emitted" / "one.gen.go").write_text("alphb\n", encoding="utf-8")

    assert goldens.compare(tmp_path, manifest, COHORTS) == 1
    assert "changed: emitted/one.gen.go" in capsys.readouterr().err


def test_compare_fails_when_the_manifest_is_missing(tmp_path: Path) -> None:
    assert goldens.compare(tmp_path, tmp_path / "never-frozen.sha256", COHORTS) == 1


def test_freeze_and_compare_refuse_a_root_holding_no_generated_files(
    tmp_path: Path,
) -> None:
    """Zero files is never a real answer here; it means generation never ran."""
    _tree(tmp_path)
    manifest = tmp_path / "frozen.sha256"
    goldens.freeze(tmp_path, manifest, COHORTS)
    empty = tmp_path / "empty"
    empty.mkdir()

    with pytest.raises(goldens.ManifestError, match="no generated files"):
        goldens.freeze(empty, manifest, COHORTS)

    with pytest.raises(goldens.ManifestError, match="no generated files"):
        goldens.compare(empty, manifest, COHORTS)


def test_every_declared_cohort_resolves_in_this_checkout() -> None:
    """A cohort pointing at nothing would report zero drift over zero files."""
    missing = [c for c in goldens.COHORTS if not (REPO_ROOT / c).exists()]

    assert missing == []
