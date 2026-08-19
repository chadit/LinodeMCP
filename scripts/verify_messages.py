#!/usr/bin/env python3
"""Cross-language confirm-sentence rendering-parity gate.

Every confirm gate on the surface is emitted now. One emitter reads the
`confirm_message` option out of the proto contract and each language's renderer
arm writes it into that language's tool tree, so what this gate proves is that
the two renderings agree: one declaration, two renderer arms, identical text.
It is not diffing two hand-written copies any more, because there are none left
to diff.

That is why both trees are scanned. The generated tree is where the sentences
live; the hand-written tree is still read for whatever has not migrated yet, so
a tool that goes back to being hand-written cannot slip past by leaving the
scanned surface.

The behavior fixtures pin the confirm messages someone wrote a case for; this
gate covers EVERY extractable one, so a text drift on a branch no fixture
exercises still fails. The extraction is heuristic (scripts/_msg_extract_go.py /
_msg_extract_py.py, promoted from the P1 sweep's tooling): it pairs each tool
with the message its confirm gate emits, and the gate compares the intersection
of tools both extractors could resolve. Tools only one side resolves are a
coverage note, not a failure (the heuristics have documented blind spots, e.g.
messages built in helpers).

This is a HARD gate: any divergence fails by name. There is no baseline file
and no acceptance path, because two languages showing a user different words
before the same mutation is not something to accept for a while.

Run directly, via `make messages` (root Makefile), or as a pre-commit hook.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import _hardgate

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SCRIPTS = _REPO_ROOT / "scripts"


def _extract(script: str, *args: str) -> dict[str, str]:
    """Run one extractor and parse its JSON stdout."""
    result = subprocess.run(
        [sys.executable, str(_SCRIPTS / script), *args],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"{script} failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _compare() -> tuple[int, list[str]]:
    """Return how many tools were compared and the divergence lines among them.

    The compared count is the intersection both extractors resolved, and it is
    reported because it is the gate's whole reach: a heuristic that stops
    matching (a renamed helper, a moved tools tree) empties the intersection,
    and an empty intersection has no divergences to find.
    """
    go_map = _extract(
        "_msg_extract_go.py",
        ",".join(
            (
                str(_REPO_ROOT / "go" / "internal" / "tools"),
                str(_REPO_ROOT / "go" / "internal" / "gentools"),
            )
        ),
        str(_REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"),
    )
    py_map = _extract(
        "_msg_extract_py.py",
        ",".join(
            (
                str(_REPO_ROOT / "python" / "src" / "linodemcp" / "tools"),
                str(_REPO_ROOT / "python" / "src" / "linodemcp" / "gentools"),
            )
        ),
    )

    compared = sorted(set(go_map) & set(py_map))
    lines: list[str] = [
        f"{tool}\t{go_map[tool]}\t{py_map[tool]}"
        for tool in compared
        if go_map[tool] != py_map[tool]
    ]

    return len(compared), lines


def main() -> int:
    compared, divergences = _compare()

    _hardgate.measured("the confirm-message extractors", compared)

    code = _hardgate.report(
        "tools whose confirm text differs between languages",
        divergences,
        "Align the text to the reference language's, or take it from the"
        " contract's confirm_message option so neither side holds a copy."
        " Each line is <tool>, then the Go text, then the Python text.",
    )

    if code == 0:
        _hardgate.say(f"message parity OK: {compared} confirm message(s) agree")

    return code


if __name__ == "__main__":
    raise SystemExit(main())
