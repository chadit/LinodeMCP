# The check gates

`make check` is the whole pre-push gate. CI's one job (`ci.yml`) and the push
hook run exactly that target, so local green, hook green, and CI green are the
same fact. The order lives in the `CHECK_GATES` variable in the root Makefile:
the venv install runs first because most gates import it, and the rest is
ordered cheap-fails-first. Only the network `sync-*` gates sit outside it.

Every gate keeps its own make target. A gate named `x-y` runs
`scripts/verify_x_y.py`; the Makefile says which interpreter (the project venv
when the script imports the Python registry, plain `python3` otherwise), and
the few gates whose script name or arguments break that mapping are called out
in their sections below.

Two shapes of gate exist. **Hard gates** hold their class at zero with no
file: a finding fails by name and the fix ships with the change that caused
it. **Ratchets** track remaining work or accepted divergence in a file under
`docs/contracts/`; entries only leave, and growth needs the dated
issue-annotated acceptance that `baseline-guard` checks. Each contract file's
header comment names its exact rules and regenerate command, and
[docs/README.md](./README.md#machine-contracts) tables every file. That is
also the general update recipe: fix the item, rerun the owning script per its
header, and the line disappears.

## The chain itself

### check

Runs `CHECK_GATES` in order. Change the order or membership there, nowhere
else.

### check-container

Builds `ci/Dockerfile` (which runs `scripts/ci-setup.sh`, the same
provisioning as the CI job) and runs the full gate against a copy of the tree
with the host venv, generated code, and caches excluded. Run it when a change
touches the gate chain or CI.

### fmt-check, scripts-fmt-check, scripts-lint

Formatting and lint for Go, Python, and the gate scripts. Read-only on
purpose: auto-fixing here would hide drift CI still fails on. Generated genpb
is excluded (Go via `GO_FMT_SRC`, Python via the ruff config), so a fresh
regen is never format-gated. The `scripts/` pair uses `scripts/ruff.toml`
(extends `python/pyproject.toml`), which ruff discovers only when run from the
repo root.

### go-build-prod

The hardened build (PIE, trimpath, stripped, static) has link constraints the
dev build lacks; building only dev locally lets a prod-only link failure reach
CI.

### coverage-report

Runs `scripts/report_coverage.py`: one coverage line per registered language,
reporting only, never fails. `coverage-floor` owns pass/fail.

### parity-todo

Runs `scripts/parity_todo.py`: a read-only report of per-language remaining
work from `docs/contracts/languages.txt` and the ratchet baselines. Fails only
when a ratchet it reads is gone, since a missing file reads as no work owed.

## Diff-aware gates

Both take `BASE` (default `origin/main`); an unreachable rev skips loudly, and
CI re-runs each with the event's true base since on main BASE equals HEAD.

### baseline-guard

Runs `scripts/verify_baseline_direction.py`: baseline growth vs BASE must
carry a dated annotation citing a tracking-issue URL. Rides early in the chain
because it needs no build artifacts.

### diff-coverage

Runs `scripts/verify_diff_coverage.py`: source lines added since BASE need
test coverage. Reads the `go/coverage.out` and `python/coverage.json` the test
targets just wrote, so it follows them in the chain.

## Coverage floor

### coverage-floor

Total unit-test coverage per language meets `docs/contracts/coverage-floors.txt`
(rise-only). Parses the `go/coverage.out` that go-check writes, with genpb and
the `cmd/` mains excluded; Python's floor is enforced by pytest
`--cov-fail-under`, so here pyproject and the contract are only checked to
agree. Raise a floor by raising real coverage first.

## Surface parity gates

### tool-parity

Go/Python tool-surface parity: capability, params, required flags, scopes.
Needs the venv. Accepted one-sided tools live annotated in
`docs/contracts/tool-parity-baseline.txt`, which only shrinks: a fixed entry
still listed also fails.

### profile-resolution

Every built-in profile serves the same tools in every language, and every tool
lands in a category some profile can serve; a Write or Destroy tool in no
category fails. Scope from `docs/contracts/languages.txt`; no baseline.

### tool-count

The README's tool count matches `docs/contracts/tools-manifest.txt`. Runs
`scripts/verify_docs_tool_count.py`. The manifest is the source of truth; the
README prose is what drifts.

### dryrun

Every Write, Admin, and Destroy input carries `dry_run` and no Read or Meta
one does. Hard, no baseline; the preview-fixture half ratchets in `behavior`.

### list-envelope

No Python list handler collapses a falsey member with `or []`, which folds
`{}`, `""`, `0`, and `false` into an empty list and ships a malformed response
as a successful empty result while Go rejects it. Hard; scope from
`languages.txt`, and a scan that finds no source files fails too.

### api-surfaces

The surface census (`docs/contracts/api-surfaces.txt`), the proto's
`tool_api_surface`, and the `[<surface>]` marker leading each language's
advertised description agree, checked both directions: a censused tool with no marker fails, and so
does a marker the census does not list.

### env-parity

`docs/contracts/env-vars.txt` pins the whole environment-variable surface;
a variable one language reads and another does not fails here.

### cli-surface

CLI verbs and per-verb flags are extracted from source and diffed across
languages, so a flag cannot land on one CLI without its twin. No baseline.

### metrics-surface

Instrument names and attribute keys match across languages; dashboards and
alerts key on these, so a one-sided rename forks every consumer. Bucket
boundaries pin separately in `testdata/observability/duration_buckets.json`.

### docs-links

Every internal link in README.md and `docs/` resolves. Offline, nothing is
fetched.

## Proto contract gates

### tool-routes

Every non-meta tool declares its Linode route as a `tool_route` option, pinned
against `docs/contracts/tools-manifest.txt` in both directions so neither the
proto nor the manifest drifts alone.

### field-location

Every routed input field declares where it goes. PATH fields must line up with
the route template both ways, and LOCAL must agree with the `// system param`
marker, which is what keeps `dry_run` off the wire.

### tool-capability

The proto's `tool_capability` for every tool matches the manifest tier in
`docs/contracts/tools-capabilities.txt`, plus exactly one of `tool_route` or
`tool_meta`. A capability value with no manifest tier fails first.

### tool-response

The proto says what every tool answers with: response message, confirm prose,
success-text placeholders, and a Destroy's two-stage resource type, failing in
both directions. Hash-ignore keys must agree across every registered language
because an unknown type and a typo look alike there.

### system-params

Every server-injected proto input field carries the trailing `// system param`
marker and is named in `docs/contracts/system-params.txt`, both ways. The
marker is a comment so it never reaches the generated JSON Schema.

### route-evidence

Every declared route is one a client can actually build. The resolvers
(`go/cmd/route-dump`, `scripts/_routescan.py`) walk the call graph out from
the request primitive, so a path built from a base constant and a format verb
still counts. Hard; a language not caught up records an annotated absence in
`tool-parity-baseline.txt` instead.

## Proto-routing gates (venv)

### write-proto, read-proto, meta-proto

Static classification of every handler by capability: success output must
route through proto on both sides, and a `*WriteResponse` proto with no
conformance fixture fails too. Hard, no baseline.

### input-proto

Tool input schemas are proto-generated; a hand-built schema fails, which is
what keeps every language advertising one contract. Hard, no baseline.

### behavior

Fixture coverage of the tool surface; correctness lives in the two test
runners replaying `testdata/behavior/`. Coverage and the malformed-response
rule are hard (a manifest tool with no fixture fails unless
`docs/contracts/behavior-exempt.txt` names it with a reason); the dry-run
preview case still ratchets in `docs/contracts/behavior-dryrun-baseline.txt`.

### messages

The emitter renders the contract's `confirm_message` into every language's
tool tree, and this diffs those renderings over every extractable gate,
including branches no fixture exercises. Hard, any divergence fails.

## Migration ratchets

### route-source

Counts request call sites that still build their endpoint by hand.
`docs/contracts/route-source-counts.txt` only falls, and a removed call site
whose line was not lowered fails too, which keeps the file honest.

### generated-tools

The generator owns its cohort: a hand-written factory left behind for a cohort
tool fails, since both languages register by scanning and the leftover stages
the tool twice. Requires `make proto` because it reads the emitted trees.
`docs/contracts/generated-tools-counts.txt` only falls.

### hand-validators

Counts hand-written argument checks; a tool whose `*Input` message declares
buf.validate rules has its check read from the contract by both languages.
`docs/contracts/hand-validator-counts.txt` only falls.

### hook-bodies

Counts the handler steps each language still writes by hand, per `tool_hooks`
kind, against the kinds the contract declares. A body no tool declares and a
declared kind no tree implements both fail by name.
`docs/contracts/hook-body-counts.txt` only falls, and its header records why
each kind stays hand-written. `validate` is left to `hand-validators` so one
population is not counted twice.

## Spec-snapshot gates

Offline and hard, judged against snapshots the sync gates own, so a
wrong-shaped fixture fails rather than teaching every language a contract the
API never had.

### pagination

A tool whose spec route paginates needs `page`/`page_size` on its proto input.
Snapshot: `docs/contracts/api-pagination-baseline.txt` (owned by
`sync-pagination`).

### response-shapes

Behavior fixtures serve each route's spec response shape. Snapshot:
`docs/contracts/api-response-shapes-baseline.txt` (owned by
`sync-response-shapes`).

## Tooling and security gates

### tool-float

Gate tooling floats at latest while app deps pin: an offline line scan over
pyproject's dev group, the Makefiles, `scripts/ci-setup.sh`, and workflow run
commands. A pinned gate tool fails unless the script's deliberate-pin
allowlist carries a reasoned entry (only buf today).

### actionlint

Lints the workflow files with an unconditional `go run ...@latest`, because a
prefer-local-binary fallback ages out of sync with what CI fetches exactly
when a new check lands. Files are passed explicitly since bare actionlint
locates the project by looking for `.git`, which a tarball checkout lacks.

### betterleaks

Secrets scan, and a hard requirement rather than skip-if-missing: a warn-skip
meant machines without the binary passed a scan CI ran. `--redact` keeps
secret values out of terminals and logs; `--regex-engine=stdlib` matches what
CI forces (the WASM engine trips betterleaks#74 there). The file list comes
from git because betterleaks has no gitignore awareness, with an existence
filter dropping index entries deleted from the working tree, which it aborts
on. `--config` is explicit because auto-discovery keys off a directory target
and never fires for a file list. Allowlist: `.betterleaks.toml`.

### trivy

Vulnerability and misconfig scan, hard requirement for the same false-green
reason. All severities on purpose: accepted findings live annotated in
`.trivyignore.yaml`, and the outside lint also scans unfiltered, so a severity
filter here would let the two disagree on the same tree. Skip flags derive
from git's ignore computation and use the `=`-attached form so each survives
word splitting.

## Network sync gates (scheduled only)

`make sync` runs all of them; `sync-drift.yml` runs it weekly. They need the
network, which is why they stay out of `check`: the offline gates prove both
languages agree with each other, these prove that agreement still matches the
live Linode API spec.

### sync-enums

Proto enums vs the live spec and changelog. Drift a human has reconciled is
recorded in `docs/contracts/enum-sync-baseline.txt` via `--update-baseline`.

### sync-defaults

Wire-body defaults vs the live spec. Snapshot:
`docs/contracts/api-defaults-baseline.txt`.

### sync-pagination

The live spec's paginated-route set and page-size bounds vs the snapshot the
offline `pagination` gate judges against.

### sync-response-shapes

The live spec's route response shapes vs the snapshot the offline
`response-shapes` gate judges against.

### sync-scopes

Per-tool OAuth scopes vs the spec's per-operation security blocks. Needs the
venv, unlike the other sync gates. Deviations live annotated in
`docs/contracts/scope-sync-baseline.txt`, structural ones in
`docs/contracts/scope-sync-exempt.txt`. A route the spec documents no
operation for is skipped, not failed: the spec lags techdocs, and
`route-evidence` already proves offline that the route is real.

### sync-issues

Runs `scripts/verify_tracking_issues.py`: every baseline acceptance still
cites an open tracking issue. `baseline-guard` only checks that an annotation
looks like an issue URL, which a closed issue satisfies forever; this resolves
each one via `gh` and skips loudly when `gh` is absent.
