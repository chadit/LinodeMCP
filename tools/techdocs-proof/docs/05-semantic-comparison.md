# 5. Semantic Comparison

A semantic comparison asks whether two contracts mean the same thing, not whether their source text looks the same.

The proof therefore converts both sides into normalized operation and parameter facts before comparing them.

## Operation key

The primary route key is:

```text
METHOD + normalized path
```

Example:

```text
GET /v4/linode/instances/{param}
```

Method is uppercased. Repeated slashes are collapsed. Version and placeholder syntax are normalized. The placeholder name is not part of route identity.

This prevents cosmetic differences from producing false route findings while preserving method distinctions:

```text
GET  /v4/widgets/{param}
POST /v4/widgets/{param}
```

are different operations.

## Comparison partitions

Routes are divided into:

```text
TechDocs only
Proto only
Present on both sides
Deprecated TechDocs routes
```

Each partition has its own rules. Deprecated routes do not flow through the active-route absence rules unchanged.

## Route presence

### TechDocs active route missing from proto

Finding:

```text
route_missing_from_proto
```

Meaning: rendered TechDocs publishes an active operation, but no associated protobuf tool contract was found for the normalized method/path.

Questions for investigation:

- Is the tool missing?
- Is the protobuf `tool_route` option missing or stale?
- Does proto encode the operation under an unresolved tool?
- Was the API route added to docs before LinodeMCP support?

### Proto route absent from TechDocs

Finding:

```text
proto_route_absent_from_techdocs
```

Meaning: a protobuf `tool_route` option describes a tool operation that does not appear among active or handled deprecated TechDocs operations.

Possible causes:

- the API route was removed;
- the protobuf route option is stale;
- TechDocs discovery or parsing changed;
- the operation was renamed or versioned;
- the route should have been handled as deprecated.

A complete TechDocs snapshot is required before this finding is meaningful.

## Parameter identity

Names are normalized before matching. Typical normalization includes:

- camelCase to snake_case;
- hyphens and dots to underscores;
- repeated underscores collapsed;
- case folded.

Examples:

```text
linodeId     -> linode_id
linode-id    -> linode_id
LINODE_ID    -> linode_id
```

Name normalization does not prove location. If `id` appears in both query and body, the proof must use location evidence or emit ambiguity.

## Parameter presence

### TechDocs parameter missing from proto

Finding:

```text
techdocs_parameter_missing_from_proto
```

The rendered operation documents a path, query, or body parameter that is not represented by the matched proto request.

Investigation should check:

- spelling and normalization;
- nested message flattening;
- route association;
- API version;
- whether the protobuf request is stale.

### Proto parameter missing from TechDocs

Finding:

```text
proto_parameter_not_in_techdocs
```

The matched protobuf request contains a field not documented on the rendered operation.

The only exemption is a leading field comment beginning exactly:

```text
System parameter:
```

No field name is exempt by convention.

Every non-exempt finding carries this guidance:

> This proto parameter is not present in the rendered TechDocs contract. Investigate whether it is an internal MCP/system parameter. If it is internal, add a proto field comment beginning with `System parameter:` that explains its purpose and confirms it is not sent to the Linode API. Otherwise, reconcile the proto field with the rendered TechDocs contract.

## Location comparison

Location values are:

```text
path
query
body
```

If both sides contain a unique normalized parameter name, the proof can compare location. If more than one candidate exists and location cannot establish a unique match, it emits:

```text
parameter_location_ambiguous
```

It does not choose the first field or prefer body by convention.

## Type comparison

Rendered TechDocs and protobuf have different type vocabularies. The comparator maps both to semantic families while retaining raw source values in evidence.

Typical equivalences:

| TechDocs | Protobuf semantic family |
|---|---|
| boolean | bool |
| integer | signed/unsigned integer family |
| number | float/double numeric family |
| string, password, UUID, date-time | string family, with raw type retained |
| array of strings | repeated string |
| object/map | message, map, or structured value depending on descriptor shape |

A mismatch becomes:

```text
parameter_type_mismatch
```

The proof ignores formatting and package names, not shape. Scalar versus repeated, scalar versus map, or integer versus boolean are material differences.

## Requiredness comparison

TechDocs marks a parameter as required through rendered prose. The protobuf side
answers in this order:

1. a `buf.validate` message rule whose identifier names the field and whose
   expression reads that same field is a declared requirement, and it wins;
2. a `proto3Optional` field, and a singular message field, are presence tracked,
   so an omitted argument never reaches the wire and the field is optional;
3. a repeated or map field with no rule cannot state requiredness in proto3;
4. anything else is a field the generated body builder sends on every call, so
   it is required.

The rule comes first because it is what the handler runs. proto3 presence says
whether a field can be omitted on the wire, not whether the tool accepts the call
without it, and reading presence alone got both directions wrong: a singular
message field read as required reported an omittable argument as a stricter
contract than the tool has, and a field the rule requires read as optional where
the field carries `optional`.

When both sides provide a reliable signal and disagree:

```text
parameter_requiredness_mismatch
```

When the protobuf side cannot prove a boolean requiredness value:

```text
proto_requiredness_ambiguous
```

Ambiguity is separate from mismatch. This distinction prevents “unknown” from being silently coerced to optional.

## Body sections the page rendered one variant of

Where the Body Params section carries a schema switcher, the page published the
fields of one variant and the labels of the rest. Two comparisons then have no
documented side to judge against, and both are reported at limitation severity
with the unrendered labels named:

```text
techdocs_body_variant_unrendered
```

- a protobuf body field with no documented match, where an unrendered variant is
  where the documentation for it would live;
- an allowed-value list where the protobuf set is a strict superset of the
  rendered variant's, which is the shape a union across variants produces.

Everything else on such a route is still compared. A query field is outside the
switcher. A type or requiredness disagreement is about a fact the page states for
the variant it rendered, and an allowed-value list the protobuf side does not
cover is a disagreement no unrendered variant explains.

## Default comparison

TechDocs defaults are captured as typed JSON values when possible. Protobuf may represent defaults through explicit options, enum initialization, wrapper behavior, or no schema-level value.

When TechDocs documents a default that the protobuf contract does not represent:

```text
parameter_default_unrepresented_in_proto
```

The proof does not infer runtime defaults from Go or Python code. If a runtime default is contractually important, it should be represented at the protobuf boundary or the finding should remain visible.

Default equality is type-sensitive:

```text
false != "false"
0     != "0"
null  != absent
```

## Enum comparison

TechDocs allowed values and descriptor enum values are normalized and sorted before comparison. Ordering does not matter. Membership does.

The proto side of a scalar field is its declared value set: a `.known` rule
whose expression tests `this.<field> in [...]`, else the field's
`reader_values` option, else nothing. Chapter 4 says how each is read. An enum
descriptor on the field wins over both.

Finding:

```text
parameter_enum_mismatch
```

Evidence should show values only in TechDocs and values only in proto.

Synthetic protobuf initialization values such as an `UNSPECIFIED` member are normally excluded from API allowed-value comparison.

## Parameter deprecation

If one side marks a matched parameter deprecated and the other does not:

```text
parameter_deprecation_mismatch
```

This can reveal a documentation lag, a missing proto option, or an obsolete field still exposed by a tool.

## Route deprecation and replacements

Deprecated routes are checked separately.

### Deprecated route still represented

```text
deprecated_route_still_in_proto
```

This asks whether the tool should be removed, retained for compatibility, or annotated with an explicit policy.

### Advertised replacement missing

```text
deprecated_replacement_missing_from_proto
```

If rendered TechDocs clearly names a replacement route, that replacement is independently checked. The old route's presence does not satisfy the replacement requirement.

### Unclear replacement

The parser does not convert vague prose into a route. An unclear replacement remains an investigation item.

## Determinism

For fixed inputs, findings are sorted by stable keys and serialized with sorted JSON keys. The proof avoids dependence on:

- worker completion order;
- filesystem enumeration order;
- protobuf declaration order where semantics do not depend on it;
- parameter enum display order;
- route placeholder spelling;
- source formatting.

The verified finding-list SHA-256 for the 2026-08-01 proof was:

```text
905d0159e385792ebbd53303d19c3b734bb645952eef9f8a9bcce67f09fe9337
```

That hash is a reproducibility observation for the referenced inputs, not a permanent expected value.

## Reading zero exact route matches

The summary currently reports `exact_route_contract_matches: 0`. This does not mean no route keys joined. The run compared 474 active routes. “Exact” requires all compared route and parameter details to produce no findings. Broad proto-only system fields and other contract differences mean every joined route currently has at least one finding.

The useful metrics are therefore both:

```text
active routes compared
findings grouped by kind
```

A single headline match percentage would hide why contracts differ.
