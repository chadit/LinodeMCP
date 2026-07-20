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
- [Git hooks](./git-hooks.md): pre-commit setup for commit-time and push-time
  checks (`make install-hooks`).
- [Deprecated routes](./deprecated-routes.md): tools and routes removed from
  the surface, with the replacement to use instead.

## Machine contracts

Gate-consumed files, all under `docs/contracts/`. `make check` reads every one
of these by exact path, so moving or renaming one means a coordinated sweep of
its consumers. Baselines are ratchets: fixing an item removes its line, and
lines are never added by hand. Each file's header comment holds its full rules
and exact regenerate command.

### Registries

| File | Pins | Consumed by |
|------|------|-------------|
| [tools-manifest.txt](./contracts/tools-manifest.txt) | The full tool surface: every tool any registered language implements, one name per line | Manifest gate tests in each language |
| [tools-capabilities.txt](./contracts/tools-capabilities.txt) | Capability tier (`Read`/`Write`/`Destroy`/`Admin`/`Meta`) for every tool | Capability gate tests in each language |
| [languages.txt](./contracts/languages.txt) | The registered language implementations: name, working dir, surface-dump command | `Makefile`, `scripts/verify_tool_parity.py` |
| [env-vars.txt](./contracts/env-vars.txt) | The complete environment-variable surface every language reads (observability has none by design) | `scripts/verify_env_parity.py` |
| [coverage-floors.txt](./contracts/coverage-floors.txt) | Minimum total unit-test statement coverage per registered language (rise-only; the per-line half is `make diff-coverage`) | `scripts/verify_coverage_floor.py` |

### Ratchet baselines

| File | Tracks | Owning script |
|------|--------|---------------|
| [tool-parity-baseline.txt](./contracts/tool-parity-baseline.txt) | Accepted one-sided tools, each annotated with a tracking reason | `scripts/verify_tool_parity.py` |
| [behavior-baseline.txt](./contracts/behavior-baseline.txt) | Tools with no shared behavior fixture yet | `scripts/verify_behavior.py` |
| [behavior-dryrun-baseline.txt](./contracts/behavior-dryrun-baseline.txt) | Mutating tools whose fixture lacks a pinned dry-run preview case (Destroy stays at zero) | `scripts/verify_behavior.py` |
| [behavior-exempt.txt](./contracts/behavior-exempt.txt) | Tools the behavior gate structurally cannot pin, with reasons (hand-curated; new entries need the dated acceptance annotation) | `scripts/verify_behavior.py` |
| [input-proto-baseline.txt](./contracts/input-proto-baseline.txt) | Tools whose input schema is not yet proto-generated on both sides | `scripts/verify_input_proto.py` |
| [read-proto-baseline.txt](./contracts/read-proto-baseline.txt) | Read tools not yet proto-routed on both sides | `scripts/verify_read_proto.py` |
| [write-proto-baseline.txt](./contracts/write-proto-baseline.txt) | Mutating tools not yet proto-routed on both sides | `scripts/verify_write_proto.py` |
| [write-proto-fixture-baseline.txt](./contracts/write-proto-fixture-baseline.txt) | `*WriteResponse` protos still missing a conformance fixture | `scripts/verify_write_proto.py` |
| [meta-proto-baseline.txt](./contracts/meta-proto-baseline.txt) | Meta tools not yet proto-routed on both sides | `scripts/verify_meta_proto.py` |
| [message-parity-baseline.txt](./contracts/message-parity-baseline.txt) | Cross-language confirm-message divergences | `scripts/verify_messages.py` |
| [pagination-baseline.txt](./contracts/pagination-baseline.txt) | Tools whose spec route paginates but whose input has no page/page_size yet | `scripts/verify_pagination.py` |
| [enum-sync-baseline.txt](./contracts/enum-sync-baseline.txt) | Enum drift against the Linode OpenAPI spec (network; runs on the sync schedule) | `scripts/verify_sync_enums.py` |
| [api-defaults-baseline.txt](./contracts/api-defaults-baseline.txt) | Snapshot of API wire-body defaults at a reviewed OpenAPI version (network; runs on the sync schedule) | `scripts/verify_sync_defaults.py` |
| [api-pagination-baseline.txt](./contracts/api-pagination-baseline.txt) | Snapshot of paginated GET routes and their page_size bounds at a reviewed OpenAPI version (network; runs on the sync schedule) | `scripts/verify_sync_pagination.py` |

A few cross-language pins live as shared fixtures under `testdata/` rather
than contracts files, because a language's own unit tests consume them:
`testdata/config/parity.yml` (config parsing), `testdata/observability/duration_buckets.json`
(histogram bucket boundaries), and `testdata/audit/event_fields.json` (the
audit JSONL field set). Each language asserts against the same fixture, so a
one-sided edit fails that language's suite.
