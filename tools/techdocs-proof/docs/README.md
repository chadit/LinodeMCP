# TechDocs-to-Proto Contract Proof

This wiki explains the first stage of the Linode API documentation workflow: turn the rendered TechDocs site into a machine-readable contract, compile the LinodeMCP protobuf contract into descriptors, compare the two, and preserve reproducible evidence for later issue creation.

The comparator lives in the LinodeMCP repository, under `tools/techdocs-proof/`. Any host with Python, Buf, and network access runs the repo command directly. The cloud development host is one optional runner among them, not a component: nothing in the pipeline calls it by name. Run evidence stays outside the repository so a comparison never dirties the tree it measures.

## The idea in one picture

```text
Rendered TechDocs pages                     Local LinodeMCP working tree
(external API authority)                        (tool-contract authority)
          │                                                │
          ▼                                                ▼
complete rendered snapshot                   proto/linode/mcp/v1/**/*.proto
          │                                  + ToolRoute message options
          ▼                                                │
normalized TechDocs contract                               ▼
          │                                      Buf FileDescriptorSet
          ├─────────────── semantic join ──────────────────┤
          │                                                │
          ▼                                                ▼
                  reproducible comparison
                            │
                            ▼
                   investigation findings
                            │
                            ▼
             dated evidence directory + latest.json
                            │
                            ▼
                 later issue-creation workflow
```

The central rule is simple:

> Compare contracts at their declared boundary. Do not reverse-engineer the contract from generated Go or Python source.

## Where it runs

Entry point, from the repository root:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof
```

Default LinodeMCP source: the working tree the command lives in. That is what
retired the push-first friction, where a comparison could only read what GitHub
already served. `--github-source` still resolves an immutable commit archive
with `gh` for a run that has to name a SHA.

Default evidence root: `$LINODEMCP_TECHDOCS_PROOF_ROOT`, else
`$XDG_DATA_HOME/linodemcp/techdocs-proof`, else
`~/.local/share/linodemcp/techdocs-proof`. `--evidence-root` overrides it. A
root inside the repository is refused by name.

The offline arm is the one `make check` runs:

```bash
make techdocs-proof
```

A normal completed comparison exits `0`, even if differences were found. Differences are data, not a process crash. Use `--fail-on-findings` when a caller wants exit `3` for a completed run carrying a finding at medium or high; the known, limitation, and info tiers never reach the exit code, because each of them records a disagreement someone already accepted. Input, network, parsing, or compilation failures exit `1`.

## What this stage does

1. Discovers rendered Linode API TechDocs pages from the API reference root and sitemap.
2. Fetches every discovered page with bounded retries.
3. Fails if the snapshot is incomplete.
4. Converts rendered HTML into stable, line-oriented text.
5. Redacts credential-shaped values before evidence is saved.
6. Parses routes, parameter locations, types, requiredness, defaults, enums, deprecations, and response statuses.
7. Writes a normalized JSON contract.
8. Generates a candidate TechDocs protobuf representation and compiles it with Buf.
9. Reads the local working tree's Buf module and protobuf tree, or with `--github-source` resolves an exact commit and preserves only those files, then compiles either into a descriptor set with source comments.
10. Reads each tool name, HTTP method, and path from its protobuf `linode.mcp.v1.tool_route` message option.
11. Compares the two contracts semantically.
12. Writes findings, source evidence, checksums, and a stable `latest.json` handoff.
13. Removes dated run directories older than five days.

## What this stage intentionally does not do

It does not:

- read OpenAPI;
- import LinodeMCP Python packages;
- parse generated Go or Python source;
- create or update issues;
- add labels;
- dispatch workers;
- edit protobuf source;
- guess whether a mismatch is harmless;
- guess a deprecated route replacement;
- treat a clean comparison process as proof that contracts match.

Those boundaries matter. Phase 1 should produce facts that later stages can consume without hiding uncertainty.

## Current proof snapshot

The validated 2026-08-03 proto-option proof processed:

| Measure | Count |
|---|---:|
| Rendered pages fetched | 516 |
| TechDocs operations | 489 |
| Active routes compared | 474 |
| Proto tools compared | 444 |
| Deprecated routes checked | 15 |
| Replacement routes checked | 5 |
| Findings | 1,747 |

These values describe one run. They are not hardcoded expectations. Page, route, tool, and finding counts can change as TechDocs and LinodeMCP change.

## Wiki map

- [1. Boundaries and authority](01-boundaries-and-authority.md)
- [2. End-to-end data flow](02-end-to-end-data-flow.md)
- [3. TechDocs capture and normalization](03-techdocs-capture-and-normalization.md)
- [4. Protobuf and descriptor construction](04-protobuf-and-descriptors.md)
- [5. Semantic comparison](05-semantic-comparison.md)
- [6. Findings reference](06-findings-reference.md)
- [7. Run artifacts, retention, and handoff](07-artifacts-retention-and-handoff.md)
- [8. Operations and debugging](08-operations-and-debugging.md)
- [9. Design rationale and invariants](09-design-rationale-and-invariants.md)
- [Glossary](glossary.md)

## Five invariants to remember

1. **Rendered TechDocs is the external API authority.** A second schema source cannot silently overrule it.
2. **Protobuf is the LinodeMCP tool-contract authority.** Generated language source is downstream.
3. **Ambiguity becomes a finding or a failed run.** It is never guessed away.
4. **A completed run and a matching contract are different facts.** Exit `0` means the proof executed; findings say whether differences exist.
5. **Phase 1 creates evidence, not incidents.** Issue creation is a separate, reviewable stage.
