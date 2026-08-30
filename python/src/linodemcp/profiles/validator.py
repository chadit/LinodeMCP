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
from typing import TYPE_CHECKING, Any, Protocol

from linodemcp.genpb.linode.mcp.v1.profile_pb2 import (
    ProfileGetInput,
    ProfileGrantsGetInput,
)
from linodemcp.linode import parse_grants, parse_profile
from linodemcp.linode.routes import tool_of
from linodemcp.profiles.scopecheck import (
    compare_scopes,
    flatten_grants,
    parse_pat_scopes,
)

if TYPE_CHECKING:
    from collections.abc import Sequence

    from linodemcp.linode import Profile
    from linodemcp.profiles.profile import Profile as ProfileModel
    from linodemcp.profiles.scopecheck import ScopeComparison


def profile_is_elevated(profile: ProfileModel) -> bool:
    """Return True if the profile permits any mutating tool.

    Reads the capability-derived ``elevated`` flag, not scope suffixes:
    the API documents ``:read_write`` scopes on several read-only routes
    (kubeconfig, managed contacts, instance interfaces), so a read-only
    profile's scope union can legitimately contain write scopes without
    the profile being able to mutate anything. The missing-token policy
    uses this to decide whether to fail load (elevated) or
    warn-and-continue (read-only).

    Profiles with no allowed tools are NOT elevated; spec behavior is
    that such profiles can start without a token.
    """
    return profile.elevated


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
    """The client surface ``validate_scopes`` reads through: the
    contract-routed JSON read ``RetryableClient`` provides.

    Tests inject a stub keyed by tool so the validator stays network-free.
    Async because the production client is async.
    """

    async def route_raw(self, tool: str, *values: object) -> Any: ...


@dataclass(frozen=True)
class ScopeValidationResult:
    """The validator's structured return value.

    The caller decides what to do: ``missing`` is always a hard fail at
    load time; ``excess`` is a warn by default and a fail under strict
    mode. The profile is preserved on the result so callers can log
    the username/restricted flag alongside the comparison.
    """

    kind: TokenKind
    actual_scopes: tuple[str, ...]
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
    required: Sequence[str],
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

    Each read names the tool through its generated input message rather than
    spelling the name here, so the route stays declared in one place.

    Raises:
        ProfileFetchError: when the profile read fails. Wraps the original
            exception in ``__cause__``.
        GrantsFetchError: when the grants read fails on the OAuth branch.
            Wraps the original exception in ``__cause__``.
    """
    try:
        profile = parse_profile(await inspector.route_raw(tool_of(ProfileGetInput)))
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
        grants = parse_grants(await inspector.route_raw(tool_of(ProfileGrantsGetInput)))
    except Exception as exc:
        raise GrantsFetchError("fetch /profile/grants failed") from exc

    actual = [str(scope) for scope in flatten_grants(grants)]
    return ScopeValidationResult(
        kind=TokenKind.OAuth,
        actual_scopes=tuple(actual),
        comparison=compare_scopes(required, actual),
        profile=profile,
    )
