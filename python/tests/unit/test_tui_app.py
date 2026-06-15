"""Pilot-harness navigation tests for the TUI app.

These drive the app through Textual's ``run_test`` pilot to exercise navigation
and the form-to-run wiring, not pixel rendering. The catalog filters and
toggles, a selection opens the form, and a Run opens the result screen with the
dispatched output. A temp config and temp audit dir keep it offline and
isolated.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest
from textual.widgets import Input, Label

from linodemcp.profiles import Capability
from linodemcp.tui.app import (
    CatalogScreen,
    FormScreen,
    LinodeTUI,
    OpenForm,
    ResultScreen,
)
from linodemcp.tui.model import CatalogEntry
from linodemcp.tui.runtime import TuiRuntime

if TYPE_CHECKING:
    from pathlib import Path

# A test screen size large enough that the form's Run button is on-screen for
# the pilot to click.
_TEST_SIZE = (120, 50)


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Point the audit log at a temp dir for the session."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def _runtime(tmp_path: Path) -> TuiRuntime:
    """Build a TUI runtime on the offline default (no config file present)."""
    return TuiRuntime.create(tmp_path / "absent.yml")


def _subtitle(screen: CatalogScreen) -> str:
    """Return the catalog screen's subtitle, asserting it has been set.

    ``Screen.sub_title`` is typed ``str | None``; the catalog always sets it on
    load, so this narrows it for the assertions without an inline guard at every
    call site.
    """
    title = screen.sub_title
    assert isinstance(title, str)
    return title


async def test_app_launches_on_catalog(tmp_path: Path) -> None:
    """Launching the app lands on the catalog screen with the profile surface."""
    app = LinodeTUI(_runtime(tmp_path))
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        assert isinstance(app.screen, CatalogScreen)
        assert "active profile" in _subtitle(app.screen)


async def test_catalog_toggle_full_surface(tmp_path: Path) -> None:
    """Pressing ``f`` toggles between the profile surface and the full registry.

    The table is focused on mount so the binding fires; the subtitle and count
    flip between the two surfaces.
    """
    app = LinodeTUI(_runtime(tmp_path))
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        screen = app.screen
        assert isinstance(screen, CatalogScreen)
        profile_title = _subtitle(screen)
        await pilot.press("f")
        await pilot.pause()
        assert "full registry" in _subtitle(screen)
        assert _subtitle(screen) != profile_title
        await pilot.press("f")
        await pilot.pause()
        assert "active profile" in _subtitle(screen)


async def test_catalog_filter_narrows_list(tmp_path: Path) -> None:
    """Typing in the filter narrows the catalog to matching tools."""
    app = LinodeTUI(_runtime(tmp_path))
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        screen = app.screen
        assert isinstance(screen, CatalogScreen)
        full_title = _subtitle(screen)
        await pilot.click("#filter")
        await pilot.press(*"version")
        await pilot.pause()
        assert _subtitle(screen) != full_title
        # The filter box holds what we typed.
        assert screen.query_one("#filter", Input).value == "version"


async def test_selection_opens_form(tmp_path: Path) -> None:
    """Posting a catalog selection opens the tool form for that tool."""
    app = LinodeTUI(_runtime(tmp_path))
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        entry = CatalogEntry(name="hello", capability=Capability.Meta, category="core")
        app.post_message(OpenForm(entry))
        await pilot.pause()
        await pilot.pause()
        assert isinstance(app.screen, FormScreen)
        # The form rendered the 'name' field for the hello tool.
        assert app.screen.query_one("#field-name", Input) is not None


async def test_form_run_opens_result_with_output(tmp_path: Path) -> None:
    """Filling the form and pressing Run opens the result screen with output.

    Drives the full catalog -> form -> run -> result path through the shared
    dispatch; the result screen shows the meta tool's payload.
    """
    app = LinodeTUI(_runtime(tmp_path))
    async with app.run_test(size=_TEST_SIZE) as pilot:
        await pilot.pause()
        entry = CatalogEntry(name="hello", capability=Capability.Meta, category="core")
        app.post_message(OpenForm(entry))
        await pilot.pause()
        await pilot.pause()
        assert isinstance(app.screen, FormScreen)
        app.screen.query_one("#field-name", Input).value = "Ada"
        await pilot.pause()
        await pilot.click("#run")
        await pilot.pause()
        await pilot.pause()
        assert isinstance(app.screen, ResultScreen)
        status = app.screen.query_one("#status", Label)
        # The status line reflects a successful meta-tool run.
        assert "OK" in str(status.render())
