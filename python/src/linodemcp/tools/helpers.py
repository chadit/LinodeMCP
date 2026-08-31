"""Shared helper utilities for MCP tool implementations."""

from __future__ import annotations

import dataclasses
import json
import keyword
import logging
import math
import re
from datetime import datetime
from typing import TYPE_CHECKING, Any, TypedDict, cast
from urllib.parse import urlencode

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
from linodemcp.linode.routes import message_class
from linodemcp.tools.declared_state import DeclaredState
from linodemcp.tools.proto_response import (
    proto_to_canonical_dict,
    serialize_preview_envelope,
)

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable, Mapping

    from linodemcp.config import Config


logger = logging.getLogger(__name__)

# strconv.Atoi's legal set, which is what Go reads a string id with.
_ATOI = re.compile(r"[+-]?[0-9]+")

# The largest integer a JSON number carries exactly. Past it the value a route
# receives is not the one the caller wrote, which is why an id above it is
# refused rather than sent. Go spells the same number tools.MaxJSONSafeID.
MAX_JSON_SAFE_ID = 9007199254740991

SSH_KEY_TRUNCATE_LIMIT = 50
DESCRIPTION_TRUNCATE_LIMIT = 100

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


def is_dry_run(arguments: dict[str, Any]) -> bool:
    """Report whether ``arguments[PARAM_DRY_RUN]`` is the literal True.

    Mirrors the Go-side ``IsDryRun`` helper. Non-bool values degrade
    to False; MCP schema validation enforces the type upstream, so a
    wrong-type value reaching the handler implies a bug elsewhere.
    Keeping the strict-bool path avoids string-truthiness surprises.
    """
    value = arguments.get(PARAM_DRY_RUN, False)
    return value is True


# One page bounds a dependency-walk list fetch, mirroring the Go walk's
# dependencyWalkPageSize: a resource with more dependents than this is rare,
# and a preview notes possible truncation instead of paging exhaustively.
WALK_PAGE_SIZE = 100


def walk_page_items(page: Any) -> list[dict[str, Any]]:
    """Extract the data[] items from a raw paginated walk response."""
    if not isinstance(page, dict):
        return []
    raw_items = cast("dict[str, Any]", page).get("data", [])
    if not isinstance(raw_items, list):
        return []
    items = cast("list[object]", raw_items)
    return [cast("dict[str, Any]", item) for item in items if isinstance(item, dict)]


def preview_state_str(state: Any, field: str) -> str:
    """Read a string field from a fetched preview state.

    Fetched state arrives either as a plain dict (a declared fetch mirrors
    Go's struct marshal) or as a dataclass, so walks read fields through this
    accessor instead of committing to one shape.
    """
    if isinstance(state, dict):
        value = cast("dict[str, Any]", state).get(field, "")
        return value if isinstance(value, str) else ""
    return str(getattr(state, field, "") or "")


def keyword_escape_dict_factory(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    """dict_factory for dataclasses.asdict that restores wire field names.

    A dataclass field cannot be named after a Python keyword, so models
    escape them with a trailing underscore (Transfer.in_ for the API's
    "in"). Go marshals the real name, and a dry-run current_state must match
    it byte for byte, so the escape is stripped at this serialization
    boundary.
    """
    return {
        (key[:-1] if key.endswith("_") and keyword.iskeyword(key[:-1]) else key): value
        for key, value in pairs
    }


def _dataclass_json_default(obj: Any) -> Any:
    """json.dumps ``default`` that serializes Linode dataclass models as plain
    dicts. Without this, a dry-run whose ``current_state`` is a dataclass would
    raise "Object of type X is not JSON serializable".

    A declared fetch's state answers the projected object, which is the bare
    resource the preview reports and the plan hashes, never a wrapper naming
    the read it came from.
    """
    if isinstance(obj, DeclaredState):
        return obj.fields
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        return dataclasses.asdict(obj, dict_factory=keyword_escape_dict_factory)
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


def _select_environment(cfg: Config, environment: object) -> EnvironmentConfig:
    """Select an environment from configuration.

    A value that is not text names no environment, so it is refused the way an
    unknown name is rather than read past: the default environment carries a
    different account's token, and serving it silently ran the call somewhere
    the caller never asked for. The refused value is spelled as the JSON the
    caller wrote, sorted keys and no ASCII escaping, which is the same text Go's
    environmentArgument renders. An absent argument and an explicit null both
    leave the environment unnamed, which is what selects the default.
    """
    if environment is None:
        return cfg.select_environment("default")

    if not isinstance(environment, str):
        spelled = json.dumps(
            environment, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        msg = f"environment not found in configuration: {spelled}"
        raise EnvironmentNotFoundError(msg)

    if not environment:
        return cfg.select_environment("default")

    if environment in cfg.environments:
        return cfg.environments[environment]

    msg = f"environment not found in configuration: {environment}"
    raise EnvironmentNotFoundError(msg)


def _linode_config_complete(env: EnvironmentConfig) -> None:
    """Check that an environment carries what a client needs.

    Named apart from the argument readers on purpose: this judges the
    deployment's own configuration rather than anything a caller sent, and the
    hand-validator scan counts by that naming convention. Go spells it
    linodeConfigComplete.
    """
    if not env.linode.api_url or not env.linode.token:
        msg = "linode configuration is incomplete: check your API URL and token"
        raise ValueError(msg)


async def execute_tool(
    cfg: Config,
    arguments: dict[str, Any],
    error_action: str,
    callback: Callable[[RetryableClient], Awaitable[dict[str, Any]]],
    *,
    failure: str = "",
) -> list[TextContent]:
    """Run a tool handler with standard environment/client/error boilerplate.

    failure is the whole sentence a failed Linode call answers with, already
    rendered except for the failure itself, for the tools whose proto contract
    declares one under error_message. Passing the sentence rather than a verb
    is what lets a declared message name the ids it was called with, which
    "Failed to {error_action}" cannot reach.
    """
    environment = arguments.get("environment", "")
    try:
        selected_env = _select_environment(cfg, environment)
        _linode_config_complete(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
            cfg.object_storage,
        ) as client:
            response = await callback(client)
            return [TextContent(type="text", text=json.dumps(response, indent=2))]
    except Exception as e:
        # A decode failure is the read's failure, not the caller's: it takes
        # the dry-run wording Go gives it rather than the argument-error one.
        if isinstance(e, (EnvironmentNotFoundError, ValueError)) and not isinstance(
            e, json.JSONDecodeError
        ):
            return [TextContent(type="text", text=f"Error: {e}")]
        reported = (
            failure.format(error=e) if failure else f"Failed to {error_action}: {e}"
        )
        if isinstance(e, (APIError, NetworkError, httpx.HTTPError)):
            return [TextContent(type="text", text=reported)]
        logger.exception("Unexpected error in tool handler")
        return [TextContent(type="text", text=reported)]


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
    _linode_config_complete(selected_env)
    async with RetryableClient(
        selected_env.linode.api_url,
        selected_env.linode.token,
        _retry_config_from(cfg),
        cfg.object_storage,
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
        _linode_config_complete(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
            cfg.object_storage,
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
        # A decode failure is the read's failure, not the caller's: it takes
        # the dry-run wording Go gives it rather than the argument-error one.
        if isinstance(e, (EnvironmentNotFoundError, ValueError)) and not isinstance(
            e, json.JSONDecodeError
        ):
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
        _linode_config_complete(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            _retry_config_from(cfg),
            cfg.object_storage,
        ) as client:
            response = await callback(client)
            return [TextContent(type="text", text=json.dumps(response, indent=2))]
    except Exception as e:
        # A decode failure is the read's failure, not the caller's: it takes
        # the dry-run wording Go gives it rather than the argument-error one.
        if isinstance(e, (EnvironmentNotFoundError, ValueError)) and not isinstance(
            e, json.JSONDecodeError
        ):
            return [TextContent(type="text", text=f"Error: {e}")]
        if isinstance(e, (APIError, NetworkError, httpx.HTTPError)):
            return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]
        logger.exception("Unexpected error in tool handler")
        return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]


def error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


def meta_response(response: str, **members: Any) -> list[TextContent]:
    """Answer a meta tool with the message its contract names.

    A meta tool reaches no Linode route, so nothing decodes a body into its
    answer and no driver assembles one: the members are what the handler read
    off the call. Serializing here rather than in each generated handler is what
    keeps that answer canonical, the way Go's MarshalProtoToolResponse does.
    """
    message = message_class(response)(**members)
    return [
        TextContent(
            type="text", text=json.dumps(proto_to_canonical_dict(message), indent=2)
        )
    ]


def path_int(value: object) -> int:
    """Read a numeric path argument, answering 0 for anything that is not one.

    This is Go's request.GetInt for the generated handlers: it answers the
    absent value rather than raising, so a caller who sent "abc" for an id gets
    the tool's own "<name> is required" sentence from the check that follows.
    Coercing with int() instead raised out of the handler, which reached the
    caller as an unhandled failure in one language only.

    A bool is not an id. Python counts it as an int, so True would otherwise
    address resource 1.

    A removal reads its ids through destroy_id instead, which refuses what this
    one reads past, the way Go's own destroy tier does.
    """
    if isinstance(value, bool):
        return 0

    if isinstance(value, int):
        return value

    # Go's GetInt truncates a float64, so 42.0 addresses resource 42 there;
    # answering 0 here sent the two languages to different resources.
    if isinstance(value, float):
        # Go's JSON decoder refuses NaN and Infinity, so only this language can
        # reach int() with one, where it raises out of the handler.
        return int(value) if math.isfinite(value) else 0

    if isinstance(value, str):
        # Go reads a string id with strconv.Atoi, which allows a sign and ASCII
        # digits and nothing else; int() also took " 5 ", "4_5", and non-ASCII
        # digits, which addressed a resource Go answered as absent.
        return int(value) if _ATOI.fullmatch(value) else 0

    return 0


def path_str(value: object) -> str:
    """Read a text path argument, answering "" for anything that is not text.

    This is Go's request.GetString: a wrong-typed value reads as absent, so the
    caller gets the tool's own "<name> is required" sentence. Reading the
    argument raw instead spliced a number the caller sent into the path, where
    Go had already refused the call.
    """
    return value if isinstance(value, str) else ""


def destroy_id(arguments: Mapping[str, Any], name: str) -> tuple[int, str]:
    """Read one integer id a removal is addressed by, and the sentence it fails on.

    Mirrors Go's tools.DestroyID rather than path_int: a removal refuses a value
    that is not a whole positive number instead of reading past it, since
    truncating 456.5 or coercing "456" would remove resource 456 in one language
    while the other refused the call. Answers ``(0, sentence)`` on refusal.
    """
    if name not in arguments:
        return 0, f"{name} is required"

    value = _whole_number(arguments[name])
    if value is None or value < 0:
        return 0, f"{name} must be a positive integer"

    if value == 0:
        return 0, f"{name} is required"

    return value, ""


def _whole_number(value: object) -> int | None:
    """The whole number a value is, or None when it is not one.

    Mirrors Go's numberArgToInt: a string is never a number here, and a float
    counts only when it carries no fraction.
    """
    if isinstance(value, bool):
        return None

    if isinstance(value, int):
        return value

    if isinstance(value, float):
        # False for a fraction and for the infinities and NaN alike, so int()
        # is only ever reached with a value it can convert.
        if not value.is_integer():
            return None

        return int(value)

    return None


def required_int_id(arguments: dict[str, Any], name: str) -> tuple[int | None, str]:
    """Validate a required positive-integer id path argument (Option B).

    Returns ``(id, "")`` on success and ``(None, message)`` on failure, mirroring
    the Go requiredIDArgument helper's ``(int, string)`` pair so callers can guard
    with ``if id is None:`` and still hand the message straight to
    ``error_response``. Absent key -> ``"<name> is required"``; present but not a
    positive integer (bool, non-number, zero, negative) -> ``"<name> must be a
    positive integer"``. A present-but-null value is treated as invalid (matches
    Go, which reaches its numeric parser for an explicit null).

    The number test is Go's own, so a whole float counts: refusing 5.0 here
    rejected a call Go addressed resource 5 with.
    """
    return declared_int_id(arguments, name, 0, "", "")


def declared_or(declared: str, own: str) -> str:
    """The sentence a declaration words for one refusal arm, or the reader's own.

    Mirrors Go's declaredOr. A declaration that words nothing reads exactly as
    it did before `reader_message` existed.
    """
    return declared or own


def declared_int_id(
    arguments: dict[str, Any], name: str, maximum: int, absent: str, refused: str
) -> tuple[int | None, str]:
    """The id reader answering the sentences one declaration words.

    Mirrors Go's DeclaredIDArgument. A maximum of 0 leaves the id unbounded,
    and an empty arm falls back to the pair required_int_id documents.
    """
    if name not in arguments:
        return None, declared_or(absent, f"{name} is required")

    value = _whole_number(arguments[name])
    if value is None or value < 1 or (maximum > 0 and value > maximum):
        return None, declared_or(refused, f"{name} must be a positive integer")

    return value, ""


def required_bounded_int_id(
    arguments: dict[str, Any], name: str
) -> tuple[int | None, str]:
    """required_int_id plus the largest id JSON carries exactly, as a ceiling.

    Mirrors Go's RequiredBoundedIDArgument with MaxJSONSafeID. Past 2**53 - 1 a
    JSON number stops round-tripping, so the id reaching the route is not the one
    the caller wrote; Go has refused that since these parsers were written and
    Python accepted it, which is the divergence this closes.

    The ceiling reuses "<name> must be a positive integer" rather than wording
    itself, because that is the sentence Go answers for an oversized id too.
    """
    return declared_int_id(arguments, name, MAX_JSON_SAFE_ID, "", "")


def member_choice(
    arguments: dict[str, Any],
    name: str,
    required: bool,
    members: tuple[str, ...],
) -> tuple[str, str]:
    """Hold one raw text argument to the member names its contract declares.

    Mirrors Go's MemberChoiceArgument, for the fields whose vocabulary a rule
    cannot see: an enum argument naming no member decodes to the enum's zero,
    the same as an absent one. The generated handlers pass the names, sentinel
    excluded, in enum-number order.

    An absent, non-string, or empty argument answers "<name> is required" when
    the field is required and is accepted when it is optional. Any other string
    outside the members answers "<name> must be one of: a, b", or
    "<name> must be <a>" for a vocabulary of one.
    """
    return declared_member_choice(arguments, name, required, "", "", members)


def declared_member_choice(
    arguments: dict[str, Any],
    name: str,
    required: bool,
    absent: str,
    refused: str,
    members: tuple[str, ...],
) -> tuple[str, str]:
    """The membership reader answering the sentences one declaration words.

    Mirrors Go's DeclaredMemberChoice: a declared ``refused`` stands in for the
    sentence built from the member names, which is how a vocabulary of one keeps
    a "must be one of:" wording a caller has always read.
    """
    value = arguments.get(name)
    if not isinstance(value, str) or not value:
        if required:
            return "", declared_or(absent, f"{name} is required")

        return "", ""

    if value in members:
        return value, ""

    if refused:
        return "", refused

    if len(members) == 1:
        return "", f"{name} must be {members[0]}"

    return "", f"{name} must be one of: {', '.join(members)}"


def required_present(arguments: dict[str, Any], name: str) -> tuple[None, str]:
    """Whether the caller sent an argument at all, apart from whether it is usable.

    Mirrors Go's RequiredPresentArgument. A repeated body field cannot be asked
    this through the message: proto3 reads an absent list and an empty one alike,
    and an empty list is a legal value on the routes that use this, meaning
    "remove every one". The answer is None because presence says nothing about
    the value, which the body builder reads for itself.
    """
    if name not in arguments:
        return None, f"{name} is required"

    return None, ""


def present_text(
    arguments: dict[str, Any], name: str, required: bool
) -> tuple[None, str]:
    """One text argument every unusable shape of which answers "is required".

    Mirrors Go's PresentTextArgument: the hooks this replaces never told a
    missing value from one sent as a number or as spaces. required is False
    for an ``optional`` field, where only a present-but-unusable value is
    refused.
    """
    return declared_present_text(arguments, name, required, "", "")


def declared_present_text(
    arguments: dict[str, Any], name: str, required: bool, absent: str, unusable: str
) -> tuple[None, str]:
    """The text reader answering the sentences one declaration words.

    Mirrors Go's DeclaredPresentText. Wording the two arms apart is what lets a
    tool that has always answered "<name> must be a non-empty string" to a blank
    value keep saying it while an absent one still reads as missing.
    """
    if name not in arguments:
        if required:
            return None, declared_or(absent, f"{name} is required")
        return None, ""

    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        return None, declared_or(unusable, f"{name} is required")

    return None, ""


def present_bool(
    arguments: dict[str, Any], name: str, required: bool
) -> tuple[None, str]:
    """One argument the caller must have sent as a boolean.

    Mirrors Go's PresentBoolArgument: an implicit-presence bool reads absent
    and false alike, so the argument map is what the question is asked of.
    required is False for an ``optional`` field, where an absent flag is
    accepted and only a present non-boolean is refused.
    """
    return declared_present_bool(arguments, name, required, "", "")


def declared_present_bool(
    arguments: dict[str, Any], name: str, required: bool, absent: str, unusable: str
) -> tuple[None, str]:
    """The boolean reader answering the sentences one declaration words.

    Mirrors Go's DeclaredPresentBool, for the tools that tell a caller who sent
    no flag from one who sent something that is not a flag.
    """
    if name not in arguments:
        if required:
            return None, declared_or(absent, f"{name} must be a boolean")
        return None, ""

    if not isinstance(arguments.get(name), bool):
        return None, declared_or(unusable, f"{name} must be a boolean")

    return None, ""


def require_any_argument(arguments: dict[str, Any], sentence: str, *names: str) -> str:
    """Answer sentence when the caller sent none of the named arguments.

    Mirrors Go's RequireAnyArgument, reading the map because the updates that
    ask this accept a field's zero as a real change (an empty tags list clears
    the tags), so absent and empty conflate anywhere later.
    """
    if any(name in arguments for name in names):
        return ""

    return sentence


def present_string(
    arguments: dict[str, Any], name: str, required: bool
) -> tuple[None, str]:
    """One argument the caller must have sent as a string, blank included.

    Mirrors Go's PresentStringArgument. present_text's sibling over the wider
    set: a blank string is a value on the routes that declare this, so refusing
    it would refuse the call that clears a setting. required is False for an
    ``optional`` field, where an absent argument is accepted.
    """
    return declared_present_string(arguments, name, required, "", "")


def declared_present_string(
    arguments: dict[str, Any], name: str, required: bool, absent: str, unusable: str
) -> tuple[None, str]:
    """The string reader answering the sentences one declaration words.

    Mirrors Go's DeclaredPresentString.
    """
    if name not in arguments:
        if required:
            return None, declared_or(absent, f"{name} must be a string")
        return None, ""

    if not isinstance(arguments.get(name), str):
        return None, declared_or(unusable, f"{name} must be a string")

    return None, ""


def id_list(arguments: dict[str, Any], name: str) -> tuple[None, str]:
    """Hold one argument to being a list of distinct positive ids."""
    return declared_id_list(arguments, name, "", "", "")


def declared_id_list(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[None, str]:
    """The id-list reader answering the sentences one declaration words.

    Mirrors Go's DeclaredIDList. It reads the argument map because proto3 reads
    an absent list and an empty one as the same empty list, and the routes
    declaring this answer those two differently.
    """
    if name not in arguments:
        return None, declared_or(absent, f"{name} is required")

    raw = arguments[name]
    if not isinstance(raw, list):
        return None, declared_or(
            unusable, f"{name} must be a JSON array of positive integers"
        )

    entries = cast("list[object]", raw)

    shape = declared_or(
        refused, f"{name} must be a non-empty array of distinct positive integers"
    )
    if not entries:
        return None, shape

    seen: set[int] = set()
    for entry in entries:
        value = _whole_number(entry)
        if value is None or value < 1 or value in seen:
            return None, shape
        seen.add(value)

    return None, ""


def refused_arguments(
    arguments: dict[str, Any], sentence: str, names: tuple[str, ...]
) -> str:
    """Answer for any of the named arguments the caller set.

    Mirrors Go's RefusedArguments. The names never reach the input message, so
    the argument map is the only place a caller's value is still visible.
    """
    supplied = sorted(name for name in names if name in arguments)
    if not supplied:
        return ""

    return _fill_refused_names(sentence, supplied)


def unknown_arguments(
    arguments: dict[str, Any], sentence: str, declared: tuple[str, ...]
) -> str:
    """Answer for any argument the tool's input message does not declare.

    Mirrors Go's UnknownArguments.
    """
    unknown = sorted(name for name in arguments if name not in declared)
    if not unknown:
        return ""

    return _fill_refused_names(sentence, unknown)


def _fill_refused_names(sentence: str, names: list[str]) -> str:
    """Write the refused names into a declared sentence, sorted by the caller."""
    filled = sentence.replace("{fields}", ", ".join(names))

    return filled.replace("{field}", names[0])


# The declared argument rewrites. A tool names its transforms through
# normalize_fields and the generated handler calls the one below that renders
# it, before anything reads an argument, so every later read sees one value.


def trim_arguments(arguments: dict[str, Any], *names: str) -> None:
    """Drop surrounding whitespace from the named text arguments in place.

    Mirrors Go's TrimArguments. A value that arrived as anything else is left
    alone, so the body builder still refuses it by type rather than reading one
    this rewrote.
    """
    for name in names:
        value = arguments.get(name)
        if isinstance(value, str):
            arguments[name] = value.strip()


def trim_list_drop_blank(arguments: dict[str, Any], *names: str) -> None:
    """Trim every entry of the named list arguments and drop the blanks.

    Mirrors Go's TrimListDropBlank. A list holding anything but text is left
    whole: the body builder words that refusal for the whole surface alike.
    """
    for name in names:
        value = arguments.get(name)
        if not isinstance(value, list):
            continue

        entries = cast("list[object]", value)
        if all(isinstance(entry, str) for entry in entries):
            arguments[name] = [
                trimmed for entry in entries if (trimmed := cast("str", entry).strip())
            ]


def trim_list(arguments: dict[str, Any], *names: str) -> None:
    """Trim every text entry of the named list arguments in place, keep blanks.

    Mirrors Go's TrimList. An entry that was only padding becomes the blank the
    rule refuses by name rather than silently disappearing; entries that are
    not text are left for the type refusal.
    """
    for name in names:
        value = arguments.get(name)
        if not isinstance(value, list):
            continue

        entries = cast("list[object]", value)
        arguments[name] = [
            entry.strip() if isinstance(entry, str) else entry for entry in entries
        ]


def uppercase_arguments(arguments: dict[str, Any], *names: str) -> None:
    """Fold the named text arguments to upper case in place.

    Mirrors Go's UppercaseArguments: a value the API reads as an upper-case
    vocabulary reaches the rules that way however the caller spelled it.
    """
    for name in names:
        value = arguments.get(name)
        if isinstance(value, str):
            arguments[name] = value.upper()


def fold_int_list(
    arguments: dict[str, Any], source: str, target: str, key: str
) -> None:
    """Fold a convenience integer-list argument into a member of an object one.

    Mirrors Go's FoldIntList: a caller-supplied non-empty target wins and the
    source is dropped, so the wire never carries both spellings of one fact; a
    target or source no reader can parse is left alone so the rules and the
    body builder refuse it by name. Integral floats fold the way every body
    number does, since a JSON-RPC number reaches Go as one.
    """
    target_value = arguments.get(target)
    if target_value is not None and not isinstance(target_value, dict):
        return

    if target_value:
        arguments.pop(source, None)
        return

    if source not in arguments:
        return

    values = _positive_int_entries(arguments[source])
    if values is None:
        return

    arguments[target] = {key: values}
    del arguments[source]


def _positive_int_entries(raw: object) -> list[int] | None:
    """The positive integers a fold source holds, or None when it is unusable.

    ``type(entry) is int`` rather than ``isinstance`` because bool subclasses
    int; a float passes only when exactly integral, matching the body reader.
    """
    if not isinstance(raw, list) or not raw:
        return None

    values: list[int] = []
    for entry in cast("list[object]", raw):
        if type(entry) is int and entry > 0:
            values.append(entry)
        elif isinstance(entry, float) and entry.is_integer() and entry > 0:
            values.append(int(entry))
        else:
            return None
    return values


# The named format readers, one body per member, reached from generated code
# through the argument_reader each field declares. Go spells the same three in
# tools/format_readers.go.
_SERVICE_TYPE_SLUG_PATTERN = re.compile(r"^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$")
_REGION_SLUG_PATTERN = re.compile(r"^[a-z0-9-]+$")
_BETA_SLUG_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")


def service_type_slug(arguments: dict[str, Any], name: str) -> tuple[str, str]:
    """Read a monitoring service type, carried as a bare path segment."""
    return declared_service_type_slug(arguments, name, "", "", "")


def declared_service_type_slug(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[str, str]:
    """The service-type reader answering the sentences one declaration words.

    Mirrors Go's DeclaredServiceTypeSlug. Surrounding space is refused rather
    than trimmed, because a trimmed value addresses a different route than the
    caller spelled.
    """
    if name not in arguments:
        return "", declared_or(absent, f"{name} is required")

    value = arguments[name]
    if not isinstance(value, str):
        return "", declared_or(unusable, f"{name} must be a string")

    if _SERVICE_TYPE_SLUG_PATTERN.fullmatch(value) is None:
        return "", declared_or(
            refused, f"{name} must be a single non-empty service type slug"
        )

    return value, ""


def region_slug(arguments: dict[str, Any], name: str) -> tuple[str, str]:
    """Read a region id, which Linode spells as a lowercase slug."""
    return declared_region_slug(arguments, name, "", "", "")


def declared_region_slug(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[str, str]:
    """The region reader answering the sentences one declaration words.

    Mirrors Go's DeclaredRegionSlug. The accepted set is exactly what the
    refusal names, so the stricter spellings these routes grew apart into are
    gone: no sentence ever told a caller that a region may not begin with a
    hyphen.
    """
    value, message = _non_blank_text(arguments, name, absent, unusable)
    if message:
        return "", message

    if _REGION_SLUG_PATTERN.fullmatch(value) is None:
        return "", declared_or(
            refused,
            f"{name} must be a lowercase region slug containing only letters,"
            " numbers, and hyphens",
        )

    return value, ""


def beta_slug(arguments: dict[str, Any], name: str) -> tuple[str, str]:
    """Read a beta program id, which the API spells as a slug not a number."""
    return declared_beta_slug(arguments, name, "", "", "")


def declared_beta_slug(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[str, str]:
    """The beta-program reader answering the sentences one declaration words.

    Mirrors Go's DeclaredBetaSlug. Wider than the region member by case and the
    underscore, which is the whole difference between the two.
    """
    value, message = _non_blank_text(arguments, name, absent, unusable)
    if message:
        return "", message

    if value != value.strip() or _BETA_SLUG_PATTERN.fullmatch(value) is None:
        return "", declared_or(
            refused,
            f"{name} must contain only letters, numbers, underscores, and hyphens",
        )

    return value, ""


def free_text(arguments: dict[str, Any], name: str) -> tuple[str, str]:
    """Read a path segment whose shape the message's own rules judge."""
    return declared_free_text(arguments, name, "", "")


def declared_free_text(
    arguments: dict[str, Any], name: str, absent: str, unusable: str
) -> tuple[str, str]:
    """The free-text reader answering the sentences one declaration words.

    Mirrors Go's DeclaredFreeText. It refuses nothing a rule can see; what it
    adds is the answer for a caller who sent a non-string, which no rule reaches
    because a message that cannot be built evaluates none.
    """
    return _non_blank_text(arguments, name, absent, unusable)


def _non_blank_text(
    arguments: dict[str, Any], name: str, absent: str, unusable: str
) -> tuple[str, str]:
    """The two arms the slug readers share before they judge a charset."""
    if name not in arguments:
        return "", declared_or(absent, f"{name} is required")

    value = arguments[name]
    if not isinstance(value, str) or not value.strip():
        return "", declared_or(unusable, f"{name} must be a non-empty string")

    return value, ""


# The RFC 3339 grammar Go's time.Parse accepts for the audit window bounds:
# uppercase T and Z, whole seconds, an optional fraction, or a numeric offset.
_RFC3339 = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})")


def read_optional_time(value: str) -> tuple[datetime | None, str]:
    """Parse an RFC 3339 timestamp, answering the cause rather than raising.

    The cause names what the value got wrong and nothing else, because the
    sentence around it belongs to whichever tool read the bound.

    fromisoformat alone would take spellings Go's time.Parse(time.RFC3339, ...)
    refuses, such as a bare date or a space separator, so the same window
    argument answered events in one language and a refusal in the other. The
    pattern is the RFC 3339 grammar Go accepts: an uppercase T, whole seconds,
    an optional fraction, and Z or a numeric offset.
    """
    if not value:
        return None, ""

    if not _RFC3339.fullmatch(value):
        return None, _timestamp_cause(value, "not RFC 3339")

    try:
        return datetime.fromisoformat(value), ""
    except ValueError as exc:
        return None, _timestamp_cause(value, str(exc))


def _timestamp_cause(value: str, reason: str) -> str:
    """What a malformed bound got wrong, without naming the bound."""
    return f"expected RFC 3339, got {value!r}: {reason}"


# Tag-validation messages, held identical to Go's ErrTagsMustBeJSONStringArray
# and ErrTagsEntriesNonEmpty so both clients reject the same input the same way.
TAGS_MUST_BE_JSON_STRING_ARRAY = "tags must be a JSON string array"
TAGS_ENTRIES_NON_EMPTY = "tags entries must be non-empty strings"


def _tag_element_list(raw: object) -> list[object] | None:
    """Unwrap a tags argument to its element list, or None when it is not one.

    A JSON-encoded string is accepted the way Go's tagsValueFromToolArg accepts
    one, so a client that cannot send a native array still reaches the same
    request body.
    """
    if isinstance(raw, str):
        try:
            raw = json.loads(raw.strip())
        except ValueError:
            return None
    if not isinstance(raw, list):
        return None
    return cast("list[object]", raw)


def optional_tags_argument(
    arguments: dict[str, Any],
) -> tuple[list[str] | None, str]:
    """Read the optional ``tags`` body array, mirroring Go's tagsValueFromToolArg.

    Returns ``(tags, "")`` when the argument is absent (tags is None) or valid,
    and ``(None, message)`` otherwise. Entries are trimmed and an entry that is
    empty after trimming is rejected, matching Go, so the two clients never
    disagree on which tag set the API sees.
    """
    if "tags" not in arguments:
        return None, ""

    values = _tag_element_list(arguments["tags"])
    if values is None:
        return None, TAGS_MUST_BE_JSON_STRING_ARRAY

    tags: list[str] = []
    for value in values:
        if not isinstance(value, str):
            return None, TAGS_MUST_BE_JSON_STRING_ARRAY
        trimmed = value.strip()
        if not trimmed:
            return None, TAGS_ENTRIES_NON_EMPTY
        tags.append(trimmed)
    return tags, ""


# Standard Linode collection pagination bounds, shared by every family that has
# no bounds of its own. Mirrors the Go standardPageSizeMin/Max pair.
STANDARD_PAGE_SIZE_MIN = 25
STANDARD_PAGE_SIZE_MAX = 500


def standard_pagination_arguments(
    arguments: dict[str, Any],
) -> tuple[int | None, int | None]:
    """Read page/page_size under the standard Linode bounds.

    Raises the same TypeError/ValueError ``pagination_int_argument`` raises, so
    the validation text stays identical to Go's standardPaginationFromTool.
    """
    page = pagination_int_argument(arguments, "page", 1)
    page_size = pagination_int_argument(
        arguments, "page_size", STANDARD_PAGE_SIZE_MIN, STANDARD_PAGE_SIZE_MAX
    )
    return page, page_size


def pagination_query(page: int | None, page_size: int | None) -> str:
    """Encode page/page_size as a query string, omitting unset values.

    Mirrors Go's withPaginationQuery: an unset value stays off the query string
    so the API's own default applies, which keeps the two languages issuing
    byte-identical requests for the same arguments. Empty when neither is set,
    which the route primitive reads as "no query" rather than a bare "?".
    """
    params: dict[str, int] = {}
    if page is not None:
        params["page"] = page
    if page_size is not None:
        params["page_size"] = page_size
    if not params:
        return ""
    return urlencode(params)


def paginated_path(path: str, page: int | None, page_size: int | None) -> str:
    """Append page/page_size to a raw request path, omitting unset values."""
    query = pagination_query(page, page_size)
    if not query:
        return path
    return path + "?" + query


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
