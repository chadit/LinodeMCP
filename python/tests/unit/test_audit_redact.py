"""Audit redaction tests.

Mirrors ``go/internal/audit/redact_test.go``. Tests define the
spec contract: exact-name matching (no substring, no case-fold),
recursive walk into nested dicts, no-mutation guarantee, nil safety,
and cross-language parity with the Go redaction list.
"""

from __future__ import annotations

from typing import Any, cast

from linodemcp.audit import (
    is_redacted,
    redact,
    redact_with_pii,
    redaction_field_set,
    redaction_field_set_pii,
    redaction_fields,
    redaction_fields_pii,
)

_ARG_ROOT_PASS = "root_pass"


def test_redaction_list_no_duplicates() -> None:
    """Duplicate entries don't break behavior but signal drift."""
    fields = redaction_fields()
    assert len(fields) == len(set(fields)), "redaction list must not contain duplicates"


def test_redaction_list_matches_go_canonical() -> None:
    """Cross-language parity check.

    The Go side carries the same set of redaction names. Drift here
    means the two implementations would log different things and
    confuse cross-language analysis.
    """
    expected = {
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
        "ssl_key",
        "ssh_keys",
        "token",
        "token_id",
        "token_uuid",
    }
    assert set(redaction_fields()) == expected


def test_redact_replaces_sensitive_top_level_keys() -> None:
    """Top-level sensitive keys get scrubbed; others pass through."""
    args = {
        "linode_id": 12345,
        _ARG_ROOT_PASS: "super-secret",
        "label": "my-instance",
        "token": "abc123",
    }

    redacted, keys = redact(args)

    assert redacted is not None
    assert redacted["linode_id"] == 12345
    assert redacted["label"] == "my-instance"
    assert is_redacted(redacted[_ARG_ROOT_PASS])
    assert is_redacted(redacted["token"])
    assert sorted(keys) == [_ARG_ROOT_PASS, "token"]


def test_redact_recurses_into_nested_maps() -> None:
    """Spec's match-by-name-not-depth rule.

    A sensitive name nested inside an object literal still gets
    redacted.
    """
    args = {
        "label": "test",
        "meta": {
            "api_key": "sk-leaked",
            "region": "us-east",
        },
    }

    redacted, keys = redact(args)

    assert redacted is not None
    assert isinstance(redacted["meta"], dict)
    nested = cast("dict[str, Any]", redacted["meta"])
    assert is_redacted(nested["api_key"])
    assert nested["region"] == "us-east"
    assert "api_key" in keys


def test_redact_exact_name_match() -> None:
    """Variants like ``cluster_root_pass`` don't match ``root_pass``.

    The spec's Risks section calls out that variants need explicit
    list entries; this test locks the exact-match rule.
    """
    args = {
        "cluster_root_pass": "should-pass-through-because-variant",
        "new_root_pass": "also-variant",
        "Root_Pass": "different-case",
    }

    redacted, keys = redact(args)

    assert redacted is not None
    assert redacted["cluster_root_pass"] == "should-pass-through-because-variant"
    assert redacted["new_root_pass"] == "also-variant"
    assert redacted["Root_Pass"] == "different-case"
    assert keys == []


def test_redact_none_args_produces_empty_result() -> None:
    """None safety: walker must not crash on None input."""
    redacted, keys = redact(None)

    assert redacted is None
    assert keys == []


def test_redact_returns_copy_not_mutation() -> None:
    """No-mutation contract: the caller's original args are untouched.

    A regression that scrubbed values in place would silently leak
    the original sensitive value out of the caller's continued use.
    """
    args = {_ARG_ROOT_PASS: "secret"}

    redact(args)

    assert args[_ARG_ROOT_PASS] == "secret", "original args dict must not be mutated"


def test_redaction_field_set_matches_list() -> None:
    """Set helper must reflect the same membership as the list."""
    fields = redaction_fields()
    field_set = redaction_field_set()

    assert len(field_set) == len(fields)

    for name in fields:
        assert name in field_set


def test_redaction_fields_pii_locks_conservative_scope() -> None:
    """The PII list mirrors the source-verified Go list. Each name
    appears only in account-update or profile-verification tool
    schemas; drift here means the wrong field gets redacted (or
    missed). The expected set must stay in sync with the Go side at
    ``go/internal/audit/redact.go``.
    """
    expected = {
        "address_1",
        "address_2",
        "city",
        "contact_email",
        "contact_name",
        "phone",
        "phone_number",
        "phone_primary",
        "phone_secondary",
        "state",
        "tax_id",
        "zip",
    }
    assert set(redaction_fields_pii()) == expected


def test_redaction_pii_list_no_duplicates() -> None:
    """PII list must not contain duplicates. Same drift guard as the
    credential-side dup check.
    """
    fields = redaction_fields_pii()
    assert len(fields) == len(set(fields)), (
        "PII redaction list must not contain duplicates"
    )


def test_redaction_lists_disjoint() -> None:
    """Credential and PII lists share no entries. The combined-set
    helper assumes disjoint sets so it can merge without dedup; an
    overlap would still work today (set semantics) but signals
    taxonomy drift worth catching now.
    """
    overlap = set(redaction_fields()) & set(redaction_fields_pii())
    assert overlap == set(), (
        f"credential and PII lists must be disjoint; overlap: {sorted(overlap)}"
    )


def test_redact_with_pii_scrubs_pii_fields() -> None:
    """The PII-aware entry point redacts both credential and PII names
    in one walk. This is the path the audit middleware takes when
    audit.redact_pii=true.
    """
    args = {
        "linode_id": 42,
        "label": "primary",
        "token": "abc123",
        "tax_id": "TX-99",
        "phone": "+1-555-0100",
        "address_1": "123 Main St",
        "city": "Springfield",
        "contact_name": "Jane Doe",
        "contact_email": "jane@example.org",
        "country": "us",  # not in PII list, must pass through
    }

    redacted, keys = redact_with_pii(args)

    assert redacted is not None
    assert redacted["linode_id"] == 42
    assert redacted["label"] == "primary"
    assert redacted["country"] == "us", (
        "country is a region filter, must NOT be redacted"
    )
    assert is_redacted(redacted["token"]), "credential still redacted"
    assert is_redacted(redacted["tax_id"])
    assert is_redacted(redacted["phone"])
    assert is_redacted(redacted["address_1"])
    assert is_redacted(redacted["city"])
    assert is_redacted(redacted["contact_name"])
    assert is_redacted(redacted["contact_email"])
    assert sorted(keys) == sorted(
        [
            "token",
            "tax_id",
            "phone",
            "address_1",
            "city",
            "contact_name",
            "contact_email",
        ],
    )


def test_redact_leaves_pii_when_flag_off() -> None:
    """When the operator opts out of PII redaction (audit.redact_pii=
    false), the middleware uses ``redact`` (credentials-only). PII
    passes through in cleartext; credentials stay scrubbed.
    """
    args = {
        "token": "abc123",
        "tax_id": "TX-99",
        "phone": "+1-555-0100",
        "address_1": "123 Main St",
    }

    redacted, keys = redact(args)

    assert redacted is not None
    assert is_redacted(redacted["token"]), "credential must always be redacted"
    assert redacted["tax_id"] == "TX-99", (
        "PII passes through when caller uses redact (flag-off path)"
    )
    assert redacted["phone"] == "+1-555-0100"
    assert redacted["address_1"] == "123 Main St"
    assert keys == ["token"], (
        "only the credential should appear in the redacted-key list"
    )


def test_redaction_field_set_pii_matches_list() -> None:
    """Set helper for the PII list reflects the same membership."""
    fields = redaction_fields_pii()
    field_set = redaction_field_set_pii()

    assert len(field_set) == len(fields)

    for name in fields:
        assert name in field_set


def test_redact_with_pii_none_args() -> None:
    """None safety on the PII-aware path mirrors the credential path."""
    redacted, keys = redact_with_pii(None)
    assert redacted is None
    assert keys == []
