"""One-shot server runtime for the non-interactive CLI.

The CLI subcommands (`call`, `audit`) need a fully-wired ``Server`` so they
can drive ``Server.dispatch`` exactly like an MCP request does. That gets the
call the same audit middleware, profile filter, and dry-run/two-stage engine
the stdio server applies, with no per-tool CLI code.

What this does NOT do, deliberately: it never starts the OpenTelemetry/metrics
HTTP server, the config watcher, or the retention sweep loops. A one-shot
command builds the server, attaches the audit sink so the call lands on disk,
runs once, and tears down. The long-running concerns belong to ``serve`` (the
stdio server in ``main.async_main``), not to a single shell invocation.

Audit wiring mirrors ``main._start_audit`` but without the background tasks:
the JSONL sink is always on, the SQLite sink joins it behind a ``MultiSink``
when ``audit.sqlite.enabled``. The summary/report query tools are pointed at
the same paths so ``linodemcp audit ...`` reads what ``linodemcp call ...``
just wrote.
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
from linodemcp.config import (
    Config,
    ConfigFileNotFoundError,
    get_config_path,
    load_from_file,
)
from linodemcp.server import Server
from linodemcp.tools.linode_audit_report import set_audit_reports
from linodemcp.tools.linode_audit_summary import set_audit_sqlite_path

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator
    from types import TracebackType


def load_config_or_default(path: Path) -> Config:
    """Load the config from ``path`` or return the in-memory CLI default.

    A missing file is the only error swallowed: it yields ``Config()``, whose
    dataclass defaults match what an empty config file produces (empty
    ``active_profile`` so the resolver selects the read-only ``default``
    profile, no environments, audit defaults intact). This mirrors Go's
    ``defaultCLIConfig`` / ``loadConfigOrDefault`` so both CLIs run meta-tool
    calls offline. A malformed or unreadable config still propagates so the
    user sees the real problem instead of a silent fallback.
    """
    try:
        return load_from_file(path)
    except ConfigFileNotFoundError:
        return Config()


class OneShotRuntime:
    """Build, hold, and tear down a ``Server`` for a single CLI command.

    Use it as an async context manager::

        async with OneShotRuntime.create() as runtime:
            result = await runtime.server.dispatch(name, arguments)

    ``create`` loads the config from the standard path (or an override) and
    constructs the server. ``__aenter__`` attaches the audit sinks; ``__aexit__``
    closes them. Server construction can raise the profile-resolution errors
    (``ActiveProfileUnknownError`` etc.); callers catch those and turn them
    into a usage error.
    """

    def __init__(self, config: Config) -> None:
        self._config = config
        self._server = Server(config)
        self._jsonl_sink: JSONLSink | None = None
        self._sqlite_sink: SQLiteSink | None = None

    @classmethod
    def create(cls, config_path: Path | None = None) -> OneShotRuntime:
        """Load the config and build the runtime without opening sinks yet.

        ``config_path`` overrides the standard path (tests pass a temp file).
        A missing config file is not fatal: the runtime falls back to an
        in-memory default (read-only ``default`` profile active, no
        environments) so tool discovery and meta-tool calls work offline, the
        same way the Go CLI does. Linode-API tools then fail at call time with
        a clear no-environment message rather than the whole command dying at
        load. The audit sinks open in ``__aenter__`` so a construction failure
        does not leave a half-open log file behind.
        """
        path = config_path if config_path is not None else get_config_path()
        config = load_config_or_default(path)
        return cls(config)

    @property
    def server(self) -> Server:
        """The wired server. Drive ``server.dispatch`` to run a tool."""
        return self._server

    @property
    def config(self) -> Config:
        """The config the server was built from."""
        return self._config

    async def __aenter__(self) -> OneShotRuntime:
        """Attach the audit sinks so a CLI call is recorded like an MCP call.

        JSONL is always on; SQLite joins it when enabled. On a JSONL open
        failure the server keeps its NoopSink default (audit never blocks a
        command), matching ``main._start_audit``.
        """
        self._attach_audit()
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Close the audit sinks. sqlite3 close never raises; JSONL close can,
        so it is suppressed (the command result already printed by now)."""
        if self._sqlite_sink is not None:
            self._sqlite_sink.close()
        if self._jsonl_sink is not None:
            with contextlib.suppress(OSError):
                self._jsonl_sink.close()

    def _attach_audit(self) -> None:
        """Open the sinks and wire them into the server and the query tools."""
        audit_dir = resolve_default_audit_dir()
        try:
            jsonl_sink = JSONLSink(audit_dir)
        except OSError:
            # Audit must never block a command; fall back to the NoopSink the
            # server already holds.
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

        set_audit_reports(cfg.audit.reports)

    def _open_sqlite_sink(self, jsonl_sink: JSONLSink) -> SQLiteSink | None:
        """Open the SQLite sink and point the summary query tool at it.

        Returns None on failure; the JSONL sink stays the durable record.
        Mirrors ``main._open_sqlite_sink`` minus the logging (a one-shot
        command has no structured logger wired).
        """
        cfg = self._config
        db_path = cfg.audit.sqlite.path or str(
            Path(jsonl_sink.path).parent / "audit.db"
        )
        try:
            sink = SQLiteSink(db_path, cfg.audit.sqlite.busy_timeout_ms)
        except Exception:
            return None

        set_audit_sqlite_path(db_path)
        return sink


@contextlib.asynccontextmanager
async def open_runtime(
    config_path: Path | None = None,
) -> AsyncGenerator[OneShotRuntime]:
    """Convenience async context manager around ``OneShotRuntime.create``.

    Keeps the call site to a single ``async with`` even though construction
    (``create``) and sink attachment (``__aenter__``) are separate steps.
    """
    runtime = OneShotRuntime.create(config_path)
    async with runtime:
        yield runtime
