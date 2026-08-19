#!/usr/bin/env python3
"""Offline gate: the API surface each tool answers on, three ways in agreement.

Nearly every Linode route answers under /v4. A handful answer only under
/v4beta, and which ones is a per-tool declaration (`linode.mcp.v1
.tool_api_surface`) rather than a deployment setting, so one tool can be on
another surface without moving the whole client.

That declaration reaches three places, and this holds all three to the same set:

- docs/contracts/api-surfaces.txt, the checked-in census, so the beta set is a
  reviewed diff rather than something that drifts a tool at a time;
- the contract itself, read from the generated descriptors;
- the description each language advertises, which is the only thing a model
  choosing a tool reads about where the call goes. A tool on a non-default
  surface leads its description with a "[<surface>] " marker.

Both directions, both languages. A censused tool missing its marker in either
tree fails, and so does a marker on a tool the census does not list: that one
is the contamination case, where a caller reading a listing would believe a v4
tool reaches beta.

The marker is checked against the generated trees rather than the descriptors
because the emitter adds it; the descriptors carry the declared description
without it. Which trees are scanned comes from docs/contracts/languages.txt
rather than a path literal, so a language registered with no arm here fails by
name instead of going unchecked.

Deliberately NOT a bare "beta" search: five shipped v4 tools describe Linode's
Beta Programs feature (linode_account_beta_enroll and friends), which has
nothing to do with API versioning. The marker token is what is matched.

Run via `make api-surfaces` (inside `make check`).
"""

from __future__ import annotations

import re
import sys
from typing import TYPE_CHECKING, NamedTuple

import _surface
import _toolroutes

if TYPE_CHECKING:
    from pathlib import Path

_CONTRACTS = _surface.REPO_ROOT / "docs" / "contracts"
_CENSUS = _CONTRACTS / "api-surfaces.txt"
_LANGUAGES = _CONTRACTS / "languages.txt"

# The surface an unannotated tool answers on, which the census never lists.
_DEFAULT_SURFACE = "v4"

# The declared enum value names that mean the default surface.
_DEFAULT_VALUES = frozenset({"", "API_SURFACE_UNSPECIFIED", "API_SURFACE_V4"})


class Tree(NamedTuple):
    """One language's generated tool tree, and the glob that finds its files."""

    root: str
    glob: str


_TREES = {
    "go": Tree("go/internal/gentools", "*.gen.go"),
    "python": Tree("python/src/linodemcp/gentools", "*.py"),
}

# A surface marker leading an advertised description: "[v4beta] Lists ...".
_MARKER = re.compile(r"\[(v4beta)\]\s")

# The tool a generated factory or handler names, as the emitter writes it.
_TOOL_NAME = re.compile(r'"(linode_[a-z0-9_]+|hello|version)"')


def read_census(path: Path = _CENSUS) -> dict[str, str]:
    """Tool name to the surface the census says it answers on."""
    listed: dict[str, str] = {}

    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue

        parts = line.split()
        if len(parts) != 2:
            msg = f"{path.name}: {line!r} is not '<tool> <surface>'"
            raise SystemExit(msg)

        tool, surface = parts
        if tool in listed:
            msg = f"{path.name}: {tool} is listed twice"
            raise SystemExit(msg)

        listed[tool] = surface

    return listed


def declared_surfaces() -> dict[str, str]:
    """Tool name to its declared non-default surface, read from the contract."""
    found: dict[str, str] = {}

    for entry in _toolroutes.declarations():
        if entry.api_surface in _DEFAULT_VALUES or not entry.route_tool:
            continue
        found[entry.route_tool] = entry.api_surface.removeprefix("API_SURFACE_").lower()

    return found


def registered_languages(path: Path = _LANGUAGES) -> list[str]:
    """Every language the registry lists, so none goes unscanned."""
    languages = [
        line.split("\t", maxsplit=1)[0].strip()
        for raw in path.read_text(encoding="utf-8").splitlines()
        if (line := raw.strip()) and not line.startswith("#")
    ]
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def marked_tools(language: str) -> dict[str, str]:
    """Tool name to the surface its generated description is marked with.

    A generated file names its tool beside the description, so the marker is
    attributed to the nearest tool name above it. An emitted description is one
    line in both languages, which is what makes that safe.
    """
    tree = _TREES[language]
    root = _surface.REPO_ROOT / tree.root
    found: dict[str, str] = {}

    for path in sorted(root.glob(tree.glob)):
        current = ""
        for line in path.read_text(encoding="utf-8").splitlines():
            named = _TOOL_NAME.search(line)
            if named:
                current = named.group(1)
            marker = _MARKER.search(line)
            if marker and current:
                found[current] = marker.group(1)

    return found


def census_problems(census: dict[str, str], declared: dict[str, str]) -> list[str]:
    """Ways the census and the contract disagree, in both directions."""
    problems = [
        f"{tool}: the census lists {surface}, which the contract does not declare"
        for tool, surface in sorted(census.items())
        if tool not in declared
    ]
    problems.extend(
        f"{tool}: declares {surface}, which the census does not list"
        for tool, surface in sorted(declared.items())
        if tool not in census
    )
    problems.extend(
        f"{tool}: the census says {census[tool]}, the contract says {surface}"
        for tool, surface in sorted(declared.items())
        if tool in census and census[tool] != surface
    )
    problems.extend(
        f"{tool}: the census lists the default surface {surface}"
        for tool, surface in sorted(census.items())
        if surface == _DEFAULT_SURFACE
    )

    return problems


def marker_problems(
    census: dict[str, str], language: str, marked: dict[str, str]
) -> list[str]:
    """Ways one language's advertised descriptions disagree with the census.

    Takes the scanned markers rather than reading the tree, so the checks below
    can be handed a tree that carries the contamination they exist to catch. The
    shipped trees cannot: the emitter derives the marker from the annotation.
    """
    problems = [
        f"{language}: {tool} answers on {surface} and its description carries no marker"
        for tool, surface in sorted(census.items())
        if tool not in marked
    ]
    problems.extend(
        f"{language}: {tool} advertises a {surface} marker, and the census does not"
        f" list it, so a caller would read a v4 tool as reaching another surface"
        for tool, surface in sorted(marked.items())
        if tool not in census
    )
    problems.extend(
        f"{language}: {tool} advertises {marked[tool]}, the census says {surface}"
        for tool, surface in sorted(census.items())
        if tool in marked and marked[tool] != surface
    )

    return problems


def main() -> int:
    """Report every disagreement; zero when all three readings match."""
    census = read_census()
    declared = declared_surfaces()
    languages = registered_languages()

    if unscanned := sorted(name for name in languages if name not in _TREES):
        named = ", ".join(unscanned)
        print(
            f"registered language(s) with no generated tree here: {named}",
            file=sys.stderr,
        )
        return 1

    problems = census_problems(census, declared)
    for language in languages:
        problems.extend(marker_problems(census, language, marked_tools(language)))

    if problems:
        print("API surface census and the tools do not agree:", file=sys.stderr)
        for entry in problems:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (annotate the tool with tool_api_surface, list it in"
            " docs/contracts/api-surfaces.txt, then run `make proto`)",
            file=sys.stderr,
        )
        return 1

    print(
        f"api-surfaces gate OK: {len(census)} tool(s) on a non-default surface,"
        f" marked in {len(languages)} language(s)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
