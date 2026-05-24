# Claude Desktop: audit log shortcuts

Claude Desktop has no user-defined slash command system, so audit queries live in two places: as direct MCP tool calls inside a Claude conversation, and as shell aliases for the same operations when you want to query from a terminal without the LLM in the loop.

Assumes you have already registered LinodeMCP with Claude Desktop per `../profile.md` step 1. The audit tools have capability `CapMeta`, so they are available in every profile (including `default`); you do not need a write-tier profile to query the audit log.

## 1. Confirm the audit tools are available

In a Claude Desktop session:

```text
List the LinodeMCP tools available to you whose name starts with linode_audit_.
```

You should see at least `linode_audit_recent`, `linode_audit_summary`, `linode_audit_health`, `linode_audit_export`, and `linode_audit_report`. If they are missing, the active profile has somehow filtered them out; check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## 2. Ask Claude directly

Inside a Claude Desktop conversation you can drive the audit query tools by asking the model in natural language. Phrases that map cleanly to the underlying tools:

- "What did you do in the last hour?" → `linode_audit_recent` with no filters
- "Show me only the errors from today" → `linode_audit_recent` with `status: "error"` and a `since` of midnight UTC
- "How many linode_instance_* calls in the last day?" → `linode_audit_recent` with `tool: "linode_instance_*"` and a `since` window, or `linode_audit_summary` for a count
- "Group destroys by environment for the last 24 hours" → `linode_audit_summary` with `since: <24h ago>` and `group_by: ["tool", "environment"]`, plus a follow-up filter

The model selects between `_recent` (returns events) and `_summary` (returns counts) based on whether the question is about specific calls or aggregate counts. If the model picks the wrong one, name the tool explicitly: "use linode_audit_summary with group_by tool".

## 3. Query from a terminal with jq

The same audit data is valuable outside a Claude session, especially during incident review when you would rather skip LLM latency for a known query. The JSONL audit log lives at `~/.local/state/linodemcp/audit.log` (or `/var/log/linodemcp/audit.log` on a system-service install), with one event per line. `jq` is enough:

```bash
# Last 20 events, human-readable.
tail -n 20 ~/.local/state/linodemcp/audit.log | jq '.'

# Errors only (most recent first).
tac ~/.local/state/linodemcp/audit.log \
  | jq -c 'select(.status == "error")' \
  | head -n 20

# Count by tool and status across the whole log.
jq -r '[.tool, .status] | @tsv' ~/.local/state/linodemcp/audit.log \
  | sort | uniq -c | sort -rn
```

The field names match the event schema in `docs/audit-log.md` (Phase 5). When the SQLite sink is enabled, the same data is queryable via `sqlite3 ~/.local/state/linodemcp/audit.db` for SQL-shaped questions; the `linode_audit_summary` MCP tool is the structured wrapper around that table.

Wrap whatever queries you repeat into shell aliases:

```bash
# LinodeMCP audit jq shortcuts. Reload your shell after editing ~/.zshrc.
alias lma='tail -n 20 ~/.local/state/linodemcp/audit.log | jq .'
alias lmae='tac ~/.local/state/linodemcp/audit.log | jq -c "select(.status == \"error\")" | head -n 20'
```

## 4. Optional: status-line helper

If you want a quick "is anything broken?" indicator in your shell prompt, count errors in the last hour:

```bash
lma_recent_errors() {
  local cutoff_ns
  cutoff_ns=$(date -u -v-1H +"%s")000000000 2>/dev/null \
    || cutoff_ns=$(date -u -d "1 hour ago" +"%s")000000000
  jq -c --argjson c "$cutoff_ns" \
    'select(.status == "error" and .ts_unix_ns >= $c)' \
    ~/.local/state/linodemcp/audit.log 2>/dev/null \
    | wc -l \
    | tr -d ' '
}
```

Then in your prompt config:

```bash
PS1='[lmp-err:$(lma_recent_errors)] %~ %# '
```

A non-zero count tells you to run `/audit-errors`-style queries inside Claude or `lmae` from the shell to investigate.

## What audit cannot see

Two things to know before relying on these queries for investigations:

1. **User prompts are not recorded.** The audit log captures what the model called, not what the user asked. MCP does not surface the user's natural-language prompt to the server. The Claude Desktop conversation transcript is the source of truth for intent.
2. **Meta events are excluded by default.** Calls to `linode_audit_*` and `linode_profile_*` produce audit events of their own, with `capability: meta`. Query tools omit them by default so activity reviews show Linode work, not the assistant's bookkeeping. Ask the model to pass `include_meta: true` when you specifically want to inspect those calls.

## Gotchas

- **Empty audit log.** The JSONL writer creates `~/.local/state/linodemcp/audit.log` on the first tool call after server startup. If the file does not exist, no LinodeMCP tool has been called yet against this instance. Run any tool (e.g. ask the model to call `hello`) and the log appears.
- **`since` rejected.** The audit query tools require RFC 3339 with a timezone (`Z` or `±HH:MM`). A bare `2026-05-20T00:00:00` will be refused. The model usually formats this correctly; if it does not, point it at `linode_audit_recent`'s description.
- **Claude Desktop quit kills in-flight reads.** The MCP server runs as a child of Claude Desktop. Cmd+Q (macOS) sends SIGTERM to the server, which flushes its sink buffers before exiting. Force-quit (Cmd+Option+Esc) does not, so a `kill -9` style exit may lose the last fraction of a second of events. Prefer Cmd+Q.
- **Filter by profile or environment.** The `linode_audit_recent` tool does not accept `profile` or `environment` filter args today. Use `linode_audit_summary` with `group_by: ["profile"]` for aggregate counts, or define a named report in `audit.reports` and call `linode_audit_report`.

## Why no in-app slash command?

Claude Desktop does not currently expose a user-defined slash command system. The equivalent MCP feature is "prompts": text templates the server exposes that the host renders as `/server-name/prompt-name`. The audit query tools could ship as MCP prompts, but those rendered prompts would still call the same `linode_audit_*` tools under the hood and would not save any work versus asking the model directly. The shell aliases above cover the terminal-only path. The model-driven path lives inside the conversation.

## See also

- `linode_audit_health` reports the audit subsystem's own state (active log path, rotated file count, disk bytes, SQLite stats when enabled). Ask the model to call it when investigations suggest the audit log itself might be missing data.
- `linode_audit_export` dumps a filtered range to a temp file in JSON, CSV, or NDJSON. Use it when the inline output gets too long to render.
- `linode_audit_report` runs a user-defined report from `audit.reports` in the config file. Useful for recurring queries like "destroys on prod this week" that you would otherwise repeat across many ad-hoc calls.
