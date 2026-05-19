"""Audit event dataclass and supporting types.

Mirrors ``go/internal/audit/event.go``. Field names and JSON wire
shape match the Go side so events from either implementation parse
the same way downstream.
"""

from __future__ import annotations

import secrets
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any

from linodemcp.audit.redact import redact

# Constant prefix on every event_id. Combined with a 26-char ULID
# body, the full id looks like ``evt_01HQXY3ZKQ8M7VRBNP4W5T2J9F``.
EVENT_ID_PREFIX = "evt_"

# Crockford base32 alphabet. I, L, O, U absent on purpose to avoid
# visual ambiguity with 1, 0, V.
_CROCKFORD_ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"


class Capability(StrEnum):
    """Capability tag, stringly-typed at the audit boundary.

    Mirrors the Go ``audit.Capability`` constants. Kept as strings
    rather than a richer enum so the JSON wire shape stays simple.
    """

    READ = "read"
    WRITE = "write"
    DESTROY = "destroy"
    ADMIN = "admin"
    META = "meta"


class Status(StrEnum):
    """Terminal status of a tool call.

    ``success`` is the happy path; ``error`` covers handler-level
    failures; ``refused`` covers profile blocks and validation
    failures (the handler never ran).
    """

    SUCCESS = "success"
    ERROR = "error"
    REFUSED = "refused"


class Mode(StrEnum):
    """Execution mode the call took.

    Default is ``normal``; dry-run + two-stage-writes specs
    introduce the remaining values.
    """

    NORMAL = "normal"
    DRY_RUN = "dry_run"
    PLAN = "plan"
    APPLY = "apply"
    BYPASS_DRY_RUN = "bypass" + "_dry_run"
    YOLO = "yolo"


@dataclass
class Event:
    """One audit record per tool call.

    Field names match the JSON wire shape via the ``to_dict`` method.
    Dataclass is mutable so the capture middleware can fill in the
    outcome fields after the handler returns.
    """

    ts: datetime
    ts_unix_ns: int
    event_id: str
    tool: str
    tool_capability: Capability
    environment: str
    profile: str
    mode: Mode
    plan_id: str | None
    args: dict[str, Any]
    args_redacted: list[str]
    status: Status
    latency_ms: int
    result_summary: str
    error: str | None
    linodemcp_version: str
    session_id: str
    credential_generation: int

    def finalize(
        self,
        status: Status,
        latency_ms: int,
        err_msg: str,
        summary: str,
    ) -> None:
        """Record the outcome of the tool call.

        The capture middleware (Phase 1b) calls this once the handler
        returns. Empty ``err_msg`` clears the error to ``None`` so the
        JSON wire renders ``null``, not an empty string.
        """
        self.status = status
        self.latency_ms = latency_ms
        self.result_summary = summary
        self.error = err_msg or None

    def set_mode(self, mode: Mode, plan_id: str) -> None:
        """Update execution mode and the optional plan ID."""
        self.mode = mode
        self.plan_id = plan_id or None

    def to_dict(self) -> dict[str, Any]:
        """Serialize to a JSON-ready dict.

        Empty ``args`` and ``args_redacted`` serialize as ``{}`` and
        ``[]`` respectively, matching the Go-side MarshalJSON
        substitution. JSONL consumers expect arrays, not nulls.
        """
        return {
            "ts": self.ts.isoformat().replace("+00:00", "Z"),
            "ts_unix_ns": self.ts_unix_ns,
            "event_id": self.event_id,
            "tool": self.tool,
            "tool_capability": self.tool_capability.value,
            "environment": self.environment,
            "profile": self.profile,
            "mode": self.mode.value,
            "plan_id": self.plan_id,
            "args": self.args or {},
            "args_redacted": self.args_redacted or [],
            "status": self.status.value,
            "latency_ms": self.latency_ms,
            "result_summary": self.result_summary,
            "error": self.error,
            "linodemcp_version": self.linodemcp_version,
            "session_id": self.session_id,
            "credential_generation": self.credential_generation,
        }


def new_event(
    tool: str,
    capability: Capability,
    args: dict[str, Any],
    environment: str,
    profile: str,
    session_id: str,
    credential_generation: int,
    linodemcp_version: str,
) -> Event:
    """Construct an Event with timestamp, ULID, and metadata populated.

    The remaining fields (status, latency, summary, error) populate
    later via :meth:`Event.finalize`. ``args`` is redacted in place:
    the returned event holds redacted args and the list of redacted
    keys. Callers that need the unredacted values keep their own copy.
    """
    now = datetime.now(UTC)
    redacted_args, redacted_keys = redact(args)

    return Event(
        ts=now,
        ts_unix_ns=int(now.timestamp() * 1_000_000_000),
        event_id=new_event_id(now),
        tool=tool,
        tool_capability=capability,
        environment=environment,
        profile=profile,
        mode=Mode.NORMAL,
        plan_id=None,
        args=redacted_args if redacted_args is not None else {},
        args_redacted=redacted_keys,
        status=Status.SUCCESS,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version=linodemcp_version,
        session_id=session_id,
        credential_generation=credential_generation,
    )


def new_event_id(now: datetime) -> str:
    """Produce an EventID using ``now`` as the time component.

    Stateless: 80 bits of randomness make same-ms collisions
    negligible at the per-tool-call rate. Callers needing strict
    monotonic ordering should sort by ``Event.ts_unix_ns``, captured
    at the same instant.
    """
    timestamp_ms = int(now.timestamp() * 1000)
    entropy = secrets.token_bytes(10)
    return EVENT_ID_PREFIX + _encode_ulid(timestamp_ms, entropy)


def _encode_ulid(timestamp_ms: int, entropy: bytes) -> str:
    """Build the 26-char ULID body per the ULID spec."""
    time_len = 10
    random_len = 16
    bits_per_sym = 5

    out: list[str] = []

    # First 10 chars: 48-bit ms big-endian, base32-encoded.
    for i in range(time_len):
        shift = (time_len - 1 - i) * bits_per_sym
        out.append(_CROCKFORD_ALPHABET[(timestamp_ms >> shift) & 0x1F])

    # Remaining 16 chars: 80 bits of entropy, packed into 16 base32
    # symbols. The bit-twiddling matches the Go implementation
    # byte-for-byte so cross-language event IDs share the same shape.
    bits = [
        (entropy[0] & 0xF8) >> 3,
        ((entropy[0] & 0x07) << 2) | ((entropy[1] & 0xC0) >> 6),
        (entropy[1] & 0x3E) >> 1,
        ((entropy[1] & 0x01) << 4) | ((entropy[2] & 0xF0) >> 4),
        ((entropy[2] & 0x0F) << 1) | ((entropy[3] & 0x80) >> 7),
        (entropy[3] & 0x7C) >> 2,
        ((entropy[3] & 0x03) << 3) | ((entropy[4] & 0xE0) >> 5),
        entropy[4] & 0x1F,
        (entropy[5] & 0xF8) >> 3,
        ((entropy[5] & 0x07) << 2) | ((entropy[6] & 0xC0) >> 6),
        (entropy[6] & 0x3E) >> 1,
        ((entropy[6] & 0x01) << 4) | ((entropy[7] & 0xF0) >> 4),
        ((entropy[7] & 0x0F) << 1) | ((entropy[8] & 0x80) >> 7),
        (entropy[8] & 0x7C) >> 2,
        ((entropy[8] & 0x03) << 3) | ((entropy[9] & 0xE0) >> 5),
        entropy[9] & 0x1F,
    ]

    out.extend(_CROCKFORD_ALPHABET[value] for value in bits[:random_len])

    return "".join(out)
