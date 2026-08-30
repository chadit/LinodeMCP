"""Profiles infrastructure: capability tags, the ``Profile`` value type, and
the built-in catalog. Consumers import from this package rather than the
submodules so the surface stays stable across future phase refactors.
"""

from linodemcp.profiles.builtin import (
    ToolDescriptor,
    builtin_catalog_json,
    builtin_profiles,
)
from linodemcp.profiles.capability import (
    CAPABILITY_PREFIX,
    Capability,
    capability_spelling,
)
from linodemcp.profiles.errors import (
    ActiveProfileDisabledError,
    ActiveProfileUnknownError,
    ProfileError,
)
from linodemcp.profiles.loader import (
    DEFAULT_PROFILE_NAME,
    lookup_profile,
    resolve_active_profile,
)
from linodemcp.profiles.profile import Profile
from linodemcp.profiles.scope import Scope
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
    TokenNotConfiguredError,
    profile_is_elevated,
    validate_scopes,
)

__all__ = [
    "CAPABILITY_PREFIX",
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
    "TokenNotConfiguredError",
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profiles",
    "capability_spelling",
    "compare_scopes",
    "flatten_grants",
    "lookup_profile",
    "parse_pat_scopes",
    "profile_is_elevated",
    "resolve_active_profile",
    "validate_scopes",
]
