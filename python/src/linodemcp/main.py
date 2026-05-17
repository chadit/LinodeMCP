"""Main entry point for LinodeMCP server."""

import asyncio
import logging
import sys

import structlog

from linodemcp.config import Config, ConfigError, get_config_path
from linodemcp.config.watcher import ConfigWatcher
from linodemcp.observability import Observability
from linodemcp.server import Server
from linodemcp.tools import helpers as tool_helpers
from linodemcp.version import get_version_info

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
    try:
        server = Server(cfg)
        _wire_profile_hot_reload(watcher, server, log)
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

        try:
            obs.shutdown()
        except Exception as exc:
            log.exception("observability shutdown error", error=str(exc))

    log.info("server shutdown complete")
    return 0


def main() -> None:
    """Main entry point."""
    try:
        exit_code = asyncio.run(async_main())
        sys.exit(exit_code)
    except KeyboardInterrupt:
        logger.info("shutdown signal received")
        sys.exit(0)


if __name__ == "__main__":
    main()
