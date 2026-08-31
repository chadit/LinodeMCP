"""SQLite audit sink.

Mirrors ``go/internal/audit/sqlite.go``. Opt-in via the audit.sqlite
config block; when enabled it runs alongside the JSONL sink behind a
MultiSink. Uses the stdlib ``sqlite3`` module (no driver-name footgun
like the Go side).

The Python Sink protocol's ``write`` takes no context, unlike the Go
Sink: Python has no context.Context, and the Go signature only carries
one to satisfy database/sql's ExecContext.
"""

from __future__ import annotations

import asyncio
import json
import logging
import sqlite3
import threading
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit.reader import unix_ns_bound

if TYPE_CHECKING:
    from linodemcp.audit.event import Event

_LOG = logging.getLogger(__name__)

# Idempotent DDL run at sink open. Matches the spec's SQLite section
# and the Go createSchema byte-for-byte (args stored as JSON text;
# ts represented only as ts_unix_ns).
_CREATE_SCHEMA = """
CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    ts_unix_ns INTEGER NOT NULL,
    tool TEXT NOT NULL,
    tool_capability TEXT NOT NULL,
    environment TEXT NOT NULL,
    profile TEXT NOT NULL,
    mode TEXT NOT NULL,
    plan_id TEXT,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL,
    result_summary TEXT,
    error TEXT,
    linodemcp_version TEXT NOT NULL,
    session_id TEXT NOT NULL,
    credential_generation INTEGER NOT NULL,
    args_json TEXT NOT NULL,
    args_redacted_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_tool ON events(tool, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_profile ON events(profile, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_credential_generation
    ON events(credential_generation, ts_unix_ns DESC);
"""

# Parameterized insert. INSERT OR IGNORE makes a duplicate event_id a
# no-op so a re-delivered event stays idempotent.
_INSERT_EVENT = """
INSERT OR IGNORE INTO events (
    event_id, ts_unix_ns, tool, tool_capability, environment, profile,
    mode, plan_id, status, latency_ms, result_summary, error,
    linodemcp_version, session_id, credential_generation,
    args_json, args_redacted_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
"""

# Milliseconds-per-second divisor for the connect timeout, which
# sqlite3 takes in seconds while the config carries milliseconds.
_MS_PER_SECOND = 1000.0

# A day in nanoseconds, the unit a retention window counts in once the
# subtraction leaves the calendar.
_NS_PER_DAY = 86_400 * 1_000_000_000

# The int64 floor a saturated nanosecond count stops at, the same floor the
# reader's query bounds clamp to. Written as a shift because it is a power of
# two: 2^63 is the magnitude of the smallest int64.
_INT64_MIN = -(1 << 63)


class SQLiteSink:
    """Write audit events to a SQLite database.

    Synchronous inserts, matching the JSONL sink; the spec's 100ms
    batching is a later speedup. Write failures are best-effort:
    they log and drop, leaving the JSONL sink as the durable record.
    """

    def __init__(self, path: str, busy_timeout_ms: int) -> None:
        """Open (creating if needed) the database and ensure the schema.

        ``check_same_thread=False`` plus an internal lock lets the sink
        be written from whichever thread the dispatcher runs on. The
        busy timeout is passed as sqlite3's connect ``timeout`` (in
        seconds), avoiding a string-built PRAGMA.
        """
        self._lock = threading.Lock()
        self._conn = sqlite3.connect(
            path,
            timeout=busy_timeout_ms / _MS_PER_SECOND,
            check_same_thread=False,
        )
        self._conn.executescript(_CREATE_SCHEMA)
        self._conn.commit()

    def write(self, event: Event) -> None:
        """Insert one event row. Marshal/insert failures log and drop."""
        try:
            args_json = json.dumps(event.args or {}, separators=(",", ":"))
            redacted_json = json.dumps(event.args_redacted or [], separators=(",", ":"))
        except (TypeError, ValueError) as exc:
            _LOG.warning("audit sqlite sink: marshal failed: %s", exc)
            return

        params = (
            event.event_id,
            event.ts_unix_ns,
            event.tool,
            event.tool_capability,
            event.environment,
            event.profile,
            event.mode,
            event.plan_id,
            event.status,
            event.latency_ms,
            event.result_summary,
            event.error,
            event.linodemcp_version,
            event.session_id,
            event.credential_generation,
            args_json,
            redacted_json,
        )

        try:
            with self._lock:
                self._conn.execute(_INSERT_EVENT, params)
                self._conn.commit()
        except sqlite3.Error as exc:
            _LOG.warning("audit sqlite sink: insert failed: %s", exc)

    def close(self) -> None:
        """Close the database connection."""
        with self._lock:
            self._conn.close()

    def sweep_retention(self, now: datetime, retention_days: int) -> int:
        """Delete events older than ``now - retention_days`` and return the
        row count removed. A ``retention_days`` of 0 or less disables
        deletion (keep forever) and returns 0 without touching the table. A
        window so wide its cutoff predates the representable nanosecond range
        keeps every row.
        """
        if retention_days <= 0:
            return 0

        cutoff_ns = _retention_cutoff_ns(now, retention_days)

        with self._lock:
            cursor = self._conn.execute(
                "DELETE FROM events WHERE ts_unix_ns < ?", (cutoff_ns,)
            )
            self._conn.commit()
            return cursor.rowcount

    async def run_retention(self, retention_days: int, interval_seconds: float) -> None:
        """Sweep once immediately, then every interval until cancelled.

        Intended to run as an asyncio task. Cancellation breaks the loop
        cleanly; sweep failures log and do not stop the loop.
        """
        _LOG.info(
            "audit sqlite retention started",
            extra={
                "retention_days": retention_days,
                "interval_seconds": interval_seconds,
            },
        )

        try:
            while True:
                self._run_retention_once(retention_days)
                await asyncio.sleep(interval_seconds)
        except asyncio.CancelledError:
            return

    def _run_retention_once(self, retention_days: int) -> None:
        """Run a single sweep, logging the row count or a failure."""
        try:
            removed = self.sweep_retention(datetime.now(UTC), retention_days)
        except sqlite3.Error as exc:
            _LOG.warning("audit sqlite retention sweep failed: %s", exc)
            return

        if removed > 0:
            _LOG.info("audit sqlite retention removed %d expired rows", removed)

    @property
    def connection(self) -> sqlite3.Connection:
        """Expose the connection for the Phase 3d/3e query tools."""
        return self._conn


def _retention_cutoff_ns(now: datetime, retention_days: int) -> int:
    """The retention cutoff as the nanosecond count a row carries, saturated
    at the int64 floor.

    The subtraction runs in nanoseconds rather than on the datetime because a
    window wide enough to walk off the calendar raises OverflowError there,
    and a count past the int64 floor is one SQLite refuses to bind. Both ways
    the sweep failed on a window that should keep every row instead: nothing
    is older than a cutoff predating the first instant a row can carry.
    """
    return max(_INT64_MIN, unix_ns_bound(now) - retention_days * _NS_PER_DAY)
