"""Wildcard pattern expansion for the draft builder.

Mirrors ``go/internal/profiles/builder/match.go``. Used by the Phase
8.4 mutator methods on :class:`linodemcp.profiles.builder.Registry`
to turn the model's literal-or-wildcard tool arguments into the
explicit list stored on the Draft.
"""

from __future__ import annotations

import fnmatch
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Sequence

    from linodemcp.profiles.builtin import ToolDescriptor


_WILDCARD_CHAR = "*"


def match_patterns(
    patterns: Sequence[str],
    catalog: Sequence[ToolDescriptor],
) -> list[str]:
    """Expand patterns against the catalog, deduplicate, sort.

    - Literal entries (no ``*``) must equal a catalog tool name to
      contribute. Unknown literals contribute nothing rather than
      raising; the caller reports a hit count so the model can
      detect typos by absence.
    - Wildcard entries use ``fnmatch.fnmatch`` (shell-glob). The
      same name produced by multiple patterns appears once in the
      output.

    Returns an empty list when ``patterns`` is empty; never ``None``.
    """
    seen: set[str] = set()
    out: list[str] = []

    for pattern in patterns:
        if not pattern:
            continue

        for name in _match_one(pattern, catalog):
            if name in seen:
                continue

            seen.add(name)
            out.append(name)

    out.sort()
    return out


def _match_one(pattern: str, catalog: Sequence[ToolDescriptor]) -> list[str]:
    """Expand a single pattern against the catalog."""
    if _WILDCARD_CHAR not in pattern:
        for entry in catalog:
            if entry.name == pattern:
                return [pattern]
        return []

    return [entry.name for entry in catalog if fnmatch.fnmatch(entry.name, pattern)]
