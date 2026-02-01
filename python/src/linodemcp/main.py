"""Main entry point for LinodeMCP server."""

import asyncio
import logging
import sys

import structlog

from linodemcp.config import ConfigError, load
from linodemcp.server import Server
from linodemcp.version import get_version_info

# Configure structlog for production-ready logging
structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.StackInfoRenderer(),
        structlog.dev.set_exc_info,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.dev.ConsoleRenderer(),
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
    except ConfigError as e:
        logger.error("failed to load configuration", error=str(e))
        return 1

    version_info = get_version_info()

    logger.info("starting LinodeMCP server")
    logger.info(
        "server configuration",
        version=version_info.version,
        server=cfg.server.name,
        platform=version_info.platform,
        git_commit=version_info.git_commit,
    )

    try:
        server = Server(cfg)
        await server.start()
    except Exception as e:
        logger.error("server error", error=str(e))
        return 1

    logger.info("server shutdown complete")
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
