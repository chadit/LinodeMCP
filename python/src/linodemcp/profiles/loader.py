"""Active-profile resolution against the live tool registry.

Pure function with no I/O: takes a parsed ``Config`` plus the registered
tool descriptors and returns a ``Profile`` whose ``allowed_tools`` is the
explicit, expanded list the registration filter will iterate at Phase 4.

The module deliberately does NOT import ``linodemcp.server`` or
``linodemcp.tools``. Phase 4 will wire the resolver into the server at
startup; this module's job ends with returning the resolved ``Profile``.

Wildcard semantics use ``fnmatch.fnmatch`` (shell-glob: only ``*`` is
special). Resolution order: expand ``allowed_tools`` against the registry,
then subtract expanded ``denied_tools``. Built-in profile overrides
(``profiles_builtin_overrides[name].disabled``) refuse a disabled built-in;
overrides naming a non-built-in are logged and ignored.
"""

from __future__ import annotations

import fnmatch
import logging
from typing import TYPE_CHECKING

from linodemcp.profiles.builtin import builtin_profiles, has_mutating_tools
from linodemcp.profiles.errors import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
)
from linodemcp.profiles.profile import Profile

if TYPE_CHECKING:
    from collections.abc import Sequence

    from linodemcp.config import Config, UserProfileConfig
    from linodemcp.profiles.builtin import ToolDescriptor

logger = logging.getLogger(__name__)

DEFAULT_PROFILE_NAME = "default"

__all__ = ["DEFAULT_PROFILE_NAME", "lookup_profile", "resolve_active_profile"]


def lookup_profile(
    name: str,
    cfg: Config,
    registry: Sequence[ToolDescriptor],
) -> Profile | None:
    """Resolve a profile by name across built-ins and user-defined entries.

    Mirror of Go ``profiles.LookupProfile``. The Phase 8.3 ``_draft_new``
    handler uses this to seed a new draft from the named ``clone_from``
    profile. Unlike :func:`resolve_active_profile` this ignores the
    Disabled flag: callers may clone from a disabled built-in like
    ``full-access`` or ``emergency``. User-defined entries shadow
    built-ins by name (same precedence as the active resolver).

    Returns the materialized ``Profile`` on hit or ``None`` on miss.
    Empty ``name`` returns ``None``; callers that need
    empty-equals-default semantics fall back themselves.
    """
    if not name:
        return None

    if name in cfg.profiles:
        return _resolve_user_profile(name, cfg.profiles[name], registry)

    builtins = builtin_profiles(registry)
    found = builtins.get(name)
    if found is None:
        return None

    # Strip the disabled flag so a clone from a disabled built-in
    # doesn't carry the flag into the new draft.
    return Profile(
        name=found.name,
        description=found.description,
        allowed_tools=found.allowed_tools,
        allowed_environments=found.allowed_environments,
        required_token_scopes=found.required_token_scopes,
        elevated=found.elevated,
        allow_yolo=found.allow_yolo,
        disabled=False,
    )


def _expand_patterns(
    patterns: Sequence[str],
    registry_names: Sequence[str],
    field_label: str,
) -> set[str]:
    """Expand a list of literal or wildcard patterns against the registry.

    Each pattern is fed through ``fnmatch.fnmatch`` so callers get shell-glob
    semantics. A pattern containing ``*`` that matches nothing logs a
    warning. A literal name not present in the registry also logs a warning.
    Both cases drop silently from the resolved set (load continues).

    ``field_label`` shows up in the warning so users can tell whether the
    bad entry was in ``allowed_tools`` or ``denied_tools``.
    """
    registry_set = set(registry_names)
    resolved: set[str] = set()
    for pattern in patterns:
        if "*" in pattern:
            matches = {
                name for name in registry_names if fnmatch.fnmatch(name, pattern)
            }
            if not matches:
                logger.warning(
                    "profile %s pattern %r matched no registered tools",
                    field_label,
                    pattern,
                )
                continue
            resolved.update(matches)
            continue
        if pattern in registry_set:
            resolved.add(pattern)
            continue
        logger.warning(
            "profile %s entry %r is not a registered tool name",
            field_label,
            pattern,
        )
    return resolved


def _resolve_user_profile(
    name: str,
    entry: UserProfileConfig,
    registry: Sequence[ToolDescriptor],
) -> Profile:
    """Build a ``Profile`` from a user-defined config entry.

    Order of operations matches the spec: expand ``allowed_tools``, then
    subtract expanded ``denied_tools``. Explicit deny wins over allow.
    Tool names are sorted in the returned tuple so the resolved profile is
    stable for log output, parity comparison, and golden-file tests.
    """
    registry_names = [tool.name for tool in registry]
    allowed = _expand_patterns(entry.allowed_tools, registry_names, "allowed_tools")
    denied = _expand_patterns(entry.denied_tools, registry_names, "denied_tools")
    resolved_names = tuple(sorted(allowed - denied))
    return Profile(
        name=name,
        description=entry.description,
        allowed_tools=resolved_names,
        allowed_environments=entry.allowed_environments,
        required_token_scopes=entry.required_token_scopes,
        # Derived from the resolved tool list, never from user config:
        # the missing-token policy must reflect what the profile can
        # actually mutate.
        elevated=has_mutating_tools(registry, resolved_names),
        allow_yolo=entry.allow_yolo,
        disabled=False,
    )


def resolve_active_profile(
    cfg: Config,
    registry: Sequence[ToolDescriptor],
) -> Profile:
    """Resolve ``cfg.active_profile`` against the built-in catalog and user entries.

    Falls back to the ``default`` built-in when ``cfg.active_profile`` is
    empty. Returns the resolved ``Profile`` whose ``allowed_tools`` is the
    explicit list the registration filter consumes in Phase 4.

    Raises:
        ActiveProfileDisabledError: the active name is a built-in disabled
            via ``profiles_builtin_overrides``.
        ActiveProfileUnknownError: the active name is neither a built-in
            nor a user-defined entry.
    """
    requested = cfg.active_profile.strip() or DEFAULT_PROFILE_NAME
    builtins = builtin_profiles(registry)

    # Surface overrides aimed at non-built-in names so users notice the
    # typo: only built-ins can be disabled, so an override on a user-defined
    # name silently does nothing.
    for override_name in cfg.profiles_builtin_overrides:
        if override_name not in builtins:
            logger.warning(
                "profiles_builtin_overrides entry %r does not name a built-in "
                "profile; override ignored",
                override_name,
            )

    if requested in builtins:
        override = cfg.profiles_builtin_overrides.get(requested)
        if override is not None and override.disabled:
            msg = (
                f"active profile {requested!r} is a built-in disabled via "
                "profiles_builtin_overrides; enable it in config or pick "
                "a different profile"
            )
            raise ActiveProfileDisabledError(msg)
        return builtins[requested]

    if requested in cfg.profiles:
        return _resolve_user_profile(requested, cfg.profiles[requested], registry)

    msg = (
        f"active profile {requested!r} is not a known built-in and is not "
        "defined under 'profiles:' in config"
    )
    raise ActiveProfileUnknownError(msg)
