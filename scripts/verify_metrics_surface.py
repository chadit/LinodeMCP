#!/usr/bin/env python3
"""Offline gate: the metrics surface matches across languages.

The exported telemetry is an operator-facing contract: dashboards and
alerts key on instrument names and attribute labels, so a one-sided rename
forks every consumer silently. Bucket boundaries are already pinned by
testdata/observability/duration_buckets.json and each language's unit
tests; this gate covers the other two axes, extracted from source:

- instrument names: the "linodemcp.*" literals at the meter-creation sites,
- metric attribute keys: the label names each language attaches when it
  records (Go: attribute.X("key") in metrics.go; Python: the "key": entries
  in the record-call attribute dicts).

Sets are compared, not counts: how many record sites share a label is an
implementation choice, which labels exist on the wire is not.

Stdlib only, so no venv is needed. Run via `make metrics-surface` (in
`make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parent.parent
_GO_METRICS = _REPO_ROOT / "go" / "internal" / "observability" / "metrics.go"
_PY_OBSERVABILITY = (
    _REPO_ROOT / "python" / "src" / "linodemcp" / "observability" / "__init__.py"
)

_INSTRUMENT = re.compile(r'"(linodemcp\.[a-z.]+)"')
_GO_ATTRIBUTE = re.compile(r'attribute\.[A-Za-z]+\("([a-z_]+)"')
_PY_ATTRIBUTE = re.compile(r'"([a-z_]+)":')


def go_metrics_surface() -> tuple[set[str], set[str]]:
    """(instrument names, attribute keys) declared in Go's metrics.go."""
    text = _GO_METRICS.read_text(encoding="utf-8")
    return set(_INSTRUMENT.findall(text)), set(_GO_ATTRIBUTE.findall(text))


def python_metrics_surface() -> tuple[set[str], set[str]]:
    """(instrument names, attribute keys) declared in Python's observability.

    Attribute keys are taken only from the dict literal passed to an
    instrument's ``.add(value, {...})`` or ``.record(value, {...})`` call,
    so config keys elsewhere in the module never leak into the comparison.
    """
    text = _PY_OBSERVABILITY.read_text(encoding="utf-8")
    attributes: set[str] = set()
    for block in re.findall(
        r"\.(?:add|record)\(\s*[^,]+,\s*\{(.*?)\}", text, re.DOTALL
    ):
        attributes.update(_PY_ATTRIBUTE.findall(block))
    return set(_INSTRUMENT.findall(text)), attributes


def metrics_violations() -> list[str]:
    """Human-readable differences between the two metrics surfaces."""
    problems: list[str] = []
    go_instruments, go_attributes = go_metrics_surface()
    py_instruments, py_attributes = python_metrics_surface()

    surfaces = [
        ("instrument", go_instruments, py_instruments),
        ("attribute key", go_attributes, py_attributes),
    ]
    for label, go_side, python_side in surfaces:
        if not go_side or not python_side:
            problems.append(
                f"{label}s: extraction found nothing for"
                f" {'go' if not go_side else 'python'};"
                " the source shape changed, update this gate's patterns"
            )
            continue
        problems.extend(
            f"{label} {name} exists in go only"
            for name in sorted(go_side - python_side)
        )
        problems.extend(
            f"{label} {name} exists in python only"
            for name in sorted(python_side - go_side)
        )
    return problems


def main() -> int:
    problems = metrics_violations()
    if problems:
        print("metrics surface diverges between languages:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        print(
            "  (rename or add the instrument/label in every language in the"
            " same change; dashboards key on these names)",
            file=sys.stderr,
        )
        return 1

    instruments, attributes = go_metrics_surface()
    print(
        f"metrics-surface gate OK: {len(instruments)} instruments and"
        f" {len(attributes)} attribute keys match across languages"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
