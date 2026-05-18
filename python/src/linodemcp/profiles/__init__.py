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
from linodemcp.profiles.scope import Scope, required_scopes
from linodemcp.profiles.scopecheck import (
    ScopeComparison,
    compare_scopes,
    flatten_grants,
    parse_pat_scopes,
)
from linodemcp.profiles.validator import (
    GrantsFetchError,
    ProfileFetchError,
    ScopeValidationResult,
    TokenInspector,
    TokenKind,
    validate_scopes,
)

__all__ = [
    "DEFAULT_PROFILE_NAME",
    "ActiveProfileDisabledError",
    "ActiveProfileUnknownError",
    "Capability",
    "GrantsFetchError",
    "Profile",
    "ProfileError",
    "ProfileFetchError",
    "Scope",
    "ScopeComparison",
    "ScopeValidationResult",
    "TokenInspector",
    "TokenKind",
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profiles",
    "compare_scopes",
    "flatten_grants",
    "parse_pat_scopes",
    "required_scopes",
    "resolve_active_profile",
    "validate_scopes",
]
