"""Unit tests for Phase 6.4b scope validator.

Mirrors ``go/internal/profiles/validator_test.go``. Uses a stub
``TokenInspector`` so the orchestrator stays network-free in tests.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from linodemcp.linode import RetryableClient
from linodemcp.profiles import (
    GrantsFetchError,
    ProfileFetchError,
    Scope,
    TokenKind,
    validate_scopes,
)

# The two tools the validator reads through, spelled here because a stub has
# to dispatch on what the real client would be asked for.
PROFILE_TOOL = "linode_profile_get"
GRANTS_TOOL = "linode_profile_grants_get"


class _FakeInspector:
    """Stub ``TokenInspector`` with programmable responses.

    Each test dials in profile/grants bodies and optional exceptions so PAT
    vs OAuth and the success/failure paths can be exercised without spinning
    up an httpx mock. Any other tool is refused, so a validator that reached
    for a third route fails here by name.
    """

    def __init__(
        self,
        *,
        profile: dict[str, Any] | None = None,
        profile_exc: Exception | None = None,
        grants: dict[str, Any] | None = None,
        grants_exc: Exception | None = None,
    ) -> None:
        self._profile = profile
        self._profile_exc = profile_exc
        self._grants = grants
        self._grants_exc = grants_exc
        self.grants_called = False

    async def route_raw(self, tool: str, *values: object) -> Any:
        assert not values, "the profile reads fill no path slot"
        if tool == PROFILE_TOOL:
            return self._answer(self._profile, self._profile_exc)
        if tool == GRANTS_TOOL:
            self.grants_called = True
            return self._answer(self._grants, self._grants_exc)
        msg = f"unexpected tool {tool}"
        raise AssertionError(msg)

    @staticmethod
    def _answer(body: dict[str, Any] | None, exc: Exception | None) -> Any:
        if exc is not None:
            raise exc
        assert body is not None, "test bug: a body must be set when exc is None"
        return body


def _profile(scopes: str = "", username: str = "user") -> dict[str, Any]:
    """A /profile body, the shape the validator parses."""
    return {
        "username": username,
        "email": "u@example.com",
        "timezone": "UTC",
        "email_notifications": False,
        "restricted": False,
        "two_factor_auth": False,
        "uid": 1,
        "scopes": scopes,
    }


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
        "the grants route must not be read on the PAT path"
    )


async def test_validate_scopes_pat_path_with_scopes_outside_the_catalog() -> None:
    """The PAT path compares catalog-external scope strings by value.

    Built-in unions require declared values the token catalog does not
    spell, and user-defined profiles may require anything; both must
    validate when the token genuinely carries the scope.
    """
    inspector = _FakeInspector(
        profile=_profile(scopes="events:read_only child_account:read_write")
    )

    result = await validate_scopes(
        inspector,
        ["events:read_only", "child_account:read_write"],
    )

    assert not result.comparison.has_missing
    assert not result.comparison.has_excess


async def test_validate_scopes_oauth_path() -> None:
    """Empty Profile.scopes triggers a grants fetch and uses flatten_grants."""
    inspector = _FakeInspector(
        profile=_profile(scopes=""),
        grants={"global": {"account_access": "read_only", "add_linodes": True}},
    )

    result = await validate_scopes(inspector, [Scope.LinodesReadWrite])

    assert result.kind == TokenKind.OAuth
    assert inspector.grants_called, "OAuth path must read the grants route"
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
    """A failed profile read bubbles up as ProfileFetchError with __cause__ set."""
    original = RuntimeError("network down")
    inspector = _FakeInspector(profile_exc=original)

    with pytest.raises(ProfileFetchError) as excinfo:
        await validate_scopes(inspector, [])

    assert excinfo.value.__cause__ is original, (
        "wrapped exception must preserve the original via __cause__"
    )


async def test_validate_scopes_grants_error_wrapped() -> None:
    """An OAuth-path grants read failure raises GrantsFetchError."""
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


async def test_validate_scopes_reads_the_declared_profile_routes() -> None:
    """The real client is proven to reach GET /profile and GET /profile/grants.

    An empty scope string sends the validator down the OAuth path, the one
    that makes both reads, so both routes are observed in one run.
    """
    bodies = {
        "https://api.linode.com/v4/profile": _profile(username="oauthuser"),
        "https://api.linode.com/v4/profile/grants": {"global": {"add_linodes": True}},
    }

    async def answer(method: str, url: str, **_: Any) -> MagicMock:
        response = MagicMock()
        response.status_code = 200
        response.json.return_value = bodies[url]
        return response

    client = RetryableClient("https://api.linode.com/v4", "oauth-token")
    with patch.object(client.client.client, "request", new_callable=AsyncMock) as req:
        req.side_effect = answer

        result = await validate_scopes(client, [Scope.LinodesReadWrite])

        assert [call.args[:2] for call in req.await_args_list] == [
            ("GET", "https://api.linode.com/v4/profile"),
            ("GET", "https://api.linode.com/v4/profile/grants"),
        ]

    await client.close()

    assert result.kind == TokenKind.OAuth
    assert result.profile.username == "oauthuser"
    assert not result.comparison.has_missing, (
        "add_linodes implies linodes:read_write, so nothing is missing"
    )
