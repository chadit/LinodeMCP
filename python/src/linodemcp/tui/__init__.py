"""Interactive TUI for LinodeMCP (Phase 2).

``linodemcp tui`` launches a full-screen Textual app over the same dispatch the
Phase 1 CLI uses. The shell has three screens: a searchable, profile-filtered
catalog; a tool form built from the selected tool's input schema with the
safety controls; and a result view that runs the call through the shared
dispatch and renders the output.

The package is split so the logic is testable without a terminal: ``model``
(catalog, filtering, form fields, form-to-arguments) and ``run`` (dispatch +
classify) are pure modules with unit tests, while ``app`` holds the Textual
views and ``runtime`` holds the session-lived server. ``run_tui`` is the entry
point ``main`` dispatches to.
"""

from __future__ import annotations

from linodemcp.tui.runner import run_tui

__all__ = ["run_tui"]
