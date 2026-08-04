"""Tests for the tool declarations the proto contract carries.

Every *Input message declares its capability tier, and names its tool in exactly
one marker: `tool_route` for the 444 tools that reach the Linode API, `tool_meta`
for the 17 that work on local state. That is what lets validate take no
arguments and lets a server check its own registry against the contract.

The manifest at docs/contracts/tools-capabilities.txt is the cross-language
record of the same surface, so the walk here is read against it rather than
against a count: adding a tool moves both or fails.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

from linodemcp.genpb.linode.mcp.v1 import options_pb2
from linodemcp.linode.routes import (
    Declaration,
    RouteError,
    capability_name,
    tools,
    validate_declarations,
    validate_registered,
)

_MANIFEST = (
    Path(__file__).resolve().parents[3]
    / "docs"
    / "contracts"
    / "tools-capabilities.txt"
)

_META_TIER = "Meta"

# The message a broken declaration would sit on. None of these can be generated,
# so every declaration below is built by hand.
_BROKEN = "linode.mcp.v1.BrokenInput"

_VERSION_TOOL = "version"
_INSTANCE_DELETE = "linode_instance_delete"


def _manifest_tiers() -> dict[str, str]:
    """Tool name to tier, from the cross-language capability manifest."""
    tiers: dict[str, str] = {}
    for raw in _MANIFEST.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        tool, _, tier = line.partition("\t")
        tiers[tool] = tier
    assert tiers, "the capability manifest lists no tools"
    return tiers


def _broken(
    *,
    route_tool: str = "",
    meta_tool: str = "",
    capability: int = options_pb2.TOOL_CAPABILITY_UNSPECIFIED,
) -> Declaration:
    """One hand-built declaration on the stand-in message."""
    return Declaration(
        message=_BROKEN,
        route_tool=route_tool,
        meta_tool=meta_tool,
        capability=capability,
    )


def test_tools_covers_the_capability_manifest() -> None:
    """The descriptors declare the surface the manifest lists, both ways."""
    declared = {tool.name for tool in tools()}

    assert declared == set(_manifest_tiers())


def test_tools_agrees_with_the_manifest_on_which_are_meta() -> None:
    """Which tools reach no Linode route is the fact the meta marker carries.

    The manifest says it with a tier and the contract says it with the marker
    that holds the tool name, so the two have to answer alike.
    """
    tiers = _manifest_tiers()

    for tool in tools():
        assert tool.routed == (tiers[tool.name] != _META_TIER), tool.name


def test_tools_reports_the_declared_tier() -> None:
    """Three tools pinned by hand, so a walk that keeps the count right while
    reading the wrong option still fails."""
    found = {tool.name: tool.capability for tool in tools()}

    assert found[_INSTANCE_DELETE] == options_pb2.TOOL_CAPABILITY_DESTROY
    assert found["linode_tag_list"] == options_pb2.TOOL_CAPABILITY_READ
    assert found[_VERSION_TOOL] == options_pb2.TOOL_CAPABILITY_META


def test_every_declared_tool_carries_a_tier() -> None:
    """The check that would pass by measuring nothing if the walk stopped
    reading the option: an unspecified tier is what a message with no
    declaration reads back as, so none may survive into the result."""
    declared = tools()

    assert declared
    for tool in declared:
        assert tool.capability != options_pb2.TOOL_CAPABILITY_UNSPECIFIED, tool.name


def test_capability_name_renders_the_contract_spelling() -> None:
    """Reports name the tier the way the proto spells it."""
    assert capability_name(options_pb2.TOOL_CAPABILITY_META) == "TOOL_CAPABILITY_META"


@pytest.mark.parametrize(
    "declared",
    [
        _broken(
            route_tool=_INSTANCE_DELETE,
            meta_tool=_VERSION_TOOL,
            capability=options_pb2.TOOL_CAPABILITY_DESTROY,
        ),
        _broken(capability=options_pb2.TOOL_CAPABILITY_READ),
        _broken(route_tool=_INSTANCE_DELETE),
        _broken(meta_tool=_VERSION_TOOL),
        _broken(meta_tool=_VERSION_TOOL, capability=options_pb2.TOOL_CAPABILITY_READ),
        _broken(
            route_tool=_INSTANCE_DELETE, capability=options_pb2.TOOL_CAPABILITY_META
        ),
    ],
)
def test_validate_declarations_rejects_broken_declarations(
    declared: Declaration,
) -> None:
    """Each way a message can look annotated while naming no tool, naming two,
    or claiming a tier that contradicts the marker beside it."""
    with pytest.raises(RouteError, match=re.escape(_BROKEN)):
        validate_declarations([declared])


def test_validate_declarations_rejects_two_messages_claiming_one_tool() -> None:
    """Two messages naming one tool leaves no answer to which input the tool
    takes, and whichever a walk reached first would silently win."""
    first = Declaration(
        message="linode.mcp.v1.FirstInput",
        route_tool=_INSTANCE_DELETE,
        meta_tool="",
        capability=options_pb2.TOOL_CAPABILITY_DESTROY,
    )
    second = Declaration(
        message="linode.mcp.v1.SecondInput",
        route_tool=_INSTANCE_DELETE,
        meta_tool="",
        capability=options_pb2.TOOL_CAPABILITY_DESTROY,
    )

    expected = re.escape("which linode.mcp.v1.FirstInput already declares")
    with pytest.raises(RouteError, match=expected):
        validate_declarations([first, second])


def test_validate_declarations_accepts_what_the_contract_holds() -> None:
    """The two shapes the descriptors really carry pass, and a message that
    declares nothing at all is not a defect, since every response type is one."""
    validate_declarations(
        [
            Declaration(
                message="linode.mcp.v1.InstanceDeleteInput",
                route_tool=_INSTANCE_DELETE,
                meta_tool="",
                capability=options_pb2.TOOL_CAPABILITY_DESTROY,
            ),
            Declaration(
                message="linode.mcp.v1.VersionInput",
                route_tool="",
                meta_tool=_VERSION_TOOL,
                capability=options_pb2.TOOL_CAPABILITY_META,
            ),
            _broken(),
        ]
    )


def test_validate_registered_accepts_the_declared_set() -> None:
    """The startup path a real server takes, so a false positive here would
    stop it from starting."""
    validate_registered(tool.name for tool in tools())


def test_validate_registered_reports_a_tool_the_contract_never_named() -> None:
    """A staged tool the contract never declared has no tier to filter it by."""
    staged = [tool.name for tool in tools()] + ["linode_not_a_tool"]

    with pytest.raises(RouteError, match="staged but not declared: linode_not_a_tool"):
        validate_registered(staged)


def test_validate_registered_reports_a_declared_tool_nothing_staged() -> None:
    """A declared tool nothing staged is a handler dropped or renamed without
    the contract moving with it, which the other direction cannot see."""
    staged = [tool.name for tool in tools()][1:]
    dropped = tools()[0].name

    with pytest.raises(RouteError, match=f"declared but not staged: {dropped}"):
        validate_registered(staged)


def test_validate_registered_reports_an_empty_registry() -> None:
    """Staging nothing is not "nothing to check", it is every declared tool
    missing. A check that passed here would pass for a server with no tools."""
    with pytest.raises(RouteError, match="declared but not staged"):
        validate_registered([])
