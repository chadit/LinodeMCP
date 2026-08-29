# 9. Design Rationale and Invariants

This page explains why the proof is shaped this way and which shortcuts would weaken it.

## Why the comparator lives in the repository

It renders this repository's protobuf conventions and reads this repository's
proto tree, so it has to version with them. A comparator kept on one host
drifts from the contract it measures, and a fix to either one lands without the
other.

Residency was ruled on 2026-08-19 and widened the same day: the merger, the
overlay, the comparator, its exclusion ledger, its self-test, and this wiki all
live here. The command reads the local working tree by default, which is what
retired the push-first friction where a comparison could only see what GitHub
already served.

One host is still special in exactly one way: nothing. Any host with Python,
Buf, and network access runs the repo command. The cloud development host is an
optional runner, not a component, and no automation names it.

Run evidence is the one thing that stays out. A run writes tens of megabytes of
scraped pages and descriptors, and a comparator that dirties the tree it
measures cannot be trusted to have measured the tree as committed. The evidence
root defaults to the user data directory, takes an environment variable or a
flag, and refuses by name any path inside the repository. Only reviewed harvest
snapshots land in the repository, under `docs/contracts/`.

## Why rendered TechDocs, not OpenAPI

The purpose is to check the published documentation contract against LinodeMCP. Rendered pages are what users see.

Adding OpenAPI would create this triangle:

```text
Rendered TechDocs
       ↕
OpenAPI  ↔  protobuf
```

When all three differ, the proof would need a policy for which external source wins. That policy would obscure the original question.

OpenAPI can have its own audit. It is not part of this one.

## Why protobuf, not generated source

Generated Go and Python are implementations of the protobuf contract. Scanning them duplicates code-generation and CI responsibilities.

Source scanning also imports irrelevant instability:

- parser version;
- language syntax;
- helper refactors;
- generated file layout;
- formatting;
- function naming;
- local build environment.

Descriptors expose the contract without those details.

## Why generate a TechDocs proto if comparison already uses JSON

The normalized JSON contract is convenient for repeatable comparison. The candidate proto adds a second property: the rendered contract can be expressed as a valid protobuf schema artifact.

That artifact helps reviewers:

- inspect operation messages and parameter fields;
- inspect route and field facts as protobuf options;
- compare descriptor structures;
- test future protobuf-native route metadata;
- reproduce the run without regenerating pages.

It is not automatically copied into LinodeMCP because generation does not settle design choices such as message reuse, internal controls, naming, response modeling, or backward compatibility.

## Why preserve both pages and normalized output

If only normalized JSON were saved, a parser bug could look like a documentation change. If only rendered pages were saved, every downstream investigation would need to rerun the parser.

Saving both yields a trace:

```text
source URL
  -> preserved page
  -> endpoint record
  -> normalized contract fact
  -> finding
```

That trace is what makes a mechanical finding reviewable.

## Why fail on one missing page

Suppose TechDocs publishes 516 pages and page 317 times out. If the proof continues with 515 pages, every proto route owned by page 317 appears absent from TechDocs.

The monitor would manufacture findings from a network failure.

Therefore:

```text
partial snapshot = invalid comparison input
```

Failing the run is cheaper than creating false contract work.

## Why findings do not fail the default process

A comparison tool has two independent outcomes:

```text
Did the proof execute correctly?
Did the contracts match?
```

Exit `0` answers the first question. `comparison.json.findings` answers the second.

If every mismatch caused exit `1`, infrastructure alerting and contract investigation would be mixed. Exit `3` is available for callers that want strict semantics while preserving the distinction.

## Why system parameters require an exact marker

Internal controls are valid in an MCP request but absent from the external API documentation. Examples can include environment selection, confirmation, dry-run behavior, pagination helpers, or transport controls.

A name allowlist would hide context:

```python
if field.name in {"environment", "confirm", "dry_run"}:
    ignore()
```

That approach fails when:

- the same name becomes an API parameter;
- a new internal field appears;
- an old field changes purpose;
- reviewers cannot tell why a field is excluded.

An exact protobuf comment keeps the declaration beside the field and under source review:

```proto
// System parameter: Requires explicit confirmation before destructive work.
// Consumed by the MCP layer and not sent to the Linode API.
bool confirm = 3;
```

## Why deprecations are separated

An active route missing from proto often means unsupported API surface. A deprecated route still in proto may be intentional compatibility or stale exposure. The remediation questions differ.

Replacement routes add a second obligation. A valid migration requires checking the new contract, not merely deleting the old one.

Separate finding kinds preserve this distinction for later issue grouping.

## Why ambiguity remains visible

A monitor can appear quiet by making guesses:

```text
unknown requiredness -> optional
same parameter name twice -> choose body
unclear replacement -> pick nearest route name
missing comment -> assume internal
```

These guesses reduce finding count while reducing truth.

The proof chooses the opposite rule:

```text
If equivalence cannot be proved from declared contract evidence, retain the uncertainty.
```

That may produce more findings during early rollout. It also shows exactly where contract metadata needs improvement.

## Why artifacts are immutable

A timestamped run should be a snapshot of one observation. Later consumers read it; they do not modify it.

If an issue workflow appended labels or consumption flags to `comparison.json`, checksums would fail and historical evidence would become dependent on downstream state.

Consumption state belongs in the consumer's own durable store.

## Why `latest.json` is a manifest, not a symlink

A JSON manifest works across SSH, shell, Python, and future scheduler implementations. It can represent `running`, `ok`, and `error`, plus paths and timestamps.

A `latest` symlink only points somewhere. It cannot explain whether the run finished or why it failed.

## Why five-day retention is calendar-based

The script creates UTC date directories and removes valid date directories older than the configured window.

Calendar directories make cleanup predictable and auditable. The script does not recursively search for arbitrary old paths, which protects unrelated files under the workflow root.

Five days retains enough recent evidence for comparison and issue consumption without allowing rendered-page snapshots and descriptors to grow forever.

## Core invariants

### Input invariants

- TechDocs authority string is exactly `techdocs-rendered-pages`.
- Discovered page count equals fetched page count.
- Every indexed page exists.
- Normalized operation keys are unique.
- LinodeMCP proto compiles.
- Candidate TechDocs proto compiles.
- descriptor source comments are available where classification needs them.
- route associations are parseable and unambiguous.

### Comparison invariants

- method and normalized path define operation identity;
- location remains part of parameter identity;
- raw types remain in evidence after semantic normalization;
- absent is distinct from null or false;
- ambiguity is not coerced to a value;
- only exact `System parameter:` leading comments exempt proto-only fields;
- deprecated operations use dedicated rules;
- findings are stably sorted.

### Output invariants

- every run has a unique directory;
- status begins as `running`;
- success and error both finalize manifests;
- `latest.json` points to the current run state;
- immutable evidence receives SHA-256 records;
- retention only touches valid dated run directories;
- no Phase 1 code creates issues or edits LinodeMCP.

## Future evolution

Several improvements fit the same boundary:

1. Improve explicit parameter-location, requiredness, default, and response metadata in protobuf.
2. Add parser fixtures whenever rendered TechDocs layout changes.
3. Add a finding-grouping stage that groups the same way every run.
4. Add accepted-gated issue creation after Phase 1 evidence is trusted.
5. Add exact-head review and source changes in a later worker lifecycle.

None of these move collection or comparison outside the repository command.
