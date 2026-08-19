#!/usr/bin/env python3
"""Handler-level write-proto completeness gate.

The conformance corpus proves a proto MESSAGE round-trips Go==Python. It does
NOT prove any HANDLER returns that message. So a Go write handler can emit a
proto envelope while its Python twin hand-builds a dict, and both pass. This
gate closes that hole: it statically classifies every MUTATING tool (capability
Write/Destroy/Admin) on BOTH sides as proto-routed or legacy, so a handler
cannot stay or go legacy unnoticed.

Two independent classifiers do the static analysis (no handler is executed):

  Go:     go run ./cmd/write-proto-dump  -> {tool: "proto"|"legacy"}
          proto = success returns MarshalProtoToolResponse, or a destroy whose
          Success closure returns a proto.Message. legacy = MarshalToolResponse
          / a map Success / one of the RunDestructiveAction*With* wrappers.
  Python: python -m linodemcp.tools._write_proto_classifier -> {tool: "proto"|...}
          proto = the handler reaches serialize_api_response/serialize_list_response.
          legacy = it builds a curated dict with no serialize call.

A tool is a STRAGGLER when either side is not proto (minus the allowlist of
intentionally-bare tools). This is a HARD gate: any straggler fails by name.
There is no baseline file and no acceptance path, because the conversion is
finished and a tool is born proto-routed now that the emitter writes every
handler from the contract. A straggler means someone hand-wrote a success path,
which is the thing to fix rather than to record.

A second check holds conformance fixtures the same way: every *WriteResponse
proto in proto/linode/mcp/v1 must have a fixture registered in the Go
conformance corpus, or the gate fails naming the message.

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

import _hardgate

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_DIR = _REPO_ROOT / "go"
_PY_SRC = _REPO_ROOT / "python" / "src"
_PROTO_DIR = _REPO_ROOT / "proto" / "linode" / "mcp" / "v1"
_CORPUS_TEST = (
    _REPO_ROOT / "go" / "internal" / "tools" / "proto_conformance_corpus_test.go"
)

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

    parsed: dict[str, str] = classify()
    return parsed


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any tool (outside the allowlist) that is not proto-routed on
    both sides. The line carries both statuses so the finding says which side
    needs work rather than only that the two disagree.
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


def _missing_fixtures(declared: set[str]) -> list[str]:
    """Return sorted *WriteResponse protos that lack a conformance fixture."""
    return sorted(declared - _registered_write_response_protos())


def main() -> int:
    go = _dump_go()
    py = _dump_python()
    declared = _write_response_protos()

    _hardgate.measured("the Go write-surface classifier", len(go))
    _hardgate.measured("the Python write-surface classifier", len(py))
    _hardgate.measured("the *WriteResponse proto scan", len(declared))

    straggler_code = _hardgate.report(
        "mutating handlers not proto-routed on both sides",
        _stragglers(go, py),
        "Route the handler's success path through its proto message in every"
        " language. A generated tool gets that from the contract, so a straggler"
        " is hand-written code standing where generated code belongs.",
    )
    fixture_code = _hardgate.report(
        "*WriteResponse protos with no conformance fixture",
        _missing_fixtures(declared),
        "Add a testdata fixture and register the message in"
        " go/internal/tools/proto_conformance_corpus_test.go, so the corpus"
        " proves the message round-trips Go==Python.",
    )

    if straggler_code == 0 and fixture_code == 0:
        _hardgate.say(
            f"write-proto OK: {len(go)} Go and {len(py)} Python mutating handlers"
            " proto-routed, every *WriteResponse fixtured"
        )

    return straggler_code or fixture_code


if __name__ == "__main__":
    raise SystemExit(main())
