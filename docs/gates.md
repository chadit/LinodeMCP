# The check gates

`make check` is the whole pre-push gate. CI's one job (`ci.yml`) and the push
hook run exactly that target, so local green, hook green, and CI green are the
same fact. The order lives in the `CHECK_GATES` variable in the root Makefile:
`proto` runs first, and the venv is a prerequisite of its regen stamp (the
generators run venv binaries), so a fresh checkout provisions itself in order;
the rest is ordered cheap-fails-first. Only the network `sync-*` gates sit
outside it, along with `update-deps`, which is an action target rather than a
gate: it needs the network and it writes, so nothing `check` resolves reaches
it (see [dependency-updates.md](./dependency-updates.md)).

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

### proto

First in `CHECK_GATES` because everything else reads its output: `buf generate`
writes the typed messages and strict input schemas,
`scripts/gen_tool_registries.py` writes the tool registries, and
`go/cmd/toolgen` writes every registered language's tool tree, all from
`proto/`. Stamp-gated on `.make/proto-generated`, so an unchanged contract
regenerates nothing; the venv is a prerequisite of the stamp, which is what
lets a fresh checkout provision itself in order.

### python-install-dev

Installs the Python package and dev tools into `python/.venv` with uv. Sits
right after `proto` so every later gate that runs a venv binary or imports the
Python registry finds them (on a fresh checkout the proto stamp's venv
prerequisite has usually installed them already).

### fmt-check, scripts-fmt-check, scripts-lint, tools-fmt-check, tools-lint

Formatting and lint for Go, Python, and the gate scripts. Read-only on
purpose: auto-fixing here would hide drift CI still fails on. Generated genpb
is excluded (Go via `GO_FMT_SRC`, Python via the ruff config), so a fresh
regen is never format-gated. The `scripts/` pair uses `scripts/ruff.toml`
(extends `python/pyproject.toml`), which ruff discovers only when run from the
repo root. The `tools/` pair covers the tool projects that ship with neither
language package; each carries its own pyproject and ruff discovers it per
file, so one invocation covers whatever `tools/` grows to hold.

### go-check

`make -C go check`: Go lint (golangci-lint plus the security and convention
scanners) and the test suite. The test run writes `go/coverage.out`, which
`coverage-floor` and `diff-coverage` read later in the chain.

### python-check

`make -C python check`: uv lock-check, ruff, mypy, pyright, and pytest with
coverage. pytest's `--cov-fail-under` enforces the Python floor, and the run
writes `python/coverage.json` for the diff-aware gates.

### build

Both languages' dev builds into each language's `bin/`, near the end of the
chain so a cheap gate fails before the slower builds. The hardened Go variant
is `go-build-prod`.

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

### scope-spellings

Every language's token-side Scope catalog spells what the toolgen emitter
spells: each non-wildcard value must be an emitter wire spelling or a
read_only/read_write sibling of an emitter family, and the catalogs must carry
one value set across languages. Catches the drift the grants converter would
otherwise report silently as a missing scope. Scope from
`docs/contracts/languages.txt`; no baseline.

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
still counts. A call site that names its tool to a route primitive resolves
through the contract, and so does one that hands a generated `*Input` message
to the typed lookup (`linoderoute.ToolOf`, `routes.tool_of`): the message
maps to its tool and the tool to its route, both read off the descriptors, so
the scope validator in `go/internal/profiles` and
`python/src/linodemcp/profiles` spells no tool name and still leaves
evidence. A message the contract does not know fails by name. Hard; a
language not caught up records an annotated absence in
`tool-parity-baseline.txt` instead.

## Generated-surface gates (venv)

### generated-form

One classification pass per tool surface, the merge of the former
`write-proto`, `read-proto`, `meta-proto`, and `input-proto` gates: write,
read, and meta handler success paths must route through proto on both sides,
and every tool's advertised input schema must be proto-generated. A
`*WriteResponse` proto with no conformance fixture fails too. Each surface
reports its own OK line, so a regression names its slice. Hard, no baseline.

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
buf.validate rules has its check read from the contract by both languages. It
is the population `hand-code` cannot see, since a check carries no tool name.
`docs/contracts/hand-validator-counts.txt` only falls, and a count over its line
names every site that pushed it there.

### hand-code

No hand-coded tool code beside the generated tree. Scans every non-generated
source tree each registered language owns (the working directory from
`languages.txt`, less the trees `.gitignore` marks as regenerated, less tests)
with two arms. The definition arm fails by name on any function named after a
tool. The literal arm fails by name and line on any quoted string that spells a
tool name as a whole word, whether the string is the one name or a comma-joined
list of sixty: the generated tree is the only place a tool's name is handed to,
so a hand-written file spelling one is calling, listing or dispatching on a
tool the contract has no declaration for. There is no allowed set and no count:
nothing derives a hand-written function name from a tool, so such a function is
code the contract does not account for, and the fix is a declaration on its
`*Input` message.

The literal arm alone leaves out the trees whose job is to call tools by name:
`go/internal/cli`, `python/src/linodemcp/cli` and `python/src/linodemcp/tui`
invoke tools through the same dispatcher a client uses. The script names them
in `_TOOL_CALLERS` with that reason. It is a statement of scope rather than an
exemption list: it names no tool, a function named after a tool inside those
trees still fails the definition arm, and a tree whose path merely starts the
same way is scanned by both.

Three limits worth knowing. A tool named by a single ordinary word (`version`,
`hello`) is matched by neither arm, because `version` prefixes six legitimate
definitions in this tree, is the JSON key of every version answer, and an
exemption list would be the one thing that could make the gate lie. An
argument check is named after what it checks rather than after a tool, so
neither arm can see one; `hand-validator-counts.txt` holds that population to
zero and this gate reads the file. And the literal arm reads one line at a
time, and a string only on the line that opens and closes it: a tool name
built at run time, one split across two adjacent or +-joined literals, one
inside a Go raw string or a Python triple-quoted string that spans lines, and
a body that reaches a tool through nothing but a resource type all stay
invisible, while a comment or docstring that quotes a tool name reads as a
literal to it.

### hand-arms

No hand-written local-operation arm beside the generated tree. An arm is the
four steps between a tool handler and a subsystem: read the declared inputs,
call the subsystem, map whatever it reported onto the declared conditions,
project the answer. Every one of them is written from the operation's own
declaration now, in every registered language, so a function named after a
`LocalCall` member in hand-written source is code the contract does not account
for. The fix is a subsystem method behind the generated interface.

It scans the same trees `hand-code` does, and reads each language's own arm
spelling rather than one flattened form: Go's `RunCatalogCanRun` and Python's
`run_catalog_can_run`. The case matters, because the CLI's own `runAuditRecent`
subcommand runner would otherwise read as an arm for `LOCAL_CALL_AUDIT_RECENT`.
Matching the opening of the name rather than the whole of it is what catches
both sides of a two-sided operation without this gate knowing the direction
vocabulary.

This gate replaced `docs/contracts/handwritten-arms.txt`, which listed the
operations each engine still wrote by hand. That list emptied when the last arm
was generated, and a file whose only legal content is nothing exempts nothing.
There is no allowed set and no count to raise.

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

### techdocs-proof

`python3 -m techdocs_proof --self-test` from `tools/techdocs-proof/src`: the
offline arm of the TechDocs-to-proto comparator. The comparator is stdlib
only, so this needs no venv and no network, which is what lets it sit in a
chain that stays strictly offline. It replays the comparison over frozen
descriptor fixtures, and it reads the shipped exclusion ledger
(`tools/techdocs-proof/data/known-divergences.json`), refusing a duplicate
key, an uncollapsed route shape, or an entry missing a field. It also holds
the evidence-placement refusal: a run root inside the repository fails by
name, so a comparison can never dirty the tree it measures. The scraping half
needs the network and runs only from `techdocs-drift.yml` or by hand; see
`tools/techdocs-proof/README.md` and its `docs/` wiki.

### go-analyzers

`scripts/verify_go_analyzers.py`: gopls' analyzer set (modernize and friends)
over every registered Go tree. gopls releases ahead of the x/tools tags
golangci-lint depends on, so a finding reaches a developer's editor long
before any other linter here sees it, and thirty-eight of them piled up
without turning `check` red. Scope comes from
[docs/contracts/languages.txt](./contracts/languages.txt): each registered
language whose working directory carries a go.mod, never a path literal, so a
renamed or newly registered Go tree stays in scope. Generated trees are
scanned with the rest, because an emitter that starts writing flagged code is
the finding, and `go/cmd/toolgen` is the only place it can be fixed. gopls
exits zero even when it reports, so any output is the failure. Hard
requirement rather than skip-if-missing, same false-green reason as
betterleaks; `scripts/ci-setup.sh` installs it.

### tools-typecheck

`scripts/verify_tools_typecheck.py`: mypy over each project under `tools/`.
`python-check` runs mypy from `python/` across `src/` and `tests/`, so nothing
type-checked `tools/` at all while `tools-lint` ran ruff there, and a tree that
is linted but never type-checked reads as covered.

The Python target is read from each project's own `requires-python`, never
written here and never inherited from the shipped package's `python_version`.
The comparator declares 3.14 and uses PEP 758 unparenthesized except-tuples;
mypy aimed at the package's 3.13 stops at that parse error and checks nothing
in the file. Deriving the target means a project that raises its floor raises
this with it, and there is no second place to update.

Two silent failures are guarded, and they are not the parse abort: mypy exits
non-zero there, so it fails loudly. A `tools/` that holds no project (renamed,
moved) and a run that reports no checked-file count both fail rather than
reporting a clean run. Findings print before that coverage check, because an
aborted run covers nothing *and* says why, and answering "scanned nothing"
would swallow the line naming the file that stopped it.

What it cannot see: a function declared `-> Any` that returns Any is invisible
to mypy by construction, so a caller misusing its result will never be a
finding here. Honest `Any` at a JSON boundary is fine; an `Any` standing in for
a shape nobody wanted to write is not, and no type checker distinguishes them.

### dockerfiles

`scripts/verify_dockerfiles.py`: droast at error severity over every
Dockerfile git reports as tracked or untracked-and-not-ignored, found by name
so a new image is in scope the moment its file exists. Nothing in `check`
read a Dockerfile before, which is how a `curl ... | sh` sat in the CI-mirror
image through twenty seals. Warnings and info are droast's own lower
severities and do not fail; the gate prints only what failed, each error
folded onto its own file and line. Hard requirement; `scripts/ci-setup.sh`
installs it from the release binary, checksum-verified.

### proto-lint

`scripts/verify_proto_lint.py`: `buf lint` over whatever `buf.yaml`'s modules
declare, which is why the gate carries no path of its own. `make proto` runs
`buf generate`, which does not lint, and nothing else ran buf at all, so four
findings sat in the contract every language is generated from. It stays
offline: the workspace declares no `deps`, there is no `buf.lock`, and every
import resolves inside `proto/`. Rule selection and per-path exemptions live
in `buf.yaml`, so a developer's own `buf lint` reads the same config; the
vendored `proto/buf/validate/` is exempted there under `lint.ignore`, which
is lint-only and leaves generation reading it. buf is already a hard
requirement and already pinned.

### overlay-roundtrip

`go/cmd/protomerge`: the standing control for the overlay extraction the
techdocs pipeline merges through. One run splits every proto file `buf.yaml`
declares into a SURFACE (which message carries which field, its label, its
type, its `field_location`, an enum's value names, and the method and path of
each `tool_route`) and an OVERLAY (wire numbers, declaration order, tool names,
prose, layout, and every other option the contract states), renders the pair
back, and byte-compares against the file it read. A file whose bytes move fails
by name, with the line and both spellings.

It is the only gate whose logic is Go source in this repo rather than a
`scripts/verify_x_y.py` script, because the merger it hardens renders these
proto conventions and has to version with them (the 2026-08-19 residency
ruling put it at `go/cmd/protomerge`). The recipe stays thin:
`go -C go run ./cmd/protomerge -repo ..`.

Scope comes from `buf.yaml`, the same declaration `proto-lint` reads, so a
module added later is walked without editing the gate, and the vendored tree
`lint.ignore` disowns is pruned rather than filtered. It stays offline: the
local tree and nothing else.

Wire numbers and declaration order live only in the overlay. The surface type
has no field to put a number in, so upstream drift cannot renumber anything:
the change is unrepresentable rather than refused.

This gate cannot see a reused number on its own. It measures the extractor
against the tree in front of it, so a field removed in one cycle and its number
handed to a different field in a later one round-trips clean: nothing left in
the tree says what that number used to mean. What says it is a `reserved`
statement inside the file, which `overlay-merge` writes when it retires a field
and protoc refuses to let anything reuse. `reserved` travels as overlay text
here, so a retired number round-trips like any other repo-owned line.

A declaration or a tail written in a shape the model cannot re-render is
refused rather than copied through, because copying it would leave the
extractor's gap invisible. The same holds for a `tool_route` option: the block
is modeled entry by entry, and one written in another order, with another
indent, or with an entry the model has no slot for is refused. A walk that
covered no file fails too, the way every hard gate here does.

The route is in the surface rather than in the verbatim spans because a route
is upstream's to state. Leaving the whole option as overlay text would mean a
route upstream dropped merged straight through, and the merger would never see
it. The tool name stays in the overlay: MCP names its own tools.

### overlay-merge

`go/cmd/protomerge` again, run the other way: it reads an upstream surface
descriptor off disk, renders every declared file from that surface plus the
overlay it just extracted, writes the result to a scratch directory, and diffs
it against `proto/`. At zero techdocs drift the diff is empty, which is the
merger's acceptance line.

The descriptor is written per run rather than committed. A merge input living
beside its own output would only restate it; the techdocs comparator supplies
the real one, and until then the tree emits its own with
`protomerge -emit-surface`. The gate spends two `go run` invocations and a
`diff -ru`, so the check reads as the thing it checks.

Two refusals cover REQ-D4, and a rename fires both because the merger cannot
tell a rename from a drop plus an add:

- an overlay anchor the descriptor stopped carrying, named with its file and
  line, because the repo would render a field upstream no longer states;
- upstream surface no overlay entry claims, named by key, because the field
  would reach the tree with no wire number, no declaration order, and none of
  the MCP semantics only this repo can state.

Both directions fire at three levels: fields, enum values, and routes. The
route level is what closes REQ-D4, since a route the descriptor drops means the
API the tool calls is gone, and a route it adds reaches the tree with no tool
name and no capability, scopes, or wording.

Nothing is written when either fires, so a refused merge leaves no half-merged
tree behind. A descriptor that fails to parse, and one that parses to no field,
enum value, or route, are refused the same way: every anchor in the tree would
dangle against an empty upstream and the run would read as total drift.

`-retire Message.field` is what the first refusal hands the reader. It drops the
declaration, drops the prose written above it, and leaves `reserved <number>;`
and `reserved "<name>";` in its place, so the number and the name stay spent
after the declaration that explains them is gone. Retiring a name the tree does
not carry is refused rather than ignored. Message-level `buf.validate` CEL rules
that still name the retired field are left alone on purpose: rewriting somebody
else's expression is a guess, and `proto-lint` names each one by line.

### wire-breaking

`buf breaking --against docs/contracts/wire-baseline.binpb.gz`: the
wire-stability check `buf.yaml` has declared under `breaking: use: FILE` since
the contract's first commit, which nothing invoked. A renumber, a rename on a
live number, and a deletion all fail by name and line.

The baseline is a pinned image, not a branch reference. A branch reference
compares a clean checkout against its own last commit, which is the same tree,
so the gate would report green in CI without ever measuring anything. The image
is built with `--exclude-source-info`, so comments and layout never move it and
it churns only when the wire shape does; two builds over one tree write the same
bytes.

Refreshing it is part of the reviewed change that earns it:

    buf build --exclude-source-info -o docs/contracts/wire-baseline.binpb.gz

`use: FILE` treats every field deletion as breaking, whether or not the number
retires to `reserved`. That is the right split rather than a gap: this gate
catches a wire change at the moment it is introduced, and the `reserved`
statement carries the retirement forever, since protoc refuses a later
declaration that reuses the number or the name. A refreshed baseline forgets the
retired field; the file does not.

Offline like `proto-lint`, for the same reason: no `deps`, no `buf.lock`, every
import inside `proto/`. Proven under a sandbox profile denying all network. A
baseline file that is missing or empty fails rather than passing a comparison
against nothing.

### techdocs-routes

`scripts/verify_techdocs_routes.py`: the repo half of the TechDocs loop. It
reads `docs/contracts/api-techdocs-routes-baseline.txt`, the reviewed snapshot
of every route the rendered TechDocs state, and every `tool_route` the proto
tree declares, and refuses a route that no longer lines up.

Two failures, both by name and both with the proto file and line:

- a `tool_route` the snapshot no longer states, which means the API the tool
  calls is gone;
- a tool built on the v4 surface whose route the snapshot restricts to
  v4beta, or the reverse, which means upstream withdrew the surface the tool
  is built on.

It answers the one question `wire-breaking` cannot. `wire-breaking` compares
`proto/` against a proto-derived image, so it catches a wire break WE made.
This snapshot is TechDocs-derived, so it catches a route THEY dropped, renamed,
or moved between surfaces. A gate that compared the tree against something
derived from the tree would report green no matter what upstream did.

Same reviewed-snapshot pattern as the `api-*-baseline` files: the scraping
half runs weekly in `.github/workflows/techdocs-drift.yml`, writes a candidate
snapshot into its run evidence, reports the diff in the job summary, and
uploads it. A human copies the candidate over the reviewed file and lands it,
because REQ-D5 keeps every repository change behind a reviewed diff. Refreshing
by hand from a run:

    PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof \
      --techdocs-contract <run>/techdocs-contracts.json \
      --emit-route-snapshot docs/contracts/api-techdocs-routes-baseline.txt

Placeholder names collapse to `{}` in both directions, because TechDocs write
`clusterId` where the proto writes `cluster_id`. That is a spelling variant
rather than a different slot, and the shape is the only join the two sides
agree on. The per-slot names stay in the comparator's findings.

Routes the snapshot marks deprecated are counted and named in the report
rather than failed. REQ-D7 owns that worklist and its six standing entries
would make this gate red for a debt it does not own. A route TechDocs state
that no tool declares is not this gate's business either: that is
`route_missing_from_proto` in the comparator's findings.

Scope comes from `buf.yaml`, the same declaration `proto-lint` and
`overlay-roundtrip` read. Nothing here is per-language, so
`docs/contracts/languages.txt` has no scope to give: both languages generate
their route tables from this one proto tree. Offline, stdlib only, and a
snapshot that is missing or states no route fails rather than passing a
comparison against nothing.

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

`techdocs-drift.yml` is the same shape for the other upstream: it scrapes
rendered TechDocs and compares them against the checked-out proto tree,
uploading the run directory as an artifact. It reads only, never commits, and
its findings feed the sync batches rather than any gate.

It also carries the weekly refresh for `techdocs-routes`. The run writes a
candidate `route-snapshot.txt` into its evidence, the job diffs it against
`docs/contracts/api-techdocs-routes-baseline.txt` and prints the difference in
its summary, and the upload step carries the candidate out. Landing it is a
human copying the file and reviewing the diff, because REQ-D5 puts every
repository change behind that review rather than behind a bot commit or a bot
pull request.

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

The contract's declared per-tool OAuth scopes, as Python renders them, vs the
spec's per-operation security blocks. Needs the venv, unlike the other sync
gates. Deviations live annotated in
`docs/contracts/scope-sync-baseline.txt`, structural ones in
`docs/contracts/scope-sync-exempt.txt`. A route the spec documents no
operation for is skipped, not failed: the spec lags techdocs, and
`route-evidence` already proves offline that the route is real.

### sync-issues

Runs `scripts/verify_tracking_issues.py`: every baseline acceptance still
cites an open tracking issue. `baseline-guard` only checks that an annotation
looks like an issue URL, which a closed issue satisfies forever; this resolves
each one via `gh` and skips loudly when `gh` is absent.
