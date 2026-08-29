# 7. Run Artifacts, Retention, and Handoff

A proof is only as useful as the evidence it leaves behind. The run directory is designed so a later process can inspect a result without fetching TechDocs again or rebuilding descriptors.

## Root layout

```text
$LINODEMCP_TECHDOCS_PROOF_ROOT (default ~/.local/share/linodemcp/techdocs-proof)
├── latest.json
└── YYYY-MM-DD/
    └── YYYYMMDDTHHMMSSZ/
        └── run artifacts
```

The code lives in the repository under `tools/techdocs-proof/`, so retention
cleanup cannot reach it, and neither can a run reach the repository: a root
inside the tree is refused by name.

## Why use date plus timestamp

The date directory makes retention and manual browsing simple. The timestamp directory permits multiple runs on the same date without overwriting evidence.

```text
2026-08-01/
├── 20260801T044642Z/
└── 20260801T045641Z/
```

The latest successful run is selected through `latest.json`, not by sorting directory names in a downstream cron.

## Artifact inventory

### `run.json`

Per-run lifecycle manifest.

During execution:

```json
{
  "schema": "linodeapi.techdocs_proto_proof.run.v1",
  "status": "running",
  "started_at": "...",
  "run_dir": "...",
  "removed_old_runs": []
}
```

On success it adds:

- `status: ok`;
- completion timestamp;
- report path;
- LinodeMCP commit;
- summary;
- artifact map.

On failure it adds:

- `status: error`;
- completion timestamp;
- error text.

### `latest.json`

Root-level handoff for the next stage. It mirrors the current run manifest.

Consumers must read and validate it. They must not assume that the newest date directory contains a finished result.

Minimum consumer checks:

```text
schema is recognized
status is ok
run_dir exists
report exists
checksums exist and verify
completed_at is newer than last consumed run
repo_sha is present
```

### `pages/`

One redacted, line-oriented rendered page per discovered TechDocs URL. These files are the primary published-document evidence.

### `url-index.json`

Maps source URLs to page files, original discovery index, and saved byte count.

Use it to locate the page behind a route or deprecation source URL.

### `endpoints/`

One parsed endpoint JSON record per recognized operation.

These files are convenient for focused inspection. They sit between page parsing and normalized contract flattening.

### `endpoint-index.json`

Ordered list of all parsed endpoint records. This is useful for parser analysis and candidate-proto generation.

### `techdocs-contracts.json`

Normalized rendered-TechDocs contract used by the comparator.

Important top-level authority:

```json
"source_authority": "techdocs-rendered-pages"
```

### `generated-proto/techdocs_contract.proto`

Candidate protobuf representation of rendered operations and parameters, rendered the same way on every run.

It is evidence only. It is not written into the LinodeMCP repository.

### `generated-proto/buf.yaml`

Minimal Buf module for compiling the candidate proto.

### `techdocs-descriptor.json`

Compiled descriptor set for the candidate TechDocs proto. Its existence proves the generated proto compiled.

### `linodemcp-descriptor.json`

Compiled descriptor set for the checked-out LinodeMCP proto tree, including source comments.

### `route-snapshot.txt`

One line per rendered route, in the vocabulary
`docs/contracts/api-techdocs-routes-baseline.txt` uses:

```text
GET /lke/clusters/{}/dashboard surface=both status=deprecated
```

Placeholder names collapse to `{}` because TechDocs and the proto tree spell
them differently, and the shape is the only join the two sides agree on.

This is the refresh candidate for the reviewed snapshot that
`make techdocs-routes` gates `proto/` against offline. It is evidence like
everything else here: the scheduled job reports its diff against the reviewed
file, and a human lands the refresh.

### `comparison.json`

Primary Phase 1 result. It contains:

- status and authority;
- timestamps;
- LinodeMCP commit;
- execution metadata;
- summary;
- findings in a stable order;
- artifact paths.

A later issue stage should read this path from `latest.json.report`.

### `checksums.sha256`

SHA-256 records for immutable run evidence.

The file excludes:

- itself;
- `run.json`, because the manifest is finalized after checksums are generated.

Verify from the run directory:

```bash
sha256sum -c checksums.sha256
```

The completed 2026-08-01 run contained 1,013 checksum entries.

## Artifact graph

```text
pages/*.md
   │
   ├──> url-index.json
   │
   └──> endpoints/*.json
            │
            ├──> endpoint-index.json
            ├──> techdocs-contracts.json
            └──> generated-proto/techdocs_contract.proto
                            │
                            └──> techdocs-descriptor.json

working tree (or GitHub SHA) + LinodeMCP proto and ToolRoute options
            │
            └──> linodemcp-descriptor.json

techdocs-contracts.json
            │
            └──> route-snapshot.txt

techdocs-contracts.json + linodemcp-descriptor.json
            │
            └──> comparison.json

immutable evidence
            │
            └──> checksums.sha256

run state + report paths
            │
            ├──> run.json
            └──> root/latest.json
```

## Retention rule

At the beginning of a run, the script examines only immediate child directories whose names match:

```text
YYYY-MM-DD
```

It parses the date and removes a directory only when its age is greater than the configured retention window. The default is five days.

It deliberately ignores:

- the script file;
- `latest.json`;
- `docs/` if present at the parent workflow root;
- directories that do not match a valid date;
- current and recent date directories.

The retention test verifies that an old valid date directory is removed while a recent date and a non-run directory remain.

Run with another retention period only when explicitly needed:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof --retention-days 7
```

Negative values fail argument validation.

## Failure artifacts

A failed run is still evidence.

Expected contents may include:

- `run.json` with `status: error`;
- `latest.json` pointing to that error run;
- pages fetched before failure;
- partial indexes or descriptors, depending on stage;
- error text.

A later issue-creation process must not convert an infrastructure error into API mismatch issues. Infrastructure failures need their own alerting path.

## Stable downstream read pattern

A future orchestrator can use this sequence:

```text
1. invoke the cloud script
2. read root/latest.json
3. require recognized schema
4. require status == ok
5. resolve run_dir and report from the manifest
6. verify checksums.sha256
7. require completed_at newer than the last consumed run
8. read comparison.json
9. group findings
10. begin issue policy
```

The caller should not hardcode the timestamped run path.

## Idempotence and consumption state

The proof itself can run more than once per day. It creates a new timestamped run each time.

The later consumer should maintain its own last-consumed identifier, such as:

```text
completed_at + report checksum
```

That state must live outside the retained evidence directory if it needs to outlive the five-day window.

The consumer should never mutate `comparison.json` to mark findings consumed. Run artifacts remain immutable evidence.
