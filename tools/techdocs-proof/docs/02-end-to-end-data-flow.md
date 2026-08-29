# 2. End-to-End Data Flow

This page follows one run from process start to the final `latest.json` handoff.

## Stage map

```text
0. Validate arguments and create run directory
1. Discover TechDocs URLs
2. Fetch and preserve every rendered page
3. Parse endpoint records
4. Build normalized TechDocs contract
5. Generate and compile candidate TechDocs proto
6. Compile LinodeMCP proto
7. Extract MCP tool contracts
8. Join operations and compare semantics
9. Write report, checksums, and manifest
10. Publish latest.json
11. Return an exit code
```

The ordering is intentional. A later stage never runs on a partially accepted result from an earlier stage.

## Stage 0: create an isolated run

The script uses UTC and creates:

```text
<root>/<YYYY-MM-DD>/<YYYYMMDDTHHMMSSZ>/
```

For example:

```text
techdocs-proof/2026-08-01/20260801T045641Z/
```

If two runs begin in the same second, the second receives `-2`, then `-3`, and so on. Existing evidence is not overwritten.

Before collection begins, `run.json` and `latest.json` are written with:

```json
{
  "schema": "linodeapi.techdocs_proto_proof.run.v1",
  "status": "running",
  "started_at": "2026-08-01T04:56:41+00:00",
  "run_dir": "...",
  "removed_old_runs": []
}
```

This matters to the later orchestrator. If the process dies halfway through, `latest.json` does not falsely describe an older successful run as the just-finished result.

## Stage 1: discover the page set

The script queries two rendered-site inputs:

```text
https://techdocs.akamai.com/linode-api/reference/api
https://techdocs.akamai.com/sitemap.xml
```

Links are normalized, fragments are removed, and duplicates are collapsed. Static asset URLs, unresolved template URLs, and generic numeric HTTP error pages are excluded.

The output is a sorted set. Sorting removes thread-completion order from downstream artifacts.

Failure condition:

```text
No usable page URLs were discovered.
```

The script stops rather than comparing an empty documentation set with a non-empty proto tree.

## Stage 2: fetch a complete snapshot

Pages are fetched concurrently, with the worker count bounded to 32. Each request uses a named user agent, a timeout, and at most three attempts. Retries are limited to transient classes such as timeouts, connection failures, rate limiting, and selected server errors.

Every fetched page is transformed into line-oriented text and written under `pages/` with its source URL in front matter.

Credential-shaped values are replaced with `[REDACTED]` before the page is saved. Redaction covers private-key blocks, long bearer values, GitHub-token shapes, and JWT shapes.

The key invariant is completeness:

```text
fetched page count == discovered page count
```

If one page remains unavailable after bounded retries, the run fails. It does not compare the 515 pages it happened to retrieve against the entire protobuf tree. A partial snapshot would create false “proto route absent from TechDocs” findings.

## Stage 3: parse endpoint records

Each saved page is scanned for the rendered method-and-route line:

```text
GET https://api.linode.com / {apiVersion} /linode/instances/{instanceId}
```

The parser identifies these sections when rendered:

```text
Path Params
Query Params
Body Params
Responses
```

Parameter blocks yield:

- original name;
- rendered type;
- required flag;
- documented default;
- allowed values;
- parameter deprecation.

The route record retains the source page, source URL, raw path, normalized path, operation ID, response statuses, and route deprecation.

Pages without an endpoint route are preserved in the page snapshot but do not become endpoint records. Duplicate normalized route keys fail the run because the proof cannot know which page owns the operation.

Each endpoint is written to `endpoints/`, and the full ordered list is written to `endpoint-index.json`.

## Stage 4: build the normalized contract

Endpoint records are flattened into six collections, each in a stable order:

```text
routes
parameters
defaults
enums
deprecated_routes
deprecated_parameters
```

The contract declares:

```json
"source_authority": "techdocs-rendered-pages"
```

The comparator rejects another authority value. This prevents a file generated from OpenAPI or a stale fixture from being passed in under a misleading filename.

The result is saved as:

```text
techdocs-contracts.json
```

## Stage 5: generate a TechDocs candidate proto

The script emits:

```text
generated-proto/techdocs_contract.proto
generated-proto/buf.yaml
techdocs-descriptor.json
```

The generated proto is evidence, not a patch for LinodeMCP. It records each rendered operation as a request message and stores operation and parameter facts in custom protobuf options.

Each operation option records:

- API version;
- method;
- normalized path;
- route deprecation;
- source URL;
- documented success statuses.

Each field option records:

- location;
- rendered type;
- requiredness;
- whether a default is documented;
- default as stable JSON text;
- allowed values;
- deprecation.

Buf compiles this candidate. A generated `.proto` file that cannot compile is not useful evidence, so compilation failure stops the run.

## Stage 6: compile LinodeMCP protobuf

By default the command executes Buf against the working tree it lives in and records that tree's HEAD and whether the proto sources are dirty. With `--github-source` it resolves the requested ref to an exact SHA, downloads that immutable archive with `gh`, preserves only `buf.yaml`, `buf.lock`, and `proto/`, and compiles the preserved module instead. Either way it writes:

```text
linodemcp-descriptor.json
```

The descriptor retains source information because leading field comments participate in system-parameter classification.

No generated Go or Python code is loaded. The proof reads protobuf descriptors only.

## Stage 7: extract tool contracts

The extractor indexes descriptor messages and enums. For each annotated `*Input` message, it reads tool name, method, and path directly from the compiled `[linode.mcp.v1.tool_route]` message option.

For each tool, it derives:

- request message;
- route key;
- field names;
- normalized names;
- scalar or message type;
- repeated/map shape;
- enum values;
- deprecation;
- optionality evidence;
- leading comment;
- exact `System parameter:` classification.

Missing source comments, a missing extension definition, malformed route metadata, or duplicate tool ownership fails closed. The extractor never consults a separate route map.

## Stage 8: semantic comparison

The join key is the normalized operation:

```text
HTTP_METHOD + normalized_path
```

For a route found on both sides, fields are compared by normalized parameter name, with location used when the name is unique enough to prove the match.

The proof compares:

- route presence;
- parameter presence;
- location;
- type;
- requiredness;
- default representation;
- enum values;
- deprecation;
- deprecated replacements.

Every finding has a stable kind, and the same inputs produce the same evidence payload.

## Stage 9: write immutable evidence

The primary report is:

```text
comparison.json
```

After the report is written, the script hashes immutable files into:

```text
checksums.sha256
```

`run.json` is excluded because it is updated after checksums are generated. The checksum file also excludes itself.

The current run contains page text, endpoint records, both descriptors, candidate proto, normalized contract, comparison report, and indexes. A later worker can trace a finding back to both source sides without rerunning collection.

## Stage 10: publish the handoff

On success, `run.json` becomes `status: ok`, then the root `latest.json` receives the same finished manifest.

On failure, both files become `status: error` and contain the exception text. A later cron must inspect `status`; the existence of `latest.json` alone is not success.

## Stage 11: exit semantics

| Exit | Meaning |
|---:|---|
| `0` | The comparison completed. Findings may exist. |
| `1` | Collection, input validation, parsing, descriptor compilation, or comparison infrastructure failed. |
| `3` | The comparison completed with findings and `--fail-on-findings` was requested. |

This separation lets an issue-creation stage consume findings from a successful proof without treating expected differences as a crashed collector.
