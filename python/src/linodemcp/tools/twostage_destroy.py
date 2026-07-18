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
from linodemcp.genpb.linode.mcp.v1 import dryrun_pb2
from linodemcp.linode import APIError, NetworkError
from linodemcp.profiles import Capability
from linodemcp.tools import helpers
from linodemcp.tools.proto_response import serialize_preview_envelope
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
# Per-tool dependency walk run at plan time. Mirrors the dry-run details_fn:
# given a live client and the fetched state, it returns the same DryRunDetails
# (dependencies / side_effects / billing_delta / warnings) the dry-run path
# surfaces, so a plan reads like a dry-run preview.
type _DependencyWalk = Callable[
    [RetryableClient, Any], Awaitable[helpers.DryRunDetails]
]


def _json_default(obj: object) -> Any:
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        # Same keyword-escape stripping as the dry-run path, so a plan's
        # current_state carries the wire field names ("in", not "in_").
        return dataclasses.asdict(obj, dict_factory=helpers.keyword_escape_dict_factory)
    return str(obj)


def _state_hash_and_fields(
    state: Any, hash_ignore: list[str]
) -> tuple[str, dict[str, Any] | None]:
    """Hash the state for drift detection and return its normalized top-level
    field map with hash-ignore fields stripped. The map lets the apply path
    name the changed fields on a drift refusal; it is None when the state does
    not serialize to a JSON object. Plan and apply both call this, so the hash
    is identical on both sides. Mirrors the Go stateHashAndFields.
    """
    serialized = json.dumps(state, sort_keys=True, default=_json_default)
    parsed = json.loads(serialized)
    if not isinstance(parsed, dict):
        return "sha256:" + hashlib.sha256(serialized.encode()).hexdigest(), None

    obj = cast("dict[str, Any]", parsed)
    for field in hash_ignore:
        obj.pop(field, None)

    stripped = json.dumps(obj, sort_keys=True)
    return "sha256:" + hashlib.sha256(stripped.encode()).hexdigest(), obj


def _changed_field_names(
    planned: dict[str, Any] | None, current: dict[str, Any] | None
) -> list[str]:
    """Return the sorted top-level keys whose values differ between the planned
    and current field maps. Drives the changed-fields list in a drift refusal.
    Returns an empty list when either map is None (no per-field diff available).
    """
    if planned is None or current is None:
        return []

    def encode(value: Any) -> str:
        return json.dumps(value, sort_keys=True, default=_json_default)

    keys = set(planned) | set(current)
    changed = [
        key for key in keys if encode(planned.get(key)) != encode(current.get(key))
    ]
    return sorted(changed)


def _non_control_args(arguments: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in arguments.items() if key not in _CONTROL_KEYS}


def _text(message: str) -> list[TextContent]:
    return [TextContent(type="text", text=message)]


def _refusal(
    err_code: str, plan_id: str, changed_fields: list[str] | None = None
) -> list[TextContent]:
    changed = ", ".join(changed_fields) if changed_fields else "one or more fields"
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
            f"was created (changed fields: {changed}). "
            'Create a new plan with mode: "plan" and review first.'
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
    dependency_walk: _DependencyWalk | None = None,
    capability: Capability = Capability.Destroy,
) -> list[TextContent] | None:
    """Handle plan/apply for a destroy tool, or None to fall through.

    Returns None when two-stage does not apply: a yolo call (it dominates via
    the server's single-step path), no plan store on the context, a call
    without mode:"plan"/"apply", or a tool that is not opted in.

    capability is the tool's profile capability, consulted at the opt-in gate.
    It defaults to Destroy (every delete tool), so a CapWrite tool like
    instance_resize passes Capability.Write to stay opt-in by config only.
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
    if not settings.opted_in(tool_name, capability):
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
            dependency_walk,
        )

    return await _run_apply(cfg, arguments, fetch_state, store, ignore, capability)


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
    dependency_walk: _DependencyWalk | None,
) -> list[TextContent]:
    # Fetch the state and run the dependency walk under one client so the plan
    # body reads like a dry-run preview (dependencies / side_effects /
    # billing_delta / warnings). Mirrors the Go runPlan + planDependencies.
    async def _fetch_and_walk(
        client: RetryableClient,
    ) -> tuple[Any, helpers.DryRunDetails]:
        state = await fetch_state(client)
        details: helpers.DryRunDetails = {}
        if dependency_walk is not None:
            details = await dependency_walk(client, state)
        return state, details

    try:
        state, details = await helpers.with_client(cfg, arguments, _fetch_and_walk)
    except _FETCH_ERRORS as exc:
        return _text(f"Failed to fetch state for plan: {exc}")

    state_hash, state_fields = _state_hash_and_fields(state, hash_ignore)
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
            state_fields=state_fields,
        )
    )

    raw: dict[str, Any] = {
        "plan_id": plan_id,
        "created_at": _rfc3339(now),
        "expires_at": _rfc3339(expires),
        "tool": tool_name,
        "environment": environment,
        "would_execute": {"method": method, "path": path},
        "current_state": state,
        "current_state_hash": state_hash,
        **details,
    }
    # Round-trip through JSON first so a dataclass current_state collapses to the
    # plain dict ParseDict accepts, mirroring the Go builder. The PlanResponse
    # proto then makes the envelope proto-canonical on both languages.
    plain = cast("dict[str, Any]", json.loads(json.dumps(raw, default=_json_default)))
    result = serialize_preview_envelope(plain, dryrun_pb2.PlanResponse())
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def _rfc3339(moment: datetime) -> str:
    """Format a UTC datetime as RFC3339 with a Z suffix and whole seconds.

    Matches Go's time.RFC3339 output (no fractional seconds, Z zone) so the plan
    timestamps read identically on both languages, replacing Python's
    isoformat() which emitted a +00:00 offset with microseconds.
    """
    return (
        moment.astimezone(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    )


async def _run_apply(
    cfg: Config,
    arguments: dict[str, Any],
    fetch_state: _FetchState,
    store: PlanStore,
    hash_ignore: list[str],
    capability: Capability,
) -> list[TextContent]:
    plan_id = arguments.get("plan_id", "")
    lookup, entry, changed_fields, error = await _classify_plan(
        cfg, arguments, fetch_state, store, plan_id, hash_ignore
    )
    if error is not None:
        return error

    decision = twostage.resolve(
        twostage.Request(
            capability=capability,
            two_stage_opted_in=True,
            mode=twostage.MODE_APPLY,
            plan_id=plan_id,
            plan_lookup=lookup,
        )
    )
    if decision.branch != twostage.Branch.APPLY or entry is None:
        return _refusal(decision.err_code, plan_id, changed_fields)

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
) -> tuple[twostage.PlanLookup, PlanEntry | None, list[str], list[TextContent] | None]:
    # The third element is the list of changed top-level field names, populated
    # only on a drift verdict so the refusal can name them. Mirrors the Go
    # classifyPlan, whose fourth return is the changed-field slice.
    try:
        entry = await store.get(plan_id)
    except PlanNotFoundError:
        return twostage.PlanLookup.UNKNOWN, None, [], None
    except PlanExpiredError:
        return twostage.PlanLookup.EXPIRED, None, [], None

    supplied = _non_control_args(arguments)
    if supplied and supplied != entry.args:
        return twostage.PlanLookup.ARGS_MISMATCH, entry, [], None

    try:
        state = await helpers.with_client(cfg, arguments, fetch_state)
    except _FETCH_ERRORS as exc:
        return (
            twostage.PlanLookup.NOT_APPLICABLE,
            None,
            [],
            _text(f"Failed to re-fetch state for apply: {exc}"),
        )

    current_hash, current_fields = _state_hash_and_fields(state, hash_ignore)
    if current_hash != entry.state_hash:
        changed = _changed_field_names(entry.state_fields, current_fields)
        return twostage.PlanLookup.DRIFTED, entry, changed, None

    return twostage.PlanLookup.VALID, entry, [], None
