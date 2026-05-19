"""Audit event tests.

Mirrors ``go/internal/audit/event_test.go``. Tests define contracts
from the spec (every field populated, empty collections serialize as
arrays, ULID format, Crockford alphabet) rather than just exercising
the current code path.
"""

from __future__ import annotations

from datetime import UTC, datetime

from linodemcp.audit import (
    EVENT_ID_PREFIX,
    Capability,
    Event,
    Mode,
    Status,
    new_event,
    new_event_id,
)

_FIXTURE_TOOL = "fixture_tool"
_FIXTURE_ENV = "fixture_env"
_FIXTURE_PROFILE = "fixture_profile"
_FIXTURE_SESSION = "sess_test_01"
_FIXTURE_VERSION = "0.0.0-test"
_ARG_LINODE_ID = "linode_id"


def _new_fixture_event() -> Event:
    """Build a baseline event used by the outcome tests."""
    return new_event(
        tool=_FIXTURE_TOOL,
        capability=Capability.WRITE,
        args={_ARG_LINODE_ID: 1},
        environment=_FIXTURE_ENV,
        profile=_FIXTURE_PROFILE,
        session_id=_FIXTURE_SESSION,
        credential_generation=1,
        linodemcp_version=_FIXTURE_VERSION,
    )


def test_new_event_populates_every_field() -> None:
    """The wire format claims every field is non-optional; assert each value lands.

    A regression that drops a field surfaces here, not at the first
    sink that tries to read it.
    """
    args = {_ARG_LINODE_ID: 12345, "confirm": True}

    evt = new_event(
        tool=_FIXTURE_TOOL,
        capability=Capability.DESTROY,
        args=args,
        environment=_FIXTURE_ENV,
        profile=_FIXTURE_PROFILE,
        session_id=_FIXTURE_SESSION,
        credential_generation=3,
        linodemcp_version=_FIXTURE_VERSION,
    )

    assert evt.ts is not None
    assert evt.ts_unix_ns > 0
    assert evt.event_id.startswith(EVENT_ID_PREFIX)
    assert evt.tool == _FIXTURE_TOOL
    assert evt.tool_capability is Capability.DESTROY
    assert evt.environment == _FIXTURE_ENV
    assert evt.profile == _FIXTURE_PROFILE
    assert evt.mode is Mode.NORMAL
    assert evt.plan_id is None
    assert evt.args[_ARG_LINODE_ID] == args[_ARG_LINODE_ID]
    assert evt.args_redacted == []
    assert evt.status is Status.SUCCESS
    assert evt.latency_ms == 0
    assert evt.result_summary == ""
    assert evt.error is None
    assert evt.linodemcp_version == _FIXTURE_VERSION
    assert evt.session_id == _FIXTURE_SESSION
    assert evt.credential_generation == 3


def test_finalize_writes_outcome_fields() -> None:
    """Finalize updates status/latency/summary and populates Error when provided."""
    evt = _new_fixture_event()

    evt.finalize(Status.ERROR, 250, "API returned 500", "instance update failed")

    assert evt.status is Status.ERROR
    assert evt.latency_ms == 250
    assert evt.result_summary == "instance update failed"
    assert evt.error == "API returned 500"


def test_finalize_with_empty_error_leaves_error_none() -> None:
    """Empty err_msg produces None error so JSON renders ``null``, not ``""``."""
    evt = _new_fixture_event()

    evt.finalize(Status.SUCCESS, 100, "", "ok")

    assert evt.status is Status.SUCCESS
    assert evt.error is None


def test_set_mode_populates_plan_id() -> None:
    """Pointer-style plan_id: set with non-empty, clear with empty."""
    evt = _new_fixture_event()

    evt.set_mode(Mode.APPLY, "plan_01H...")
    assert evt.plan_id == "plan_01H..."

    evt.set_mode(Mode.NORMAL, "")
    assert evt.plan_id is None, "empty plan_id must clear back to None"


def test_to_dict_serializes_empty_collections_as_arrays() -> None:
    """Empty args / args_redacted serialize as ``{}`` / ``[]`` not ``null``.

    JSONL consumers downstream of this expect arrays. A regression
    that drops the substitution would produce ``null`` and break the
    consumer parse.
    """
    evt = Event(
        ts=datetime.now(UTC),
        ts_unix_ns=0,
        event_id="evt_test",
        tool=_FIXTURE_TOOL,
        tool_capability=Capability.META,
        environment="",
        profile="",
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="",
        session_id="",
        credential_generation=0,
    )

    payload = evt.to_dict()

    assert payload["args"] == {}
    assert payload["args_redacted"] == []


def test_event_id_is_correct_length() -> None:
    """ULID body is 26 chars + ``evt_`` prefix = 30 total."""
    event_id = new_event_id(datetime.now(UTC))

    assert len(event_id) == 30
    assert event_id.startswith(EVENT_ID_PREFIX)


def test_event_id_uses_crockford_alphabet() -> None:
    """Crockford base32 excludes I, L, O, U to dodge visual ambiguity."""
    event_id = new_event_id(datetime.now(UTC))
    body = event_id[len(EVENT_ID_PREFIX) :]

    for char in body:
        assert char not in "ILOU", (
            "ULID body must not contain ambiguous Crockford characters"
        )


def test_event_ids_are_unique() -> None:
    """Two consecutive IDs must not collide.

    80 bits of randomness make collisions effectively impossible at
    our call rate; this is a smoke test, not a probability check.
    """
    assert new_event_id(datetime.now(UTC)) != new_event_id(datetime.now(UTC))
