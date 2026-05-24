# Audit log

LinodeMCP records every tool invocation to a structured, queryable audit log. The log answers three different questions, each from a different audience:

1. The user asking "what did the AI just do?"
2. The model in a long-running session asking "did I already check on this resource?"
3. External log aggregators (Loki, Splunk) consuming the same data for compliance and SRE workflows

The log layers on top of the existing OpenTelemetry observability. OTel handles distributed-tracing and metrics; audit is a higher-level, more structured stream focused on tool-call accountability.

For the custom-report filter grammar, see [audit-reports.md](./audit-reports.md).

## Quick start

By default, every tool call lands on disk as JSON lines:

```bash
$ tail -n 1 ~/.local/state/linodemcp/audit.log | jq '.'
{
  "ts": "2026-05-24T15:30:00.123Z",
  "ts_unix_ns": 1748100600123000000,
  "event_id": "evt_01HQXY3ZKQ8M7VRBNP4W5T2J9F",
  "tool": "linode_instance_delete",
  "tool_capability": "destroy",
  "environment": "prod",
  "profile": "compute-admin",
  "mode": "normal",
  "args": { "linode_id": 12345, "confirm": true },
  "args_redacted": [],
  "status": "success",
  "latency_ms": 384,
  "linodemcp_version": "0.1.0",
  ...
}
```

Inside a Claude conversation, the same data is queryable via five MCP tools (all `CapMeta`, available in every profile):

- `linode_audit_recent` returns recent events with optional filters
- `linode_audit_summary` counts events grouped by tool, status, profile, etc.
- `linode_audit_health` reports the audit subsystem's own state
- `linode_audit_export` dumps a filtered range to a temp file in JSON / CSV / NDJSON
- `linode_audit_report` runs a named report from config

## Event schema

One event per tool call, written when the handler returns. All fields are non-optional unless noted.

| Field | Type | Meaning |
| --- | --- | --- |
| `ts` | ISO 8601 string | UTC timestamp, microsecond precision |
| `ts_unix_ns` | int64 | Unix nanoseconds for sort/index use |
| `event_id` | string | ULID, prefixed `evt_` |
| `tool` | string | Tool name, e.g. `linode_instance_delete` |
| `tool_capability` | string | One of `read`, `write`, `destroy`, `admin`, `meta` |
| `environment` | string | Linode environment selected by the call |
| `profile` | string | Active profile at call time |
| `mode` | string | One of `normal`, `dry_run`, `plan`, `apply`, `bypass_dry_run`, `yolo` |
| `plan_id` | string or null | Present for `plan` and `apply` modes |
| `args` | object | Tool arguments, with sensitive fields scrubbed |
| `args_redacted` | array of strings | Names of args that were scrubbed |
| `status` | string | One of `success`, `error`, `refused` |
| `latency_ms` | int64 | Handler entry to handler exit, milliseconds |
| `result_summary` | string | Short human-readable summary (empty in current implementation) |
| `error` | string or null | Error message when status is `error` or `refused` |
| `linodemcp_version` | string | Binary version that wrote the event |
| `session_id` | string | Best-effort transport-connection identifier |
| `credential_generation` | int64 | Monotonic counter, increments on hot-reload of any credential field |

`session_id` deserves a note: it identifies one MCP transport connection, not a conversation. If the host reconnects mid-conversation (Claude Desktop on Windows occasionally does), a new `session_id` is issued even though the model's conversation continues. Group by conversation using the host-side transcript, not `session_id`.

`credential_generation` lets investigators correlate audit entries with which credential value was live at call time. In-flight calls keep their original generation number even if a subsequent reload bumps the live counter higher.

## What audit captures and what it does not

Two limitations that analysts and users need to know.

### No user prompts

The audit log records what the AI did, not what the user asked. The MCP protocol does not surface the user's natural-language prompt to the server, so audit events have no `user_prompt` field. If a user asks "why did the AI delete that instance?", the answer is in the model's conversation transcript (host-side), not in the audit log.

### `session_id` is not a conversation id

`session_id` covers one transport-level connection. A host reconnect mid-conversation issues a new `session_id` while the conversation continues. The host's transcript is the source of truth for conversation grouping.

### Meta events are excluded by default

Calls to `linode_audit_*` and `linode_profile_*` produce audit events with `tool_capability: meta`. The default query view excludes them so analysts looking at "what did the AI do" see Linode activity, not the AI's own bookkeeping. Pass `include_meta: true` (or `capability_in: ["meta"]` for meta-only) when you want them.

### Custom reports control meta inclusion explicitly

The meta-default-exclude shortcut does NOT apply to custom reports. Report authors control meta inclusion via the `capability` grammar:

- `capability: "meta"` for meta-only
- `capability_in: ["read", "write", "destroy", "admin"]` to exclude meta (common pattern)
- No `capability` field at all to include every event, meta included

A report that omits the `capability` field will pick up meta events alongside everything else, which is rarely what authors intend. See [audit-reports.md](./audit-reports.md) for the full grammar.

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

Failure mode: if the file is unwritable (permission, disk full), audit writes a warning to stderr with a `LINODEMCP_AUDIT_FAILED` prefix and continues serving. Tool calls do not fail because audit is unavailable. Note that on Claude Desktop the host may swallow stderr in some configurations; if audit reliability matters, also enable the SQLite sink and check `linode_audit_health` periodically.

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

### OTel exporter (placeholder)

A config block exists for `audit.otel`, but the exporter is not wired today. The shape will be:

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

## Redaction

The redactor walks the `args` map recursively and replaces sensitive values with the literal string `[REDACTED]`. The keys that were redacted appear in `args_redacted`. Match semantics: **exact field name only**. No substring, no suffix, no case-fold. A field named `cluster_root_pass` does NOT match `root_pass` and would not be redacted unless the variant is added explicitly.

The rationale for exact-match: substring rules invite false positives that obscure real activity. A field named `passkey_id` would redact under a `pass` rule even though it carries an ID, not a secret. Reviewable beats clever.

### Two tiers

| Tier | Default | Operator-controllable | Scope |
| --- | --- | --- | --- |
| Credential | always on | No | API tokens, passwords, SSH keys, kubeconfig, object-storage data, share-group token UUIDs |
| PII | on by default | Yes, via `audit.redact_pii` | Postal address, phone, tax ID |

The credential tier cannot be disabled. The PII tier can be disabled by setting `audit.redact_pii: false` (or the env override `LINODEMCP_AUDIT_REDACT_PII=false`); operators investigating account-level activity where PII identifiers help with accountability can opt out.

Credential field list:

```text
api_key, apiKey, authorized_keys, data, kubeconfig, pass, password,
password_created, private_key, root_pass, secret, service_token,
ssh_key, ssh_keys, token, token_uuid
```

PII field list (conservative, source-verified against the live tool surface):

```text
address_1, address_2, city, phone, phone_number, state, tax_id, zip
```

PII names deliberately left visible so audit reports stay readable: `email`, `first_name`, `last_name`, `company`. Operators investigating account changes need a recognizable identifier in the audit row, and login email is usually the only one they can match against an external record. If a future scope wants stricter privacy, a more aggressive flag can layer in those names without widening the conservative list in place.

Names dropped from the PII list after source review:

- `country` collides with `linode_regions_list`'s filter input where `country=us` is a non-sensitive selector; the privacy benefit of redacting a region code did not justify losing audit signal on regions calls
- `dob`, `credit_card`, `cvv`, `card_number` are not in any current tool schema; they can be added when payment-method tools land

The bare name `address` is also not redacted: every current tool that uses it (e.g. `linode_instance_ips`, `linode_networking`, `linode_nodebalancers`) means a network or IP address, not a postal address.

### Catch-net heuristic

A unit test in `go/internal/server/audit_redaction_coverage_test.go` scans every registered tool's input schema for arg names containing the substrings `pass`, `token`, `key`, `secret` (credential tier) and `tax`, `address`, `phone`, `dob`, `card`, `cvv` (PII tier). Each hit must be in the corresponding redaction list or in an explicit `knownSafe` / `knownSafePII` allowlist with a justification. The catch-net would have caught the SSL-upload tool's `private_key` arg before it leaked TLS key material; today it guards against the next such miss.

The Python side relies on the Go catch-net plus the cross-language parity test that asserts both lists carry the same names. Parity drift between Go and Python lists fails the build.

### Recursive walk

The walker descends into nested objects: `{"meta": {"api_key": "..."}}` redacts the inner `api_key` even though it is one level down. It does not descend into arrays of objects today; every sensitive arg in the current tool surface lives at the top level or inside a nested object literal, never inside an array element. Array recursion lands when a tool needs it.

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

## Query tools

All five tools have capability `CapMeta` and are available in every profile, including read-only ones. Inspecting what the assistant did should never require write access.

### `linode_audit_recent`

Returns the most recent N events, newest first.

Arguments (all optional):

- `limit` (number, default 20, max 200)
- `since` (RFC 3339 timestamp; inclusive lower bound)
- `until` (RFC 3339 timestamp; inclusive upper bound)
- `tool` (glob, e.g. `linode_instance_*`)
- `capability` (one of read/write/destroy/admin/meta)
- `status` (one of success/error/refused)
- `include_meta` (bool, default false)

Reads JSONL by default for guaranteed newest-first across the active log and rotated files.

### `linode_audit_summary`

Counts events grouped by columns over a time window.

Arguments:

- `since` (RFC 3339, optional)
- `group_by` (array of column names; allowed: tool, status, capability, profile, environment; default `[tool, status]`)
- `include_meta` (bool, default false)

Reads SQLite when enabled; falls back to JSONL scan otherwise. Useful for "how many destroys in the last 24h" questions.

### `linode_audit_health`

Reports the audit subsystem's own state. No arguments.

Returns:

- JSONL path, whether the active log exists, rotated file count, oldest rotated date, total disk bytes
- SQLite stats when enabled: event count, oldest event nanoseconds, DB byte size
- `dropped_events` count (always 0 today; both sinks are synchronous)

Call this when you suspect the audit log itself might be missing data.

### `linode_audit_export`

Dumps a filtered range to a temp file in JSON, CSV, or NDJSON. Returns the file path; the model surfaces the path to the user.

Arguments:

- `format` (required: `json`, `csv`, or `ndjson`)
- `since`, `until` (optional RFC 3339)
- `tool` (optional glob)
- `max_records` (default 10000, hard cap 100000)
- `include_meta` (bool, default false)

Bounded by `max_records` to keep large ranges from blowing memory.

### `linode_audit_report`

Runs a named report from `audit.reports` in the config. Reports are resolved at call time so editing the report file takes effect on the next call. No need to restart the server.

The report grammar is documented in [audit-reports.md](./audit-reports.md).

## Investigative patterns

### Hallucination detection

The `linode_profile_can_run` audit events carry a `result_summary` that, when the model has passed a tool name not registered in the active surface, contains `summary.blocked_by_reason.unregistered > 0`. Repeated unregistered-tool calls from the same model in a short window are a signal:

- A hallucinated tool name (the model invented an endpoint that doesn't exist)
- A reference to a tool removed in a prior release (the model's training data is stale)
- A typo in tool selection (less common; usually one-off)

Pair this with `linode_audit_summary` over a 24-hour window grouped by tool to spot the same hallucinated name across sessions. The same fake name repeating across sessions points at training-data drift or a host-config issue (the host might be advertising tools it shouldn't); a one-off is usually just a mistake.

A worked example query lives in [audit-reports.md](./audit-reports.md) under "Hallucination detection".

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

- JSONL write failure: warning to stderr with prefix `LINODEMCP_AUDIT_FAILED`, tool call continues, next write retries the same file
- SQLite write failure: warning logged, JSONL remains the durable record, next write retries
- Both sinks fail: the event is lost, but the tool call still succeeds

Audit is observability, not gatekeeping. Refusal and dry-run gating live elsewhere (profiles, dry-run spec).
