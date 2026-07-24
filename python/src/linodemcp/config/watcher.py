"""Config hot-reload via mtime polling."""

from __future__ import annotations

import asyncio
import contextlib
import inspect
import logging
from collections.abc import Awaitable, Callable
from typing import TYPE_CHECKING

from linodemcp.config import ConfigError, load_from_file

if TYPE_CHECKING:
    from pathlib import Path

    from linodemcp.config import Config

logger = logging.getLogger(__name__)

OnChangeCallback = Callable[["Config"], Awaitable[None] | None]

# Default polling cadence. 5 seconds balances responsiveness with
# filesystem load.
DEFAULT_WATCH_INTERVAL = 5.0


class ConfigWatcher:
    """Hot-reloads a Config from disk by polling the file's mtime.

    The current Config is held atomically (single-attribute swap on the
    asyncio event loop is safe without locks). New configs are validated
    before being swapped in; a bad reload leaves the previous Config in
    place and logs the error rather than blanking out get().

    Tokens, API URLs, and other environment-scoped fields are NOT
    special-cased: a reload swaps the whole Config. Operators who don't
    want a field to change at runtime should keep it out of the file (use
    env-var overrides), since those are only read once at startup.
    """

    def __init__(self, path: Path, interval: float = DEFAULT_WATCH_INTERVAL) -> None:
        if interval <= 0:
            interval = DEFAULT_WATCH_INTERVAL
        self._path = path
        self._interval = interval
        # Initial load: blocks if the file is unreadable. Caller decides
        # how to handle ConfigError.
        self._current: Config = load_from_file(path)
        self._last_mtime = path.stat().st_mtime
        self._task: asyncio.Task[None] | None = None
        self._stop = asyncio.Event()
        self._on_change: OnChangeCallback | None = None

    def get(self) -> Config:
        """Return the latest Config snapshot."""
        return self._current

    def set_on_change(self, callback: OnChangeCallback | None) -> None:
        """Register a callback fired after each successful reload.

        Accepts sync or async callables; async callbacks are awaited.
        Subscribers that do non-trivial work should still hand off to a
        separate task to avoid blocking the polling loop. Passing ``None``
        clears the callback. The initial Config from ``__init__`` does not
        trigger the callback; only post-startup reloads do.
        """
        self._on_change = callback

    def start(self) -> None:
        """Begin polling the file mtime in a background task.

        Idempotent. Subsequent calls are no-ops while the watcher is
        running.
        """
        if self._task is not None and not self._task.done():
            return
        self._stop.clear()
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        """Cancel the background polling task and wait for it to exit."""
        self._stop.set()
        if self._task is not None:
            with contextlib.suppress(asyncio.CancelledError):
                await self._task
            self._task = None

    async def _run(self) -> None:
        """Poll mtime; reload + swap on change."""
        while not self._stop.is_set():
            try:
                await asyncio.wait_for(self._stop.wait(), timeout=self._interval)
                # If wait_for returned without timeout, stop was set.
                return
            except TimeoutError:
                # Normal path: timeout means it's time to poll.
                await self._check_and_reload()

    async def _check_and_reload(self) -> None:
        try:
            mtime = self._path.stat().st_mtime
        except OSError as exc:
            logger.warning("config watcher stat failed: %s", exc)
            return

        if mtime <= self._last_mtime:
            return

        try:
            new_cfg = load_from_file(self._path)
        except ConfigError as exc:
            # Don't update _last_mtime: leave the bad file flagged so the
            # next poll re-attempts and re-reports.
            logger.warning("config reload failed: %s", exc)
            return

        self._last_mtime = mtime
        self._current = new_cfg
        logger.info("config reloaded from %s", self._path)

        if self._on_change is not None:
            try:
                result = self._on_change(new_cfg)
                if inspect.isawaitable(result):
                    await result
            except Exception:
                # Callback failure must not poison the watcher loop.
                # Phase 5 subscribers (Server.reload_profile) log their
                # own errors; this block is the last-resort net.
                logger.exception("on_change callback raised")
