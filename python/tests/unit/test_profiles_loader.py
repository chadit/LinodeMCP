"""Phase 3 tests for active-profile resolution.

The resolver consumes a parsed ``Config`` plus a synthetic tool registry
and returns a ``Profile`` whose ``allowed_tools`` is the explicit, expanded
list Phase 4 will pass to the registration filter. Tests below exercise
every spec case from ``.claude/tmp/phase3_config_spec.md``'s "Tests"
section, using synthetic descriptors so failures point at the resolver
rather than at whatever lives in the live registry today.
"""

from __future__ import annotations

import logging

import pytest

from linodemcp.config import BuiltinOverride, Config, UserProfileConfig
from linodemcp.profiles import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
    Capability,
    ToolDescriptor,
    resolve_active_profile,
)


def _synthetic_registry() -> list[ToolDescriptor]:
    """Tool registry covering enough categories to drive every test case.

    Names match the prefix categorisation in
    ``linodemcp.profiles.builtin`` so built-in resolution behaves the same
    way the Phase 2 tests already cover.
    """
    return [
        # Core and read tools (always included in every built-in).
        ToolDescriptor("hello", Capability.Meta),
        ToolDescriptor("version", Capability.Meta),
        ToolDescriptor("linode_profile", Capability.Read),
        ToolDescriptor("linode_account", Capability.Read),
        # Compute.
        ToolDescriptor("linode_instances_list", Capability.Read),
        ToolDescriptor("linode_instance_get", Capability.Read),
        ToolDescriptor("linode_instance_create", Capability.Write),
        ToolDescriptor("linode_instance_delete", Capability.Destroy),
        # Block storage. Mix of read and mutate so wildcards have real tools
        # to match against.
        ToolDescriptor("linode_volumes_list", Capability.Read),
        ToolDescriptor("linode_volume_get", Capability.Read),
        ToolDescriptor("linode_volume_create", Capability.Write),
        ToolDescriptor("linode_volume_update", Capability.Write),
        ToolDescriptor("linode_volume_delete", Capability.Destroy),
        # Networking. Used by the "non-builtin override is ignored" test.
        ToolDescriptor("linode_firewalls_list", Capability.Read),
        ToolDescriptor("linode_firewall_create", Capability.Write),
    ]


def _config_with(
    *,
    active_profile: str = "",
    profiles: dict[str, UserProfileConfig] | None = None,
    overrides: dict[str, BuiltinOverride] | None = None,
) -> Config:
    """Build a ``Config`` with only the profile fields populated.

    The resolver does not touch the other fields, so leaving them at
    defaults keeps each test focused on the bits that matter.
    """
    return Config(
        active_profile=active_profile,
        profiles=profiles or {},
        profiles_builtin_overrides=overrides or {},
    )


def test_empty_active_profile_falls_back_to_default_builtin() -> None:
    """Unset ``active_profile`` resolves to the built-in ``default``."""
    registry = _synthetic_registry()
    cfg = _config_with()

    profile = resolve_active_profile(cfg, registry)

    assert profile.name == "default"
    # The default profile must not include the write/destroy tools that
    # live in the synthetic registry.
    assert "linode_instance_create" not in profile.allowed_tools
    assert "linode_volume_delete" not in profile.allowed_tools
    # Core meta + read tools always make it into default.
    assert "hello" in profile.allowed_tools
    assert "linode_instance_get" in profile.allowed_tools


def test_builtin_compute_admin_selected_by_name() -> None:
    """Naming a built-in resolves to that built-in's tool set."""
    registry = _synthetic_registry()
    cfg = _config_with(active_profile="compute-admin")

    profile = resolve_active_profile(cfg, registry)

    assert profile.name == "compute-admin"
    assert "linode_instance_create" in profile.allowed_tools
    assert "linode_instance_delete" in profile.allowed_tools
    # Compute admin elevates block storage too per the built-in catalog.
    assert "linode_volume_create" in profile.allowed_tools


def test_disabled_builtin_active_raises() -> None:
    """An override that disables the active built-in is rejected at load."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="compute-admin",
        overrides={"compute-admin": BuiltinOverride(disabled=True)},
    )

    with pytest.raises(ActiveProfileDisabledError, match="compute-admin"):
        resolve_active_profile(cfg, registry)


def test_user_defined_profile_with_literal_tool() -> None:
    """A literal entry in ``allowed_tools`` resolves to just that tool."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="my-prof",
        profiles={
            "my-prof": UserProfileConfig(
                description="just volumes list",
                allowed_tools=("linode_volumes_list",),
            ),
        },
    )

    profile = resolve_active_profile(cfg, registry)

    assert profile.name == "my-prof"
    assert profile.allowed_tools == ("linode_volumes_list",)


def test_wildcard_expansion_resolves_every_matching_tool() -> None:
    """``linode_volume_*`` expands to every matching tool in the registry."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="vol-everything",
        profiles={
            "vol-everything": UserProfileConfig(
                description="all volume-prefixed tools",
                allowed_tools=("linode_volume_*",),
            ),
        },
    )

    profile = resolve_active_profile(cfg, registry)

    assert set(profile.allowed_tools) == {
        "linode_volume_get",
        "linode_volume_create",
        "linode_volume_update",
        "linode_volume_delete",
    }


def test_denied_subtracts_from_allowed() -> None:
    """Explicit deny removes a tool even when the allow wildcard matches it."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="vol-no-delete",
        profiles={
            "vol-no-delete": UserProfileConfig(
                description="all volume tools except delete",
                allowed_tools=("linode_volume_*",),
                denied_tools=("linode_volume_delete",),
            ),
        },
    )

    profile = resolve_active_profile(cfg, registry)

    assert "linode_volume_delete" not in profile.allowed_tools
    assert "linode_volume_create" in profile.allowed_tools


def test_unknown_active_profile_raises() -> None:
    """An ``active_profile`` not built-in and not defined raises the right error."""
    registry = _synthetic_registry()
    cfg = _config_with(active_profile="nonexistent")

    with pytest.raises(ActiveProfileUnknownError, match="nonexistent"):
        resolve_active_profile(cfg, registry)


def test_star_wildcard_resolves_every_registered_tool() -> None:
    """``*`` matches the whole registry."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="kitchen-sink",
        profiles={
            "kitchen-sink": UserProfileConfig(
                description="everything",
                allowed_tools=("*",),
            ),
        },
    )

    profile = resolve_active_profile(cfg, registry)

    assert set(profile.allowed_tools) == {tool.name for tool in registry}


def test_wildcard_matching_nothing_logs_warning_and_resolves_empty(
    caplog: pytest.LogCaptureFixture,
) -> None:
    """A wildcard that matches no tool warns but does not error."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="empty",
        profiles={
            "empty": UserProfileConfig(
                description="no matches",
                allowed_tools=("zzz_*",),
            ),
        },
    )

    with caplog.at_level(logging.WARNING, logger="linodemcp.profiles.loader"):
        profile = resolve_active_profile(cfg, registry)

    assert profile.allowed_tools == ()
    matched_warnings = [
        record
        for record in caplog.records
        if "zzz_*" in record.getMessage()
        and "matched no registered tools" in record.getMessage()
    ]
    assert matched_warnings, "expected a warning about the unmatched wildcard"


def test_override_naming_non_builtin_is_ignored_with_warning(
    caplog: pytest.LogCaptureFixture,
) -> None:
    """Overrides target built-ins only; non-built-in names log and are ignored."""
    registry = _synthetic_registry()
    cfg = _config_with(
        active_profile="my-custom",
        profiles={
            "my-custom": UserProfileConfig(
                description="user profile sharing a name with an override",
                allowed_tools=("linode_volumes_list",),
            ),
        },
        overrides={"my-custom": BuiltinOverride(disabled=True)},
    )

    with caplog.at_level(logging.WARNING, logger="linodemcp.profiles.loader"):
        profile = resolve_active_profile(cfg, registry)

    # The user-defined profile loads normally despite the bogus override.
    assert profile.name == "my-custom"
    assert profile.allowed_tools == ("linode_volumes_list",)
    ignored_warnings = [
        record
        for record in caplog.records
        if "my-custom" in record.getMessage()
        and "does not name a built-in" in record.getMessage()
    ]
    assert ignored_warnings, "expected a warning about the non-builtin override"
