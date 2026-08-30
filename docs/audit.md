# Audit

LinodeMCP records every tool invocation to a structured, queryable audit log. The log answers three different questions, each from a different audience:

1. The user asking "what did the AI just do?"
2. The model in a long-running session asking "did I already check on this resource?"
3. External log aggregators (Loki, Splunk) consuming the same data for compliance and SRE workflows

The log layers on top of the existing OpenTelemetry observability. OTel handles distributed-tracing and metrics; audit is a higher-level, more structured stream focused on tool-call accountability.

This page covers the whole subsystem: what the log records, how to query it, the sinks and retention behind it, and the named-report grammar.

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

Inside a Claude conversation, the same data is queryable via five MCP tools (all `CapMeta`, available in every profile); see [Query tools](#query-tools).

The same five queries work from the shell, no MCP client needed. The `audit`
CLI verb wraps each tool one-to-one and drives it through the same dispatch,
so the read is itself audited and profile-checked:

```bash
linodemcp audit recent [--tool GLOB] [--since TS] [--limit N] [--include-meta]
linodemcp audit summary [--since TS]
linodemcp audit health
linodemcp audit export --format json|csv|ndjson [--tool GLOB] [--since TS]
linodemcp audit report NAME
```

The flags map straight onto the tool arguments: `--tool` is a tool-name glob,
`--since` an RFC 3339 timestamp, `--limit` a result cap. Meta events (the
audit and profile tools' own calls) are excluded by default; `--include-meta`
widens the view, same as the `include_meta` tool argument. `report` takes one
argument, the name of a report defined under `audit.reports`
(see [Reports](#reports)).

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

`credential_generation` lets investigators correlate audit entries with which credential value was live at call time. In-flight calls keep their original generation number even if a subsequent reload bumps the live counter higher.

## What audit captures and what it does not

### No user prompts

The audit log records what the AI did, not what the user asked. The MCP protocol does not surface the user's natural-language prompt to the server, so audit events have no `user_prompt` field. If a user asks "why did the AI delete that instance?", the answer is in the model's conversation transcript (host-side), not in the audit log.

### `session_id` is not a conversation id

`session_id` covers one transport-level connection. A host reconnect mid-conversation issues a new `session_id` while the conversation continues (Claude Desktop on Windows occasionally does this). The host's transcript is the source of truth for conversation grouping.

### Meta events are excluded by default

Calls to `linode_audit_*` and `linode_profile_*` produce audit events with `tool_capability: meta`. The default query view excludes them so analysts looking at "what did the AI do" see Linode activity, not the AI's own bookkeeping. Pass `include_meta: true` (or `capability_in: ["meta"]` for meta-only) when you want them.

Custom reports have no `include_meta` shortcut; report filters evaluate against the raw event stream without any tool-layer defaults. Report authors control meta inclusion via the `capability` grammar:

| Goal | Filter expression |
| --- | --- |
| Meta only | `capability: "meta"` |
| Exclude meta | `capability_in: ["read", "write", "destroy", "admin"]` |
| Include every event including meta | Omit the `capability` field entirely |

A report that omits the `capability` field will pick up meta events alongside everything else, which is rarely what authors intend.

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

- `country` collides with `linode_region_list`'s filter input where `country=us` is a non-sensitive selector; the privacy benefit of redacting a region code did not justify losing audit signal on regions calls
- `dob`, `credit_card`, `cvv`, `card_number` are not in any current tool schema; they can be added when payment-method tools land

The bare name `address` is also not redacted: every current tool that uses it (the `linode_instance_ip_*`, `linode_networking_*`, and `linode_nodebalancer_*` families) means a network or IP address, not a postal address.

### Catch-net heuristic

A unit test in `go/internal/server/audit_redaction_coverage_test.go` scans every registered tool's input schema for arg names containing the substrings `pass`, `token`, `key`, `secret` (credential tier) and `tax`, `address`, `phone`, `dob`, `card`, `cvv` (PII tier). Each hit must be in the corresponding redaction list or in an explicit `knownSafe` / `knownSafePII` allowlist with a justification. The catch-net would have caught the SSL-upload tool's `private_key` arg before it leaked TLS key material; today it guards against the next such miss.

The Python side relies on the Go catch-net plus the cross-language parity test that asserts both lists carry the same names. Parity drift between Go and Python lists fails the build.

### Recursive walk

The walker descends into nested objects: `{"meta": {"api_key": "..."}}` redacts the inner `api_key` even though it is one level down. It does not descend into arrays of objects today; every sensitive arg in the current tool surface lives at the top level or inside a nested object literal, never inside an array element. Array recursion lands when a tool needs it.

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

The report grammar is documented in [Reports](#reports) below.

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

When enabled, events dual-write to both JSONL and SQLite. JSONL is the durable record; if a SQLite insert fails, the audit query tools fall back to scanning JSONL. A database that will not open at read time falls back the same way, and the answer carries a `warnings` line naming the configured path and the reason, so a degraded read never looks like a clean one. Both languages derive the path from `audit.sqlite.path` (or `audit.db` beside the log) on every call, so they always read the same file.

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

The pure-Go driver is required for the `CGO_ENABLED=0` Windows build matrix.

## Retention

`audit.retention_days` (default 14) controls how long the JSONL and SQLite sinks keep events.

- JSONL: rotated daily, compressed, rotated files older than the cutoff are deleted
- SQLite: hourly DELETE of rows older than the cutoff

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
    # See the Reports section below for the report grammar.
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

Until you get to it, `linode_audit_health`, `linode_audit_summary`, `linode_audit_export` and `linode_audit_report` keep answering from JSONL, each one carrying a `warnings` line that names the unreadable database. That line is how you tell a degraded answer from a clean one, so treat its presence as the signal to run the three steps above.

### JSONL log gaps

A failed rotation reopens the active log rather than risk dropping events, so the event that triggered it still lands. A failure before the rename keeps the old day recorded and the next write retries the whole rotation; a gzip failure does not retry, because the rename already moved that day's data aside and the uncompressed `audit-YYYY-MM-DD.log` stays readable. When the reopen fails too, the write reports `audit: jsonl sink has no active file` through the write-error handler instead of dropping in silence. Gaps in rotated files are unusual but possible if the rotation race fails repeatedly. `linode_audit_health` surfaces the rotated file count and oldest rotated date so a missing day is visible.

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

## Reports

Named queries against the audit log, defined in config and invoked by name. Reports cover repeated questions like "destructive ops on prod this week" or "errors from the lke-admin profile in the last 24h" without restating the filter every time.

The runner is the `linode_audit_report` MCP tool (capability `CapMeta`, available in every profile). Reports resolve at call time, so editing the config file takes effect on the next call. No server restart needed.

Reports exist for the second-and-onward time you'd run the same filter: a named report runs the same query every time, is short to invoke, and is shareable text in a config file. For a one-off question, call `linode_audit_recent` or `linode_audit_summary` directly.

### Config schema

Reports live under `audit.reports` in `~/.config/linodemcp/config.yml`. Each entry is a named report:

```yaml
audit:
  reports:
    daily-destroys:
      description: "Destructive ops in the last 24h"
      filter:
        capability: "destroy"
        since_offset: "24h"
      group_by: ["tool", "environment"]
      output: "summary"

    prod-writes-this-week:
      description: "Write/destroy ops on prod this week"
      filter:
        capability_in: ["write", "destroy"]
        environment: "prod"
        since_offset: "168h"
      output: "list"
      limit: 100
```

`daily-destroys` answers the morning-standup question: did the AI delete anything overnight, and where. `prod-writes-this-week` is the weekly review: drill into the actual events, not just counts.

Top-level fields per report:

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `description` | string | `""` | Human-readable summary; shown nowhere today but useful for git diffs |
| `filter` | object | `{}` | The grammar below |
| `group_by` | array of strings | `[tool, status]` (when output is `summary`) | Columns to group counts by |
| `output` | string | `summary` | Either `summary` (counts) or `list` (events) |
| `limit` | int | 0 (no cap) | Max events for `list` output; ignored when `output: summary` |

Validation runs at config load:

- `output` must be `summary` or `list`
- `filter.since_offset` must parse as a Go-style duration (`24h`, `15m`, `168h`)
- `filter.since` and `filter.until` must parse as RFC 3339 timestamps
- A scalar field and its `*_in` list form are mutually exclusive on the same filter (you cannot set both `capability` and `capability_in`)

Config-load errors surface specific sentinels via `errors.Is`: `ErrInvalidReportOutput`, `ErrReportScalarAndList`, `ErrInvalidReportDuration`, `ErrInvalidReportTimestamp`.

### Filter grammar

The `filter` block is a typed grammar, not a free-form map. No SQL injection, no eval, no expression language. The grammar is intentionally small: if you need more, SQL the SQLite store directly.

| Field | Type | Match against | Notes |
| --- | --- | --- | --- |
| `tool` | string (glob) | event `tool` | `*` matches any chars; e.g. `linode_instance_*` |
| `capability` | string (scalar) | event `tool_capability` | One of read/write/destroy/admin/meta |
| `capability_in` | array of strings | event `tool_capability` | Any-of |
| `status` | string (scalar) | event `status` | One of success/error/refused |
| `status_in` | array of strings | event `status` | Any-of |
| `environment` | string (glob) | event `environment` | |
| `profile` | string (glob) | event `profile` | |
| `since_offset` | duration string | computed `now - offset` | Wins over `since` when both are set |
| `since` | RFC 3339 timestamp | event `ts_unix_ns` | Inclusive lower bound |
| `until` | RFC 3339 timestamp | event `ts_unix_ns` | Inclusive upper bound |

Meta inclusion is controlled through the `capability` field; see
[Meta events are excluded by default](#meta-events-are-excluded-by-default).

#### Scalar vs list

For `capability` and `status`, pick one form per report:

```yaml
# Single value (any of):
filter:
  capability: "destroy"

# Or list (any-of-many):
filter:
  capability_in: ["write", "destroy"]

# Not both: this fails validation at config load
filter:
  capability: "destroy"
  capability_in: ["write", "destroy"]  # rejected
```

#### `since_offset` precedence

A duration relative to call time is friendlier than a literal ISO timestamp. Both fields are valid; if both are set, `since_offset` wins:

```yaml
filter:
  since_offset: "24h"        # used
  since: "2026-01-01T00:00:00Z"  # ignored when since_offset is set
```

Parse failures on `since_offset` are wrapped (not swallowed) at call time so a report that ships with a typo surfaces an error on the first invocation rather than silently filtering nothing.

### Output shapes

#### `output: summary`

Returns counts per group bucket, sorted count-descending then by grouped values:

```json
{
  "name": "daily-destroys",
  "output": "summary",
  "total_events": 8,
  "rows": [
    {"groups": {"tool": "linode_instance_delete", "environment": "prod"}, "count": 4},
    {"groups": {"tool": "linode_volume_delete", "environment": "staging"}, "count": 3},
    {"groups": {"tool": "linode_domain_delete", "environment": "prod"}, "count": 1}
  ]
}
```

`group_by` defaults to `[tool, status]` when unspecified.

Unknown column names in `group_by` are a load-time error: `ErrUnknownGroupByColumn` (Go) / `UnknownGroupByColumnError` (Python). Allowed columns: `tool`, `status`, `capability`, `profile`, `environment`.

#### `output: list`

Returns the matching events directly, capped by `limit` when positive:

```json
{
  "name": "prod-writes-this-week",
  "output": "list",
  "total_events": 27,
  "events": [
    { "ts": "...", "tool": "linode_instance_update", ... },
    ...
  ]
}
```

`total_events` reflects the capped length, so a model won't get a "I returned N but said M" mismatch when `limit` truncates a larger result set.

### Example: `error-bursts`

Errors and refusals in the last 6 hours, grouped by tool and status.

```yaml
audit:
  reports:
    error-bursts:
      description: "Errors and refusals in the last 6h"
      filter:
        status_in: ["error", "refused"]
        since_offset: "6h"
      group_by: ["tool", "status"]
      output: "summary"
```

Use case: incident review. Which tools failed recently, and how often?

### Invoking reports

From inside a Claude conversation:

```text
Run the daily-destroys audit report.
```

The model calls `linode_audit_report` with `name: "daily-destroys"` and renders the response.

From the command line:

```bash
linodemcp audit report daily-destroys
```

The subcommand wraps the same tool through the same dispatch, so the output
is the report's JSON response, ready to pipe into `jq`. An unknown report
name is the tool's error and exits 1.

### What reports cannot do today

- Filter on `mode` (`normal`, `dry_run`, `plan`, `apply`, `bypass_dry_run`, `yolo`). The grammar doesn't expose it. Workaround: `output: list` and post-filter on the returned events.
- Filter on `credential_generation`. Same reason.
- Filter on `args` content. The redactor scrubs sensitive values before write, and the grammar is field-name based, not args-search.
- Filter on free-text `error` content. The grammar doesn't expose `error`; the audit-export tool with `format: ndjson` plus `jq` is the workaround.
- Define computed fields ("count of distinct profiles"). Each row maps to one event; aggregation is fixed at `group_by` columns.

Extensions to the grammar are intentionally additive. Adding a field is a typed-struct change plus a predicate compile-step extension. No new operators (regex, NOT, OR-of-different-fields); the grammar stays small on purpose. If you need more, run SQL against the SQLite store directly.

### Validation errors

Config-load errors (raised when `linodemcp` starts or hot-reloads the config; all wrap `config.ErrConfigInvalid`):

| Sentinel (Go) | Cause | Fix |
| --- | --- | --- |
| `ErrInvalidReportOutput` | `output` is not `summary` or `list` | Use one of the two |
| `ErrReportScalarAndList` | Both `capability` and `capability_in` (or `status` / `status_in`) set on the same report | Pick one form |
| `ErrInvalidReportDuration` | `since_offset` not a valid Go duration | Use `24h`, `15m`, `168h`, etc. |
| `ErrInvalidReportTimestamp` | `since` or `until` not RFC 3339 | Use `2026-05-24T00:00:00Z` style |

`errors.Is(err, config.ErrConfigInvalid)` identifies a report-config problem broadly; the specific sentinel pinpoints the field.

Call-time errors (raised when `linode_audit_report` runs):

| Sentinel / message | Cause | Fix |
| --- | --- | --- |
| `audit.ErrUnknownGroupByColumn` | `group_by` contains a column name not in the allowed list | Allowed: `tool`, `status`, `capability`, `profile`, `environment` |
| `unknown report: "<name>"` (non-sentinel string) | Requested name not in `audit.reports` | Check spelling against the config |

`ErrUnknownGroupByColumn` is an audit-package sentinel, not a config-package one; it does not wrap `ErrConfigInvalid`. The unknown-report case returns an MCP tool-result error with the literal message format above rather than a typed sentinel; if you need to distinguish it programmatically, match on the prefix `unknown report:` for now.

## Related

- [dry-run.md](./dry-run.md) and [two-stage-writes.md](./two-stage-writes.md): where the `mode` values come from
