"""Dry-run helper tests.

Mirrors ``go/internal/tools/dryrun_test.go``. Verifies the
``is_dry_run`` predicate (strict-bool, default-false) and the
``build_dry_run_response`` v0 wire shape. Cross-language parity:
both helpers must produce the same JSON shape for the same inputs.
"""

from __future__ import annotations

import json
from typing import Any, cast

import pytest
from mcp.types import TextContent

from linodemcp.tools.helpers import (
    PARAM_DRY_RUN,
    build_dry_run_response,
    is_dry_run,
)


def test_is_dry_run_true() -> None:
    """An explicit ``dry_run: True`` returns True. The happy path."""
    assert is_dry_run({PARAM_DRY_RUN: True}) is True


def test_is_dry_run_false() -> None:
    """An explicit ``dry_run: False`` returns False."""
    assert is_dry_run({PARAM_DRY_RUN: False}) is False


def test_is_dry_run_missing_defaults_false() -> None:
    """An omitted ``dry_run`` defaults to False (execute the call).

    Catches a regression that flips the default; the spec is explicit
    that omitting the parameter means "execute".
    """
    assert is_dry_run({}) is False


@pytest.mark.parametrize(
    ("value", "label"),
    [
        ("true", "string-true"),
        ("false", "string-false"),
        (1, "numeric-one"),
        (0, "numeric-zero"),
        (None, "none"),
        ([], "empty-list"),
        ({}, "empty-dict"),
    ],
)
def test_is_dry_run_wrong_type_degrades_to_false(
    value: Any,
    label: str,
) -> None:
    """Only the literal Python ``True`` counts as dry-run.

    String "true", numeric 1, and other truthy values do not.
    Matches the Go-side strict-bool rule. MCP schema validation
    enforces the type upstream so a wrong-type value reaching the
    handler implies an upstream bug; degrading to False keeps the
    behavior predictable.
    """
    assert is_dry_run({PARAM_DRY_RUN: value}) is False, (
        f"non-bool {label} must not satisfy dry_run"
    )


def test_build_dry_run_response_wire_shape() -> None:
    """The v0 wire shape: dry_run, tool, environment, would_execute, current_state.

    The test decodes the JSON body back into a dict so a future
    renaming of a struct field or JSON key gets caught.
    """
    current_state = {"id": 12345, "label": "web-01", "status": "running"}

    out = build_dry_run_response(
        "linode_instance_delete",
        "prod",
        "DELETE",
        "/linode/instances/12345",
        current_state,
    )

    assert len(out) == 1, "must return exactly one TextContent"
    assert isinstance(out[0], TextContent)
    body = json.loads(out[0].text)

    assert body["dry_run"] is True
    assert body["tool"] == "linode_instance_delete"
    assert body["environment"] == "prod"
    assert body["would_execute"] == {
        "method": "DELETE",
        "path": "/linode/instances/12345",
    }
    state = cast("dict[str, Any]", body["current_state"])
    assert state["id"] == 12345
    assert state["label"] == "web-01"
    assert state["status"] == "running"


def test_build_dry_run_response_omits_empty_environment() -> None:
    """An empty ``environment`` is omitted from the wire shape, not
    serialized as ``"environment": ""``.

    Models reading the response can distinguish absent from
    present-but-empty. Mirrors the Go-side ``omitempty`` JSON tag
    behavior; if Python's conditional-insert is removed by mistake,
    this test breaks.
    """
    out = build_dry_run_response(
        "linode_audit_health",
        "",
        "GET",
        "/linode/audit/health",
        None,
    )
    body = json.loads(out[0].text)
    assert "environment" not in body, (
        "environment must be omitted when empty, not present as empty string"
    )


def test_build_dry_run_response_null_current_state() -> None:
    """A None current_state round-trips as JSON null.

    Useful for tools where the resource doesn't exist yet (create
    operations in dry-run mode); the response still tells the caller
    what would have been requested.
    """
    out = build_dry_run_response(
        "linode_volume_create",
        "prod",
        "POST",
        "/linode/volumes",
        None,
    )
    body = json.loads(out[0].text)
    assert body["current_state"] is None


def test_build_dry_run_response_includes_request_body() -> None:
    """Optional request_body is nested under the would_execute preview."""
    response = build_dry_run_response(
        "tool",
        "",
        "POST",
        "/resource",
        None,
        request_body={"id": "beta"},
    )

    body = json.loads(response[0].text)
    assert body["would_execute"]["body"] == {"id": "beta"}
