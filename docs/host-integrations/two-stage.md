# Two-stage writes per host

How to drive the LinodeMCP [two-stage write flow](../two-stage-writes.md) from inside each MCP host: `/plan` and `/apply` slash commands for Claude Code, and plain conversation instructions for Claude Desktop, which has no user-defined slash command system. The flow is: plan a destructive call, review the state Claude shows you, then apply the returned `plan_id` after the server re-checks for drift.

Assumes the server is already registered with your host per the [directory README](./README.md), and that the active profile permits the delete tool you want to plan (a write-tier or destroy-tier profile).

## Confirm a delete tool is available

In a session with your host:

```text
List the LinodeMCP tools available to you whose name ends in _delete.
```

If the list is empty, the active profile has filtered the destructive tools out. Check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## Claude Code: `/plan` and `/apply` slash commands

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
  `/plan` again and review the fresh state.
````

### Try the slash commands

```text
/plan linode_volume_delete volume_id=12345
```

Review the `current_state` Claude shows you, then:

```text
/apply linode_volume_delete plan_018f...
```

## Claude Desktop: conversation instructions

### Plan a destructive call

Ask Claude to plan, not run, the delete:

```text
Plan the deletion of volume 12345: call linode_volume_delete with
mode: "plan" and volume_id: 12345. Show me the current_state and the plan_id,
and do not apply anything.
```

Claude calls the tool with `mode: "plan"`, which performs no delete. It returns
the volume's current state, a `plan_id` (starts with `plan_`), and an
`expires_at` (five minutes out by default).

### Review, then apply

Look at the `current_state`. If it's what you expect, tell Claude to apply:

```text
Apply plan <plan_id>: call linode_volume_delete with mode: "apply" and that
plan_id, and pass nothing else.
```

Before the delete runs, the server re-reads the volume and compares it to the
plan. If it still matches, the delete executes. If the volume changed, Claude
gets one of these back and should stop, not retry:

| Refusal | Meaning | What to do |
| --- | --- | --- |
| `PLAN_DRIFT_DETECTED` | the volume changed since the plan | plan again, review the new state |
| `PLAN_EXPIRED` | more than five minutes passed | plan again |
| `PLAN_NOT_FOUND` | already applied, or the server restarted | plan again |
| `PLAN_ARGS_MISMATCH` | apply-time args differ from the plan | apply with just the `plan_id` |

### A standing instruction

If you want this to be the default for destructive calls in a conversation, paste
this once near the top:

```text
For any LinodeMCP *_delete tool, never run it in one step. First call it with
mode: "plan", show me the current_state and plan_id, and wait. Only after I
confirm, call it again with mode: "apply" and that plan_id and no other args.
If an apply is refused (PLAN_DRIFT_DETECTED / PLAN_EXPIRED / PLAN_NOT_FOUND /
PLAN_ARGS_MISMATCH), stop and tell me; do not retry.
```

## Gotchas

- **Plans don't survive a server restart.** They live in the running server's
  memory. If you restart LinodeMCP between plan and apply, the apply fails
  with `PLAN_NOT_FOUND` and you re-plan.
- **A plan is single-use.** Applying it consumes it; a second apply of the
  same id returns `PLAN_NOT_FOUND`.
- **Plan and `dry_run` overlap.** A plan already gives you the preview a
  dry-run would, plus an id to apply. Use a plain `dry_run: true` only when you
  want the preview with nothing to apply.

For the full flow and the drift refusal reference, see
[two-stage-writes.md](../two-stage-writes.md).
