"""Unit tests for the TUI model logic (no terminal involved).

These cover the pure pieces the screens delegate to: catalog building and
filtering, form-field construction from a schema, and the form-to-arguments
mapping. The mapping test is the contract that matters most: a filled form must
produce the same arguments dict the Phase 1 ``call`` builds from the same
values, since both front-ends drive the same dispatch.
"""

from __future__ import annotations

import pytest
from mcp.types import Tool

from linodemcp.cli._shared import build_arguments
from linodemcp.profiles import Capability
from linodemcp.tui import model


def _tool(properties: dict[str, object], required: list[str]) -> Tool:
    """Build a throwaway Tool with the given input schema."""
    return Tool(
        name="linode_test_tool",
        description="test tool",
        inputSchema={
            "type": "object",
            "properties": properties,
            "required": required,
        },
    )


def test_build_catalog_full_includes_registry() -> None:
    """With ``allowed=None`` the catalog covers the whole registry."""
    from linodemcp.server import get_tool_registry

    entries = model.build_catalog(allowed=None)
    assert len(entries) == len(get_tool_registry())
    names = {e.name for e in entries}
    assert "version" in names
    assert "linode_instance_list" in names


def test_build_catalog_filtered_is_subset() -> None:
    """A profile allow-set yields only those tools, a strict subset of all."""
    allowed = frozenset({"version", "hello", "linode_instance_list"})
    entries = model.build_catalog(allowed=allowed)
    assert {e.name for e in entries} == allowed


def test_build_catalog_sorted_by_category_then_name() -> None:
    """Entries come out ordered by (category, name) for a stable grouped view."""
    entries = model.build_catalog(allowed=None)
    keys = [(e.category, e.name) for e in entries]
    assert keys == sorted(keys)


def test_catalog_entry_capability_label_and_destructive() -> None:
    """The capability tag is lowercased; Destroy entries flag as destructive."""
    entry = model.CatalogEntry(
        name="linode_instance_delete",
        capability=Capability.Destroy,
        category="compute",
    )
    assert entry.capability_label == "destroy"
    assert entry.is_destructive is True

    read = model.CatalogEntry(
        name="linode_instance_list", capability=Capability.Read, category="compute"
    )
    assert read.is_destructive is False


def test_filter_catalog_by_name() -> None:
    """Filtering matches a substring of the tool name."""
    entries = model.build_catalog(allowed=None)
    filtered = model.filter_catalog(entries, "instance_list")
    assert filtered
    assert all("instance_list" in e.name for e in filtered)


def test_filter_catalog_by_category() -> None:
    """Filtering also matches a substring of the category."""
    entries = model.build_catalog(allowed=None)
    filtered = model.filter_catalog(entries, "networking")
    assert filtered
    assert all(e.category == "networking" for e in filtered)


def test_filter_catalog_empty_query_returns_all() -> None:
    """A blank query restores the full list."""
    entries = model.build_catalog(allowed=None)
    assert len(model.filter_catalog(entries, "   ")) == len(entries)


def test_group_by_category() -> None:
    """Grouping returns category-ordered buckets, each name-ordered inside."""
    entries = model.build_catalog(allowed=None)
    grouped = model.group_by_category(entries)
    categories = [cat for cat, _ in grouped]
    assert categories == sorted(categories)
    for _, bucket in grouped:
        names = [e.name for e in bucket]
        assert names == sorted(names)


def test_lookup_tool_found_and_missing() -> None:
    """lookup_tool returns the Tool for a real name and None otherwise."""
    assert model.lookup_tool("version") is not None
    assert model.lookup_tool("linode_not_real") is None


def test_build_form_fields_marks_required_and_skips_safety() -> None:
    """Fields come from schema properties; required ones flag, safety keys drop.

    A schema that includes a safety key (``dry_run``) must not render it as a
    free-text field; the safety controls own it.
    """
    tool = _tool(
        {
            "instance_id": {"type": "integer", "description": "the id"},
            "label": {"type": "string"},
            "dry_run": {"type": "boolean"},
        },
        ["instance_id"],
    )
    fields = model.build_form_fields(tool)
    names = [f.name for f in fields]
    assert "dry_run" not in names
    assert names[0] == "instance_id"  # required sorts first
    by_name = {f.name: f for f in fields}
    assert by_name["instance_id"].required is True
    assert by_name["instance_id"].json_type == "integer"
    assert by_name["label"].required is False


def test_form_field_label() -> None:
    """The field label shows name, type, and a ``*`` for required fields."""
    required = model.FormField(
        name="instance_id", json_type="integer", required=True, description=""
    )
    optional = model.FormField(
        name="label", json_type="string", required=False, description=""
    )
    assert required.label == "instance_id (integer) *"
    assert optional.label == "label (string)"


def test_form_to_arguments_matches_cli_call() -> None:
    """The core contract: a filled form maps to the same args the CLI builds.

    The form values and safety controls are mapped through the model; the same
    values fed to the Phase 1 ``build_arguments`` (the ``call`` path) must
    produce an identical dict, proving the two front-ends cannot diverge.
    """
    tool = _tool(
        {
            "instance_id": {"type": "integer"},
            "label": {"type": "string"},
        },
        ["instance_id"],
    )
    state = model.FormState(
        tool_name="linode_test_tool",
        fields=[
            model.FormField(
                name="instance_id", json_type="integer", required=True, description=""
            ),
            model.FormField(
                name="label", json_type="string", required=False, description=""
            ),
        ],
        safety=model.SafetyControls(
            dry_run=True,
            confirm=True,
            mode="apply",
            confirmed_dry_run=True,
            yolo=True,
            environment="staging",
        ),
    )
    state.fields[0].value = "123"
    state.fields[1].value = "web"

    from_form = model.form_to_arguments(tool, state)

    # The equivalent CLI invocation: --arg instance_id=123 --arg label=web with
    # the same safety flags.
    from_cli = build_arguments(
        tool,
        json_args=None,
        arg_pairs=["instance_id=123", "label=web"],
        dry_run=True,
        confirm=True,
        mode="apply",
        plan_id=None,
        confirmed_dry_run=True,
        yolo=True,
        environment="staging",
    )

    assert from_form == from_cli
    assert from_form["instance_id"] == 123  # coerced to int by the schema
    assert from_form["label"] == "web"
    assert from_form["mode"] == "apply"


def test_form_to_arguments_skips_empty_optional_fields() -> None:
    """An empty optional field is omitted, not sent as an empty string."""
    tool = _tool({"label": {"type": "string"}}, [])
    state = model.FormState(
        tool_name="linode_test_tool",
        fields=[
            model.FormField(
                name="label", json_type="string", required=False, description=""
            )
        ],
        safety=model.SafetyControls(),
    )
    # value left empty
    args = model.form_to_arguments(tool, state)
    assert args == {}


def test_form_to_arguments_missing_required_raises() -> None:
    """A required field left empty raises before any dispatch."""
    tool = _tool({"instance_id": {"type": "integer"}}, ["instance_id"])
    state = model.FormState(
        tool_name="linode_test_tool",
        fields=[
            model.FormField(
                name="instance_id", json_type="integer", required=True, description=""
            )
        ],
        safety=model.SafetyControls(),
    )
    with pytest.raises(model.FormValidationError, match="instance_id"):
        model.form_to_arguments(tool, state)


def test_form_to_arguments_mode_none_omits_mode() -> None:
    """``mode='none'`` is not sent as an MCP field (a normal call)."""
    tool = _tool({"label": {"type": "string"}}, [])
    state = model.FormState(
        tool_name="linode_test_tool",
        fields=[],
        safety=model.SafetyControls(mode="none"),
    )
    args = model.form_to_arguments(tool, state)
    assert "mode" not in args


def test_configured_environments_prepends_blank() -> None:
    """The environment picker offers a blank default ahead of the names."""
    assert model.configured_environments(["default", "staging"]) == [
        "",
        "default",
        "staging",
    ]
    assert model.configured_environments([]) == [""]
