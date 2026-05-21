"""Main entry point for LinodeMCP server."""

import asyncio
import contextlib
import logging
import sys
from pathlib import Path

import structlog

from linodemcp.audit import (
    JSONLSink,
    MultiSink,
    RetentionSweeper,
    SQLiteSink,
    resolve_default_audit_dir,
)
from linodemcp.cli import run_profile_command
from linodemcp.config import Config, ConfigError, get_config_path
from linodemcp.config.watcher import ConfigWatcher
from linodemcp.observability import Observability
from linodemcp.profiles import (
    TokenNotConfiguredError,
    profile_is_elevated,
)
from linodemcp.server import Server
from linodemcp.tools import helpers as tool_helpers
from linodemcp.version import get_version_info

# Number of positional arguments required before sys.argv[1] is safe to
# index. Extracted so the magic-number check passes and the guard is
# self-documenting in main().
_MIN_ARGV_FOR_SUBCOMMAND = 2

# Bootstrap logger for startup. The Observability constructor reconfigures
# structlog once it knows the configured level/format, so this is just for
# any output before that happens.
structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.StackInfoRenderer(),
        structlog.dev.set_exc_info,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(logging.INFO),
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=False,
)

logger = structlog.get_logger(__name__)


async def _run_scope_validation(
    server: Server,
    log: structlog.stdlib.BoundLogger,
) -> bool:
    """Run Phase 6.4 token-scope validation and apply policy.

    Returns True to continue startup, False to abort. Policy:
      - Missing required scopes: always fail.
      - Excess scopes: warn (least-privilege signal), continue.
      - API failure or no-token configured: fail for elevated profiles,
        warn for read-only ones.
    """
    active = server.active_profile
    elevated = profile_is_elevated(active)

    try:
        result = await server.validate_scopes()
    except TokenNotConfiguredError as exc:
        if elevated:
            log.exception(
                "profile requires a Linode token; "
                "configure environments.<env>.linode.token",
                profile=active.name,
                error=str(exc),
            )
            return False
        log.warning(
            "no Linode token configured; read-only profile starts but "
            "API calls will fail",
            profile=active.name,
        )
        return True
    except Exception as exc:
        if elevated:
            log.exception(
                "token-scope validation failed; profile requires write "
                "access so refusing to start",
                profile=active.name,
                error=str(exc),
            )
            return False
        log.warning(
            "token-scope validation failed; read-only profile continues "
            "without verified token",
            profile=active.name,
            error=str(exc),
        )
        return True

    if result.comparison.has_missing:
        log.error(
            "active token is missing scopes the profile requires; refusing to start",
            profile=active.name,
            token_kind=result.kind.name,
            missing=[s.value for s in result.comparison.missing],
        )
        return False

    if result.comparison.has_excess:
        log.warning(
            "active token carries more scopes than the profile requires "
            "(least-privilege violated)",
            profile=active.name,
            token_kind=result.kind.name,
            excess=[s.value for s in result.comparison.excess],
        )

    log.info(
        "token-scope validation passed",
        profile=active.name,
        token_kind=result.kind.name,
        username=result.profile.username,
    )
    return True


def _start_audit(
    server: Server,
    cfg: Config,
    log: structlog.stdlib.BoundLogger,
) -> tuple[JSONLSink | None, SQLiteSink | None, asyncio.Task[None] | None]:
    """Open the audit sinks, attach them, and start the retention sweeper.

    Opens the JSONL sink (always on) and, when audit.sqlite.enabled,
    the SQLite sink too; the two are combined behind a MultiSink.
    Returns ``(jsonl_sink, sqlite_sink, sweeper_task)`` for teardown.
    On JSONL-open failure returns ``(None, None, None)``; the server
    keeps its NoopSink default. Audit never blocks startup.
    """
    audit_dir = resolve_default_audit_dir()
    try:
        jsonl_sink = JSONLSink(audit_dir)
    except OSError as exc:
        log.warning(
            "audit JSONL sink unavailable; continuing without audit",
            directory=audit_dir,
            error=str(exc),
        )
        return None, None, None

    log.info("audit JSONL sink open", path=jsonl_sink.path)

    sqlite_sink = (
        _open_sqlite_sink(cfg, jsonl_sink, log) if cfg.audit.sqlite.enabled else None
    )
    audit_sink = MultiSink(jsonl_sink, sqlite_sink) if sqlite_sink else jsonl_sink
    server.set_audit_sink(audit_sink)

    # Phase 2b/3a: sweep rotated logs older than the retention window
    # in the background. The window comes from audit.retention_days
    # config (0 = never delete).
    sweeper = RetentionSweeper(
        str(Path(jsonl_sink.path).parent),
        cfg.audit.retention_days,
    )
    sweeper_task = asyncio.create_task(sweeper.run())
    return jsonl_sink, sqlite_sink, sweeper_task


def _open_sqlite_sink(
    cfg: Config,
    jsonl_sink: JSONLSink,
    log: structlog.stdlib.BoundLogger,
) -> SQLiteSink | None:
    """Open the SQLite sink at the configured path (or audit.db beside
    the JSONL log). Returns None on failure after logging; the caller
    keeps the JSONL sink as the durable record.
    """
    db_path = cfg.audit.sqlite.path or str(Path(jsonl_sink.path).parent / "audit.db")
    try:
        sink = SQLiteSink(db_path, cfg.audit.sqlite.busy_timeout_ms)
    except Exception as exc:
        log.warning(
            "audit SQLite sink unavailable; continuing with JSONL only",
            path=db_path,
            error=str(exc),
        )
        return None

    log.info("audit SQLite sink open", path=db_path)
    return sink


async def _stop_audit(
    jsonl_sink: JSONLSink | None,
    sqlite_sink: SQLiteSink | None,
    sweeper_task: asyncio.Task[None] | None,
    log: structlog.stdlib.BoundLogger,
) -> None:
    """Stop the sweeper and close the sinks on shutdown.

    Cancels the sweeper task first so it stops touching the directory,
    then closes the SQLite sink (sqlite3 close does not raise) and the
    JSONL sink so final events land and the file releases.
    """
    if sweeper_task is not None:
        sweeper_task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await sweeper_task

    if sqlite_sink is not None:
        sqlite_sink.close()

    if jsonl_sink is None:
        return
    try:
        jsonl_sink.close()
    except OSError as exc:
        log.warning("audit JSONL sink close error", error=str(exc))


def _wire_profile_hot_reload(
    watcher: ConfigWatcher,
    server: Server,
    log: structlog.stdlib.BoundLogger,
) -> None:
    """Phase 5: wire profile reload to config-file changes.

    The watcher fires on_change inside its polling task; reload_profile
    is fast (resolver + dict swap) so awaiting it inline is fine. A
    failed reload logs and the previous profile stays active.
    """

    async def _on_config_change(new_cfg: Config) -> None:
        try:
            await server.reload_profile(new_cfg)
        except Exception as exc:
            log.warning("profile reload failed", error=str(exc))

    watcher.set_on_change(_on_config_change)


async def async_main() -> int:
    """Async main function."""
    path = get_config_path()
    try:
        watcher = ConfigWatcher(path)
    except ConfigError as exc:
        logger.exception("failed to load configuration", error=str(exc))
        return 1

    cfg = watcher.get()

    try:
        obs = Observability(cfg.observability)
    except Exception as exc:
        logger.exception("failed to initialize observability", error=str(exc))
        obs = Observability(None)

    log = obs.logger
    version_info = get_version_info()

    log.info("starting LinodeMCP server")
    log.info(
        "server configuration",
        version=version_info.version,
        server=cfg.server.name,
        platform=version_info.platform,
        git_commit=version_info.git_commit,
    )

    # Bridge the watcher to tool helpers so reloaded resilience and
    # environment values take effect on the next tool call.
    tool_helpers.set_live_config_source(watcher.get)

    server: Server | None = None
    jsonl_sink: JSONLSink | None = None
    sqlite_sink: SQLiteSink | None = None
    sweeper_task: asyncio.Task[None] | None = None
    try:
        server = Server(cfg)

        # Phase 2a/2b/3b: attach the JSONL sink (and SQLite sink when
        # enabled) so every tool call lands on disk, and start the
        # background retention sweeper. On JSONL failure the server
        # keeps its NoopSink default; audit never blocks startup.
        jsonl_sink, sqlite_sink, sweeper_task = _start_audit(server, cfg, log)

        _wire_profile_hot_reload(watcher, server, log)

        # Phase 6.4c: validate the active token's scopes against the
        # active profile's required scopes. Missing scopes always fail
        # load; an API failure or missing token fails for elevated
        # profiles and warns-and-continues for read-only ones. Excess
        # scopes warn only.
        if not await _run_scope_validation(server, log):
            return 1

        watcher.start()
        await server.start()
    except Exception as exc:
        log.exception("server error", error=str(exc))
        return 1
    finally:
        if server is not None:
            try:
                drained = await server.shutdown(timeout=10.0)
                if not drained:
                    log.warning(
                        "server shutdown drain timed out before all handlers completed"
                    )
            except Exception as exc:
                log.exception("server shutdown drain error", error=str(exc))

        # Stop the watcher and unregister the live source so subsequent
        # imports of helpers don't see a dangling reference.
        try:
            await watcher.stop()
        except Exception as exc:
            log.exception("config watcher stop error", error=str(exc))
        tool_helpers.set_live_config_source(None)

        await _stop_audit(jsonl_sink, sqlite_sink, sweeper_task, log)

        try:
            obs.shutdown()
        except Exception as exc:
            log.exception("observability shutdown error", error=str(exc))

    log.info("server shutdown complete")
    return 0


def main() -> None:
    """Main entry point.

    Bare invocation (``linodemcp``) starts the MCP server via stdio.
    Phase 7a profile subcommand: ``linodemcp profile list|show ...``
    dispatches to the CLI helpers and exits without touching the
    server runtime. Subcommand mode is synchronous and never calls
    ``asyncio.run`` so simple operations don't pay the event-loop tax.
    """
    if len(sys.argv) >= _MIN_ARGV_FOR_SUBCOMMAND and sys.argv[1] == "profile":
        sys.exit(run_profile_command(sys.argv[2:], sys.stdout, sys.stderr))

    try:
        exit_code = asyncio.run(async_main())
        sys.exit(exit_code)
    except KeyboardInterrupt:
        logger.info("shutdown signal received")
        sys.exit(0)


if __name__ == "__main__":
    main()
