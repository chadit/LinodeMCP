# Claude Desktop: two-stage write shortcuts

Claude Desktop has no user-defined slash command system, so the [two-stage write
flow](../../../two-stage-writes.md) lives as plain instructions you give inside a
conversation. The flow is: plan a destructive call, review the state Claude shows
you, then apply the returned `plan_id`.

Assumes you have already registered LinodeMCP with Claude Desktop per
`../profile.md` step 1, and that the active profile permits the delete tool (a
write-tier or destroy-tier profile). The fifteen opted-in delete tools are listed
in [two-stage-writes.md](../../../two-stage-writes.md#which-tools-are-opted-in).

## 1. Confirm a delete tool is available

In a Claude Desktop session:

```text
List the LinodeMCP tools available to you whose name ends in _delete.
```

If the list is empty, the active profile has filtered the destructive tools out.
Check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## 2. Plan a destructive call

Ask Claude to plan, not run, the delete:

```text
Plan the deletion of volume 12345: call linode_volume_delete with
mode: "plan" and volume_id: 12345. Show me the current_state and the plan_id,
and do not apply anything.
```

Claude calls the tool with `mode: "plan"`, which performs no delete. It returns
the volume's current state, a `plan_id` (starts with `plan_`), and an
`expires_at` (five minutes out by default).

## 3. Review, then apply

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

## A standing instruction

If you want this to be the default for destructive calls in a conversation, paste
this once near the top:

```text
For any LinodeMCP *_delete tool, never run it in one step. First call it with
mode: "plan", show me the current_state and plan_id, and wait. Only after I
confirm, call it again with mode: "apply" and that plan_id and no other args.
If an apply is refused (PLAN_DRIFT_DETECTED / PLAN_EXPIRED / PLAN_NOT_FOUND /
PLAN_ARGS_MISMATCH), stop and tell me; do not retry.
```

For the full flow and the drift refusal reference, see
[two-stage-writes.md](../../../two-stage-writes.md) and
[state-drift.md](../../../state-drift.md).
