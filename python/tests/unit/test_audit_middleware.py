"""Audit middleware tests for Server.dispatch.

Mirrors ``go/internal/server/audit_middleware_test.go``. Verifies
that the capture wraps every dispatched call, records success vs
refusal correctly, and that ``set_audit_sink(None)`` restores the
NoopSink default.
"""

from __future__ import annotations

import dataclasses
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import Capability as AuditCapability
from linodemcp.audit import CapturingSink, Status
from linodemcp.config import BuiltinOverride
from linodemcp.server import Server

if TYPE_CHECKING:
    from linodemcp.config import Config


def _full_access(base: Config) -> Config:
    """Switch the supplied config to the full-access built-in.

    Mirrors the helper in test_server.py so audit tests don't need
    to share a separate fixture file.
    """
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={
            "full-access": BuiltinOverride(disabled=False),
        },
    )


@pytest.mark.asyncio
async def test_audit_middleware_records_success(sample_config: Config) -> None:
    """A reaching hello call records one event with success + CapMeta + args."""
    srv = Server(_full_access(sample_config))
    sink = CapturingSink()
    srv.set_audit_sink(sink)

    await srv.dispatch("hello", {"name": "Auditor"})

    events = sink.events()
    assert len(events) == 1
    event = events[0]
    assert event.tool == "hello"
    assert event.tool_capability is AuditCapability.META
    assert event.status is Status.SUCCESS
    assert event.error is None
    assert event.args["name"] == "Auditor"
    assert event.latency_ms >= 0


@pytest.mark.asyncio
async def test_audit_middleware_records_refusal_on_unknown_tool(
    sample_config: Config,
) -> None:
    """Dispatch on an unknown tool name records status=refused, then raises."""
    srv = Server(_full_access(sample_config))
    sink = CapturingSink()
    srv.set_audit_sink(sink)

    with pytest.raises(ValueError, match="Unknown tool"):
        await srv.dispatch("nonexistent_tool", {})

    events = sink.events()
    assert len(events) == 1
    event = events[0]
    assert event.tool == "nonexistent_tool"
    assert event.status is Status.REFUSED
    assert event.error is not None


@pytest.mark.asyncio
async def test_set_audit_sink_none_restores_noop(sample_config: Config) -> None:
    """Passing None restores NoopSink; the previous sink stops receiving."""
    srv = Server(_full_access(sample_config))
    sink = CapturingSink()
    srv.set_audit_sink(sink)
    srv.set_audit_sink(None)

    await srv.dispatch("hello", {"name": "Auditor"})

    assert sink.events() == [], (
        "previously-installed sink must not receive events after set_audit_sink(None)"
    )


_PII_HELLO_ARGS = {
    "name": "Auditor",
    "phone": "+1-555-0100",
    "tax_id": "TX-99",
    "address_1": "123 Main St",
    "city": "Springfield",
    "country": "us",
    "token": "shh-credential",
}


@pytest.mark.asyncio
async def test_audit_middleware_redacts_pii_when_flag_on(
    sample_config: Config,
) -> None:
    """With set_audit_redact_pii(True), the captured event scrubs PII
    fields by name while non-PII passes through and credentials stay
    scrubbed. Hello ignores unknown args, but the audit walker reads
    the raw input map, so this exercises the Phase 4c tier end-to-end.
    """
    srv = Server(_full_access(sample_config))
    sink = CapturingSink()
    srv.set_audit_sink(sink)
    srv.set_audit_redact_pii(redact_pii=True)

    await srv.dispatch("hello", dict(_PII_HELLO_ARGS))

    event = sink.events()[0]
    assert event.args["phone"] == "[REDACTED]"
    assert event.args["tax_id"] == "[REDACTED]"
    assert event.args["address_1"] == "[REDACTED]"
    assert event.args["city"] == "[REDACTED]"
    assert event.args["token"] == "[REDACTED]", (
        "credential is always redacted regardless of redact_pii"
    )
    assert event.args["country"] == "us", (
        "country is a non-sensitive filter, must pass through"
    )
    assert event.args["name"] == "Auditor"
    assert "phone" in event.args_redacted
    assert "token" in event.args_redacted


@pytest.mark.asyncio
async def test_audit_middleware_leaves_pii_when_flag_off(
    sample_config: Config,
) -> None:
    """With the default opt-out (set_audit_redact_pii(False) or never
    set), PII passes through in cleartext while credentials are still
    scrubbed.
    """
    srv = Server(_full_access(sample_config))
    sink = CapturingSink()
    srv.set_audit_sink(sink)
    srv.set_audit_redact_pii(redact_pii=False)

    await srv.dispatch("hello", dict(_PII_HELLO_ARGS))

    event = sink.events()[0]
    assert event.args["phone"] == "+1-555-0100"
    assert event.args["tax_id"] == "TX-99"
    assert event.args["address_1"] == "123 Main St"
    assert event.args["city"] == "Springfield"
    assert event.args["token"] == "[REDACTED]", (
        "credential is still redacted when redact_pii=False"
    )
    assert "phone" not in event.args_redacted, (
        "PII names absent from args_redacted when flag is off"
    )
    assert "token" in event.args_redacted
