"""Profiles infrastructure: capability tags, the ``Profile`` value type, and
the built-in catalog. Consumers import from this package rather than the
submodules so the surface stays stable across future phase refactors.
"""

from linodemcp.profiles.builtin import (
    ToolDescriptor,
    builtin_catalog_json,
    builtin_profiles,
)
from linodemcp.profiles.capability import Capability
from linodemcp.profiles.errors import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
    ProfileError,
)
from linodemcp.profiles.loader import (
    DEFAULT_PROFILE_NAME,
    resolve_active_profile,
)
from linodemcp.profiles.profile import Profile

__all__ = [
    "DEFAULT_PROFILE_NAME",
    "ActiveProfileDisabledError",
    "ActiveProfileUnknownError",
    "Capability",
    "Profile",
    "ProfileError",
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profiles",
    "resolve_active_profile",
]
