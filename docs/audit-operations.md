# Audit operations

Operator-facing reference for running the audit subsystem: where events land
(sinks), how long they live (retention), the config block, and what happens
when writes fail. For what the log records and how to query it (event schema,
redaction, the query tools), see [audit-log.md](./audit-log.md).

## Sinks

The capture middleware fans out each event to whichever sinks are enabled.

### JSONL file (always on)

Writer: appending file writer with daily rotation.

Path resolution:

- System service (UID < 1000 or systemd-managed): `/var/log/linodemcp/audit.log`
- Otherwise: `$XDG_STATE_HOME/linodemcp/audit.log` (default `~/.local/state/linodemcp/audit.log`)

Rotation:

- File rotates daily at UTC midnight
- Rotated files named `audit-YYYY-MM-DD.log`
- Rotated files compressed with gzip
- Retention purges files older than `audit.retention_days` (default 14)

Format: one JSON object per line, no trailing comma, newline-terminated. Compatible with Promtail's `pipeline_stages` → `json` parser.

Failure mode: if the sink cannot be set up (permission, disk full), the server logs `audit JSONL sink unavailable; continuing without audit` to stderr, in the same words in every language and entry point, and keeps serving. Tool calls do not fail because audit is unavailable. Note that on Claude Desktop the host may swallow stderr in some configurations; if audit reliability matters, also enable the SQLite sink and check `linode_audit_health` periodically.

### SQLite (optional)

Opt-in via config:

```yaml
audit:
  sqlite:
    enabled: true
    path: ""           # default: audit.db alongside the JSONL log
    busy_timeout_ms: 5000
```

When enabled, events dual-write to both JSONL and SQLite. JSONL is the durable record; if a SQLite insert fails, the audit query tools fall back to scanning JSONL.

Schema:

```sql
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
CREATE INDEX IF NOT EXISTS idx_events_credential_generation ON events(credential_generation, ts_unix_ns DESC);
```

Retention: an hourly `DELETE FROM events WHERE ts_unix_ns < ?` with cutoff `now - retention_days`.

The audit query tools prefer SQLite when available for indexed reads. `linode_audit_summary`, `linode_audit_health`, and `linode_audit_export` all benefit; `linode_audit_recent` reads JSONL either way for newest-first semantics.

### SQLite driver-name footgun (Go integrators)

The Go side uses `modernc.org/sqlite` (pure-Go, no CGO). It registers as `"sqlite"`, NOT `"sqlite3"`. Most online SQLite-for-Go examples target `mattn/go-sqlite3` which registers as `"sqlite3"`; pasting one of those in produces `sql: unknown driver "sqlite3" (forgotten import?)` at runtime. The correct usage:

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"  // registers driver name "sqlite"
)

db, err := sql.Open("sqlite", path)  // "sqlite", not "sqlite3"
```

The Python implementation uses stdlib `sqlite3` and has no equivalent gotcha.

The pure-Go driver is required for the `CGO_ENABLED=0` Windows build matrix; the 100ms batching strategy keeps overhead within the documented 5ms p99 budget.

### OTel exporter (planned, not yet implemented)

There is no `audit.otel` config block today; neither implementation reads one. When the exporter lands, the planned shape is:

```yaml
audit:
  otel:
    enabled: false
    exporter: "otlp"
    endpoint: "localhost:4318"
```

When wired, each event will also emit as a span event on the existing OTel pipeline. The infrastructure already exists (`go/internal/observability/`); the audit-side wiring lands in a follow-up.

## Retention

`audit.retention_days` (default 14) controls how long the JSONL and SQLite sinks keep events.

- JSONL: rotated daily, compressed, rotated files older than the cutoff are deleted
- SQLite: hourly DELETE of rows older than the cutoff
- OTel (when wired): handled by the downstream collector

`audit.retention_days: 0` disables deletion (keep forever). The server logs a loud warning at startup when retention is disabled, so the choice is visible.

Negative values are rejected at config-load time with `audit.retention_days cannot be negative`.

## Configuration

The full `audit` block in `~/.config/linodemcp/config.yml`:

```yaml
audit:
  retention_days: 14         # 0 = never delete (loud warning at startup)
  redact_pii: true           # false = log PII in cleartext
  sqlite:
    enabled: false
    path: ""                 # default: audit.db alongside the JSONL log
    busy_timeout_ms: 5000
  reports:
    # See docs/audit-reports.md for the report grammar.
```

Environment overrides (take precedence over file values):

| Variable | Effect |
| --- | --- |
| `LINODEMCP_AUDIT_RETENTION_DAYS` | Override `retention_days` |
| `LINODEMCP_AUDIT_REDACT_PII` | Override `redact_pii` (`true`/`1` or `false`/`0`) |
| `LINODEMCP_AUDIT_SQLITE_ENABLED` | Override `sqlite.enabled` |
| `LINODEMCP_AUDIT_SQLITE_PATH` | Override `sqlite.path` |
| `LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS` | Override `sqlite.busy_timeout_ms` |

## Recovery

### SQLite corruption

SQLite is durable but not perfect. If `audit.db` becomes unreadable:

1. Stop the LinodeMCP server
2. Delete `audit.db`
3. Restart

The JSONL log is the durable record; the SQLite sink rebuilds from new events going forward. Past events that landed only in the corrupted DB stay only in JSONL. Reconstructing the SQLite table from JSONL is not a built-in operation today.

### JSONL log gaps

Rotation failures leave the active log in place (the sink keeps writing rather than risk dropping events). On the next rotation attempt the sink retries; gaps in rotated files are unusual but possible if the rotation race fails repeatedly. `linode_audit_health` surfaces the rotated file count and oldest rotated date so a missing day is visible.

## Performance

Target: audit overhead adds less than 5ms p99 to tool-call latency.

Strategy as shipped:

- JSONL writes are synchronous (no buffer; one write syscall per event)
- SQLite writes are synchronous, one INSERT per event
- Both sinks honor the request context, with `WithoutCancel` semantics so an audit write still lands after the request that produced it is canceled

If sustained load measurably exceeds the budget, the design accommodates an async buffer with bounded channels and drop-counter accounting via `linode_audit_health`; the implementation hooks are in place but unused.

## Failure mode

Audit never blocks tool calls. If a sink write fails:

- JSONL sink unavailable at startup: `audit JSONL sink unavailable; continuing without audit` on stderr, tool calls continue
- SQLite sink unavailable at startup: `audit SQLite sink unavailable; continuing with JSONL only` on stderr, JSONL remains the durable record
- Write failure after startup: warning logged, the next write retries the same file
- Both sinks fail: the event is lost, but the tool call still succeeds

Audit is observability, not gatekeeping. Refusal and dry-run gating live elsewhere ([profiles](./profiles.md), [dry-run](./dry-run.md)).
