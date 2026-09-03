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
here is hand-written; the **generated-form** gate fails any handler or
advertised schema that hand-builds what should be generated.

The per-tool code is derived the same way, from one emitter. `make proto` runs
`go/cmd/toolgen` once: it reads the descriptors into a single contract model and
each registered language's renderer arm writes that model into that language's
`gentools` tree. There is no second generator to forget to run, and an option
one arm acts and another says nothing about stops the run before either tree is
rewritten, so a capability cannot land in one language and go missing in the
other. Which languages get an arm comes from
[`contracts/languages.txt`](./contracts/languages.txt); a registered language
with no arm fails by name.

The route a tool calls is derived the same way, and so is the API surface it
answers on: `tool_route` and `tool_api_surface` sit on the tool's input message,
and every language resolves both at runtime rather than spelling out a URL. A
tool on a non-default surface also leads its advertised description with a
`[v4beta]` marker, so a caller choosing from a tool listing can tell which
surface it reaches. The **api-surfaces** gate holds the census in
`contracts/api-surfaces.txt`, the contract, and every language's descriptions to
one answer, in both directions.

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
case that pins their preview content, because a preview is where drift hides:
the prose most tools report is declared once in the proto and emitted into
both languages, but the bodies that compute it from fetched state are still
written per language.

A case may also declare `setup_calls`, `audit_store` and `now`. `setup_calls`
sends prior tool calls to the same server instance, which is what makes a
builder tool's success path reachable at all: the draft a `draft_show` or a
`draft_save` acts on exists because an earlier call in the same case made it.
`audit_store` seeds the store the query tools read (its `backend` picks the
JSONL log or the SQLite database, and a SQLite case seeds only the database, so
an answer carrying the events proves the tool read it), and `now` fixes
the reference clock the audit report measures a relative window back from. A
case using any of the three runs in a serial lane that redirects
`XDG_STATE_HOME` and `LINODEMCP_CONFIG_PATH` into a directory the case owns, so
a `draft_save` fixture writes a real config file without touching yours, and
`{{audit_dir}}` and `{{sqlite_path}}` resolve in `expect_result` because the
runner owns those paths.

A case may also declare `host_decided`, naming the answer fields whose exact
value the host or the database driver picks: the OS and architecture `version`
reports, the temp path `audit_export` writes to, the file size the two SQLite
drivers land on. Everything a case does not name stays a whole-answer literal.
A named field is never ignored: it carries its declared type plus a pattern (for
a string) or a floor (for an integer), and a reason a reader can weigh. Both
runners fail a case that names a field the answer does not carry, so a rename
cannot quietly turn a pin into a no-op, and fail one that names a field
`expect_result` also spells, so every field has one owner.

**3. Registries and ratchets.** `contracts/tools-manifest.txt` lists the full
tool surface and `contracts/tools-capabilities.txt` pins each tool's tier;
per-language tests enforce both. `make proto` writes both from the proto
contract and they are gitignored, so they are read and never edited. Most
gates keep no list: a finding fails on the spot, which is what "the gap class
is closed" looks like once it is. The gaps that remain live in a baseline
ratchet under [`contracts/`](./contracts/): the gate fails on any NEW
divergence and on any stale entry, so those lists only shrink. When a gap is
accepted on purpose (one language landing ahead), its baseline line must carry
an annotation, and CI's baseline guard blocks unannotated growth.

## You changed one language. What pulls the others along?

### Adding a tool (a new route family)

Start at the proto: define the input message (and response message for the
output surface), run `make proto`, and every language gets the schema and
types. Then, in the same change:

- nothing per-language: the emitter covers every tier, so `make proto` writes
  the tool into every registered language's tree. Only a tool still named in
  [`contracts/handwritten-tools.txt`](./contracts/handwritten-tools.txt) needs
  hand-written code, and that list is empty,
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

The same flow covers the dry-run preview ratchet when a partial landing
touches it. It does not extend to the
[hard gates](./gates.md#hard-gates-carry-no-file), which have no list to add a
line to, so a partial landing that breaks one of them is not a partial landing,
it is a broken build.

### Changing a tool's input contract

Edit the proto message, run `make proto`, and both languages advertise the
new schema automatically. **tool-parity** catches param/type/required and
OAuth-scope drift; **generated-form** catches a language quietly reverting to
a hand-built schema.

An argument check belongs on the message too, as a `buf.validate` message-level
CEL rule carrying the exact sentence the tool answers with. One rule, one
wording, both languages: `go/internal/toolvalidate` and
`linodemcp.tools.constraints` evaluate it, and every generated handler asks them
before it reads an argument. Rules are evaluated in declaration order and the
first one broken is the answer, so the order is part of the contract.

Use message-level rules only. A field-level `buf.validate` rule becomes a JSON
Schema keyword, which changes the schema every client reads.

Two things a rule cannot say, both worth knowing before reaching for one:

- Anything about an enum value that names no member. It reaches a rule as the
  enum's zero, which is what an absent argument reaches it as too, so
  "type is required" and "type must be one of: ..." cannot be told apart. Those
  are answered by a declared `argument_reader`, and any rule ordered behind one
  has to hold its peace until the enum names something (see `DomainCreateInput`).
- Anything about an argument the message does not declare.

Anything still written out by hand is counted by **hand-validators**, and the
count only falls; it reads zero in both languages today. **hand-code** covers
the other half: no hand-coded tool code may sit beside the generated tree. What
each of the two scans, and the three things `hand-code` cannot see, are in
[the check gates](./gates.md). Rejection behavior is pinned by the behavior
fixtures either way, so update the fixture case and both languages must match
it.

### Adding pagination to a list tool

A tool whose route paginates in the OpenAPI mirror must expose `page` and
`page_size`, and **pagination** fails by name when one does not. That makes this
the recipe for a new list tool, and for the moment a snapshot refresh turns an
existing route paginated:

1. Add `optional int32 page` and `optional int32 page_size` to the tool's
   proto input message and run `make proto`. Both languages pick up the
   schema, including the field-comment text that becomes the parameter
   description, so do not restate it in either language.
2. Read the pair with the shared reader, never a new per-family copy: Go's
   `standardPaginationFromTool`, Python's `standard_pagination_arguments`.
   Both apply the standard 25-500 bounds and emit identical rejection text.
3. Build the request path with Go's `withPaginationQuery` or Python's
   `paginated_path`. An unset value stays off the query string so the API's
   own default applies, which is what keeps the two languages issuing
   byte-identical requests.
4. Pin the result in the tool's behavior fixture: the non-integer rejection,
   both bound rejections, and one `expect_request` whose path carries
   `?page=2&page_size=50`. The fixture is the cross-language contract; a
   language-only test is not.

Do not declare new page-size constants. The bounds live in
[api-pagination-baseline.txt](./contracts/api-pagination-baseline.txt), which
the scheduled `sync-pagination` gate writes from the OpenAPI mirror, and
**pagination** fails any constant in either language that disagrees with it.
The mirror is the secondary source: when it and TechDocs disagree, TechDocs
rules and the snapshot is what gets refreshed, per [the network sync
gates](./gates.md#network-sync-gates-scheduled-only).

### Changing output or behavior

Output shape changes go through the response proto (the conformance corpus
and the **generated-form** gate keep both languages on the generated message).
Behavior changes (validation, request bodies, confirm text, previews) go
through the fixture: change the case, and the language you did not touch
fails its conformance run until its handler matches. This is deliberate. A
preview enrichment added only to Go, for example, changes Go's pinned
`expect_result` and immediately reddens Python's runner, which is the
mechanism that used to be missing. Confirm wording is additionally covered
repo-wide by the **messages** gate, which diffs how the emitter rendered the
contract's `confirm_message` into each language's tool tree, even for branches
no fixture exercises.

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
| `make parity-todo` | Per-language remaining-work report aggregated from the baselines, plus the gates that keep their own class at zero. |
| `make baseline-guard BASE=<rev>` | Baseline growth must carry issue-linked annotations. Runs inside `make check` against origin/main; CI re-runs it with the event's true base. |
| `make diff-coverage BASE=<rev>` | Added (and untracked) source lines must be covered by tests. Runs inside `make check` against origin/main; CI re-runs it with the event's true base. |
| `make <gate>` | Run one gate alone while iterating. Every gate keeps its own target, named and described in [the check gates](./gates.md). |
| `python scripts/<gate>.py --update-baseline` | Regenerate a ratchet after intentional work; annotations on surviving entries are preserved. |

## Where to look next

- [Adding a language](./adding-a-language.md): onboarding a whole new
  implementation, from `contracts/languages.txt` registration to a passing
  conformance runner.
- [Docs index, machine contracts table](./README.md#machine-contracts): what
  every file under `contracts/` pins and which script owns it.
- [Dry-run](./dry-run.md) and [two-stage writes](./two-stage-writes.md): the
  safety semantics the behavior fixtures pin.
