"""Offline tests for the tool-capability gate.

verify_tool_capability.py holds docs/contracts/tools-capabilities.txt to being a
mirror of what the proto declares, from both sides. These tests cover each
violation class, the two ways the gate could pass while measuring nothing, and
one that reads the real descriptors so the checked-in surface has to stay fully
declared.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from types import ModuleType

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


gate = _load_script("verify_tool_capability")
reader = _load_script("_toolroutes")

_ROUTED_TOOL = "linode_tag_list"
_META_TOOL = "version"
_MESSAGE = "linode.mcp.v1.TagListInput"

_READ = "TOOL_CAPABILITY_READ"
_META = "TOOL_CAPABILITY_META"
_UNSPECIFIED = "TOOL_CAPABILITY_UNSPECIFIED"


def _declaration(
    *,
    message: str = _MESSAGE,
    route_tool: str = "",
    meta_tool: str = "",
    capability: str = _UNSPECIFIED,
) -> object:
    """One declaration as the reader hands it over."""
    return reader.ToolDeclaration(
        message=message,
        route_tool=route_tool,
        meta_tool=meta_tool,
        capability=capability,
    )


def test_shipped_contract_mirrors_the_manifest() -> None:
    """The gate passes on the real descriptors, which is what it exists to hold.

    A failure here means the proto and docs/contracts/tools-capabilities.txt
    have drifted, not that the test is wrong.
    """
    assert gate.main() == 0


def test_every_capability_value_maps_to_a_tier() -> None:
    """A proto tier with no manifest spelling would drop out of the comparison
    silently, so it has to be a failure rather than a skipped entry."""
    assert gate.unmapped_values(reader.capability_values()) == []


def test_unmapped_value_is_reported() -> None:
    """The other half: a value the table does not translate is named."""
    assert gate.unmapped_values([_READ, "TOOL_CAPABILITY_INVENTED"]) == [
        "TOOL_CAPABILITY_INVENTED"
    ]


def test_unspecified_is_not_treated_as_an_unmapped_tier() -> None:
    """The enum's zero value is how "declared nothing" reads back, and the
    marker checks report that; it is not a tier the manifest should spell."""
    assert gate.unmapped_values([_UNSPECIFIED]) == []


def test_both_markers_on_one_message_is_reported() -> None:
    """A message naming its tool twice leaves a consumer choosing a reading."""
    found = gate.marker_violations(
        [_declaration(route_tool=_ROUTED_TOOL, meta_tool=_META_TOOL, capability=_READ)]
    )

    expected = (
        f"TagListInput: names {_ROUTED_TOOL} as a routed tool"
        f" and {_META_TOOL} as a meta tool"
    )

    assert found == [expected]


def test_tier_with_no_marker_is_reported() -> None:
    """A tier on a message that names no tool declares nothing usable."""
    found = gate.marker_violations([_declaration(capability=_READ)])

    assert found == ["TagListInput: declares Read but no marker names its tool"]


def test_meta_marker_at_a_routed_tier_is_reported() -> None:
    """Meta means "reaches no Linode route", so the marker and the tier are two
    spellings of one fact and may not disagree."""
    found = gate.marker_violations(
        [_declaration(meta_tool=_META_TOOL, capability=_READ)]
    )

    assert found == [f"TagListInput: {_META_TOOL} is Read, not a meta tool"]


def test_routed_marker_at_the_meta_tier_is_reported() -> None:
    """The same disagreement seen from the route side."""
    found = gate.marker_violations(
        [_declaration(route_tool=_ROUTED_TOOL, capability=_META)]
    )

    assert found == [f"TagListInput: {_ROUTED_TOOL} is Meta, so it carries no route"]


def test_marker_without_a_tier_is_reported() -> None:
    """A tool named but untiered would register under no capability at all."""
    found = gate.marker_violations([_declaration(route_tool=_ROUTED_TOOL)])

    assert found == ["TagListInput: names a tool but declares no capability"]


def test_well_formed_declarations_report_nothing() -> None:
    """Both shapes the descriptors really carry pass the marker checks."""
    found = gate.marker_violations(
        [
            _declaration(route_tool=_ROUTED_TOOL, capability=_READ),
            _declaration(
                message="linode.mcp.v1.VersionInput",
                meta_tool=_META_TOOL,
                capability=_META,
            ),
        ]
    )

    assert found == []


def test_declared_tiers_skips_a_contradictory_message() -> None:
    """A message naming two tools is reported by the marker checks; folding it
    into the tier map would report the same defect again under a name it may
    not own."""
    tiers = gate.declared_tiers(
        [_declaration(route_tool=_ROUTED_TOOL, meta_tool=_META_TOOL, capability=_READ)]
    )

    assert tiers == {}


def test_tool_declared_but_not_listed_is_reported() -> None:
    """A tool the proto declares and the manifest never listed."""
    unlisted, undeclared, retiered = gate.manifest_violations(
        {_ROUTED_TOOL: "Read"}, {}
    )

    assert unlisted == [f"{_ROUTED_TOOL} (Read)"]
    assert undeclared == []
    assert retiered == []


def test_tool_listed_but_not_declared_is_reported() -> None:
    """The other direction: a listed tool no message declares."""
    unlisted, undeclared, retiered = gate.manifest_violations(
        {}, {_ROUTED_TOOL: "Read"}
    )

    assert unlisted == []
    assert undeclared == [f"{_ROUTED_TOOL} (Read)"]
    assert retiered == []


def test_tier_disagreement_is_reported() -> None:
    """Retiering a tool in one place and not the other is the drift the gate
    was added for: the same tool would land in a different profile per
    language."""
    unlisted, undeclared, retiered = gate.manifest_violations(
        {_ROUTED_TOOL: "Read"}, {_ROUTED_TOOL: "Admin"}
    )

    assert unlisted == []
    assert undeclared == []
    assert retiered == [f"{_ROUTED_TOOL}: proto says Read, manifest says Admin"]


def test_agreeing_sides_report_nothing() -> None:
    """The passing case, so the checks above cannot be passing by accident."""
    assert gate.manifest_violations({_ROUTED_TOOL: "Read"}, {_ROUTED_TOOL: "Read"}) == (
        [],
        [],
        [],
    )


def test_reader_refuses_an_empty_declaration_set(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The guard that measures nothing. With no declarations every comparison
    is empty and the gate would print OK for a contract that lost its options,
    so the reader fails instead of handing back nothing."""
    monkeypatch.setattr(reader, "_read_declarations", list)

    with pytest.raises(SystemExit, match="no tool declarations"):
        reader.declarations()


def test_reader_refuses_an_empty_capability_enum(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The same guard on the enum: no values means no tier could be unmapped,
    which would make the first check vacuous."""
    monkeypatch.setattr(reader, "_read_capability_values", list)

    with pytest.raises(SystemExit, match="defines no values"):
        reader.capability_values()
