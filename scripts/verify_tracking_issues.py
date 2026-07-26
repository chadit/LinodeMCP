#!/usr/bin/env python3
"""LIVE-check that every baseline's tracking issue is still open.

A ratchet acceptance is a promise to come back, and _baselines.py says a
promise with no issue has no home. The guard that enforces that
(verify_baseline_direction.py) matches ISSUE_URL_PATTERN, which is a check on
the shape of a URL and nothing else. A closed issue passes it forever.

That is not hypothetical. Issue 1038 closed on 2026-07-20 while twenty baseline
lines across eight files cited it as the thing that would close them, and six of
those annotations were written on 2026-07-21, the day after. Nothing reported
it, because every one of those URLs still looked like a URL.

This resolves each cited issue and fails on any that is closed, so an
acceptance cannot outlive the promise behind it. Being wrong here is cheap in
one direction and expensive in the other: a closed issue named out loud costs
one re-annotation, while a silent one costs the next reader an investigation to
discover the debt has no owner.

Deliberately NOT part of `make check`: it queries the forge, so it is
non-deterministic and offline-hostile, exactly like the other sync gates. Run
via `make sync-issues` (in `make sync`, so the scheduled drift workflow covers
it). Without gh on PATH it skips loudly rather than failing work that has
nothing to do with issue state, matching how diff-coverage handles an
unreachable base rev.

Stdlib only.

Usage: verify_tracking_issues.py [--states PATH]

  --states PATH  read {url: state} from a JSON file instead of querying,
                 which is what keeps this script's own tests offline
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"

# Snapshots record what upstream said at a reviewed version; they carry no
# acceptances, so they cite no issues. Kept in step with the same list in
# verify_baseline_direction.py.
_SNAPSHOT_BASELINES = frozenset(
    {
        "api-defaults-baseline.txt",
        "api-pagination-baseline.txt",
        "api-response-shapes-baseline.txt",
    }
)

# The annotated files that are not ratchets, same set the baseline guard walks.
_ANNOTATED_EXTRAS = ("behavior-exempt.txt", "scope-sync-exempt.txt")

# A GitHub issue URL, captured as owner / repo / number so it can be turned
# into an API call. Host-agnostic matching is not possible here: resolving an
# issue needs a forge API, and GitHub is the only one this repo cites.
_ISSUE_URL = re.compile(r"https://github\.com/([^/\s]+)/([^/\s]+)/issues/(\d+)")

# Resolution goes through the gh CLI rather than a raw API call: the issue
# tracker is a private repo, so the request needs credentials, and gh already
# owns that in both places this runs (a developer's login locally, GH_TOKEN in
# the workflow). It is also the only GitHub client this repo already depends on.
_GH = "gh"

_OPEN = "open"


def guarded_files(contracts: Path) -> list[Path]:
    """Every file whose annotations carry a tracking issue."""
    guarded = [
        path
        for path in sorted(contracts.glob("*-baseline.txt"))
        if path.name not in _SNAPSHOT_BASELINES
    ]
    guarded.extend(contracts / name for name in _ANNOTATED_EXTRAS)

    return [path for path in guarded if path.exists()]


def cited_issues(paths: list[Path]) -> dict[str, list[str]]:
    """Map each cited issue URL to the entries citing it.

    Entries are reported per issue rather than per line so a closed issue names
    everything that has to move, which is the whole cost of the fix.
    """
    citations: dict[str, list[str]] = {}

    for path in paths:
        for entry, annotation in _baselines.read_baseline(path).items():
            match = _ISSUE_URL.search(annotation or "")
            if match is None:
                continue

            citations.setdefault(match.group(0), []).append(f"{path.name}: {entry}")

    return citations


def issue_state(url: str) -> str:
    """Resolve one issue's state, or a reason it could not be resolved.

    A URL that does not resolve is reported rather than assumed open: an issue
    that was deleted, moved, or renumbered is exactly as ownerless as a closed
    one, and assuming otherwise is how this gate would learn to lie.
    """
    match = _ISSUE_URL.match(url)
    if match is None:
        return "unparsable URL"

    owner, repo, number = match.groups()
    # Fixed argv, no shell; every part comes from the already-matched URL.
    proc = subprocess.run(
        [_GH, "api", f"repos/{owner}/{repo}/issues/{number}", "--jq", ".state"],
        capture_output=True,
        text=True,
        check=False,
        timeout=30,
    )

    if proc.returncode != 0:
        return (
            f"unresolvable ({proc.stderr.strip().splitlines()[-1:] or ['no output']})"
        )

    return proc.stdout.strip() or "unknown"


def closed_citations(
    citations: dict[str, list[str]], states: dict[str, str]
) -> list[str]:
    """Report lines for every cited issue that is not open."""
    reported: list[str] = []

    for url in sorted(citations):
        state = states.get(url, "unknown")
        if state == _OPEN:
            continue

        reported.append(f"{url} is {state}")
        reported.extend(f"    {entry}" for entry in sorted(citations[url]))

    return reported


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--states",
        help="read {url: state} from a JSON file instead of querying the forge",
    )
    args = parser.parse_args(argv)

    citations = cited_issues(guarded_files(_CONTRACTS))
    if not citations:
        print("tracking-issue gate OK: no baseline cites an issue")
        return 0

    if args.states:
        states = {
            str(url): str(state)
            for url, state in json.loads(
                Path(args.states).read_text(encoding="utf-8")
            ).items()
        }
    elif shutil.which(_GH) is None:
        print(
            f"tracking-issue gate SKIPPED: {_GH} is not on PATH, so the "
            f"{len(citations)} cited issue(s) went unchecked. Skipping loudly "
            "rather than passing an unchecked promise.",
            file=sys.stderr,
        )
        return 0
    else:
        states = {url: issue_state(url) for url in citations}

    closed = closed_citations(citations, states)
    if not closed:
        print(f"tracking-issue gate OK: {len(citations)} cited issue(s) still open")
        return 0

    print(
        "baseline acceptances citing an issue that is no longer open:",
        file=sys.stderr,
    )
    for line in closed:
        print(f"  {line}", file=sys.stderr)

    print(
        "\nEach line above is a promise with no owner. Close the debt, or move the"
        " entry to an exemption file when nothing here can close it, or re-annotate"
        " it against a live issue.",
        file=sys.stderr,
    )

    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
