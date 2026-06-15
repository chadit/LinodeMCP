"""Subcommands for the linodemcp CLI.

The CLI extends the single binary with non-interactive subcommands while bare
invocation stays the MCP stdio server. Surfaces:

    linodemcp call <tool> [args]     run any registered tool through dispatch
    linodemcp tools [--all]          list the tool surface; `show <tool>` detail
    linodemcp audit <sub> [flags]    read the audit log via the audit tools
    linodemcp profile <sub> [args]   manage profiles (list/show/use/...)

The ``call`` and ``audit`` commands never reimplement tool logic; they build a
tool call and feed it to ``Server.dispatch``, so they get the same audit,
profile filter, and dry-run/two-stage middleware an MCP request gets. The
``profile`` subcommands load the config, build the catalog, and print without a
server. Every command takes its output streams as parameters so they are
unit-testable the way ``profile.py`` already is.
"""

from __future__ import annotations

from linodemcp.cli.audit import run_audit_command
from linodemcp.cli.call import run_call_command
from linodemcp.cli.profile import (
    EXIT_USAGE_ERROR,
    all_profiles,
    print_profile_detail,
    resolve_active_name,
    run_profile_clone,
    run_profile_command,
    run_profile_delete,
    run_profile_disable,
    run_profile_enable,
    run_profile_list,
    run_profile_show,
    run_profile_use,
)
from linodemcp.cli.tools import run_tools_command

__all__ = [
    "EXIT_USAGE_ERROR",
    "all_profiles",
    "print_profile_detail",
    "resolve_active_name",
    "run_audit_command",
    "run_call_command",
    "run_profile_clone",
    "run_profile_command",
    "run_profile_delete",
    "run_profile_disable",
    "run_profile_enable",
    "run_profile_list",
    "run_profile_show",
    "run_profile_use",
    "run_tools_command",
]
