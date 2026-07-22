#!/usr/bin/env python3
"""Baseline growth guard: added ratchet entries must carry an annotation.

Most docs/contracts/*-baseline.txt files are ratchets whose entries are supposed
to shrink. `make check` runs against committed state only, so it structurally
cannot see that a baseline GREW; the growth direction needs a reference point.
This guard supplies it: given a base git rev, it diffs each baseline's entry set
and fails when a change ADDS an entry without a valid acceptance annotation:

    <entry>  # accepted YYYY-MM-DD <tracking-issue URL, plus optional context>

On the ratchet baselines the annotation MUST cite a tracking-issue URL
(https://.../issues/<n>): a ratchet entry is a promise to come back, and a
free-text reason gives that promise no home. One shipped reading "needs
classifier review" with nowhere to follow up, and both the human and the
automated pair review passed it because this guard was green; conventions
that matter get enforced here, not in prose. behavior-exempt.txt is the one
annotated file where free text stays valid, because a permanent exemption
carries no follow-up work to track.

Growth stays possible (landing one language ahead of the others is a real
workflow), but only as a visible, dated commitment that review can see in
the diff itself. Removals never fail; shrinking is the point of a ratchet.

Two files under the same glob are not ratchets: api-defaults-baseline.txt and
enum-sync-baseline.txt are full drift snapshots the scheduled sync scripts
regenerate wholesale from the live Linode spec. A new API-side default or enum
there is ordinary upstream drift the sync gate itself already watches, not a
divergence someone chose to accept, and `--update-baseline` writes those lines
with no annotation it could attach. So they are exempt from this guard; see
_SNAPSHOT_BASELINES.

behavior-exempt.txt is guarded too, though it is not a ratchet: an entry
there removes a tool from behavior-fixture coverage permanently, so a NEW
exemption needs the same dated annotation as accepted ratchet growth (with
a free-text reason allowed in place of an issue URL). See _ANNOTATED_EXTRAS.

Runs inside `make check` (and so the pre-push hook) against origin/main,
which is the right base locally and on PRs; an unreachable base rev skips
loudly rather than failing unrelated work. CI additionally runs it with the
event's true base (.github/workflows/baseline-guard.yml: the PR merge
parent, or the push's previous tip), which matters on pushes to main where
origin/main already equals HEAD and the in-check run sees an empty diff.
Stdlib plus scripts/_baselines.py only, so no venv is needed.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"

# Regenerated drift snapshots, not hand-shrinkable ratchets: the sync scripts
# rewrite these wholesale from the live spec and their added lines carry no
# annotation, so the growth-must-be-annotated rule does not apply. Kept in
# lockstep with verify_sync_defaults.BASELINE / verify_sync_enums.BASELINE by
# test_verify_baseline_direction.py, so a rename there cannot silently re-guard
# a snapshot or leave a new one unguarded.
_SNAPSHOT_BASELINES = frozenset(
    {
        "api-defaults-baseline.txt",
        "api-pagination-baseline.txt",
        "api-response-shapes-baseline.txt",
        "enum-sync-baseline.txt",
    }
)

# Outside the *-baseline.txt glob but under the same growth rule: an entry in
# behavior-exempt.txt removes a tool from behavior-fixture coverage for good,
# which is a bigger commitment than any ratchet line. New exemptions must
# carry the same dated annotation; verify_behavior reads only the tab-split
# tool name, so the annotation is invisible to the gate itself.
_ANNOTATED_EXTRAS = ("behavior-exempt.txt",)


def _guarded_baselines(contracts: Path) -> list[Path]:
    """Files this guard checks: the ratchets minus snapshots, plus the exempt list."""
    guarded = [
        path
        for path in sorted(contracts.glob("*-baseline.txt"))
        if path.name not in _SNAPSHOT_BASELINES
    ]
    guarded.extend(contracts / name for name in _ANNOTATED_EXTRAS)
    return guarded


def _git_show(rev: str, rel_path: str) -> str | None:
    """Return the file content at rev, or None when absent there."""
    result = subprocess.run(
        ["git", "show", f"{rev}:{rel_path}"],
        cwd=_REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return None
    return result.stdout


def _rev_exists(rev: str) -> bool:
    """Report whether the rev resolves in this clone."""
    result = subprocess.run(
        ["git", "rev-parse", "--verify", f"{rev}^{{commit}}"],
        cwd=_REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    return result.returncode == 0


def _entries_from_text(text: str) -> set[str]:
    """Parse baseline text into its annotation-stripped entry set."""
    entries: set[str] = set()
    for raw in text.splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            entry, _ = _baselines.split_annotation(stripped)
            entries.add(entry)
    return entries


def _check_file(path: Path, base_rev: str) -> list[str]:
    """Return violation lines for one baseline file."""
    rel = path.relative_to(_REPO_ROOT).as_posix()
    base_text = _git_show(base_rev, rel)
    if base_text is None:
        # Base revs older than the docs/contracts/ move hold the baselines at
        # docs/<name>; anchor the diff there so the move itself does not read
        # as unannotated growth.
        base_text = _git_show(base_rev, f"docs/{path.name}")
    base_entries = _entries_from_text(base_text) if base_text is not None else set()

    current = _baselines.read_baseline(path)
    added = sorted(set(current) - base_entries)
    if not added:
        return []

    bad = set(_baselines.unannotated(added, current))
    # Ratchet acceptances must point at a tracking issue; the extras list
    # (behavior-exempt.txt) documents permanent exemptions, so a free-text
    # reason remains valid there.
    bad_url: set[str] = set()
    if path.name not in _ANNOTATED_EXTRAS:
        bad_url = set(_baselines.missing_issue_url(added, current))

    problems: list[str] = []
    for entry in added:
        marker = "ok (annotated)"
        if entry in bad:
            marker = "MISSING ANNOTATION"
        elif entry in bad_url:
            marker = "MISSING TRACKING-ISSUE URL"
        problems.append(f"  {rel}: + {entry}  [{marker}]")

    if not bad and not bad_url:
        return []
    return problems


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: verify_baseline_direction.py <base-git-rev>", file=sys.stderr)
        return 2

    base_rev = sys.argv[1]
    if not _rev_exists(base_rev):
        # A force-push or shallow clone can drop the reference point; without
        # one there is nothing to diff against, so report and stand down
        # rather than fail every unrelated change.
        print(f"baseline guard: base rev {base_rev!r} not found; skipping")
        return 0

    problems: list[str] = []
    for path in _guarded_baselines(_CONTRACTS):
        problems.extend(_check_file(path, base_rev))

    if not problems:
        print(f"baseline guard OK: no unannotated growth vs {base_rev}")
        return 0

    print(f"baseline entries added since {base_rev}:")
    for line in problems:
        print(line)
    print(
        "\nEvery ADDED baseline entry must carry an acceptance annotation so"
        " the growth is a visible, dated commitment:"
        "\n  <entry>  # accepted YYYY-MM-DD <tracking-issue URL, plus"
        " optional context>"
        "\nRatchet acceptances must cite a tracking issue (https://.../"
        "issues/<n>); a promise to come back needs a home. behavior-exempt"
        ".txt may carry a free-text reason instead (permanent exemptions"
        " have no follow-up to track)."
        "\nFix the divergence instead if it was not meant to be accepted."
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
