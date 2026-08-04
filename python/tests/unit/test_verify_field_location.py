"""Offline tests for the field-location gate.

verify_field_location.py pins where every routed input field goes in the HTTP
request its tool makes. These cover each violation class, and one test reads the
real descriptors so the checked-in surface has to stay fully declared.

One test here guards the parser rather than the gate. Adding the
`[(linode.mcp.v1.field_location) = ...]` block to every routed field broke
verify_system_params.py, whose field pattern stopped at the tag number: it went
from seeing 970 markers to seeing 1 and still printed OK. A gate that passes by
matching nothing is the failure this file exists to keep from recurring.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_field_location")
reader = _load_script("_toolroutes")
sysparams = _load_script("verify_system_params")

_PATH = "FIELD_LOCATION_PATH"
_QUERY = "FIELD_LOCATION_QUERY"
_LOCAL = "FIELD_LOCATION_LOCAL"
_UNSET = "FIELD_LOCATION_UNSPECIFIED"

_MESSAGE = "linode.mcp.v1.TagGetInput"


def _route(path: str = "/tags/{label}") -> object:
    return reader.ToolRoute(_MESSAGE, "linode_tag_get", "GET", path)


def _marked(*names: str) -> object:
    """Stand in for verify_system_params.all_fields with a fixed marker set."""
    fields = [
        sysparams.Field(
            path="tag.proto",
            line=index,
            message="TagGetInput",
            name=name,
            type="string",
            marked=True,
        )
        for index, name in enumerate(names, start=1)
    ]
    return lambda: fields


def test_a_fully_located_surface_reports_nothing() -> None:
    """Every field declared, the slot bound, and LOCAL matching the marker."""
    located = {_MESSAGE: {"label": _PATH, "environment": _LOCAL}}

    assert gate.unannotated(located) == []
    assert gate.path_mismatches([_route()], located) == []


def test_an_unannotated_field_is_named() -> None:
    """A field with no location would be dropped from the request silently."""
    located = {_MESSAGE: {"label": _PATH, "environment": _UNSET}}

    assert gate.unannotated(located) == ["TagGetInput.environment"]


def test_a_path_slot_no_field_declares_fails() -> None:
    """A template naming a parameter nothing declares cannot be filled."""
    located = {_MESSAGE: {"label": _QUERY}}

    assert gate.path_mismatches([_route()], located) == [
        "TagGetInput: path names {label}, which no field declares as PATH"
    ]


def test_a_path_field_with_no_slot_fails() -> None:
    """A PATH field the template never mentions would never be consumed."""
    located = {_MESSAGE: {"label": _PATH, "region": _PATH}}

    assert gate.path_mismatches([_route()], located) == [
        "TagGetInput.region: declared PATH, but '/tags/{label}' has no such slot"
    ]


def test_a_local_field_without_the_marker_fails(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """LOCAL and the `// system param` marker are two records of one fact."""
    monkeypatch.setattr(gate.verify_system_params, "all_fields", _marked())
    located = {_MESSAGE: {"environment": _LOCAL}}

    assert gate.local_mismatches(located) == [
        "TagGetInput.environment: LOCAL but carries no `// system param`"
    ]


def test_a_marked_field_that_is_not_local_fails(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """This is the direction that puts dry_run on the wire, so it must fail."""
    monkeypatch.setattr(gate.verify_system_params, "all_fields", _marked("dry_run"))
    located = {_MESSAGE: {"dry_run": _QUERY}}

    assert gate.local_mismatches(located) == [
        "TagGetInput.dry_run: `// system param` but declared FIELD_LOCATION_QUERY"
    ]


def test_a_marked_local_field_agrees(monkeypatch: pytest.MonkeyPatch) -> None:
    """Marker and annotation on the same field is the correct state."""
    monkeypatch.setattr(gate.verify_system_params, "all_fields", _marked("dry_run"))

    assert gate.local_mismatches({_MESSAGE: {"dry_run": _LOCAL}}) == []


def test_the_system_param_parser_reads_annotated_field_lines() -> None:
    """A field-options block sits where the parser used to expect the semicolon.

    Without this the system-params gate matches no routed field at all and
    reports success over an empty set.
    """
    source = (
        "message TagGetInput {\n"
        "  optional string environment = 1"
        " [(linode.mcp.v1.field_location) = FIELD_LOCATION_LOCAL];"
        " // system param\n"
        "  string label = 2"
        " [(linode.mcp.v1.field_location) = FIELD_LOCATION_PATH];\n"
        "}\n"
    )

    parsed = sysparams.parse_fields(source, "tag.proto")

    assert [(field.name, field.marked) for field in parsed] == [
        ("environment", True),
        ("label", False),
    ]


def test_the_checked_in_surface_declares_every_location() -> None:
    """The real descriptors carry a location on every routed input field."""
    located = reader.field_locations()

    assert gate.unannotated(located) == []
    assert gate.path_mismatches(reader.tool_routes(), located) == []
    assert gate.local_mismatches(located) == []
