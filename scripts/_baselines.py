"""Annotation-aware baseline file helpers shared by the gate scripts.

Every ratchet baseline under docs/contracts/ holds one entry per line. An entry may
carry a trailing acceptance annotation introduced by ``  # ``:

    <entry>  # accepted YYYY-MM-DD <reason or tracking-issue URL>

The annotation documents WHY the entry was accepted and when, so a baseline
line is a visible commitment instead of silent debt. Gate scripts compare
entries with the annotation stripped (the generated divergence strings never
contain one), and ``--update-baseline`` re-attaches the stored annotation for
every entry that survives, so regeneration cannot silently drop the audit
trail. The CI baseline guard (scripts/verify_baseline_direction.py) requires
a valid annotation on every line a change ADDS to a baseline.
"""

from __future__ import annotations

import re
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Iterable, Mapping
    from pathlib import Path

# The separator between an entry and its annotation. Two spaces before the
# hash so a single-space hash inside an entry (none exist today) could never
# be misread as an annotation.
ANNOTATION_SEPARATOR = "  # "

# What a valid acceptance annotation looks like: the word "accepted", an ISO
# date, and at least one non-space character after it. What that trailing
# text must contain depends on the file: ratchet baselines require a
# tracking-issue URL (see ISSUE_URL_PATTERN), the behavior-exempt list may
# carry a free-text reason.
ANNOTATION_PATTERN = re.compile(r"^accepted \d{4}-\d{2}-\d{2} \S")

# A ratchet acceptance is a promise to come back, and a promise with no
# issue has no home: free-text reasons pass review once and then rot, which
# is how "needs classifier review" shipped with nowhere to follow up. The
# guard therefore requires a resolvable issue URL inside ratchet
# annotations. Host-agnostic on purpose; the /issues/<n> path is the issue
# semantics being pinned, not a specific forge.
ISSUE_URL_PATTERN = re.compile(r"https://\S+/issues/\d+")


def split_annotation(line: str) -> tuple[str, str | None]:
    """Split one baseline line into (entry, annotation-or-None)."""
    entry, sep, note = line.partition(ANNOTATION_SEPARATOR)
    if not sep:
        return line.strip(), None
    return entry.strip(), note.strip()


def read_baseline(path: Path) -> dict[str, str | None]:
    """Read a baseline into {entry: annotation}, skipping comments and blanks."""
    if not path.exists():
        return {}

    entries: dict[str, str | None] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            entry, annotation = split_annotation(stripped)
            entries[entry] = annotation

    return entries


def read_entries(path: Path) -> set[str]:
    """Read a baseline's entry set with annotations stripped."""
    return set(read_baseline(path))


def format_line(entry: str, annotation: str | None) -> str:
    """Render one baseline line, re-attaching the annotation when present."""
    if annotation:
        return f"{entry}{ANNOTATION_SEPARATOR}{annotation}"
    return entry


def write_baseline(
    path: Path,
    header: str,
    entries: Iterable[str],
    annotations: Mapping[str, str | None],
) -> None:
    """Write a baseline, preserving the known annotation of each entry."""
    lines = [format_line(entry, annotations.get(entry)) for entry in sorted(entries)]
    path.write_text(header + "\n".join(lines) + "\n", encoding="utf-8")


def unannotated(
    entries: Iterable[str], annotations: Mapping[str, str | None]
) -> list[str]:
    """Return the sorted entries whose annotation is missing or malformed."""
    bad: list[str] = []
    for entry in entries:
        note = annotations.get(entry)
        if note is None or not ANNOTATION_PATTERN.match(note):
            bad.append(entry)

    return sorted(bad)


def missing_issue_url(
    entries: Iterable[str], annotations: Mapping[str, str | None]
) -> list[str]:
    """Sorted entries whose well-formed annotation cites no tracking issue.

    Only entries that already pass the ``unannotated`` check land here, so
    each entry appears in at most one failure bucket and the guard's output
    names the one thing to fix.
    """
    bad: list[str] = []
    for entry in entries:
        note = annotations.get(entry)
        well_formed = note is not None and ANNOTATION_PATTERN.match(note)
        if well_formed and not ISSUE_URL_PATTERN.search(note or ""):
            bad.append(entry)

    return sorted(bad)
