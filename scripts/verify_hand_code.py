#!/usr/bin/env python3
"""Hard gate: no hand-coded tool code beside the generated tree.

Every tool's handler is generated from its *Input message, and every step a
handler runs is something the message declares. Nothing derives a hand-written
function name from a tool any more, so a function named after a tool is, by
construction, code the contract does not account for. This scans every
non-generated source tree a registered language owns and fails by name on each
one it finds.

The fix for a finding here is always a declaration on the tool's *Input message,
never a line in a contract file. There is no allowed set and no count to raise:
the population this gate measures is zero, and zero is a measurement that has to
keep being taken.

What is scanned: each registered language's working directory as
docs/contracts/languages.txt names it, less the trees .gitignore marks as
regenerated output, less test code, less dot directories. Reading the ignore
file rather than a path list is what keeps a new generated tree from being
scanned as hand-written, or a renamed one from going unscanned.

Which definitions are per-tool: the name with its case and underscores dropped
starts with a tool name flattened the same way, so Go's LinodeAuditExportAnswer
and Python's linode_audit_export_answer both reduce to the tool
linode_audit_export. A tool named by a single ordinary word (version, hello) is
NOT matched, and cannot be: `version` prefixes six legitimate definitions in
this tree today, from a tracing helper to a response builder, and an exact-name
rule still catches the tracing helper. That is a real hole in the scan, stated
here rather than papered over with an exemption list, which is the one thing
that could make this gate lie.

Argument checks are a population of their own: they are named after what they
check rather than after a tool, so this scan cannot see them.
docs/contracts/hand-validator-counts.txt holds them to zero in both languages,
and this gate reads that file and fails when any registered language's line is
not zero.

Four ways this could pass while measuring nothing are checked first: the
contract declaring no tools (the descriptors did not load), a registered
language with no scanner arm here, a language whose scan found no definition at
all (the tree moved or the pattern broke), and a language's line missing from
the validator counts.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
reaches through the venv when the plain interpreter cannot import them, so
`make proto` must have run. Run via `make hand-code` (in `make check`, and so
in the pre-push hook and the CI gate on every branch).

Usage: verify_hand_code.py
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import NamedTuple

from _toolroutes import declarations

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"
_LANGUAGES = _CONTRACTS / "languages.txt"
_VALIDATOR_COUNTS = _CONTRACTS / "hand-validator-counts.txt"


class Language(NamedTuple):
    """One registry line: the language's name and its working directory."""

    name: str
    workdir: str


@dataclass(frozen=True)
class Arm:
    """How one language spells a top-level function definition.

    The pattern captures the name, and matches only top-level definitions, so
    the scan reads definitions rather than call sites: a generated handler
    calling a shared driver would otherwise count as a second copy of it.
    """

    suffix: str
    definition: re.Pattern[str]


_ARMS: dict[str, Arm] = {
    "go": Arm(
        suffix=".go",
        definition=re.compile(r"^func ([A-Za-z_]\w*)\(", re.MULTILINE),
    ),
    "python": Arm(
        suffix=".py",
        definition=re.compile(r"^(?:async )?def ([A-Za-z_]\w*)\(", re.MULTILINE),
    ),
}


class Definition(NamedTuple):
    """One top-level function, with the file it is defined in."""

    path: str
    name: str


# The directory patterns .gitignore names: anchored repo-relative paths, then
# bare directory names matched at any depth.
Ignored = tuple[frozenset[str], frozenset[str]]


@dataclass(frozen=True)
class Declared:
    """The contract's side of the comparison, flattened for the scan.

    tools maps each flattened tool name to its spelled one.
    """

    tools: dict[str, str]


def read_lines(path: Path) -> list[str]:
    """One entry per line, comments and blanks dropped, in file order."""
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]


def registered_languages(path: Path) -> list[Language]:
    """The languages the registry declares, in file order."""
    languages: list[Language] = []
    for line in read_lines(path):
        fields = line.split("\t")
        if len(fields) < 2:
            msg = f"{path.name} line is not `<name>\t<working-dir>\t<dump>`: {line!r}"
            raise SystemExit(msg)
        languages.append(Language(fields[0].strip(), fields[1].strip()))

    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def flatten(name: str) -> str:
    """A name with its case and underscores dropped, for cross-language compare."""
    return name.replace("_", "").lower()


def declared_contract() -> Declared:
    """The tools the descriptors declare."""
    tools: dict[str, str] = {}
    for declaration in declarations():
        tool = declaration.route_tool or declaration.meta_tool
        if not tool:
            continue
        tools[flatten(tool)] = tool

    return Declared(tools=tools)


def ignored_directories(path: Path) -> Ignored:
    """The directory patterns .gitignore names: anchored paths, then bare names.

    Only directory patterns (ending in a slash) are read, since a regenerated
    tree is ignored as a whole. Negations and comments are skipped: nothing
    this scans is re-included, and a comment names nothing.
    """
    anchored: set[str] = set()
    names: set[str] = set()
    for line in read_lines(path):
        if line.startswith("!") or not line.endswith("/"):
            continue
        pattern = line.rstrip("/")
        if pattern.startswith("/"):
            anchored.add(pattern.lstrip("/"))
        else:
            names.add(pattern)

    return frozenset(anchored), frozenset(names)


def is_test_path(relative: Path) -> bool:
    """Whether a path is test code rather than source."""
    if any(part in {"tests", "test", "testdata"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.stem.endswith("_test")


def is_ignored(relative: Path, ignored: Ignored) -> bool:
    """Whether a repo-relative path sits under a directory .gitignore names."""
    anchored, names = ignored
    posix = relative.as_posix()
    if any(posix == entry or posix.startswith(entry + "/") for entry in anchored):
        return True

    return any(part in names for part in relative.parts[:-1])


def source_files(root: Path, workdir: str, arm: Arm, ignored: Ignored) -> list[Path]:
    """One language's hand-written source, in path order.

    A working directory that does not exist is an error rather than an empty
    scan, since a scan of nothing would pass every check below.
    """
    tree = root / workdir
    if not tree.is_dir():
        msg = f"{workdir} is not a directory; languages.txt names it"
        raise SystemExit(msg)

    found: list[Path] = []
    for path in sorted(tree.rglob("*" + arm.suffix)):
        relative = path.relative_to(root)
        if any(part.startswith(".") for part in relative.parts):
            continue
        if is_test_path(relative.relative_to(workdir)) or is_ignored(relative, ignored):
            continue
        found.append(path)

    return found


def defined(root: Path, workdir: str, arm: Arm, ignored: Ignored) -> list[Definition]:
    """Every top-level definition one language's hand-written trees make."""
    found: list[Definition] = []
    for path in source_files(root, workdir, arm, ignored):
        relative = path.relative_to(root).as_posix()
        found.extend(
            Definition(relative, name)
            for name in arm.definition.findall(path.read_text(encoding="utf-8"))
        )

    return found


def classify(name: str, declared: Declared) -> str | None:
    """The tool a definition is named after, if any.

    A compound tool name owns every definition starting with it. A single-word
    tool owns nothing: see the module docstring for the six legitimate
    definitions that rules it out.
    """
    flat = flatten(name)
    for flat_tool in sorted(declared.tools, key=len, reverse=True):
        tool = declared.tools[flat_tool]
        if "_" in tool and flat.startswith(flat_tool):
            return tool

    return None


def attribute(
    language: str, definitions: list[Definition], declared: Declared
) -> list[str]:
    """Report one language's definitions that are named after a tool.

    Takes the definitions and the contract rather than reading them, so a test
    can hold a fabricated tree against a fabricated contract and see what the
    comparison does to each disagreement.
    """
    return [
        f"{language}: {definition.path} defines {definition.name}, which is"
        f" hand-coded tool code for {tool}: the generated tree writes every step"
        " its *Input message declares, so this one is accounted for nowhere"
        for definition in definitions
        if (tool := classify(definition.name, declared)) is not None
    ]


def validator_problems(path: Path, languages: list[Language]) -> list[str]:
    """Hold the argument-check population to the zero its own contract file states."""
    recorded: dict[str, str] = {}
    for line in read_lines(path):
        fields = line.split()
        if len(fields) == 2:
            recorded[fields[0]] = fields[1]

    problems: list[str] = []
    for language in languages:
        count = recorded.get(language.name)
        if count is None:
            problems.append(f"{language.name}: {path.name} records no count for it")
        elif count != "0":
            problems.append(
                f"{language.name}: {path.name} reads {count}, so an argument check"
                " is hand-written; it belongs on the *Input message"
            )

    return problems


def measure(root: Path, languages: list[Language], declared: Declared) -> list[str]:
    """Scan each language's hand-written trees, reporting what cannot be scanned."""
    problems: list[str] = []
    if not declared.tools:
        problems.append(
            "the contract declares no tools, so there is nothing to measure"
            " against: `make proto` has not run"
        )

    ignored = ignored_directories(root / ".gitignore")

    for language in languages:
        arm = _ARMS.get(language.name)
        if arm is None:
            problems.append(
                f"{language.name} is registered in languages.txt and has no"
                " scanner arm here, so its hand-written trees go unscanned"
            )
            continue

        definitions = defined(root, language.workdir, arm, ignored)
        if not definitions:
            problems.append(
                f"{language.name}: no definition found under {language.workdir},"
                " so the scan sees nothing rather than a tree with nothing to find"
            )
            continue

        problems.extend(attribute(language.name, definitions, declared))

    return problems


def main() -> int:
    argparse.ArgumentParser(description=__doc__).parse_args()

    languages = registered_languages(_LANGUAGES)
    problems = measure(_REPO_ROOT, languages, declared_contract())
    problems.extend(validator_problems(_VALIDATOR_COUNTS, languages))

    if problems:
        for problem in problems:
            sys.stderr.write(f"hand-code: {problem}\n")
        return 1

    sys.stdout.write(
        f"hand-code: no hand-coded tool code across {len(languages)} languages\n"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
