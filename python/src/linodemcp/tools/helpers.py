"""Shared helper utilities for MCP tool implementations."""

from __future__ import annotations

import json
import logging
from typing import TYPE_CHECKING, Any

import httpx
from mcp.types import TextContent

from linodemcp.config import EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.linode import (
    APIError,
    NetworkError,
    RetryableClient,
    RetryConfig,
)

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from linodemcp.config import Config

logger = logging.getLogger(__name__)

# Constants for truncation limits
SSH_KEY_TRUNCATE_LIMIT = 50
DESCRIPTION_TRUNCATE_LIMIT = 100

# Environment parameter schema (reused across all tools)
ENV_PARAM_SCHEMA = {
    "environment": {
        "type": "string",
        "description": ("Linode environment to use (optional, defaults to 'default')"),
    },
}


def truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value


# Module-level live config source for hot-reload. main.py sets this to
# `watcher.get` so each tool call resolves through the latest reloaded
# Config rather than the snapshot captured at startup. None disables the
# bridge (default: callers receive their snapshot unchanged).
_live_config_source: Callable[[], Config] | None = None


def set_live_config_source(getter: Callable[[], Config] | None) -> None:
    """Register the function that returns the latest Config.

    Pass None to unregister. Called once by main.py at startup.
    """
    global _live_config_source  # noqa: PLW0603 - process-wide hot-reload bridge
    _live_config_source = getter


def _resolve_config(snapshot: Config) -> Config:
    """Return the live config when a source is registered, else snapshot."""
    if _live_config_source is not None:
        return _live_config_source()
    return snapshot


def _retry_config_from(cfg: Config) -> RetryConfig:
    """Build a RetryConfig from the loaded resilience settings.

    Threads rate-limit, circuit-breaker, retry, and HTTP pool tuning through
    to the client so operator-set values take effect instead of dataclass
    defaults. Reads through `_resolve_config` so a registered live source
    (set by main.py from the ConfigWatcher) wins over the snapshot.
    """
    res = _resolve_config(cfg).resilience
    return RetryConfig(
        max_retries=res.max_retries,
        base_delay=float(res.base_retry_delay),
        max_delay=float(res.max_retry_delay),
        circuit_breaker_threshold=res.circuit_breaker_threshold,
        circuit_breaker_timeout=float(res.circuit_breaker_timeout),
        rate_limit_per_minute=res.rate_limit_per_minute,
        pool_max_connections=res.pool_max_connections,
        pool_max_keepalive_connections=res.pool_max_keepalive_connections,
        pool_keepalive_expiry=res.pool_keepalive_expiry,
    )


def _select_environment(cfg: Config, environment: str) -> EnvironmentConfig:
    """Select an environment from configuration."""
    if environment:
        if environment in cfg.environments:
            return cfg.environments[environment]
        msg = f"environment not found: {environment}"
        raise EnvironmentNotFoundError(msg)

    return cfg.select_environment("default")


def _validate_linode_config(env: EnvironmentConfig) -> None:
    """Validate Linode configuration."""
    if not env.linode.api_url or not env.linode.token:
        msg = "linode configuration is incomplete: check your API URL and token"
        raise ValueError(msg)


async def execute_tool(
    cfg: Config,
    arguments: dict[str, Any],
    error_action: str,
    callback: Callable[[RetryableClient], Awaitable[dict[str, Any]]],
) -> list[TextContent]:
    """Run a tool handler with standard environment/client/error boilerplate."""
    environment = arguments.get("environment", "")
    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
        ) as client:
            response = await callback(client)
            return [TextContent(type="text", text=json.dumps(response, indent=2))]
    except Exception as e:
        if isinstance(e, (EnvironmentNotFoundError, ValueError)):
            return [TextContent(type="text", text=f"Error: {e}")]
        if isinstance(e, (APIError, NetworkError, httpx.HTTPError)):
            return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]
        logger.exception("Unexpected error in tool handler")
        return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]


async def execute_tool_list(
    cfg: Config,
    arguments: dict[str, Any],
    error_action: str,
    callback: Callable[[RetryableClient], Awaitable[list[dict[str, Any]]]],
) -> list[TextContent]:
    """Run a tool handler that returns a list with standard boilerplate."""
    environment = arguments.get("environment", "")
    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
        ) as client:
            response = await callback(client)
            return [TextContent(type="text", text=json.dumps(response, indent=2))]
    except Exception as e:
        if isinstance(e, (EnvironmentNotFoundError, ValueError)):
            return [TextContent(type="text", text=f"Error: {e}")]
        if isinstance(e, (APIError, NetworkError, httpx.HTTPError)):
            return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]
        logger.exception("Unexpected error in tool handler")
        return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]


def error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]
