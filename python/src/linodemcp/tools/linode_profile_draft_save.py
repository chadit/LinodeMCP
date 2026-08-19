"""Phase 8.5 draft save builder tool.

Wraps the Phase 8.4 draft state with a confirm-gated write to the
config file. Computes the diff against the prior user-defined profile
with the same name (or against empty for a new profile) and returns
it in the response so the model can summarize the change.

Does NOT change the active profile. After save, the user runs
``linodemcp profile use <name>`` (or the equivalent step) to switch.
"""

from __future__ import annotations

import json
from typing import Any

from mcp.types import TextContent

from linodemcp.config import ConfigError, get_config_path, load_from_file, write_atomic
from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles.builder import (
    compute_diff,
    draft_as_user_profile,
)
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    builder_state_from_context,
)
from linodemcp.tools.helpers import error_response
from linodemcp.tools.linode_profile_draft import NAME_MISSING, draft_not_found
from linodemcp.tools.proto_response import serialize_api_response

# Built-in profile names. Saving a draft to one of these is refused
# (the entry would silently shadow the built-in in the catalog).
# Match Go's profiles.BuiltinXxx constants and the Phase 7c clone
# rejection set.
_BUILTIN_PROFILE_NAMES: frozenset[str] = frozenset(
    {
        "default",
        "readonly-full",
        "compute-admin",
        "network-admin",
        "kubernetes-admin",
        "storage-admin",
        "full-access",
        "emergency",
    }
)


_ARG_NAME = "name"


def _argument_refusal(arguments: dict[str, Any]) -> str:
    """The refusal the save answers from its arguments alone, or "".

    The confirm gate is NOT here: the generated handler runs it from the
    contract's confirm_message before this is reached, so a copy would be a
    check no call could get past. A built-in name is refused whether or not a
    draft carries it, which is why that one is asked ahead of the lookup.
    """
    name = arguments.get(_ARG_NAME, "")
    if not name:
        return NAME_MISSING

    if name in _BUILTIN_PROFILE_NAMES:
        return f"cannot save over built-in profile name: {name}"

    return ""


def profile_draft_save_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Save a draft to the config file and return the diff payload.

    Every refusal is a tool result: an absent name, a missing confirm, a
    built-in name, a name no draft carries, and a config that cannot be read
    or written.
    """
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    refusal = _argument_refusal(arguments)
    if refusal:
        return error_response(refusal)

    name = arguments[_ARG_NAME]
    draft = state.drafts.get(name)
    if draft is None:
        return error_response(draft_not_found(name))

    # Read fresh on every call so a concurrent edit is not stomped, and read
    # the path itself at call time so a LINODEMCP_CONFIG_PATH override applies.
    path = get_config_path()

    try:
        cfg = load_from_file(path)
    except (ConfigError, OSError) as exc:
        return error_response(f"failed to load config from {path}: {exc}")

    draft_cfg = draft_as_user_profile(draft)
    existing = cfg.profiles.get(name)

    diff = compute_diff(name, draft_cfg, existing)

    cfg.profiles[name] = draft_cfg

    try:
        write_atomic(path, cfg)
    except (ConfigError, OSError) as exc:
        return error_response(f"failed to write config to {path}: {exc}")

    result = serialize_api_response(
        diff.to_payload(), profile_builder_pb2.ProfileDraftSaveResponse()
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


__all__ = [
    "profile_draft_save_result",
]
