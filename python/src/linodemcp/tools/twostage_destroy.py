"""Two-stage plan/apply flow for destructive Python tools.

Mirrors ``go/internal/tools/twostage_destroy.go``. ``run_two_stage_destroy``
reads the plan store the server published on the ContextVar, handles
``mode:"plan"`` and ``mode:"apply"``, and returns ``None`` for everything else
so the caller falls through to its existing single-step path.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent

from linodemcp import twostage
from linodemcp.config import EnvironmentNotFoundError
from linodemcp.linode import APIError, NetworkError
from linodemcp.profiles import Capability
from linodemcp.tools import helpers
from linodemcp.twostage.store import PlanEntry, PlanExpiredError, PlanNotFoundError

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient
    from linodemcp.twostage.store import PlanStore

# Control flags stripped before comparing apply-time args to the stored args.
_CONTROL_KEYS = frozenset(
    {
        "mode",
        "plan_id",
        "confirm",
        "dry_run",
        "confirmed_dry_run",
        "confirm_bypass_dry_run",
        "yolo",
    }
)

# Errors a state fetch can raise, matching execute_tool's handling.
_FETCH_ERRORS = (
    EnvironmentNotFoundError,
    ValueError,
    APIError,
    NetworkError,
    httpx.HTTPError,
)

type _FetchState = Callable[[RetryableClient], Awaitable[Any]]
type _Execute = Callable[[RetryableClient], Awaitable[dict[str, Any]]]


def _json_default(obj: object) -> Any:
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        return dataclasses.asdict(obj)
    return str(obj)


def _state_hash(state: Any, hash_ignore: list[str]) -> str:
    serialized = json.dumps(state, sort_keys=True, default=_json_default)
    if hash_ignore:
        serialized = _strip_hash_fields(serialized, hash_ignore)
    return "sha256:" + hashlib.sha256(serialized.encode()).hexdigest()


def _strip_hash_fields(serialized: str, hash_ignore: list[str]) -> str:
    parsed = json.loads(serialized)
    if not isinstance(parsed, dict):
        return serialized
    obj = cast("dict[str, Any]", parsed)
    for field in hash_ignore:
        obj.pop(field, None)
    return json.dumps(obj, sort_keys=True)


def _non_control_args(arguments: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in arguments.items() if key not in _CONTROL_KEYS}


def _text(message: str) -> list[TextContent]:
    return [TextContent(type="text", text=message)]


def _refusal(err_code: str, plan_id: str) -> list[TextContent]:
    messages = {
        twostage.ERR_PLAN_EXPIRED: (
            f"PLAN_EXPIRED: plan {plan_id!r} has expired. "
            'Create a new plan with mode: "plan".'
        ),
        twostage.ERR_PLAN_ARGS_MISMATCH: (
            f"PLAN_ARGS_MISMATCH: args supplied at apply time differ from plan "
            f"{plan_id!r}. Apply without passing args, or create a new plan."
        ),
        twostage.ERR_PLAN_DRIFT: (
            f"PLAN_DRIFT_DETECTED: the resource changed since plan {plan_id!r} "
            'was created. Create a new plan with mode: "plan" and review first.'
        ),
    }
    default = (
        f"PLAN_NOT_FOUND: no plan with id {plan_id!r}. "
        'Create a new plan with mode: "plan". Plans do not persist across a restart.'
    )
    return _text(messages.get(err_code, default))


async def run_two_stage_destroy(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool_name: str,
    method: str,
    path: str,
    fetch_state: _FetchState,
    execute: _Execute,
    hash_ignore: list[str] | None = None,
) -> list[TextContent] | None:
    """Handle plan/apply for a destroy tool, or None to fall through.

    Returns None when two-stage does not apply: a yolo call (it dominates via
    the server's single-step path), no plan store on the context, a call
    without mode:"plan"/"apply", or a tool that is not opted in.
    """
    if arguments.get("yolo") is True:
        return None

    store = twostage.plan_store_from_context()
    if store is None:
        return None

    mode = arguments.get("mode", "")
    if mode not in (twostage.MODE_PLAN, twostage.MODE_APPLY):
        return None

    settings = _two_stage_settings(cfg)
    if not settings.opted_in(tool_name, Capability.Destroy):
        return None

    ignore = hash_ignore or []
    if mode == twostage.MODE_PLAN:
        return await _run_plan(
            cfg,
            arguments,
            tool_name,
            method,
            path,
            fetch_state,
            execute,
            store,
            ignore,
            settings,
        )

    return await _run_apply(cfg, arguments, fetch_state, store, ignore)


def _two_stage_settings(cfg: Config) -> twostage.Settings:
    """Resolve the operator-tunable two-stage parameters from config.

    Non-positive TTL values are dropped here so the resolver falls back to the
    next level (per-tool to default to built-in). Mirrors the Go
    twoStageSettings helper.
    """
    two_stage = cfg.two_stage

    default_ttl: timedelta | None = None
    if (
        two_stage.default_plan_ttl_seconds is not None
        and two_stage.default_plan_ttl_seconds > 0
    ):
        default_ttl = timedelta(seconds=two_stage.default_plan_ttl_seconds)

    tool_ttl = {
        tool: timedelta(seconds=secs)
        for tool, secs in two_stage.tool_ttl_seconds.items()
        if secs > 0
    }

    return twostage.Settings(
        default_ttl=default_ttl, tool_ttl=tool_ttl, opt_in=dict(two_stage.opt_in)
    )


async def _run_plan(
    cfg: Config,
    arguments: dict[str, Any],
    tool_name: str,
    method: str,
    path: str,
    fetch_state: _FetchState,
    execute: _Execute,
    store: PlanStore,
    hash_ignore: list[str],
    settings: twostage.Settings,
) -> list[TextContent]:
    try:
        state = await helpers.with_client(cfg, arguments, fetch_state)
    except _FETCH_ERRORS as exc:
        return _text(f"Failed to fetch state for plan: {exc}")

    state_hash = _state_hash(state, hash_ignore)
    plan_id = twostage.new_plan_id()
    now = datetime.now(UTC)
    expires = now + settings.plan_ttl(tool_name)
    environment = arguments.get("environment", "")

    async def _apply() -> list[TextContent]:
        return await helpers.execute_tool(cfg, arguments, f"apply {tool_name}", execute)

    await store.put(
        PlanEntry(
            id=plan_id,
            tool=tool_name,
            environment=environment,
            args=_non_control_args(arguments),
            state_hash=state_hash,
            planned_at=now,
            expires_at=expires,
            apply=_apply,
        )
    )

    body = {
        "plan_id": plan_id,
        "created_at": now.isoformat(),
        "expires_at": expires.isoformat(),
        "tool": tool_name,
        "environment": environment,
        "would_execute": {"method": method, "path": path},
        "current_state": state,
        "current_state_hash": state_hash,
    }
    return [
        TextContent(type="text", text=json.dumps(body, indent=2, default=_json_default))
    ]


async def _run_apply(
    cfg: Config,
    arguments: dict[str, Any],
    fetch_state: _FetchState,
    store: PlanStore,
    hash_ignore: list[str],
) -> list[TextContent]:
    plan_id = arguments.get("plan_id", "")
    lookup, entry, error = await _classify_plan(
        cfg, arguments, fetch_state, store, plan_id, hash_ignore
    )
    if error is not None:
        return error

    decision = twostage.resolve(
        twostage.Request(
            capability=Capability.Destroy,
            two_stage_opted_in=True,
            mode=twostage.MODE_APPLY,
            plan_id=plan_id,
            plan_lookup=lookup,
        )
    )
    if decision.branch != twostage.Branch.APPLY or entry is None:
        return _refusal(decision.err_code, plan_id)

    result = await entry.apply()
    await store.remove(plan_id)
    return result


async def _classify_plan(
    cfg: Config,
    arguments: dict[str, Any],
    fetch_state: _FetchState,
    store: PlanStore,
    plan_id: str,
    hash_ignore: list[str],
) -> tuple[twostage.PlanLookup, PlanEntry | None, list[TextContent] | None]:
    try:
        entry = await store.get(plan_id)
    except PlanNotFoundError:
        return twostage.PlanLookup.UNKNOWN, None, None
    except PlanExpiredError:
        return twostage.PlanLookup.EXPIRED, None, None

    supplied = _non_control_args(arguments)
    if supplied and supplied != entry.args:
        return twostage.PlanLookup.ARGS_MISMATCH, entry, None

    try:
        state = await helpers.with_client(cfg, arguments, fetch_state)
    except _FETCH_ERRORS as exc:
        return (
            twostage.PlanLookup.NOT_APPLICABLE,
            None,
            _text(f"Failed to re-fetch state for apply: {exc}"),
        )

    if _state_hash(state, hash_ignore) != entry.state_hash:
        return twostage.PlanLookup.DRIFTED, entry, None

    return twostage.PlanLookup.VALID, entry, None
