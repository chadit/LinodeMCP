# Adding a new language to LinodeMCP

This repo ships the same Linode MCP server in more than one language (today Go and
Python). They are not independent reimplementations that happen to agree. They are
byte-identical on the wire *by construction*, because everything derives from one source:
the protobuf contract in `proto/linode/mcp/v1`. A new language is worth adding only if it
holds that line. This guide is the checklist for doing that, and the list of gates that
fail you if you skip a step.

## The one rule

Everything a tool exposes (its input schema, its output shape, its enum value sets) comes
from the proto contract and its generated artifacts. You do not hand-write any of it. If
you find yourself typing a JSON schema, a field list, or an enum by hand in the new
language, stop: that is the drift this project exists to prevent.

The goal that rule serves, repo-wide: the only code written by hand for a language is the
bootstrap that starts the MCP server and the CLI, plus the engine below it that step 5
lists, written once and never per tool. Tool bodies, argument checks, hooks, a transport's
per-tool behavior, and every refusal sentence come out of proto declarations through one
emitter into every language `docs/contracts/languages.txt` registers. The proto tree is
still edited by hand and that is fine. A monitoring agent will later emit the upstream
surface and keep proto in step with it; building that emitter is separate work, not part
of onboarding a language.

Run `make proto` first. The generated trees are gitignored and nothing builds without
them:

- **`genpb`**: the message types, from `buf generate`.
- **`toolschemas`**: the MCP input JSON Schemas (`<full.msg.name>.schema.strict.json`),
  from the same `buf generate`. A tool advertises its input by loading its strict schema.
- **`gentools`**: the per-tool factory and handler code, emitted by `go/cmd/toolgen` from
  the compiled descriptors, one renderer arm per registered language. A generated tool has
  NO hand-written per-tool code anywhere: route, capability tier, description, confirm and
  success and error prose, response binding, argument extraction, filters, local answer,
  and list envelope all come off the options, so a new tool is one proto message and
  reaches every language with no line in any registry.
- **`genlocal` and the operations tree**: the answer each local operation fills, the
  projection turning one into the plain body its tool answers with (top level and nested
  both), the condition vocabulary a subsystem reports, and the arm behind each operation,
  emitted beside the handlers that call it (`go/internal/gentools/operations.gen.go`,
  `python/src/linodemcp/gentools/operations.py`). What your language owes them is step 5.
- **The tool registries**: `docs/contracts/tools-manifest.txt` and `tools-capabilities.txt`,
  written by `scripts/gen_tool_registries.py` inside the same `make proto`. Read and never
  edited; [the three layers that keep languages
  aligned](./parity.md#the-three-layers-that-keep-languages-aligned) covers what each pins.

## Step zero: register the language

`docs/contracts/languages.txt` is the registry the cross-language gates read. One line per
implementation: `<name>\t<working-dir>\t<dump command>`, where the command prints the
language's tool surface as JSON records (`go/cmd/parity-dump` and
`python -m linodemcp.parity_dump` show the record shape). Registering turns the parity
gates on: a freshly registered language with no tools yet fails `tool-parity` with one
`missing in <name>` line per manifest tool.

That failure list is the onboarding plan, and it lives in
`docs/contracts/tool-parity-baseline.txt` as accepted absences. The
[accepted-absence flow](./parity.md#landing-one-language-first-the-accepted-absence-flow)
is the recipe and it holds here unchanged, seeded in one pass with `--update-baseline`;
`make parity-todo` renders what is left. Line order in `languages.txt` matters: the first
language that implements a tool is the reference its contract is diffed against, so keep
the most complete implementation first.

Every other gate holds its class at zero with [no list to add a finding
to](./gates.md#hard-gates-carry-no-file), so a finding fails outright. That changes the
order of the work rather than the amount: enroll in a gate when the language can pass it,
not before. It bites during onboarding at `route-evidence` (step 7), because registering
makes it demand a scanner arm by name and the arm then has to resolve every declared
route. Until the client can build them, the unbuilt surface is recorded once, as annotated
absences in `tool-parity-baseline.txt`.

## What a new language must implement

Each item names the gate that enforces it, so you know what "done" is checked by.

1. **Consume the generated input schemas.** Build every tool's advertised input from its
   `*.schema.strict.json`, not from code. Enforced by **`tool-parity`** (surface, params,
   required, types) and the input surface of **`generated-form`**.

2. **Emit proto-canonical output.** Serialize every response through the generated message
   (the equivalent of Go's `MarshalProtoToolResponse`), so the JSON on the wire is
   identical across languages. Enforced by the write, read, and meta surfaces of
   **`generated-form`**.

3. **Match the tool surface exactly.** Same tool names, same capabilities, per
   `tools-manifest.txt` and `tools-capabilities.txt`, with the manifest tests proving it
   (Go `TestToolSurfaceMatchesManifest`, Python `test_tools_manifest.py`). The manifest
   lists the FULL surface; a tool your language has not caught up on lives in
   `tool-parity-baseline.txt` as an annotated `missing in <name>` entry, never as a
   manifest omission.

4. **Implement a behavior-conformance runner.** This is the most important one. The
   language-agnostic fixtures in `testdata/behavior/*.json`, and what a case may state as
   its outcome, are [parity's second
   layer](./parity.md#the-three-layers-that-keep-languages-aligned). What a new language
   owes is the runner that replays every case through its own dispatch path with the HTTP
   layer faked, the way `go/internal/server/behavior_conformance_test.go` and
   `python/tests/unit/test_behavior_conformance.py` do, plus the rule the runners carry
   themselves: a `dry_run: true` case may only issue GETs.

   Four fields carry the meta tools whose answers depend on prior state: `setup_calls`,
   `audit_store`, `now`, and `host_decided`. [Parity's fixture
   layer](./parity.md#the-three-layers-that-keep-languages-aligned) states what each pins
   and how a `host_decided` field is compared. Your runner owes all four (a `setup_calls`
   answer is never asserted), plus three mechanics that page does not carry. Cases using
   any of the first three need process-scoped redirection of `XDG_STATE_HOME` and
   `LINODEMCP_CONFIG_PATH`, plus `{{audit_dir}}` and `{{sqlite_path}}` substitution in
   `expect_result`. `audit_store` writes its events before the server starts, into the
   JSONL log or, when `backend` says so, into a SQLite database through the sink's own
   writer, and JSONL lines are re-encoded with sorted keys, compact separators and no
   escaping so every language writes the same bytes. And your runner MUST refuse a case
   key it does not implement: a field one language honors and another ignores is the drift
   these fixtures exist to stop.

   Enforced by **`behavior`**, which also requires every Destroy tool's fixture to carry a
   dry-run preview case; `docs/contracts/behavior-dryrun-baseline.txt` ratchets any gap.

5. **Implement the contract runtime, once.** This is the layer every generated tool calls,
   and it is the whole per-language cost of the surface.

   - **A route builder** reading `tool_route` from the descriptors (the shape of
     `go/internal/linoderoute` and `python/src/linodemcp/linode/routes.py`), with the shared
     path-escaping contract: RFC 3986 unreserved plus a literal colon, uppercase hex,
     all-dots values percent-encoded. Both existing languages pin an identical escaping
     vector table in their route tests; port that table verbatim, it IS the contract.
   - **API surface selection**, from `tool_api_surface` on the same message as the route.
     Nearly every route answers under the configured base; the tools in
     `docs/contracts/api-surfaces.txt` answer under another, and a language that skips this
     silently 404s every one of them. Three shared parts: swap the base's trailing `/v4` for
     the declared surface and use any other configured base exactly as it is (the
     per-environment `apiUrl` override wins, since a surface picks among versions of one
     deployment and never picks the deployment); refuse a surface the build cannot address
     rather than defaulting it; and lead the advertised description with its `[<surface>]`
     marker. Enforced by **`api-surfaces`** and by `expect_request.api_surface`, which
     defaults to v4 so a tool that moved surfaces without its fixture fails.
   - **A runtime reader** for whatever tool options your arm does not spell into the code it
     emits: capability, response binding, confirm/success/error prose, resource type, retry
     policy, description. Go emits the declared sentence into the factory; Python's driver
     reads the same option off the descriptor at call time through `contract_for`. Both are
     honest; your arm states which way it went, option by option, in `acts` (step 6).
   - **The driver tier**: list, write, and destructive drivers the generated code configures
     (Go's list factories and destroy drivers; Python's `linodemcp.tools.drivers`). Drivers
     own pagination, dry-run ordering (dry-run validates before confirm; live gates confirm
     before validating), the confirm and destroy gates, retry policy, and serialization.
   - **The engine operations the declarations name.** Every step a handler runs is something
     the tool's `*Input` message declares, and your arm renders a call into a shared engine
     rather than a body per tool.

   The declarations and the engines they reach:

   - `normalize_fields` and `normalize_fold` rewrite the arguments in place before anything
     reads them, each naming a transform both languages spell once.
   - The whole argument check is declared, and you implement four readers rather than any
     tool's check (see [where the argument checks live](#where-the-argument-checks-live)).
   - `state_route` and `state_composite` name the read a removal previews and a plan hashes
     for drift; a removal declared beside a GET on its own route has that read derived.
   - `dependency_walk` and `billing_delta` say what else the change touches, reading the
     state that read produced.
   - `preview_sentence` is a dry run's prose: two readers, a sentence chooser, and the
     filter that drops a line none of whose wordings could be filled
     (`go/internal/tools/preview_sentence.go`, `python/src/linodemcp/tools/preview.py`). A
     presigned upload's preview reads what its transport measured off the local file
     through `{transport:size_bytes}` (`PreviewPresignSource`, `preview_presign_source`).
   - `execute_transport` covers a route whose request or answer is not JSON: a multipart
     form framed from a local file, a raw image body, a presigned transfer, a presigned
     removal. Implement the four arms once as an engine the emitter renders a call or a
     spec literal into (`go/internal/tools/transport.go`,
     `python/src/linodemcp/tools/transport.py`).
   - `local_answer` is a meta tool's whole result, read from local state rather than any
     route. It names an operation and the guards around it; the operation takes typed
     parameters and never the tool's argument bag, so it cannot tell which tool called it
     (`go/internal/tools/local_answer.go` and `builderops.go`,
     `python/src/linodemcp/tools/local_answer.py` and `builderstate.py`). A generated arm is
     handed the ambient state the operation requires, or the engine function serving it
     where it requires none.

   ### What local operations cost beyond their declaration

   Start with the shapes. Build each answer through the generated constructor and hand it to
   the generated projection, so a member added to a message stops the build until every
   language carries it, and hold a projected body to the message it fills in both
   directions: a record leaving a declared member out is refused rather than filled with
   that member's zero, which is what a plain proto decode would do while reporting success.
   Write the subsystem method behind each operation and no arm at all, held to the emitted
   type by the compiler in Go and by `linodemcp.gentools.conformance` plus the type gate in
   Python, which is the one hand-written test a dynamic language needs. A two-directional
   operation gets an arm and a method per direction, named for the direction, because a
   generated arm fills one answer shape; the direction is the declaring tool's, so nothing
   carries one at run time. A condition is reported bare or carrying a cause, since some
   tools word a sentence around the cause: Python's condition is a class whose constructor
   takes the cause, and Go gets `NewLocalCause` beside the sentinels, because one shared
   error value cannot carry a cause itself.

   An operation may declare a VERDICT VOCABULARY, a second set of conditions it reports one
   per entry of what it was handed rather than once per call (`LOCAL_CALL_CATALOG_CAN_RUN`
   is the only one today). The bucket a verdict counts in and the two sentences it answers
   with are declared on the OPERATION rather than on a tool, because a caller reading two
   binaries has to read one key and one sentence. Your language gets the vocabulary and a
   wording lookup emitted into its answers tree; the subsystem picks the verdict each entry
   meets and asks the lookup for the words, writing none of that prose, so a reword reaches
   every language in one `make proto`.

   An operation may also declare AMBIENT READINGS, values no argument carries and no tool
   answers for. Three exist today, and your rendering of each says two things: the type the
   subsystem takes, and the expression the handler hands over. A cancellation renders as
   Go's `context.Context` and as nothing at all in Python, whose handler takes the arguments
   and the configuration and nothing else. The configuration renders as a value in both,
   handed over as the handler's own parameter. The clock renders as an instant (`time.Time`,
   `datetime`) neither handler holds, so each language hands over the expression its own
   seam reads (`tools.ClockFromContext(ctx)`, `now()` from `linodemcp.tools.clock`); it is
   declared rather than left to each language because a language reading its own wall clock
   cannot replay a pinned report, and the behavior corpus holds the report tool to one. A
   registered language stating no rendering for a declared reading stops the run by name,
   because its arm would hand the subsystem less than the declaration says while reading as
   though it had handed over everything; so does a rendering naming a type and no
   expression, which Go answers as a build failure in emitted code and a dynamic language
   only on the first call that arrives. The cost is one spelling per reading in your arm (or
   a stated rendering in nothing), a configuration value your subsystems can take, and a
   clock seam a test can publish an instant through.

   Two methods reach a file their declaration does not mention, so read the shipped
   implementations rather than the contract for what they do. `DraftSave` writes the profile
   draft to the operator's own configuration, reading the path and the file at call time
   rather than through the configuration the state carries, so a concurrent edit is not
   stomped and a path override applies per call. `AuditExport` writes the exported events
   into a temp file the OS names, answers that path, and removes a half-written file rather
   than naming it; the format doubles as the extension. Their read and write conditions are
   the only trace either leaves.

   The four audit query operations share one rule the declaration also does not carry: each
   resolves the SQLite path from the running configuration on every call and attempts it
   whatever it holds, falling back to the JSONL log with a warning, the read behavior
   [audit](./audit.md#sqlite-optional) describes. Only a log nothing can read reports the
   read condition. The rule lives in the audit subsystem beside the readers
   (`go/internal/audit/store.go`, `python/src/linodemcp/audit/store.py`), and every failure
   your readers can raise goes through it: catching a narrower set makes a corrupt column
   degrade in one language and raise out of the handler in another.

   The audit record is the one shape the contract declares `local_record`, and that message
   is written TO DISK as well as answered with. Your language gets its serializer and reader
   emitted (`RecordAuditEvent` / `record_audit_event`, `AuditEventFromRecord` /
   `audit_event_from_record`), so a log line's member order is the message's own rather than
   your struct's. Two things you still write: the sink that appends a line and rotates the
   file, and the export documents the three formats wrap records in. Both are held to
   `testdata/audit/record-bytes`, a fixture every language asserts its own bytes against, so
   the CSV terminator, the JSON document's trailing newline, and the record order have one
   answer. What the fixture cannot hold is the free-form `args` member: a value carrying a
   character your JSON encoder escapes its own way comes out as your language spells it. The
   reader is lenient about a member a record leaves out, reading it as that member's own
   zero, because a log written by an EARLIER version is the case it exists for; it is not
   lenient about a line that is not a record at all, which the readers skip.

6. **Write a renderer arm.** The emitter is one program in two halves, described in
   [parity](./parity.md#the-three-layers-that-keep-languages-aligned): your language is an
   arm below the descriptor-reading seam, never a second program reading them again. So you
   inherit the derivation rules instead of restating them: response binding comes from
   `tool_response` only (never from names), tier from the capability and then the response
   shape, list envelope from the response's single repeated field, filters from QUERY fields
   matched against element fields, and every prose string from its option. A mutation whose
   declared response is page-shaped decodes through the list envelope, since its route
   reports the collection that now exists. A BODY member carrying `body_fold` is assembled
   from the flat arguments beside it rather than taken whole, and those arguments stop
   traveling at the top level; a key the caller wrote wins over a folded one.

   The arm is the `renderer` interface in `go/cmd/toolgen/renderer.go`. Five methods:
   `renderGroup` (the file holding every tool declared in one proto source file),
   `renderRegistry` (the file the server reads to register the cohort, emitted from that same
   cohort because a tool nothing registers passes every contract gate and is still missing
   from the running server), `owns` (which output files were yours, so a run clears its own
   stale output and nothing else), `language` (the name `languages.txt` registers you under),
   and `acts`. Add a case for the name to `armFor` in `languages.go`, or the run stops by
   name. Read `gorenderer.go` and `pyrenderer.go` with its `py*.go` family before you start.

   `acts` is the one to get right. It is your arm's answer for EVERY option the contract
   declares, read live out of the compiled descriptors rather than kept as a list, so an
   option added to the proto joins it the moment it compiles. Each answer says one of two
   things: my rendered bytes move with this option, or this named file in my language's
   support layer acts it. Say nothing about an option and the run stops before a tree is
   rewritten. Arms may disagree about WHERE an option is acted, so the check is that each
   arm answers at all, not that the answers match.

   Your language also needs the support layer the emitted code calls into: Go's
   `go/internal/tools`, Python's `linodemcp.tools`. A rendered `TrimArguments`,
   `PreviewSentence`, or `WriteBody` lives there, and that layer is what an `acts` claim
   names when your arm does not emit the option itself. A declared state read lives there too
   (`go/internal/tools/declared_state.go`, `python/src/linodemcp/tools/declared_state.py`): a
   removal's state is the body the API sent projected through the read's own message, so a
   key it sent as null survives at any depth and a key it never sent is not invented. You
   need that projection, the three readers a dependency walk uses over it, and the refusal
   that keeps a walk from being handed a state some other fetch produced. The Python arm
   renders through the repo's own ruff, so a new arm may shell out to its language's
   formatter the same way. The emitter refuses, by tool name, anything under-declared.
   Enforced by **`generated-tools`** and the emitter's own coverage guard.

7. **Enroll in the route-evidence scanners.** `make route-source` and `make route-evidence`
   need a scanner arm for your language (the job `scripts/_routescan.py` does for Python and
   `go/cmd/route-dump` does for Go): which call sites build requests, which primitives resolve
   routes from the contract, and which routes therefore have evidence. A registered language
   with no scanner arm fails by name, and once the arm exists every declared route must
   resolve through it: `route-evidence` keeps no list of routes a client cannot build yet.

8. **Match confirm-text.** Enforced by **`messages`** (cross-language confirm-message parity).

9. **Wire it into `make check` and CI** so all of the above run on every change.

## The value sets that are not proto enums

Proto enums generate for free. Three validation value-sets **cannot** be proto enums,
because their values are not valid proto identifiers (`public-read` has a hyphen,
`anti_affinity:local` a colon) or they are map keys rather than a scalar field (config
device slots `sda` through `sdh`). The contract carries them anyway:

| value set | where the contract carries it |
|---|---|
| bucket ACL (`private`, `public-read`, `authenticated-read`, `public-read-write`) | a CEL alternation on the bucket-access rule |
| placement group type (`anti_affinity:local`) | the field's `reader_values` |
| config device slots (`sda` through `sdh`) | the `object_walk` key vocabulary on `devices` |

A new language reads all three the way it reads everything else: through the shared rule
evaluator and the shared object walker. There is nothing to enroll and nothing to copy.
`sync-enums` checks each declaration against the live Linode API once in the scheduled
tier, rather than once per language.

## Scopes are declared, not mapped

Every tool's required OAuth scopes live on its proto input message: `tool_scopes` declares
either the documented scope list or an explicit `none` for a route documented without any.
`go/cmd/toolgen` resolves the declaration once and renders a per-language registry table
(`gentools.ScopesFor`, `linodemcp.gentools.scopes_for`), which the tool catalog, profile
resolution, and the parity dumper all read. A new language writes no scope code: its
renderer arm emits the same table from the same contract. Two guards hold it in place.
**The parity dumper must emit `scopes`**, since `tool-parity` diffs the rendered scope list
per tool across languages and refuses a surface with zero scopes. **The emitter refuses** a
routed tool declaring neither `none` nor a scope, by tool name, so a family cannot ship
silently unrestricted in any language, registered or not.

There is no sync-gate enrollment step: `sync-scopes` reads Python alone and `tool-parity`
pins every other language equal to Python, so the new language is covered transitively.
New routed tools need three options on their proto input message, whatever language adds
them: `tool_capability`, `tool_route`, and `tool_scopes`; a tool that reaches no Linode API
declares `tool_meta` instead of the route and carries no scopes.

## Where the argument checks live

The North Star was nothing handwritten, and for argument checks it is reached:
`docs/contracts/hand-validator-counts.txt` reads zero in both languages. Every check a tool
makes is declared on its `*Input` message, in one of three forms, and a new language
implements the three readers rather than any tool's check:

- a `buf.validate` message rule, read by `go/internal/toolvalidate` and
  `linodemcp.tools.constraints`;
- an `argument_reader` on a field, with `reader_message` wording its refusals;
- `refuse_arguments` and `require_any_of` over the whole argument map;
- an `object_walk` over a map or repeated-Struct argument, read by `go/internal/toolwalk`
  and `linodemcp.tools.objectwalk`.

One check is not declared at all. Every tool refuses an argument its input message does
not declare, in one sentence the engine formats: `Unsupported argument(s) for <tool>:
<names sorted, joined with ", ">`. A language holds one copy of that sentence and one copy
of the allowlist, which is the message's own field names plus `confirm_bypass_dry_run`,
`confirmed_dry_run` and `yolo`; those three are read off the argument map by the server and
the destroy gate and no input message declares them. The behavior fixtures are what hold
the copies to the same words, so a new language earns its wording by passing them.

The check has to reach every tool, not every emitted handler. Where a language runs a tier
through a shared driver rather than an emitted body, the driver is where the check goes:
Go's list tiers call `tools.CheckArgumentRefusals` inside
`go/internal/tools/gentools_seam.go` because `emitList` writes a driver call and no handler,
so an emitter-only check would leave every Go list tool dropping arguments while Python
refused them, and no gate would notice.
Go's 25 single-id destroys reach it the same way, from `RunDestructiveActionWithID` in
`go/internal/tools/destroy.go`, because their emitted handler is one struct literal and the
wrapper is what holds the message name.
`refuse_arguments` rides in the same functions for the same reason, read off the descriptor
rather than written into each tool.
Counting emitted calls does not prove this: a tier that emits neither the rules check nor
the refusal moves both counts by zero. `TestEveryGeneratedFactoryReachesTheRefusal`
(`go/cmd/toolgen/argument_refusal_emit_test.go`) walks every factory the emitted registry
lists into the engine instead, so a new tier that skips the check fails by tool name.

A caller who sends a flag the tool does not declare now reads that refusal instead of
having it dropped. Both CLIs fold `--environment`, `--dry-run`, `--mode` and `--plan-id`
into the argument map whenever the flag is set, and 17 meta tools declare no `environment`,
269 tools no `dry_run`, and 493 no `mode`, so those combinations are refused rather than
ignored. That matches what the published input schemas already said: every one of them sets
`additionalProperties: false`.

A check none of the three can carry has nowhere left to go but a hand-written body, and
`make hand-validators` fails in both directions on one, naming the site it found.
`hand-code` covers the rest of that boundary: a function named after a tool in a
non-generated tree fails by name, whatever it does.

## The gates

[The check gates](./gates.md) tables every gate in both tiers, one row each; this page does
not restate that list. What matters for onboarding:

- `tool-parity` and the classifiers behind `generated-form` read every language in
  `docs/contracts/languages.txt`, so registering yours is what enrolls you.
- the baseline guard is diff-aware and CI-only: `make check` reads committed state and
  cannot see direction.
- the scheduled tier reads the contract rather than any language, so there is nothing to
  register there.

A new language is fully enrolled when it passes every per-commit gate.

## Definition of done

- `make proto` then `make check` is green with the new language's lint plus tests plus all
  per-commit gates included.
- The new language has a behavior-conformance runner passing every fixture in
  `testdata/behavior/`, dry-run preview cases included.
- The new language is registered in `docs/contracts/languages.txt` with a working parity
  dumper, and that dumper emits `scopes` read from its generated registry table.
- `make parity-todo` shows an empty (or fully annotated and shrinking) list for the new
  language, and every accepted absence carries its tracking annotation.
- Tool surface, capabilities, and manifest match; no hand-written input schemas or output
  shapes anywhere in the new language.
- The contract runtime exists (route builder with the shared escaping vector table, option
  readers, the three drivers, the local-answer and transport engines), the language has a
  renderer arm in `go/cmd/toolgen` that answers for every option the contract declares,
  every tool outside `docs/contracts/handwritten-tools.txt` is generated with zero
  hand-written per-tool code, and the language has a scanner arm in the route-source and
  route-evidence gates.
