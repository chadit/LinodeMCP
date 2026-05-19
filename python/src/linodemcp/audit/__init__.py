"""Audit event schema and supporting helpers.

Mirrors ``go/internal/audit``. Phase 1a (this code) builds the event
dataclass and the redaction helper; Phase 1b wires capture into the
tool dispatch middleware; later phases add JSONL and SQLite sinks
plus query tools.
"""

from __future__ import annotations

from linodemcp.audit.event import (
    EVENT_ID_PREFIX,
    Capability,
    Event,
    Mode,
    Status,
    new_event,
    new_event_id,
)
from linodemcp.audit.redact import (
    REDACTED_VALUE,
    is_redacted,
    redact,
    redaction_field_set,
    redaction_fields,
)
from linodemcp.audit.sink import CapturingSink, NoopSink, Sink

__all__ = [
    "EVENT_ID_PREFIX",
    "REDACTED_VALUE",
    "Capability",
    "CapturingSink",
    "Event",
    "Mode",
    "NoopSink",
    "Sink",
    "Status",
    "is_redacted",
    "new_event",
    "new_event_id",
    "redact",
    "redaction_field_set",
    "redaction_fields",
]
