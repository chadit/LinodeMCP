"""Rotated audit-log retention sweeper tests.

Mirrors ``go/internal/audit/retention_test.go``. Tests define the
cutoff boundary, the disabled-when-zero contract, the ignore-unrelated
contract, and the missing-directory error.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import RetentionSweeper

if TYPE_CHECKING:
    from collections.abc import Callable
    from pathlib import Path


def _write_rotated_file(directory: Path, name: str) -> Path:
    """Create a rotated-log file so the sweeper has something to find."""
    path = directory / name
    path.write_text("x\n", encoding="utf-8")
    return path


def _fixed_clock(now: datetime) -> Callable[[], datetime]:
    """Return a clock pinned to one instant."""
    return lambda: now


def test_sweep_removes_expired_keeps_recent(tmp_path: Path) -> None:
    """With a 14-day window, files before the cutoff are deleted."""
    now = datetime(2026, 5, 19, 12, 0, 0, tzinfo=UTC)

    # cutoff = 2026-05-05; strictly-before is expired.
    expired_gz = _write_rotated_file(tmp_path, "audit-2026-05-04.log.gz")
    expired_plain = _write_rotated_file(tmp_path, "audit-2026-05-01.log")
    cutoff_day = _write_rotated_file(tmp_path, "audit-2026-05-05.log.gz")
    recent = _write_rotated_file(tmp_path, "audit-2026-05-18.log.gz")
    active = _write_rotated_file(tmp_path, "audit.log")

    sweeper = RetentionSweeper(str(tmp_path), 14, clock=_fixed_clock(now))
    removed = sweeper.sweep()

    assert removed == 2
    assert not expired_gz.exists(), "file before cutoff must be deleted"
    assert not expired_plain.exists(), "uncompressed file before cutoff must be deleted"
    assert cutoff_day.exists(), "file dated on the cutoff day must be kept"
    assert recent.exists(), "recent file must be kept"
    assert active.exists(), "active audit.log must never be swept"


def test_sweep_disabled_when_zero(tmp_path: Path) -> None:
    """retention_days <= 0 is a no-op; nothing is removed."""
    now = datetime(2026, 5, 19, 12, 0, 0, tzinfo=UTC)
    old = _write_rotated_file(tmp_path, "audit-2020-01-01.log.gz")

    sweeper = RetentionSweeper(str(tmp_path), 0, clock=_fixed_clock(now))
    removed = sweeper.sweep()

    assert removed == 0
    assert old.exists(), "retention=0 must keep even very old files"


def test_sweep_ignores_unrelated_files(tmp_path: Path) -> None:
    """Only audit-YYYY-MM-DD.log[.gz] files are eligible for removal."""
    now = datetime(2026, 5, 19, 12, 0, 0, tzinfo=UTC)

    keepers = [
        "audit.log",  # active file
        "audit-not-a-date.log",  # prefix but unparseable date
        "audit-2026-05-04.txt",  # right date, wrong suffix
        "README.md",  # unrelated
        "audit-2026-13-99.log.gz",  # prefix + suffix but invalid date
    ]
    paths = [_write_rotated_file(tmp_path, name) for name in keepers]

    sweeper = RetentionSweeper(str(tmp_path), 14, clock=_fixed_clock(now))
    removed = sweeper.sweep()

    assert removed == 0
    for path in paths:
        assert path.exists(), f"non-rotated file must be left alone: {path}"


def test_sweep_missing_dir_raises(tmp_path: Path) -> None:
    """A sweep against a non-existent directory raises OSError."""
    now = datetime(2026, 5, 19, 12, 0, 0, tzinfo=UTC)
    missing = tmp_path / "does-not-exist"

    sweeper = RetentionSweeper(str(missing), 14, clock=_fixed_clock(now))

    with pytest.raises(OSError, match="does-not-exist"):
        sweeper.sweep()
