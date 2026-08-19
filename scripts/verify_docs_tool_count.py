#!/usr/bin/env python3
"""Doc drift guard: README's tool count must match the proto contract.

README.md's Status section cites docs/contracts/tools-manifest.txt and states
how many tools it lists. That number is prose a human reads, so it stays written
out in the doc, and it goes stale the moment the surface grows or shrinks. This
guard supplies the moment: it counts the tools the descriptors declare and fails
when a README line citing the manifest states a different "<N> tools". The
failure prints the right number, so fixing it is mechanical.

The count comes from the descriptors rather than from the manifest file because
the manifest is itself generated from them (scripts/gen_tool_registries.py).
Counting the generated file would give the same answer through one more hop, and
that hop is a file that need not exist yet.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
resolves through python/.venv/bin/python when the running interpreter cannot
import them. `make proto` must have run. Run via `make tool-count` (in
`make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import _toolroutes

_REPO_ROOT = Path(__file__).resolve().parents[1]
_README = _REPO_ROOT / "README.md"

# The README line that states the count also links the manifest file, so anchor
# the check to that link and read the count next to it. A bare "<N> tools"
# elsewhere in the README (unrelated feature counts) is intentionally ignored.
# Match both "460 tools" and the "454-tool surface" phrasing so either wording
# is checked rather than silently skipped.
_MANIFEST_LINK = "tools-manifest.txt"
_COUNT_RE = re.compile(r"(\d+)[\s-]tools?\b")


def declared_total() -> int:
    """How many tools the proto contract declares.

    A tool names itself in exactly one of the two markers, so the union of the
    names those markers carry is the surface. `make tool-capability` is what
    holds a message to naming its tool once, which is why a name read twice here
    would be counted once rather than reported.
    """
    return len(
        {
            entry.route_tool or entry.meta_tool
            for entry in _toolroutes.declarations()
            if entry.route_tool or entry.meta_tool
        }
    )


def readme_claims(text: str) -> list[int]:
    """Return every '<N> tools' count on a README line that cites the manifest."""
    claims: list[int] = []
    for line in text.splitlines():
        if _MANIFEST_LINK in line:
            claims.extend(int(match) for match in _COUNT_RE.findall(line))
    return claims


def main() -> int:
    total = declared_total()
    claims = readme_claims(_README.read_text(encoding="utf-8"))

    if not claims:
        print(
            f"tool-count guard: README.md states no '<N> tools' count on a line "
            f"citing {_MANIFEST_LINK}; expected {total}.",
            file=sys.stderr,
        )
        return 1

    stale = sorted({n for n in claims if n != total})
    if stale:
        print(
            f"tool-count guard: README.md states {stale} beside {_MANIFEST_LINK}, "
            f"but the manifest lists {total} tools. Update the README count to "
            f"{total}.",
            file=sys.stderr,
        )
        return 1

    print(f"tool-count guard OK: README matches manifest ({total} tools)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
