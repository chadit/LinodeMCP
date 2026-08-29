"""The projection every declared local answer ends in.

An operation answers a plain body naming no message, and the tool's own
declared message is filled from it here. A body that disagrees with the message
is reported rather than reshaped, which is the whole reason this step exists.
Mirrors ``go/internal/tools/local_response_test.go``.
"""

import json
from typing import Any

import pytest

from linodemcp.tools.local_answer import local_response

DISCARD_NAME = "seam-draft"
DISCARD_MESSAGE = "linode.mcp.v1.ProfileDraftDiscardResponse"

DISCARD_UNFILLED = f"{DISCARD_MESSAGE}: the answer leaves discarded unfilled"
DISCARD_UNDECLARED = (
    f"{DISCARD_MESSAGE}: the answer names stray, which the message does not declare"
)
DISCARD_UNFIT = f"{DISCARD_MESSAGE}: the answer does not fit"


def test_local_response_fills_the_declared_message() -> None:
    """A body that agrees with the message answers the message's own members."""
    answered = local_response(
        DISCARD_MESSAGE, {"name": DISCARD_NAME, "discarded": True}
    )

    assert len(answered) == 1
    body = json.loads(answered[0].text)
    assert body == {"name": DISCARD_NAME, "discarded": True}


@pytest.mark.parametrize(
    ("body", "want"),
    [
        pytest.param({"name": DISCARD_NAME}, DISCARD_UNFILLED, id="member left out"),
        pytest.param(
            {"name": DISCARD_NAME, "discarded": True, "stray": 1},
            DISCARD_UNDECLARED,
            id="member the message does not declare",
        ),
        pytest.param(
            {"name": DISCARD_NAME, "discarded": "yes"},
            DISCARD_UNFIT,
            id="member declared under another type",
        ),
    ],
)
def test_local_response_refuses_a_body_the_message_disagrees_with(
    body: dict[str, Any], want: str
) -> None:
    """Every disagreement is reported, in both directions, rather than dropped."""
    answered = local_response(DISCARD_MESSAGE, body)

    assert len(answered) == 1
    assert answered[0].text.startswith(f"Error: {want}")


# The nested half of the same rule. A record a member carries is a message of
# its own, and ParseDict reads a member that record leaves out as that member's
# zero, so an incomplete record answers a value nothing computed and reports
# success. These hold the walk that closes it.

AUDIT_RECENT_MESSAGE = "linode.mcp.v1.AuditRecentResponse"
AUDIT_HEALTH_MESSAGE = "linode.mcp.v1.AuditHealthResponse"
AUDIT_EVENT_MESSAGE = "linode.mcp.v1.AuditEvent"
AUDIT_SQLITE_MESSAGE = "linode.mcp.v1.AuditHealthSQLite"

AUDIT_EVENT_UNFILLED = f"{AUDIT_EVENT_MESSAGE}: the answer leaves session_id unfilled"
AUDIT_EVENT_UNDECLARED = (
    f"{AUDIT_EVENT_MESSAGE}: the answer names stray, which the message does not declare"
)
AUDIT_SQLITE_UNFILLED = f"{AUDIT_SQLITE_MESSAGE}: the answer leaves db_bytes unfilled"
AUDIT_RECENT_UNFIT = f"{AUDIT_RECENT_MESSAGE}: the answer does not fit"


def _audit_event_record() -> dict[str, Any]:
    """One event as the engine hands it over, spelled out rather than read off
    the descriptor: a case reading the message it is meant to hold the record to
    would pass whatever that message said.
    """
    return {
        "ts": "2026-05-20T00:00:01Z",
        "ts_unix_ns": 1779235201000000000,
        "event_id": "evt_nested",
        "tool": "linode_instance_list",
        "tool_capability": "read",
        "environment": "default",
        "profile": "operator",
        "mode": "normal",
        "plan_id": None,
        "args": {},
        "args_redacted": [],
        "status": "success",
        "latency_ms": 0,
        "result_summary": "",
        "error": None,
        "linodemcp_version": "0.1.0",
        "session_id": "sess-nested",
        "credential_generation": 0,
    }


def _audit_health_body(sqlite: dict[str, Any] | None) -> dict[str, Any]:
    """One health answer whose store section is the singular nested member the
    rule walks.
    """
    return {
        "jsonl_path": "audit-dir/audit.log",
        "active_log_exists": True,
        "rotated_file_count": 0,
        "oldest_rotated_date": "",
        "disk_bytes": 0,
        "dropped_events": 0,
        "sqlite": sqlite,
        "warnings": [],
    }


def test_local_response_fills_a_nested_record() -> None:
    """The control the refusing cases need: a complete record reaches the
    message and answers its own members.
    """
    answered = local_response(
        AUDIT_RECENT_MESSAGE, {"count": 1, "events": [_audit_event_record()]}
    )

    body = json.loads(answered[0].text)
    assert len(body["events"]) == 1
    assert body["events"][0]["session_id"] == "sess-nested"


def _incomplete_event() -> dict[str, Any]:
    record = _audit_event_record()
    del record["session_id"]
    return record


def _undeclared_event() -> dict[str, Any]:
    record = _audit_event_record()
    record["stray"] = 1
    return record


@pytest.mark.parametrize(
    ("message", "body", "want"),
    [
        pytest.param(
            AUDIT_RECENT_MESSAGE,
            {"count": 1, "events": [_incomplete_event()]},
            AUDIT_EVENT_UNFILLED,
            id="a listed record leaving a member out",
        ),
        pytest.param(
            AUDIT_RECENT_MESSAGE,
            {"count": 1, "events": [_undeclared_event()]},
            AUDIT_EVENT_UNDECLARED,
            id="a listed record naming a member the message does not declare",
        ),
        pytest.param(
            AUDIT_HEALTH_MESSAGE,
            _audit_health_body(
                {
                    "path": "audit-dir/audit.db",
                    "event_count": 0,
                    "oldest_event_unix_ns": 0,
                }
            ),
            AUDIT_SQLITE_UNFILLED,
            id="the single record a member carries leaving a member out",
        ),
        pytest.param(
            AUDIT_RECENT_MESSAGE,
            {"count": 1, "events": ["not a record"]},
            AUDIT_RECENT_UNFIT,
            id="a listed value that is no record at all",
        ),
    ],
)
def test_local_response_refuses_a_nested_record_the_message_disagrees_with(
    message: str, body: dict[str, Any], want: str
) -> None:
    """Every nested disagreement is answered as an error rather than dropped."""
    answered = local_response(message, body)

    assert answered[0].text.startswith(f"Error: {want}")


def test_local_response_reads_an_absent_nested_record_as_absent() -> None:
    """A store section nothing filled is not a record that left every member
    out, so it reaches the message as the absence it is.
    """
    answered = local_response(AUDIT_HEALTH_MESSAGE, _audit_health_body(None))

    body = json.loads(answered[0].text)
    assert "sqlite" not in body
