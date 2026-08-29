# Glossary

## Active route

A rendered TechDocs operation not marked deprecated.

## Ambiguity

A condition where available contract evidence permits more than one interpretation. The proof reports or fails on ambiguity rather than selecting one interpretation.

## API authority

The source allowed to define externally documented API facts for this proof. It is the rendered TechDocs site.

## Buf

The protobuf tool used to compile modules into descriptor sets. This proof does not run generated language code.

## Candidate TechDocs proto

A generated, compilable protobuf representation of the rendered TechDocs snapshot. It is evidence, not an automatic LinodeMCP patch.

## Contract

A structured declaration of operations, parameters, types, requiredness, defaults, allowed values, and deprecations.

## Descriptor / FileDescriptorSet

The protobuf compiler's structured representation of files, messages, fields, enums, options, and source locations.

## Reproducible finding

A finding whose content and order are stable for fixed inputs.

## Endpoint record

The structured interpretation of one rendered TechDocs operation page before facts are flattened into the normalized contract.

## Evidence

Saved source pages, indexes, normalized contracts, generated proto, descriptors, findings, manifests, and checksums that allow later review.

## External API authority

See **API authority**.

## Fail closed

Stop the run when inputs are incomplete or interpretation is unsafe, rather than continuing with guessed or partial data.

## Finding

A mechanical record that a rule observed a difference or could not prove equivalence. A finding is not automatically an incident.

## Finding kind

A stable category such as `parameter_type_mismatch` or `route_missing_from_proto`.

## Internal parameter

A protobuf request field consumed by MCP behavior rather than sent to the Linode API. It must use the exact leading `System parameter:` comment to be exempt from proto-only findings.

## Latest manifest

`latest.json`, the stable root-level handoff describing the current run's status, report, artifacts, timestamps, and LinodeMCP commit.

## LinodeMCP contract authority

The protobuf tree compiled into descriptors: the local working tree by default, or an exact GitHub commit under `--github-source`.

## Normalized name

A comparison form that removes cosmetic naming differences such as camelCase versus snake_case.

## Normalized path

A route path with stable version, slash, and placeholder representation for semantic joining.

## OpenAPI

A schema source deliberately excluded from this proof. It can be audited separately.

## Operation key

The uppercase HTTP method plus normalized path.

## Parameter location

One of `path`, `query`, or `body`.

## Phase 1

The evidence-producing stage that captures TechDocs, compiles contracts, compares them, and writes findings. It does not create issues or edit source.

## Proto-only parameter

A field present in the matched protobuf request but absent from the rendered TechDocs operation.

## Raw type

The type spelling retained from the source before semantic type normalization.

## Rendered TechDocs

The published HTML documentation pages retrieved from the TechDocs site and converted into preserved text evidence.

## Replacement route

An operation clearly advertised by rendered TechDocs as the successor to a deprecated route.

## Route association

The link between an MCP tool/request message and an HTTP method/path, declared on that request message by its `linode.mcp.v1.tool_route` option.

## Run directory

The unique timestamped directory containing evidence for one execution.

## Semantic comparison

Comparison of meaning after removing irrelevant formatting and representation differences.

## Source comments

Comments retained through descriptor source information. Leading field comments are required for exact system-parameter classification.

## System parameter marker

The exact leading comment prefix:

```text
System parameter:
```

No name-based substitute exists.

## TechDocs contract

The JSON contract derived only from rendered TechDocs pages. The same pages always produce the same contract.

## TechDocs-only parameter

A rendered parameter absent from the matched protobuf request.

## ToolRoute option

The protobuf custom message option attached to each API-backed `*Input` message. It owns the registered MCP tool name, uppercase HTTP method, and normalized path template. The compiled JSON descriptor represents it as `[linode.mcp.v1.tool_route]`.
