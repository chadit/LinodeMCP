# Two-stage writes (plan / apply)

A dry-run shows you what a destructive call *would* do, but nothing ties that
preview to the call you make next. Between previewing a delete and running it,
the resource can change underneath you, and the plain dry-run flow has no way to
notice. Two-stage writes close that gap: you ask for a **plan**, review it, then
**apply** it by id. On apply the server re-reads the resource, compares it
against what you planned, and refuses if it moved.

This builds on the same machinery as [dry-run, confirm, and yolo](./dry-run.md)
and feeds the same [audit log](./audit-log.md). If you only need a preview and
don't care about catching drift, a plain `dry_run: true` is simpler and there's
nothing to apply. Reach for plan/apply when the gap between "I looked at it" and
"I deleted it" matters.

For what counts as drift and how to read a refusal, see
[state drift](./state-drift.md).

## The two modes

Every opted-in tool takes two extra arguments:

| Arg | Meaning |
| --- | --- |
| `mode` | `"plan"` previews and returns a `plan_id`; `"apply"` runs a stored plan. Omit it for an ordinary single-step call. |
| `plan_id` | The id from a `mode: "plan"` response, supplied with `mode: "apply"`. |

### Plan

Call the tool with `mode: "plan"`. The server fetches the current state, hashes
it, stores a single-use plan, and hands back a preview:

```jsonc
// linode_volume_delete  with  {"volume_id": 12345, "mode": "plan"}
{
  "plan_id": "plan_018f...c2a",
  "created_at": "2026-06-08T17:30:00Z",
  "expires_at": "2026-06-08T17:35:00Z",
  "tool": "linode_volume_delete",
  "environment": "prod",
  "would_execute": { "method": "DELETE", "path": "/volumes/12345" },
  "current_state": { "id": 12345, "label": "data", "linode_id": 999, "size": 80 },
  "current_state_hash": "sha256:9f86d0...",
  "dependencies": [
    { "kind": "instance", "id": 999, "label": "web-01", "action": "detached",
      "note": "Volume is attached; it detaches from this instance before deletion." }
  ],
  "warnings": ["Volume is currently attached to an instance; it will be detached as part of deletion."]
}
```

A plan never mutates anything. Like a dry-run, it only issues `GET` calls to
populate `current_state`. No delete happens until you apply.

A plan is a superset of a detailed dry-run: it runs the same dependency walk, so
when a tool has one the plan body carries `dependencies`, `side_effects`,
`billing_delta`, and `warnings` right alongside the state hash. You review the
blast radius and the exact resource you'll apply against in one response. Tools
without a walk just omit those fields.

### Apply

Call the same tool with `mode: "apply"` and the `plan_id` you got back:

```jsonc
// linode_volume_delete  with  {"mode": "apply", "plan_id": "plan_018f...c2a"}
{ "message": "Volume 12345 removed successfully", "volume_id": 12345 }
```

Before it runs the delete, the server re-fetches the resource and re-hashes it.
If the hash still matches the plan, the call executes. If it doesn't, the server
refuses and tells you the resource drifted - see [state drift](./state-drift.md).

You don't have to repeat the resource arguments on apply; the plan keeps them. If
you *do* pass them, they have to match what you planned, otherwise the apply is
refused with `PLAN_ARGS_MISMATCH`. This guards against applying plan A's id with
plan B's arguments.

## Where plan/apply sits next to confirm, dry-run, and yolo

A single destructive call can carry several of these controls at once. The server
resolves them in a fixed order and the first match wins:

1. **yolo** - if the profile permits it and the call sets `yolo: true`, it runs
   with no preview and no confirm. Dominates everything.
2. **apply** - `mode: "apply"` with a valid, un-drifted plan runs the call.
3. **plan** - `mode: "plan"` produces a plan and stops.
4. **dry_run** - `dry_run: true` previews and stops.
5. **single-step confirm** - the ordinary path: `confirm: true` plus either
   `confirmed_dry_run` or `confirm_bypass_dry_run` (see
   [dry-run.md](./dry-run.md)).
6. **refuse** - none of the above; the call is rejected and tells you what it
   needs.

So `mode` and `dry_run` are alternatives, not partners: `mode: "plan"` already
gives you the preview that `dry_run` would, plus a `plan_id` to apply against.

## Plan lifetime

- **Single use.** Applying a plan consumes it. A second apply of the same
  `plan_id` returns `PLAN_NOT_FOUND`.
- **Expiry.** A plan is valid for five minutes by default. After that, apply
  returns `PLAN_EXPIRED` and you re-plan. The window is the trade-off between
  giving a human time to review and the resource drifting while the plan sits.
- **In memory only.** Plans live in the running server's memory. A restart drops
  them all; there's nothing to apply afterward, so you re-plan.
- **Bounded.** The store holds at most 1000 outstanding plans. Past that, the
  oldest is evicted. A background sweeper also drops expired plans so they don't
  pile up.

## Which tools are opted in

The opt-in default is by capability: a destructive (`CapDestroy`) tool that
routes through the shared destroy flow opts in, so plan/apply works for it. The
delete tools are the core of that surface:

`linode_instance_delete`, `linode_volume_delete`, `linode_lke_cluster_delete`,
`linode_firewall_delete`, `linode_nodebalancer_delete`, `linode_vpc_delete`,
`linode_domain_delete`, `linode_image_delete`, `linode_stackscript_delete`,
`linode_sshkey_delete`, `linode_placement_group_delete`,
`linode_instance_disk_delete`, `linode_vpc_subnet_delete`,
`linode_domain_record_delete`, and `linode_lke_pool_delete`.

`linode_instance_rebuild` is also `CapDestroy` (a rebuild wipes every disk), so
it's opted in too. Its plan walks the instance's disks the same way its dry-run
does.

`CapAdmin` tools do not route through this flow, so they stay out by default,
opting them in would advertise a flow they can't run. Other capabilities stay
out unless you name them in `opt_in`. `linode_instance_resize` is the notable
write-tool case: it's `CapWrite`, so it's single-step until you opt it in (see
below), at which point a resize plan covers both the instance plan and its disk
layout.

A tool that isn't opted in ignores `mode` and `plan_id` and behaves like an
ordinary single-step call.

## Configuration

The defaults need no config. To tune the flow, add a `two_stage` block:

```yaml
two_stage:
  # Override the 5-minute default plan lifetime (seconds). Optional.
  default_plan_ttl_seconds: 600

  # Per-tool lifetime overrides (seconds). A tool with a bigger blast radius
  # can get more review time. Optional.
  tool_ttl_seconds:
    linode_lke_cluster_delete: 1800

  # Force a tool in or out of the flow by name, overriding the capability
  # default. Optional.
  opt_in:
    linode_image_delete: false      # take a destructive tool out
    linode_instance_resize: true    # pull a write tool in
```

Resolution order for a tool's plan lifetime: a `tool_ttl_seconds` entry wins,
then `default_plan_ttl_seconds`, then the built-in five minutes. A non-positive
value at any level is ignored and falls through to the next. The `opt_in` map
overrides the capability default per tool: set a destructive tool to `false` to
take it out of the flow, or a write tool like `linode_instance_resize` to `true`
to pull it in. The capability default itself only opts in `CapDestroy` tools;
everything else waits for an explicit `opt_in` entry.

## Audit trail

Every plan and apply is recorded in the [audit log](./audit-log.md) with its
`mode` (`plan` or `apply`), alongside the `bypass_dry_run`, `yolo`, and `normal`
modes the other paths record. To see the two-stage calls a session made:

```text
linode_audit_recent  with  {"tool": "linode_*_delete"}
```

The apply event is the one that actually changed something; the plan event before
it is the preview.
