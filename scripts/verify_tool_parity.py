#!/usr/bin/env python3
"""Cross-language tool-parity verifier.

Dumps the Go and Python tool registries and compares, per tool, the
observable contract that a client or model sees: capability tier, the set of
input parameters, each parameter's JSON-Schema type, and the required set.
Descriptions are intentionally ignored, since wording is allowed to differ.

Run it directly, via ``make tool-parity`` (root Makefile), or as a pre-commit
hook. Exits non-zero and prints every divergence when the two implementations
disagree, so a tool cannot drift in shape between Go and Python.

The Go surface comes from ``go run ./cmd/parity-dump`` (built with a throwaway
config; tool registration makes no network calls). The Python surface comes
from importing ``linodemcp``'s registry, so this must run under the Python
venv interpreter (the Makefile and pre-commit hook do that).
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_DIR = _REPO_ROOT / "go"
_PY_SRC = _REPO_ROOT / "python" / "src"


def _dump_go() -> dict[str, dict[str, Any]]:
    """Run the Go dumper and return {tool_name: normalized_record}."""
    result = subprocess.run(
        ["go", "run", "./cmd/parity-dump"],
        cwd=_GO_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"go dumper failed (exit {result.returncode})"
        raise SystemExit(msg)

    records = json.loads(result.stdout)
    return {rec["name"]: _normalize(rec) for rec in records}


def _dump_python() -> dict[str, dict[str, Any]]:
    """Import the Python registry and return {tool_name: normalized_record}."""
    if str(_PY_SRC) not in sys.path:
        sys.path.insert(0, str(_PY_SRC))

    from linodemcp.server import get_tool_registry  # noqa: PLC0415

    out: dict[str, dict[str, Any]] = {}
    for entry in get_tool_registry():
        schema: dict[str, Any] = entry.tool.inputSchema or {}
        properties: dict[str, Any] = schema.get("properties", {})
        params: dict[str, str] = {}
        for pname, prop in properties.items():
            ptype = prop.get("type", "") if isinstance(prop, dict) else ""
            params[pname] = ptype if isinstance(ptype, str) else ""
        out[entry.name] = _normalize(
            {
                "name": entry.name,
                "capability": entry.capability.name,
                "params": params,
                "required": schema.get("required", []),
            }
        )
    return out


# Go's mcp-go only offers WithNumber (emits "number"); Python uses "integer".
# They are equivalent for the integer ids/pages that dominate, and the Linode
# API rejects non-integers anyway, so collapse them. A real float-vs-int bug
# would instead surface as number-vs-string or a missing param.
_TYPE_ALIASES = {"integer": "number"}


def _canon_type(typ: str) -> str:
    """Canonicalize a JSON-Schema type so integer and number compare equal."""
    return _TYPE_ALIASES.get(typ, typ)


def _normalize(rec: dict[str, Any]) -> dict[str, Any]:
    """Sort the required list and canonicalize param types for comparison."""
    params = {
        name: _canon_type(str(typ)) for name, typ in (rec.get("params") or {}).items()
    }
    return {
        "capability": rec["capability"],
        "params": params,
        "required": sorted(rec.get("required") or []),
    }


def _compare(go: dict[str, dict[str, Any]], py: dict[str, dict[str, Any]]) -> list[str]:
    """Return a sorted list of human-readable divergence lines."""
    problems: list[str] = []

    problems.extend(
        f"{name}: registered in Go but not Python" for name in sorted(set(go) - set(py))
    )
    problems.extend(
        f"{name}: registered in Python but not Go" for name in sorted(set(py) - set(go))
    )

    for name in sorted(set(go) & set(py)):
        problems.extend(_compare_one(name, go[name], py[name]))

    return problems


def _compare_one(name: str, go: dict[str, Any], py: dict[str, Any]) -> list[str]:
    """Compare a single tool's capability, params, types, and required set."""
    out: list[str] = []

    if go["capability"] != py["capability"]:
        out.append(
            f"{name}: capability Go={go['capability']} Python={py['capability']}"
        )

    go_params, py_params = go["params"], py["params"]

    out.extend(
        f"{name}: param '{param}' in Go but not Python"
        for param in sorted(set(go_params) - set(py_params))
    )
    out.extend(
        f"{name}: param '{param}' in Python but not Go"
        for param in sorted(set(py_params) - set(go_params))
    )
    out.extend(
        f"{name}: param '{param}' type "
        f"Go={go_params[param] or '?'} Python={py_params[param] or '?'}"
        for param in sorted(set(go_params) & set(py_params))
        if go_params[param] != py_params[param]
    )

    if go["required"] != py["required"]:
        out.append(f"{name}: required Go={go['required']} Python={py['required']}")

    return out


_BASELINE = _REPO_ROOT / "docs" / "tool-parity-baseline.txt"


def _load_baseline() -> set[str]:
    """Read the accepted-divergence baseline (one line per known divergence)."""
    if not _BASELINE.exists():
        return set()
    lines: set[str] = set()
    for raw in _BASELINE.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            lines.add(stripped)
    return lines


def main() -> int:
    go = _dump_go()
    py = _dump_python()
    current = set(_compare(go, py))
    baseline = _load_baseline()

    # The gate is a ratchet: the baseline can only shrink. New divergences
    # (not in the baseline) and stale baseline entries (fixed, so no longer
    # diverging) both fail, so the file stays accurate and the count drops.
    if "--update-baseline" in sys.argv:
        _BASELINE.write_text(
            "# Accepted (known) Go/Python tool-parity divergences. Ratchet:\n"
            "# fix one and remove its line; never add a line by hand. Regenerate\n"
            "# with: python scripts/verify_tool_parity.py --update-baseline\n"
            + "\n".join(sorted(current))
            + "\n",
            encoding="utf-8",
        )
        print(f"baseline updated: {len(current)} accepted divergence(s)")
        return 0

    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        print(
            f"tool parity OK: {len(go)} tools, "
            f"{len(baseline)} known divergence(s) unchanged"
        )
        return 0

    if new:
        print(f"NEW tool-parity divergence(s) ({len(new)}):")
        for line in new:
            print(f"  {line}")
    if fixed:
        print(f"\nFIXED since baseline ({len(fixed)}) - remove these lines:")
        for line in fixed:
            print(f"  {line}")
        print("\nRun: python scripts/verify_tool_parity.py --update-baseline")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
