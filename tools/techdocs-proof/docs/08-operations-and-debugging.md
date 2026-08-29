# 8. Operations and Debugging

## Normal run

From the repository root, on any host with Python, Buf, and network access:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof
```

The command fetches the complete rendered snapshot. A normal run takes roughly one to several minutes, depending on TechDocs response times.

Progress is written to stderr every 50 completed page fetches:

```text
fetched 50/516 pages
fetched 100/516 pages
...
```

The finished manifest is printed as JSON.

## Strict exit mode

To return exit `3` when findings exist:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof --fail-on-findings
```

Use this only when the caller understands that exit `3` means a completed comparison, not collector failure.

## Development page limit

For parser development only:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof --max-pages 10
```

A limited run is not a production proof. It will naturally produce many route-absence findings because most TechDocs pages were intentionally omitted. Never feed a limited run into issue creation.

## Self-test

```bash
make techdocs-proof
```

or directly:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof --self-test
```

Expected output:

```text
SELF_TEST_OK
```

The self-test exercises core comparison behavior, including the exact system-parameter marker policy, the evidence-root refusal, and the shipped exclusion ledger. It touches no network and starts no subprocess, which is why it is the arm `make check` runs.

## Reading the latest result

```bash
python3 - <<'PY'
import json

from techdocs_proof.proof import evidence_root

root = evidence_root()
latest = json.loads((root / "latest.json").read_text())
print(json.dumps(latest, indent=2, sort_keys=True))
PY
```

Check status first:

```bash
python3 - <<'PY'
import json

from techdocs_proof.proof import evidence_root

root = evidence_root()
p = root / "latest.json"
data = json.loads(p.read_text())
if data.get("status") != "ok":
    raise SystemExit(f"latest proof failed: {data.get('error')}")
print(data["report"])
PY
```

## Verifying evidence

```bash
RUN_DIR=$(python3 - <<'PY'
import json
from techdocs_proof.proof import evidence_root

root = evidence_root()
p = root / "latest.json"
print(json.loads(p.read_text())["run_dir"])
PY
)

cd "$RUN_DIR"
sha256sum -c checksums.sha256
```

Do this before a downstream process creates or updates issues.

## Inspecting the finding summary

```bash
python3 - <<'PY'
from pathlib import Path
import json
from techdocs_proof.proof import evidence_root

root = evidence_root()
latest = json.loads((root / "latest.json").read_text())
report = json.loads(Path(latest["report"]).read_text())
print(json.dumps(report["summary"], indent=2, sort_keys=True))
PY
```

## Inspecting one finding kind

```bash
python3 - <<'PY'
from pathlib import Path
import json
from techdocs_proof.proof import evidence_root

root = evidence_root()
latest = json.loads((root / "latest.json").read_text())
report = json.loads(Path(latest["report"]).read_text())
kind = "route_missing_from_proto"
for finding in report["findings"]:
    if finding.get("kind") == kind:
        print(json.dumps(finding, indent=2, sort_keys=True))
        break
PY
```

## Troubleshooting by stage

### No pages discovered

Symptoms:

```text
rendered TechDocs discovery returned no pages
```

Check:

1. HTTPS access from the running host.
2. The reference root and sitemap URLs.
3. DNS resolution.
4. Whether TechDocs changed host or path structure.
5. Whether HTML link structure changed.

Do not bypass the failure with an empty contract.

### Incomplete snapshot

Symptoms:

```text
incomplete TechDocs snapshot: fetched=N discovered=M
```

Check:

- transient response codes;
- rate limiting;
- page-specific permanent errors;
- timeout behavior;
- whether a sitemap entry was removed during the run.

The correct response is to rerun after the source is stable or update discovery rules. Do not lower the expected count manually.

### Duplicate rendered route

Symptoms:

```text
duplicate rendered TechDocs route contract
```

Inspect both source URLs and pages. Possible causes:

- duplicate published pages;
- an alias page;
- version normalization collapsing distinct operations;
- a parser matching an example route instead of the page's operation.

Do not choose one page by lexical order.

### Candidate proto build failure

Check:

```text
generated-proto/techdocs_contract.proto
generated-proto/buf.yaml
```

Likely causes:

- generated identifier collision not handled by the generator;
- a string escaping case;
- a field type mapping error;
- custom option syntax;
- Buf version incompatibility.

This failure concerns the evidence generator, not LinodeMCP source.

### LinodeMCP descriptor build failure

Check:

1. Resolved GitHub repository, ref, and exact commit SHA.
2. `gh` authentication and archive retrieval.
3. `buf` resolution and version.
4. preserved `buf.yaml`, `buf.lock`, and module dependencies.
5. protobuf syntax or dependency failures in the preserved source.

Run Buf directly against the preserved run artifact to obtain compiler diagnostics. Do not fall back to generated-language imports or a shared developer worktree.

### Missing source comments

If descriptor source locations are absent, confirm the Buf output mode retains source information. The exact system-parameter rule cannot run safely without leading comments.

### Unexpected route explosion

If `proto_route_absent_from_techdocs` or `route_missing_from_proto` suddenly jumps:

1. Compare discovered and fetched page counts with the prior run.
2. Inspect a few pages for route-rendering format changes.
3. Check API-version and placeholder normalization.
4. Compare `linode.mcp.v1.tool_route` options with the prior LinodeMCP commit.
5. Confirm `linodemcp-source.json` records the intended working tree (or, under `--github-source`, the intended commit SHA).

Treat broad count changes as a possible parser or input problem before creating hundreds of issues.

### Unexpected proto-only parameter explosion

Check whether:

- route/request association changed;
- nested messages are being flattened differently;
- leading comments disappeared from descriptors;
- fields intended as internal controls lack `System parameter:` comments.

Do not suppress names globally.

## Safe reruns

Rerunning on the same day creates another timestamped directory. It does not overwrite the prior run.

After a successful rerun, `latest.json` points to the new run. Old same-day runs remain until their date directory passes retention.

## Changing the comparator

The source is versioned with the contract it measures, so there is no deployment step and no second copy to checksum.

After changing it:

1. run `make techdocs-proof` (lint and format run under `make tools-lint` and `make fmt-check`);
2. run a complete proof by hand;
3. verify `latest.json` and the artifact checksums;
4. inspect finding determinism against the same LinodeMCP tree.

Do not let a schedule pick up a new version before a complete manual proof succeeds.

## Scheduled-run contract

The scheduled workflow does little:

```text
invoke the repo command
wait for completion
read latest.json
validate status and checksums
read comparison.json
hand findings to the sync batch
```

It does not copy TechDocs pages anywhere, reconstruct the result from console logs, or modify the repository. `.github/workflows/techdocs-drift.yml` is that schedule.
