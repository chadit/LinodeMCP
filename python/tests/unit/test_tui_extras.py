"""Unit tests for the Phase 3 TUI extras logic (no terminal involved).

Covers the pure pieces the audit/profile/health screens delegate to: mapping an
audit event to a row, parsing the recent feed, listing profiles with the active
marker, the config-write switch, and the health/version/metrics rendering. The
screens are thin wrappers over these, so this is where the behavior is locked.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from linodemcp.tui import extras

if TYPE_CHECKING:
    from pathlib import Path


def _event(**overrides: object) -> dict[str, object]:
    """Build a representative audit event dict, overridable per test."""
    base: dict[str, object] = {
        "ts": "2026-06-14T18:30:05Z",
        "tool": "linode_instance_delete",
        "tool_capability": "destroy",
        "mode": "plan",
        "status": "success",
        "plan_id": "plan_abc",
    }
    base.update(overrides)
    return base


def test_audit_event_row_maps_fields() -> None:
    """An event maps to the row fields the audit screen shows, in order."""
    row = extras.audit_event_row(_event())
    assert row.as_cells() == (
        "2026-06-14T18:30:05Z",
        "linode_instance_delete",
        "destroy",
        "plan",
        "success",
        "plan_abc",
    )


def test_audit_event_row_null_plan_id_is_blank() -> None:
    """A null plan_id (the common case) renders as an empty cell, not 'None'."""
    row = extras.audit_event_row(_event(plan_id=None))
    assert row.plan_id == ""


def test_audit_event_row_missing_fields_blank() -> None:
    """Absent fields render blank rather than crashing the view."""
    row = extras.audit_event_row({"tool": "version"})
    assert row.tool == "version"
    assert row.timestamp == ""
    assert row.capability == ""


def test_parse_audit_events_from_feed() -> None:
    """A recent-feed payload parses into one row per event."""
    payload = json.dumps({"count": 2, "events": [_event(), _event(tool="version")]})
    rows = extras.parse_audit_events(payload)
    assert len(rows) == 2
    assert rows[1].tool == "version"


def test_parse_audit_events_empty_feed() -> None:
    """An empty feed yields no rows (the screen shows 'no events')."""
    assert extras.parse_audit_events(json.dumps({"count": 0, "events": []})) == []


def test_parse_audit_events_non_json_is_empty() -> None:
    """A non-JSON payload (e.g. an error string) yields no rows, not a crash."""
    assert extras.parse_audit_events("Error: something") == []


def test_parse_audit_events_skips_non_dict_entries() -> None:
    """Malformed entries inside the events array are skipped."""
    payload = json.dumps({"events": [_event(), "garbage", 42]})
    rows = extras.parse_audit_events(payload)
    assert len(rows) == 1


def test_audit_columns_match_row_cells() -> None:
    """The declared column count matches the row's cell count (header/row sync)."""
    row = extras.audit_event_row(_event())
    assert len(extras.AUDIT_COLUMNS) == len(row.as_cells())


def _write_config(path: Path, *, active: str | None = None) -> None:
    """Write a minimal config file, optionally with an active_profile."""
    lines = ["server:", "  name: ExtrasTest"]
    if active is not None:
        lines.append(f"active_profile: {active}")
    lines += [
        "environments:",
        "  default:",
        "    label: D",
        "    linode:",
        "      apiUrl: https://api.linode.com/v4",
        "      token: t",
    ]
    path.write_text("\n".join(lines) + "\n")


def test_profile_rows_lists_builtins_with_active(tmp_path: Path) -> None:
    """profile_rows lists the built-ins and marks the active one.

    With no active_profile set, the read-only ``default`` is active, matching
    the resolver the rest of the system uses.
    """
    config_file = tmp_path / "config.yml"
    _write_config(config_file)
    rows = extras.profile_rows(config_file)
    names = {r.name for r in rows}
    for builtin in ("default", "full-access", "compute-admin"):
        assert builtin in names
    active = [r.name for r in rows if r.active]
    assert active == ["default"]


def test_profile_rows_missing_config_uses_default(tmp_path: Path) -> None:
    """A missing config file falls back to the offline default catalog."""
    rows = extras.profile_rows(tmp_path / "absent.yml")
    assert rows
    assert any(r.active and r.name == "default" for r in rows)


def test_profile_row_cells() -> None:
    """A profile row renders marker/name/state/tools cells."""
    active_row = extras.ProfileRow(
        name="default", active=True, disabled=False, tool_count=231
    )
    assert active_row.as_cells() == ("*", "default", "enabled", "231")
    inactive = extras.ProfileRow(
        name="emergency", active=False, disabled=True, tool_count=5
    )
    assert inactive.as_cells() == ("", "emergency", "disabled", "5")


def test_switch_active_profile_writes_config(tmp_path: Path) -> None:
    """Switching writes active_profile to the config, reusing the CLI write.

    Confirms the active marker moves after the switch (the file is the source
    of truth the next read sees).
    """
    config_file = tmp_path / "config.yml"
    _write_config(config_file)

    extras.switch_active_profile(config_file, "full-access")

    rows = extras.profile_rows(config_file)
    active = [r.name for r in rows if r.active]
    assert active == ["full-access"]


def test_switch_active_profile_unknown_raises(tmp_path: Path) -> None:
    """Switching to an unknown profile raises and does not write."""
    config_file = tmp_path / "config.yml"
    _write_config(config_file, active="default")

    with pytest.raises(extras.ProfileSwitchError, match="not found"):
        extras.switch_active_profile(config_file, "no-such-profile")

    # The active profile is unchanged.
    rows = extras.profile_rows(config_file)
    assert [r.name for r in rows if r.active] == ["default"]


def test_health_rows_maps_payload() -> None:
    """A health payload maps to the labeled rows the screen renders."""
    payload = json.dumps(
        {
            "jsonl_path": "/var/log/linodemcp/audit.log",
            "active_log_exists": True,
            "rotated_file_count": 2,
            "oldest_rotated_date": "2026-06-01",
            "disk_bytes": 4096,
            "dropped_events": 0,
            "sqlite": None,
        }
    )
    rows = {r.label: r.value for r in extras.health_rows(payload)}
    assert rows["jsonl path"] == "/var/log/linodemcp/audit.log"
    assert rows["disk bytes"] == "4096"
    assert rows["dropped events"] == "0"
    assert rows["sqlite"] == "disabled"


def test_health_rows_sqlite_enabled_with_count() -> None:
    """An enabled SQLite block summarizes its row count."""
    payload = json.dumps({"sqlite": {"event_count": 17}})
    rows = {r.label: r.value for r in extras.health_rows(payload)}
    assert rows["sqlite"] == "enabled (17 rows)"


def test_health_rows_non_json_is_single_error_row() -> None:
    """A non-JSON health payload yields one error row, never a blank screen."""
    rows = extras.health_rows("Error: audit unavailable")
    assert len(rows) == 1
    assert "unavailable" in rows[0].value


def test_version_rows_maps_build_info() -> None:
    """The version info maps to the build rows the health screen shows."""
    info = {
        "version": "0.1.0",
        "git_commit": "abc123",
        "git_branch": "main",
        "python_version": "3.13.0",
        "platform": "Linux/x86_64",
        "features": {"tools": "..."},
    }
    rows = {r.label: r.value for r in extras.version_rows(info)}
    assert rows["version"] == "0.1.0"
    assert rows["git commit"] == "abc123"
    assert rows["platform"] == "Linux/x86_64"
    # The nested features block is not surfaced here.
    assert "features" not in rows


def test_metrics_pointer_enabled() -> None:
    """The metrics pointer gives the endpoint URL and notes it needs serve."""
    pointer = extras.metrics_pointer(enabled=True, port=8888, path="/metrics")
    assert "http://localhost:8888/metrics" in pointer
    assert "serve" in pointer


def test_metrics_pointer_disabled() -> None:
    """When metrics are disabled, the pointer says so instead of a URL."""
    pointer = extras.metrics_pointer(enabled=False, port=8888, path="/metrics")
    assert "disabled" in pointer
    assert "http://" not in pointer
