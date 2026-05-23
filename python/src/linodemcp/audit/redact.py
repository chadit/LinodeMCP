"""Redaction helper for the audit event args field.

Mirrors ``go/internal/audit/redact.go``. Match semantics per the spec:
exact field name only. No substring, no suffix, no case-fold. A
variant like ``cluster_root_pass`` needs its own entry.

The cross-language parity test asserts that the field list here
matches the Go side byte-for-byte.
"""

from __future__ import annotations

from typing import Any, cast

# Placeholder string written into the audit event in place of a
# sensitive value. Exposed as a constant so tests can compare
# without re-defining the literal.
REDACTED_VALUE = "[REDACTED]"


def redaction_fields() -> list[str]:
    """Canonical list of arg names whose values get redacted.

    Match semantics: exact field name. The list is returned by
    function rather than a module-level constant so tests can call
    it without worrying about a shared mutable reference, and so the
    runtime walker rebuilds a fresh set on each call (the audit
    middleware in Phase 1b caches the lookup set; per-call rebuild
    is fine until then).
    """
    return [
        "api_key",
        "apiKey",
        "authorized_keys",
        "data",
        "kubeconfig",
        "pass",
        "password",
        "password_created",
        "private_key",
        "root_pass",
        "secret",
        "service_token",
        "ssh_key",
        "ssh_keys",
        "token",
    ]


def redaction_field_set() -> frozenset[str]:
    """Return the redaction list as a frozenset for O(1) lookups."""
    return frozenset(redaction_fields())


def redact(args: dict[str, Any] | None) -> tuple[dict[str, Any] | None, list[str]]:
    """Walk ``args`` and replace sensitive values with REDACTED_VALUE.

    Returns the redacted copy and a sorted list of every key that
    was redacted (deduped across the recursive walk). The original
    args dict is NOT mutated.

    The walk recurses into nested dicts but does NOT recurse into
    lists of dicts. Deliberate simplification: every sensitive arg
    in the current tool surface lives at the top level or inside a
    nested object literal, never inside an array element.
    """
    if args is None:
        return None, []

    fields = redaction_field_set()
    redacted_keys: set[str] = set()
    out = _redact_map(args, fields, redacted_keys)

    return out, sorted(redacted_keys)


def _redact_map(
    source: dict[str, Any],
    fields: frozenset[str],
    redacted_keys: set[str],
) -> dict[str, Any]:
    """Recursive worker. Copies into a new dict; sensitive values are replaced."""
    out: dict[str, Any] = {}

    for key, value in source.items():
        if key in fields:
            out[key] = REDACTED_VALUE
            redacted_keys.add(key)
            continue

        if isinstance(value, dict):
            nested_typed = cast("dict[str, Any]", value)
            nested = _redact_map(nested_typed, fields, redacted_keys)
            out[key] = nested
            continue

        out[key] = value

    return out


def is_redacted(value: object) -> bool:
    """Report whether a value position holds the redaction placeholder."""
    return isinstance(value, str) and value == REDACTED_VALUE
