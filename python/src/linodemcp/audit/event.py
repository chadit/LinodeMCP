"""Audit event vocabularies and the record they tag.

The record itself is the contract's own ``AuditEvent``, which declares
``local_record``, so the members and their order on disk come from the
declaration rather than from this language's dataclass. Mirrors
``go/internal/audit/event.go``.

The vocabularies below stay this module's own words: they are what the server
tags a call with, and the record carries their text.
"""

from __future__ import annotations

import secrets
from dataclasses import replace
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any

from linodemcp.audit.redact import redact, redact_with_pii
from linodemcp.genlocal import AuditEvent

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


# Event is one audit record per tool call, which is the contract's own
# AuditEvent under this module's name for it. One declaration owns the member
# set, their order and their types, so a log line reads the same whichever
# language wrote it.
Event = AuditEvent


def event_timestamp(instant: datetime) -> str:
    """One instant as an event's ts member spells it.

    No fractional part where the instant lands on a whole second, and
    microseconds where it does not, which is the spelling a record written by
    an earlier version carries and a reconstructed timestamp comes back in.
    """
    return instant.astimezone(UTC).isoformat().replace("+00:00", "Z")


def new_event(
    tool: str,
    capability: Capability,
    args: dict[str, Any],
    environment: str,
    profile: str,
    session_id: str,
    credential_generation: int,
    linodemcp_version: str,
    *,
    redact_pii: bool = False,
) -> Event:
    """Construct an Event with timestamp, ULID, and metadata populated.

    The remaining fields (status, latency, summary, error) populate
    later via :func:`finalize`. ``args`` is redacted in place: the
    returned event holds redacted args and the list of redacted keys.
    Callers that need the unredacted values keep their own copy.

    ``redact_pii`` controls the PII redaction tier (Phase 4c). The
    default False applies only the always-on credential list; the
    middleware passes True when the operator's config has
    ``audit.redact_pii: true`` (the default). Keyword-only to avoid
    accidental positional confusion with the other booleans the
    capture middleware might add later.

    The nanosecond count is built from the seconds and the microseconds rather
    than from a float, which loses the last digits of a modern timestamp.
    """
    now = datetime.now(UTC)
    redact_fn = redact_with_pii if redact_pii else redact
    redacted_args, redacted_keys = redact_fn(args)

    return AuditEvent(
        ts=event_timestamp(now),
        ts_unix_ns=int(now.timestamp()) * 1_000_000_000 + now.microsecond * 1000,
        event_id=new_event_id(now),
        tool=tool,
        tool_capability=capability.value,
        environment=environment,
        profile=profile,
        mode=Mode.NORMAL.value,
        plan_id=None,
        args=redacted_args if redacted_args is not None else {},
        args_redacted=redacted_keys,
        status=Status.SUCCESS.value,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version=linodemcp_version,
        session_id=session_id,
        credential_generation=credential_generation,
    )


def finalize(
    event: Event,
    status: Status,
    latency_ms: int,
    err_msg: str,
    summary: str,
) -> Event:
    """Answer the event the call ended as.

    The capture middleware calls this once the handler returns. A new record
    rather than four writes into the old one, because the record is a value the
    contract owns and this language's is frozen. Empty ``err_msg`` clears the
    error to ``None`` so the record renders ``null``, not an empty string.
    """
    return replace(
        event,
        status=status.value,
        latency_ms=latency_ms,
        result_summary=summary,
        error=err_msg or None,
    )


def set_mode(event: Event, mode: Mode, plan_id: str) -> Event:
    """Answer the event under the execution mode the call took."""
    return replace(event, mode=mode.value, plan_id=plan_id or None)


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
