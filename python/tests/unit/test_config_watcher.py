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

    from linodemcp.config import Config

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


def test_non_positive_interval_is_accepted(tmp_path: Path) -> None:
    """A zero interval is accepted rather than rejected. The constructor
    clamps a non-positive cadence to the default internally so polling never
    becomes a busy loop; construction succeeds and the config is available."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "X")

    watcher = ConfigWatcher(cfg_path, interval=0)

    assert watcher.get().server.name == "X"


async def test_start_is_idempotent_while_running(tmp_path: Path) -> None:
    """A second start() while already polling is a no-op: it hits the
    already-running guard, does not raise, and the watcher keeps serving its
    config and still stops cleanly."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "X")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    watcher.start()
    watcher.start()

    try:
        assert watcher.get().server.name == "X"
    finally:
        await watcher.stop()


async def test_reload_skips_when_stat_fails(tmp_path: Path) -> None:
    """A stat failure during polling is logged and leaves the config intact."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "Persisted")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    watcher.start()

    try:
        cfg_path.unlink()  # stat() inside _check_and_reload now raises OSError

        for _ in range(10):
            await asyncio.sleep(POLL_INTERVAL)

        assert watcher.get().server.name == "Persisted", (
            "a stat failure must not blank out the cached config"
        )
    finally:
        await watcher.stop()


async def test_sync_on_change_callback_fires_after_reload(tmp_path: Path) -> None:
    """A registered sync callback runs with the freshly reloaded config."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "Original")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    received: list[str] = []
    watcher.set_on_change(lambda cfg: received.append(cfg.server.name))
    watcher.start()

    try:
        _write_config(cfg_path, "Reloaded")
        _bump_mtime(cfg_path)

        for _ in range(40):
            await asyncio.sleep(POLL_INTERVAL)
            if received:
                break

        assert received == ["Reloaded"]
    finally:
        await watcher.stop()


async def test_async_on_change_callback_is_awaited(tmp_path: Path) -> None:
    """An async callback is awaited, not left as an un-awaited coroutine."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "Original")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)
    received: list[str] = []

    async def _record(cfg: Config) -> None:
        received.append(cfg.server.name)

    watcher.set_on_change(_record)
    watcher.start()

    try:
        _write_config(cfg_path, "AsyncReloaded")
        _bump_mtime(cfg_path)

        for _ in range(40):
            await asyncio.sleep(POLL_INTERVAL)
            if received:
                break

        assert received == ["AsyncReloaded"]
    finally:
        await watcher.stop()


async def test_on_change_callback_error_does_not_stop_watcher(
    tmp_path: Path,
) -> None:
    """A raising callback is caught; the reload still swaps in the new config."""
    cfg_path = tmp_path / "config.yml"
    _write_config(cfg_path, "Original")

    watcher = ConfigWatcher(cfg_path, interval=POLL_INTERVAL)

    def _boom(cfg: Config) -> None:
        msg = "callback failed"
        raise RuntimeError(msg)

    watcher.set_on_change(_boom)
    watcher.start()

    try:
        _write_config(cfg_path, "Reloaded")
        _bump_mtime(cfg_path)

        for _ in range(40):
            await asyncio.sleep(POLL_INTERVAL)
            if watcher.get().server.name == "Reloaded":
                break

        assert watcher.get().server.name == "Reloaded", (
            "the reload must still land even when the callback raises"
        )
    finally:
        await watcher.stop()
