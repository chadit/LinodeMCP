"""The audit JSONL wire schema must match the shared cross-language fixture.

testdata/audit/event_fields.json pins the event field names; Go's
event_schema_test.go asserts the same list on its side. The audit readers
and external log pipelines parse these keys, so a one-sided rename would
fork the on-disk schema; this test turns that into a failure here.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from pathlib import Path

from linodemcp.audit.event import Capability, Event, Mode, Status

_FIXTURE = (
    Path(__file__).resolve().parents[3] / "testdata" / "audit" / "event_fields.json"
)


def test_event_wire_fields_match_shared_fixture() -> None:
    event = Event(
        ts=datetime(2026, 7, 18, tzinfo=UTC),
        ts_unix_ns=1,
        event_id="evt_test",
        tool="linode_instance_list",
        tool_capability=Capability.READ,
        environment="default",
        profile="default",
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS,
        latency_ms=1,
        result_summary="ok",
        error=None,
        linodemcp_version="0.1.0",
        session_id="ses_test",
        credential_generation=1,
    )

    fixture = json.loads(_FIXTURE.read_text(encoding="utf-8"))

    assert sorted(event.to_dict()) == sorted(fixture["fields"])
