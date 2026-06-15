"""Session-lived server runtime for the TUI.

The Phase 1 CLI builds a server per one-shot command. The TUI is the opposite:
it builds the server once and holds it open for the whole interactive session,
dispatching many calls against it, then closes the audit sinks on exit. This
wrapper provides that lifecycle as an async context manager.

It reuses the Phase 1 building blocks rather than re-deriving them: the same
``load_config_or_default`` (so the TUI launches without a config file), the
same audit-sink wiring (so a call made in the TUI lands in the audit log like
any MCP call), and the same ``Server`` whose ``dispatch`` every screen drives.
"""

from __future__ import annotations

import contextlib
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit import (
    JSONLSink,
    MultiSink,
    SQLiteSink,
    resolve_default_audit_dir,
)
from linodemcp.cli.runtime import load_config_or_default
from linodemcp.config import Config, get_config_path
from linodemcp.server import Server

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator
    from types import TracebackType


class TuiRuntime:
    """Build, hold open, and tear down a ``Server`` for a TUI session.

    Constructed once at launch; ``server`` is reused by every screen for the
    life of the app. ``__aenter__`` attaches the audit sinks (JSONL always on,
    SQLite when enabled) so TUI calls are recorded; ``__aexit__`` closes them.
    No metrics endpoint, config watcher, or retention sweep runs, the same as
    the one-shot CLI runtime.
    """

    def __init__(self, config: Config, config_path: Path) -> None:
        self._config = config
        self._config_path = config_path
        self._server = Server(config)
        self._jsonl_sink: JSONLSink | None = None
        self._sqlite_sink: SQLiteSink | None = None

    @classmethod
    def create(cls, config_path: Path | None = None) -> TuiRuntime:
        """Load the config (or the offline default) and build the runtime.

        A missing config file falls back to the in-memory default so the
        catalog browses offline and Linode-API tools fail at call time with the
        clear no-environment message, matching the Phase 1 CLI. The resolved
        path is retained so the profile switcher can rewrite ``active_profile``
        to the same file the rest of the TUI read from.
        """
        path = config_path if config_path is not None else get_config_path()
        config = load_config_or_default(path)
        return cls(config, path)

    @property
    def server(self) -> Server:
        """The open server. Drive ``server.dispatch`` to run a tool."""
        return self._server

    @property
    def config(self) -> Config:
        """The config the server was built from (its environments feed the
        form's environment picker)."""
        return self._config

    @property
    def config_path(self) -> Path:
        """The resolved config-file path (where the profile switcher writes)."""
        return self._config_path

    async def reload_profile_from_disk(self) -> None:
        """Re-read the config file and swap the live server's active profile.

        Called after the profile switcher rewrites ``active_profile``. Reuses
        ``Server.reload_profile`` (the same atomic swap the long-running server
        uses on a config change), so the running session reflects the new
        profile without a relaunch. The retained ``config`` is refreshed too so
        later reads (e.g. the environment picker) see the current file.
        """
        new_config = load_config_or_default(self._config_path)
        await self._server.reload_profile(new_config)
        self._config = new_config

    async def __aenter__(self) -> TuiRuntime:
        """Attach the audit sinks for the session."""
        self._attach_audit()
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Close the audit sinks on session exit."""
        if self._sqlite_sink is not None:
            self._sqlite_sink.close()
        if self._jsonl_sink is not None:
            with contextlib.suppress(OSError):
                self._jsonl_sink.close()

    def _attach_audit(self) -> None:
        """Open the sinks and wire them into the server.

        On a JSONL open failure the server keeps its NoopSink default; audit
        never blocks the session. Mirrors the one-shot CLI runtime so a TUI
        call and a CLI call write the same audit record shape.
        """
        try:
            jsonl_sink = JSONLSink(resolve_default_audit_dir())
        except OSError:
            return

        self._jsonl_sink = jsonl_sink
        cfg = self._config
        sqlite_sink = (
            self._open_sqlite_sink(jsonl_sink) if cfg.audit.sqlite.enabled else None
        )
        self._sqlite_sink = sqlite_sink

        audit_sink = MultiSink(jsonl_sink, sqlite_sink) if sqlite_sink else jsonl_sink
        self._server.set_audit_sink(audit_sink)
        self._server.set_audit_redact_pii(cfg.audit.redact_pii)

    def _open_sqlite_sink(self, jsonl_sink: JSONLSink) -> SQLiteSink | None:
        """Open the SQLite sink beside the JSONL log, or None on failure."""
        cfg = self._config
        db_path = cfg.audit.sqlite.path or str(
            Path(jsonl_sink.path).parent / "audit.db"
        )
        try:
            return SQLiteSink(db_path, cfg.audit.sqlite.busy_timeout_ms)
        except Exception:
            return None


@contextlib.asynccontextmanager
async def open_tui_runtime(
    config_path: Path | None = None,
) -> AsyncGenerator[TuiRuntime]:
    """Async context manager that builds and tears down a ``TuiRuntime``."""
    runtime = TuiRuntime.create(config_path)
    async with runtime:
        yield runtime
