"""Entry point for ``linodemcp tui``.

Builds the session runtime (server held open), runs the Textual app against it,
and tears the runtime down on exit. ``main`` calls ``run_tui``; the async body
is separate so the same app can be driven from a test (or a future embedding)
without owning the event loop here.
"""

from __future__ import annotations

import asyncio
from typing import TYPE_CHECKING

from linodemcp.tui.app import LinodeTUI
from linodemcp.tui.runtime import open_tui_runtime

if TYPE_CHECKING:
    from pathlib import Path


async def _run_tui_async(config_path: Path | None) -> int:
    """Open the runtime and run the app for the session.

    The runtime stays open for the whole ``app.run_async`` call, so every
    screen dispatches against the one server. Returns 0 on a clean exit.
    """
    async with open_tui_runtime(config_path) as runtime:
        app = LinodeTUI(runtime)
        await app.run_async()
    return 0


def run_tui(config_path: Path | None = None) -> int:
    """Launch the TUI. Synchronous entry that owns the event loop.

    A missing config file is fine: the runtime falls back to the offline
    default so the catalog browses without a config, matching the CLI. Returns
    the process exit code.
    """
    return asyncio.run(_run_tui_async(config_path))
