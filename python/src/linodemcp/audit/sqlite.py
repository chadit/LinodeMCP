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

import json
import logging
import sqlite3
import threading
from typing import TYPE_CHECKING

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


class SQLiteSink:
    """Write audit events to a SQLite database.

    Synchronous inserts, matching the JSONL sink; the spec's 100ms
    batching is a later optimization. Write failures are best-effort:
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
            event.tool_capability.value,
            event.environment,
            event.profile,
            event.mode.value,
            event.plan_id,
            event.status.value,
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

    @property
    def connection(self) -> sqlite3.Connection:
        """Expose the connection for the Phase 3c sweeper and 3d/3e tools."""
        return self._conn
