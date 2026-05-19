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
from pathlib import Path
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.config import load_from_file, write_atomic
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import (
    DraftNotFoundError,
    compute_diff,
    draft_as_user_profile,
)
from linodemcp.tools.linode_profile_draft import (
    BuilderUnconfiguredError,
    DraftNameMissingError,
    get_draft_registry,
)

if TYPE_CHECKING:
    from collections.abc import Callable


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


# Bridge for the config path. The server installs ``config.get_config_path``
# (or equivalent) at startup; tests install a deterministic path via
# :func:`set_save_config_path_provider`. Phase 8.5 reads from this
# fresh on every call so concurrent edits don't get stomped.
_config_path_provider: Callable[[], str] | None = None


def set_save_config_path_provider(provider: Callable[[], str] | None) -> None:
    """Register the function returning the config path. Pass ``None`` to clear."""
    global _config_path_provider  # noqa: PLW0603 - process-wide bridge
    _config_path_provider = provider


def _resolve_config_path() -> str:
    """Return the live config path or an empty string when no bridge is set."""
    if _config_path_provider is None:
        return ""
    return _config_path_provider()


class ConfirmRequiredError(ValueError):
    """``confirm=true`` is required for the save."""

    def __init__(self) -> None:
        super().__init__("confirm=true is required for draft save")


class SaveBuiltinNameError(ValueError):
    """Save target name matches a built-in profile."""

    def __init__(self, name: str) -> None:
        super().__init__(f"cannot save over built-in profile name: {name}")
        self.profile_name = name


class ConfigPathUnknownError(RuntimeError):
    """No config path provider is wired."""

    def __init__(self) -> None:
        super().__init__("config path not configured")


_ARG_NAME = "name"
_ARG_CONFIRM = "confirm"


def create_linode_profile_draft_save_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_save`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_save",
            description=(
                "Save a profile draft to the config file. Requires "
                "confirm=true. Computes the diff against the prior "
                "user-defined profile with the same name (or against "
                "empty for a new profile) and returns it in the "
                "response so the model can summarize. Does NOT change "
                "the active profile; the user runs `linodemcp profile "
                "use <name>` separately. Saving over a built-in "
                "profile name is refused."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_NAME: {
                        "type": "string",
                        "description": "Draft name to save.",
                    },
                    _ARG_CONFIRM: {
                        "type": "boolean",
                        "description": (
                            "Must be true. The save is a write "
                            "operation that mutates the config file."
                        ),
                    },
                },
                "required": [_ARG_NAME, _ARG_CONFIRM],
            },
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_save(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Save a draft to the config file and return the diff payload.

    Raises:
        DraftNameMissingError: ``name`` is empty.
        ConfirmRequiredError: ``confirm`` is not true.
        SaveBuiltinNameError: ``name`` matches a built-in profile.
        DraftNotFoundError: the named draft is not in the registry.
        ConfigPathUnknownError: no config path provider is wired.
        BuilderUnconfiguredError: no draft registry is wired.
    """
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    if not arguments.get(_ARG_CONFIRM, False):
        raise ConfirmRequiredError

    if name in _BUILTIN_PROFILE_NAMES:
        raise SaveBuiltinNameError(name)

    registry = get_draft_registry()
    if registry is None:
        raise BuilderUnconfiguredError

    draft = registry.get(name)
    if draft is None:
        raise DraftNotFoundError(name)

    path_str = _resolve_config_path()
    if not path_str:
        raise ConfigPathUnknownError

    path = Path(path_str)
    cfg = load_from_file(path)
    draft_cfg = draft_as_user_profile(draft)
    existing = cfg.profiles.get(name)

    diff = compute_diff(name, draft_cfg, existing)

    cfg.profiles[name] = draft_cfg
    write_atomic(path, cfg)

    payload = json.dumps(diff.to_payload())
    return [TextContent(type="text", text=payload)]


__all__ = [
    "ConfigPathUnknownError",
    "ConfirmRequiredError",
    "SaveBuiltinNameError",
    "create_linode_profile_draft_save_tool",
    "handle_linode_profile_draft_save",
    "set_save_config_path_provider",
]
