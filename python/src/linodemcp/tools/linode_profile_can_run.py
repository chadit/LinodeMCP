"""``linode_profile_can_run`` pre-check tool (dry-run spec Phase 3).

Answers "would the active profile permit this sequence of tool calls?" so the
model can bail before partial execution strands the user. The tool carries
``Capability.Meta`` so the profile filter always admits it. It inspects only
each call's tool name and optional ``environment`` arg against the active
profile; it never checks resource IDs, token scope, resource existence, or
rate limits. Pre-check is advice, not a transactional plan.

The handler reads the live tool catalog and the active profile through two
module-level bridges the server installs at startup. Tests inject
reproducible fixtures via :func:`set_can_run_catalog_provider` and
:func:`set_can_run_active_profile_provider`. The bridges live on a small
holder object so the setters mutate an attribute rather than rebinding a
module global (no ``global`` statement, no lint suppression needed).
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.profiles import Profile
    from linodemcp.profiles.builtin import ToolDescriptor


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


class _Bridges:
    """Holds the catalog and active-profile providers the server installs.

    Mutating attributes on a single module-level instance avoids a ``global``
    rebind in the setters (and the lint suppression that would require).
    """

    catalog: Callable[[], list[ToolDescriptor]] | None = None
    active_profile: Callable[[], Profile] | None = None


_bridges = _Bridges()


def set_can_run_catalog_provider(
    provider: Callable[[], list[ToolDescriptor]] | None,
) -> None:
    """Register the function returning the live tool catalog (or clear it)."""
    _bridges.catalog = provider


def set_can_run_active_profile_provider(
    provider: Callable[[], Profile] | None,
) -> None:
    """Register the function returning the active profile (or clear it)."""
    _bridges.active_profile = provider


def _resolve_catalog() -> list[ToolDescriptor]:
    """Return the live catalog, or an empty list when no bridge is set."""
    provider = _bridges.catalog
    return provider() if provider is not None else []


def _resolve_active_profile() -> Profile | None:
    """Return the active profile, or None when no bridge is set."""
    provider = _bridges.active_profile
    return provider() if provider is not None else None


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


def create_linode_profile_can_run_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_can_run`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_can_run",
            description=(
                "Pre-check whether the active profile would permit a sequence "
                "of tool calls before executing any of them. Returns a "
                "per-call allowed/blocked verdict with a reason and remedy, "
                "plus a summary. Inspects only the tool name and optional "
                "environment arg, not resource IDs. Advice only; it does not "
                "execute anything."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_CALLS: {
                        "type": "array",
                        "description": (
                            "Tool calls to pre-check. Each entry is an object "
                            'with a required "tool" name and optional "args" '
                            '(only "environment" is inspected).'
                        ),
                        "items": {
                            "type": "object",
                            "properties": {
                                _ENTRY_TOOL: {"type": "string"},
                                _ENTRY_ARGS: {"type": "object"},
                            },
                            "required": [_ENTRY_TOOL],
                        },
                    },
                },
                "required": [_ARG_CALLS],
            },
        ),
        Capability.Meta,
    )


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


async def handle_linode_profile_can_run(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Pre-check the requested call sequence against the active profile."""
    profile = _resolve_active_profile()
    registered = {entry.name: entry.capability for entry in _resolve_catalog()}
    allowed_tools: set[str] = (
        set(profile.allowed_tools) if profile is not None else set()
    )
    allowed_envs: list[str] = (
        list(profile.allowed_environments) if profile is not None else []
    )
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
        "active_profile": profile.name if profile is not None else "",
        "results": results,
        "summary": {
            "total": len(results),
            "allowed": allowed_count,
            "blocked": len(results) - allowed_count,
            "blocked_by_reason": buckets,
        },
    }

    return [TextContent(type="text", text=json.dumps(response))]
