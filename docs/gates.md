# The check gates

`make check` is the whole pre-push gate. CI's one job (`ci.yml`) and the push
hook run exactly that target, so local green, hook green, and CI green are the
same fact. Membership and order live in `CHECK_GATES` in the root Makefile and
nowhere else: `proto` runs first because everything reads its output, the venv
is a prerequisite of the proto stamp so a fresh checkout provisions itself, and
the rest is ordered cheap-fails-first. Only the network `sync-*` gates sit
outside the list, along with `update-deps`, an action target rather than a gate
(see [dependency-updates.md](./dependency-updates.md)).

A gate named `x-y` runs `scripts/verify_x_y.py`, and each script's module
docstring is the long form of its row below: what it scans, how it decides, the
ways it could pass while measuring nothing, and what it cannot see. The few
gates that break the name mapping say so in the Reads column.

Two shapes of gate exist. **Hard gates** hold their class at zero with no file:
a finding fails by name and the fix ships with the change that caused it.
**Ratchets** track remaining work or accepted divergence in a file under
`docs/contracts/`; entries only leave, and growth needs the dated
issue-annotated acceptance that `baseline-guard` checks. Each contract file's
header comment names its exact rules and regenerate command, and
[docs/README.md](./README.md#machine-contracts) tables every file. That is also
the general update recipe: fix the item, rerun the owning script per its header,
and the line disappears.

## The `make check` gates

In `CHECK_GATES` order.

| Gate | What it holds | Reads | A red means |
|---|---|---|---|
| `proto` | Every derived tree is regenerated from the contract: `buf generate` writes the typed messages and strict input schemas, `scripts/gen_tool_registries.py` the registries, `go/cmd/toolgen` each registered language's tool tree | `proto/`, stamped on `.make/proto-generated` | The contract does not compile, or an emitter refused an under-declared tool by name |
| `proto-lint` | No `buf lint` finding on the source every language is generated from | `buf.yaml` modules; rules and per-path exemptions live there too | A lint finding on the contract |
| `overlay-roundtrip` | Splitting each declared proto file into the upstream surface and the repo-owned overlay, rendering the pair back, and byte-comparing | `buf.yaml`, `proto/`; logic is `go/cmd/protomerge`, not a script | A file whose bytes moved, or a declaration written in a shape the model cannot re-render |
| `overlay-merge` | The tree merged from an upstream surface descriptor plus the overlay is byte-identical to `proto/` | `proto/` plus a descriptor written per run to a scratch dir | An overlay anchor the descriptor stopped carrying, or upstream surface no overlay entry claims |
| `wire-breaking` | `buf breaking` against a pinned image, so a renumber, a rename on a live number, or a deletion fails by name and line | `docs/contracts/wire-baseline.binpb.gz` | A wire change that breaks every client already speaking the contract |
| `techdocs-routes` | Every declared `tool_route` still exists upstream, on the API surface it declares | `docs/contracts/api-techdocs-routes-baseline.txt` | Upstream dropped the route, or moved it between v4 and v4beta |
| `python-install-dev` | The Python package and dev tools land in `python/.venv` for every later gate that runs a venv binary | `python/pyproject.toml`, `python/uv.lock` | The venv cannot be provisioned, so nothing downstream can run |
| `fmt-check` | Go, Python, `scripts/`, and `tools/` formatting, read-only on purpose: auto-fixing here would hide drift CI still fails on | `GO_FMT_SRC` and the ruff configs; generated genpb is excluded | Unformatted source |
| `scripts-lint` | ruff over the gate scripts | `scripts/ruff.toml`, discovered only from the repo root | A lint finding in a gate script |
| `tools-lint` | ruff over the tool projects that ship with neither language package | each `tools/` project's own pyproject | A lint finding in a tool project |
| `techdocs-proof` | The offline arm of the TechDocs comparator: a replay over frozen descriptor fixtures, the exclusion ledger's own rules, and the refusal of a run root inside the repository | `tools/techdocs-proof/data/known-divergences.json`; stdlib only, so no venv | A replay mismatch, a duplicate or incomplete ledger entry, a ledger kind the comparison cannot raise, or a comparison that would dirty the tree it measures |
| `actionlint` | The workflow files, via an unconditional `go run ...@latest` so the local run matches what CI fetches | `.github/workflows/*.yml`, passed explicitly | A workflow finding |
| `dockerfiles` | droast at error severity over every Dockerfile the repo tracks, found by name so a new image is in scope the moment its file exists | git's tracked and untracked-not-ignored file list | A droast error, folded onto its own file and line |
| `tools-typecheck` | mypy over each project under `tools/`, at that project's own `requires-python` | each `tools/` project's pyproject | A type error, a `tools/` holding no project, or a run that reports no checked-file count |
| `strict-typecheck` | basedpyright in strict mode over every Python tree the repo owns at once, including the `scripts/` gates that pyright and mypy between them never reached | `pyrightconfig.json` `include`, the only place the scanned surface is declared | A type error, an `include` entry that is not on disk, or a run that reports no analyzed files |
| `baseline-guard` | Baseline growth against `BASE` carries a dated annotation citing a tracking-issue URL | the ratchet files, diffed against `BASE` (default `origin/main`) | An added baseline line with no issue URL behind it |
| `tool-float` | Gate tooling floats at latest while app dependencies pin | pyproject's dev group, the Makefiles, `scripts/ci-setup.sh`, workflow run lines | A pinned or capped gate tool with no reasoned allowlist entry (only buf today) |
| `go-check` | `make -C go check`: Go lint (golangci-lint plus the security and convention scanners) and the test suite | `go/`; writes the `go/coverage.out` two later gates read | A Go lint finding or a failing test |
| `go-analyzers` | gopls' analyzer set (modernize and friends) over every registered Go tree, since gopls releases ahead of the x/tools tags golangci-lint depends on | `docs/contracts/languages.txt`, each entry carrying a go.mod | Any gopls output at all: it exits zero even when it reports |
| `python-check` | `make -C python check`: uv lock-check, ruff, mypy, pyright, and pytest with coverage | `python/`; writes `python/coverage.json`, and pytest's `--cov-fail-under` enforces the Python floor | A Python lint, type, or test failure |
| `coverage-floor` | Total unit-test coverage per language meets the floor (rise-only) | `docs/contracts/coverage-floors.txt`, `go/coverage.out` with genpb and `cmd/` mains excluded | Coverage under the floor, or pyproject and the contract disagreeing on Python's |
| `diff-coverage` | Source lines added since `BASE` are covered by tests | `go/coverage.out`, `python/coverage.json` | An added line no test reaches |
| `tool-parity` | Go/Python tool-surface parity: capability, params, required flags, scopes | `docs/contracts/tool-parity-baseline.txt` (shrink-only); needs the venv | A one-sided tool the baseline does not annotate, or a fixed entry still listed |
| `profile-resolution` | Every built-in profile serves the same tools in every language, and every tool lands in a category some profile can serve | `docs/contracts/languages.txt`; no baseline | A profile that diverges, or a Write or Destroy tool in no category |
| `scope-spellings` | Every language's token-side Scope catalog spells what the toolgen emitter spells, and the catalogs carry one value set | `docs/contracts/languages.txt`; no baseline | A catalog value that is no emitter wire spelling or family sibling, which the grants converter would otherwise report as a missing scope |
| `tool-count` | The README's tool count matches the manifest | `docs/contracts/tools-manifest.txt` and `README.md`; runs `scripts/verify_docs_tool_count.py` | README prose drifted from the manifest, which is the source of truth |
| `dryrun` | Every Write, Admin, and Destroy input carries `dry_run` and no Read or Meta one does | the compiled descriptors; hard, no baseline | A missing or misplaced `dry_run` (the preview-fixture half ratchets in `behavior`) |
| `pagination` | A tool whose route paginates in the mirror carries `page` and `page_size` on its proto input | `docs/contracts/api-pagination-baseline.txt`, owned by `sync-pagination` | A paginated route whose tool cannot page |
| `response-shapes` | Behavior fixtures serve each route's response shape as the mirror records it | `docs/contracts/api-response-shapes-baseline.txt`, owned by `sync-response-shapes` | A fixture shaped unlike what the API answers with |
| `list-envelope` | No Python list handler collapses a falsey member with `or []` | `docs/contracts/languages.txt`; hard | An `or []` fold that ships a malformed response as a successful empty result while Go rejects it, or a scan that found no source files |
| `tool-routes` | Every non-meta tool declares its Linode route as a `tool_route` option, and every Meta tool carries none | `docs/contracts/tools-manifest.txt`, pinned in both directions | A tool with no route, a Meta tool with one, or a proto and manifest that disagree |
| `api-surfaces` | The surface census, the proto's `tool_api_surface`, and the `[<surface>]` marker leading each language's advertised description agree | `docs/contracts/api-surfaces.txt`, checked both directions | A censused tool with no marker, or a marker the census does not list |
| `field-location` | Every routed input field declares where it goes; PATH lines up with the route template both ways and LOCAL agrees with the `// system param` marker | the compiled descriptors | A field with no location, or one whose location contradicts its route (this is what keeps `dry_run` off the wire) |
| `tool-response` | The proto says what every tool answers with: response message, confirm and success and error prose, description, a Destroy's resource type, retry policy | the compiled descriptors, plus every language's hash-ignore table | An under-declared answer, a placeholder naming no field, or hash-ignore keys that disagree across languages |
| `route-evidence` | Every declared route is one a client can actually build, resolved by walking the call graph out from the request primitive | `go/cmd/route-dump` and `scripts/_routescan.py`; hard | A route no call site resolves, or a message the contract does not know (a language not caught up records an annotated absence in `tool-parity-baseline.txt` instead) |
| `route-source` | Request call sites that still build their endpoint by hand | `docs/contracts/route-source-counts.txt` (fall-only) | The count rose, or a removed call site whose line was not lowered |
| `generated-tools` | The generator owns its cohort: a hand-written factory left behind for a cohort tool stages that tool twice | `docs/contracts/generated-tools-counts.txt` (fall-only), `handwritten-tools.txt`; needs `make proto` | A leftover factory beside the emitted tree |
| `hand-validators` | Hand-written argument checks, the population `hand-code` cannot see since a check is named after what it checks | `docs/contracts/hand-validator-counts.txt` (fall-only) | A count over its line, with every site that pushed it there named |
| `hand-code` | No hand-coded tool code beside the generated tree, through a definition arm (a function named after a tool) and a literal arm (a quoted string spelling a tool name as a whole word) | `docs/contracts/languages.txt` less the trees `.gitignore` marks regenerated, less tests | Code the contract has no declaration for; the fix is a declaration on the tool's `*Input` message, never a line in a contract file |
| `hand-arms` | No hand-written local-operation arm beside the generated tree, matched on each language's own arm spelling (`RunCatalogCanRun`, `run_catalog_can_run`) | the same trees `hand-code` scans | A function named after a `LocalCall` member in hand-written source; the fix is a subsystem method behind the generated interface |
| `system-params` | Every server-injected proto input field carries the trailing `// system param` marker and is named in the contract, both ways | `docs/contracts/system-params.txt` | An unmarked injected field, or a contract line no field carries |
| `env-parity` | The whole environment-variable surface, read the same way by every language | `docs/contracts/env-vars.txt` | A variable one language reads and another does not |
| `cli-surface` | CLI verbs and per-verb flags, extracted from source and diffed across languages | each registered language's CLI source; no baseline | A flag that landed on one CLI without its twin |
| `docs-links` | Every internal link in `README.md`, `llms.txt`, `docs/`, and the prose tool projects ship under `tools/` resolves | those files, offline; nothing is fetched | A link target that does not exist on disk |
| `metrics-surface` | Instrument names and attribute keys match across languages, since dashboards and alerts key on them | each language's instrument registration; buckets pin separately in `testdata/observability/duration_buckets.json` | A one-sided rename that would fork every consumer |
| `generated-form` | One classification pass per tool surface (the merged `write-proto`, `read-proto`, `meta-proto`, and `input-proto` gates): handler success paths route through proto on both sides, and every advertised input schema is proto-generated | the descriptors and each language's tree; hard, no baseline | A hand-built wire shape or input schema, or a `*WriteResponse` proto with no conformance fixture; each surface reports its own OK line, so a regression names its slice |
| `behavior` | Fixture coverage of the tool surface; correctness itself lives in the two runners replaying `testdata/behavior/` | `docs/contracts/behavior-exempt.txt` (reasons) and `behavior-dryrun-baseline.txt` (ratchet) | A manifest tool with no fixture and no exemption, or a malformed-response rule break |
| `messages` | The emitter's rendering of the contract's `confirm_message` into every language's tool tree, diffed over every extractable gate including branches no fixture exercises | the generated trees; hard | Any divergence at all |
| `betterleaks` | Secrets scan over git's file list, a hard requirement rather than skip-if-missing | `.betterleaks.toml` | A secret in a tracked or untracked-not-ignored file |
| `trivy` | Vulnerability and misconfig scan at every severity | `.trivyignore.yaml`, `trivy.yaml` | An unaccepted finding |
| `build` | Both languages' dev builds into each language's `bin/`, near the end so a cheap gate fails first | `go/`, `python/` | A build failure |
| `go-build-prod` | The hardened Go build (PIE, trimpath, stripped, static) | `go/` | A link constraint the dev build lacks, which would otherwise reach CI |

## Hard gates carry no file

Most gates hold their class at zero and have nothing to accept a finding into:
`generated-form`, `behavior` coverage and its malformed-response rule,
`messages`, `pagination`, `response-shapes`, `list-envelope`, `route-evidence`,
`tool-routes`, `field-location`, `tool-response`, `dryrun`, `env-parity`,
`cli-surface`, `metrics-surface`, and `system-params`.

An empty baseline file used to say the same thing, and it also proved the gate
still had a surface to look at. Nothing carries that second meaning now, so each
hard gate reports what it measured and fails when that count is zero: a
classifier that stops resolving handlers, a scan whose tree moved, or a snapshot
that stopped parsing shows up as a failure instead of a clean run.

## What `hand-code` cannot see

Three holes, stated here rather than papered over with an exemption list, which
is the one thing that could make the gate lie:

- A tool named by a single ordinary word (`version`, `hello`) is matched by
  neither arm. `version` prefixes six legitimate definitions in this tree and is
  the JSON key of every version answer.
- An argument check is named after what it checks rather than after a tool, so
  neither arm can see one. `hand-validator-counts.txt` holds that population to
  zero and this gate reads the file.
- The literal arm reads one line at a time, and a string only on the line that
  opens and closes it. A tool name built at run time, split across two adjacent
  or `+`-joined literals, inside a Go raw string or a Python triple-quoted string
  that spans lines, or reached through nothing but a resource type all stay
  invisible, while a comment or docstring quoting a tool name reads as a literal.

`hand-code` also names `go/internal/cli`, `python/src/linodemcp/cli`, and
`python/src/linodemcp/tui` in `_TOOL_CALLERS`, because their job is to call tools
by name through the same dispatcher a client uses. That is a statement of scope
rather than an exemption: it names no tool, and a function named after a tool
inside those trees still fails the definition arm.

## Notes the table cannot hold

- **`overlay-roundtrip`**: wire numbers and declaration order live only in the
  overlay, so upstream drift cannot renumber anything. The gate cannot see a
  reused number on its own, because nothing left in the tree says what a number
  used to mean; the `reserved` statement `overlay-merge` writes on retirement is
  what says it, and protoc refuses the reuse.
- **`overlay-merge`**: both refusals fire at three levels (fields, enum values,
  routes), and a rename fires both, since the merger cannot tell a rename from a
  drop plus an add. Nothing is written when either fires, so a refused merge
  leaves no half-merged tree. `-retire Message.field` drops the declaration and
  its prose and leaves `reserved <number>;` and `reserved "<name>";` behind;
  message-level `buf.validate` CEL rules naming the retired field are left alone
  on purpose, and `proto-lint` names each one by line. A `preview_sentence` that
  guards on or reports the retired argument survives the same way, but
  `proto-lint` cannot see it: `toolgen` is what refuses, with "tool declares a
  preview_sentence reading an argument the message does not declare", so a
  retirement takes those out by hand too and only `make proto` says so.
- **`wire-breaking`**: the baseline is a pinned image rather than a branch
  reference, because a checkout compared against its own last commit measures
  nothing. `use: FILE` treats every field deletion as breaking whether or not the
  number retires, which is the right split: this gate catches the change at the
  moment it is introduced, and the `reserved` statement carries the retirement
  forever. Refresh it inside the reviewed change that earns it:
  `buf build --exclude-source-info -o docs/contracts/wire-baseline.binpb.gz`.
  Retyping a field on the same number is a third shape the failure text does not
  name, and it is answerable only for an Input message: those never travel as
  protobuf binary between peers, because MCP hands arguments over as JSON keyed
  by field name and the generated types back descriptor reads and schema
  generation. Retiring the field instead would leave `reserved "<name>";` behind
  and lock out the argument name the published docs pin, which is the one thing a
  correction to a wrong type has to keep. Deleting a whole message is a fourth
  shape: retiring a tool takes its input message with it, and proto3 has no way
  to reserve a deleted top-level message name, so nothing stops a later change
  reusing that name for something unrelated. The baseline catches the reuse only
  if the shape differs, which is why the deletion belongs in the record beside
  the replacement. Deleting an enum value is a fifth, and `reserved <number>;`
  inside the enum documents the retirement without keeping the gate quiet: the
  finding still fires and the refresh is still owed. Say which of the five
  shapes a change took, since the baseline refresh looks the same whichever it
  is. Refresh on an addition too: adding a field keeps the gate green against a
  baseline that has never seen the number, so until the next refresh nothing here
  would catch a later change deleting or retyping it. Retiring an extension field spends two
  FILE-category findings at `options.proto:1:1` rather than one, because the
  message that carried the extension's value goes with it. `reserved` is a parse
  error inside an `extend` block, so a retired extension number is held by a
  comment where the field stood; `50012` and `50030` are the two the contract has
  spent so far.
- **`betterleaks` flags**: `--redact` keeps secret values out of terminals and
  logs, `--regex-engine=stdlib` matches what CI forces (the WASM engine trips
  betterleaks#74 there), the file list comes from git because betterleaks has no
  gitignore awareness (with an existence filter dropping index entries deleted
  from the working tree, which it aborts on), and `--config` is explicit because
  auto-discovery keys off a directory target and never fires for a file list.
- **`trivy` severities**: all of them on purpose. Accepted findings live
  annotated in `.trivyignore.yaml` and the outside lint also scans unfiltered, so
  a severity filter here would let the two disagree on the same tree. Skip flags
  derive from git's ignore computation and use the `=`-attached form so each
  survives word splitting.

## Report-only targets

Outside `CHECK_GATES`, offline, and never a source of red on their own.

| Target | What it does |
|---|---|
| `check-container` | Builds `ci/Dockerfile` (the same provisioning as the CI job) and runs the full gate against a copy of the tree with the host venv, generated code, and caches excluded. Run it when a change touches the gate chain or CI |
| `coverage-report` | One coverage line per registered language. `coverage-floor` owns pass/fail |
| `parity-todo` | Per-language remaining work from `docs/contracts/languages.txt` and the ratchets. Fails only when a ratchet it reads is gone, since a missing file reads as no work owed |

## Network sync gates (scheduled only)

`make sync` runs all of them and `sync-drift.yml` runs it weekly. They need the
network, which is why they stay out of `check`: the offline gates prove both
languages agree with each other, these prove that agreement still matches the
Linode OpenAPI mirror at `linode/linode-api-openapi`.

TechDocs is the API contract's authority and that mirror is secondary: it
updates less often and may be deprecated. So a sync finding a TechDocs page
contradicts is a stale mirror, not a repo defect. Leave `proto/` alone, record
what the two sources say, and refresh that gate's snapshot with
`--update-baseline` in a reviewed change so the next run is quiet. A sync
finding TechDocs agrees with is real drift and `proto/` is what moves. The two
offline gates fed by these snapshots, `pagination` and `response-shapes`, sit
under the same rule: neither takes a baseline acceptance, so a snapshot refresh
is the only exit when the mirror is the side that is wrong. The comparator
states the same boundary from its own side in
[chapter 01](../tools/techdocs-proof/docs/01-boundaries-and-authority.md).

| Gate | What it compares | Baseline |
|---|---|---|
| `sync-enums` | Proto enums against the live mirror and its changelog | `docs/contracts/enum-sync-baseline.txt`, written with `--update-baseline` |
| `sync-defaults` | Wire-body defaults against the live mirror | `docs/contracts/api-defaults-baseline.txt` |
| `sync-pagination` | The mirror's paginated-route set and page-size bounds against the snapshot the offline `pagination` gate judges by | `docs/contracts/api-pagination-baseline.txt` |
| `sync-response-shapes` | The mirror's route response shapes against the snapshot `response-shapes` judges by | `docs/contracts/api-response-shapes-baseline.txt` |
| `sync-scopes` | The contract's declared per-tool OAuth scopes, as Python renders them, against the mirror's per-operation security blocks. Needs the venv, unlike the others. A route the mirror documents no operation for is skipped rather than failed, since the mirror lags TechDocs and `route-evidence` already proves the route is real | `docs/contracts/scope-sync-baseline.txt`, structural deviations in `scope-sync-exempt.txt` |
| `sync-issues` | Every baseline acceptance still cites an open tracking issue, resolved through `gh`. `baseline-guard` only checks that an annotation looks like an issue URL, which a closed issue satisfies forever | none; skips loudly when `gh` is absent |

`techdocs-drift.yml` is the same shape for the other upstream: it scrapes
rendered TechDocs, compares them against the checked-out proto tree, and uploads
the run directory as an artifact. It reads only and never commits. It passes
`--fail-on-findings`, so a finding at medium or high fails the weekly job while
known, limitation and info never do. One thing that gate cannot reach: a ledger
entry that stopped matching raises no finding, so it lands in
`known_divergences_unmatched` and the job summary tables it instead, with the
readings such a line can carry written out in
[operations](../tools/techdocs-proof/docs/08-operations-and-debugging.md). It
also carries the weekly refresh for `techdocs-routes`: the run writes a candidate
`route-snapshot.txt` into its evidence, the job prints the diff against
`docs/contracts/api-techdocs-routes-baseline.txt` in its summary, and a human
copies the candidate over the reviewed file, because REQ-D5 puts every repository
change behind a reviewed diff rather than a bot commit. By hand from a run:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof \
  --techdocs-contract <run>/techdocs-contracts.json \
  --emit-route-snapshot docs/contracts/api-techdocs-routes-baseline.txt
```
