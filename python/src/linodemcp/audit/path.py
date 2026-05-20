"""Audit log directory resolution.

Mirrors ``go/internal/audit/path.go``. Picks where the rolling
``audit.log`` lives based on whether the process looks like a system
service or an interactive user.
"""

from __future__ import annotations

import os
from pathlib import Path

# UID cutoff distinguishing system daemons (typically UID < 1000) from
# interactive users. A process running below this UID gets the system
# log directory; everyone else writes under their XDG state directory.
_SYSTEM_SERVICE_UID_THRESHOLD = 1000

# Path used when the process runs as a system daemon.
SYSTEM_AUDIT_DIR = "/var/log/linodemcp"

# Directory name appended to XDG_STATE_HOME (or ~/.local/state) for
# non-system users.
USER_AUDIT_DIR_RELATIVE = "linodemcp"


def resolve_default_audit_dir() -> str:
    """Pick the audit log directory for the current host.

    Resolution order:

    1. ``$XDG_STATE_HOME/linodemcp`` if ``XDG_STATE_HOME`` is set.
       Explicit user intent always wins. Daemons don't set this so
       they fall through.
    2. ``/var/log/linodemcp`` if the process looks like a system
       service (POSIX heuristic: UID below the system threshold).
    3. ``$HOME/.local/state/linodemcp`` otherwise.

    Never raises. Directory-creation failures surface later from the
    sink constructor, where the caller can decide whether to bail or
    fall back to a noop sink.
    """
    xdg = os.environ.get("XDG_STATE_HOME", "")
    if xdg:
        return str(Path(xdg) / USER_AUDIT_DIR_RELATIVE)

    if _is_system_service():
        return SYSTEM_AUDIT_DIR

    return _user_audit_dir()


def _is_system_service() -> bool:
    """Report whether the process appears to run as a system daemon.

    POSIX heuristic: effective UID below the system threshold. On
    platforms without ``os.geteuid`` (Windows) the UID concept does
    not apply, so this returns ``False`` and Windows hosts get the
    user path.
    """
    if not hasattr(os, "geteuid"):
        return False

    return os.geteuid() < _SYSTEM_SERVICE_UID_THRESHOLD


def _user_audit_dir() -> str:
    """Return the home-relative audit directory for an interactive user.

    ``$XDG_STATE_HOME`` handling already happened in the caller, so
    this only covers the home-directory fallback. If the home
    directory can't be resolved (rare; stripped-down containers), it
    falls back to the working directory so the path is never empty.
    """
    home = Path.home() if _home_resolvable() else None
    if home is None:
        return str(Path.cwd() / USER_AUDIT_DIR_RELATIVE)

    return str(home / ".local" / "state" / USER_AUDIT_DIR_RELATIVE)


def _home_resolvable() -> bool:
    """Report whether ``Path.home()`` resolves without raising."""
    try:
        Path.home()
    except RuntimeError:
        return False

    return True
