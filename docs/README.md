# LinodeMCP documentation

The map of everything under `docs/`. Two kinds of file live here: **pages**
(`.md`) are prose for people and agents, one topic each; **machine contracts**
(under [`contracts/`](./contracts/), text apart from one pinned binary image)
are read by the `make check` gates by exact path and tabled below so nobody
mistakes them for reading material. The repo root carries an `llms.txt` with
this same map, one line per page.

New here? Read the [root README](../README.md) for install and first run, then
[profiles](./profiles.md), [dry-run](./dry-run.md), and
[two-stage writes](./two-stage-writes.md) for the safety model.

## Using the server

- [Profiles](./profiles.md): the permission model. Which tools the AI client can
  see and call, plus capability tags, the built-in catalog, config schema,
  token-scope validation, hot-reload, and copy-paste recipes.
- [Dry-run, bypass-confirm, pre-check, and yolo](./dry-run.md): preview any
  mutator before it runs. Destructive calls must be previewed or waived.
- [Two-stage writes](./two-stage-writes.md): plan a destructive call, review it,
  apply it by id. Covers drift and how to read each refusal.
- [Object Storage data plane](./object-storage-data-plane.md): moving object
  bytes through a presigned URL, both directions, no access key.
- [Audit](./audit.md): the record of every tool invocation. Event schema,
  redaction, query tools, sinks, retention, recovery, report grammar.
- [Host integrations](./host-integrations/README.md): registering the server with
  each MCP host, plus per-topic wrappers for Claude Code and Claude Desktop
  ([profiles](./host-integrations/profiles.md),
  [audit](./host-integrations/audit.md),
  [two-stage](./host-integrations/two-stage.md)).
- [CLI and server modes](./cli.md): bare invocation is the MCP stdio server; the
  verbs run as shell commands and exit.
- [Observability](./observability.md): Prometheus metrics on :8888, health
  endpoints on :8889, OTLP tracing, and which probe goes where.

## Releases

- [Release process](./release-process.md): the maintainer runbook. The two
  workflows that cut a release, and everything a release ships.
- [Verifying releases](./verifying-releases.md): copy-paste commands for
  checksums, container signatures, SBOMs, and SLSA provenance.

## Contributing

- [Cross-language parity](./parity.md): the day-to-day playbook. Adding,
  changing, or removing a tool; landing one language first with a tracked
  absence; the commands.
- [Adding a language](./adding-a-language.md): the checklist for a new
  implementation. Everything derives from the proto contract.
- [The check gates](./gates.md): every `make check` gate as one table row, plus
  the network sync gates.
- [Refreshing dependency versions](./dependency-updates.md): what
  `make update-deps` moves, the pins it refuses, and how it coexists with
  Renovate.
- [Git hooks](./git-hooks.md): `make install-hooks` and what the hooks run.
- [Deprecated routes](./deprecated-routes.md): removed tools and routes, with
  the replacement to use instead.
- [TechDocs comparator](../tools/techdocs-proof/README.md): the tool project that
  compares the proto contract against the rendered Linode TechDocs site. Its
  self-test is a `make check` gate; the scrape runs on a schedule.

## Machine contracts

Gate-consumed files, all under `docs/contracts/`. `make check` reads every one
by exact path; never edit their entry lines by hand. Each file's header comment
holds its full rules and exact regenerate command. Baselines are ratchets:
fixing an item removes its line, lines are never added by hand, and an accepted
line carries a dated annotation citing a tracking-issue URL that
`make sync-issues` resolves on the sync schedule (the two `*-exempt.txt` files
may use a free-text reason instead). Most gates carry no file at all; see
[hard gates](./gates.md#hard-gates-carry-no-file).

### Registries

| File | Pins | Consumed by |
|------|------|-------------|
| tools-manifest.txt | The full tool surface, one name per line. Generated from the `tool_route` and `tool_meta` options by `make proto` and gitignored | Manifest gate tests in each language |
| tools-capabilities.txt | Capability tier (`Read`/`Write`/`Destroy`/`Admin`/`Meta`) for every tool. Generated from the `tool_capability` options by `make proto` and gitignored | Capability gate tests in each language |
| [handwritten-tools.txt](./contracts/handwritten-tools.txt) | The tools each language still serves from a hand-written factory, which is the codegen cohort inverted: everything else is generated, so new surface is born generated (shrink-only) | `scripts/verify_generated_tools.py`, `go/cmd/toolgen` |
| [languages.txt](./contracts/languages.txt) | The registered language implementations: name, working dir, surface-dump command. `make proto` emits one tool tree per language listed here | `Makefile`, `go/cmd/toolgen`, `scripts/verify_tool_parity.py` |
| [env-vars.txt](./contracts/env-vars.txt) | The complete environment-variable surface every language reads (observability has none by design) | `scripts/verify_env_parity.py` |
| [coverage-floors.txt](./contracts/coverage-floors.txt) | Minimum total unit-test statement coverage per registered language (rise-only; the per-line half is `make diff-coverage`) | `scripts/verify_coverage_floor.py` |
| [route-source-counts.txt](./contracts/route-source-counts.txt) | Request call sites per registered language that still build their endpoint by hand instead of resolving it from the proto (fall-only; what is left of the route-builder migration) | `scripts/verify_route_source.py` |
| [generated-tools-counts.txt](./contracts/generated-tools-counts.txt) | Tools per registered language still served by a hand-written factory rather than by the tree the emitter writes for it from the proto (fall-only; what is left of the codegen migration) | `scripts/verify_generated_tools.py` |
| [hand-validator-counts.txt](./contracts/hand-validator-counts.txt) | Argument checks per registered language still written out by hand rather than declared on the tool's `*Input` message (fall-only; the population `hand-code` cannot see, since a check is named after what it checks rather than after a tool) | `scripts/verify_hand_validators.py` |
| [system-params.txt](./contracts/system-params.txt) | The proto input fields the server consumes itself rather than passing to the Linode API, by field name and proto type; each one carries a trailing `// system param` marker that stays out of the generated schema | `scripts/verify_system_params.py` |
| [api-surfaces.txt](./contracts/api-surfaces.txt) | Every tool that answers on an API surface other than `/v4`, one `<tool> <surface>` line each. Hand-maintained, and it grows and shrinks with what Linode ships on beta rather than ratcheting | `scripts/verify_api_surfaces.py` |

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
| [api-techdocs-routes-baseline.txt](./contracts/api-techdocs-routes-baseline.txt) | Snapshot of every route the rendered TechDocs state, with its API surface and deprecation status. `make techdocs-routes` gates `proto/` against it offline; the weekly comparator run writes the refresh candidate (network; runs on the techdocs-drift schedule) | `tools/techdocs-proof` (`--emit-route-snapshot`), read by `scripts/verify_techdocs_routes.py` |
| [wire-baseline.binpb.gz](./contracts/wire-baseline.binpb.gz) | The proto tree's wire shape at a reviewed revision, as a `buf build --exclude-source-info` image. `make wire-breaking` compares `proto/` against it offline, so a renumbered, retyped, or reused field number fails by name. The only binary contract here, and it refreshes in the same reviewed change as any field retirement, because `buf breaking` cannot tell a retirement from a careless drop | `buf breaking`, via `make wire-breaking` |

### Shared fixtures

A few cross-language pins live under `testdata/` rather than in contract files,
because a language's own unit tests consume them: `testdata/config/parity.yml`
(config parsing), `testdata/observability/duration_buckets.json` (histogram
bucket boundaries), `testdata/audit/event_fields.json` (the audit JSONL field
set), and `testdata/audit/record-bytes` (the bytes each export format writes).
Each language asserts against the same fixture, so a one-sided edit fails that
language's suite.
