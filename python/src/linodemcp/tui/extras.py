"""Pure logic for the Phase 3 TUI extras (audit / profile / health screens).

Keeps the screens in ``app.py`` thin: everything testable without a terminal
lives here. The three concerns:

- audit: parse the ``linode_audit_recent`` payload into table rows (tool,
  capability, mode, status, timestamp, plan id),
- profile: list the configured + built-in profiles with the active marker
  (reusing ``cli.profile``'s catalog), and switch the active profile (reusing
  the CLI's config write, not a new mutation),
- health: map the ``linode_audit_health`` payload and the build/version info
  into display rows, plus the Prometheus endpoint pointer.

None of this reimplements tool logic or the config write. The audit and health
data come from dispatching the existing ``linode_audit_*`` tools (the screens
call ``run.execute``); the profile switch calls the same atomic config write the
CLI ``profile use`` uses.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any, cast

from linodemcp.cli.profile import all_profiles, resolve_active_name
from linodemcp.cli.runtime import load_config_or_default
from linodemcp.config import write_atomic

if TYPE_CHECKING:
    from pathlib import Path

    from linodemcp.config import Config

# Column order for the audit table. Kept here so the screen header and the row
# mapping stay in sync.
AUDIT_COLUMNS = ("timestamp", "tool", "capability", "mode", "status", "plan_id")


@dataclass(frozen=True)
class AuditRow:
    """One audit event flattened to the fields the audit screen shows."""

    timestamp: str
    tool: str
    capability: str
    mode: str
    status: str
    plan_id: str

    def as_cells(self) -> tuple[str, str, str, str, str, str]:
        """Return the row as cells in ``AUDIT_COLUMNS`` order."""
        return (
            self.timestamp,
            self.tool,
            self.capability,
            self.mode,
            self.status,
            self.plan_id,
        )


def _event_str(event: dict[str, Any], key: str) -> str:
    """Read a string field from an event dict, blank when absent or null.

    Audit events are JSON, so values arrive as ``Any``; this coerces to a
    display string and turns ``None`` (e.g. an unset ``plan_id``) into "".
    """
    value = event.get(key)
    if value is None:
        return ""
    return str(value)


def audit_event_row(event: dict[str, Any]) -> AuditRow:
    """Map one ``linode_audit_recent`` event to an ``AuditRow``.

    The event keys come from the audit wire format (``ts``, ``tool``,
    ``tool_capability``, ``mode``, ``status``, ``plan_id``). Missing or null
    fields render blank so a partial event never crashes the view.
    """
    return AuditRow(
        timestamp=_event_str(event, "ts"),
        tool=_event_str(event, "tool"),
        capability=_event_str(event, "tool_capability"),
        mode=_event_str(event, "mode"),
        status=_event_str(event, "status"),
        plan_id=_event_str(event, "plan_id"),
    )


def parse_audit_events(payload: str) -> list[AuditRow]:
    """Parse a ``linode_audit_recent`` JSON payload into audit rows.

    Returns an empty list for a non-JSON payload (e.g. an error string), for a
    payload without an ``events`` array, or for an empty feed, so the screen
    shows "no events" rather than raising.
    """
    try:
        data = json.loads(payload)
    except json.JSONDecodeError:
        return []
    if not isinstance(data, dict):
        return []
    events = cast("dict[str, Any]", data).get("events")
    if not isinstance(events, list):
        return []
    return _rows_from_events(events)


def _rows_from_events(events: Any) -> list[AuditRow]:
    """Map a list of raw event objects to ``AuditRow``s, skipping non-dicts.

    Takes ``Any`` (the value came from a JSON ``.get``) and casts to a concrete
    list here, so neither strict type checker sees a partially-unknown iterable
    nor a redundant cast (the source is ``Any``).
    """
    rows = cast("list[Any]", events)
    return [
        audit_event_row(cast("dict[str, Any]", raw))
        for raw in rows
        if isinstance(raw, dict)
    ]


@dataclass(frozen=True)
class ProfileRow:
    """One profile in the switcher: name, active marker, and a short summary."""

    name: str
    active: bool
    disabled: bool
    tool_count: int

    def as_cells(self) -> tuple[str, str, str, str]:
        """Return the row as (marker, name, state, tools) cells."""
        marker = "*" if self.active else ""
        state = "disabled" if self.disabled else "enabled"
        return (marker, self.name, state, str(self.tool_count))


def profile_rows(config_path: Path) -> list[ProfileRow]:
    """List every profile with the active marker, read from ``config_path``.

    Reuses ``cli.profile.all_profiles`` (built-ins + user-defined, with
    overrides folded in) and ``resolve_active_name`` so the TUI's list matches
    what ``profile list`` shows. A missing config file falls back to the
    in-memory default (no profiles file, the read-only ``default`` active), the
    same offline behavior the rest of the TUI uses.
    """
    cfg = _load_or_default(config_path)
    catalog = all_profiles(cfg)
    active = resolve_active_name(cfg)
    rows: list[ProfileRow] = []
    for name in sorted(catalog):
        prof = catalog[name]
        rows.append(
            ProfileRow(
                name=name,
                active=name == active,
                disabled=prof.disabled,
                tool_count=len(prof.allowed_tools),
            )
        )
    return rows


class ProfileSwitchError(ValueError):
    """A requested profile switch could not be applied.

    Raised for an unknown profile name; the screen shows the message rather
    than writing the config.
    """


def switch_active_profile(config_path: Path, name: str) -> None:
    """Set ``active_profile`` to ``name`` and write the config atomically.

    This is the same human-only operation ``profile use`` performs: validate the
    name against the catalog, set ``active_profile``, and persist via the CLI's
    ``write_atomic``. No MCP tool exposes this; it is a config mutation, not a
    dispatch. Raises ``ProfileSwitchError`` for an unknown name (the config is
    left untouched).

    A missing config file is materialized from the in-memory default so the
    switch creates the file with the chosen profile active.
    """
    cfg = _load_or_default(config_path)
    if name not in all_profiles(cfg):
        msg = f"profile {name!r} not found"
        raise ProfileSwitchError(msg)
    cfg.active_profile = name
    write_atomic(config_path, cfg)


def _load_or_default(config_path: Path) -> Config:
    """Load the config from ``config_path`` or fall back to the offline default.

    Reuses the CLI runtime's ``load_config_or_default`` so a missing file yields
    the in-memory default (read-only ``default`` profile, no environments), the
    same offline behavior the rest of the TUI relies on. A malformed file still
    raises, surfacing the real problem.
    """
    return load_config_or_default(config_path)


@dataclass(frozen=True)
class HealthRow:
    """One label/value pair for the health screen (rendered as a table)."""

    label: str
    value: str


def health_rows(payload: str) -> list[HealthRow]:
    """Map a ``linode_audit_health`` JSON payload to label/value rows.

    Surfaces the audit subsystem state the spec calls out: the JSONL path,
    whether the active log exists, rotated file count, disk bytes, and the
    dropped-event counter, plus a one-line SQLite summary. A non-JSON payload
    yields a single error row so the screen never blanks.
    """
    try:
        data = json.loads(payload)
    except json.JSONDecodeError:
        return [HealthRow("audit", "unavailable (no JSON payload)")]
    if not isinstance(data, dict):
        return [HealthRow("audit", "unavailable")]
    health = cast("dict[str, Any]", data)

    return [
        HealthRow("jsonl path", _event_str(health, "jsonl_path")),
        HealthRow("active log exists", _event_str(health, "active_log_exists")),
        HealthRow("rotated files", _event_str(health, "rotated_file_count")),
        HealthRow("oldest rotated", _event_str(health, "oldest_rotated_date")),
        HealthRow("disk bytes", _event_str(health, "disk_bytes")),
        HealthRow("dropped events", _event_str(health, "dropped_events")),
        HealthRow("sqlite", _sqlite_summary(health.get("sqlite"))),
    ]


def _sqlite_summary(sqlite: Any) -> str:
    """One-line summary of the health payload's ``sqlite`` block.

    ``null`` means the SQLite sink is off; a dict reports the row count when
    present, else just "enabled".
    """
    if sqlite is None:
        return "disabled"
    if isinstance(sqlite, dict):
        block = cast("dict[str, Any]", sqlite)
        count = block.get("event_count", block.get("row_count"))
        if count is not None:
            return f"enabled ({count} rows)"
        return "enabled"
    return "enabled"


def version_rows(version_info: dict[str, Any]) -> list[HealthRow]:
    """Map the build/version info dict to label/value rows for the health view.

    Shows the fields a user wants when reporting a problem: version, git commit
    and branch, the Python version, and the platform. Nested ``features`` is
    omitted (the catalog is the better surface for that).
    """
    fields = (
        ("version", "version"),
        ("git commit", "git_commit"),
        ("git branch", "git_branch"),
        ("python", "python_version"),
        ("platform", "platform"),
    )
    return [HealthRow(label, _event_str(version_info, key)) for label, key in fields]


def metrics_pointer(*, enabled: bool, port: int, path: str) -> str:
    """Return a one-line pointer to the Prometheus metrics endpoint.

    The TUI does not scrape metrics; it only tells the user where the endpoint
    lives when the server runs under ``serve``. When metrics are disabled in
    config, that is stated instead. The note makes clear the endpoint is not up
    during a one-shot TUI session.
    """
    if not enabled:
        return "metrics: disabled in config"
    return (
        f"metrics: http://localhost:{port}{path} "
        "(served by `linodemcp serve`, not the TUI session)"
    )
