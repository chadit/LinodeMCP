#!/usr/bin/env python3
"""Doc drift guard: README's tool count must match the manifest.

README.md's Status section cites docs/contracts/tools-manifest.txt and states
how many tools it lists. That hand-written number goes stale when the surface
grows or shrinks (the manifest regenerates, the prose does not). This guard
reads the manifest's real entry count and fails when a README line that cites
the manifest states a different "<N> tools", so the count stays single-sourced
in the manifest that review can trust.

Stdlib only, so no venv is needed. Run via `make tool-count` (in `make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[1]
_MANIFEST = _REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"
_README = _REPO_ROOT / "README.md"

# The README line that states the count also links the manifest file, so anchor
# the check to that link and read the count next to it. A bare "<N> tools"
# elsewhere in the README (unrelated feature counts) is intentionally ignored.
# Match both "460 tools" and the "454-tool surface" phrasing so either wording
# is checked rather than silently skipped.
_MANIFEST_LINK = "tools-manifest.txt"
_COUNT_RE = re.compile(r"(\d+)[\s-]tools?\b")


def manifest_total(path: Path) -> int:
    """Count the tool entries the manifest pins (non-comment, non-blank lines)."""
    return sum(
        1
        for raw in path.read_text(encoding="utf-8").splitlines()
        if raw.strip() and not raw.strip().startswith("#")
    )


def readme_claims(text: str) -> list[int]:
    """Return every '<N> tools' count on a README line that cites the manifest."""
    claims: list[int] = []
    for line in text.splitlines():
        if _MANIFEST_LINK in line:
            claims.extend(int(match) for match in _COUNT_RE.findall(line))
    return claims


def main() -> int:
    total = manifest_total(_MANIFEST)
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
