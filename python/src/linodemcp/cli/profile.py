"""Profile subcommand implementations.

Mirrors ``go/internal/cli/profile.go``. Pure I/O helpers: load config,
build catalog, print. No watcher, no linode client. Tests live in
``tests/unit/test_cli_profile.py``.
"""

from __future__ import annotations

import dataclasses
from typing import TextIO

from linodemcp.config import Config, get_config_path, load_from_file
from linodemcp.profiles import (
    DEFAULT_PROFILE_NAME,
    Profile,
    ToolDescriptor,
    builtin_profiles,
)
from linodemcp.server import get_tool_registry

# EXIT_USAGE_ERROR is the conventional code for argument-shape problems
# (matches sysexits' EX_USAGE). Re-exported so main can use the same
# constant when dispatching.
EXIT_USAGE_ERROR = 2

# Column widths for the `profile list` table. Extracted so the header
# and row formatting stay in sync.
_COL_MARKER = 3
_COL_NAME = 20
_COL_YOLO = 8
_COL_STATE = 8

PROFILE_USAGE = """\
Usage: linodemcp profile <subcommand> [args]

Read-only:
  list           List all built-in and user-defined profiles.
  show <name>    Show details for a single profile.

Mutators (Phase 7b/7c): use, clone, delete, enable, disable.\
"""


def run_profile_command(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """Dispatch ``linodemcp profile <subcommand> ...`` to its handler.

    Unknown subcommand or empty args print usage to stderr and exit
    with ``EXIT_USAGE_ERROR``. Output streams are parameters so tests
    can capture them without swapping ``sys.stdout``/``sys.stderr``.
    """
    if not args:
        print(PROFILE_USAGE, file=stderr)
        return EXIT_USAGE_ERROR

    sub = args[0]
    rest = args[1:]
    if sub == "list":
        return run_profile_list(rest, stdout, stderr)
    if sub == "show":
        return run_profile_show(rest, stdout, stderr)
    print(f"unknown profile subcommand: {sub}\n\n{PROFILE_USAGE}", file=stderr)
    return EXIT_USAGE_ERROR


def run_profile_list(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """List every built-in and user-defined profile with active marker."""
    if args:
        print(f"profile list takes no arguments, got: {args}", file=stderr)
        return EXIT_USAGE_ERROR

    cfg = _load_config_or_error(stderr)
    if cfg is None:
        return 1

    catalog = all_profiles(cfg)
    active = resolve_active_name(cfg)

    header = (
        f"{'*':<{_COL_MARKER}} {'name':<{_COL_NAME}} "
        f"{'yolo':<{_COL_YOLO}} {'state':<{_COL_STATE}} tools"
    )
    print(header, file=stdout)

    for name in sorted(catalog):
        prof = catalog[name]
        marker = "*" if name == active else " "
        state = "DISABLED" if prof.disabled else "enabled"
        yolo = "YES" if prof.allow_yolo else "no"
        row = (
            f"{marker:<{_COL_MARKER}} {name:<{_COL_NAME}} "
            f"{yolo:<{_COL_YOLO}} {state:<{_COL_STATE}} "
            f"{len(prof.allowed_tools)}"
        )
        print(row, file=stdout)

    return 0


def run_profile_show(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """Print one profile's full detail by exact name.

    Unknown names exit 1 with a sorted list of valid names to help the
    user recover from typos.
    """
    if len(args) != 1:
        print("Usage: linodemcp profile show <name>", file=stderr)
        return EXIT_USAGE_ERROR

    name = args[0]

    cfg = _load_config_or_error(stderr)
    if cfg is None:
        return 1

    catalog = all_profiles(cfg)
    prof = catalog.get(name)
    if prof is None:
        print(f'profile "{name}" not found.', file=stderr)
        print("Available profiles:", file=stderr)
        for available in sorted(catalog):
            print(f"  {available}", file=stderr)
        return 1

    print_profile_detail(stdout, prof, resolve_active_name(cfg))
    return 0


def print_profile_detail(stdout: TextIO, prof: Profile, active: str) -> None:
    """Write one Profile in a stable human-readable shape.

    Exported so tests can exercise formatting in isolation without
    going through the full subcommand path.
    """
    suffix = " (active)" if prof.name == active else ""
    print(f"Profile: {prof.name}{suffix}", file=stdout)
    print(f"Description: {prof.description}", file=stdout)
    print(f"Disabled: {prof.disabled}", file=stdout)
    print(f"Allow yolo: {prof.allow_yolo}", file=stdout)

    if not prof.allowed_environments:
        print("Allowed environments: <all>", file=stdout)
    else:
        joined = ", ".join(prof.allowed_environments)
        print(f"Allowed environments: {joined}", file=stdout)

    print(
        f"Required token scopes ({len(prof.required_token_scopes)}):",
        file=stdout,
    )
    for scope in prof.required_token_scopes:
        print(f"  {scope}", file=stdout)

    print(f"Allowed tools ({len(prof.allowed_tools)}):", file=stdout)
    for tool in prof.allowed_tools:
        print(f"  {tool}", file=stdout)


def all_profiles(cfg: Config) -> dict[str, Profile]:
    """Return every profile keyed by name.

    Built-ins come first; user-defined entries from ``cfg.profiles``
    shadow built-ins of the same name to match the resolver's order.
    Built-in disabled flags fold in any per-name overrides.
    """
    descriptors = [
        ToolDescriptor(name=entry.name, capability=entry.capability)
        for entry in get_tool_registry()
    ]
    builtins = builtin_profiles(descriptors)
    overrides = cfg.profiles_builtin_overrides or {}

    out: dict[str, Profile] = {}
    for name, prof in builtins.items():
        override = overrides.get(name)
        resolved = (
            dataclasses.replace(prof, disabled=override.disabled)
            if override is not None and override.disabled != prof.disabled
            else prof
        )
        out[name] = resolved

    for name, user_cfg in (cfg.profiles or {}).items():
        out[name] = Profile(
            name=name,
            description=user_cfg.description,
            allowed_tools=tuple(user_cfg.allowed_tools),
            allowed_environments=tuple(user_cfg.allowed_environments),
            required_token_scopes=tuple(user_cfg.required_token_scopes),
            allow_yolo=user_cfg.allow_yolo,
            disabled=False,
        )

    return out


def resolve_active_name(cfg: Config) -> str:
    """Return ``cfg.active_profile`` or ``DEFAULT_PROFILE_NAME`` if unset."""
    return cfg.active_profile or DEFAULT_PROFILE_NAME


def _load_config_or_error(stderr: TextIO) -> Config | None:
    """Read the user config from the standard path.

    Returns ``None`` on failure after emitting a friendly error to
    stderr so the caller can early-return with a non-zero exit code.
    """
    path = get_config_path()
    try:
        return load_from_file(path)
    except Exception as exc:
        print(f"load config from {path}: {exc}", file=stderr)
        return None
