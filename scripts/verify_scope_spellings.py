#!/usr/bin/env python3
"""Offline gate: every language's Scope catalog spells what the emitter spells.

Per-tool scopes are declared in the proto contract and rendered by
go/cmd/toolgen, whose scopeStrings map is the one place a scope's wire
spelling is written. Each language also keeps a small hand catalog of Scope
constants for token-side parsing (the OAuth grants converter's vocabulary),
and nothing else compares those constants to anything: a catalog value that
drifts from the emitter's spelling makes the grants converter emit a value no
profile requirement ever matches, and both directions of that failure are
silent at runtime.

So this pins two facts. Every non-wildcard catalog value must spell a family
the emitter renders, either as one of the emitter's own wire spellings or as
that family's read_only/read_write sibling, because the grants converter's
vocabulary carries pairs while the tool surface may declare only one half
(vpc:read_only is the live case: the converter reads it, no tool declares
it). And every registered language's catalog must carry the same value set,
so one language cannot validate a token differently from another. The
wildcard "*" is token-side vocabulary the emitter never spells, which is why
it is the one exemption.

Which languages are asked comes from docs/contracts/languages.txt. A
registered language with no catalog reader here fails by name rather than
going unchecked. There is no baseline: this is a pure consistency check with
nothing to ratchet.

Run via `make scope-spellings`, which `make check` includes.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_EMITTER = _REPO_ROOT / "go" / "cmd" / "toolgen" / "scopes.go"

# The token-side wildcard, the one catalog value the emitter never spells.
_WILDCARD = "*"

# One catalog reader per registered language: the file the catalog lives in
# and the pattern one constant matches, capturing (name, value).
_CATALOGS: dict[str, tuple[Path, re.Pattern[str]]] = {
    "go": (
        _REPO_ROOT / "go" / "internal" / "profiles" / "scope.go",
        re.compile(r"^\t(Scope\w+)\s+Scope = \"([^\"]+)\"$", re.MULTILINE),
    ),
    "python": (
        _REPO_ROOT / "python" / "src" / "linodemcp" / "profiles" / "scope.py",
        re.compile(r"^    (\w+) = \"([^\"]+)\"$", re.MULTILINE),
    ),
}


def registered_languages(path: Path) -> list[str]:
    """The language names the registry declares, in file order."""
    languages = [
        line.strip().split("\t", maxsplit=1)[0].strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def emitter_spellings() -> set[str]:
    """The wire spellings the toolgen emitter renders, from its one map."""
    spellings = set(
        re.findall(
            r"linodev1\.ToolScope_TOOL_SCOPE_\w+:\s*\"([^\"]+)\"",
            _EMITTER.read_text(encoding="utf-8"),
        )
    )
    if not spellings:
        msg = f"{_EMITTER} yields no scope spellings; the emitter map moved"
        raise SystemExit(msg)

    return spellings


def read_catalog(language: str) -> dict[str, str]:
    """One language's Scope catalog as {constant name: value}."""
    path, pattern = _CATALOGS[language]
    catalog = dict(pattern.findall(path.read_text(encoding="utf-8")))
    if not catalog:
        msg = f"{language}: {path} yields no Scope constants; the catalog moved"
        raise SystemExit(msg)

    return catalog


def misspelled(
    language: str, catalog: dict[str, str], spellings: set[str]
) -> list[str]:
    """Catalog values outside the emitter's families, wildcard exempted."""
    families = {value.split(":", maxsplit=1)[0] for value in spellings}

    return [
        f"{language}: {name} = {value!r} does not spell a family the emitter renders"
        for name, value in sorted(catalog.items())
        if value != _WILDCARD and not _spells_family(value, spellings, families)
    ]


def _spells_family(value: str, spellings: set[str], families: set[str]) -> bool:
    """Whether one catalog value is an emitter spelling or a well-formed
    read_only/read_write sibling of a family the emitter renders."""
    if value in spellings:
        return True

    family, _, suffix = value.partition(":")

    return family in families and suffix in {"read_only", "read_write"}


def main() -> int:
    languages = registered_languages(_LANGUAGES)
    spellings = emitter_spellings()

    problems: list[str] = []
    value_sets: dict[str, set[str]] = {}

    for language in languages:
        if language not in _CATALOGS:
            problems.append(
                f"{language} is registered in languages.txt and has no"
                " catalog reader here; add one or record the exemption"
            )
            continue

        catalog = read_catalog(language)
        value_sets[language] = set(catalog.values())
        problems.extend(misspelled(language, catalog, spellings))

    if len(value_sets) > 1:
        reference, *others = [name for name in languages if name in value_sets]
        for other in others:
            problems.extend(
                f"{value!r} is in the {reference} catalog and not the {other} one"
                for value in sorted(value_sets[reference] - value_sets[other])
            )
            problems.extend(
                f"{value!r} is in the {other} catalog and not the {reference} one"
                for value in sorted(value_sets[other] - value_sets[reference])
            )

    if problems:
        print("scope catalog spelling drift:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)

        return 1

    counted = next(iter(value_sets.values()))
    print(
        f"scope spellings OK: {len(counted)} catalog value(s) match the"
        f" emitter across {len(value_sets)} language(s)"
    )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
