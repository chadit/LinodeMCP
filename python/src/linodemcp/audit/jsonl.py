"""Rolling JSONL audit sink.

Mirrors ``go/internal/audit/jsonl.go``. Writes one JSON line per
event to ``audit.log`` and rotates the file to
``audit-YYYY-MM-DD.log.gz`` when the UTC day rolls over.
"""

from __future__ import annotations

import gzip
import logging
import shutil
import threading
from contextlib import suppress
from datetime import UTC, datetime
from pathlib import Path
from typing import IO, TYPE_CHECKING

from linodemcp.genlocal import record_audit_event

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.audit.event import Event

_LOG = logging.getLogger(__name__)

# File name for the active rolling log while it's the current day's
# file. Rotated copies use the audit-YYYY-MM-DD.log[.gz] naming.
ACTIVE_LOG_FILE_NAME = "audit.log"

# Directory mode used when the audit directory is created. Keeps the
# log readable by the owning user and group but not world-readable.
_AUDIT_DIR_MODE = 0o750


class JSONLSinkClosedError(RuntimeError):
    """Raised internally when ``write`` is called after ``close``.

    The sink does not propagate this to callers: ``write`` routes it
    to the configured error handler (default: a ``logging.warning``)
    and drops the event. Exposed so tests can assert on it.
    """


def _default_write_error_handler(error: Exception) -> None:
    """Log every write or rotate failure at WARNING.

    Audit is best-effort; a single drop should leave a breadcrumb
    without crashing the server.
    """
    _LOG.warning("audit jsonl sink: %s", error)


class JSONLSink:
    """Write audit events to a rolling JSONL file.

    Rotation happens lazily on each ``write``: if the active file was
    opened on a different UTC day than the current event, the active
    file is renamed to ``audit-YYYY-MM-DD.log``, gzipped, and a fresh
    ``audit.log`` opens for the new day.

    Writes are serialized by a lock. The sink is safe for concurrent
    use. Writes are synchronous (Phase 2a); a later phase can swap in
    a queue-fed worker thread without changing the ``Sink`` contract.
    """

    def __init__(
        self,
        directory: str,
        *,
        clock: Callable[[], datetime] | None = None,
        on_write_error: Callable[[Exception], None] | None = None,
    ) -> None:
        """Open the active ``audit.log`` under ``directory``.

        The directory is created if absent. Raises ``OSError`` if the
        directory or file can't be opened; the caller decides whether
        that's fatal or whether to fall back to a noop sink.
        """
        self._dir = Path(directory)
        self._clock = clock if clock is not None else lambda: datetime.now(UTC)
        self._on_write_error = (
            on_write_error
            if on_write_error is not None
            else _default_write_error_handler
        )
        self._lock = threading.Lock()
        self._closed = False
        self._file: IO[str] | None = None
        self._open_day = ""

        self._dir.mkdir(parents=True, exist_ok=True, mode=_AUDIT_DIR_MODE)
        self._open_active()

    @property
    def path(self) -> str:
        """Absolute path of the active ``audit.log``."""
        return str(self._dir / ACTIVE_LOG_FILE_NAME)

    def write(self, event: Event) -> None:
        """Append one JSON line for ``event``, rotating if the day rolled.

        Errors route to the configured handler; there is no error
        return because the ``Sink`` contract must not block the
        dispatcher.
        """
        with self._lock:
            if self._closed:
                self._on_write_error(
                    JSONLSinkClosedError("audit: jsonl sink is closed"),
                )
                return

            current_day = _utc_day_string(self._clock())
            if current_day != self._open_day:
                try:
                    self._rotate_locked()
                except OSError as exc:
                    # A failed rotation leaves an open file behind, so this
                    # event still lands on disk and the next write retries.
                    self._on_write_error(
                        OSError(f"audit: rotate failed: {exc}"),
                    )

            try:
                # The record's own canonical text rather than this language's
                # dict order, which is what puts every sink's line in the
                # contract's order.
                line = record_audit_event(event, "", "")
            except (TypeError, ValueError) as exc:
                self._on_write_error(
                    ValueError(f"audit: marshal event: {exc}"),
                )
                return

            if self._file is None:
                # Only reachable when a failed rotation could not reopen the
                # log either; report the gap rather than dropping in silence.
                self._on_write_error(
                    OSError("audit: jsonl sink has no active file"),
                )
                return

            try:
                self._file.write(line + "\n")
                self._file.flush()
            except OSError as exc:
                self._on_write_error(OSError(f"audit: write line: {exc}"))

    def close(self) -> None:
        """Close the active file. Idempotent.

        After close, ``write`` reports ``JSONLSinkClosedError`` via the
        error handler and drops the event.
        """
        with self._lock:
            if self._closed:
                return

            self._closed = True

            if self._file is not None:
                self._file.close()
                self._file = None

    def _open_active(self) -> None:
        """Open the active ``audit.log`` and record the day it belongs to.

        Caller holds the lock, or is the constructor before the sink
        is published.
        """
        self._open_active_file()
        self._open_day = _utc_day_string(self._clock())

    def _open_active_file(self) -> None:
        """Open or create the active ``audit.log`` for append, leaving the
        recorded open day alone, which is what a failed rotation needs so the
        next write tries again. Caller holds the lock.
        """
        path = self._dir / ACTIVE_LOG_FILE_NAME
        self._file = path.open("a", encoding="utf-8")

    def _rotate_locked(self) -> None:
        """Rotate the active file to the dated gzip form.

        Caller MUST hold the lock. The rotated file's date is the OLD
        ``open_day`` value: rotation fires because the day rolled over,
        so the file being closed holds the prior day's data.

        Every failure path reopens ``audit.log`` before the error leaves, so
        the event that triggered the rotation still reaches disk. A failure
        before the rename keeps the old day recorded, which is what makes the
        next write retry the whole rotation; a gzip failure does not, because
        the rename already moved the closed day's data aside.

        A close failure does not stop the rotation. Records are flushed per
        write, so a close failure cannot mean lost records, and Python closes
        the descriptor even when ``close`` reports a failure. Letting it out
        here would leave ``_file`` pointing at a closed handle, which the
        write that follows would hit with a ``ValueError`` no caller expects.
        """
        if self._file is None:
            self._open_active()
            return

        old_day = self._open_day
        active_path = self._dir / ACTIVE_LOG_FILE_NAME
        rotated_path = self._dir / f"audit-{old_day}.log"

        with suppress(OSError):
            self._file.close()

        self._file = None

        try:
            active_path.rename(rotated_path)
        except OSError:
            # The day's data is still in audit.log, so reopening under the day
            # it already had is what makes the next write retry the rotation.
            self._open_active_file()
            raise

        try:
            _gzip_file(rotated_path)
        except OSError:
            # The rename already moved the closed day's data aside, so the file
            # opened here belongs to the current day. Reopening under the old
            # day would send the next write to rename it over the rotated file
            # this attempt already wrote.
            self._open_active()
            raise

        self._open_active()


def _utc_day_string(moment: datetime) -> str:
    """Format ``moment`` as ``YYYY-MM-DD`` in UTC.

    Used as the rotation key (same-day-or-not) and the rotated-file
    name segment.
    """
    return moment.astimezone(UTC).strftime("%Y-%m-%d")


def _gzip_file(path: Path) -> None:
    """Compress ``path`` to ``path.gz`` and delete the original.

    On success the uncompressed source is removed. On failure the
    source is left in place so data isn't lost; the partial ``.gz`` is
    cleaned up.
    """
    gz_path = path.with_suffix(path.suffix + ".gz")

    try:
        with path.open("rb") as source, gzip.open(gz_path, "wb") as dest:
            shutil.copyfileobj(source, dest)
    except OSError:
        gz_path.unlink(missing_ok=True)
        raise

    path.unlink()
