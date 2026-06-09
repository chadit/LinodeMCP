# State drift

Drift is when a resource changes between the moment you [plan](./two-stage-writes.md)
a destructive call and the moment you apply it. Say you plan to delete a volume,
and before you apply, someone attaches it to an instance or resizes it. The thing
you reviewed is no longer the thing in front of you. An apply that went ahead
anyway would be acting on stale information.

The two-stage flow catches this. On plan, the server hashes the resource's
current state and stores the hash with the plan. On apply, it re-fetches the
resource, hashes it again, and compares. Same hash, the apply runs. Different
hash, the apply is refused and you're told to re-plan.

## What counts as a change

The hash covers the resource's fields, with one deliberate exception:
**cosmetic fields are stripped before hashing.** Some fields move on their own,
with no action from you - a server-side `updated` timestamp, a telemetry counter,
a `last_seen_ipv4`, the node list of a pool that recycles nodes. If those counted
as drift, a plan would refuse for reasons that have nothing to do with what you're
deleting.

Each resource type has a small ignore list of these fields. A change limited to
an ignored field is not drift; the apply still runs. A change to any other field
is drift. The lists start conservative (strip the obvious timestamps and
telemetry, hash everything else) and grow as real false-positive reports come in.
Over-detecting drift fails safe: the worst case is a needless re-plan, never a
delete you didn't review.

## Reading a refusal

An apply that doesn't run returns one of four messages. Each names the plan id and
tells you what to do.

### `PLAN_DRIFT_DETECTED`

```text
PLAN_DRIFT_DETECTED: the resource changed since plan "plan_018f..." was created.
Create a new plan with mode: "plan" and review before applying.
```

The resource moved in a way that isn't cosmetic. Re-plan, look at the new
`current_state`, and decide whether you still want to delete it. Don't reuse the
old plan id.

### `PLAN_EXPIRED`

```text
PLAN_EXPIRED: plan "plan_018f..." has expired. Create a new plan with mode: "plan".
```

A plan is valid for five minutes by default (configurable - see
[two-stage-writes.md](./two-stage-writes.md#configuration)). Past that it's gone,
because the older a plan gets the more likely the resource has drifted under it.
Re-plan.

### `PLAN_NOT_FOUND`

```text
PLAN_NOT_FOUND: no plan with id "plan_018f...". Create a new plan with mode: "plan".
Plans do not persist across a server restart.
```

The id isn't in the store. Either it was already applied (plans are single-use, so
a second apply of the same id fails this way), or the server restarted and dropped
its in-memory plans, or the id is wrong. Re-plan.

### `PLAN_ARGS_MISMATCH`

```text
PLAN_ARGS_MISMATCH: args supplied at apply time differ from plan "plan_018f...".
Apply without passing args (the plan retains them), or create a new plan.
```

You passed resource arguments on apply that don't match the plan. The plan already
holds the arguments from when you planned it, so the simplest fix is to apply with
just `mode` and `plan_id`. This check stops you from pairing one plan's id with
another call's arguments.

## Recovering

The recovery is the same for all four: **plan again.** A fresh plan re-reads the
resource, so you review what's actually there now, and hands back a new id to
apply. There's no way to "force" an apply past a refusal short of dropping to the
single-step `confirm` path or `yolo` (see [dry-run.md](./dry-run.md)), which skip
the plan entirely and so skip drift detection with them.
