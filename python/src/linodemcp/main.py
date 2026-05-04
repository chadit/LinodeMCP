"""Main entry point for LinodeMCP server."""

import asyncio
import logging
import sys

import structlog

from linodemcp.config import ConfigError, load
from linodemcp.observability import Observability
from linodemcp.server import Server
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


async def async_main() -> int:
    """Async main function."""
    try:
        cfg = load()
    except ConfigError as exc:
        logger.exception("failed to load configuration", error=str(exc))
        return 1

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

    try:
        server = Server(cfg)
        await server.start()
    except Exception as exc:
        log.exception("server error", error=str(exc))
        return 1
    finally:
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
