#!/usr/bin/env python3
"""Handler-level write-proto completeness gate.

The conformance corpus proves a proto MESSAGE round-trips Go==Python. It does
NOT prove any HANDLER returns that message. So a Go write handler can emit a
proto envelope while its Python twin hand-builds a dict, and both pass. This
gate closes that hole: it statically classifies every MUTATING tool (capability
Write/Destroy/Admin) on BOTH sides as proto-routed or legacy, then ratchets the
straggler set down so a handler cannot stay or go legacy unnoticed.

Two independent classifiers do the static analysis (no handler is executed):

  Go:     go run ./cmd/write-proto-dump  -> {tool: "proto"|"legacy"}
          proto = success returns MarshalProtoToolResponse, or a destroy whose
          Success closure returns a proto.Message. legacy = MarshalToolResponse
          / a map Success / one of the RunDestructiveAction*With* wrappers.
  Python: python -m linodemcp.tools._write_proto_classifier -> {tool: "proto"|...}
          proto = the handler reaches serialize_api_response/serialize_list_response.
          legacy = it builds a curated dict with no serialize call.

A tool is a STRAGGLER when either side is not proto (minus the allowlist of
intentionally-bare tools). The gate passes iff the current straggler set is a
subset of docs/write-proto-baseline.txt: a NEW straggler fails, and a straggler
that got fixed (removed from both classifiers) must be dropped from the baseline
(the file only shrinks). Regenerate with --update-baseline.

A second check ratchets conformance fixtures: every *WriteResponse proto in
proto/linode/mcp/v1 should have a fixture registered in the Go conformance
corpus. Missing ones are pinned in docs/write-proto-fixture-baseline.txt and
ratchet the same way.

Run directly, via `make write-proto` (root Makefile), or as a pre-commit hook.
The Go dumper needs the Go toolchain; the Python classifier is imported under
the venv, so this runs under the venv interpreter (the Makefile/hook do that).
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_DIR = _REPO_ROOT / "go"
_PY_SRC = _REPO_ROOT / "python" / "src"
_PROTO_DIR = _REPO_ROOT / "proto" / "linode" / "mcp" / "v1"
_CORPUS_TEST = (
    _REPO_ROOT / "go" / "internal" / "tools" / "proto_conformance_corpus_test.go"
)

_STRAGGLER_BASELINE = _REPO_ROOT / "docs" / "write-proto-baseline.txt"
_FIXTURE_BASELINE = _REPO_ROOT / "docs" / "write-proto-fixture-baseline.txt"

# Tools whose success body is intentionally not a proto message. monitor's token
# create returns a bare {token, expiry} struct by design (proto-everywhere Wave
# 2). These are excluded from the straggler set on both sides.
_ALLOWLIST: frozenset[str] = frozenset({"linode_monitor_service_token_create"})


def _dump_go() -> dict[str, str]:
    """Run the Go classifier and return {tool: "proto"|"legacy"}."""
    result = subprocess.run(
        ["go", "run", "./cmd/write-proto-dump"],
        cwd=_GO_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"go write-proto-dump failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _dump_python() -> dict[str, str]:
    """Import the Python classifier and return {tool: "proto"|"legacy"}."""
    if str(_PY_SRC) not in sys.path:
        sys.path.insert(0, str(_PY_SRC))

    from linodemcp.tools._write_proto_classifier import classify  # noqa: PLC0415

    return classify()


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any tool (outside the allowlist) that is not proto-routed on
    both sides. The line carries both statuses so the baseline documents which
    side needs work and a status flip (legacy -> review, say) is a new line.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        if tool in _ALLOWLIST:
            continue

        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != "proto" or py_status != "proto":
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def _write_response_protos() -> set[str]:
    """Return the set of *WriteResponse message names declared in the protos."""
    pattern = re.compile(r"^message\s+([A-Za-z0-9]+WriteResponse)\b", re.MULTILINE)
    names: set[str] = set()

    for proto_file in sorted(_PROTO_DIR.glob("*.proto")):
        text = proto_file.read_text(encoding="utf-8")
        names.update(pattern.findall(text))

    return names


def _registered_write_response_protos() -> set[str]:
    """Return the *WriteResponse names registered in the Go conformance corpus.

    A proto is registered iff it appears as linode.mcp.v1.<Name>WriteResponse in
    the corpus test's message registry, which also implies a testdata fixture.
    """
    text = _CORPUS_TEST.read_text(encoding="utf-8")
    pattern = re.compile(r"linode\.mcp\.v1\.([A-Za-z0-9]+WriteResponse)\b")

    return set(pattern.findall(text))


def _missing_fixtures() -> list[str]:
    """Return sorted *WriteResponse protos that lack a conformance fixture."""
    return sorted(_write_response_protos() - _registered_write_response_protos())


def _load_baseline(path: Path) -> set[str]:
    """Read a ratchet baseline into a set, skipping comments and blanks."""
    if not path.exists():
        return set()

    entries: set[str] = set()
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            entries.add(stripped)

    return entries


def _write_baseline(path: Path, header: str, entries: list[str]) -> None:
    """Overwrite a baseline file with the sorted current entries."""
    path.write_text(header + "\n".join(sorted(entries)) + "\n", encoding="utf-8")


_STRAGGLER_HEADER = (
    "# Handler-level write-proto stragglers: mutating tools not yet proto-routed\n"
    "# on both sides. One line per straggler: <tool>\\t<go_status>\\t<py_status>.\n"
    "# Ratchet: convert a handler to proto on both sides, then remove its line;\n"
    "# never add a line by hand. Regenerate:\n"
    "#   python scripts/verify_write_proto.py --update-baseline\n"
)

_FIXTURE_HEADER = (
    "# *WriteResponse protos still missing a conformance fixture in the Go corpus\n"
    "# (go/internal/tools/proto_conformance_corpus_test.go). Ratchet: add a\n"
    "# testdata fixture and register the message, then remove its line.\n"
    "# Regenerate: python scripts/verify_write_proto.py --update-baseline\n"
)


def _update_baselines(stragglers: list[str], missing: list[str]) -> int:
    """Rewrite both baselines to the current sets and report the counts."""
    _write_baseline(_STRAGGLER_BASELINE, _STRAGGLER_HEADER, stragglers)
    _write_baseline(_FIXTURE_BASELINE, _FIXTURE_HEADER, missing)
    print(
        f"baselines updated: {len(stragglers)} straggler(s), "
        f"{len(missing)} missing fixture(s)"
    )
    return 0


def _report_drift(
    label: str, current: set[str], baseline: set[str], fix_hint: str
) -> bool:
    """Print new/fixed drift for one ratchet. Return True when it is clean."""
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        print(f"{label} OK: {len(baseline)} known, unchanged")
        return True

    if new:
        print(f"NEW {label} ({len(new)}):")
        for line in new:
            print(f"  {line}")

    if fixed:
        print(f"\nFIXED {label} ({len(fixed)}) - remove these lines:")
        for line in fixed:
            print(f"  {line}")
        print(f"\nRun: {fix_hint}")

    return False


def main() -> int:
    go = _dump_go()
    py = _dump_python()

    stragglers = _stragglers(go, py)
    missing = _missing_fixtures()

    if "--update-baseline" in sys.argv:
        return _update_baselines(stragglers, missing)

    fix_hint = "python scripts/verify_write_proto.py --update-baseline"

    straggler_ok = _report_drift(
        "write-proto stragglers",
        set(stragglers),
        _load_baseline(_STRAGGLER_BASELINE),
        fix_hint,
    )
    fixture_ok = _report_drift(
        "write-proto fixtures",
        set(missing),
        _load_baseline(_FIXTURE_BASELINE),
        fix_hint,
    )

    return 0 if straggler_ok and fixture_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
