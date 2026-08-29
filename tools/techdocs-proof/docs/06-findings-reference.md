# 6. Findings Reference

A finding is a mechanical “needs investigation” record. Phase 1 does not decide severity, create an issue, or edit source.

## Finding envelope

Each finding is designed to answer:

```text
What rule fired?
Which operation is affected?
Which parameter is affected, if any?
What did TechDocs say?
What did protobuf say?
Where can an investigator verify both sides?
What should the next stage consider?
```

Exact fields vary by kind, but stable keys and source evidence make findings suitable for repeatable grouping later.

## Current finding distribution

From the 2026-08-01 proof:

| Kind | Count |
|---|---:|
| `proto_parameter_not_in_techdocs` | 1,082 |
| `techdocs_parameter_missing_from_proto` | 216 |
| `parameter_default_unrepresented_in_proto` | 147 |
| `proto_requiredness_ambiguous` | 92 |
| `parameter_type_mismatch` | 64 |
| `route_missing_from_proto` | 58 |
| `parameter_enum_mismatch` | 30 |
| `proto_route_absent_from_techdocs` | 23 |
| `parameter_requiredness_mismatch` | 20 |
| `deprecated_route_still_in_proto` | 5 |
| `deprecated_replacement_missing_from_proto` | 5 |
| `parameter_location_ambiguous` | 3 |
| `parameter_deprecation_mismatch` | 2 |
| **Total** | **1,747** |

Counts are observations, not thresholds. A later run may legitimately have more or fewer findings.

## Route findings

### `route_missing_from_proto`

**Rule:** An active rendered TechDocs operation has no associated protobuf tool route.

**Do not conclude immediately:** “LinodeMCP lacks support.”

**Check first:**

1. Does the intended `*Input` message carry a `linode.mcp.v1.tool_route` option?
2. Does that option declare the expected tool, method, and path?
3. Did the option survive into the compiled descriptor?
4. Did path version or placeholder normalization change?
5. Is the TechDocs operation actually active?

**Likely ownership:** route association, missing tool contract, or new API support.

### `proto_route_absent_from_techdocs`

**Rule:** A route-associated protobuf tool is not present in the active rendered contract or handled deprecated set.

**Check first:**

1. Confirm the run fetched every discovered page.
2. Search `url-index.json` for the expected docs page.
3. Inspect the rendered page for parser format changes.
4. Check whether the route is deprecated or replaced.
5. Verify the protobuf route option at the recorded GitHub commit.

**Likely ownership:** stale route association, removed API operation, parser adjustment, or version transition.

## Parameter presence findings

### `techdocs_parameter_missing_from_proto`

**Rule:** TechDocs documents a parameter that the matched request message does not expose.

**Evidence to compare:**

- TechDocs source URL;
- page file;
- location and raw name;
- normalized name;
- matched proto request message;
- list of proto fields.

**Common investigations:** nested-body modeling, spelling changes, route joined to wrong tool, new API parameter, or stale protobuf.

### `proto_parameter_not_in_techdocs`

**Rule:** The protobuf request has a field absent from the rendered operation and its leading comment does not begin with `System parameter:`.

**Required investigator decision:**

```text
API parameter -> reconcile with rendered TechDocs
internal MCP field -> add exact System parameter: comment and explanation
obsolete field -> remove through normal reviewed source change
wrong association -> repair route/request association
```

**Forbidden shortcut:** add the field name to a monitor allowlist.

The large current count is expected while internal controls lack explicit comments. That is useful debt inventory, not a reason to suppress the rule.

## Shape findings

### `parameter_type_mismatch`

**Rule:** Matched parameters normalize to different semantic type or collection shapes.

**Examples:**

```text
TechDocs integer vs proto string
TechDocs array of strings vs proto scalar string
TechDocs object vs proto bool
```

**Check:** raw type, semantic type, repeated/map flags, enum/message references, and whether a body object was flattened correctly.

### `parameter_requiredness_mismatch`

**Rule:** Both sides provide a usable requiredness signal and the booleans disagree.

**Check:** rendered `Required` marker, the `buf.validate` rule that names the field, proto presence, and whether the field is path/query/body. Chapter 5 gives the order the proto side answers in. The rule is the fact worth reading first: a field with no rule and no `optional` keyword is one the generated body builder sends on every call, which is a different claim from one the handler rejects the call without.

### `proto_requiredness_ambiguous`

**Rule:** TechDocs provides requiredness, but the descriptor does not provide enough evidence to classify the proto field safely.

**This is not the same as optional.** The resolution may be better protobuf validation metadata, a schema annotation, or a refined extractor rule backed by descriptor facts.

### `techdocs_body_variant_unrendered`

**Rule:** The Body Params section rendered one variant of a schema switcher, and the comparison landed on something only an unrendered variant could document: a protobuf body field with no documented match, or an allowed-value list the protobuf side strictly contains.

**Severity:** limitation. The documented side was never published, so there is nothing to judge against, the way proto3 cannot state a repeated field's requiredness.

**Do not act on this as a repo defect.** The field or value is real and the API accepts it. The finding carries `body_variants`, `rendered_body_variant`, and `unrendered_body_variants` so a reader can open the page, switch the tab, and check by hand.

**Resolution:** none available from the rendered page. Closing this class needs the site to render every variant server-side, or a ruling that the page's own OpenAPI payload becomes a second authority, which chapter 1 currently forbids.

### `parameter_location_ambiguous`

**Rule:** Name normalization produced multiple possible path/query/body matches and no unique mapping could be proved.

**Resolution:** improve explicit location metadata or contract structure. Do not hardcode a preferred location based on common usage.

## Value findings

### `parameter_default_unrepresented_in_proto`

**Rule:** TechDocs documents a default, but the protobuf contract does not represent an equivalent default.

**Check:** value type, enum initialization, explicit field options, and whether runtime-only behavior should be promoted into the protobuf contract.

**Do not check:** Go or Python source as authoritative proof. Runtime source may explain behavior during investigation, but it cannot erase the contract difference.

### `parameter_enum_mismatch`

**Rule:** Allowed-value sets differ after normalization.

**Useful evidence shape:**

```json
{
  "only_in_techdocs": ["a"],
  "only_in_proto": ["c"]
}
```

**Check:** naming transformations, excluded unspecified values, API version, and whether a protobuf enum contains internal states not accepted by the API.

### `parameter_deprecation_mismatch`

**Rule:** Matched parameter deprecation differs.

**Check:** rendered page wording, field deprecation option, and whether the API still accepts the parameter.

## Deprecated-route findings

### `deprecated_route_still_in_proto`

**Rule:** Rendered TechDocs marks an operation deprecated while LinodeMCP still represents it.

**Possible outcomes:** retain with explicit compatibility policy, mark tool deprecated, migrate callers, or remove through reviewed work.

The monitor does not choose one.

### `deprecated_replacement_missing_from_proto`

**Rule:** TechDocs clearly advertises a replacement, but no protobuf `tool_route` option represents it.

**Check both sides:**

- old route handling;
- replacement route existence;
- replacement request contract.

Removing the old route without adding or confirming the replacement does not resolve the whole finding.

## Grouping findings for later issue creation

The next workflow stage should not create one issue per raw finding. It should group facts by a stable ownership unit, likely:

```text
normalized operation
or
proto tool/request message
```

A useful candidate group key:

```text
api_version + method + normalized_path + tool
```

Within a group, preserve all finding kinds and source evidence. Do not combine unrelated product routes just because they share a kind such as `proto_parameter_not_in_techdocs`.

Before issue creation, the later stage should:

1. verify `latest.json.status == "ok"`;
2. verify checksums;
3. read the report path from the manifest;
4. ensure the result is newer than the last consumed run;
5. group by the same rule every run;
6. deduplicate against open tracked work;
7. apply acceptance and worker-lifecycle policy;
8. create no issue for infrastructure errors.

## What a high-quality issue eventually needs

Although Phase 1 does not write issues, it should leave enough evidence for one to contain:

- exact operation and tool;
- finding kinds and affected fields;
- rendered TechDocs source URLs;
- relevant page and contract artifact paths;
- protobuf file/message/field identity;
- LinodeMCP commit;
- expected investigation decision;
- special `System parameter:` guidance where applicable;
- deprecation replacement evidence where applicable.

The later stage should report the mismatch, not speculate about a fix it has not verified.
