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
    redaction_field_set,
    redaction_fields,
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
        "root_pass",
        "secret",
        "service_token",
        "token",
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
