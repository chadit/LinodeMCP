"""Phase 6.4b token-scope validator.

Mirrors ``go/internal/profiles/validator.go``. Pure orchestration:
fetch /profile, decide PAT vs OAuth from the Scopes string, fetch
/profile/grants on the OAuth branch, then run the scope comparison.
Returns a structured result; the caller (Server.validate_scopes /
main.py) decides policy.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import IntEnum
from typing import TYPE_CHECKING, Protocol

from linodemcp.profiles.scopecheck import (
    compare_scopes,
    flatten_grants,
    parse_pat_scopes,
)

if TYPE_CHECKING:
    from linodemcp.linode import Grants, Profile
    from linodemcp.profiles.profile import Profile as ProfileModel
    from linodemcp.profiles.scope import Scope
    from linodemcp.profiles.scopecheck import ScopeComparison


def profile_is_elevated(profile: ProfileModel) -> bool:
    """Return True if the profile carries any write/destroy capability.

    Judged by the presence of a ``:read_write`` suffix in any required
    token scope. The missing-token policy uses this to decide whether
    to fail load (elevated) or warn-and-continue (read-only).

    Profiles with no required scopes (e.g. a freshly-loaded user
    profile with no allowed tools) are NOT elevated by this rule; spec
    behavior is that such profiles can start without a token.
    """
    return any(scope.endswith(":read_write") for scope in profile.required_token_scopes)


class TokenNotConfiguredError(Exception):
    """Raised from ``Server.validate_scopes`` when the active environment
    has no Linode token set. The caller (main) decides what to do:
    read-only profiles warn and continue, elevated profiles fail to
    start.
    """


class TokenKind(IntEnum):
    """Classifies the active token as PAT vs OAuth.

    Personal access tokens carry their scope string directly on the
    ``/profile`` response; OAuth tokens leave it empty and require a
    second call to ``/profile/grants``. The validator picks the path
    based on what ``/profile`` returns; consumers use the kind for
    logging and audit.
    """

    Unknown = 0
    PAT = 1
    OAuth = 2


class TokenInspector(Protocol):
    """Minimal client surface ``validate_scopes`` needs.

    The real ``RetryableClient`` satisfies this Protocol structurally;
    tests inject a stub so the validator stays network-free. Both
    methods are async because the production client is async.
    """

    async def get_profile(self) -> Profile: ...

    async def get_profile_grants(self) -> Grants: ...


@dataclass(frozen=True)
class ScopeValidationResult:
    """The validator's structured return value.

    The caller decides what to do: ``missing`` is always a hard fail at
    load time; ``excess`` is a warn by default and a fail under strict
    mode. The profile is preserved on the result so callers can log
    the username/restricted flag alongside the comparison.
    """

    kind: TokenKind
    actual_scopes: tuple[Scope, ...]
    comparison: ScopeComparison
    profile: Profile


class ProfileFetchError(Exception):
    """Raised when the underlying GET /profile call fails.

    Wraps the original exception so callers can match this class to
    distinguish a network/API failure from a scope mismatch (which is
    reported via the comparison, not an exception).
    """


class GrantsFetchError(Exception):
    """Raised when the OAuth-path GET /profile/grants call fails."""


async def validate_scopes(
    inspector: TokenInspector,
    required: list[Scope],
) -> ScopeValidationResult:
    """Inspect a token's scopes and diff against the profile's required set.

    PAT path: ``Profile.scopes`` is non-empty, parse it via
    ``parse_pat_scopes`` and skip ``/profile/grants``. OAuth path:
    empty ``Profile.scopes`` triggers a grants fetch and
    ``flatten_grants`` produces the actual set.

    Policy decisions live in the caller: this function reports facts
    only. A comparison with ``missing`` entries is a load-time failure
    under the spec, but this function returns the result normally so
    callers can inspect ``missing`` and ``excess`` together.

    Raises:
        ProfileFetchError: when ``get_profile`` fails. Wraps the
            original exception in ``__cause__``.
        GrantsFetchError: when ``get_profile_grants`` fails on the
            OAuth branch. Wraps the original exception in ``__cause__``.
    """
    try:
        profile = await inspector.get_profile()
    except Exception as exc:
        raise ProfileFetchError("fetch /profile failed") from exc

    if profile.scopes:
        actual = parse_pat_scopes(profile.scopes)
        return ScopeValidationResult(
            kind=TokenKind.PAT,
            actual_scopes=tuple(actual),
            comparison=compare_scopes(required, actual),
            profile=profile,
        )

    try:
        grants = await inspector.get_profile_grants()
    except Exception as exc:
        raise GrantsFetchError("fetch /profile/grants failed") from exc

    actual = flatten_grants(grants)
    return ScopeValidationResult(
        kind=TokenKind.OAuth,
        actual_scopes=tuple(actual),
        comparison=compare_scopes(required, actual),
        profile=profile,
    )
