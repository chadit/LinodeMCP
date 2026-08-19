# LinodeMCP documentation

The map of everything under `docs/`. Two kinds of files live here, and telling
them apart is most of the orientation you need:

- **Pages** (`.md`) are prose for people and agents. Each covers one topic and
  links to its neighbors, so any page can be read on its own.
- **Machine contracts** (`.txt`, all under [`contracts/`](./contracts/)) are
  read by the `make check` gates by exact path. Scripts generate and ratchet
  them. Never edit their entry lines by hand; each file's header comment names
  its rules and regenerate command. They are listed in
  [Machine contracts](#machine-contracts) below so nobody mistakes them for
  reading material.

New here? Read the [root README](../README.md) for install and first-run, then
[profiles](./profiles.md), [dry-run](./dry-run.md), and
[two-stage writes](./two-stage-writes.md) for the safety model. The repo root
also carries an `llms.txt` with this same map, one line per page.

## Permissions

- [Profiles](./profiles.md): the permission model. A profile names the set of
  tools the connected AI client can see and call. Also covers capability tags,
  the built-in catalog, config schema, token-scope validation, and hot-reload.
- [Profile recipes](./profile-recipes.md): copy-paste profile configs for
  common postures (read-only oncall, DNS admin, dev-environment-only).

## Write safety

- [Dry-run, bypass-confirm, pre-check, and yolo](./dry-run.md): preview any
  mutator before it runs. Destructive calls must be previewed or explicitly
  waived.
- [Two-stage writes](./two-stage-writes.md): plan a destructive call, review
  it, apply it by id. The server refuses the apply if the resource changed.
- [State drift](./state-drift.md): what counts as a change between plan and
  apply, and how to read each refusal (`PLAN_DRIFT_DETECTED`, `PLAN_EXPIRED`,
  `PLAN_NOT_FOUND`).

## Object Storage

- [Object Storage data plane](./object-storage-data-plane.md): moving object
  bytes through a presigned URL, in both directions. Why no access key is
  involved, what the server needs on its own filesystem, how to verify a stored
  object, the single-part ceiling, and how a download guards the file it
  writes.

## Audit

- [Audit log](./audit-log.md): structured record of every tool invocation.
  Event schema, redaction, the query tools, and investigative patterns.
- [Audit operations](./audit-operations.md): running the audit subsystem.
  Sinks, retention, the config block, recovery, and failure modes.
- [Audit reports](./audit-reports.md): named queries against the audit log,
  defined in config. The filter grammar, with worked examples.

## Host integrations

- [Host integrations](./host-integrations/README.md): wiring per MCP host.
  Launch config plus command wrappers for Claude Code and Claude Desktop,
  each in its own self-contained directory.

## Running and operating

- [CLI and server modes](./cli.md): one binary, two modes. Bare invocation is
  the MCP stdio server; the verbs (`profile`, `call`, `tools`, `audit`,
  `tui`, `version`) are shell commands that exit without starting it.
- [Observability](./observability.md): Prometheus metrics on :8888, health
  endpoints on :8889, and OTLP tracing. Metric names and labels, and which
  probe goes where.

## Releases

- [Release process](./release-process.md): maintainer runbook. The two
  workflows that cut a release, and everything a release ships.
- [Verifying releases](./verifying-releases.md): copy-paste commands to check
  checksums, container signatures, SBOMs, and SLSA provenance.

## Contributing

- [Cross-language parity](./parity.md): how the implementations stay
  wire-identical, and what pulls the other languages along when you change
  one. The day-to-day playbook: adding, changing, or removing a tool;
  landing one language first with a tracked absence; the commands.
- [Adding a language](./adding-a-language.md): the checklist for a new
  language implementation. Everything derives from the proto contract, and
  the gates fail you if you hand-write any of it.
- [The check gates](./gates.md): every `make check` gate in one page. What
  each one checks, the contract file it ratchets, and how to update it.
- [Git hooks](./git-hooks.md): pre-commit setup for commit-time and push-time
  checks (`make install-hooks`).
- [Deprecated routes](./deprecated-routes.md): tools and routes removed from
  the surface, with the replacement to use instead.

## Machine contracts

Gate-consumed files, all under `docs/contracts/`. `make check` reads every one
of these by exact path, so moving or renaming one means a coordinated sweep of
its consumers. Baselines are ratchets: fixing an item removes its line, and
lines are never added by hand. Most gates have no baseline at all (see [Hard
gates](#hard-gates-no-baseline)); the ones below are what is left. An accepted
line carries a dated annotation
citing a tracking-issue URL, and the baseline guard fails growth without one
(the two `*-exempt.txt` files may use a free-text reason instead). That guard
only checks the shape of the URL, so `make sync-issues` resolves each cited
issue on the sync schedule and fails when one is closed; an acceptance whose
issue can never close belongs in an exempt file, not a ratchet. Each file's
header comment holds its full rules and exact regenerate command.

Which Linode API operation a tool calls is not one of these files. It lives in
the proto contract, as a `linode.mcp.v1.tool_route` option on the tool's
`*Input` message carrying the tool name, the method, and the path template.
That makes an input message the equivalent of an OpenAPI operation object, so
the generated descriptors answer "which route?" for every consumer.
`scripts/_toolroutes.py` is the shared reader; `make tool-routes` pins the
annotations from both sides against `tools-manifest.txt`.

A path template names each of its parameters in snake_case, the way the Linode
API documentation names them:
`/databases/postgresql/instances/{postgresql_instance_id}`. The name is there
to be read, not dispatched on. The two route scanners resolve routes out of
code that builds URLs from variables, where no name exists, so they emit the
`{p}` placeholder for every parameter and anything comparing a declared route
against a resolved one runs it through `_toolroutes.norm_template` first.
Shape is what the gates enforce; `make tool-routes` is the only place a name
is checked, and only for the convention.

Which tools exist and what each may do lives in the proto too. Every `*Input`
message declares `linode.mcp.v1.tool_capability`, and names its tool in exactly
one marker: `tool_route` for the 444 that reach the Linode API, and
`linode.mcp.v1.tool_meta` for the 17 that work on local config or session state.
So the descriptors alone answer "which tools exist, and which are Meta by
design", which is what lets each language's route validator take no arguments
and lets a server check its own registry against the contract at startup.
`tools-manifest.txt` and `tools-capabilities.txt` are that declaration written
out: `scripts/gen_tool_registries.py` emits both inside `make proto`, and both
are gitignored the way the generated code is, so neither is a second place a
tool can be added. `make tool-capability` still compares them against the
descriptors, which now catches a stale file rather than a hand edit.

What a tool answers with lives on the same input message. `tool_response` names
the message the handler serializes, `confirm_message` carries the exact prose a
Write, Admin, or Destroy tool returns when `confirm` is unset, `success_message`
carries the completed-mutation text with `{field}` placeholders bound to fields
of the input or the response, `resource_type` names the two-stage hash-ignore
key a Destroy uses, and `retry_disabled` marks a call that must not be replayed.
The response binding is written out rather than derived: about half the surface
answers with a message spelled differently from what its input name would
suggest, so `AccountBetaGetInput` naming `AccountBetaProgram` is ordinary. Four
tools whose Linode response is an open-ended object declare no response and are
listed by name in the gate. `make tool-response` pins all five from both
directions.

### Registries

| File | Pins | Consumed by |
|------|------|-------------|
| tools-manifest.txt | The full tool surface, one name per line. Generated from the `tool_route` and `tool_meta` options by `make proto` and gitignored | Manifest gate tests in each language |
| tools-capabilities.txt | Capability tier (`Read`/`Write`/`Destroy`/`Admin`/`Meta`) for every tool. Generated from the `tool_capability` options by `make proto` and gitignored | `scripts/verify_tool_capability.py`, capability gate tests in each language |
| [handwritten-tools.txt](./contracts/handwritten-tools.txt) | The tools each language still serves from a hand-written factory, which is the codegen cohort inverted: everything else is generated, so new surface is born generated (shrink-only) | `scripts/verify_generated_tools.py`, `go/cmd/toolgen` |
| [languages.txt](./contracts/languages.txt) | The registered language implementations: name, working dir, surface-dump command. `make proto` emits one tool tree per language listed here | `Makefile`, `go/cmd/toolgen`, `scripts/verify_tool_parity.py` |
| [env-vars.txt](./contracts/env-vars.txt) | The complete environment-variable surface every language reads (observability has none by design) | `scripts/verify_env_parity.py` |
| [coverage-floors.txt](./contracts/coverage-floors.txt) | Minimum total unit-test statement coverage per registered language (rise-only; the per-line half is `make diff-coverage`) | `scripts/verify_coverage_floor.py` |
| [route-source-counts.txt](./contracts/route-source-counts.txt) | Request call sites per registered language that still build their endpoint by hand instead of resolving it from the proto (fall-only; what is left of the route-builder migration) | `scripts/verify_route_source.py` |
| [generated-tools-counts.txt](./contracts/generated-tools-counts.txt) | Tools per registered language still served by a hand-written factory rather than by the tree the emitter writes for it from the proto (fall-only; what is left of the codegen migration) | `scripts/verify_generated_tools.py` |
| [hand-validator-counts.txt](./contracts/hand-validator-counts.txt) | Tools per registered language whose argument check is still hand-written rather than declared as buf.validate rules on the tool's `*Input` message (fall-only; what is left of the validator migration) | `scripts/verify_hand_validators.py` |
| [hook-body-counts.txt](./contracts/hook-body-counts.txt) | Handler steps per registered language per `tool_hooks` kind that are still written by hand rather than derived from the tool's `*Input` message (fall-only; the header records why each kind stays) | `scripts/verify_hook_bodies.py` |
| [system-params.txt](./contracts/system-params.txt) | The proto input fields the server consumes itself rather than passing to the Linode API, by field name and proto type; each one carries a trailing `// system param` marker that stays out of the generated schema | `scripts/verify_system_params.py` |

### Ratchet baselines

| File | Tracks | Owning script |
|------|--------|---------------|
| [tool-parity-baseline.txt](./contracts/tool-parity-baseline.txt) | Accepted one-sided tools, each annotated with a tracking reason. This is also how a newly registered language records the surface it has not caught up on (see [adding-a-language.md](./adding-a-language.md)) | `scripts/verify_tool_parity.py` |
| [behavior-dryrun-baseline.txt](./contracts/behavior-dryrun-baseline.txt) | Mutating tools whose fixture lacks a pinned dry-run preview case (Destroy stays at zero) | `scripts/verify_behavior.py` |
| [behavior-exempt.txt](./contracts/behavior-exempt.txt) | Tools the behavior gate structurally cannot pin, with reasons (hand-curated; new entries need the dated acceptance annotation) | `scripts/verify_behavior.py` |
| [scope-sync-exempt.txt](./contracts/scope-sync-exempt.txt) | Scope deviations no change here can close, such as a route upstream gates behind limited availability, with reasons (hand-curated; new entries need the dated acceptance annotation) | `scripts/verify_sync_scopes.py` |
| [enum-sync-baseline.txt](./contracts/enum-sync-baseline.txt) | Enum drift against the Linode OpenAPI spec (network; runs on the sync schedule) | `scripts/verify_sync_enums.py` |
| [api-defaults-baseline.txt](./contracts/api-defaults-baseline.txt) | Snapshot of API wire-body defaults at a reviewed OpenAPI version (network; runs on the sync schedule) | `scripts/verify_sync_defaults.py` |
| [scope-sync-baseline.txt](./contracts/scope-sync-baseline.txt) | Accepted deviations between the per-tool OAuth scope mapping and the spec's per-operation security blocks, each annotated with its tracking issue (network; runs on the sync schedule) | `scripts/verify_sync_scopes.py` |
| [api-pagination-baseline.txt](./contracts/api-pagination-baseline.txt) | Snapshot of paginated GET routes and their page_size bounds at a reviewed OpenAPI version (network; runs on the sync schedule) | `scripts/verify_sync_pagination.py` |
| [api-response-shapes-baseline.txt](./contracts/api-response-shapes-baseline.txt) | Snapshot of every route's success response shape at a reviewed OpenAPI version (network; runs on the sync schedule) | `scripts/verify_sync_response_shapes.py` |

### Hard gates (no baseline)

Most gates hold their class at zero and carry no file at all: a finding fails
by name, there is nothing to accept it into, and the fix ships with the change
that caused it. `input-proto`, `read-proto`, `write-proto` and `meta-proto`
(a hand-written tool surface), `behavior` coverage and its malformed-response
rule, `messages`, `pagination`, `response-shapes`, `list-envelope` and
`route-evidence` all work this way, along with the offline gates that never
had a baseline (`tool-routes`, `field-location`, `tool-capability`,
`tool-response`, `dryrun`, `env-parity`, `cli-surface`, `metrics-surface`,
`system-params`).

An empty baseline file used to say the same thing, and it also proved the gate
still had a surface to look at. Nothing carries that second meaning now, so
each hard gate reports what it measured and fails when that count is zero: a
classifier that stops resolving handlers, a scan whose tree moved, or a
snapshot that stopped parsing shows up as a failure instead of a clean run.

A few cross-language pins live as shared fixtures under `testdata/` rather
than contracts files, because a language's own unit tests consume them:
`testdata/config/parity.yml` (config parsing), `testdata/observability/duration_buckets.json`
(histogram bucket boundaries), and `testdata/audit/event_fields.json` (the
audit JSONL field set). Each language asserts against the same fixture, so a
one-sided edit fails that language's suite.
