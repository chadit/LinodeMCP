# Audit reports

Named queries against the audit log, defined in config and invoked by name. Reports cover repeated questions like "destructive ops on prod this week" or "errors from the lke-admin profile in the last 24h" without restating the filter every time.

The runner is the `linode_audit_report` MCP tool (capability `CapMeta`, available in every profile). Reports resolve at call time, so editing the config file takes effect on the next call. No server restart needed.

For the event schema and redaction model, see [audit-log.md](./audit-log.md); for sinks and retention, [audit-operations.md](./audit-operations.md).

## Why reports

The five MCP audit query tools (`_recent`, `_summary`, `_health`, `_export`, `_report`) cover any ad-hoc question. Reports add three things:

1. **Repeatable**: a named report runs the same filter every time. Useful for recurring activity reviews.
2. **Short to invoke**: the model calls `linode_audit_report` with `name: "daily-destroys"` rather than restating ten filter fields.
3. **Shareable**: a report definition is text in a config file. Comment it, version it, send it.

If you only ask a question once, reach for `linode_audit_recent` or `linode_audit_summary` directly. Reports are for the second-and-onward time you'd run the same filter.

## Config schema

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

## Filter grammar

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

### Scalar vs list

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

### `since_offset` precedence

A duration relative to call time is friendlier than a literal ISO timestamp. Both fields are valid; if both are set, `since_offset` wins:

```yaml
filter:
  since_offset: "24h"        # used
  since: "2026-01-01T00:00:00Z"  # ignored when since_offset is set
```

Parse failures on `since_offset` are wrapped (not swallowed) at call time so a report that ships with a typo surfaces an error on the first invocation rather than silently filtering nothing.

### Meta inclusion

Custom reports do NOT have an `include_meta` shortcut. The default at the query-tool layer (`include_meta: false`) does NOT apply to reports; report filters evaluate against the raw event stream without any tool-layer defaults.

Report authors control meta inclusion explicitly via the `capability` grammar:

| Goal | Filter expression |
| --- | --- |
| Meta only | `capability: "meta"` |
| Exclude meta | `capability_in: ["read", "write", "destroy", "admin"]` |
| Include every event including meta | Omit the `capability` field entirely |

A report that omits `capability` will pick up meta events alongside everything else, which is rarely what authors intend. The two example reports earlier in this doc both exclude meta implicitly because they set `capability` or `capability_in` to non-meta values.

## Output shapes

### `output: summary`

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

### `output: list`

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

## Example reports

### `daily-destroys`

Destructive operations across all environments in the last 24 hours, grouped by tool and environment.

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
```

Use case: morning standup. Did the AI delete anything overnight, and where?

### `prod-writes-this-week`

Write and destroy operations on production in the last 168 hours, returned as a list capped at 100 events.

```yaml
audit:
  reports:
    prod-writes-this-week:
      description: "Write/destroy ops on prod this week"
      filter:
        capability_in: ["write", "destroy"]
        environment: "prod"
        since_offset: "168h"
      output: "list"
      limit: 100
```

Use case: weekly review. Drill into the actual events, not just counts.

### `error-bursts`

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

### `hallucination-detection`

Meta-events from `linode_profile_can_run` that the model invoked but where the tool name didn't resolve. Helps spot the model inventing endpoints that don't exist.

```yaml
audit:
  reports:
    hallucination-detection:
      description: "linode_profile_can_run calls in the last 24h; cross-reference with result_summary for unregistered tools"
      filter:
        tool: "linode_profile_can_run"
        since_offset: "24h"
      output: "list"
      limit: 50
```

Use case: training-data drift detection. After running the report, walk the `result_summary` field on each event; entries showing `summary.blocked_by_reason.unregistered > 0` flag a hallucinated tool name. Repeated invented names across sessions point at training-data drift rather than a one-off mistake.

The investigative pattern is documented in [audit-log.md](./audit-log.md) under "Investigative patterns".

### `bypass-audit`

Calls that bypassed the dry-run gate (`mode: bypass_dry_run` or `mode: yolo`) in the last 7 days. Useful for security review even though these modes are operator-authorized.

```yaml
audit:
  reports:
    bypass-audit:
      description: "Calls that bypassed dry-run in the last 7 days"
      filter:
        since_offset: "168h"
      group_by: ["tool", "profile"]
      output: "summary"
```

Use case: weekly bypass review. The filter doesn't gate on `mode` directly (the grammar doesn't expose it today); the query returns all events and the analyst filters on `mode` post-hoc using `linode_audit_export` to CSV.

If `mode` filtering becomes a recurring need, extending `ReportFilter` with a `mode_in` field is a one-line addition; the runner already loads the full event.

## Invoking reports

From inside a Claude conversation:

```text
Run the daily-destroys audit report.
```

The model calls `linode_audit_report` with `name: "daily-destroys"` and renders the response.

From the command line (when the audit CLI subcommand ships):

```bash
linodemcp audit report daily-destroys
```

Until that ships, query the report results inside a Claude session, or read the JSONL log directly with `jq` (see audit-log.md's quick-start section).

## What reports cannot do today

- Filter on `mode` (`normal`, `dry_run`, `plan`, `apply`, `bypass_dry_run`, `yolo`). The grammar doesn't expose it. Workaround: `output: list` and post-filter on the returned events.
- Filter on `credential_generation`. Same reason.
- Filter on `args` content. The redactor scrubs sensitive values before write, and the grammar is field-name based, not args-search.
- Filter on free-text `error` content. The grammar doesn't expose `error`; the audit-export tool with `format: ndjson` plus `jq` is the workaround.
- Define computed fields ("count of distinct profiles"). Each row maps to one event; aggregation is fixed at `group_by` columns.

Extensions to the grammar are intentionally additive. Adding a field is a typed-struct change plus a predicate compile-step extension. No new operators (regex, NOT, OR-of-different-fields); the grammar stays small on purpose. If you need more, run SQL against the SQLite store directly.

## Validation errors

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
