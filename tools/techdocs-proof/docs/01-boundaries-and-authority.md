# 1. Boundaries and Authority

## Why this proof exists

A documentation monitor sounds simple: fetch the docs, inspect the implementation, report differences. The trouble starts with the phrase “inspect the implementation.”

LinodeMCP has Go and Python implementations generated around a protobuf contract. If a monitor scans those languages directly, it inherits their parser versions, syntax changes, generated-code shapes, helper conventions, and build layout. A monitor can then fail because a source parser is old even when the API documentation and protobuf are both valid.

That failure mode says the monitor chose the wrong boundary.

The right boundary is contractual:

```text
published API behavior represented by rendered TechDocs
                         versus
MCP tool inputs represented by protobuf descriptors
```

The comparison should not care which Python syntax is current or how a Go helper formats an HTTP path. LinodeMCP CI owns generated-language correctness. This proof owns documentation-to-protobuf consistency.

## The three authorities

### 1. External API authority: rendered TechDocs

Rendered TechDocs pages are the source for externally documented API facts:

- HTTP method;
- route path;
- path parameters;
- query parameters;
- body parameters;
- documented types;
- requiredness;
- default values;
- allowed values;
- route deprecation;
- parameter deprecation;
- response statuses;
- displayed replacement text for deprecated operations.

“Rendered” is important. The proof compares what a reader can retrieve from the published documentation site. It does not use an unpublished generator input as a substitute.

OpenAPI is not consulted as a fallback, tie breaker, validator, or repair source. If OpenAPI differs from rendered TechDocs, that is a separate investigation. Pulling OpenAPI into this proof would create two external authorities and make every disagreement ambiguous.

That rule costs something, and the cost is worth naming. A handful of operations render their Body Params section behind a client-side switcher: the HTML carries one variant's fields, and the rest are built in the browser from an OpenAPI payload embedded in the page. Reading that payload would recover them. It would also make OpenAPI the authority for those routes and nothing else, which is the split this rule exists to prevent. The comparator records the variant labels the page does publish and reports the affected comparisons as limits instead. See chapter 3 and `techdocs_body_variant_unrendered` in chapter 6.

### 2. LinodeMCP tool authority: protobuf

Files under the LinodeMCP protobuf tree define tool request messages, field names, scalar and enum types, optionality signals, comments, and deprecation metadata.

Buf compiles that tree into a `FileDescriptorSet`. The descriptor is the comparison input because it provides a language-neutral view of:

- files and packages;
- messages and nested messages;
- fields and field numbers;
- scalar, enum, message, repeated, and map shapes;
- proto2/proto3 optional markers;
- enum values;
- deprecation options;
- source-code locations;
- leading comments.

The proof never imports generated Python or Go. It also never asks generated code to describe itself.

### 3. Tool-to-route association: protobuf message options

Every Linode API-backed `*Input` message carries a protobuf-owned route declaration:

```proto
message VolumeGetInput {
  option (linode.mcp.v1.tool_route) = {
    tool: "linode_volume_get"
    method: "GET"
    path: "/volumes/{p}"
  };
}
```

`options.proto` defines `ToolRoute` and extends `google.protobuf.MessageOptions` with field `50001`. Buf serializes the custom option into the descriptor, so the request message, tool name, method, and path arrive as one compiled contract record.

The proof requires exactly one valid extension definition. An annotation on a non-`Input` message, malformed method/path, duplicate tool, missing extension, or descriptor with no annotations stops the run. There is no route-map fallback and no name-based route inference.

## Ownership by stage

| Concern | Owner |
|---|---|
| Published HTTP contract | Rendered TechDocs |
| MCP request schema | LinodeMCP protobuf |
| Generated Go/Python consistency with protobuf | LinodeMCP code generation and CI |
| TechDocs-to-protobuf differences | This Phase 1 proof |
| Human classification and issue wording | Later workflow stage |
| Source edits and pull requests | Accepted-gated worker lifecycle |

The table prevents the monitor from becoming a second implementation test suite or an automatic source editor.

## Phase 1 output is evidence

A finding means:

```text
A fixed rule observed a difference or could not prove equivalence.
```

A finding does not mean:

```text
The API is broken.
```

For example, a protobuf field absent from TechDocs may be an internal control such as a confirmation flag. That can be valid, but validity must be declared in protobuf with a leading comment beginning exactly:

```text
System parameter:
```

Without that marker, the proof reports the field. The later investigator decides whether to add the marker or reconcile the field with TechDocs.

## The system-parameter contract

The marker is deliberately exact:

```text
System parameter:
```

It must begin the leading field comment. Case, spelling, punctuation, and position matter.

Accepted shape:

```proto
// System parameter: Selects the configured API environment.
// This value is consumed by the MCP transport and is not sent to the Linode API.
string environment = 1;
```

Not accepted:

```proto
// Internal field used by the system.
string environment = 1;
```

Also not accepted:

```proto
// This is a System parameter: used for confirmation.
bool confirm = 2;
```

The exact prefix turns an implicit exception into a reviewable protobuf-owned declaration. There is no name allowlist for `environment`, `confirm`, `dry_run`, or any other field.

`field_location` is the machine-readable form of the same declaration, and two of its values keep a field off the wire. `FIELD_LOCATION_LOCAL` is MCP plumbing and stays locked to the marker above. `FIELD_LOCATION_TOOL` is the tool's own domain argument, such as a profile name a meta tool builds or a local path an object-storage transfer reads; it reaches no Linode route, so there is nothing on the documented side to join it to. Both are left out of the comparison and counted in the summary, `system_parameters` and `tool_arguments` separately, because the marker covers one and not the other. A `field_location` value the map does not carry is still unreadable and still reported.

## Deprecation has its own boundary

A deprecated route is not equivalent to an active route missing from proto.

The proof separates three cases:

1. **Deprecated without a clear replacement.** If it remains represented in proto, report it for investigation or removal.
2. **Deprecated with a clear replacement.** Check both the old route and the advertised replacement.
3. **Unclear replacement text.** Do not infer a route from prose. Report ambiguity.

This prevents an old route from being silently retained and prevents a replacement from being assumed merely because its name looks similar.

## Trust model

The proof assumes:

- DNS and HTTPS return the published TechDocs pages;
- under `--github-source`, GitHub resolves the requested LinodeMCP ref to the intended immutable commit;
- Buf faithfully compiles the preserved protobuf module;
- each API-backed input message owns a valid `linode.mcp.v1.tool_route` option;
- local filesystem writes complete or fail visibly.

The proof does not assume:

- page discovery always succeeds;
- every page parses into an operation;
- a page that parses published every field it documents;
- every proto field has obvious requiredness, or that proto3 presence answers it;
- field names alone identify location;
- comments are present;
- all deprecation prose is machine-readable;
- a zero process exit means zero findings.

These assumptions drive the fail-closed behavior described in later pages.
