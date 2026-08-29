# Audit queries per host

How to query the LinodeMCP [audit log](../audit.md) from inside each MCP host: four slash commands for Claude Code, and natural-language patterns plus CLI verbs for Claude Desktop, which has no user-defined slash command system.

Assumes the server is already registered with your host per the [directory README](./README.md). The audit tools have capability `CapMeta`, so they are available in every profile (including `default`); you do not need a write-tier profile to query the audit log.

## Confirm the audit tools are available

In a session with your host:

```text
List the LinodeMCP tools available to you whose name starts with linode_audit_.
```

You should see at least `linode_audit_recent`, `linode_audit_summary`, `linode_audit_health`, `linode_audit_export`, and `linode_audit_report`. If they are missing, the active profile has somehow filtered them out (which would require explicit denial). Check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## Claude Code: `/audit` slash commands

Claude Code slash commands are markdown files under `~/.claude/commands/` (user-level) or `.claude/commands/` (project-level). Copy each of the blocks below into a separate file under one of those directories.

### `/audit`: recent activity

`~/.claude/commands/audit.md`:

````markdown
---
description: Show recent LinodeMCP audit events (what tools were called, with what outcome).
---

# `/audit`

Call the `linode_audit_recent` MCP tool with default arguments and present the
result.

- If `$ARGUMENTS` is empty, pass no filters; the tool returns the 20 most
  recent events.
- If `$ARGUMENTS` is a positive integer, pass it as the `limit` argument
  (capped at 200 by the tool).
- Otherwise, pass `$ARGUMENTS` through verbatim as the `tool` glob filter
  (e.g. `linode_instance_*`).

Render the response as a compact table: timestamp (local time), tool,
capability, status, profile, environment, result_summary. Sort newest first
(the tool already returns newest-first; preserve that order).

Do not page beyond the returned `count`. If the user wants more, suggest
they raise `limit` or set a `since` window.
````

### `/audit-errors`: only failures and refusals

`~/.claude/commands/audit-errors.md`:

````markdown
---
description: Show recent LinodeMCP audit events with status=error or refused.
---

# `/audit-errors`

Call `linode_audit_recent` with `status: "error"` and present the result. If
`$ARGUMENTS` contains the literal word `refused`, call it twice (once with
`status: "error"`, once with `status: "refused"`) and merge the results
newest-first; the underlying tool accepts only one status value per call.

If `$ARGUMENTS` is a positive integer, pass it as `limit` for each call.
Otherwise default to limit 20.

For each event, show timestamp, tool, status, and the `error` field. If
`error` is unusually long, truncate to ~120 characters and tell the user
to run `/audit` with a `since` filter for the full record.
````

### `/audit-by`: filter by tool name

`~/.claude/commands/audit-by.md`:

````markdown
---
description: Show recent LinodeMCP audit events filtered by tool name (glob).
---

# `/audit-by`

Read `$ARGUMENTS`. The first word selects the filter dimension; the rest is
the filter value. Supported dimensions today:

- `tool <glob>`: pass `$2` as the `tool` argument to `linode_audit_recent`
  (e.g. `tool linode_instance_*`, `tool linode_database_*`)

Unknown dimensions: tell the user the supported list and stop. Do not guess
at a different MCP argument name; the audit recent tool only accepts the
filters documented in its schema.

Pass through `limit` if the user appended one (e.g. `/audit-by tool
linode_lke_* 50`). Otherwise default to 20.

Render the result the same way as `/audit`.
````

### `/audit-summary`: counts in the last 24 hours

`~/.claude/commands/audit-summary.md`:

````markdown
---
description: Summarize LinodeMCP audit activity in the last 24h as counts per tool and status.
---

# `/audit-summary`

Compute an RFC 3339 timestamp for "24 hours ago" in UTC. Call
`linode_audit_summary` with `since: <that timestamp>` and no other
arguments; the tool defaults to grouping by `[tool, status]`.

If `$ARGUMENTS` is a number followed by `h` (e.g. `48h`, `168h`), use that
window instead of 24h. If `$ARGUMENTS` matches one of the group_by allowed
values (`tool`, `status`, `capability`, `profile`, `environment`), pass it
as a single-element `group_by` array; otherwise leave `group_by` unset
(default `[tool, status]`).

Render the response as a count-descending table. Mention the window in the
header so the user is sure what range they are looking at.
````

### Try the slash commands

Open a Claude Code session and run:

```text
/audit
/audit-errors
/audit-by tool linode_instance_*
/audit-summary
/audit-summary 168h
```

## Claude Desktop: ask Claude directly

Inside a Claude Desktop conversation you can drive the audit query tools by asking the model in natural language. Phrases that map cleanly to the underlying tools:

- "What did you do in the last hour?" → `linode_audit_recent` with no filters
- "Show me only the errors from today" → `linode_audit_recent` with `status: "error"` and a `since` of midnight UTC
- "How many linode_instance_* calls in the last day?" → `linode_audit_recent` with `tool: "linode_instance_*"` and a `since` window, or `linode_audit_summary` for a count
- "Group destroys by environment for the last 24 hours" → `linode_audit_summary` with `since: <24h ago>` and `group_by: ["tool", "environment"]`, plus a follow-up filter

The model selects between `_recent` (returns events) and `_summary` (returns counts) based on whether the question is about specific calls or aggregate counts. If the model picks the wrong one, name the tool explicitly: "use linode_audit_summary with group_by tool".

## Query from a terminal

The same audit data is valuable outside a Claude session, especially during incident review when you would rather skip LLM latency for a known query. The `audit` CLI verb wraps each query tool one-to-one and drives it through the same dispatch, so the read is itself audited and profile-checked:

```bash
linodemcp audit recent [--tool GLOB] [--since TS] [--limit N] [--include-meta]
linodemcp audit summary [--since TS]
linodemcp audit health
linodemcp audit export --format json|csv|ndjson [--tool GLOB] [--since TS]
linodemcp audit report NAME
```

Output is the tool's JSON response, ready to pipe into `jq`. See [cli.md](../cli.md) for the verb reference and [audit.md](../audit.md) for the flags and filters.

Wrap whatever queries you repeat into shell aliases:

```bash
# LinodeMCP audit shortcut. Reload your shell after editing ~/.zshrc.
alias lma='linodemcp audit recent'
```

## What audit cannot see

Two things to know before relying on these queries for investigations:

1. **User prompts are not recorded.** The audit log captures what the model called, not what the user asked. MCP does not surface the user's natural-language prompt to the server. The conversation transcript on the host side is the source of truth for intent.
2. **Meta events are excluded by default.** Calls to `linode_audit_*` and `linode_profile_*` produce audit events of their own, with `capability: meta`. The query tools omit them by default so activity reviews show Linode work, not the assistant's bookkeeping. Pass `include_meta: true` (or ask the model to do so) when you specifically want to inspect those calls.

## Gotchas

- **Slash command not found (Claude Code).** Claude Code reloads slash commands on session start, not on file save. Restart Claude Code after adding any of the four files.
- **Empty result on `/audit`.** The audit log writes to `$XDG_STATE_HOME/linodemcp/audit.log` (default `~/.local/state/linodemcp/audit.log`). If the directory does not exist yet, no tool has been called against this LinodeMCP instance since startup. Run any LinodeMCP tool (e.g. `hello`) and try again.
- **`since` rejected.** The tools require RFC 3339 with a timezone (`Z` or `±HH:MM`). A bare `2026-05-20T00:00:00` will be refused. The model usually formats this correctly; if it doesn't, point it at `linode_audit_recent`'s description.
- **Filtering by profile or environment.** The `linode_audit_recent` tool's input schema only exposes `tool`, `capability`, `status`, `include_meta`, and the time-window filters. Filtering by profile or environment is available in `linode_audit_summary` (via group_by) and in named custom reports (`linode_audit_report`), not in the recent query.
- **Claude Desktop quit kills in-flight reads.** The MCP server runs as a child of Claude Desktop. Cmd+Q (macOS) sends SIGTERM to the server, which flushes its sink buffers before exiting. Force-quit (Cmd+Option+Esc) does not, so a `kill -9` style exit may lose the last fraction of a second of events. Prefer Cmd+Q.

## See also

- `linode_audit_health` reports the audit subsystem's own state (active log path, rotated file count, disk bytes, SQLite stats when enabled). Ask the model to call it when investigations suggest the audit log itself might be missing data.
- `linode_audit_export` dumps a filtered range to a temp file in JSON, CSV, or NDJSON. Use it when the inline output gets too long to render.
- `linode_audit_report` runs a user-defined report from `audit.reports` in the config file. Useful for recurring queries like "destroys on prod this week" that you would otherwise repeat across many ad-hoc calls.
