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
from linodemcp.profiles.profile import Profile

__all__ = [
    "Capability",
    "Profile",
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profiles",
]
