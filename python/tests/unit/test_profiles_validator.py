"""Unit tests for Phase 6.4b scope validator.

Mirrors ``go/internal/profiles/validator_test.go``. Uses a stub
``TokenInspector`` so the orchestrator stays network-free in tests.
"""

from __future__ import annotations

import pytest

from linodemcp.linode import GlobalGrants, Grants, Profile
from linodemcp.profiles import (
    GrantsFetchError,
    ProfileFetchError,
    Scope,
    TokenKind,
    validate_scopes,
)


class _FakeInspector:
    """Stub ``TokenInspector`` with programmable responses.

    Each test dials in profile/grants payloads and optional exceptions
    so PAT vs OAuth and the success/failure paths can be exercised
    without spinning up an httpx mock.
    """

    def __init__(
        self,
        *,
        profile: Profile | None = None,
        profile_exc: Exception | None = None,
        grants: Grants | None = None,
        grants_exc: Exception | None = None,
    ) -> None:
        self._profile = profile
        self._profile_exc = profile_exc
        self._grants = grants
        self._grants_exc = grants_exc
        self.grants_called = False

    async def get_profile(self) -> Profile:
        if self._profile_exc is not None:
            raise self._profile_exc
        assert self._profile is not None, (
            "test bug: profile must be set when profile_exc is None"
        )
        return self._profile

    async def get_profile_grants(self) -> Grants:
        self.grants_called = True
        if self._grants_exc is not None:
            raise self._grants_exc
        assert self._grants is not None, (
            "test bug: grants must be set when grants_exc is None"
        )
        return self._grants


def _profile(scopes: str = "", username: str = "user") -> Profile:
    return Profile(
        username=username,
        email="u@example.com",
        timezone="UTC",
        email_notifications=False,
        restricted=False,
        two_factor_auth=False,
        uid=1,
        scopes=scopes,
    )


async def test_validate_scopes_pat_path() -> None:
    """Non-empty Profile.scopes uses parse_pat_scopes and skips grants."""
    inspector = _FakeInspector(
        profile=_profile(scopes="linodes:read_write volumes:read_only")
    )

    result = await validate_scopes(
        inspector,
        [Scope.LinodesReadWrite, Scope.VolumesReadOnly],
    )

    assert result.kind == TokenKind.PAT, (
        "non-empty Profile.scopes must be classified as PAT"
    )
    assert not result.comparison.has_missing
    assert not result.comparison.has_excess
    assert not inspector.grants_called, (
        "GetProfileGrants must not be called on the PAT path"
    )


async def test_validate_scopes_oauth_path() -> None:
    """Empty Profile.scopes triggers a grants fetch and uses flatten_grants."""
    inspector = _FakeInspector(
        profile=_profile(scopes=""),
        grants=Grants(
            global_=GlobalGrants(account_access="read_only", add_linodes=True)
        ),
    )

    result = await validate_scopes(inspector, [Scope.LinodesReadWrite])

    assert result.kind == TokenKind.OAuth
    assert inspector.grants_called, "OAuth path must call get_profile_grants"
    assert not result.comparison.has_missing, (
        "add_linodes implies linodes:read_write, so nothing is missing"
    )


async def test_validate_scopes_reports_missing() -> None:
    """Under-scoped tokens surface as missing, not as an exception.

    Policy lives in the caller; this function reports the diff.
    """
    inspector = _FakeInspector(profile=_profile(scopes="linodes:read_only"))

    result = await validate_scopes(
        inspector,
        [Scope.LinodesReadWrite, Scope.VolumesReadOnly],
    )

    assert result.comparison.has_missing
    assert result.comparison.missing == (
        Scope.LinodesReadWrite,
        Scope.VolumesReadOnly,
    )


async def test_validate_scopes_profile_error_wrapped() -> None:
    """GetProfile failures bubble up as ProfileFetchError with __cause__ set."""
    original = RuntimeError("network down")
    inspector = _FakeInspector(profile_exc=original)

    with pytest.raises(ProfileFetchError) as excinfo:
        await validate_scopes(inspector, [])

    assert excinfo.value.__cause__ is original, (
        "wrapped exception must preserve the original via __cause__"
    )


async def test_validate_scopes_grants_error_wrapped() -> None:
    """OAuth-path GetProfileGrants failures raise GrantsFetchError."""
    original = RuntimeError("rate limited")
    inspector = _FakeInspector(
        profile=_profile(scopes=""),
        grants_exc=original,
    )

    with pytest.raises(GrantsFetchError) as excinfo:
        await validate_scopes(inspector, [])

    assert excinfo.value.__cause__ is original


def test_token_kind_string_values() -> None:
    """Stable string forms for log messages and audit fields."""
    assert TokenKind.Unknown.name == "Unknown"
    assert TokenKind.PAT.name == "PAT"
    assert TokenKind.OAuth.name == "OAuth"
