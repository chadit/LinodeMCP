#!/usr/bin/env python3
"""Ratchet gate: how much of a tool's handler each language still writes by hand.

A tool's handler is generated from its *Input message, except for the steps the
descriptors cannot express. Those the tool names by KIND in its `tool_hooks`
option, and each language writes the body out. That is the debt this counts: one
step of one tool, spelled twice, with nothing holding the two copies to the same
answer.

Direction: counts only FALL, the same as the hand-validator ratchet next to it.
A count above its line fails, which is a step that went back to being
hand-written or new surface that wrote one instead of declaring it. A count
below its line fails too, asking for the line to be lowered in the same change,
so the file keeps stating the real remaining work rather than an old high-water
mark.

The `validate` kind is not counted here. It is the whole subject of
docs/contracts/hand-validator-counts.txt, which reads zero in both languages and
fails the same way in both directions, so counting it twice would put one
population under two gates that could disagree. Any other kind a tool declares
must have a line here or fail by name.

Four ways this could pass while measuring nothing are checked first:

- The contract declares no tools at all, which means the descriptors did not
  load and every comparison below would be against an empty set.
- The contract declares no hooks at all, which means the option went rather than
  the bodies.
- A registered language has no tree here, or its tree is not where this says it
  is, so its hand-written bodies go uncounted.
- A tree defines a hook function no tool declares, which is a body nothing calls
  and a number that would never fall.

The declared set comes from the descriptors and the implemented set from each
language's tree, so the two are read from different places and a scan that broke
cannot agree with the contract by accident. Which trees are scanned comes from
docs/contracts/languages.txt rather than a path written here.

Function names are matched with their case and underscores removed, which is
what lets one rule read both languages: the tool linode_volume_delete and the
kind fetch_state give Go's LinodeVolumeDeleteFetchState and Python's
linode_volume_delete_fetch_state, and all three reduce to
linodevolumedeletefetchstate. A definition is attributed to the tool and kind
whose flattened pair it equals, so the kind a body is counted under is the kind
its tool declared rather than something read off its spelling.

A definition matching no declared pair is reported by name. Go's FunctionName is
the one that is not a hook, and it needs no exemption: it carries no tool and no
kind, so it matches nothing and the report names it. Its `plumbing` entry says
so, keeping the exemption a written decision rather than a silent one, the same
standing scripts/verify_hand_validators.py gives its shared readers.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
reaches through the venv when the plain interpreter cannot import them, so
`make proto` must have run. Run via `make hook-bodies` (in `make check`, and so
in the pre-push hook and the CI gate on every branch).

Usage: verify_hook_bodies.py [--update-baseline]
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

from _toolroutes import declarations

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"
_LANGUAGES = _CONTRACTS / "languages.txt"
_COUNTS = _CONTRACTS / "hook-body-counts.txt"

# Counted by docs/contracts/hand-validator-counts.txt, which holds it to zero in
# both languages from both directions. The reason rides with the name so the one
# uncounted kind stays a written decision.
_OWNED_ELSEWHERE = {
    "validate": (
        "counted by hand-validator-counts.txt, which holds it to zero in both languages"
    ),
}

_COUNTS_HEADER = (
    "# Hook-body counts: how many of each tool-handler step each registered\n"
    "# language still writes by hand, per kind, rather than deriving it from\n"
    "# the tool's *Input message.\n"
    "# Owner: scripts/verify_hook_bodies.py (`make hook-bodies`, inside\n"
    "# `make check`).\n"
    "#\n"
    "# Direction: counts only FALL. A count above its line fails (a step went\n"
    "# back to being hand-written, or new surface wrote one instead of\n"
    "# declaring it). A count below its line fails too, asking for the line to\n"
    "# be lowered in the same change, so this file keeps stating the real\n"
    "# remaining work rather than an old high-water mark.\n"
    "#\n"
    "# One line per registered language per declared kind:\n"
    "#   <language> <kind> <hand-written bodies>\n"
    "# A registered language with no line fails by name, a declared kind with\n"
    "# no line fails by name, and a line naming neither a registered language\n"
    "# nor a declared kind fails by name, so this file, the registry and the\n"
    "# contract cannot drift apart. A kind whose cohort is fully retired stops\n"
    "# being declared, so its lines come out in that same change.\n"
    "#\n"
    "# `validate` has no line here on purpose: it is the whole subject of\n"
    "# hand-validator-counts.txt, at zero in both languages, and one population\n"
    "# under two gates is two numbers that can disagree.\n"
    "#\n"
    "# WHY THESE REMAIN. Each kind below was measured against a declaration\n"
    "# that would have replaced it, and kept for a reason the contract cannot\n"
    "# carry. Per-tool detail is in the B2, B3 and B4 correction sections of\n"
    "# the hook-body inventory written during the migration.\n"
    "#\n"
    "#   normalize   What is left after named transforms took the rest: the\n"
    "#               rewrites no transform spells, such as folding one\n"
    "#               argument spelling into another.\n"
    "#\n"
    "#   fetch_state What is left after the projected state landed: the reads\n"
    "#               that are not one GET the contract can name. A list scan\n"
    "#               with a match predicate (the VLAN delete), a two-call\n"
    "#               composite over a chosen field subset (the instance\n"
    "#               resize), and a page whose envelope, results total\n"
    "#               included, is itself the state (the tag delete). Each\n"
    "#               needs something a route name cannot say.\n"
    "#\n"
    "#               The rows blocked by shape are gone: a declared fetch\n"
    "#               reports the body the API sent keyed to the read's own\n"
    "#               message, so a null it sent survives at any depth and a\n"
    "#               key it never sent is not invented. So are the rows\n"
    "#               blocked by spelling: a state route fills the read's\n"
    "#               slots from the removal's path arguments by name, and\n"
    "#               the three instance action routes that spell one id\n"
    "#               linode_id where their read spells it instance_id say so\n"
    "#               in the declaration instead of writing the read out.\n"
    "#\n"
    "#   dependency_walk\n"
    "#               A walk says what else a removal touches, which no\n"
    "#               descriptor carries: the members a group detaches, the\n"
    "#               Linodes a subnet drops, the pools a cluster destroys.\n"
    "#               Reading the state is no longer what keeps them here. A\n"
    "#               walk paired with a declared fetch takes the projected\n"
    "#               state as its own type, so a mismatched pair fails to\n"
    "#               compile rather than reporting an empty walk.\n"
    "#\n"
    "#   preview     Both halves are declarable now. The sentence half was\n"
    "#               declared first; the fetch half followed, since a preview\n"
    "#               reads its resource through the same state route a removal\n"
    "#               does, with the read's slots filled from the tool's own\n"
    "#               path arguments.\n"
    "#\n"
    "#               Every declarable fetch has landed, so what remains is one\n"
    "#               of four things. A call with nothing to read: a create, or\n"
    "#               a preview whose reported body is rebuilt rather than\n"
    "#               echoed, such as the redacted security answers and the\n"
    "#               upload that sizes a local file. A sentence computed from\n"
    "#               the fetched state rather than interpolated from an\n"
    "#               argument, which the hook still owns because the declared\n"
    "#               preview hands its prose no state to read. A wording the\n"
    "#               rule cannot say: a bool or a float, which the two\n"
    "#               languages do not spell the same way; a constant\n"
    "#               substituted for an absent argument, which is a default\n"
    "#               rather than an alternative wording; and a line per element\n"
    "#               of a repeated argument, or one joining it, where a\n"
    "#               placeholder reports a single value. Two shapes have left\n"
    "#               this list: a whole line that vanishes, since a declared\n"
    "#               line none of whose wordings can be filled is now dropped\n"
    "#               rather than reported with a gap in it, and a bool, since a\n"
    "#               flag can select a wording without being spelled into one.\n"
    "#               And a read that is not one GET the contract can name,\n"
    "#               which is the fetch_state wall above reached from this\n"
    "#               kind: a page of a collection. The object ACL has left that\n"
    "#               one too, since a state route now fills a read's query\n"
    "#               parameters as well as its path slots, and refuses to build\n"
    "#               when a required one has nothing to fill it.\n"
    "#\n"
    "#   execute     Each owns a transport that is not the JSON request the\n"
    "#               emitter derives: multipart framing from a local file, a\n"
    "#               raw PNG body, a presigned transfer. The presign pair looks\n"
    "#               like one flow and is not; the two diverge at the guard,\n"
    "#               the transfer fields and the answer shape, so a shared\n"
    "#               option would need a verb, a URL field, a local-path\n"
    "#               argument, a guard kind and a per-direction constant set to\n"
    "#               cover two tools.\n"
    "#\n"
    "#   answer      Each forwards to local state no data option describes: a\n"
    "#               log on disk, the live catalog, an in-memory registry.\n"
    "#               Dispatching straight to the implementation instead would\n"
    "#               delete the forwarders and no logic, and it needs a module\n"
    "#               name Python cannot derive, since eight implementations\n"
    "#               share modules organized by concern. It would also make\n"
    "#               this one kind resolve outside the hooks module, which the\n"
    "#               two-way hook-completeness tests in both languages and the\n"
    "#               language-onboarding rule all state as uniform.\n"
    "#\n"
    "# Deliberately not named *-baseline.txt: files under that glob hold one\n"
    "# accepted divergence per line, each carrying a dated tracking-issue\n"
    "# annotation. A line here is a measurement, not an acceptance, so there\n"
    "# is no promise for an annotation to record.\n"
    "#\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_hook_bodies.py --update-baseline\n"
)


@dataclass(frozen=True)
class Tree:
    """Where one language keeps its hook bodies, and how it spells a definition.

    root is a directory or a single file, because the two languages differ:
    Go gives the hooks a package and Python gives them one module. Naming the
    file rather than its parent is what keeps a generated handler out of the
    scan, since Python's generated tree sits beside the module and a handler for
    linode_profile_security_question_answer ends in the answer kind's spelling.

    definition matches a top-level definition and captures its name, so the scan
    reads definitions rather than call sites: a generated handler calling a hook
    would otherwise count as a second copy of it.

    plumbing names the definitions that are not hooks and are expected to be
    here. An entry is a written decision, not a silent skip, which is the one
    place this gate could be made to lie.
    """

    root: str
    suffix: str
    definition: re.Pattern[str]
    plumbing: frozenset[str] = field(default_factory=frozenset)


_TREES: dict[str, Tree] = {
    "go": Tree(
        root="go/internal/toolhooks",
        suffix=".go",
        definition=re.compile(r"^func ([A-Z]\w*)\(", re.MULTILINE),
        plumbing=frozenset(
            {
                # The rule that turns a tool and a kind into the function name,
                # which the emitter writes its call through. It names no tool and
                # no kind, so it matches no declared pair; the entry says the
                # report naming it would be expected rather than a finding.
                "FunctionName",
            }
        ),
    ),
    "python": Tree(
        root="python/src/linodemcp/toolhooks.py",
        suffix=".py",
        definition=re.compile(r"^(?:async )?def ([a-zA-Z]\w*)\(", re.MULTILINE),
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


def flatten(name: str) -> str:
    """A name with its case and underscores dropped, for cross-language compare."""
    return name.replace("_", "").lower()


def declared_hooks() -> dict[str, str]:
    """Flattened function name to the kind its tool declared it under.

    Keyed by the name rather than the pair so a tree definition resolves to its
    kind in one lookup, and so two tools whose names collide once flattened
    would be visible as a lost entry rather than counted twice.
    """
    found: dict[str, str] = {}
    for declaration in declarations():
        tool = declaration.route_tool or declaration.meta_tool
        for kind in declaration.hooks:
            if kind in _OWNED_ELSEWHERE:
                continue
            found[flatten(tool) + flatten(kind)] = kind

    return found


def source_files(tree: Tree) -> list[Path]:
    """The files one language's tree keeps its hooks in.

    A root that names nothing is a scan that would count zero and pass, so it
    fails here instead.
    """
    root = _REPO_ROOT / tree.root
    if root.is_file():
        return [root]

    if not root.is_dir():
        msg = f"{tree.root} is neither a file nor a directory"
        raise SystemExit(msg)

    return [
        path
        for path in sorted(root.rglob("*" + tree.suffix))
        if not is_test_path(path.relative_to(root))
    ]


def is_test_path(relative: Path) -> bool:
    """Whether a path is test code rather than hook source."""
    if any(part in {"tests", "test", "testdata"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.stem.endswith("_test")


def defined(tree: Tree) -> list[str]:
    """Every top-level definition one language's tree makes, in file order."""
    found: list[str] = []
    for path in source_files(tree):
        found.extend(tree.definition.findall(path.read_text(encoding="utf-8")))

    return found


def attribute(
    language: str, tree: Tree, declared: dict[str, str]
) -> tuple[dict[str, int], list[str]]:
    """Count one tree's definitions under the kinds their tools declared.

    Takes the declared mapping rather than reading it, so the two halves this
    gate compares stay separable: a test can hold a fabricated tree against a
    fabricated contract and see what the comparison does to each disagreement.
    """
    found: dict[str, int] = dict.fromkeys(set(declared.values()), 0)
    seen: set[str] = set()
    problems: list[str] = []

    for name in defined(tree):
        if name in tree.plumbing:
            continue

        kind = declared.get(flatten(name))
        if kind is None:
            problems.append(
                f"{language}: {tree.root} defines {name}, which no tool"
                " declares a hook for"
            )
            continue

        found[kind] += 1
        seen.add(flatten(name))

    problems.extend(
        f"{language}: the contract declares a {declared[missing]} hook that"
        f" {tree.root} defines no function for ({missing})"
        for missing in sorted(set(declared) - seen)
    )

    return found, problems


def measure(languages: list[str]) -> tuple[dict[tuple[str, str], int], list[str]]:
    """Count each language's hook bodies per kind, reporting what cannot be counted."""
    problems: list[str] = []

    if not declarations():
        problems.append(
            "the contract declares no tools, so there is nothing to measure"
            " against: `make proto` has not run"
        )

    declared = declared_hooks()
    if not declared:
        problems.append(
            "no tool declares a countable hook kind, which is the option going"
            " rather than the bodies"
        )

    counts: dict[tuple[str, str], int] = {}

    for language in languages:
        tree = _TREES.get(language)
        if tree is None:
            problems.append(
                f"{language} is registered in languages.txt and has no hook tree"
                " here, so its hand-written bodies go uncounted"
            )
            continue

        found, trouble = attribute(language, tree, declared)
        problems.extend(trouble)

        for kind, count in found.items():
            counts[language, kind] = count

    return counts, problems


def read_counts(path: Path) -> dict[tuple[str, str], int]:
    """The recorded count per language per kind."""
    counts: dict[tuple[str, str], int] = {}
    for line in read_lines(path):
        fields = line.split()
        if len(fields) != 3 or not fields[2].isdigit():
            msg = f"{path.name} line is not `<language> <kind> <count>`: {line!r}"
            raise SystemExit(msg)

        language, kind, number = fields
        counts[language, kind] = int(number)

    return counts


def write_counts(path: Path, counts: dict[tuple[str, str], int]) -> None:
    """Rewrite the contract with the counts measured now.

    Languages keep the registry's order and kinds sort by name, so regenerating
    an unchanged tree rewrites the same bytes.
    """
    body = "".join(
        f"{language} {kind} {count}\n" for (language, kind), count in counts.items()
    )
    path.write_text(_COUNTS_HEADER + body, encoding="utf-8")


def ordered(
    counts: dict[tuple[str, str], int], languages: list[str]
) -> dict[tuple[str, str], int]:
    """The counts in registry order by language, then by kind name."""
    return {
        (language, kind): counts[language, kind]
        for language in languages
        for kind in sorted(kind for name, kind in counts if name == language)
    }


def compare(
    counts: dict[tuple[str, str], int], recorded: dict[tuple[str, str], int]
) -> list[str]:
    """Hold each measured count against its recorded line."""
    problems: list[str] = []

    problems.extend(
        f"{language} {kind}: {_COUNTS.name} names it, and {kind} is"
        f" {_OWNED_ELSEWHERE[kind]}"
        if kind in _OWNED_ELSEWHERE
        else f"{language} {kind}: {_COUNTS.name} names it, and it is neither a"
        " registered language nor a declared kind. A kind whose cohort is fully"
        " retired stops being declared, so its lines come out with it"
        for language, kind in sorted(set(recorded) - set(counts))
    )

    for (language, kind), count in counts.items():
        if (language, kind) not in recorded:
            problems.append(
                f"{language} {kind}: {_COUNTS.name} records no count for it"
            )
            continue

        if count > recorded[language, kind]:
            problems.append(
                f"{language} {kind}: {count} hand-written bodies, up from"
                f" {recorded[language, kind]}. A step the contract can carry"
                " belongs on the tool's *Input message"
            )
        elif count < recorded[language, kind]:
            problems.append(
                f"{language} {kind}: {count} hand-written bodies, down from"
                f" {recorded[language, kind]}. Lower the line in this change:"
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
            sys.stderr.write(f"hook-bodies: {problem}\n")
        return 1

    if arguments.update_baseline:
        write_counts(_COUNTS, ordered(counts, languages))
        sys.stdout.write(f"hook-bodies: wrote {_COUNTS}\n")
        return 0

    problems = compare(counts, read_counts(_COUNTS))
    if problems:
        for problem in problems:
            sys.stderr.write(f"hook-bodies: {problem}\n")
        return 1

    total = sum(counts.values())
    sys.stdout.write(f"hook-bodies: {total} across {len(languages)} languages\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
