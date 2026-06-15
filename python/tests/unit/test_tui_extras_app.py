"""Pilot-harness tests for the Phase 3 TUI screens (audit / profile / health).

These drive the app through Textual's ``run_test`` pilot to confirm the screens
open, render their data from the shared dispatch / config, and that the profile
switch updates the live server. Not pixel rendering, the data flow and the
navigation.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

import pytest
from textual.widgets import DataTable, Static

from linodemcp.tui.app import (
    AuditScreen,
    CatalogScreen,
    HealthScreen,
    LinodeTUI,
    OpenAudit,
    OpenHealth,
    OpenProfiles,
    ProfileScreen,
)
from linodemcp.tui.runtime import TuiRuntime, open_tui_runtime

if TYPE_CHECKING:
    from pathlib import Path

    from textual.pilot import Pilot
    from textual.screen import Screen

# A test screen size large enough for the tables to render rows.
_TEST_SIZE = (140, 50)


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Point the audit log at a temp dir for the session."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def _runtime(tmp_path: Path) -> TuiRuntime:
    """Build a TUI runtime on the offline default (no config file present)."""
    return TuiRuntime.create(tmp_path / "absent.yml")


def _write_config(path: Path) -> None:
    """Write a minimal config file (no active_profile, so default is active)."""
    path.write_text(
        "server:\n"
        "  name: ExtrasApp\n"
        "environments:\n"
        "  default:\n"
        "    label: D\n"
        "    linode:\n"
        "      apiUrl: https://api.linode.com/v4\n"
        "      token: t\n"
    )


def _table(screen: Screen[Any], selector: str) -> DataTable[str]:
    """Fetch a screen's DataTable by id, narrowed for the strict checkers.

    ``query_one`` with a bare ``DataTable`` returns ``DataTable[Unknown]``; the
    cast pins the cell type without the subscripted-generic ``isinstance`` that
    fails at runtime. The ``Screen[Any]`` parameter sidesteps the invariant
    ``ScreenResultType`` so any concrete screen subclass can be passed.
    """
    return cast("DataTable[str]", screen.query_one(selector, DataTable))


async def _select_profile(pilot: Pilot[None], screen: ProfileScreen, name: str) -> None:
    """Move the cursor to ``name``'s row and select it to switch profiles."""
    idx = screen.row_for_profile(name)
    assert idx is not None, f"profile {name!r} not listed"
    _table(screen, "#profiles").move_cursor(row=idx)
    await pilot.pause()
    await pilot.press("enter")
    await pilot.pause()
    await pilot.pause()


async def test_audit_screen_shows_events(tmp_path: Path) -> None:
    """Opening the audit screen shows the events the session dispatched.

    The runtime is opened (audit sink attached) so a dispatched ``version`` call
    is recorded on disk; the audit screen, driven through the same dispatch with
    include_meta, then shows that event.
    """
    async with open_tui_runtime(tmp_path / "absent.yml") as runtime:
        await runtime.server.dispatch("version", {})
        app = LinodeTUI(runtime)
        async with app.run_test(size=_TEST_SIZE) as pilot:
            await pilot.pause()
            app.post_message(OpenAudit())
            await pilot.pause()
            await pilot.pause()
            assert isinstance(app.screen, AuditScreen)
            assert _table(app.screen, "#audit").row_count >= 1


async def test_audit_screen_refresh(tmp_path: Path) -> None:
    """The ``r`` key reloads the audit feed and picks up new events.

    After opening, another call is dispatched; refreshing shows more rows than
    the first load.
    """
    async with open_tui_runtime(tmp_path / "absent.yml") as runtime:
        await runtime.server.dispatch("version", {})
        app = LinodeTUI(runtime)
        async with app.run_test(size=_TEST_SIZE) as pilot:
            await pilot.pause()
            app.post_message(OpenAudit())
            await pilot.pause()
            await pilot.pause()
            assert isinstance(app.screen, AuditScreen)
            table = _table(app.screen, "#audit")
            before = table.row_count
            await runtime.server.dispatch("hello", {"name": "x"})
            await pilot.press("r")
            await pilot.pause()
            await pilot.pause()
            assert table.row_count > before


async def test_profile_screen_lists_and_switches(tmp_path: Path) -> None:
    """The profile screen lists profiles and switching updates the live server.

    Selecting ``full-access`` writes the config and reloads the running server,
    so its active profile and allow-list change in the session.
    """
    config_file = tmp_path / "config.yml"
    _write_config(config_file)
    runtime = TuiRuntime.create(config_file)
    assert runtime.server.active_profile.name == "default"

    app = LinodeTUI(runtime)
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        app.post_message(OpenProfiles())
        await pilot.pause()
        await pilot.pause()
        screen = app.screen
        assert isinstance(screen, ProfileScreen)
        assert _table(screen, "#profiles").row_count >= 8  # the built-in profiles
        await _select_profile(pilot, screen, "full-access")

    # The live server reloaded to the switched profile.
    assert runtime.server.active_profile.name == "full-access"


async def test_profile_switch_persists_to_config(tmp_path: Path) -> None:
    """A switch writes active_profile to the config file (human-only op)."""
    from linodemcp.config import load_from_file

    config_file = tmp_path / "config.yml"
    _write_config(config_file)
    runtime = TuiRuntime.create(config_file)

    app = LinodeTUI(runtime)
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        app.post_message(OpenProfiles())
        await pilot.pause()
        await pilot.pause()
        screen = app.screen
        assert isinstance(screen, ProfileScreen)
        await _select_profile(pilot, screen, "compute-admin")

    written = load_from_file(config_file)
    assert written.active_profile == "compute-admin"


async def test_health_screen_renders_rows_and_pointer(tmp_path: Path) -> None:
    """The health screen shows audit-health rows, version rows, and the pointer."""
    runtime = _runtime(tmp_path)
    app = LinodeTUI(runtime)
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        app.post_message(OpenHealth())
        await pilot.pause()
        await pilot.pause()
        assert isinstance(app.screen, HealthScreen)
        # 7 health rows + a blank spacer + 5 version rows.
        assert _table(app.screen, "#health").row_count >= 12
        pointer = app.screen.query_one("#metrics-pointer", Static)
        assert "metrics" in str(pointer.render())


async def test_catalog_reflects_switch_on_resume(tmp_path: Path) -> None:
    """Returning to the catalog after a switch reflects the new profile surface.

    Switching from the read-only default to full-access widens the catalog's
    profile-filtered surface; the catalog re-resolves on resume.
    """
    config_file = tmp_path / "config.yml"
    _write_config(config_file)
    runtime = TuiRuntime.create(config_file)

    app = LinodeTUI(runtime)
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        catalog = app.screen
        assert isinstance(catalog, CatalogScreen)
        before_count = _table(catalog, "#catalog").row_count

        app.post_message(OpenProfiles())
        await pilot.pause()
        await pilot.pause()
        switcher = app.screen
        assert isinstance(switcher, ProfileScreen)
        await _select_profile(pilot, switcher, "full-access")

        # Back to the catalog, which re-resolves the now-wider surface.
        await pilot.press("escape")
        await pilot.pause()
        await pilot.pause()
        assert isinstance(app.screen, CatalogScreen)
        after_count = _table(app.screen, "#catalog").row_count
        assert after_count > before_count
