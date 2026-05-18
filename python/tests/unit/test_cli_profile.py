"""Unit tests for Phase 7a CLI profile subcommands.

Mirrors ``go/internal/cli/profile_test.go``. Exercises the in-memory
helpers directly so tests stay decoupled from the on-disk config file
the subcommands read via ``get_config_path``.
"""

from __future__ import annotations

import dataclasses
import io
from typing import TYPE_CHECKING

from linodemcp.cli import (
    EXIT_USAGE_ERROR,
    all_profiles,
    print_profile_detail,
    resolve_active_name,
    run_profile_command,
    run_profile_show,
)
from linodemcp.profiles import DEFAULT_PROFILE_NAME, Profile

if TYPE_CHECKING:
    from linodemcp.config import Config


_TEST_PROFILE_COMPUTE_ADMIN = "compute-admin"
_TEST_USER_PROFILE = "my-custom"
_TEST_VOLUMES_LIST_TOOL = "linode_volumes_list"


def test_all_profiles_contains_builtins(sample_config: Config) -> None:
    """``all_profiles`` must include every shipped built-in profile.

    Locks in the catalog contract the list view depends on.
    """
    catalog = all_profiles(sample_config)
    for name in (
        "default",
        "readonly-full",
        "compute-admin",
        "network-admin",
        "kubernetes-admin",
        "storage-admin",
        "full-access",
        "emergency",
    ):
        assert name in catalog, f"catalog must include built-in {name!r}"


def test_all_profiles_includes_user_defined(sample_config: Config) -> None:
    """User-defined profiles get folded into the listed catalog."""
    from linodemcp.config import UserProfileConfig

    cfg = dataclasses.replace(
        sample_config,
        profiles={
            _TEST_USER_PROFILE: UserProfileConfig(
                description="User-defined for the CLI list test",
                allowed_tools=(_TEST_VOLUMES_LIST_TOOL,),
            ),
        },
    )

    catalog = all_profiles(cfg)
    assert _TEST_USER_PROFILE in catalog, (
        "user-defined profile must appear in the listed catalog"
    )
    prof = catalog[_TEST_USER_PROFILE]
    assert prof.description == "User-defined for the CLI list test"
    assert prof.allowed_tools == (_TEST_VOLUMES_LIST_TOOL,)


def test_all_profiles_applies_builtin_overrides(sample_config: Config) -> None:
    """Disabling a built-in via overrides propagates into the catalog.

    The `list` view shows DISABLED for these so users can spot a
    misconfigured override.
    """
    from linodemcp.config import BuiltinOverride

    cfg = dataclasses.replace(
        sample_config,
        profiles_builtin_overrides={
            _TEST_PROFILE_COMPUTE_ADMIN: BuiltinOverride(disabled=True),
        },
    )

    catalog = all_profiles(cfg)
    assert catalog[_TEST_PROFILE_COMPUTE_ADMIN].disabled is True, (
        "override Disabled=true must propagate into the listed profile"
    )


def test_resolve_active_name_defaults(sample_config: Config) -> None:
    """Unset active_profile falls back to the built-in default name."""
    assert resolve_active_name(sample_config) == DEFAULT_PROFILE_NAME

    cfg = dataclasses.replace(sample_config, active_profile=_TEST_PROFILE_COMPUTE_ADMIN)
    assert resolve_active_name(cfg) == _TEST_PROFILE_COMPUTE_ADMIN


def test_run_profile_command_unknown_subcommand_returns_usage_error() -> None:
    """Unknown subcommand exits with the usage-error code and prints usage."""
    stderr = io.StringIO()
    rc = run_profile_command(["nonexistent"], io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR, (
        "unknown subcommand must exit with the usage-error code"
    )
    assert "Usage:" in stderr.getvalue(), (
        "unknown subcommand must surface the full usage block to stderr"
    )


def test_run_profile_command_empty_args_returns_usage_error() -> None:
    """`linodemcp profile` with no subcommand prints usage and exits 2."""
    stderr = io.StringIO()
    rc = run_profile_command([], io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()


def test_run_profile_show_zero_args_returns_usage() -> None:
    """`profile show` without a name argument exits with the usage code."""
    stderr = io.StringIO()
    rc = run_profile_show([], io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR, (
        "show with zero args must exit with the usage-error code"
    )
    assert "Usage:" in stderr.getvalue(), (
        "zero-arg invocation must print usage to stderr"
    )


def test_print_profile_detail_marks_active() -> None:
    """The detail header includes ``(active)`` when the name matches."""
    prof = Profile(
        name=_TEST_PROFILE_COMPUTE_ADMIN,
        description="test profile",
        allowed_tools=(),
        allowed_environments=(),
        required_token_scopes=(),
        allow_yolo=False,
        disabled=False,
    )

    buf = io.StringIO()
    print_profile_detail(buf, prof, _TEST_PROFILE_COMPUTE_ADMIN)

    assert "Profile: compute-admin (active)" in buf.getvalue(), (
        "active profile must be marked in the header"
    )


def test_print_profile_detail_omits_marker_for_inactive() -> None:
    """A profile that isn't the active one must NOT carry the marker."""
    prof = Profile(
        name=_TEST_PROFILE_COMPUTE_ADMIN,
        description="test",
        allowed_tools=(),
        allowed_environments=(),
        required_token_scopes=(),
        allow_yolo=False,
        disabled=False,
    )

    buf = io.StringIO()
    print_profile_detail(buf, prof, DEFAULT_PROFILE_NAME)

    assert "(active)" not in buf.getvalue(), "inactive profile must not be marked"


def test_print_profile_detail_lists_allowed_tools() -> None:
    """AllowedTools appears in the output with its count header."""
    prof = Profile(
        name=_TEST_PROFILE_COMPUTE_ADMIN,
        description="",
        allowed_tools=("linode_instances_list", "linode_instance_create"),
        allowed_environments=(),
        required_token_scopes=(),
        allow_yolo=False,
        disabled=False,
    )

    buf = io.StringIO()
    print_profile_detail(buf, prof, "")

    out = buf.getvalue()
    assert "Allowed tools (2):" in out
    assert "linode_instances_list" in out
    assert "linode_instance_create" in out


def test_print_profile_detail_shows_required_scopes() -> None:
    """The required-scopes list and count header appear in the output."""
    prof = Profile(
        name=_TEST_PROFILE_COMPUTE_ADMIN,
        description="",
        allowed_tools=(),
        allowed_environments=(),
        required_token_scopes=("linodes:read_write", "volumes:read_write"),
        allow_yolo=False,
        disabled=False,
    )

    buf = io.StringIO()
    print_profile_detail(buf, prof, "")

    out = buf.getvalue()
    assert "Required token scopes (2):" in out
    assert "linodes:read_write" in out
    assert "volumes:read_write" in out


def test_all_profiles_user_defined_shadows_builtin(
    sample_config: Config,
) -> None:
    """User-defined profile with built-in name replaces the built-in.

    Matches the resolver precedence so the list view reflects what
    would actually run.
    """
    from linodemcp.config import UserProfileConfig

    cfg = dataclasses.replace(
        sample_config,
        profiles={
            DEFAULT_PROFILE_NAME: UserProfileConfig(
                description="shadowed default",
                allowed_tools=(_TEST_VOLUMES_LIST_TOOL,),
            ),
        },
    )

    catalog = all_profiles(cfg)
    got = catalog[DEFAULT_PROFILE_NAME]
    assert got.description == "shadowed default", (
        "user-defined profile with built-in name must replace the built-in"
    )
    assert _TEST_VOLUMES_LIST_TOOL in got.allowed_tools, (
        "shadowed entry must carry the user-defined allow list"
    )
