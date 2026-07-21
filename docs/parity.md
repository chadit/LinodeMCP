# Cross-language parity: how a change in one language reaches the others

LinodeMCP ships the same MCP server in more than one language (today Go and
Python, registered in [`contracts/languages.txt`](./contracts/languages.txt)).
The implementations are wire-identical by construction, not by discipline:
shared sources generate the contract, shared fixtures pin the behavior, and
`make check` fails until every registered language agrees. This page explains
that machinery and walks through what happens when you change one language.
For bringing up an entirely new language, read
[adding a language](./adding-a-language.md) instead.

## The three layers that keep languages aligned

**1. Derived artifacts.** Input schemas, output shapes, and enum value sets
all come from the protobuf contract in `proto/linode/mcp/v1`. `make proto`
(`buf generate`) emits each language's typed messages plus the MCP input JSON
Schemas, so a contract change lands in every language from one edit. Nothing
here is hand-written; the **input-proto**, **read-proto**, **write-proto**,
and **meta-proto** gates fail any handler that hand-builds what should be
generated.

**2. Shared behavior fixtures.** The hand-written part of a tool (argument
validation, error text, the HTTP call it makes, its dry-run preview) is
pinned by `testdata/behavior/*.json`. Every fixture case replays through each
language's real dispatch path with the HTTP layer faked, and each case states
its outcome: an exact error, an exact outgoing request, or an exact result
compared as JSON against routed `api_responses` fakes. Both runners
(`go/internal/server/behavior_conformance_test.go`,
`python/tests/unit/test_behavior_conformance.py`) run every case, so a
semantic change in one language fails the other language's test suite until
its twin catches up. Destroy tools must additionally carry a `dry_run: true`
case that pins their preview content, because previews are hand-written per
language and are exactly where drift hides.

**3. Registries and ratchets.** [`contracts/tools-manifest.txt`](./contracts/tools-manifest.txt)
lists the full tool surface and
[`contracts/tools-capabilities.txt`](./contracts/tools-capabilities.txt) pins
each tool's tier; per-language tests enforce both. Every remaining gap lives
in a baseline ratchet under [`contracts/`](./contracts/): the gate fails on
any NEW divergence and on any stale entry, so the lists only shrink. When a
gap is accepted on purpose (one language landing ahead), its baseline line
must carry an annotation, and CI's baseline guard blocks unannotated growth.

## You changed one language. What pulls the others along?

### Adding a tool (a new route family)

Start at the proto: define the input message (and response message for the
output surface), run `make proto`, and every language gets the schema and
types. Then, in the same change:

- implement the tool in **every** registered language,
- add its name to `contracts/tools-manifest.txt` and its tier to
  `contracts/tools-capabilities.txt`,
- add a behavior fixture in `testdata/behavior/` (Write tools need a
  confirm-rejection case, Destroy tools need the destroy-gate case and a
  dry-run preview case).

Skip any of that and a specific gate names the gap: **tool-parity** reports
`missing in <language>`, the manifest tests fail the language that lacks the
tool, **behavior** reports the uncovered tool or the missing safety case.
"Compiles in Go" is not done; `make check` green is done.

### Landing one language first (the accepted-absence flow)

Sometimes an issue is scoped to one language and the twin lands later. That
is allowed, but never silent:

1. Implement the first language; add the tool to the manifest as usual.
2. `python scripts/verify_tool_parity.py --update-baseline` records the
   absence as `<tool>: missing in <language>` in
   `contracts/tool-parity-baseline.txt`.
3. Annotate each new line: `<entry>  # accepted YYYY-MM-DD <tracking-issue
   URL>`. The parity gate hard-requires the annotation on absences, and the
   baseline guard (`make baseline-guard` locally, `baseline-guard.yml` in CI)
   fails any added baseline line that lacks one or whose annotation cites no
   tracking-issue URL. A dated free-text reason is not enough on a ratchet:
   the entry is a promise to come back, and the issue is where that promise
   lives. Only `behavior-exempt.txt` accepts a reason without a URL, since a
   permanent exemption has no follow-up to track.
4. The catch-up work stays visible in `make parity-todo` until the twin
   lands, at which point the gate fails on the stale line and the entry
   comes out.

The same flow covers the other ratchets (behavior coverage, proto-routing)
when a partial landing touches them.

### Changing a tool's input contract

Edit the proto message, run `make proto`, and both languages advertise the
new schema automatically. What does not move automatically is hand-written
validation around it: error messages and rejection behavior are pinned by
the behavior fixtures, so update the fixture case and both languages must
match it. **tool-parity** catches param/type/required drift;
**input-proto** catches a language quietly reverting to a hand-built schema.

### Changing output or behavior

Output shape changes go through the response proto (the conformance corpus
and the proto-routing gates keep both languages on the generated message).
Behavior changes (validation, request bodies, confirm text, previews) go
through the fixture: change the case, and the language you did not touch
fails its conformance run until its handler matches. This is deliberate. A
preview enrichment added only to Go, for example, changes Go's pinned
`expect_result` and immediately reddens Python's runner, which is the
mechanism that used to be missing. Confirm-message wording is additionally
diffed repo-wide by the **messages** gate even for branches no fixture
exercises.

### Removing a tool

Remove it from every language, the manifest, the capabilities file, and its
fixture, in one change; record the removal and its replacement in
[deprecated routes](./deprecated-routes.md). Leftovers fail the same gates
in reverse (extra registered tool, fixture naming an unknown tool, stale
baseline entries).

## The commands

| Command | What it does |
|---|---|
| `make check` | The gate. Both languages' lint and tests plus every cross-language gate; local green, hook green, and CI green are the same fact. |
| `make parity-todo` | Per-language remaining-work report aggregated from the baselines. |
| `make baseline-guard BASE=<rev>` | Baseline growth must carry issue-linked annotations. Runs inside `make check` against origin/main; CI re-runs it with the event's true base. |
| `make diff-coverage BASE=<rev>` | Added (and untracked) source lines must be covered by tests. Runs inside `make check` against origin/main; CI re-runs it with the event's true base. |
| `make tool-parity` / `behavior` / `input-proto` / `read-proto` / `write-proto` / `meta-proto` / `messages` / `pagination` / `dryrun` / `env-parity` / `cli-surface` / `docs-links` / `metrics-surface` / `coverage-floor` | Run one gate alone while iterating. |
| `python scripts/<gate>.py --update-baseline` | Regenerate a ratchet after intentional work; annotations on surviving entries are preserved. |

## Where to look next

- [Adding a language](./adding-a-language.md): onboarding a whole new
  implementation, from `contracts/languages.txt` registration to a passing
  conformance runner.
- [Docs index, machine contracts table](./README.md#machine-contracts): what
  every file under `contracts/` pins and which script owns it.
- [Dry-run](./dry-run.md) and [two-stage writes](./two-stage-writes.md): the
  safety semantics the behavior fixtures pin.
