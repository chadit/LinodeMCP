"""Shared helper utilities for MCP tool implementations."""

from __future__ import annotations

import dataclasses
import ipaddress
import json
import logging
from typing import TYPE_CHECKING, Any, TypedDict, cast

import httpx
from mcp.types import TextContent

from linodemcp.config import EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.genpb.linode.mcp.v1 import dryrun_pb2
from linodemcp.linode import (
    APIError,
    NetworkError,
    RetryableClient,
    RetryConfig,
)
from linodemcp.tools.proto_response import serialize_preview_envelope

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

# dry_run parameter name and JSON-schema fragment for mutating tools.
# Mirrors the Go-side `paramDryRun` constant. Tools that opt into
# dry-run merge ``DRY_RUN_PROP`` into their input schema's properties
# under the key ``PARAM_DRY_RUN`` (no need to add it to ``required``;
# the param defaults to False when omitted).
PARAM_DRY_RUN = "dry_run"
DRY_RUN_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": (
        "Preview the call without making it: returns the would-be "
        "request and current resource state. Default false."
    ),
}

# Two-stage (plan/apply) schema fragments. Mirror the Go-side `paramMode`
# and `paramPlanID`. Opted-in CapDestroy tools merge ``MODE_PROP`` and
# ``PLAN_ID_PROP`` into their input schema so one wording is shared across
# every delete tool.
PARAM_MODE = "mode"
MODE_PROP: dict[str, Any] = {
    "type": "string",
    "description": (
        'Two-stage flow: "plan" previews and returns a plan_id; "apply" '
        "with plan_id re-checks drift and executes. Omit for a single-step "
        "call."
    ),
}
PARAM_PLAN_ID = "plan_id"
PLAN_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": (
        'The plan_id returned by a mode:"plan" call, supplied with '
        'mode:"apply" to execute it.'
    ),
}

# Appended to every opted-in delete tool's description so the plan/apply flow
# shows up at the tool level, not only on the mode and plan_id params. Mirrors
# the Go twoStageNote. See docs/two-stage-writes.md.
TWO_STAGE_NOTE = (
    ' Supports two-stage writes: mode="plan" returns a plan_id; mode="apply" '
    "with that plan_id re-checks for drift, then executes."
)

# Variant for a tool whose two-stage flow is off until an operator enables it
# (e.g. instance_resize, a CapWrite tool that does not opt in by default).
TWO_STAGE_OPT_IN_NOTE = (
    ' Supports two-stage writes when enabled in the two_stage config: mode="plan"'
    ' returns a plan_id; mode="apply" with that plan_id re-checks for drift, then'
    " executes."
)


def is_dry_run(arguments: dict[str, Any]) -> bool:
    """Report whether ``arguments[PARAM_DRY_RUN]`` is the literal True.

    Mirrors the Go-side ``IsDryRun`` helper. Non-bool values degrade
    to False; MCP schema validation enforces the type upstream, so a
    wrong-type value reaching the handler implies a bug elsewhere.
    Keeping the strict-bool path avoids string-truthiness surprises.
    """
    value = arguments.get(PARAM_DRY_RUN, False)
    return value is True


def _dataclass_json_default(obj: Any) -> Any:
    """json.dumps ``default`` that serializes Linode dataclass models (e.g.
    the ``Instance`` returned by ``get_instance``) as plain dicts. Without
    this, a dry-run whose ``current_state`` is a dataclass would raise
    "Object of type X is not JSON serializable".
    """
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        return dataclasses.asdict(obj)
    msg = f"Object of type {type(obj).__name__} is not JSON serializable"
    raise TypeError(msg)


class DryRunDetails(TypedDict, total=False):
    """Phase 2 dependency-walk enrichment for a dry-run preview. All keys
    optional: a walk fills whichever apply to its tier and omits the rest,
    so the v0 wire shape is unchanged for tools without a walk. Mirrors the
    Go-side ``DryRunDetails``.
    """

    dependencies: list[dict[str, Any]]
    side_effects: list[str]
    billing_delta: dict[str, Any]
    warnings: list[str]


def build_dry_run_response(
    tool_name: str,
    environment: str,
    method: str,
    path: str,
    current_state: Any,
    *,
    dependencies: list[dict[str, Any]] | None = None,
    side_effects: list[str] | None = None,
    billing_delta: dict[str, Any] | None = None,
    warnings: list[str] | None = None,
    request_body: Any | None = None,
) -> list[TextContent]:
    """Build the dry-run wire shape and wrap it as MCP text content.

    Tool handlers call this from their dry_run branch after fetching
    current_state. The envelope serializes through the DryRunResponse proto so
    it is proto-canonical on both languages: current_state routes through a
    google.protobuf.Value (JSON null when None, sorted object keys otherwise),
    and the empty dependency-walk fields serialize as ``[]`` the same way the Go
    builder emits them. Cross-language parity is asserted by test_tools_dryrun
    and the conformance corpus.
    """
    would_execute: dict[str, Any] = {"method": method, "path": path}
    if request_body is not None:
        would_execute["body"] = request_body

    raw: dict[str, Any] = {
        "dry_run": True,
        "tool": tool_name,
        "would_execute": would_execute,
        "current_state": current_state,
    }
    if environment:
        raw["environment"] = environment
    if dependencies:
        raw["dependencies"] = dependencies
    if side_effects:
        raw["side_effects"] = side_effects
    if billing_delta:
        raw["billing_delta"] = billing_delta
    if warnings:
        raw["warnings"] = warnings

    # Round-trip through JSON first so a dataclass current_state (the read
    # sibling model a walk fetches) becomes the plain dict ParseDict accepts,
    # mirroring the Go builder's json.Marshal into a structpb.Value.
    plain = cast(
        "dict[str, Any]",
        json.loads(json.dumps(raw, default=_dataclass_json_default)),
    )
    result = serialize_preview_envelope(plain, dryrun_pb2.DryRunResponse())

    return [TextContent(type="text", text=json.dumps(result, indent=2))]


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


async def with_client[T](
    cfg: Config,
    arguments: dict[str, Any],
    callback: Callable[[RetryableClient], Awaitable[T]],
) -> T:
    """Open a RetryableClient for the selected environment and run a callback.

    Unlike execute_tool the result is returned raw (not JSON-wrapped) and
    errors propagate, so callers such as the two-stage plan/apply flow handle
    fetch failures themselves.
    """
    environment = arguments.get("environment", "")
    selected_env = _select_environment(cfg, environment)
    _validate_linode_config(selected_env)
    async with RetryableClient(
        selected_env.linode.api_url,
        selected_env.linode.token,
        _retry_config_from(cfg),
    ) as client:
        return await callback(client)


async def execute_dry_run(
    cfg: Config,
    arguments: dict[str, Any],
    tool_name: str,
    method: str,
    path: str,
    fetch_state: Callable[[RetryableClient], Awaitable[Any]],
    details_fn: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]]
    | None = None,
    *,
    request_body: Any | None = None,
) -> list[TextContent]:
    """Run the dry-run code path: fetch current state, return the v0
    preview wire shape, never mutate.

    Parallel to ``execute_tool`` for tools opted into Phase 1 dry-run.
    The caller is responsible for validating tool-specific input
    (e.g. instance_id non-zero) before calling this; this helper only
    handles environment selection, client setup, error handling, and
    response shaping. ``fetch_state`` performs the GET that supplies
    ``current_state`` in the response. ``details_fn``, when given, runs the
    Phase 2 dependency walk against the same client and fetched state and
    its result enriches the preview (Tier A/B tools).
    """
    environment = arguments.get("environment", "")
    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
        ) as client:
            current_state = await fetch_state(client)
            details: DryRunDetails = {}
            if details_fn is not None:
                details = await details_fn(client, current_state)
            return build_dry_run_response(
                tool_name,
                environment,
                method,
                path,
                current_state,
                request_body=request_body,
                **details,
            )
    except Exception as e:
        if isinstance(e, (EnvironmentNotFoundError, ValueError)):
            return [TextContent(type="text", text=f"Error: {e}")]
        if isinstance(e, (APIError, NetworkError, httpx.HTTPError)):
            return [
                TextContent(
                    type="text",
                    text=f"Failed to fetch state for dry-run: {e}",
                )
            ]
        logger.exception("Unexpected error in dry-run handler")
        return [
            TextContent(
                type="text",
                text=f"Failed to fetch state for dry-run: {e}",
            )
        ]


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


def required_int_id(arguments: dict[str, Any], name: str) -> tuple[int | None, str]:
    """Validate a required positive-integer id path argument (Option B).

    Returns ``(id, "")`` on success and ``(None, message)`` on failure, mirroring
    the Go requiredIDArgument helper's ``(int, string)`` pair so callers can guard
    with ``if id is None:`` and still hand the message straight to
    ``error_response``. Absent key -> ``"<name> is required"``; present but not a
    positive integer (bool, non-int, zero, negative) -> ``"<name> must be a
    positive integer"``. A present-but-null value is treated as invalid (matches
    Go, which reaches its numeric parser for an explicit null).
    """
    if name not in arguments:
        return None, f"{name} is required"
    value = arguments[name]
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return None, f"{name} must be a positive integer"
    return value, ""


def valid_ipv6_prefix(value: str) -> bool:
    """Return whether value is a masked IPv6 CIDR prefix.

    Mirrors Go's ipv6RangeFromTool (netip.ParsePrefix + Is6 + prefix==Masked):
    the value must carry an explicit ``/bits`` suffix, parse as an IPv6 network,
    and have no host bits set below the prefix length (strict). Ported so both
    languages reject a malformed range locally instead of sending it on the wire.
    """
    if "/" not in value:
        return False
    try:
        network = ipaddress.ip_network(value, strict=True)
    except ValueError:
        return False
    return isinstance(network, ipaddress.IPv6Network)


def pagination_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    """Parse an optional pagination integer with Go-aligned range messages.

    Returns None when the argument is absent (pagination is optional). A non-int
    raises ``TypeError("<name> must be an integer")``; an out-of-range value
    raises ``ValueError`` with Go's ranged text: "greater than or equal to
    {minimum}" when there is no upper bound, "from {minimum} through {maximum}"
    otherwise. Mirrors the Go optionalPaginationInt helper.
    """
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        msg = f"{name} must be an integer"
        raise TypeError(msg)
    if value < minimum or (maximum is not None and value > maximum):
        if maximum is not None:
            msg = f"{name} must be an integer from {minimum} through {maximum}"
        else:
            msg = f"{name} must be an integer greater than or equal to {minimum}"
        raise ValueError(msg)
    return value
