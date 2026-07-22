"""Offline tests for the baseline growth guard.

verify_baseline_direction.py guards ratchet baselines: an ADDED entry must
carry an acceptance annotation citing a tracking-issue URL, because a
ratchet entry is a promise to come back. Two files under the same glob are
regenerated drift snapshots (api-defaults, enum-sync) whose added lines
never carry an annotation, so they are exempt, and behavior-exempt.txt may
carry a free-text reason since a permanent exemption has no follow-up to
track. These tests pin the exemptions, the URL requirement, and the guard's
flagging of real unannotated growth.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _fixed_git_show(base_text: str) -> Callable[[str, str], str]:
    """A _git_show stand-in that returns fixed base text for any rev and path."""

    def _show(_rev: str, _rel: str) -> str:
        return base_text

    return _show


def _load_script(name: str) -> ModuleType:
    # verify_baseline_direction imports _baselines, so the scripts dir has to be
    # importable before exec_module runs the module body.
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


guard = _load_script("verify_baseline_direction")


def test_snapshot_exemption_matches_sync_scripts() -> None:
    """The exempt set must name exactly the files the sync scripts regenerate.

    This gates the guard's hand-list: rename a sync BASELINE and this fails,
    so a snapshot can never be silently re-guarded or left off the exemption.
    """
    defaults = _load_script("verify_sync_defaults")
    enums = _load_script("verify_sync_enums")
    pagination = _load_script("verify_sync_pagination")
    response_shapes = _load_script("verify_sync_response_shapes")
    expected = {
        defaults.BASELINE.name,
        enums.BASELINE.name,
        pagination.BASELINE.name,
        response_shapes.BASELINE.name,
    }
    assert expected == guard._SNAPSHOT_BASELINES


def test_guarded_baselines_excludes_snapshots_and_adds_exempt_list(
    tmp_path: Path,
) -> None:
    contracts = tmp_path / "contracts"
    contracts.mkdir()
    for name in (
        "api-defaults-baseline.txt",
        "enum-sync-baseline.txt",
        "tool-parity-baseline.txt",
        "write-proto-baseline.txt",
    ):
        (contracts / name).write_text("# header\n", encoding="utf-8")

    guarded = {path.name for path in guard._guarded_baselines(contracts)}

    assert guarded == {
        "tool-parity-baseline.txt",
        "write-proto-baseline.txt",
        "behavior-exempt.txt",
    }


def _write_ratchet(tmp_path: Path, body: str) -> Path:
    path = tmp_path / "docs" / "contracts" / "tool-parity-baseline.txt"
    path.parent.mkdir(parents=True)
    path.write_text("# header\n" + body, encoding="utf-8")
    return path


def test_unannotated_growth_is_flagged(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show("# header\nold\n"))
    path = _write_ratchet(tmp_path, "old\nnew\n")

    problems = guard._check_file(path, "base")

    assert len(problems) == 1
    assert "new" in problems[0]
    assert "MISSING ANNOTATION" in problems[0]


def test_growth_annotated_with_issue_url_is_accepted(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show("# header\nold\n"))
    path = _write_ratchet(
        tmp_path,
        "old\nnew  # accepted 2026-01-01"
        " https://github.com/chadit/LinodeMCP-Issue/issues/999 catch-up\n",
    )

    assert guard._check_file(path, "base") == []


def test_ratchet_growth_with_free_text_reason_is_flagged(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A dated reason with no issue URL is a promise with no home."""
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show("# header\nold\n"))
    path = _write_ratchet(tmp_path, "old\nnew  # accepted 2026-01-01 tracking reason\n")

    problems = guard._check_file(path, "base")

    assert len(problems) == 1
    assert "new" in problems[0]
    assert "MISSING TRACKING-ISSUE URL" in problems[0]


def test_removal_is_never_flagged(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show("# header\nold\ngone\n"))
    path = _write_ratchet(tmp_path, "old\n")

    assert guard._check_file(path, "base") == []


def _write_exempt(tmp_path: Path, body: str) -> Path:
    path = tmp_path / "docs" / "contracts" / "behavior-exempt.txt"
    path.parent.mkdir(parents=True)
    path.write_text("# header\n" + body, encoding="utf-8")
    return path


_EXEMPT_BASE = "# header\nhello\tlocal greeting, no HTTP\n"


def test_new_exempt_entry_without_annotation_is_flagged(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A new behavior exemption is permanent coverage loss; it must be dated."""
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show(_EXEMPT_BASE))
    path = _write_exempt(
        tmp_path,
        "hello\tlocal greeting, no HTTP\nnew_tool\tstateful local data\n",
    )

    problems = guard._check_file(path, "base")

    assert len(problems) == 1
    assert "new_tool" in problems[0]
    assert "MISSING ANNOTATION" in problems[0]


def test_new_exempt_entry_with_annotation_is_accepted(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(guard, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(guard, "_git_show", _fixed_git_show(_EXEMPT_BASE))
    path = _write_exempt(
        tmp_path,
        "hello\tlocal greeting, no HTTP\n"
        "new_tool\tstateful local data  # accepted 2026-07-18 tracking reason\n",
    )

    assert guard._check_file(path, "base") == []
