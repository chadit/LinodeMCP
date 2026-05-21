"""Recent-events reader over the JSONL audit log.

Mirrors ``go/internal/audit/reader.go``. Scans the active ``audit.log``
and rotated ``audit-YYYY-MM-DD.log[.gz]`` files, newest first, applying
optional filters and returning up to a limit.
"""

from __future__ import annotations

import fnmatch
import gzip
import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit.event import Event
from linodemcp.audit.jsonl import ACTIVE_LOG_FILE_NAME
from linodemcp.audit.retention import parse_rotated_file_day

if TYPE_CHECKING:
    from datetime import date, datetime

    from linodemcp.audit.event import Capability, Status

# Default number of events returned when the caller does not specify a
# limit. Mirrors the Go DefaultRecentLimit.
DEFAULT_RECENT_LIMIT = 20

# Hard cap so a single query can't pull an unbounded slice into memory.
MAX_RECENT_LIMIT = 200


@dataclass
class RecentQuery:
    """Filters for a recent-events scan.

    Unset fields (None / empty / 0) mean "no constraint". ``tool`` is a
    glob (fnmatch syntax); ``capability`` and ``status`` are exact
    matches; ``since`` and ``until`` bound the event timestamp
    (inclusive). Meta events are excluded unless ``include_meta``.
    """

    limit: int = 0
    since: datetime | None = None
    until: datetime | None = None
    tool: str = ""
    capability: Capability | str = ""
    status: Status | str = ""
    include_meta: bool = field(default=False)


def read_recent(directory: str, query: RecentQuery) -> list[Event]:
    """Return up to ``limit`` events, newest first, matching the filters.

    Scans the active ``audit.log`` first (today's events), then rotated
    files in date-descending order, stopping once the limit is reached.
    A missing directory returns an empty list (querying before the
    first event is normal). Undecodable lines and unreadable files are
    skipped so one corrupt record doesn't abort the scan.
    """
    limit = query.limit
    if limit <= 0:
        limit = DEFAULT_RECENT_LIMIT
    limit = min(limit, MAX_RECENT_LIMIT)

    base = Path(directory)
    if not base.is_dir():
        return []

    results: list[Event] = []

    for name in _ordered_audit_files(base):
        events = _read_events_from_file(base / name)

        # Within a file lines are append-order (oldest first); walk
        # backwards so the accumulator stays newest-first across the
        # date-descending file order.
        for event in reversed(events):
            if not _matches(query, event):
                continue

            results.append(event)
            if len(results) >= limit:
                return results

    return results


def scan_events(
    directory: str, since: datetime | None, include_meta: bool
) -> list[Event]:
    """Return every event at or after ``since`` (None = all), honoring
    ``include_meta``. Unlike :func:`read_recent` there is no count limit:
    aggregation (Phase 3d summary, 3f export) needs the whole window. A
    missing directory returns an empty list.
    """
    base = Path(directory)
    if not base.is_dir():
        return []

    events: list[Event] = []

    for name in _ordered_audit_files(base):
        for event in _read_events_from_file(base / name):
            if not include_meta and event.tool_capability == "meta":
                continue

            if since is not None and event.ts < since:
                continue

            events.append(event)

    return events


def _matches(query: RecentQuery, event: Event) -> bool:
    """Report whether an event satisfies every set filter."""
    if not query.include_meta and event.tool_capability == "meta":
        return False

    if query.tool and not fnmatch.fnmatchcase(event.tool, query.tool):
        return False

    if query.capability and event.tool_capability != query.capability:
        return False

    if query.status and event.status != query.status:
        return False

    if query.since is not None and event.ts < query.since:
        return False

    return not (query.until is not None and event.ts > query.until)


def _ordered_audit_files(base: Path) -> list[str]:
    """Return audit file names to scan, newest first.

    The active ``audit.log`` (if present) comes first, followed by
    rotated files in date-descending order.
    """
    has_active = False
    rotated: list[tuple[date, str]] = []

    for entry in base.iterdir():
        if not entry.is_file():
            continue

        name = entry.name
        if name == ACTIVE_LOG_FILE_NAME:
            has_active = True
            continue

        day = parse_rotated_file_day(name)
        if day is not None:
            rotated.append((day, name))

    rotated.sort(key=lambda pair: pair[0], reverse=True)

    names: list[str] = []
    if has_active:
        names.append(ACTIVE_LOG_FILE_NAME)

    names.extend(name for _, name in rotated)
    return names


def _read_events_from_file(path: Path) -> list[Event]:
    """Decode every JSON line in ``path`` into an Event, oldest first.

    Gzipped rotated files are decompressed transparently. Open failures
    yield an empty list (the scan skips the file); undecodable lines are
    skipped individually.
    """
    try:
        raw = _read_file_text(path)
    except OSError:
        return []

    events: list[Event] = []

    for line in raw.splitlines():
        stripped = line.strip()
        if not stripped:
            continue

        try:
            events.append(Event.from_dict(json.loads(stripped)))
        except (ValueError, KeyError):
            continue

    return events


def _read_file_text(path: Path) -> str:
    """Read a log file as text, decompressing .gz rotated files."""
    if path.name.endswith(".gz"):
        with gzip.open(path, "rt", encoding="utf-8") as handle:
            return handle.read()

    return path.read_text(encoding="utf-8")
