#!/usr/bin/env python3
"""Ratchet gate: call sites that still build their request endpoint by hand.

Each tool's route is declared once, as a `linode.mcp.v1.tool_route` option on
its proto input message, and each client now has request primitives that
resolve the method and path from that declaration: Go's makeRouteRequest,
makeRouteRequestQuery and makeRouteRequestContentType plus the generic
routedGet helper, Python's make_route_request and
make_route_request_content_type. A call
site that reaches one of those names its tool and passes the path values, so
the URL exists in the proto and nowhere else.

Every other call site still spells the route out again in its own language.
That is the duplication tool_route was added to remove, and it is the reason
verify_route_evidence.py has to resolve a URL out of a base constant and a
format verb before it can check the contract against real code. Nothing was
counting the remaining copies, so the migration had no measure and no end.

This counts them, per registered language, and holds the number against
docs/contracts/route-source-counts.txt.

What each failure means:

- MORE hand-built call sites than the file records: a new call site was written
  against the older primitive, or a migration went backwards. Route the site.
  The recorded number is the remaining debt, not a budget to spend.
- FEWER: call sites moved and the file still claims the old debt. Lower the
  line in the same change, the way a fixed ratchet entry leaves its baseline
  instead of sitting there once it is no longer true.
- a registered language with no scanner arm here, no line in the file, or a
  line naming no registered language: reported by name, so a language cannot
  join the tool surface with its migration unwatched.
- a primitive that no declaration in its own tree matches: call sites are
  counted by primitive name, so a rename would empty the count and read as a
  finished migration. It fails instead.

Only the primitives that put a request on the wire are counted. A wrapper that
takes an endpoint from its caller and forwards it is counted once, at its own
primitive call, rather than once per caller, so the wrapper set does not have
to be maintained here to keep the count honest. A name in a client's plumbing
tuple (Go's fetchList) is not counted even once: its endpoints are already
contract-resolved, arriving only from the routed list fetchers, and the gate
targets sites that spell routes out again, which fetchList never does. Zero
still means done: at zero, nothing reaches a primitive with an endpoint built
anywhere but the proto.

Which trees are scanned comes from docs/contracts/languages.txt rather than a
path written here, and each language's client shape is one entry in _CLIENTS.
Source is read as text with line comments stripped; test files and generated
trees are skipped, since a fixture endpoint is not debt this migration removes.

Stdlib only, so no venv is needed. Run via `make route-source` (in `make
check`, and so in the pre-push hook and the CI gate on every branch).

Usage: verify_route_source.py [--update-baseline]
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Iterable, Iterator
    from re import Pattern

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_COUNTS = _REPO_ROOT / "docs" / "contracts" / "route-source-counts.txt"

_COUNTS_HEADER = (
    "# Route-source counts: how many call sites in each registered language\n"
    "# still build their request endpoint by hand, instead of resolving it\n"
    "# from the tool_route option on the tool's proto input message.\n"
    "# Owner: scripts/verify_route_source.py (`make route-source`, inside\n"
    "# `make check`).\n"
    "#\n"
    "# Direction: counts only FALL. A count above its line fails (a new\n"
    "# hand-built call site, or a migration that went backwards). A count\n"
    "# below its line fails too, asking for the line to be lowered in the\n"
    "# same change, so this file always states the real remaining work\n"
    "# rather than an old high-water mark. Every ratchet under\n"
    "# docs/contracts/ behaves that way: what got fixed leaves the file.\n"
    "#\n"
    "# One line per language registered in languages.txt:\n"
    "#   <language> <hand-built call sites>\n"
    "# A registered language with no line fails by name, and a line naming no\n"
    "# registered language fails by name, so this file and the registry\n"
    "# cannot drift apart.\n"
    "#\n"
    "# Deliberately not named *-baseline.txt: files under that glob hold one\n"
    "# accepted divergence per line, and scripts/verify_baseline_direction.py\n"
    "# makes every line a change ADDS carry a dated tracking-issue\n"
    "# annotation. A line here is a measurement, not an acceptance, so there\n"
    "# is no promise for an annotation to record. Growth is already refused\n"
    "# outright by the gate, which is stricter than the annotation rule.\n"
    "#\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_route_source.py --update-baseline\n"
)


@dataclass(frozen=True)
class Client:
    """How one language's client writes a request call site.

    suffix and comment say how to read its source; declaration recognizes a
    function declaration and captures its name, which is what lets a primitive
    calling another primitive be told apart from a call site. handbuilt names
    the primitives that take a method and an endpoint, routed the ones that
    take a tool, and plumbing the wrappers whose endpoints only ever arrive
    contract-resolved, so their bodies are skipped the way a primitive's is.

    Both are plural because a language grows variants of each: a routed
    primitive that also carries a query string still reaches the hand-built one
    to send anything, and counting that hop would leave a remainder no
    migration could clear.
    """

    suffix: str
    comment: str
    declaration: Pattern[str]
    handbuilt: tuple[str, ...]
    routed: tuple[str, ...]
    plumbing: tuple[str, ...] = ()


@dataclass(frozen=True)
class Counts:
    """What one language's scan found."""

    handbuilt: int
    routed: int
    undeclared: tuple[str, ...]


# Python's client.request is httpx's own method. The primitives reach it as
# plumbing, and naming it here keeps any future call site that hands httpx a
# hand-assembled URL counted as the debt it would be.
_CLIENTS: dict[str, Client] = {
    "go": Client(
        suffix=".go",
        comment="//",
        declaration=re.compile(
            r"^func\s+(?:\([^)]*\)\s*)?(\w+)\s*(?:\[[^\]]*\])?\s*\("
        ),
        handbuilt=("makeRequest", "makeRequestWithContentType"),
        routed=(
            "makeRouteRequest",
            "makeRouteRequestQuery",
            "makeRouteRequestContentType",
            # The generic typed helper resolves its route from the proto
            # before touching makeRequest, so its body is plumbing and its
            # call sites are proto-resolved.
            "routedGet",
        ),
        # The shared list transport: every endpoint it forwards was resolved
        # from the proto by a routed list fetcher, so its one request call is
        # the transport handing work to the transport. It keeps taking an
        # endpoint instead of a tool because cmd/route-dump discovers the
        # routed fetchers structurally through this endpoint-carrying hop,
        # and removing it would leave their routes without evidence.
        plumbing=("fetchList",),
    ),
    "python": Client(
        suffix=".py",
        comment="#",
        declaration=re.compile(r"^\s*(?:async\s+)?def\s+(\w+)\s*\("),
        handbuilt=("make_request", "client.request"),
        routed=("make_route_request", "make_route_request_content_type"),
    ),
}


def call_pattern(names: Iterable[str]) -> Pattern[str]:
    """Match a call to one of these primitives, at the dot that reaches it.

    The receiver is left out because it is a local name (c, self, client) that
    says nothing about which primitive a site calls, and one primitive is
    reached through several of them. Matching from the dot also keeps the
    declaration itself out of the count.
    """
    alternatives = "|".join(re.escape(name) for name in names)

    # The undotted branch is for Go's generic helpers, reached as plain
    # functions with a type argument (routedGet[Domain](...)); requiring the
    # bracket keeps every other undotted mention out of the count.
    return re.compile(
        rf"(?:\.(?:{alternatives})|(?<![.\w])(?:{alternatives})\[[^\]]*\])\("
    )


def is_test_path(relative: Path) -> bool:
    """Whether a path is test code rather than client source."""
    if any(part in {"tests", "test", "testdata"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.stem.endswith("_test")


def source_files(workdir: Path, suffix: str) -> Iterator[Path]:
    """Hand-written source under workdir, in path order.

    Generated trees are skipped because no migration happens in code nobody
    edits, and test files because a fixture endpoint is not a call site the
    migration removes; counting one would put a floor under the ratchet that
    no amount of work could lift.
    """
    for path in sorted(workdir.rglob(f"*{suffix}")):
        relative = path.relative_to(workdir)
        if any(part.startswith(".") for part in relative.parts):
            continue
        if "genpb" in relative.parts or is_test_path(relative):
            continue

        yield path


def undeclared_primitives(client: Client, declared: set[str]) -> tuple[str, ...]:
    """Primitives the scan never saw declared in this language's own tree.

    A primitive named as an attribute of the HTTP library (Python's
    client.request) is declared in that library, so there is nothing to find
    here and it is left out of the check.
    """
    expected = {*client.handbuilt, *client.routed, *client.plumbing}

    return tuple(sorted(name for name in expected - declared if "." not in name))


def scan(client: Client, workdir: Path) -> Counts:
    """Count one language's request call sites.

    A call made from inside a primitive's own body is plumbing rather than a
    call site: it is the transport handing work to the transport, and the
    endpoint it passes came from somewhere else. Counting those would leave a
    remainder no migration could clear, since makeRouteRequest reaching
    makeRequest is the shape this whole change is moving toward.
    """
    handbuilt = call_pattern(client.handbuilt)
    routed = call_pattern(client.routed)
    primitives = (
        frozenset(client.handbuilt)
        | frozenset(client.routed)
        | frozenset(client.plumbing)
    )

    hand = 0
    resolved = 0
    declared: set[str] = set()

    for path in source_files(workdir, client.suffix):
        enclosing = ""
        for raw in path.read_text(encoding="utf-8").splitlines():
            declaration = client.declaration.match(raw)
            if declaration is not None:
                enclosing = declaration.group(1)
            if enclosing in primitives:
                declared.add(enclosing)
                continue

            code = raw.split(client.comment, 1)[0]
            hand += len(handbuilt.findall(code))
            resolved += len(routed.findall(code))

    return Counts(hand, resolved, undeclared_primitives(client, declared))


def registered_languages(path: Path) -> list[tuple[str, Path]]:
    """(name, working dir) per registry line, in file order."""
    languages: list[tuple[str, Path]] = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = [field.strip() for field in line.split("\t") if field.strip()]
        if len(fields) < 2:
            msg = f"unparsable {path.name} line {raw!r}"
            raise SystemExit(msg)
        languages.append((fields[0], _REPO_ROOT / fields[1]))

    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def read_counts(path: Path) -> dict[str, int]:
    """Language-to-count map from the contract file."""
    if not path.exists():
        return {}

    counts: dict[str, int] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 2 or not fields[1].isdigit():
            msg = f"unparsable {path.name} line {raw!r}; want `<language> <count>`"
            raise SystemExit(msg)
        counts[fields[0]] = int(fields[1])

    return counts


def write_counts(
    path: Path, languages: list[tuple[str, Path]], measured: dict[str, Counts]
) -> None:
    """Rewrite the contract from the measured counts, header included.

    Registry order rather than sorted, so the file reads in the same order as
    languages.txt, where the first language is the reference implementation.
    """
    lines = [f"{name} {measured[name].handbuilt}" for name, _ in languages]
    path.write_text(_COUNTS_HEADER + "\n".join(lines) + "\n", encoding="utf-8")


def missing_arms(languages: list[tuple[str, Path]]) -> list[str]:
    """Registered languages with no client shape here."""
    return sorted(name for name, _ in languages if name not in _CLIENTS)


def renamed_primitives(measured: dict[str, Counts]) -> list[str]:
    """Languages whose scan could not find a primitive it counts by name."""
    return [
        f"{name}: {', '.join(counts.undeclared)}"
        for name, counts in sorted(measured.items())
        if counts.undeclared
    ]


def scope_problems(
    languages: list[tuple[str, Path]], recorded: dict[str, int]
) -> list[str]:
    """Registered languages and recorded lines that do not answer each other."""
    names = {name for name, _ in languages}

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
    measured: dict[str, Counts], recorded: dict[str, int]
) -> tuple[list[str], list[str]]:
    """Languages that gained hand-built call sites, and ones that lost them."""
    grew: list[str] = []
    shrank: list[str] = []

    for name, counts in sorted(measured.items()):
        expected = recorded[name]
        if counts.handbuilt > expected:
            grew.append(
                f"{name}: {counts.handbuilt} hand-built call site(s),"
                f" {expected} recorded (+{counts.handbuilt - expected})"
            )
        elif counts.handbuilt < expected:
            shrank.append(
                f"{name}: {counts.handbuilt} hand-built call site(s),"
                f" {expected} recorded (-{expected - counts.handbuilt})"
            )

    return grew, shrank


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def _summary(measured: dict[str, Counts]) -> str:
    """The success line: what was counted, and how it splits by language."""
    per_language = ", ".join(
        f"{name} {counts.handbuilt}" for name, counts in sorted(measured.items())
    )
    hand = sum(counts.handbuilt for counts in measured.values())
    routed = sum(counts.routed for counts in measured.values())

    return (
        f"route-source gate OK: {hand} hand-built call site(s)"
        f" ({per_language}), {routed} resolved from the proto"
    )


def main(argv: list[str]) -> int:
    """Measure every registered language and hold it to the recorded count."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--update-baseline", action="store_true")
    args = parser.parse_args(argv)

    languages = registered_languages(_LANGUAGES)

    arms = missing_arms(languages)
    if arms:
        _report(
            "no route-source scanner declared for registered language(s):",
            arms,
            "add its client shape to _CLIENTS in scripts/verify_route_source.py",
        )
        return 1

    measured = {name: scan(_CLIENTS[name], workdir) for name, workdir in languages}

    renamed = renamed_primitives(measured)
    if renamed:
        _report(
            "request primitives with no declaration in their own tree:",
            renamed,
            "call sites are counted by primitive name, so a rename empties the"
            " count and reads as a finished migration; point _CLIENTS at the"
            " new name",
        )
        return 1

    if args.update_baseline:
        write_counts(_COUNTS, languages, measured)
        print(f"wrote {len(measured)} route-source count(s)", file=sys.stderr)
        return 0

    recorded = read_counts(_COUNTS)

    scope = scope_problems(languages, recorded)
    if scope:
        _report(
            "route-source counts and the language registry disagree:",
            scope,
            f"every registered language carries one line in {_COUNTS.name} and"
            " nothing else does; measure a new one with --update-baseline",
        )
        return 1

    grew, shrank = ratchet_problems(measured, recorded)
    if grew:
        _report(
            "hand-built request call sites have grown:",
            grew,
            "resolve the new site's route from the proto (name the tool to the"
            " route primitive) rather than raising the recorded count",
        )
    if shrank:
        _report(
            "hand-built request call sites are below the recorded count:",
            shrank,
            f"lower the line in docs/contracts/{_COUNTS.name} in this change"
            " (--update-baseline writes it), so the file keeps stating the"
            " real remaining work",
        )
    if grew or shrank:
        return 1

    print(_summary(measured))

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
