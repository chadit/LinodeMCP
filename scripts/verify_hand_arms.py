#!/usr/bin/env python3
"""Hard gate: no hand-written local-operation arm beside the generated tree.

An arm is the four steps between a tool handler and a subsystem: read the
declared inputs, call the subsystem, map whatever it reported onto the declared
conditions, project the answer. Every one of them is written from the
operation's own declaration now, in every registered language, so a function
named after a LocalCall member in hand-written source is code the contract does
not account for.

This is the gate that took over from `docs/contracts/handwritten-arms.txt`. That
file listed the operations each engine still wrote by hand; it emptied when the
last arm was generated, and a file whose only legal content is nothing exempts
nothing. There is no allowed set and no count to raise: the population this gate
measures is zero, and zero is a measurement that has to keep being taken.

The fix for a finding here is always a subsystem method behind the generated
interface, never a line in a contract file.

What is scanned: each registered language's working directory as
docs/contracts/languages.txt names it, less the trees .gitignore marks as
regenerated output, less test code, less dot directories. Reading the ignore
file rather than a path list is what keeps a new generated tree from being
scanned as hand-written, or a renamed one from going unscanned.

Which definitions are arms: the name opens with the arm spelling that language
gives one operation, so Go looks for RunCatalogCanRun and Python for
run_catalog_can_run. Each language's own spelling rather than one flattened
form, because a flattened rule reads the CLI's unexported runAuditRecent as an
arm for LOCAL_CALL_AUDIT_RECENT, which it is not: an arm is exported in Go and
module-level in Python, and its case says so. Matching the opening rather than
the whole name is what catches a two-sided operation's two arms
(RunDraftToolsAdd, RunDraftToolsRemove) without this gate having to know the
direction vocabulary.

Three ways this could pass while measuring nothing are checked first: the
contract declaring no operations (the descriptors did not load), a registered
language with no scanner arm here, and a language whose scan found no definition
at all (the tree moved or the pattern broke).

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
reaches through the venv when the plain interpreter cannot import them, so
`make proto` must have run. Run via `make hand-arms` (in `make check`, and so in
the pre-push hook and the CI gate on every branch).

Usage: verify_hand_arms.py
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING, NamedTuple

from _toolroutes import local_operations

if TYPE_CHECKING:
    from collections.abc import Callable

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"

# What every member of the operation vocabulary opens with. The emitter's, and
# spelled here because this gate reads names rather than calling it.
_MEMBER_PREFIX = "LOCAL_CALL_"


class Language(NamedTuple):
    """One registry line: the language's name and its working directory."""

    name: str
    workdir: str


def go_arm_name(stem: str) -> str:
    """Go's arm spelling for one operation stem: Run plus the stem run together."""
    return "Run" + "".join(word.capitalize() for word in stem.split("_"))


def python_arm_name(stem: str) -> str:
    """Python's arm spelling for one operation stem, underscores kept."""
    return "run_" + stem


@dataclass(frozen=True)
class Arm:
    """How one language spells a top-level function definition and an arm.

    The pattern captures the name and matches only top-level definitions, so
    the scan reads definitions rather than call sites: a generated handler
    calling its own arm would otherwise count as a hand-written one.
    """

    suffix: str
    definition: re.Pattern[str]
    spell: Callable[[str], str]


_ARMS: dict[str, Arm] = {
    "go": Arm(
        suffix=".go",
        definition=re.compile(r"^func ([A-Za-z_]\w*)\(", re.MULTILINE),
        spell=go_arm_name,
    ),
    "python": Arm(
        suffix=".py",
        definition=re.compile(r"^(?:async )?def ([A-Za-z_]\w*)\(", re.MULTILINE),
        spell=python_arm_name,
    ),
}


class Definition(NamedTuple):
    """One top-level function, with the file it is defined in."""

    path: str
    name: str


# The directory patterns .gitignore names: anchored repo-relative paths, then
# bare directory names matched at any depth.
Ignored = tuple[frozenset[str], frozenset[str]]


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


def registry_path() -> Path:
    """Where the language registry lives, so a test names it once."""
    return _LANGUAGES


def arm_for(language: str) -> Arm:
    """How one registered language spells a definition and an arm."""
    return _ARMS[language]


def declared_stems() -> dict[str, str]:
    """Each operation's own stem mapped to the member it comes from."""
    return {
        member.removeprefix(_MEMBER_PREFIX).lower(): member
        for member in local_operations()
    }


def declared_arms(arm: Arm, stems: dict[str, str]) -> dict[str, str]:
    """One language's arm spelling per operation, mapped to the declared member."""
    return {arm.spell(stem): member for stem, member in stems.items()}


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


def classify(name: str, arms: dict[str, str]) -> str | None:
    """The operation a definition writes an arm for, if any.

    The longest opening wins, so an operation whose stem opens another's is
    reported under the one it really names.
    """
    for spelled in sorted(arms, key=len, reverse=True):
        if name.startswith(spelled):
            return arms[spelled]

    return None


def attribute(
    language: str, definitions: list[Definition], arms: dict[str, str]
) -> list[str]:
    """Report one language's definitions that write an arm.

    Takes the definitions and the vocabulary rather than reading them, so a
    test can hold a fabricated tree against a fabricated contract and see what
    the comparison does to each disagreement.
    """
    return [
        f"{language}: {definition.path} defines {definition.name}, which is a"
        f" hand-written arm for {member}: the generated tree writes the four"
        " steps every operation declares, so this one is accounted for nowhere"
        for definition in definitions
        if (member := classify(definition.name, arms)) is not None
    ]


def measure(root: Path, languages: list[Language], stems: dict[str, str]) -> list[str]:
    """Scan each language's hand-written trees, reporting what cannot be scanned."""
    problems: list[str] = []
    if not stems:
        problems.append(
            "the contract declares no local operations, so there is nothing to"
            " measure against: `make proto` has not run"
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

        problems.extend(
            attribute(language.name, definitions, declared_arms(arm, stems))
        )

    return problems


def main() -> int:
    """Run the scan, reporting every hand-written arm by name."""
    argparse.ArgumentParser(description=__doc__).parse_args()

    languages = registered_languages(_LANGUAGES)
    stems = declared_stems()
    problems = measure(_REPO_ROOT, languages, stems)

    if problems:
        for problem in problems:
            sys.stderr.write(f"hand-arms: {problem}\n")
        return 1

    sys.stdout.write(
        f"hand-arms: no hand-written arm across {len(languages)} languages,"
        f" {len(stems)} operations generated\n"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
