"""Unit tests for the config hot-reload watcher."""

from __future__ import annotations

import asyncio
import os
import textwrap
from typing import TYPE_CHECKING

import pytest

from linodemcp.config.watcher import ConfigWatcher

if TYPE_CHECKING:
    from pathlib import Path

POLL_INTERVAL = 0.05


def _write_config(path: Path, server_name: str) -> None:
    """Write a minimal valid config with the given server name."""
    path.write_text(
        textwrap.dedent(f"""
            server:
              name: "{server_name}"
              logLevel: "info"
            environments:
              default:
                label: "Default"
                linode:
                  apiUrl: "https://api.linode.com/v4"
                  token: "tok"
        """).strip(),
        encoding="utf-8",
    )


def _bump_mtime(path: Path) -> None:
    """Push the file mtime forward enough that the next poll detects it.

    Necessary because POSIX mtimes are second-granularity on some
    filesystems; calling write_text immediately followed by stat may
    return the same mtime as the initial load.
    """
    future = path.stat().st_mtime + 2.0
    os.utime(path, (future, future))


async def test_watcher_initial_load(tmp_path: Path) -> None:
    """get() returns the initially loaded config before any polling."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "InitialServer")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    assert watcher.get().server.name == "InitialServer"


async def test_watcher_picks_up_reload(tmp_path: Path) -> None:
    """A file change visible to mtime triggers a reload within the poll cadence."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "Original")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    watcher.start()

    try:
        _write_config(cfg_path, "Reloaded")
        _bump_mtime(cfg_path)

        # Wait up to ~10x the poll interval for the reload to land.
        for _ in range(20):
            await asyncio.sleep(POLL_INTERVAL)
            if watcher.get().server.name == "Reloaded":
                return

        pytest.fail("watcher did not reload within deadline")
    finally:
        await watcher.stop()


async def test_watcher_keeps_last_config_on_bad_reload(tmp_path: Path) -> None:
    """A bad file leaves the previous Config in place rather than blanking."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "GoodConfig")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    original = watcher.get()
    watcher.start()

    try:
        # Garbage that fails parse.
        cfg_path.write_text("not: valid: yaml: ::: ", encoding="utf-8")
        _bump_mtime(cfg_path)

        # Give the watcher a few cycles to attempt and fail the reload.
        for _ in range(10):
            await asyncio.sleep(POLL_INTERVAL)

        current = watcher.get()
        assert current.server.name == original.server.name, (
            "bad reload must not overwrite the cached config"
        )
    finally:
        await watcher.stop()


async def test_watcher_stop_is_idempotent(tmp_path: Path) -> None:
    """Calling stop() multiple times is safe."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "X")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    watcher.start()
    await watcher.stop()
    # Second stop should not raise.
    await watcher.stop()
