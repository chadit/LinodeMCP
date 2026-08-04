#!/usr/bin/env python3
"""Ratchet gate: the generator owns its cohort, and the hand tree lets go of it.

docs/contracts/generated-tools.txt names the tools whose factory and handler the
emitters write from the proto contract: go/cmd/toolgen into go/internal/gentools,
scripts/toolgen_py.py into linodemcp.gentools. Both trees are gitignored and
rewritten whole by `make proto`, and each language's server reads them alongside
its hand-written tree.

That arrangement has two ways to go quietly wrong, and this gate closes both.

- A cohort tool the generator did not write in some language. The server would
  then serve the tool from one binary and not the other, or not at all, and the
  parity gate would report it as a tool missing from a language rather than as a
  generator that stopped covering its cohort.
- A hand-written factory left behind for a cohort tool. Both languages register
  by scanning, so a leftover would stage the tool twice: Go appends
  gentools.Factories() to its category lists, and Python collects create/handle
  pairs out of both modules. Whichever copy the scan reaches second wins, which
  makes a tool's behavior depend on iteration order.

"Still present" is read the way every other offline gate reads this surface: a
factory names the tool it builds as a string literal, so a tool named by a
non-test source file in a language's hand-written tool tree still has a factory
there. Go source is read with adjacent string concatenation joined first,
because one hand-written factory spells its name as two literals, and a scan
that missed it would read as a tool nothing serves.

The second half is the measure. Each language's remaining hand-written surface
is counted and held against docs/contracts/generated-tools-counts.txt with the
same direction docs/contracts/route-source-counts.txt has: a count above its
line fails (a tool went back to being hand-written, or new surface was written
by hand), and a count below it fails too, asking for the line to be lowered in
the same change, so the file always states the real remaining work. New surface
that is born generated adds nothing to the count, which is exactly what the
number should say.

Four ways this could pass while measuring nothing are checked first: a cohort
file with no tools in it, a language whose generated tree holds no tool at all,
a registered language with no tree arm here, and a manifest tool neither tree
names, which means the scan cannot see it rather than that nothing serves it.

Which trees are scanned comes from docs/contracts/languages.txt rather than a
path written here, and each language's trees are one entry in _TREES.

Stdlib only, so no venv is needed, but `make proto` must have run: the generated
trees it writes are what this reads. Run via `make generated-tools` (in
`make check`, and so in the pre-push hook and the CI gate on every branch).

Usage: verify_generated_tools.py [--update-baseline]
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Iterator

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"
_LANGUAGES = _CONTRACTS / "languages.txt"
_MANIFEST = _CONTRACTS / "tools-manifest.txt"
_COHORT = _CONTRACTS / "generated-tools.txt"
_COUNTS = _CONTRACTS / "generated-tools-counts.txt"

_COUNTS_HEADER = (
    "# Generated-tools counts: how many tools each registered language still\n"
    "# serves from a hand-written factory, rather than from the tree its\n"
    "# emitter writes out of the proto contract.\n"
    "# Owner: scripts/verify_generated_tools.py (`make generated-tools`,\n"
    "# inside `make check`).\n"
    "#\n"
    "# Direction: counts only FALL. A count above its line fails (a tool went\n"
    "# back to being hand-written, or new surface was written by hand instead\n"
    "# of declared). A count below its line fails too, asking for the line to\n"
    "# be lowered in the same change, so this file keeps stating the real\n"
    "# remaining work rather than an old high-water mark.\n"
    "#\n"
    "# New surface that is born generated does not move these numbers, which\n"
    "# is the point: it adds no debt, so there is nothing for the ratchet to\n"
    "# record.\n"
    "#\n"
    "# One line per language registered in languages.txt:\n"
    "#   <language> <hand-written tools>\n"
    "# A registered language with no line fails by name, and a line naming no\n"
    "# registered language fails by name, so this file and the registry cannot\n"
    "# drift apart.\n"
    "#\n"
    "# Deliberately not named *-baseline.txt: files under that glob hold one\n"
    "# accepted divergence per line, each carrying a dated tracking-issue\n"
    "# annotation. A line here is a measurement, not an acceptance, so there\n"
    "# is no promise for an annotation to record.\n"
    "#\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_generated_tools.py --update-baseline\n"
)

# Go spells one tool name as two literals joined by +, which the language allows
# and a literal scan would miss. Joining the pair back into one literal is the
# smallest reading that sees it, and it cannot join anything else: the pattern
# needs a closing quote, a +, and an opening quote.
_GO_CONCATENATION = re.compile(r'"\s*\+\s*"')


@dataclass(frozen=True)
class Trees:
    """Where one language keeps its hand-written and its generated tools.

    Both are directories of source, and both are read the same way: a factory
    names the tool it builds, so the tool name appears as a string literal in
    the file that declares it. joins is the language's own string
    concatenation, applied before the scan so a name spelled in pieces still
    reads as one name.
    """

    hand: str
    generated: str
    suffix: str
    joins: re.Pattern[str] | None


_TREES: dict[str, Trees] = {
    "go": Trees(
        hand="go/internal/tools",
        generated="go/internal/gentools",
        suffix=".go",
        joins=_GO_CONCATENATION,
    ),
    "python": Trees(
        hand="python/src/linodemcp/tools",
        generated="python/src/linodemcp/gentools",
        suffix=".py",
        joins=None,
    ),
}


@dataclass(frozen=True)
class Scan:
    """The tools each of one language's two trees names."""

    hand: frozenset[str]
    generated: frozenset[str]


def read_names(path: Path) -> list[str]:
    """One name per line, comments and blanks dropped, in file order."""
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]


def registered_languages(path: Path) -> list[str]:
    """The language names the registry declares, in file order."""
    languages = [line.split("\t", maxsplit=1)[0].strip() for line in read_names(path)]
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def is_test_path(relative: Path) -> bool:
    """Whether a path is test code rather than tool source."""
    if any(part in {"tests", "test", "testdata"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.stem.endswith("_test")


def source_files(root: Path, suffix: str) -> Iterator[Path]:
    """Non-test source under root, in path order.

    Test files are skipped because a tool name in a test is naming the tool to
    exercise it, not declaring a factory for it, and counting one would put a
    floor under the ratchet that no amount of work could lift.
    """
    for path in sorted(root.rglob(f"*{suffix}")):
        relative = path.relative_to(root)
        if any(part.startswith(".") for part in relative.parts):
            continue
        if is_test_path(relative):
            continue

        yield path


def named_tools(root: Path, tree: Trees, manifest: list[str]) -> frozenset[str]:
    """The manifest tools a tree names as string literals.

    The whole literal has to match, quotes included, because the modules that
    re-export a language's factories list them by function name:
    "create_linode_domain_get_tool" holds the tool's name as a substring and
    would otherwise read as the tool still being declared there. Either quote
    style counts, since a scan tied to one of them would answer differently
    after a formatter changed its mind.
    """
    source = "".join(
        path.read_text(encoding="utf-8") for path in source_files(root, tree.suffix)
    )
    if tree.joins is not None:
        source = tree.joins.sub("", source)

    return frozenset(
        tool for tool in manifest if f'"{tool}"' in source or f"'{tool}'" in source
    )


def scan(language: str, tree: Trees, manifest: list[str]) -> Scan:
    """Read both of one language's trees.

    A missing generated tree is an error rather than an empty result: it is
    written by `make proto` and gitignored, so its absence means the scan is
    about to report every generated tool as one nothing serves.
    """
    generated_root = _REPO_ROOT / tree.generated
    if not generated_root.is_dir():
        msg = (
            f"{language}: no generated tool tree at {tree.generated};"
            " `make proto` writes it"
        )
        raise SystemExit(msg)

    return Scan(
        hand=named_tools(_REPO_ROOT / tree.hand, tree, manifest),
        generated=named_tools(generated_root, tree, manifest),
    )


def missing_arms(languages: list[str]) -> list[str]:
    """Registered languages with no tree arm here."""
    return sorted(name for name in languages if name not in _TREES)


def empty_scans(measured: dict[str, Scan]) -> list[str]:
    """Languages whose generated tree named no tool at all."""
    return [
        f"{name}: {_TREES[name].generated} names no tool in the manifest"
        for name, found in sorted(measured.items())
        if not found.generated
    ]


def cohort_problems(cohort: list[str], measured: dict[str, Scan]) -> list[str]:
    """Cohort tools a language does not generate, and generated tools outside it."""
    wanted = set(cohort)
    problems: list[str] = []

    for name, found in sorted(measured.items()):
        problems.extend(
            f"{name} generates no factory for {tool}, which the cohort lists"
            for tool in sorted(wanted - found.generated)
        )
        problems.extend(
            f"{name} generates {tool}, which the cohort does not list"
            for tool in sorted(found.generated - wanted)
        )

    return problems


def leftovers(cohort: list[str], measured: dict[str, Scan]) -> list[str]:
    """Cohort tools a language's hand-written tree still names."""
    wanted = set(cohort)

    return [
        f"{name} still names {tool} in {_TREES[name].hand}"
        for name, found in sorted(measured.items())
        for tool in sorted(wanted & found.hand)
    ]


def unlocatable(
    cohort: list[str], manifest: list[str], measured: dict[str, Scan]
) -> list[str]:
    """Hand-written manifest tools neither of a language's trees names.

    The tool is served (the parity gate says so), which makes this a scan that
    cannot see its factory rather than a tool nobody implements. Left
    unreported it would quietly lower the hand-written count by one and read as
    progress.

    Cohort tools are left out because the cohort check already covers them, and
    it says the useful thing: a cohort tool no tree names is one the generator
    did not write, not one spelled in a way the scan cannot follow.
    """
    handwritten = [tool for tool in manifest if tool not in set(cohort)]

    return [
        f"{name} names {tool} in neither {_TREES[name].hand}"
        f" nor {_TREES[name].generated}"
        for name, found in sorted(measured.items())
        for tool in handwritten
        if tool not in found.hand and tool not in found.generated
    ]


def read_counts(path: Path) -> dict[str, int]:
    """Language-to-count map from the contract file."""
    if not path.exists():
        return {}

    counts: dict[str, int] = {}
    for line in read_names(path):
        fields = line.split()
        if len(fields) != 2 or not fields[1].isdigit():
            msg = f"unparsable {path.name} line {line!r}; want `<language> <count>`"
            raise SystemExit(msg)
        counts[fields[0]] = int(fields[1])

    return counts


def write_counts(path: Path, languages: list[str], measured: dict[str, Scan]) -> None:
    """Rewrite the contract from the measured counts, header included.

    Registry order rather than sorted, so the file reads in the same order as
    languages.txt, where the first language is the reference implementation.
    """
    lines = [f"{name} {len(measured[name].hand)}" for name in languages]
    path.write_text(_COUNTS_HEADER + "\n".join(lines) + "\n", encoding="utf-8")


def scope_problems(languages: list[str], recorded: dict[str, int]) -> list[str]:
    """Registered languages and recorded lines that do not answer each other."""
    names = set(languages)

    problems = [
        f"{name} is registered but has no line in {_COUNTS.name}"
        for name in sorted(names - set(recorded))
    ]
    problems.extend(
        f"{name} has a line in {_COUNTS.name} but is not a registered language"
        for name in sorted(set(recorded) - names)
    )

    return problems


def ratchet_problems(
    measured: dict[str, Scan], recorded: dict[str, int]
) -> tuple[list[str], list[str]]:
    """Languages that gained hand-written tools, and ones that lost them."""
    grew: list[str] = []
    shrank: list[str] = []

    for name, found in sorted(measured.items()):
        counted = len(found.hand)
        expected = recorded[name]
        if counted > expected:
            grew.append(
                f"{name}: {counted} hand-written tool(s), {expected} recorded"
                f" (+{counted - expected})"
            )
        elif counted < expected:
            shrank.append(
                f"{name}: {counted} hand-written tool(s), {expected} recorded"
                f" (-{expected - counted})"
            )

    return grew, shrank


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def _summary(cohort: list[str], measured: dict[str, Scan]) -> str:
    """The success line: what is generated, and what each language still holds."""
    per_language = ", ".join(
        f"{name} {len(found.hand)}" for name, found in sorted(measured.items())
    )

    return (
        f"generated-tools gate OK: {len(cohort)} tool(s) generated in every"
        f" language, {per_language} still hand-written"
    )


def _preflight(languages: list[str], cohort: list[str]) -> int:
    """Report the ways this could run and measure nothing. 0 means none of them."""
    arms = missing_arms(languages)
    if arms:
        _report(
            "no generated-tools scanner declared for registered language(s):",
            arms,
            "add its hand-written and generated tool trees to _TREES in"
            " scripts/verify_generated_tools.py",
        )
        return 1

    if not cohort:
        _report(
            "the generated-tools cohort is empty:",
            [f"{_COHORT.name} names no tool"],
            "every check here is scoped to the cohort, so an empty file would"
            " pass while proving nothing",
        )
        return 1

    return 0


def _coverage_problems(
    cohort: list[str], manifest: list[str], measured: dict[str, Scan]
) -> int:
    """Report a cohort a language does not fully generate, or a leftover factory."""
    empty = empty_scans(measured)
    if empty:
        _report(
            "generated tool tree(s) hold no tool:",
            empty,
            "`make proto` writes them from the cohort; an empty tree passes"
            " every cohort check by measuring nothing",
        )
        return 1

    problems = cohort_problems(cohort, measured)
    if problems:
        _report(
            "the cohort and the generated tree(s) disagree:",
            problems,
            f"every language generates every tool {_COHORT.name} lists;"
            " run `make proto` after changing it",
        )
        return 1

    hidden = unlocatable(cohort, manifest, measured)
    if hidden:
        _report(
            "hand-written manifest tool(s) no tree names:",
            hidden,
            "a factory names its tool as a string literal; a tool spelled some"
            " other way is invisible here and silently lowers the count",
        )
        return 1

    stale = leftovers(cohort, measured)
    if stale:
        _report(
            "hand-written factories for generated tool(s):",
            stale,
            "both languages register by scanning, so the leftover stages the"
            " tool twice; delete the hand-written factory and its handler",
        )
        return 1

    return 0


def _count_problems(languages: list[str], measured: dict[str, Scan]) -> int:
    """Hold each language's hand-written surface to the recorded count."""
    recorded = read_counts(_COUNTS)

    scope = scope_problems(languages, recorded)
    if scope:
        _report(
            "generated-tools counts and the language registry disagree:",
            scope,
            f"every registered language carries one line in {_COUNTS.name} and"
            " nothing else does; measure a new one with --update-baseline",
        )
        return 1

    grew, shrank = ratchet_problems(measured, recorded)
    if grew:
        _report(
            "hand-written tools have grown:",
            grew,
            "declare the new tool in the proto and add it to"
            f" {_COHORT.name} rather than raising the recorded count",
        )
    if shrank:
        _report(
            "hand-written tools are below the recorded count:",
            shrank,
            f"lower the line in docs/contracts/{_COUNTS.name} in this change"
            " (--update-baseline writes it), so the file keeps stating the"
            " real remaining work",
        )

    return 1 if grew or shrank else 0


def main(argv: list[str]) -> int:
    """Check every registered language's cohort coverage, then its count."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--update-baseline", action="store_true")
    args = parser.parse_args(argv)

    languages = registered_languages(_LANGUAGES)
    cohort = read_names(_COHORT)
    manifest = read_names(_MANIFEST)

    failed = _preflight(languages, cohort)
    if failed:
        return failed

    measured = {name: scan(name, _TREES[name], manifest) for name in languages}

    failed = _coverage_problems(cohort, manifest, measured)
    if failed:
        return failed

    if args.update_baseline:
        write_counts(_COUNTS, languages, measured)
        print(f"wrote {len(measured)} generated-tools count(s)", file=sys.stderr)
        return 0

    failed = _count_problems(languages, measured)
    if failed:
        return failed

    print(_summary(cohort, measured))

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
