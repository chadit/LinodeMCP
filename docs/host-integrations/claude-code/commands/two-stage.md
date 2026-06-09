# Claude Code: `/plan` and `/apply` slash commands

This guide adds two slash commands for the LinodeMCP [two-stage write
flow](../../../two-stage-writes.md): `/plan` previews a destructive call and
returns a `plan_id`, and `/apply` runs a stored plan after the server re-checks
for drift.

Assumes you have already registered LinodeMCP with Claude Code per
`../profile.md` step 1, and that the active profile permits the delete tool you
want to plan (a write-tier or destroy-tier profile). The fifteen opted-in delete
tools are listed in [two-stage-writes.md](../../../two-stage-writes.md#which-tools-are-opted-in).

## 1. Confirm a delete tool is available

In a Claude Code session:

```text
List the LinodeMCP tools available to you whose name ends in _delete.
```

If the list is empty, the active profile has filtered the destructive tools out;
check `linodemcp profile show <active-name>`.

## 2. Drop in the slash commands

Claude Code slash commands are markdown files under `~/.claude/commands/`
(user-level) or `.claude/commands/` (project-level). Copy each block below into
its own file.

### `/plan`: preview a destructive call and get a plan id

`~/.claude/commands/plan.md`:

````markdown
---
description: Preview a LinodeMCP destructive call and return a plan_id to apply later.
---

# `/plan`

Read `$ARGUMENTS` as a tool name followed by its arguments, for example
`linode_volume_delete volume_id=12345`.

- Call that MCP tool with `mode: "plan"` plus the supplied arguments.
- Present the returned `current_state`, the `would_execute` line, and the
  `plan_id` and `expires_at`.
- Do not apply anything. Tell me to run `/apply <plan_id>` once I have reviewed
  the state. Remind me the plan expires at `expires_at` (five minutes by
  default).
````

### `/apply`: run a stored plan

`~/.claude/commands/apply.md`:

````markdown
---
description: Apply a LinodeMCP plan by its plan_id, after the server re-checks for drift.
---

# `/apply`

Read `$ARGUMENTS` as a single `plan_id` (it starts with `plan_`). It also has to
include the tool name the plan was for, for example
`linode_volume_delete plan_018f...`.

- Call that tool with `mode: "apply"` and `plan_id` set to the supplied id, and
  pass no other arguments (the plan keeps them).
- If the call succeeds, report what it did.
- If it returns `PLAN_DRIFT_DETECTED`, `PLAN_EXPIRED`, `PLAN_NOT_FOUND`, or
  `PLAN_ARGS_MISMATCH`, do not retry. Tell me which one, and that the fix is to
  `/plan` again and review the fresh state. See state-drift for what each means.
````

## 3. Try it

```text
/plan linode_volume_delete volume_id=12345
```

Review the `current_state` Claude shows you, then:

```text
/apply linode_volume_delete plan_018f...
```

## Gotchas

- **Plans don't survive a server restart.** They live in the running server's
  memory. If you restart LinodeMCP between `/plan` and `/apply`, the apply fails
  with `PLAN_NOT_FOUND` and you re-plan.
- **A plan is single-use.** Applying it consumes it; a second `/apply` of the
  same id returns `PLAN_NOT_FOUND`.
- **`/plan` and `dry_run` overlap.** `/plan` already gives you the preview a
  dry-run would, plus an id to apply. Use a plain `dry_run: true` only when you
  want the preview with nothing to apply.

For the full flow and the drift refusal reference, see
[two-stage-writes.md](../../../two-stage-writes.md) and
[state-drift.md](../../../state-drift.md).
