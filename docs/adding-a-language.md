# Adding a new language to LinodeMCP

This repo ships the same Linode MCP server in more than one language (today Go and
Python). They are not independent reimplementations that happen to agree. They are
byte-identical on the wire *by construction*, because everything derives from one
source: the protobuf contract in `proto/linode/mcp/v1`. A new language is worth adding
only if it holds that line. This guide is the checklist for doing that, and the list of
gates that fail you if you skip a step.

## The one rule

Everything a tool exposes (its input schema, its output shape, its enum value sets) comes
from the proto contract and its generated artifacts. You do not hand-write any of it. If
you find yourself typing a JSON schema, a field list, or an enum by hand in the new
language, stop: that is the drift this project exists to prevent.

Three generated trees feed every language:

- **`genpb`**: the message types (`buf generate` produces protobuf runtime types per
  language).
- **`toolschemas`**: the MCP input JSON Schemas (`<full.msg.name>.schema.strict.json`),
  emitted by the same `buf generate`. A tool advertises its input by loading its strict
  schema, so every language advertises an identical input contract without agreeing on
  anything by hand.
- **`gentools`**: the per-tool factory and handler code, emitted by `go/cmd/toolgen`
  from the compiled descriptors, one renderer arm per registered language.
  Generated is the DEFAULT: every tool the contract declares is generated except
  the ones `docs/contracts/handwritten-tools.txt` still claims, so a new tool is
  one proto message and reaches every language with no line in any registry. A generated
  tool has NO hand-written per-tool code in any language: its route, capability tier,
  description, confirm and success and error prose, response binding, argument
  extraction, filters, hooks, and list envelope all come off the proto options
  (`tool_route`, `field_location`, `tool_capability`, `tool_response`,
  `confirm_message`, `success_message`, `resource_type`, `retry_disabled`,
  `tool_description`, `error_message`, `tool_hooks`). The `generated-tools` gate ratchets
  the hand-written list downward and fails new surface written by hand.
- **The tool registries**: `docs/contracts/tools-manifest.txt` and
  `docs/contracts/tools-capabilities.txt`, written by `scripts/gen_tool_registries.py`
  inside the same `make proto`. Both are the descriptors' own answer to "which tools
  exist and what tier is each", so neither is edited.

Run `make proto` first; the generated trees are gitignored and nothing builds without them.

## Step zero: register the language

`docs/contracts/languages.txt` is the registry the cross-language gates read. One line per
implementation: `<name>\t<working-dir>\t<dump command>`, where the command prints the
language's tool surface as JSON records (see `go/cmd/parity-dump` and
`python -m linodemcp.parity_dump` for the record shape). Registering the language is what
turns the parity gates on for it: a freshly registered language with no tools yet fails
`tool-parity` with one `missing in <name>` line per manifest tool.

That failure list is the onboarding plan, and `docs/contracts/tool-parity-baseline.txt`
is where it lives. It is the one ratchet built for this: accept the seeded absences with
`--update-baseline`, then annotate each line (`<entry>  # accepted YYYY-MM-DD
<tracking-issue URL>`); the annotation is required for absences, and the CI baseline
guard (`.github/workflows/baseline-guard.yml`, locally `make baseline-guard`) blocks any
future baseline growth that lacks one. From there, `make parity-todo` renders the
per-language remaining-work view, and every tool you implement shrinks the baseline (the
gate fails on stale entries, so the list stays honest). Line order in
`docs/contracts/languages.txt` matters: the first language that implements a tool is the
reference its contract is diffed against, so keep the most complete implementation first.

The other gates have no such list. A finding fails them outright, which changes the order
of the work rather than the amount: enroll in a gate when the language can pass it, not
before. The one place that bites during onboarding is `route-evidence` (step 7), because
registering the language makes it demand a scanner arm by name and the arm then has to
resolve every declared route. Until the client can build them, the language's unbuilt
surface is recorded once, as annotated absences in `tool-parity-baseline.txt`.

## What a new language must implement

Each item lists the gate that enforces it, so you know what "done" is checked by.

1. **Consume the generated input schemas.** Build every tool's advertised input from its
   `*.schema.strict.json`, not from code. Enforced by **`tool-parity`** (surface, params,
   required, types) and **`input-proto`** (every tool's input schema is proto-generated,
   no hand-built schema).

2. **Emit proto-canonical output.** Serialize every response through the generated message
   (the equivalent of Go's `MarshalProtoToolResponse`), so the JSON on the wire is
   identical across languages. Enforced by **`write-proto`**, **`read-proto`**, and
   **`meta-proto`** (every write/read/meta handler routes output through a proto message,
   zero hand-built wire shapes).

3. **Match the tool surface exactly.** Same tool names, same capabilities. Enforced by
   `docs/contracts/tools-manifest.txt` plus the manifest tests (Go `TestToolSurfaceMatchesManifest`,
   Python `test_tools_manifest.py`) and `docs/contracts/tools-capabilities.txt`. Both files are
   generated from the proto by `make proto` and gitignored, so a tool is added by declaring it
   and never by editing them. The manifest lists the FULL surface; a tool your language has not
   caught up on yet lives in `docs/contracts/tool-parity-baseline.txt` as an annotated
   `missing in <name>` entry, never as a manifest omission.

4. **Implement a behavior-conformance runner.** This is the most important one. The
   fixtures in `testdata/behavior/*.json` are language-agnostic: each says "input X → this
   exact bare error", "input X → this HTTP method/path/body", or "input X against these
   faked API responses → this exact result content". Your language needs a runner that
   drives its own dispatch path against every fixture and asserts the same outcome, the
   way `go/internal/server/behavior_conformance_test.go` and
   `python/tests/unit/test_behavior_conformance.py` already do, including the routed
   `api_responses` fakes, the `expect_result` JSON comparison, and the rule that a
   `dry_run: true` case may only issue GETs. Enforced by **`behavior`**, which also
   requires every Destroy tool's fixture to carry a dry-run preview case
   (`docs/contracts/behavior-dryrun-baseline.txt` ratchets any gap): a preview whose
   prose the contract does not declare is written per language, so an unpinned one is
   exactly where drift hides. If your language
   rejects an invalid input with a different message, or renders a different preview for
   the same faked state, this gate catches it.

5. **Implement the contract runtime, once.** This is the layer every generated tool
   calls, and it is the whole per-language cost of the surface:
   - a route builder reading `tool_route` from the descriptors (the shape of
     `go/internal/linoderoute` and `python/src/linodemcp/linode/routes.py`), with the
     shared path-escaping contract: RFC 3986 unreserved plus a literal colon, uppercase
     hex, all-dots values percent-encoded. Both existing languages pin an identical
     escaping vector table in their route tests; port that table verbatim, it IS the
     contract.
   - **API surface selection**, read from `tool_api_surface` on the same message as the
     route. Nearly every route answers under the configured base; the tools in
     `docs/contracts/api-surfaces.txt` answer under another, and a language that skips
     this silently 404s every one of them. Three parts, all shared: swap the base's
     trailing `/v4` for the declared surface and use any other configured base exactly
     as it is (the per-environment `apiUrl` override wins, since a surface picks among
     versions of one deployment and never picks the deployment); refuse a surface the
     build cannot address rather than defaulting it; and lead the advertised description
     with a `[<surface>]` marker so a caller reading a tool listing can tell. Enforced
     by **`api-surfaces`**, which holds the census, the contract, and every registered
     language's descriptions to one answer in both directions, and by
     `expect_request.api_surface` in the behavior fixtures, which defaults to v4 so a
     tool that moved surfaces without its fixture moving with it fails.
   - a runtime reader for whatever tool options your arm does not spell into the
     code it emits: capability, response binding, confirm/success/error prose,
     resource type, retry policy, description. The two existing languages split this
     differently and both are honest. Go emits the declared sentence into the factory;
     Python's driver reads the same option off the descriptor when the tool is called,
     through `contract_for`. Your arm states which way it went, option by option, in
     `acts` (step 6).
   - the driver tier: list, write, and destructive drivers the generated code
     configures (Go's list factories and destroy drivers; Python's
     `linodemcp.tools.drivers`). Drivers own pagination, dry-run ordering (dry-run
     validates before confirm; live gates confirm before validating), the confirm and
     destroy gates, retry policy, and response serialization.
   - a hooks module bound to the `tool_hooks` option: per-tool bespoke logic under a
     function name derived from the tool and the kind, called by generated code so a
     missing hook fails the build or import, never a runtime call. The kinds today are
     `normalize` (a rewrite of the arguments in place before anything reads them; most
     of that cohort is a `normalize_fields` declaration naming a transform instead of a
     hook, so what is left is the rewrites no transform spells), `validate`
     (argument checks), `preview` (a mutation's dry run), `fetch_state` (the state a
     plan hashes for drift, left to a hook only where the read is imperative:
     most removals name their read with the `state_route` option instead, and a
     removal declared beside a GET on its own route has that read derived for
     it), `dependency_walk` (what else the change touches, reading the state
     that fetch produced), `execute` (the live
     call itself, for a route whose
     request is not the JSON body the emitter derives), and `answer` (a meta tool's
     whole result, read from local state rather than any route). A declared kind is a
     fact about the tool, so every language implements it. Prose is what a preview hook
     owns where the contract cannot declare it: a hook is handed the resolved call and
     the assembled body, so it never re-spells a route or rebuilds a request. The prose
     a tool can state outright is declared instead, through `preview_sentence`, and both
     arms emit it: your language needs the two readers, the sentence chooser and the
     filter that drops a line none of whose wordings could be filled
     (`go/internal/tools/preview_sentence.go`,
     `python/src/linodemcp/tools/preview.py`), not a hook per tool. The resource a dry
     run reports is declared the same way a removal declares its own, through
     `state_route`, so most previews name a read rather than writing one. `execute` is
     one exception and replaces the call, which is why only the acknowledge tier
     declares it: that tier assembles its answer from
     the call rather than decoding one, so the hook owes nothing back but an error.
     `answer` is the other, and only the meta tier declares it: those tools reach no
     route at all, so the hook takes no client and returns the result itself.

6. **Write a renderer arm.** There is one emitter, `go/cmd/toolgen`, and it has two
   halves. The front half reads the compiled descriptors once into a language-neutral
   contract model. Everything below that seam turns the model into one language's
   files. A new language is an arm below the seam, never a second program reading the
   descriptors again, so you inherit the derivation rules instead of restating them:
   response binding comes from `tool_response` only (never derived from names), tier
   from the capability and then the response shape (destroy is read off the capability
   first, since its answer is built from the call rather than decoded), list envelope
   from the response's single repeated field, filters from QUERY fields matched against
   element fields, and every prose string from its option. A mutation whose declared
   response is page-shaped is decoded through the list envelope rather than into that
   response, since its route reports the collection that now exists. A BODY member
   carrying `body_fold` is assembled from the flat arguments beside it rather than taken
   whole, and those arguments stop traveling at the top level; a key the caller wrote
   wins over a folded one. All of that is settled before your arm is called.

   The arm is the `renderer` interface in `go/cmd/toolgen/renderer.go`. Five methods:
   `renderGroup` (the file holding every tool declared in one proto source file),
   `renderRegistry` (the file the server reads to register the cohort, emitted from that
   same cohort because a tool nothing registers passes every contract gate and is still
   missing from the running server), `owns` (which files in the output tree were yours,
   so a run clears its own stale output and nothing else), `language` (the name
   `docs/contracts/languages.txt` registers you under), and `acts`. Add a case for the
   name to `armFor` in `languages.go`: a registered language with no arm there stops the
   run by name rather than being quietly skipped. Read `gorenderer.go` and
   `pyrenderer.go` with its `py*.go` family before you start.

   `acts` is the one to get right. It is your arm's answer for EVERY option the contract
   declares, and that set is read live out of the compiled descriptors rather than kept
   as a list, so an option added to the proto joins it the moment it compiles. Each
   answer says one of two things: my rendered bytes move with this option, or this named
   file in my language's support layer acts it. Say nothing about an option and the run
   stops before one tree is rewritten. That is the failure the guard exists for, an
   option taught to one arm and left out of another, which gives you trees that compile,
   register the same tools, and quietly do different things. Arms are allowed to
   disagree about WHERE an option is acted (Go emits a declared sentence into the
   factory, Python's driver reads that same option off the descriptor at call time), so
   the check is that each arm answers at all, not that the answers match.

   Your language also needs the support layer the emitted code calls into: Go's
   `go/internal/tools`, Python's `linodemcp.tools`. A rendered `TrimArguments`,
   `PreviewSentence`, or `WriteBody` lives there, and that layer is what an `acts` claim
   names when your arm does not emit the option itself. A declared state read lives
   there too (`go/internal/tools/declared_state.go`,
   `python/src/linodemcp/tools/declared_state.py`): a removal's state is the body the
   API sent projected through the read's own message, so a key it sent as null survives
   at any depth and a key it never sent is not invented. Your language needs that
   projection, the three readers a dependency walk uses over it, and the refusal that
   keeps a walk from being handed a state some other fetch produced. The Python arm renders through
   the repo's own ruff, so a new arm may shell out to its language's formatter the same
   way. The emitter refuses, by tool name, anything under-declared.
   Enforced by **`generated-tools`** and the emitter's own coverage guard.

7. **Enroll in the route-evidence scanners.** `make route-source` and
   `make route-evidence` need a scanner arm for your language (the job
   `scripts/_routescan.py` does for Python and `go/cmd/route-dump` does for Go): which
   call sites build requests, which primitives resolve routes from the contract, and
   which routes therefore have evidence. A registered language with no scanner arm
   fails by name, and once the arm exists every declared route must resolve through it:
   `route-evidence` keeps no list of routes a client cannot build yet.

8. **Match confirm-text.** Enforced by **`messages`** (cross-language confirm-message
   parity).

9. **Wire it into `make check` and CI** so all of the above run on every change.

## The value sets that are not proto enums

Proto enums generate for free: every enum the contract declares reaches a new language
through `genpb` with no extra work. Three validation value-sets **cannot** be proto enums, because their
values are not valid proto identifiers (`public-read` has a hyphen, `anti_affinity:local`
a colon) or they are map keys rather than a scalar field (config device slots `sda`
through `sdh`). They used to be hand-written per language. They are not any more:

| value set | where the contract carries it |
|---|---|
| bucket ACL (`private`, `public-read`, `authenticated-read`, `public-read-write`) | a CEL alternation on the bucket-access rule |
| placement group type (`anti_affinity:local`) | the field's `reader_values` |
| config device slots (`sda` through `sdh`) | the `object_walk` key vocabulary on `devices` |

A new language reads all three the same way it reads everything else: through the shared
rule evaluator and the shared object walker. There is nothing to enroll and nothing to
copy. `scripts/verify_sync_enums.py` diffs each declaration against the live Linode API
in the scheduled tier, once, rather than once per language.

## The scope mapping is also hand-written

Every language carries a per-tool OAuth scope mapping (`go/internal/profiles/scope.go`,
`python/src/linodemcp/profiles/scope.py`): a name-prefix table, a documented-scopeless
list, and a small override table for routes whose documented scope the prefix table
cannot derive. A new language must mirror all three, and two gates hold it in place:

1. **The parity dumper must emit `scopes`.** `tool-parity` diffs the resolved scope list
   per tool across languages, so a mapping that disagrees with the reference language
   fails per-commit. A dumper that stops emitting the field entirely also fails (the
   gate refuses a surface with zero scopes).
2. **Port the scope completeness test.** Cross-language parity cannot see a family that
   every language forgot together, so each language pins its own registry: every
   non-meta tool resolves at least one scope or sits on the documented scopeless list
   (see `go/internal/server/scope_completeness_test.go` and
   `python/tests/unit/test_scope_completeness.py`). This test is per-language by design;
   a new language ships one alongside its mapping.

There is no sync-gate enrollment step here: the scheduled `sync-scopes` gate compares
Python's mapping against the live spec's per-operation security blocks (routed through
the proto contract's `tool_route` options), and `tool-parity` pins every other language
equal to Python, so the docs comparison covers the new language transitively. New tools
DO need two options on their proto input message, whatever language adds them: a
`tool_capability` naming the tier, and either a `tool_route` or, for a tool that reaches
no Linode API at all, a `tool_meta` naming the tool. `make tool-routes` and
`make tool-capability` fail loudly when one is missing, and the capability gate also
fails a tool whose proto tier and `tools-capabilities.txt` tier disagree, which under
generated registries means the file predates the contract rather than that someone
mistyped it.

## Where the argument checks live

The North Star was nothing handwritten, and for argument checks it is reached:
`docs/contracts/hand-validator-counts.txt` reads zero in both languages. Every check a
tool makes is declared on its `*Input` message, in one of four forms, and a new language
implements the four readers rather than any tool's check:

- a `buf.validate` message rule, read by `go/internal/toolvalidate` and
  `linodemcp.tools.constraints`;
- an `argument_reader` on a field, with `reader_message` wording its refusals;
- `refuse_arguments`, `refuse_unknown_arguments`, and `require_any_of` over the whole
  argument map;
- an `object_walk` over a map or repeated-Struct argument, read by
  `go/internal/toolwalk` and `linodemcp.tools.objectwalk`.

A tool may still declare a `validate` hook for a check none of the four can carry. None
does today, and `make hand-validators` fails in both directions, so one that comes back
has to lower the line in the same change.

## The gates, and which threat each catches

Two tiers. The **per-commit** tier runs in `make check` and the pre-commit/pre-push hooks;
it blocks a push. The **scheduled** tier needs the network (it fetches the live API spec)
and runs on a cron, not on every change.

| Gate | Tier | Catches |
|---|---|---|
| `tool-parity`, `input-proto`, `read-proto`, `write-proto`, `meta-proto` | per-commit | schema/surface/output-shape drift between languages (tool-parity reads every language in `docs/contracts/languages.txt`; the four proto classifiers are pairwise today and grow with the language) |
| `behavior` | per-commit | per-input validation, error-message text, request bodies, confirm gates, and dry-run preview content: the cross-language contract |
| `messages` | per-commit | confirm-text parity |
| `generated-tools` | per-commit | a tool outside `docs/contracts/handwritten-tools.txt` that a language does not generate (which is how new surface written by hand fails), a hand-written factory left behind for a generated tool (both registries scan, so a leftover stages the tool twice), a listed name no tree holds, and the count of tools each language still serves by hand, which only falls |
| `hand-validators` | per-commit | a declared `validate` hook a language's hook tree does not implement, a validate function no tool declares, and the count of tools each language still checks the arguments of by hand, which only falls |
| `hook-bodies` | per-commit | the same two directions for every other `tool_hooks` kind, plus the count of hand-written bodies per language per kind, which only falls; a registered language with no hook tree fails by name |
| baseline guard | per-change (CI only) | baseline growth without an `accepted YYYY-MM-DD` annotation; `make check` reads committed state and cannot see direction, so this one check is diff-aware |
| `sync-enums` / `sync-defaults` | scheduled | proto enums plus the contract's other declared value **sets** plus defaults vs the live Linode API |
| `sync-scopes` | scheduled | the per-tool OAuth scope mapping vs the live spec's per-operation security blocks; catches all languages drifting from the docs together, which `tool-parity` cannot see |

A new language is fully enrolled when it passes every per-commit gate. The scheduled
tier reads the contract rather than any language, so there is nothing to register there.

## Definition of done

- `make proto` then `make check` is green with the new language's lint plus tests plus all
  per-commit gates included.
- The new language has a behavior-conformance runner passing every fixture in
  `testdata/behavior/`, dry-run preview cases included.
- The new language is registered in `docs/contracts/languages.txt` with a working parity dumper.
- `make parity-todo` shows an empty (or fully annotated and shrinking) list for the new
  language, and every accepted absence carries its tracking annotation.
- The new language has its scope mapping (prefix table, scopeless list, overrides), its
  parity dumper emits `scopes`, and its scope completeness test passes.
- Tool surface, capabilities, and manifest match; no hand-written input schemas or output
  shapes anywhere in the new language.
- The contract runtime exists (route builder with the shared escaping vector table,
  option readers, the three drivers, the hooks module), the language has a renderer arm
  in `go/cmd/toolgen` that answers for every option the contract declares, every tool
  outside `docs/contracts/handwritten-tools.txt` is generated with zero hand-written
  per-tool code, and the language has a scanner arm in the route-source and
  route-evidence gates.
