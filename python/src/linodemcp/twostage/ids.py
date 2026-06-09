"""Two-stage plan-id generation.

Mirrors ``go/internal/twostage/id.go``. The Go side uses a UUIDv7; Python
3.13 has no stdlib UUIDv7, so this builds a time-sortable id from a
millisecond timestamp prefix plus random entropy. The contract only needs
the id to be sortable, opaque, and distinguishable by its prefix.
"""

from __future__ import annotations

import secrets
import time

PLAN_ID_PREFIX = "plan_"

# Width of the zero-padded hex millisecond timestamp. 11 hex digits hold
# timestamps well past the year 9000, so lexical order tracks creation order.
_TIMESTAMP_HEX_WIDTH = 11

# Bytes of random suffix entropy. 8 bytes (16 hex chars) makes a collision
# within a single millisecond effectively impossible.
_ENTROPY_BYTES = 8


def new_plan_id() -> str:
    """Return a fresh, time-sortable plan id prefixed with ``plan_``."""
    timestamp_ms = int(time.time() * 1000)
    timestamp_hex = f"{timestamp_ms:0{_TIMESTAMP_HEX_WIDTH}x}"
    return f"{PLAN_ID_PREFIX}{timestamp_hex}{secrets.token_hex(_ENTROPY_BYTES)}"
