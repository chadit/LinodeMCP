# Dry-run, bypass-confirm, pre-check, and yolo

Every mutating tool in LinodeMCP can be previewed before it runs, and destructive
calls are gated so the model cannot delete or replace a resource without either
previewing it first or explicitly opting out. These safety mechanisms layer on
top of [profiles](./profiles.md) (which decide *what* the model may call) and the
[audit log](./audit-log.md) (which records *what it did*, including which safety
path each call took).

Four related features are covered here:

1. **Dry-run**: preview any mutator without performing it.
2. **Bypass-confirm**: a destructive call must have been previewed (or explicitly
   waive the preview) before it executes.
3. **Pre-check**: ask, before running a sequence, which calls the active profile
   would permit.
4. **Yolo**: a per-profile break-glass mode that skips both the preview gate and
   the confirm requirement.

## Dry-run

Pass `dry_run: true` to any `CapWrite`, `CapDestroy`, or `CapAdmin` tool. The tool
performs no mutation; it returns what *would* happen instead.

```jsonc
// linode_volume_delete  with  {"volume_id": 12345, "dry_run": true}
{
  "dry_run": true,
  "tool": "linode_volume_delete",
  "environment": "prod",
  "would_execute": { "method": "DELETE", "path": "/volumes/12345" },
  "current_state": { "id": 12345, "label": "data", "linode_id": 999, "size": 80 },
  "dependencies": [
    { "kind": "instance", "id": 999, "label": "web-01", "action": "detached",
      "note": "the volume is detached from this instance; the instance is not deleted" }
  ],
  "warnings": ["Volume deletion is irreversible; the data on it is destroyed."]
}
```

A dry-run only ever issues read (`GET`) calls to populate `current_state` and the
dependency walk. It never writes. Two identical dry-runs return identical results
(modulo timestamps and any real state drift between them). There is no plan ID and
nothing to "apply". To actually run the call, the model invokes the tool again
without `dry_run`. (The apply-by-id flow is the separate two-stage-writes spec.)

### Response fields

| Field | When present | Meaning |
| --- | --- | --- |
| `would_execute` | always | the HTTP method + path the real call would issue |
| `current_state` | when the resource exists | the resource as it is now (credential-safe: secrets are never surfaced, since token/credential previews fetch the parent resource's metadata, never the secret) |
| `dependencies` | Tier A destroys | other resources the call cascades to: each has `kind`, `id`, `label`, `action` (`detached`/`released`/`removed`/`cascade_deleted`), and a `note` |
| `side_effects` | Tier B creates/updates | plain-language description of what changes |
| `billing_delta` | some Tier A/B | estimated monthly cost change |
| `warnings` | as needed | irreversibility, downtime, secret-shown-once, etc. |

All of `dependencies` / `side_effects` / `billing_delta` / `warnings` are optional
(`omitempty`), so a simple preview carries only `would_execute` + `current_state`.

### Tiers

Tools are tiered by blast radius, which sets how rich the preview is:

- **Tier A, full dependency walk.** High-blast destroys that cascade across
  dependent resources (instance/volume/LKE-cluster/firewall/domain/nodebalancer/
  VPC deletes, etc.). These populate `dependencies[]`.
- **Tier B, side effects.** Creates and updates surface `side_effects[]` (and
  `billing_delta`/`warnings` where relevant).
- **Tier C, state-only.** Single-resource operations with no cross-resource
  cascade (e.g. token/credential deletes). The preview carries `current_state` +
  `would_execute` only, by design.

Coverage is enforced by an invariant test in each language
(`TestCapabilityAndDryRunInvariants` in Go, `test_capability_and_dry_run_invariants`
in Python): the build fails if any `CapWrite`/`CapDestroy`/`CapAdmin` tool does not
advertise `dry_run`.

## Bypass-confirm (destructive calls)

`CapDestroy` tools already require `confirm: true`. On top of that, a real
destructive call must show it previewed the operation, otherwise it is rejected:

```text
linode_instance_delete is destructive. Either:
  1. Call with dry_run: true first to preview, then call again with
     confirm: true, confirmed_dry_run: true
  2. Call with confirm: true, confirm_bypass_dry_run: true to skip preview
  3. Use yolo: true (only if profile allows)
```

So a destructive execution needs one of:

| Flags (with `confirm: true`) | Outcome |
| --- | --- |
| `confirmed_dry_run: true` | proceed: the model asserts it ran a dry-run for this exact call |
| `confirm_bypass_dry_run: true` | proceed: the model explicitly skips the preview |
| neither | rejected with the message above |

Two guard rails:

- `confirm_bypass_dry_run` without `confirm: true` → `"confirm_bypass_dry_run only
  takes effect with confirm: true"`.
- both `confirm_bypass_dry_run` and `confirmed_dry_run` → `"Pass either
  confirm_bypass_dry_run (skip preview) or confirmed_dry_run (preview was done),
  not both"`.

Detection is client-asserted: the server trusts `confirmed_dry_run` rather than
tracking session state. The defense against a model that lies is **observability**,
not server-side gatekeeping, the [audit log](./audit-log.md) records both the
dry-run and the apply (and the `mode`, below), so a destructive call with no
preceding dry-run is visible after the fact.

## Pre-check

`linode_profile_can_run` (a `CapMeta` tool, available in every profile) answers
"would the active profile permit this sequence of calls?" *before* the model
commits to a multi-step plan, so a profile block doesn't strand it after partial
execution.

```jsonc
// request
{ "calls": [
    { "tool": "linode_instance_list" },
    { "tool": "linode_instance_create", "args": { "region": "us-east" } },
    { "tool": "linode_instance_delete", "args": { "linode_id": 12345 } }
] }
// response
{
  "active_profile": "compute-readonly",
  "results": [
    { "tool": "linode_instance_list", "allowed": true },
    { "tool": "linode_instance_create", "allowed": false,
      "reason": "tool not in profile's allowed_tools",
      "remedy": "switch to a profile that permits linode_instance_create, or add it to the current profile" },
    { "tool": "linode_instance_delete", "allowed": false,
      "reason": "tool not in profile's allowed_tools (CapDestroy)",
      "remedy": "switch to a profile that permits linode_instance_delete, or use yolo on a profile that allows it" }
  ],
  "summary": { "total": 3, "allowed": 1, "blocked": 2,
    "blocked_by_reason": { "unregistered": 0, "profile_block": 1, "environment_block": 0, "capability_block": 1 } }
}
```

Pre-check inspects only the tool **name** and the optional `environment` arg
against the active profile, not resource IDs, token scope, or resource existence.
It is advice; the model is free to ignore it. The three refusal reasons are
`"tool name not registered"`, `"tool not in profile's allowed_tools"` (with an
optional `(CapXxx)` annotation that splits it into the `capability_block` summary
bucket), and `"environment not permitted by profile"`. The invariant
`sum(blocked_by_reason) <= blocked` always holds.

## Yolo

A profile may set `allow_yolo: true` (only the disabled-by-default `emergency`
built-in does). On such a profile, passing `yolo: true` bypasses **both** the
dry-run gate and the `confirm` requirement, executing immediately. On any other
profile, `yolo: true` is ignored and the normal flow applies. Yolo is the
break-glass path for an operator who knows what they're doing; it trades safety
for speed and is recorded as `mode: yolo` in the audit log.

## Audit modes

Every call records the safety path it took in the audit event's `mode` field:

| `mode` | Meaning |
| --- | --- |
| `normal` | a regular call (including a confirmed destroy) |
| `dry_run` | a preview; nothing was mutated |
| `bypass_dry_run` | a destroy executed with `confirm_bypass_dry_run` (no preview) |
| `yolo` | executed via the yolo break-glass path |

Filter on it with the audit query tools, e.g. find every preview-skipped destroy:

```text
linode_audit_recent  with  {"capability": "destroy"}
# then inspect mode == "bypass_dry_run" or "yolo"
```

## See also

- [profiles.md](./profiles.md): what the active profile permits, and `allow_yolo`
- [audit-log.md](./audit-log.md): the event schema, the `mode` field, and queries
