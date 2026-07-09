"""Unit tests for the CLI shared helpers.

These are pure functions (arg coercion, safety-flag folding, result rendering,
error detection) with no async or server involvement, so they exercise the
request-building contract directly: a CLI string becomes the right typed value
before it ever reaches dispatch.
"""

from __future__ import annotations

import json

import pytest
from mcp.types import Tool

from linodemcp.cli._shared import (
    ArgError,
    build_arguments,
    coerce_arg_pairs,
    fold_safety_flags,
    is_error_result,
    parse_json_args,
    render_output,
    required_args,
    result_text,
    schema_properties,
)


def _tool_with_schema(properties: dict[str, object], required: list[str]) -> Tool:
    """Build a throwaway Tool with the given input schema for coercion tests."""
    return Tool(
        name="linode_test_tool",
        description="test tool",
        inputSchema={
            "type": "object",
            "properties": properties,
            "required": required,
        },
    )


class _Block:
    """Minimal stand-in for a TextContent block (only ``.text`` is read)."""

    def __init__(self, text: str) -> None:
        self.text = text


def test_schema_properties_returns_property_map() -> None:
    """schema_properties pulls the properties dict out of the input schema."""
    tool = _tool_with_schema({"instance_id": {"type": "integer"}}, [])
    props = schema_properties(tool)
    assert props == {"instance_id": {"type": "integer"}}


def test_schema_properties_empty_when_absent() -> None:
    """A tool with no properties yields an empty dict, not a crash."""
    tool = Tool(name="x", description="d", inputSchema={"type": "object"})
    assert schema_properties(tool) == {}


def test_required_args_reads_required_list() -> None:
    """required_args returns the schema's required names."""
    tool = _tool_with_schema({"a": {"type": "string"}}, ["a"])
    assert required_args(tool) == ["a"]


def test_coerce_integer_and_number_and_boolean() -> None:
    """key=value pairs coerce to the schema property's type."""
    props = {
        "size": {"type": "integer"},
        "ratio": {"type": "number"},
        "on": {"type": "boolean"},
        "label": {"type": "string"},
    }
    out = coerce_arg_pairs(
        ["size=20", "ratio=1.5", "on=true", "label=v1"],
        props,
    )
    assert out == {"size": 20, "ratio": 1.5, "on": True, "label": "v1"}


def test_coerce_boolean_falsy_words() -> None:
    """Common falsy words coerce to False for a boolean property."""
    props = {"flag": {"type": "boolean"}}
    for word in ("false", "0", "no", "off"):
        assert coerce_arg_pairs([f"flag={word}"], props) == {"flag": False}


def test_coerce_unknown_key_stays_string() -> None:
    """A key absent from the schema is left as a string."""
    out = coerce_arg_pairs(["mystery=42"], {})
    assert out == {"mystery": "42"}


def test_coerce_bad_integer_raises_argerror() -> None:
    """A non-integer value for an integer property is a usage error."""
    props = {"size": {"type": "integer"}}
    with pytest.raises(ArgError):
        coerce_arg_pairs(["size=big"], props)


def test_coerce_bad_boolean_raises_argerror() -> None:
    """A value that is neither truthy nor falsy is a usage error."""
    props = {"flag": {"type": "boolean"}}
    with pytest.raises(ArgError):
        coerce_arg_pairs(["flag=maybe"], props)


def test_coerce_missing_equals_raises_argerror() -> None:
    """A --arg token without '=' cannot be parsed."""
    with pytest.raises(ArgError):
        coerce_arg_pairs(["justakey"], {})


def test_parse_json_args_object() -> None:
    """A JSON object blob parses into a dict."""
    assert parse_json_args('{"a": 1, "b": "x"}') == {"a": 1, "b": "x"}


def test_parse_json_args_rejects_non_object() -> None:
    """A top-level JSON array or scalar is a usage error."""
    with pytest.raises(ArgError):
        parse_json_args("[1, 2, 3]")


def test_parse_json_args_rejects_malformed() -> None:
    """Malformed JSON is a usage error, not a crash."""
    with pytest.raises(ArgError):
        parse_json_args("{not json}")


def test_fold_safety_flags_only_writes_set_flags() -> None:
    """Unset flags are not written, so absent stays absent for the handler."""
    folded = fold_safety_flags(
        {"instance_id": 1},
        dry_run=False,
        confirm=False,
        mode=None,
        plan_id=None,
        confirmed_dry_run=False,
        yolo=False,
        environment=None,
    )
    assert folded == {"instance_id": 1}


def test_fold_safety_flags_writes_mcp_keys() -> None:
    """Set safety flags land under the exact keys the MCP fields use."""
    folded = fold_safety_flags(
        {"instance_id": 1},
        dry_run=True,
        confirm=True,
        mode="apply",
        plan_id="p-123",
        confirmed_dry_run=True,
        yolo=True,
        environment="staging",
    )
    assert folded == {
        "instance_id": 1,
        "dry_run": True,
        "confirm": True,
        "mode": "apply",
        "plan_id": "p-123",
        "confirmed_dry_run": True,
        "yolo": True,
        "environment": "staging",
    }


def test_fold_does_not_mutate_input() -> None:
    """Folding returns a new dict; the caller's arguments stay untouched."""
    base = {"instance_id": 1}
    fold_safety_flags(
        base,
        dry_run=True,
        confirm=False,
        mode=None,
        plan_id=None,
        confirmed_dry_run=False,
        yolo=False,
        environment=None,
    )
    assert base == {"instance_id": 1}


def test_build_arguments_from_json_then_folds_flags() -> None:
    """build_arguments takes the JSON object and folds the safety flags on top."""
    tool = _tool_with_schema({"label": {"type": "string"}}, [])
    out = build_arguments(
        tool,
        json_args='{"label": "v1"}',
        arg_pairs=[],
        dry_run=True,
        confirm=False,
        mode=None,
        plan_id=None,
        confirmed_dry_run=False,
        yolo=False,
        environment=None,
    )
    assert out == {"label": "v1", "dry_run": True}


def test_build_arguments_from_pairs_coerces_by_schema() -> None:
    """With --arg pairs, build_arguments coerces by the tool's schema types."""
    tool = _tool_with_schema({"size": {"type": "integer"}}, ["size"])
    out = build_arguments(
        tool,
        json_args=None,
        arg_pairs=["size=20"],
        dry_run=False,
        confirm=True,
        mode=None,
        plan_id=None,
        confirmed_dry_run=False,
        yolo=False,
        environment=None,
    )
    assert out == {"size": 20, "confirm": True}


def test_result_text_joins_blocks() -> None:
    """result_text concatenates the text of each content block."""
    assert result_text([_Block("a"), _Block("b")]) == "a\nb"


def test_result_text_skips_textless_blocks() -> None:
    """A block without a string .text is skipped, not rendered as None."""

    class _NoText:
        text = None

    assert result_text([_Block("a"), _NoText()]) == "a"


def test_is_error_result_detects_error_prefix() -> None:
    """A plain error message is flagged as a tool-level error."""
    assert is_error_result("Error: instance_id is required") is True
    assert is_error_result("Failed to retrieve instance: 404") is True


def test_is_error_result_json_is_success() -> None:
    """A JSON payload is a success even if a value contains 'Error'."""
    assert is_error_result('{"message": "Error inside a value"}') is False


def test_is_error_result_empty_is_not_error() -> None:
    """An empty payload is not an error (nothing to report)."""
    assert is_error_result("") is False


def test_render_output_json_is_verbatim() -> None:
    """The json output mode prints the payload exactly as received."""
    payload = '{"a": 1}'
    assert render_output(payload, "json") == payload


def test_render_output_table_object() -> None:
    """A JSON object renders as aligned key/value rows under table output."""
    payload = json.dumps({"id": 1, "label": "web"})
    rendered = render_output(payload, "table")
    assert "id" in rendered
    assert "label" in rendered
    assert "web" in rendered


def test_render_output_table_list_of_objects() -> None:
    """A JSON list of objects renders as a column table with a header."""
    payload = json.dumps([{"id": 1, "label": "a"}, {"id": 2, "label": "b"}])
    rendered = render_output(payload, "table")
    lines = rendered.splitlines()
    assert lines[0].split() == ["id", "label"]
    assert "a" in rendered
    assert "b" in rendered


def test_render_output_table_dry_run_envelope() -> None:
    """The dry-run/plan envelope shape renders as a table without crashing.

    The proto-serialized envelope has nested objects (current_state,
    would_execute) and arrays (dependencies); the table view is a best-effort
    human render, so nested values collapse to a compact-JSON cell and the
    top-level keys still appear as rows.
    """
    payload = json.dumps(
        {
            "dry_run": True,
            "tool": "linode_instance_delete",
            "would_execute": {"method": "DELETE", "path": "/linode/instances/123"},
            "current_state": {"id": 123, "label": "web-01"},
            "dependencies": [{"kind": "volume", "id": 456, "action": "detached"}],
            "side_effects": [],
            "warnings": [],
        }
    )
    rendered = render_output(payload, "table")
    for field in ("dry_run", "would_execute", "current_state", "dependencies"):
        assert field in rendered


def test_render_output_table_non_json_falls_back() -> None:
    """Non-JSON text under table output falls back to the verbatim text."""
    assert render_output("just a message", "table") == "just a message"


def test_render_output_table_empty_list() -> None:
    """An empty list renders a placeholder rather than a blank line."""
    assert render_output("[]", "table") == "(no rows)"


def test_render_output_table_scalar_list() -> None:
    """A list of scalars renders one value per line."""
    rendered = render_output(json.dumps([1, 2, 3]), "table")
    assert rendered.splitlines() == ["1", "2", "3"]
