#!/usr/bin/env python3
"""Generated-form gate: every tool surface serves its proto-generated shape.

One pass per tool surface, merging the former write-proto, read-proto,
meta-proto, and input-proto gates:

  write  success paths of the mutating handlers (Write/Destroy/Admin)
  read   success paths of the Read handlers
  meta   success paths of the Meta handlers
  input  the advertised MCP input schema of every tool

The conformance corpus proves a proto MESSAGE round-trips Go==Python. It does
NOT prove any HANDLER returns that message, or where an advertised schema comes
from: a Go handler can emit a proto envelope while its Python twin hand-builds
a dict, and both pass. Two independent classifiers close that hole per surface,
statically (no handler is executed):

  Go:     go run ./cmd/write-proto-dump -surface <s> -> {tool: status}
  Python: linodemcp.tools._write_proto_classifier classify(<s>)

A handler surface is done when both sides reach the proto serializers
("proto"); the input surface when both factories load the contract's strict
schema ("generated"). A tool at any other status on either side is a
STRAGGLER and fails by name. This is a HARD gate: no baseline file, no
acceptance path, because the conversion is finished and a tool is born
generated now that the emitter writes every factory and handler from the
contract. A straggler means someone hand-wrote what the contract carries,
which is the thing to fix rather than to record.

The write surface carries one extra check: every *WriteResponse proto in
proto/linode/mcp/v1 must have a fixture registered in the Go conformance
corpus, or the gate fails naming the message.

Each surface reports its own OK line, so a regression names its slice.

Run directly, via `make generated-form` (root Makefile), or as a pre-commit
hook. The Go dumper needs the Go toolchain; the Python classifier is imported
under the venv, so this runs under the venv interpreter (the Makefile/hook do
that).
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

# Surface name to the status a finished tool classifies as on both sides,
# the OK-line spelling of that status, and what the surface covers.
_SURFACES = {
    "write": ("proto", "proto-routed", "mutating handlers"),
    "read": ("proto", "proto-routed", "read handlers"),
    "meta": ("proto", "proto-routed", "Meta handlers"),
    "input": ("generated", "proto-generated", "tool input schemas"),
}

# Tools whose success body is intentionally not a proto message. monitor's
# token create returns a bare {token, expiry} struct by design
# (proto-everywhere Wave 2). Excluded from the write surface only.
_ALLOWLIST: dict[str, frozenset[str]] = {
    "write": frozenset({"linode_monitor_service_token_create"}),
}


def _dump_go(surface: str) -> dict[str, str]:
    """Run the Go classifier for one surface and return {tool: status}."""
    result = subprocess.run(
        ["go", "run", "./cmd/write-proto-dump", "-surface", surface],
        cwd=_GO_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = (
            f"go write-proto-dump -surface {surface} failed (exit {result.returncode})"
        )
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _dump_python(surface: str) -> dict[str, str]:
    """Import the Python classifier for one surface."""
    if str(_PY_SRC) not in sys.path:
        sys.path.insert(0, str(_PY_SRC))

    from linodemcp.tools._write_proto_classifier import classify  # noqa: PLC0415

    parsed: dict[str, str] = classify(surface)
    return parsed


def _stragglers(
    go: dict[str, str], py: dict[str, str], done: str, allowed: frozenset[str]
) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for one surface.

    A straggler is any tool (outside the surface's allowlist) not at the done
    status on both sides. The line carries both statuses so the finding says
    which side needs work rather than only that the two disagree.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        if tool in allowed:
            continue

        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != done or py_status != done:
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def check_surface(surface: str) -> int:
    """Classify one surface on both sides and report its stragglers."""
    done, spelled, covers = _SURFACES[surface]
    go = _dump_go(surface)
    py = _dump_python(surface)

    _hardgate.measured(f"the Go {surface}-surface classifier", len(go))
    _hardgate.measured(f"the Python {surface}-surface classifier", len(py))

    code = _hardgate.report(
        f"{covers} not {spelled} on both sides",
        _stragglers(go, py, done, _ALLOWLIST.get(surface, frozenset())),
        "Serve the tool's generated form in every language. A generated tool"
        " gets that from the contract, so a straggler is hand-written code"
        " standing where generated code belongs.",
    )

    if code == 0:
        _hardgate.say(
            f"generated-form {surface} OK: {len(go)} Go and {len(py)} Python"
            f" {covers} {spelled}"
        )

    return code


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

    A proto is registered iff it appears as linode.mcp.v1.<Name>WriteResponse
    in the corpus test's message registry, which also implies a testdata
    fixture.
    """
    text = _CORPUS_TEST.read_text(encoding="utf-8")
    pattern = re.compile(r"linode\.mcp\.v1\.([A-Za-z0-9]+WriteResponse)\b")

    return set(pattern.findall(text))


def _missing_fixtures(declared: set[str]) -> list[str]:
    """Return sorted *WriteResponse protos that lack a conformance fixture."""
    return sorted(declared - _registered_write_response_protos())


def check_fixtures() -> int:
    """Report every *WriteResponse proto with no conformance fixture."""
    declared = _write_response_protos()

    _hardgate.measured("the *WriteResponse proto scan", len(declared))

    code = _hardgate.report(
        "*WriteResponse protos with no conformance fixture",
        _missing_fixtures(declared),
        "Add a testdata fixture and register the message in"
        " go/internal/tools/proto_conformance_corpus_test.go, so the corpus"
        " proves the message round-trips Go==Python.",
    )

    if code == 0:
        _hardgate.say(
            f"generated-form fixtures OK: every *WriteResponse ({len(declared)})"
            " has a conformance fixture"
        )

    return code


def main() -> int:
    """Check every surface, then the fixtures, reporting each slice by name."""
    code = 0
    for surface in _SURFACES:
        code = check_surface(surface) or code

    return check_fixtures() or code


if __name__ == "__main__":
    raise SystemExit(main())
