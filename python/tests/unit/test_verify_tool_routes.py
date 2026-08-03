"""Offline tests for the tool-route annotation gate and its shared reader.

verify_tool_routes.py pins the `tool_route` options the proto contract carries
against docs/contracts/tools-manifest.txt, from both sides. These tests cover
each violation class plus the reader's own refusals, and one test reads the
real descriptors so the checked-in surface has to stay fully annotated.

The reader's norm_template is covered here too. It is the boundary every route
comparison crosses: declared paths name their parameters, resolved ones cannot,
and the gates match on shape.
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


gate = _load_script("verify_tool_routes")
reader = _load_script("_toolroutes")
routescan = _load_script("_routescan")


def _route(message: str, tool: str, method: str = "GET", path: str = "/tags") -> object:
    return reader.ToolRoute(f"linode.mcp.v1.{message}", tool, method, path)


_CAPABILITIES = {"linode_tag_list": "Read", "hello": "Meta"}
_MANIFEST = {"linode_tag_list", "hello"}
_MESSAGES = {"linode_tag_list": "TagListInput", "hello": "HelloInput"}


def test_a_fully_annotated_surface_reports_nothing() -> None:
    """A routed non-meta tool and an unrouted Meta tool are both correct."""
    declared = [_route("TagListInput", "linode_tag_list")]

    assert gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    ) == ([], [], [])


def test_an_unannotated_tool_is_named_with_its_message() -> None:
    """The finding points at the message that needs the option."""
    missing, unwanted, malformed = gate.annotation_violations(
        [], _CAPABILITIES, _MANIFEST, _MESSAGES
    )

    assert missing == ["linode_tag_list (TagListInput): no tool_route option"]
    assert (unwanted, malformed) == ([], [])


def test_a_meta_tool_may_not_declare_a_route() -> None:
    """Meta tools reach no Linode route, so an option on one is a defect."""
    declared = [
        _route("TagListInput", "linode_tag_list"),
        _route("HelloInput", "hello"),
    ]

    _, unwanted, _ = gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    )

    assert unwanted == [
        "hello (linode.mcp.v1.HelloInput): Meta tools reach no Linode route"
    ]


def test_an_option_naming_an_unregistered_tool_fails() -> None:
    """A typo in the tool name cannot pass as a route for a real tool."""
    declared = [
        _route("TagListInput", "linode_tag_list"),
        _route("TagsListInput", "linode_tags_list"),
    ]

    _, _, malformed = gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    )

    assert malformed == [
        "linode.mcp.v1.TagsListInput: names unregistered tool linode_tags_list"
    ]


def test_an_option_on_the_wrong_message_fails() -> None:
    """The round-trip is blind to placement, so the gate is not.

    Regenerating the old text contract from the descriptors produces the same
    bytes whichever message an option sits on, since the option carries the
    tool name. Only comparing against what the tool actually registers catches
    an annotation that landed one message over.
    """
    declared = [_route("HelloInput", "linode_tag_list")]

    _, _, malformed = gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    )

    assert malformed == [
        (
            "linode.mcp.v1.HelloInput: annotates linode_tag_list,"
            " which registers TagListInput"
        )
    ]


@pytest.mark.parametrize(
    ("method", "path", "fragment"),
    [
        ("FETCH", "/tags", "method 'FETCH' is not a verb"),
        ("GET", "tags", "path 'tags' is not a template"),
        ("GET", "/tags?page=2", "path '/tags?page=2' is not a template"),
        ("GET", "/tags/{}", "path '/tags/{}' is not a template"),
        (
            "GET",
            "/tags/{tagLabel}",
            "path '/tags/{tagLabel}' is not a template",
        ),
        (
            "GET",
            "/a/{thing_id}/b/{thing_id}",
            "path '/a/{thing_id}/b/{thing_id}' names thing_id more than once",
        ),
    ],
)
def test_a_malformed_route_fails(method: str, path: str, fragment: str) -> None:
    """Method must be a verb, the path rooted, its parameters named once each."""
    declared = [_route("TagListInput", "linode_tag_list", method, path)]

    _, _, malformed = gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    )

    assert malformed == [f"linode.mcp.v1.TagListInput: {fragment}"]


@pytest.mark.parametrize(
    "path",
    [
        "/tags",
        "/tags/{tag_label}",
        "/a/{first_id}/b/{second_id}",
        "/object-storage/buckets/{region_id}/{bucket}",
    ],
)
def test_a_named_parameter_template_is_accepted(path: str) -> None:
    """snake_case names are the convention, so a named template is well formed."""
    declared = [_route("TagListInput", "linode_tag_list", "GET", path)]

    assert gate.annotation_violations(
        declared, _CAPABILITIES, _MANIFEST, _MESSAGES
    ) == ([], [], [])


def test_every_declared_parameter_carries_a_real_name() -> None:
    """No route kept the old placeholder when the names went in.

    A leftover {p} passes the template check, since p is snake_case, and would
    read as a named parameter to whoever binds one next.
    """
    unnamed = [
        f"{entry.message}: {entry.path}"
        for entry in reader.tool_routes()
        if "{p}" in entry.path
    ]

    assert unnamed == []


@pytest.mark.parametrize(
    ("path", "shape"),
    [
        ("/tags", "/tags"),
        ("/tags/{tag_label}", "/tags/{p}"),
        ("/a/{first_id}/b/{second_id}", "/a/{p}/b/{p}"),
        ("/tags/{tagLabel}", "/tags/{p}"),
        ("/profile/tokens?page=2", "/profile/tokens"),
        ("/tags/", "/tags"),
    ],
)
def test_norm_template_reduces_a_path_to_its_shape(path: str, shape: str) -> None:
    """Any parameter name collapses, whatever convention wrote it.

    The spec, the handlers, and the contract each name the same parameter
    differently, so the normalizer has to erase all three the same way.
    """
    assert reader.norm_template(path) == shape


def test_norm_template_emits_the_placeholder_the_scanners_emit() -> None:
    """The two sides of every route comparison share one placeholder.

    A second copy of the literal here would let the normalizer and the scanners
    drift, and the failure would read as every route losing its evidence.
    """
    assert reader.norm_template("/a/{b}") == f"/a/{routescan.PATH_PARAM}"


def test_read_manifest_rejects_an_empty_file(tmp_path: Path) -> None:
    """An empty manifest would let every option through unchecked."""
    path = tmp_path / "tools-manifest.txt"
    path.write_text("# only a header\n", encoding="utf-8")

    with pytest.raises(SystemExit):
        gate.read_manifest(path)


def test_reader_rejects_two_messages_claiming_one_tool(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """One tool calls one route, so a second claim aborts instead of winning."""
    monkeypatch.setattr(
        reader,
        "tool_routes",
        lambda: [
            _route("TagListInput", "linode_tag_list"),
            _route("TagsListInput", "linode_tag_list"),
        ],
    )

    with pytest.raises(SystemExit):
        reader.routes()


def test_reader_rejects_an_empty_descriptor_set(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """No options at all means ungenerated code, not a surface with no routes.

    Returning an empty map would make every consumer report the whole surface
    as missing, which reads like a contract break instead of a stale tree.
    """
    monkeypatch.setattr(reader, "_read_descriptors", list)

    with pytest.raises(SystemExit):
        reader.tool_routes()


def test_the_checked_in_surface_is_fully_annotated() -> None:
    """The real descriptors satisfy the gate, read through the real reader."""
    declared = reader.tool_routes()
    assert len(declared) > 400

    assert gate.annotation_violations(
        declared,
        gate._surface.read_capabilities(),
        gate.read_manifest(),
        gate._surface.tool_input_messages(),
    ) == ([], [], [])
