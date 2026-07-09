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

Two generated trees feed every language:

- **`genpb`**: the message types (`buf generate` produces protobuf runtime types per
  language).
- **`toolschemas`**: the MCP input JSON Schemas (`<full.msg.name>.schema.strict.json`),
  emitted by the same `buf generate`. A tool advertises its input by loading its strict
  schema, so every language advertises an identical input contract without agreeing on
  anything by hand.

Run `make proto` first; the generated trees are gitignored and nothing builds without them.

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
   `docs/tools-manifest.txt` plus the manifest tests (Go `TestToolSurfaceMatchesManifest`,
   Python `test_tools_manifest.py`) and `docs/tools-capabilities.txt`. Adding or renaming a
   tool requires a manifest update or the gate fails.

4. **Implement a behavior-conformance runner.** This is the most important one. The
   fixtures in `testdata/behavior/*.json` are language-agnostic: each says "input X → this
   exact bare error" or "input X → this HTTP method/path/body". Your language needs a
   runner that drives its own dispatch path against every fixture and asserts the same
   result, the way `go/internal/server/behavior_conformance_test.go` and
   `python/tests/unit/test_behavior_conformance.py` already do. Enforced by **`behavior`**.
   This is where validation logic, error-message text, request bodies, and confirm gates
   are pinned across languages. If your language rejects an invalid input with a different
   message, this gate catches it.

5. **Match confirm-text.** Enforced by **`messages`** (cross-language confirm-message
   parity).

6. **Wire it into `make check` and CI** so all of the above run on every change.

## The one thing that is NOT auto-generated: the hand-lists

Proto enums generate for free: a new language gets the 21 enum value sets from `genpb`
with no extra work. But three validation value-sets **cannot** be proto enums, because
their values are not valid proto identifiers (`public-read` has a hyphen,
`anti_affinity:local` a colon) or they are map keys rather than a scalar field (config
device slots `sda`–`sdh`). Until protovalidate lands (see below), these stay hand-written
in each language:

| value set | what it validates |
|---|---|
| bucket ACL (`private`, `public-read`, `authenticated-read`, `public-read-write`) | object-storage bucket/object ACL input |
| placement group type (`anti_affinity:local`) | placement-group create |
| config device slots (`sda`–`sdh`) | instance-config `devices` keys |

For a new language, these need **two** things, and the second is easy to forget:

1. **Implement the validation with the identical error message** the behavior fixtures pin
   (for example `acl must be one of: private, public-read, authenticated-read,
   public-read-write`, or `device slot <slot> must be one of sda through sdh`). The
   `behavior` gate catches you per-commit if the message or the rejection differs, but only
   for the specific cases the fixtures pin, not the full value set.

2. **Register the language in the sync gate** so its *full* value set is cross-checked
   against the other languages and the live Linode API. Today `scripts/verify_sync_enums.py`
   holds `HAND_LIST_SPEC_MAP`, which extracts Go's lists (via `go/cmd/hand-list-dump`, a
   `go/ast` tool) and Python's lists (via Python `ast`), then diffs every language against
   the spec and against each other. A new language must add its own source extractor and
   plug into that map. **The map is currently shaped for exactly two languages (`spec` plus
   `py`, with Go implied by the extractor binary); adding a third language means
   generalizing it to an arbitrary set of language extractors.** This is the one manual
   enrollment step with no gate to remind you. If you skip it, the scheduled sync gate
   silently never checks your language's hand-lists. Do it when you add the language.

## The endgame (so you know these hand-lists are temporary)

The North Star is nothing handwritten. The permanent fix for the three hand-lists is
`buf.validate`'s `(buf.validate.field).string.in = [...]`, which carries an
arbitrary-string set on the proto field and generates validators for every language. That
is the Phase-9 protovalidate work. When it lands, the hand-lists, `go/cmd/hand-list-dump`,
and `HAND_LIST_SPEC_MAP` all go away, and a new language gets these sets for free like any
other enum. Until then, treat the hand-lists as the one place a new language does manual,
gated work.

## The gates, and which threat each catches

Two tiers. The **per-commit** tier runs in `make check` and the pre-commit/pre-push hooks;
it blocks a push. The **scheduled** tier needs the network (it fetches the live API spec)
and runs on a cron, not on every change.

| Gate | Tier | Catches |
|---|---|---|
| `tool-parity`, `input-proto`, `read-proto`, `write-proto`, `meta-proto` | per-commit | schema/surface/output-shape drift between languages |
| `behavior` | per-commit | per-input validation, error-message text, request bodies, confirm gates: the cross-language contract |
| `messages` | per-commit | confirm-text parity |
| `sync-enums` / `sync-defaults` | scheduled | proto enums plus hand-list value **sets** plus defaults vs the live Linode API, and every language's set vs every other |

A new language is fully enrolled when it passes every per-commit gate *and* is registered
in the scheduled `sync-enums` hand-list map.

## Definition of done

- `make proto` then `make check` is green with the new language's lint plus tests plus all
  per-commit gates included.
- The new language has a behavior-conformance runner passing every fixture in
  `testdata/behavior/`.
- The new language is registered in `HAND_LIST_SPEC_MAP` (or protovalidate has landed and
  the hand-lists are gone).
- Tool surface, capabilities, and manifest match; no hand-written input schemas or output
  shapes anywhere in the new language.
