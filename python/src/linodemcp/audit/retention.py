"""Rotated audit-log retention sweeper.

Mirrors ``go/internal/audit/retention.go``. Deletes rotated
``audit-YYYY-MM-DD.log[.gz]`` files older than a cutoff. The active
``audit.log`` carries no date segment, so it never matches and is
never swept.
"""

from __future__ import annotations

import asyncio
import logging
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable
    from datetime import date

_LOG = logging.getLogger(__name__)

# Default rotated-log retention window in days. Files older than this
# are deleted. Matches the spec's Retention section and the Go
# DefaultAuditRetentionDays. A value of 0 disables deletion.
DEFAULT_AUDIT_RETENTION_DAYS = 14

# Default seconds between background sweeps after the initial pass.
# Hourly matches the SQLite retention cadence; rotated files only
# appear once per UTC day so a finer interval just rescans the set.
DEFAULT_RETENTION_SWEEP_INTERVAL_SECONDS = 3600.0

# Bracket a rotated file name of the form audit-YYYY-MM-DD.log[.gz].
_ROTATED_FILE_PREFIX = "audit-"


class RetentionSweeper:
    """Delete rotated audit logs older than a cutoff.

    Only touches files matching ``audit-YYYY-MM-DD.log[.gz]``, so the
    active ``audit.log`` is never at risk. A ``retention_days`` of 0
    (or negative) disables deletion.
    """

    def __init__(
        self,
        directory: str,
        retention_days: int,
        *,
        interval_seconds: float = DEFAULT_RETENTION_SWEEP_INTERVAL_SECONDS,
        clock: Callable[[], datetime] | None = None,
    ) -> None:
        """Build a sweeper for ``directory`` with a retention window."""
        self._dir = Path(directory)
        self._retention_days = retention_days
        self._interval = interval_seconds
        self._clock = clock if clock is not None else lambda: datetime.now(UTC)

    def sweep(self) -> int:
        """Perform one retention pass and return the count removed.

        Deletes rotated files whose embedded date is strictly older
        than ``now - retention_days`` (whole UTC days). A
        ``retention_days`` of 0 or less is a no-op returning 0. Raises
        ``OSError`` if the directory can't be listed; per-file removal
        failures are logged and skipped.
        """
        if self._retention_days <= 0:
            return 0

        cutoff = self._cutoff_day()
        removed = 0

        for entry in self._dir.iterdir():
            if not entry.is_file():
                continue

            file_day = parse_rotated_file_day(entry.name)
            if file_day is None:
                continue

            if file_day >= cutoff:
                continue

            try:
                entry.unlink()
            except OSError as exc:
                _LOG.warning(
                    "audit retention: remove failed",
                    extra={"file": entry.name, "error": str(exc)},
                )
                continue

            _LOG.info(
                "audit retention: removed expired log",
                extra={"file": entry.name},
            )
            removed += 1

        return removed

    async def run(self) -> None:
        """Sweep once immediately, then every interval until cancelled.

        Intended to run as an asyncio task. Cancellation breaks the
        loop cleanly. Sweep failures are logged and do not stop the
        loop.
        """
        _LOG.info(
            "audit retention sweeper started",
            extra={
                "dir": str(self._dir),
                "retention_days": self._retention_days,
                "interval_seconds": self._interval,
            },
        )

        try:
            while True:
                self._run_once()
                await asyncio.sleep(self._interval)
        except asyncio.CancelledError:
            return

    def _run_once(self) -> None:
        """Run a single sweep, logging a directory-level failure.

        Per-file failures are already logged inside ``sweep``.
        """
        try:
            self.sweep()
        except OSError as exc:
            _LOG.warning("audit retention sweep failed", extra={"error": str(exc)})

    def _cutoff_day(self) -> date:
        """Return the UTC day boundary; files dated before it expire."""
        today = self._clock().astimezone(UTC).date()
        return today - timedelta(days=self._retention_days)


def parse_rotated_file_day(name: str) -> date | None:
    """Extract the UTC day from a rotated file name.

    Matches ``audit-YYYY-MM-DD.log`` and ``audit-YYYY-MM-DD.log.gz``.
    Returns None for names that don't match, including the active
    ``audit.log``, so the active file is never swept.
    """
    if not name.startswith(_ROTATED_FILE_PREFIX):
        return None

    rest = name[len(_ROTATED_FILE_PREFIX) :]

    if rest.endswith(".log.gz"):
        rest = rest[: -len(".log.gz")]
    elif rest.endswith(".log"):
        rest = rest[: -len(".log")]
    else:
        return None

    try:
        return datetime.strptime(rest, "%Y-%m-%d").replace(tzinfo=UTC).date()
    except ValueError:
        return None
