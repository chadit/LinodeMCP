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
        "security_questions",
        "service_token",
        "ssh_key",
        "ssl_key",
        "ssh_keys",
        "token",
        "token_id",
        "token_uuid",
    ]


def redaction_field_set() -> frozenset[str]:
    """Return the redaction list as a frozenset for O(1) lookups."""
    return frozenset(redaction_fields())


def redaction_fields_pii() -> list[str]:
    """Conservative PII arg list that gets scrubbed in addition to
    ``redaction_fields`` when the ``audit.redact_pii`` config flag is
    true (Phase 4c, default true).

    Each name was source-verified against the live tool schemas: every
    occurrence in current tools is unambiguously postal address / PII,
    never a non-sensitive filter or selector. Names deliberately left
    out so login identifiers stay readable in audit reports: email,
    first_name, last_name, company. Contact-specific name/email tool
    args use contact_name/contact_email and are redacted. Names dropped
    after source review because they collide with non-PII tool args: country
    (linode_region_list filter), address (network/IP address in the
    linode_instance_ip_*, linode_networking_*, and linode_nodebalancer_*
    families).

    Cross-language parity is asserted by the unit test that mirrors
    this list against the Go equivalent at
    ``go/internal/audit/redact.go``.
    """
    return [
        "address_1",
        "address_2",
        "city",
        "phone",
        "phone_number",
        "state",
        "tax_id",
        "zip",
    ]


def redaction_field_set_pii() -> frozenset[str]:
    """Return the PII redaction list as a frozenset for O(1) lookups."""
    return frozenset(redaction_fields_pii())


def redact(args: dict[str, Any] | None) -> tuple[dict[str, Any] | None, list[str]]:
    """Walk ``args`` and replace sensitive (credential) values with
    REDACTED_VALUE.

    Returns the redacted copy and a sorted list of every key that
    was redacted (deduped across the recursive walk). The original
    args dict is NOT mutated. Credentials are always redacted; this
    entry point does NOT touch PII fields. Use ``redact_with_pii``
    for the combined set.

    The walk recurses into nested dicts but does NOT recurse into
    lists of dicts. Deliberate simplification: every sensitive arg
    in the current tool surface lives at the top level or inside a
    nested object literal, never inside an array element.
    """
    return _redact_with_fields(args, redaction_field_set())


def redact_with_pii(
    args: dict[str, Any] | None,
) -> tuple[dict[str, Any] | None, list[str]]:
    """Walk ``args`` and replace BOTH credential and PII values with
    REDACTED_VALUE.

    Used by the audit middleware when ``audit.redact_pii`` is true
    (Phase 4c default). When the operator opts out via
    ``audit.redact_pii: false``, the middleware uses ``redact``
    instead so PII passes through in cleartext while credentials stay
    scrubbed.
    """
    return _redact_with_fields(args, _combined_redaction_field_set())


def _combined_redaction_field_set() -> frozenset[str]:
    """Union of credential and PII redaction names. The disjoint-sets
    test guards against an entry sneaking into both lists.
    """
    return redaction_field_set() | redaction_field_set_pii()


def _redact_with_fields(
    args: dict[str, Any] | None,
    fields: frozenset[str],
) -> tuple[dict[str, Any] | None, list[str]]:
    """Shared walker entry point. Extracted so credential-only and
    credential+PII paths share the same recursive copy logic.
    """
    if args is None:
        return None, []

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
