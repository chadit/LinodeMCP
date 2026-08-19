#!/usr/bin/env python3
"""Ratchet gate: the contract owns every argument check a rule can carry.

A tool's argument check lives in one of two places. Either the tool's *Input
message declares buf.validate rules, which both languages evaluate through one
shared reader (go/internal/toolvalidate, linodemcp.tools.constraints), or the
tool declares a `validate` hook and each language writes the check out by hand.
The second is the debt: two copies of one rule, in two languages, with nothing
holding them to the same sentence. This counts what is left of it.

Direction: counts only FALL, the same as the generated-tools ratchet. A count
above its line fails, which is a check that went back to being hand-written or
new surface that wrote one instead of declaring it. A count below its line fails
too, asking for the line to be lowered in the same change, so the file keeps
stating the real remaining work rather than an old high-water mark.

The counts now read zero in both languages: every argument check the surface
makes is the contract's. That is what makes the guards below load-bearing rather
than decorative, since at zero the difference between "nothing is hand-written"
and "the scan sees nothing" is exactly one broken import.

Four ways this could pass while measuring nothing are checked first:

- The contract declares no tools at all, which means the descriptors did not
  load and every comparison below would be against an empty set.
- A registered language has no arm here, so its hand-written checks go
  uncounted.
- A language's tree defines none of the declared functions, which means the scan
  stopped seeing them rather than that the checks are gone.
- A tree defines a validate function no tool declares, which is a check nothing
  calls and a number that would never fall.

The declared set comes from the descriptors and the implemented set from each
language's hook tree, so the two are read from different places and a scan that
broke cannot agree with the contract by accident. Which trees are scanned comes
from docs/contracts/languages.txt rather than a path written here.

Function names are matched with their case and underscores removed, so the
comparison does not have to know either language's naming rules: Go's
LinodeDomainRecordGetValidate and Python's linode_domain_record_get_validate
both reduce to the tool linode_domain_record_get.

A handful of names match those spellings and are not per-tool checks at all:
the shared page reader, the shared timestamp reader, and the body two contract
members read a path segment through. Those are support vocabulary, the same
standing `make_route_request` has, and each language's Tree names them in a
`plumbing` set with the reason written beside it. The set is deliberately small
and deliberately annotated, because it is the one place this gate can be made
to lie; scripts/verify_route_source.py keeps its own for the same reason.

Which handler-tree names count as a hand-written check is a naming convention
per language, and the two trees do not spell it the same way. Both trees turn
one argument into either a value or a refusal sentence; Go writes that as
`validateX`, `parseX`, or `XFromTool`, Python as `_validate_x`, `_parse_x`,
`_x_error`, or `_x_argument`. Go's third spelling is the request-builder form,
which reads one or more arguments and returns the value beside a refusal
sentence rather than an error; every one of them ends its result list with that
string. Python's fourth is the argument-reader form, which answers the value or
None and leaves its caller to word the refusal.

Reading only one spelling per tree is how this gate came to be blind to
Python's parse-and-validate helpers, then to Go's request builders, and then to
Python's argument readers, which made migrating a tool that had one read as
growth: the hook it gained counted, the reader it lost did not.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
reaches through the venv when the plain interpreter cannot import them, so
`make proto` must have run. Run via `make hand-validators` (in `make check`, and
so in the pre-push hook and the CI gate on every branch).

Usage: verify_hand_validators.py [--update-baseline]
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

from _toolroutes import declarations

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"
_LANGUAGES = _CONTRACTS / "languages.txt"
_COUNTS = _CONTRACTS / "hand-validator-counts.txt"

_HOOK_KIND = "validate"

_COUNTS_HEADER = (
    "# Hand-validator counts: how many tools each registered language still\n"
    "# checks the arguments of in hand-written code, rather than from the\n"
    "# buf.validate rules its *Input message declares.\n"
    "# Owner: scripts/verify_hand_validators.py (`make hand-validators`,\n"
    "# inside `make check`).\n"
    "#\n"
    "# Direction: counts only FALL. A count above its line fails (a check went\n"
    "# back to being hand-written, or new surface wrote one instead of\n"
    "# declaring it). A count below its line fails too, asking for the line to\n"
    "# be lowered in the same change, so this file keeps stating the real\n"
    "# remaining work rather than an old high-water mark.\n"
    "#\n"
    "# The number is two populations added: the `validate` hooks a language's\n"
    "# hook tree defines, plus the argument checks its handler tree still\n"
    "# writes out, counted once per definition site. A rule the contract can\n"
    "# carry belongs on the message; what is left here is what CEL cannot say.\n"
    "#\n"
    "# Both populations are empty in both languages. Every argument check the\n"
    "# surface makes is now the contract's: a buf.validate rule, a declared\n"
    "# argument_reader, or an object_walk over an open map or object list. The\n"
    "# file stays because zero is a measurement that has to keep being taken.\n"
    "#\n"
    "# One line per language registered in languages.txt:\n"
    "#   <language> <hand-written validators>\n"
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
    "#   python3 scripts/verify_hand_validators.py --update-baseline\n"
)


@dataclass(frozen=True)
class Tree:
    """Where one language keeps its hand-written hooks, and how it names them.

    definition matches a top-level function definition and captures its name,
    so the scan reads declarations rather than call sites: a generated handler
    naming the same function would otherwise count as a second copy of it.

    plumbing names the functions whose spelling the scan matches but whose work
    is not a per-tool argument check. See the docstring above for why each entry
    is there; a name is added only with that reason written down, because the
    set is the one place this gate can be made to lie.
    """

    root: str
    suffix: str
    definition: re.Pattern[str]
    plumbing: frozenset[str] = frozenset()


_TREES: dict[str, Tree] = {
    "go": Tree(
        root="go/internal/toolhooks",
        suffix=".go",
        definition=re.compile(r"^func (Linode\w*Validate)\(", re.MULTILINE),
    ),
    "python": Tree(
        root="python/src/linodemcp",
        suffix=".py",
        definition=re.compile(r"^def (linode_\w*_validate)\(", re.MULTILINE),
    ),
}

# Validators still living inside hand-written handlers count too, or moving one
# into a hook would read as growth and block the migration it is part of. Each
# pattern names every spelling its tree gives an argument check, per the module
# docstring; a helper that only decodes a response body is not one of these.
_HAND_TREES: dict[str, Tree] = {
    "go": Tree(
        root="go/internal/tools",
        suffix=".go",
        definition=re.compile(
            r"^func ((?:validate|parse)[A-Z]\w*|\w+FromTool)\(", re.MULTILINE
        ),
        plumbing=frozenset(
            {
                # The page reader every new paginated family shares. It checks
                # page and page_size, which no tool declares and every tool
                # takes, so it is one vocabulary rather than a tool's own check.
                "standardPaginationFromTool",
                # The timestamp reader the audit tools share. Python's twin is
                # public and so already outside its scan, which is the whole
                # asymmetry: one shared reader, counted in one language.
                "parseOptionalTime",
            }
        ),
    ),
    "python": Tree(
        root="python/src/linodemcp/tools",
        suffix=".py",
        definition=re.compile(
            r"^def (_(?:validate|parse)_\w+|_\w+_error|_\w+_argument)\(", re.MULTILINE
        ),
        plumbing=frozenset(
            {
                # The body two contract members read a path segment through.
                # Go's twin, segmentArgument, is outside its scan already, so
                # counting this one would count one shared reader in one
                # language.
                "_segment_argument",
                # One step of the shared object_walk reader: it applies a
                # declaration to one named argument and belongs to no tool. Go's
                # twin, toolwalk.walkArgument, is outside its scan already, so
                # counting this one would count one shared reader in one
                # language.
                "_walk_argument",
            }
        ),
    ),
}


def read_lines(path: Path) -> list[str]:
    """One entry per line, comments and blanks dropped, in file order."""
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]


def registered_languages(path: Path) -> list[str]:
    """The language names the registry declares, in file order."""
    languages = [line.split("\t", maxsplit=1)[0].strip() for line in read_lines(path)]
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def declared_validators() -> set[str]:
    """The tools whose contract says their argument check is hand-written."""
    return {
        declaration.route_tool or declaration.meta_tool
        for declaration in declarations()
        if _HOOK_KIND in declaration.hooks
    }


def flatten(name: str) -> str:
    """A name with its case and underscores dropped, for cross-language compare."""
    return name.replace("_", "").lower()


def is_test_path(relative: Path) -> bool:
    """Whether a path is test code rather than hook source."""
    if any(part in {"tests", "test", "testdata"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.stem.endswith("_test")


def implemented(tree: Tree) -> set[str]:
    """The validate functions one language's hook tree defines, flattened."""
    root = _REPO_ROOT / tree.root
    found: set[str] = set()

    for path in sorted(root.rglob("*" + tree.suffix)):
        if is_test_path(path.relative_to(root)):
            continue

        for name in tree.definition.findall(path.read_text(encoding="utf-8")):
            found.add(flatten(name.removesuffix("Validate").removesuffix("_validate")))

    return found


def hand_written(tree: Tree) -> list[str]:
    """Every argument check one language's handler tree writes out, per site.

    Counted per definition rather than per distinct name: four handler files
    each spelling out their own _parse_instance_id are four copies of one rule,
    and folding them onto one name would hold the count flat while three of
    them go.
    """
    root = _REPO_ROOT / tree.root
    found: list[str] = []

    for path in sorted(root.rglob("*" + tree.suffix)):
        if is_test_path(path.relative_to(root)):
            continue

        found.extend(
            f"{path.relative_to(_REPO_ROOT)}:{name}"
            for name in tree.definition.findall(path.read_text(encoding="utf-8"))
            if name not in tree.plumbing
        )

    return found


def read_counts(path: Path) -> dict[str, int]:
    """The recorded count per language."""
    counts: dict[str, int] = {}
    for line in read_lines(path):
        language, _, number = line.partition(" ")
        counts[language.strip()] = int(number.strip())

    return counts


def write_counts(path: Path, counts: dict[str, int]) -> None:
    """Rewrite the contract with the counts measured now."""
    body = "".join(f"{language} {count}\n" for language, count in counts.items())
    path.write_text(_COUNTS_HEADER + body, encoding="utf-8")


def measure(languages: list[str]) -> tuple[dict[str, int], list[str]]:
    """Count each language's hand-written checks, reporting what cannot be counted."""
    declared = declared_validators()
    problems: list[str] = []

    if not declarations():
        problems.append(
            "the contract declares no tools, so there is nothing to measure"
            " against: `make proto` has not run"
        )

    expected = {flatten(tool) for tool in declared}
    counts: dict[str, int] = {}

    for language in languages:
        tree = _TREES.get(language)
        if tree is None:
            problems.append(
                f"{language} is registered in languages.txt and has no hook tree"
                " here, so its hand-written checks go uncounted"
            )
            continue

        found = implemented(tree)
        hand = _HAND_TREES.get(language)
        counts[language] = len(found) + (len(hand_written(hand)) if hand else 0)

        problems.extend(
            f"{language}: declares a validate hook for {missing}, and its hook"
            " tree defines no function for it"
            for missing in sorted(expected - found)
        )
        problems.extend(
            f"{language}: defines a validate function for {extra}, which no tool"
            " declares"
            for extra in sorted(found - expected)
        )

    return counts, problems


def compare(counts: dict[str, int], recorded: dict[str, int]) -> list[str]:
    """Hold each measured count against its recorded line."""
    problems: list[str] = []

    problems.extend(
        f"{language}: {_COUNTS.name} names it and languages.txt does not register it"
        for language in sorted(set(recorded) - set(counts))
    )

    for language, count in counts.items():
        if language not in recorded:
            problems.append(f"{language}: {_COUNTS.name} records no count for it")
            continue

        if count > recorded[language]:
            problems.append(
                f"{language}: {count} hand-written validators, up from"
                f" {recorded[language]}. A check a rule can carry belongs on the"
                " tool's *Input message"
            )
        elif count < recorded[language]:
            problems.append(
                f"{language}: {count} hand-written validators, down from"
                f" {recorded[language]}. Lower the line in this change:"
                f" python3 scripts/{Path(__file__).name} --update-baseline"
            )

    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--update-baseline",
        action="store_true",
        help="rewrite the counts contract from what is measured now",
    )
    arguments = parser.parse_args()

    languages = registered_languages(_LANGUAGES)
    counts, problems = measure(languages)

    if problems:
        for problem in problems:
            sys.stderr.write(f"hand-validators: {problem}\n")
        return 1

    if arguments.update_baseline:
        write_counts(_COUNTS, counts)
        sys.stdout.write(f"hand-validators: wrote {_COUNTS}\n")
        return 0

    problems = compare(counts, read_counts(_COUNTS))
    if problems:
        for problem in problems:
            sys.stderr.write(f"hand-validators: {problem}\n")
        return 1

    total = ", ".join(f"{language} {count}" for language, count in counts.items())
    sys.stdout.write(f"hand-validators: {total}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
