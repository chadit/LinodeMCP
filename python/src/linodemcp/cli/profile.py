"""Profile subcommand implementations.

Mirrors ``go/internal/cli/profile.go``. Pure I/O helpers: load config,
build catalog, print. No watcher, no linode client. Tests live in
``tests/unit/test_cli_profile.py``.
"""

from __future__ import annotations

import dataclasses
from typing import TYPE_CHECKING, TextIO

from linodemcp.config import (
    BuiltinOverride,
    Config,
    UserProfileConfig,
    get_config_path,
    load_from_file,
    write_atomic,
)
from linodemcp.profiles import (
    DEFAULT_PROFILE_NAME,
    Profile,
    ToolDescriptor,
    builtin_profiles,
)
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from collections.abc import Callable
    from pathlib import Path

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
  list                  List all built-in and user-defined profiles.
  show <name>           Show details for a single profile.

Mutators (atomic config write, comments and ordering not preserved):
  use <name>            Switch the active profile.
  enable <name>         Clear the disabled flag on a built-in profile.
  disable <name>        Set the disabled flag on a built-in profile.
  clone <src> <dst>     Copy any profile into a new user-defined entry.
  delete <name>         Remove a user-defined profile.\
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

    read_only_handlers: dict[str, Callable[[list[str], TextIO, TextIO], int]] = {
        "list": run_profile_list,
        "show": run_profile_show,
    }
    if sub in read_only_handlers:
        return read_only_handlers[sub](rest, stdout, stderr)

    mutator_handlers: dict[
        str, Callable[[list[str], Path | None, TextIO, TextIO], int]
    ] = {
        "use": run_profile_use,
        "enable": run_profile_enable,
        "disable": run_profile_disable,
        "clone": run_profile_clone,
        "delete": run_profile_delete,
    }
    if sub in mutator_handlers:
        return mutator_handlers[sub](rest, None, stdout, stderr)

    print(f"unknown profile subcommand: {sub}\n\n{PROFILE_USAGE}", file=stderr)
    return EXIT_USAGE_ERROR


def run_profile_use(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Switch the active profile after validating the target exists.

    ``config_path`` is the file to load and rewrite; ``None`` falls
    back to ``get_config_path()``. Unknown profile names exit 1 without
    writing; on success the rewrite is atomic.
    """
    if len(args) != 1:
        print("Usage: linodemcp profile use <name>", file=stderr)
        return EXIT_USAGE_ERROR

    name = args[0]
    path = config_path if config_path is not None else get_config_path()

    cfg = _load_config_from_path(path, stderr)
    if cfg is None:
        return 1

    if name not in all_profiles(cfg):
        print(f'profile "{name}" not found.', file=stderr)
        return 1

    cfg.active_profile = name
    return _write_and_report(
        path, cfg, stdout, stderr, f"active profile switched to {name}"
    )


def run_profile_enable(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Clear the disabled flag on a built-in profile via overrides.

    Only built-ins are subject to the override map; calling enable on
    a user-defined profile exits 1 since the override would silently
    do nothing.
    """
    return _run_profile_toggle(
        args, config_path, stdout, stderr, disabled=False, verb="enabled"
    )


def run_profile_disable(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Set the disabled flag on a built-in profile.

    Refuses to disable the currently-active profile so the server
    cannot get stuck unable to start.
    """
    return _run_profile_toggle(
        args, config_path, stdout, stderr, disabled=True, verb="disabled"
    )


def _validate_clone_dst(dst: str, stderr: TextIO) -> bool:
    """Return True if the destination name passes the static guards."""
    if not dst:
        print("destination name cannot be empty", file=stderr)
        return False
    if dst in _BUILTIN_PROFILE_NAMES:
        print(
            f'destination "{dst}" collides with a built-in profile name; pick another.',
            file=stderr,
        )
        return False
    return True


def run_profile_clone(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Copy any profile into a new user-defined entry.

    Source can be a built-in or a user-defined profile. Destination
    must be a fresh name: it cannot collide with a built-in (those are
    immutable in the catalog), with another user-defined entry (no
    silent overwrite), or be empty. The clone captures the source's
    description, allowed_tools, scopes, etc; the user can then edit
    the YAML to customize.
    """
    expected_args = 2
    if len(args) != expected_args:
        print("Usage: linodemcp profile clone <src> <dst>", file=stderr)
        return EXIT_USAGE_ERROR

    src, dst = args[0], args[1]
    if not _validate_clone_dst(dst, stderr):
        return 1

    path = config_path if config_path is not None else get_config_path()
    cfg = _load_config_from_path(path, stderr)
    if cfg is None:
        return 1

    if dst in (cfg.profiles or {}):
        print(
            f'user-defined profile "{dst}" already exists; pick another or '
            "delete it first.",
            file=stderr,
        )
        return 1

    source = all_profiles(cfg).get(src)
    if source is None:
        print(f'source profile "{src}" not found.', file=stderr)
        return 1

    profiles = dict(cfg.profiles or {})
    profiles[dst] = UserProfileConfig(
        description=source.description,
        allowed_tools=tuple(source.allowed_tools),
        allowed_environments=tuple(source.allowed_environments),
        required_token_scopes=tuple(source.required_token_scopes),
        allow_yolo=source.allow_yolo,
    )
    cfg.profiles = profiles

    return _write_and_report(
        path, cfg, stdout, stderr, f"profile {dst} cloned from {src}"
    )


def run_profile_delete(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Remove a user-defined profile by name.

    Built-ins cannot be deleted (they live in code, not config) and
    the currently-active profile cannot be removed since that would
    prevent the server from starting.
    """
    if len(args) != 1:
        print("Usage: linodemcp profile delete <name>", file=stderr)
        return EXIT_USAGE_ERROR

    name = args[0]

    if name in _BUILTIN_PROFILE_NAMES:
        print(
            f'profile "{name}" is a built-in; built-ins cannot be deleted '
            "(try `profile disable`).",
            file=stderr,
        )
        return 1

    path = config_path if config_path is not None else get_config_path()

    cfg = _load_config_from_path(path, stderr)
    if cfg is None:
        return 1

    if name not in (cfg.profiles or {}):
        print(f'user-defined profile "{name}" not found.', file=stderr)
        return 1

    if resolve_active_name(cfg) == name:
        print(
            f'profile "{name}" is the active profile; switch first via '
            "`profile use <other>` before deleting.",
            file=stderr,
        )
        return 1

    profiles = dict(cfg.profiles or {})
    del profiles[name]
    cfg.profiles = profiles

    return _write_and_report(path, cfg, stdout, stderr, f"profile {name} deleted")


def _run_profile_toggle(
    args: list[str],
    config_path: Path | None,
    stdout: TextIO,
    stderr: TextIO,
    *,
    disabled: bool,
    verb: str,
) -> int:
    """Shared body for enable/disable."""
    if len(args) != 1:
        print(f"Usage: linodemcp profile {verb} <name>", file=stderr)
        return EXIT_USAGE_ERROR

    name = args[0]
    path = config_path if config_path is not None else get_config_path()

    cfg = _load_config_from_path(path, stderr)
    if cfg is None:
        return 1

    if name not in _BUILTIN_PROFILE_NAMES:
        print(
            f'profile "{name}" is not a built-in; enable/disable only applies '
            "to built-in profiles.",
            file=stderr,
        )
        return 1

    if disabled and resolve_active_name(cfg) == name:
        print(
            f'profile "{name}" is the active profile; switch first via '
            "`profile use <other>` before disabling.",
            file=stderr,
        )
        return 1

    overrides = dict(cfg.profiles_builtin_overrides or {})
    overrides[name] = BuiltinOverride(disabled=disabled)
    cfg.profiles_builtin_overrides = overrides

    return _write_and_report(path, cfg, stdout, stderr, f"profile {name} {verb}")


def _load_config_from_path(path: Path, stderr: TextIO) -> Config | None:
    """Load the config from path with friendly stderr on failure."""
    try:
        return load_from_file(path)
    except Exception as exc:
        print(f"load config from {path}: {exc}", file=stderr)
        return None


def _write_and_report(
    path: Path,
    cfg: Config,
    stdout: TextIO,
    stderr: TextIO,
    success: str,
) -> int:
    """Write the config atomically and print success or the error."""
    try:
        write_atomic(path, cfg)
    except Exception as exc:
        print(f"write config to {path}: {exc}", file=stderr)
        return 1
    print(success, file=stdout)
    return 0


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
