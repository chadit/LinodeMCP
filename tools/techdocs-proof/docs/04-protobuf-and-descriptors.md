# 4. Protobuf and Descriptors

The proof creates two descriptor sets:

```text
TechDocs candidate proto -> techdocs-descriptor.json
LinodeMCP proto tree      -> linodemcp-descriptor.json
```

They serve different purposes. The candidate descriptor proves that the TechDocs-derived protobuf artifact is valid and inspectable. The LinodeMCP descriptor is the language-neutral input to the actual comparison.

## Why descriptors

A `.proto` file is source text. A `FileDescriptorSet` is the compiler's structured interpretation of that source.

Descriptors remove irrelevant differences such as:

- whitespace;
- declaration order where semantics do not depend on it;
- source file layout;
- package directory layout;
- generated-language syntax;
- generated helper naming.

They preserve facts the proof needs:

- message and field identity;
- scalar and message types;
- repeated and map structure;
- enums;
- field options;
- optional markers;
- deprecation options;
- source locations and leading comments.

## The generated TechDocs proto

The candidate proto lives at:

```text
generated-proto/techdocs_contract.proto
```

It is not proposed LinodeMCP source. It is a durable representation of what the rendered documentation said during the run.

The file declares custom options resembling:

```proto
message OperationContract {
  string api_version = 1;
  string method = 2;
  string path = 3;
  bool is_deprecated = 4;
  string source_url = 5;
  repeated int32 status_codes = 6;
}

message ParameterContract {
  string location = 1;
  string documented_type = 2;
  bool is_required = 3;
  bool has_default = 4;
  string default_json = 5;
  repeated string enum_values = 6;
  bool is_deprecated = 7;
}
```

These messages extend protobuf message and field options. Each rendered operation becomes a request message with an operation option. Each rendered parameter becomes a field with a parameter option.

Conceptual example:

```proto
message GetWidgetRequest {
  option (operation) = {
    api_version: "v4"
    method: "GET"
    path: "/v4/widgets/{param}"
    is_deprecated: false
    source_url: "https://techdocs.akamai.com/.../get-widget"
    status_codes: 200
  };

  int64 widget_id = 1 [(api_parameter) = {
    location: "path"
    documented_type: "integer"
    is_required: true
    has_default: false
    is_deprecated: false
  }];

  optional int64 page = 2 [(api_parameter) = {
    location: "query"
    documented_type: "integer"
    is_required: false
    has_default: true
    default_json: "1"
    is_deprecated: false
  }];
}
```

The same endpoint list renders the same file, byte for byte.

## Mapping rendered types into candidate proto fields

The candidate needs compilable protobuf types, but the rendered type remains preserved in the custom option.

| Rendered family | Candidate protobuf type |
|---|---|
| boolean | `bool` |
| integer | `int64` |
| number | `double` |
| string, password, UUID, date-time | `string` |
| object, map, unknown structured type | `google.protobuf.Value` |
| array of a known scalar | `repeated <scalar>` |
| generic array or array of objects | `repeated google.protobuf.Value` |

This mapping is not used to declare TechDocs equivalent to LinodeMCP. The comparator still uses its semantic type normalization rules and retains the raw rendered type.

## Candidate field names

Rendered names are normalized to protobuf-safe snake case. Reserved words receive a suffix. Duplicate normalized names gain a location suffix and then a numeric suffix if needed.

The original rendered name remains represented in the source endpoint and normalized contract. Candidate field naming is only a compilation concern.

## Candidate compilation

The generated directory includes a minimal Buf module:

```yaml
version: v2
modules:
  - path: .
```

Buf compiles the candidate into:

```text
techdocs-descriptor.json
```

The build must succeed. A malformed generated proto fails the run before LinodeMCP comparison.

## Compiling LinodeMCP

The default checkout is:

```text
~/Projects/src/github.com/chadit/LinodeMCP
```

The script resolves `buf` from the process path, then checks known cloud development-tool locations. It executes:

```text
buf build --as-file-descriptor-set --output <run>/linodemcp-descriptor.json
```

The command runs at the repository root, so `buf.yaml`, module dependencies, and the checked-out proto tree define compilation.

The proof records the Git commit returned by:

```text
git rev-parse HEAD
```

The commit identifies the baseline, but the operator should also ensure relevant protobuf and route-map files are not modified in the worktree when a production proof is run.

## Source comments are contract data

Most comments are not semantic comparison inputs. One exact comment prefix is:

```text
System parameter:
```

The descriptor must include `sourceCodeInfo.location` records so the extractor can map leading comments back to fields.

A field absent from TechDocs is exempt only when its normalized leading comment begins with that exact prefix. This means descriptor builds without source comments cannot safely classify internal fields.

## Extracting messages and enums

The extractor builds qualified indexes across files and nested messages:

```text
.<package>.<parent>.<message>
.<package>.<parent>.<enum>
```

It ignores synthetic map-entry messages as ordinary request messages while retaining map semantics on the owning field.

Enum values named like `UNSPECIFIED` are excluded from documented allowed-value comparison because they generally represent protobuf initialization rather than an API option. Remaining enum values are normalized before comparison.

A scalar field has no enum descriptor, and giving it one would change its wire
type, so LinodeMCP declares a documented value set on a string or integer field
in one of two places the descriptor already carries. A `(buf.validate.message).cel`
rule named `<tool>.<field>.known` whose expression tests
`this.<field> in ['a', 'b']` (or `in [1, 2, 3]`) is the set the handler runs;
the extractor reads the list literal, string members without their quotes and
integer members as digits, so both compare against the rendered `Allowed:`
values as strings. A field carrying the `reader_values` option is held to that
vocabulary by its ENUM_MEMBER reader instead, and the extractor reads the option.
An enum descriptor wins over both. A rule the extractor cannot read as a
membership list on the named field (a range, a member rule such as
`saml.identity_element.known`, a list holding anything but literals) declares
no set, so the field keeps reporting as a free scalar rather than as a wrong set.

## Requiredness evidence

Protobuf requiredness is not a single boolean in modern proto syntax.

The extractor considers, in this order:

- a `buf.validate` message rule whose identifier names the field and whose
  expression reads it, which is the requirement the handler enforces;
- proto3 optional presence;
- message-field presence, which proto3 tracks with or without the keyword;
- repeated/map semantics;
- ambiguity when no reliable signal proves required or optional.

Buf serializes `(buf.validate.message).cel` into the descriptor, so the rules
arrive with everything else and no source parsing is involved. LinodeMCP names a
rule after the field it constrains, `sshkey_create.label.required`, which is what
makes the identifier readable; a member rule such as
`iam_idp_config_create.saml.entity_id.required` names the member and its
expression reads `this.saml.entity_id`, so it never lands on the parent.

When requiredness cannot be proved, the proof emits `proto_requiredness_ambiguous` rather than guessing from a scalar type or field name.

## Tool-to-request association

The association is attached to the request message itself through the `linode.mcp.v1.tool_route` custom message option. Its payload contains the registered tool name, uppercase HTTP method, and normalized route template.

Extraction fails when:

- the extension definition is absent or has the wrong number/type/extendee;
- the option is attached to a non-`Input` message;
- the tool name, method, or path is malformed;
- two messages declare the same tool;
- no message carries the option.

The proof does not search generated source or read a separate route catalog.

## What descriptors do not prove

A descriptor proves that protobuf source compiled and exposes its declared schema. It does not prove:

- generated code is current;
- Go or Python runtime code sends the expected HTTP request;
- server behavior matches TechDocs;
- a field is actually used;
- a default is applied at runtime;
- runtime routing code consumes the same operation metadata.

Those are separate test and CI responsibilities. The Phase 1 comparison stays at the documentation/protobuf boundary.
