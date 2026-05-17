"""Exception types for profile resolution.

Kept in a dedicated module so the resolver (``loader.py``) and any future
consumer (Phase 4 server wiring) can import errors without pulling in the
resolver's logic. Mirrors the Go-side sentinel errors
``ErrActiveProfileUnknown`` and ``ErrActiveProfileDisabled``; Python class
names carry the ``Error`` suffix required by ``ruff``'s ``N818`` rule.
"""

from __future__ import annotations


class ProfileError(Exception):
    """Base class for profile resolution failures."""


class ActiveProfileUnknownError(ProfileError):
    """The configured ``active_profile`` does not name any known profile.

    Names a profile that is neither a built-in nor a user-defined entry in
    ``Config.profiles``. The user fixes this by editing the config; the
    server refuses to start.
    """


class ActiveProfileDisabledError(ProfileError):
    """The configured ``active_profile`` names a built-in disabled by override.

    Built-ins like ``full-access`` and ``emergency`` ship disabled and only
    become selectable after the user explicitly flips
    ``profiles_builtin_overrides.<name>.disabled`` to ``false`` (or leaves
    the override out entirely). Selecting a disabled built-in is treated as
    operator error rather than silently falling back.
    """
