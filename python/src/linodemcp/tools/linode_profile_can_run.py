"""``linode_profile_can_run`` pre-check tool (dry-run spec Phase 3).

Answers "would the active profile permit this sequence of tool calls?" so the
model can bail before partial execution strands the user. The tool carries
``Capability.Meta`` so the profile filter always admits it. It inspects only
each call's tool name and optional ``environment`` arg against the active
profile; it never checks resource IDs, token scope, resource existence, or
rate limits. Pre-check is advice, not a transactional plan.

The generator owns this tool's registration; what is left here is the body its
answer hook calls. It reads the live tool catalog and the active profile off
the builder state the server publishes for the call
(:mod:`linodemcp.tools.builderstate`), both through callables read at call
time, so a profile reload reaches this already-registered tool.
"""

from __future__ import annotations

import json
from typing import Any, cast

from mcp.types import TextContent

from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    builder_state_from_context,
)
from linodemcp.tools.helpers import error_response
from linodemcp.tools.proto_response import serialize_api_response

# Reason strings: an exact-match contract with the Go side and the spec. The
# summary bucketing keys off them, so they must not drift.
_REASON_UNREGISTERED = "tool name not registered"
_REASON_PROFILE_BLOCK = "tool not in profile's allowed_tools"
_REASON_ENV_BLOCK = "environment not permitted by profile"

_BUCKET_UNREGISTERED = "unregistered"
_BUCKET_PROFILE_BLOCK = "profile_block"
_BUCKET_ENV_BLOCK = "environment_block"
_BUCKET_CAPABILITY = "capability_block"

_ARG_CALLS = "calls"
_ENTRY_TOOL = "tool"
_ENTRY_ARGS = "args"
_ENTRY_ENV = "environment"
_ENV_WILDCARD = "*"


def _permits_all_environments(envs: list[str]) -> bool:
    """Report whether the profile imposes no environment restriction.

    Mirrors ``Profile.allowed_environments`` semantics: an empty list, or a
    list whose only entry is ``"*"``, allows every configured environment.
    """
    return len(envs) == 0 or (len(envs) == 1 and envs[0] == _ENV_WILDCARD)


def _entry_environment(entry: dict[str, Any]) -> tuple[str, bool]:
    """Extract the optional environment arg from a call entry's args object.

    Returns ``("", False)`` when args or environment is absent or not a
    non-empty string.
    """
    raw_args = entry.get(_ENTRY_ARGS)
    if not isinstance(raw_args, dict):
        return "", False

    env = cast("dict[str, Any]", raw_args).get(_ENTRY_ENV)
    if not isinstance(env, str) or not env:
        return "", False

    return env, True


def _evaluate_call(
    tool_name: str,
    env: str,
    *,
    has_env: bool,
    registered: dict[str, Capability],
    allowed_tools: set[str],
    allowed_envs: list[str],
    all_envs: bool,
) -> tuple[dict[str, Any], str]:
    """Resolve a single call to its verdict dict and summary bucket key.

    The bucket key is the empty string when the call is allowed. Refusal
    order mirrors real dispatch: unregistered, then profile membership, then
    environment.
    """
    result: dict[str, Any] = {"tool": tool_name, "allowed": False}

    capability = registered.get(tool_name)
    if capability is None:
        result["reason"] = _REASON_UNREGISTERED
        result["remedy"] = (
            "check spelling or call linode_profile_list_tools to discover "
            "the registered tool surface"
        )
        return result, _BUCKET_UNREGISTERED

    if tool_name not in allowed_tools:
        if capability == Capability.Destroy:
            result["reason"] = f"{_REASON_PROFILE_BLOCK} (Cap{capability.name})"
            result["remedy"] = (
                f"switch to a profile that permits {tool_name}, or use yolo "
                "on a profile that allows it"
            )
            return result, _BUCKET_CAPABILITY

        result["reason"] = _REASON_PROFILE_BLOCK
        result["remedy"] = (
            f"switch to a profile that permits {tool_name}, or add it to the "
            "current profile"
        )
        return result, _BUCKET_PROFILE_BLOCK

    if has_env and not all_envs and env not in allowed_envs:
        result["reason"] = _REASON_ENV_BLOCK
        result["remedy"] = (
            "target an environment in the profile's allowed_environments, or "
            "switch to a profile that permits this environment"
        )
        return result, _BUCKET_ENV_BLOCK

    result["allowed"] = True
    return result, ""


def profile_can_run_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Pre-check the requested call sequence against the active profile."""
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    profile = state.active_profile()
    registered = {entry.name: entry.capability for entry in state.catalog()}
    allowed_tools: set[str] = set(profile.allowed_tools)
    allowed_envs: list[str] = list(profile.allowed_environments)
    all_envs = _permits_all_environments(allowed_envs)

    raw_calls = arguments.get(_ARG_CALLS)
    calls: list[object] = (
        cast("list[object]", raw_calls) if isinstance(raw_calls, list) else []
    )

    results: list[dict[str, Any]] = []
    buckets = {
        _BUCKET_UNREGISTERED: 0,
        _BUCKET_PROFILE_BLOCK: 0,
        _BUCKET_ENV_BLOCK: 0,
        _BUCKET_CAPABILITY: 0,
    }
    allowed_count = 0

    for raw in calls:
        if not isinstance(raw, dict):
            continue

        entry = cast("dict[str, Any]", raw)
        tool_name = entry.get(_ENTRY_TOOL)
        if not isinstance(tool_name, str):
            tool_name = ""

        env, has_env = _entry_environment(entry)
        result, bucket = _evaluate_call(
            tool_name,
            env,
            has_env=has_env,
            registered=registered,
            allowed_tools=allowed_tools,
            allowed_envs=allowed_envs,
            all_envs=all_envs,
        )
        results.append(result)

        if result["allowed"]:
            allowed_count += 1
        else:
            buckets[bucket] += 1

    response = {
        "active_profile": profile.name,
        "results": results,
        "summary": {
            "total": len(results),
            "allowed": allowed_count,
            "blocked": len(results) - allowed_count,
            "blocked_by_reason": buckets,
        },
    }

    result = serialize_api_response(
        response, profile_builder_pb2.ProfileCanRunResponse()
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]
