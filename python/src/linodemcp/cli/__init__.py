"""Phase 7a profile subcommands for the linodemcp CLI.

Read-only enumeration only:

    linodemcp profile list           lists every built-in and user profile
    linodemcp profile show <name>    prints one profile's full details

Mutation (use, clone, delete, enable, disable) lands in 7b/7c with
atomic config writes. This package intentionally does not import the
watcher or the linode client; subcommands here load the config once,
build the catalog, and print.
"""

from __future__ import annotations

from linodemcp.cli.profile import (
    EXIT_USAGE_ERROR,
    all_profiles,
    print_profile_detail,
    resolve_active_name,
    run_profile_command,
    run_profile_disable,
    run_profile_enable,
    run_profile_list,
    run_profile_show,
    run_profile_use,
)

__all__ = [
    "EXIT_USAGE_ERROR",
    "all_profiles",
    "print_profile_detail",
    "resolve_active_name",
    "run_profile_command",
    "run_profile_disable",
    "run_profile_enable",
    "run_profile_list",
    "run_profile_show",
    "run_profile_use",
]
