"""Which store an audit query reads, and what happens when it fails.

The four query operations that can read SQLite (health, summary, export,
report) resolve the database path from the running config on every call, the
way Go does, rather than from a path a startup step installed only when the
sink opened. That is what kept the two languages on different data sources: a
configured database that would not open left Python silently reading the JSONL
log while Go answered a failure.

Neither answer is right for an audit surface. It must not change data source
without saying so, and it must not go dark when a secondary index is broken. So
a store that cannot be read falls back to the JSONL log and the answer carries a
warning naming the configured path and the reason.
"""

from __future__ import annotations

import sqlite3
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit.path import resolve_default_audit_dir

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.config import Config

# The database name the sink uses when the operator configured no path.
_DEFAULT_DB_NAME = "audit.db"

# What a read can fail with. Go falls back on any error the reader answers, so
# the set here is every failure its readers raise: the store's own, the file
# system's, and a decode of a column whose text is not the JSON it should be.
_READ_FAILURES = (OSError, sqlite3.Error, ValueError)


def resolve_sqlite_path(cfg: Config | None) -> str:
    """The SQLite database path, or "" when the sink is off.

    An empty string is what selects the JSONL fallback inside the readers, so
    the sink being disabled and the path being unset read the same way.
    """
    if cfg is None or not cfg.audit.sqlite.enabled:
        return ""

    if cfg.audit.sqlite.path:
        return cfg.audit.sqlite.path

    return str(Path(resolve_default_audit_dir()) / _DEFAULT_DB_NAME)


def read_with_fallback[Result](
    sqlite_path: str, read: Callable[[str], Result]
) -> tuple[Result, list[str]]:
    """Read through the configured store, or through JSONL with a warning.

    ``read`` takes the store path and answers the tool's own result; an empty
    path means the JSONL log. The configured path is attempted first whatever it
    holds, and a failure with nothing configured is re-raised, since a caller
    with no readable store at all has nothing to answer from.
    """
    try:
        return read(sqlite_path), []
    except _READ_FAILURES as failure:
        if not sqlite_path:
            raise

        return read(""), [store_warning(sqlite_path, str(failure))]


def store_warning(path: str, reason: str) -> str:
    """The line an answer carries when it came from JSONL instead of SQLite."""
    return (
        f"audit sqlite store at {path} could not be read ({reason});"
        " answered from the JSONL log instead"
    )
