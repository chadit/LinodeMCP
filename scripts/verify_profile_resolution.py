#!/usr/bin/env python3
"""Offline gate: every built-in profile serves the same tools in every language.

A profile answers one question at dispatch: may this tool run. Each language
answers it from its own copy of two tables, the category a tool name falls in
and the categories a profile elevates, and nothing held the copies together. The
tables had already drifted on shipped surface (one language filed longview under
monitor, the other under its own category; one had an account category the other
had never heard of), and the drift stayed invisible because the profiles that
elevate those categories elevate every category.

So this diffs both halves, and the second is the one that catches the class:

- resolution: each built-in profile's allowed tools, its derived token scopes,
  and its elevated/yolo/disabled flags. This is what dispatch reads today.
- categories: the category list each tool name falls in. Two languages that
  agree here resolve any future profile the same way; two that only agree on
  today's profiles are one new category-scoped profile away from serving
  different tool sets with every gate green.

Both languages resolve against one catalog, docs/contracts/tools-capabilities.txt,
so a difference in output is a difference in the resolver rather than in what
each side thinks the surface is. `make proto` must have run to write that file.

Which languages are asked comes from docs/contracts/languages.txt. A registered
language with no resolver here fails by name rather than going unchecked, and so
does a dump that resolves nothing, which would otherwise agree with everything.

The first registered language is the reference the others are diffed against,
matching `make tool-parity`.

Run via `make profile-resolution`, which `make check` includes, and so the
pre-push hook and the CI gate.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import _surface

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"

# How to ask each language to resolve the catalog on stdin. Written here rather
# than in languages.txt because that registry's third field is the tool-surface
# dumper, a different question with a different output shape.
_RESOLVERS: dict[str, tuple[str, list[str]]] = {
    "go": ("go", ["go", "run", "./cmd/builtin-parity-dump"]),
    "python": ("python", [".venv/bin/python", "-m", "linodemcp.builtin_parity_dump"]),
}

# Compared per profile. Descriptions are left out: wording is each language's
# own, the same reason go/cmd/parity-dump omits tool descriptions.
_PROFILE_FIELDS = (
    "allowed_tools",
    "required_token_scopes",
    "elevated",
    "allow_yolo",
    "disabled",
)


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


def catalog_fixture(capabilities: dict[str, str]) -> list[dict[str, str]]:
    """The whole tool surface as the resolvers' stdin fixture."""
    if not capabilities:
        msg = (
            "docs/contracts/tools-capabilities.txt names no tools, so every"
            " resolver would agree on an empty catalog: run `make proto`"
        )
        raise SystemExit(msg)

    return [
        {"name": tool, "capability": capability}
        for tool, capability in sorted(capabilities.items())
    ]


def run_resolver(language: str, fixture: str) -> dict[str, Any]:
    """Run one language's resolver over the fixture and parse its dump."""
    workdir, argv = _RESOLVERS[language]
    result = subprocess.run(
        argv,
        cwd=_REPO_ROOT / workdir,
        input=fixture,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"{language} resolver failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, Any] = json.loads(result.stdout)
    return parsed


def profiles_by_name(dump: dict[str, Any]) -> dict[str, dict[str, Any]]:
    """The dump's profiles keyed by name, from either export shape.

    Go exports a list sorted by name and Python a mapping; both are each
    language's own canonical export, so the gate reads what ships rather than
    asking either to grow a shape for this.
    """
    exported: Any = dump["profiles"]
    if isinstance(exported, list):
        listed = cast("list[dict[str, Any]]", exported)
        return {entry["name"]: entry for entry in listed}

    return cast("dict[str, dict[str, Any]]", exported)


def blind_spots(language: str, dump: dict[str, Any], tools: list[str]) -> list[str]:
    """Ways this dump could agree with everything while resolving nothing."""
    problems: list[str] = []
    profiles = profiles_by_name(dump)

    if not profiles:
        problems.append(f"{language}: resolved no profiles at all")

    if not any(profile.get("allowed_tools") for profile in profiles.values()):
        problems.append(
            f"{language}: every profile resolved an empty tool list, so the"
            " comparisons below would pass on emptiness"
        )

    missing = sorted(set(tools) - set(dump["categories"]))
    if missing:
        problems.append(
            f"{language}: categorized {len(dump['categories'])} of {len(tools)}"
            f" catalog tools, first unnamed: {missing[0]}"
        )

    return problems


def diff_profiles(
    reference: str,
    other: str,
    left: dict[str, dict[str, Any]],
    right: dict[str, dict[str, Any]],
) -> list[str]:
    """Per-profile disagreements, one line per tool, scope, or flag."""
    problems: list[str] = []

    problems.extend(
        f"{name}: {reference} ships this profile and {other} does not"
        for name in sorted(set(left) - set(right))
    )
    problems.extend(
        f"{name}: {other} ships this profile and {reference} does not"
        for name in sorted(set(right) - set(left))
    )

    for name in sorted(set(left) & set(right)):
        problems.extend(_diff_profile(reference, other, name, left[name], right[name]))

    return problems


def _diff_profile(
    reference: str,
    other: str,
    name: str,
    left: dict[str, Any],
    right: dict[str, Any],
) -> list[str]:
    """One profile's disagreements, naming every tool and scope that differs."""
    problems: list[str] = []

    for field in _PROFILE_FIELDS:
        mine, theirs = left.get(field), right.get(field)
        if mine == theirs:
            continue

        if not isinstance(mine, list) or not isinstance(theirs, list):
            problems.append(
                f"{name}: {field} is {mine!r} in {reference} and {theirs!r} in {other}"
            )
            continue

        ours = set(cast("list[str]", mine))
        yours = set(cast("list[str]", theirs))

        problems.extend(
            f"{name}: {reference} allows {entry} in {field}, {other} does not"
            for entry in sorted(ours - yours)
        )
        problems.extend(
            f"{name}: {other} allows {entry} in {field}, {reference} does not"
            for entry in sorted(yours - ours)
        )

    return problems


def homeless_mutators(
    language: str, dump: dict[str, Any], capabilities: dict[str, str]
) -> list[str]:
    """Write and Destroy tools this language files under no category at all.

    Such a tool resolves in full-access and emergency, which elevate every
    category, and nowhere else: no category-scoped admin profile can reach it,
    however obviously it belongs to that admin's surface. The two languages
    agreeing on the omission is what hides it, so it is checked here rather
    than left to the wildcard to paper over.
    """
    return [
        f"{language}: {tool} is a {capabilities[tool]} tool in no category"
        for tool, cats in sorted(dump["categories"].items())
        if not cats and capabilities.get(tool) in {"Write", "Destroy"}
    ]


def diff_categories(
    reference: str,
    other: str,
    left: dict[str, list[str]],
    right: dict[str, list[str]],
) -> list[str]:
    """Tools the two languages file under different categories."""
    return [
        f"{tool}: {reference} files it under {sorted(left[tool]) or ['(none)']},"
        f" {other} under {sorted(right[tool]) or ['(none)']}"
        for tool in sorted(set(left) & set(right))
        if sorted(left[tool]) != sorted(right[tool])
    ]


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def main() -> int:
    """Report every cross-language disagreement; zero when the resolvers agree."""
    languages = registered_languages(_LANGUAGES)

    if unresolved := [name for name in languages if name not in _RESOLVERS]:
        _report(
            "languages registered in languages.txt with no resolver here:",
            unresolved,
            "add the language's resolver dump command to _RESOLVERS in this"
            " script, or its profiles go unchecked",
        )
        return 1

    capabilities = _surface.read_capabilities()
    fixture = catalog_fixture(capabilities)
    tools = [entry["name"] for entry in fixture]
    payload = json.dumps(fixture)

    dumps = {name: run_resolver(name, payload) for name in languages}

    blind = [
        problem
        for name in languages
        for problem in blind_spots(name, dumps[name], tools)
    ]
    if blind:
        _report(
            "resolver dumps that would pass this gate without resolving anything:",
            blind,
            "the dump is wrong or the catalog never reached it",
        )
        return 1

    homeless = [
        problem
        for name in languages
        for problem in homeless_mutators(name, dumps[name], capabilities)
    ]

    reference = languages[0]
    resolution: list[str] = []
    tables: list[str] = []

    for other in languages[1:]:
        resolution.extend(
            diff_profiles(
                reference,
                other,
                profiles_by_name(dumps[reference]),
                profiles_by_name(dumps[other]),
            )
        )
        tables.extend(
            diff_categories(
                reference,
                other,
                dumps[reference]["categories"],
                dumps[other]["categories"],
            )
        )

    if homeless:
        _report(
            "mutating tools that only the wildcard profiles can ever serve:",
            homeless,
            "give the tool a category in both tables, or state why no"
            " category-scoped profile should reach it",
        )
    if resolution:
        _report(
            "built-in profiles that serve different tools per language:",
            resolution,
            "one profile is one set of tools; move both resolvers or neither",
        )
    if tables:
        _report(
            "tools filed under different categories per language:",
            tables,
            "categories are declared per tool in the proto contract"
            " (tool_categories on the input message; both languages render"
            " the same generated table)",
        )
    if homeless or resolution or tables:
        return 1

    print(
        f"profile-resolution gate OK: {len(languages)} language(s) resolve"
        f" {len(profiles_by_name(dumps[reference]))} built-in profile(s)"
        f" identically over {len(tools)} tools"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
