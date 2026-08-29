"""Phase 8.2 read-only builder tool tests.

Mirrors ``go/internal/tools/linode_profile_builder_test.go``. Tests
define behavior contracts (CapMeta tag, filter semantics, JSON shape,
reproducible ordering) rather than just exercising current code paths.

The handlers read their catalog via a module-level bridge. The
``_install_fixture_catalog`` autouse fixture installs a reproducible
provider and resets the bridge after each test so state never bleeds.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.config import Config
from linodemcp.gentools import (
    create_linode_profile_list_categories_tool,
    create_linode_profile_list_tools_tool,
    handle_linode_profile_list_categories,
    handle_linode_profile_list_tools,
)
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.profiles.profile import Profile
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    BuilderState,
    reset_builder_state,
    set_builder_state,
)

if TYPE_CHECKING:
    from collections.abc import Callable, Iterator

# Argument-key constants kept in sync with the production module. Reused
# across filter tests so the literal-repetition lint doesn't flag.
_ARG_CATEGORY = "category"
_ARG_CAPABILITY = "capability"
_DNS_CATEGORY = "dns"
_MISSING_CATEGORY = "no-such-category"


def fixture_catalog() -> list[ToolDescriptor]:
    """Three-tool fixture matching the Go-side equivalent.

    Covers one compute-write, one dns-read, one core-meta entry so the
    filter assertions exercise both inclusion and exclusion paths.
    """
    return [
        ToolDescriptor(
            name="linode_instance_boot",
            capability=Capability.Write,
            categories=("compute",),
        ),
        ToolDescriptor(
            name="linode_domain_get", capability=Capability.Read, categories=("dns",)
        ),
        ToolDescriptor(name="hello", capability=Capability.Meta, categories=("core",)),
    ]


def _no_profile() -> Profile:
    """The active-profile reader for the tools that never read one."""
    return Profile(name="test", description="", allowed_tools=())


def publish_catalog(catalog: Callable[[], list[ToolDescriptor]]) -> None:
    """Publish a builder state carrying the given catalog for this test."""
    set_builder_state(
        BuilderState(
            drafts=Registry(),
            catalog=catalog,
            active_profile=_no_profile,
            config=Config(),
        )
    )


@pytest.fixture(autouse=True)
def install_fixture_catalog() -> Iterator[None]:
    """Publish the reproducible catalog and reset the state afterward.

    ``yield`` shape per ruff PT021. No leading underscore so pyright doesn't
    flag the auto-applied fixture as an unused private function.
    """
    token = set_builder_state(
        BuilderState(
            drafts=Registry(),
            catalog=fixture_catalog,
            active_profile=_no_profile,
            config=Config(),
        )
    )
    yield
    reset_builder_state(token)


def _parse_envelope_list(payload: str, key: str) -> list[dict[str, object]]:
    """Deserialize a {count, <key>} envelope and return its entry list.

    json.loads returns Any, which pyright (strict) widens to Unknown
    through subsequent operations. The cast collapses that ambiguity
    in one place so call sites can reason about the result statically.
    """
    parsed: object = json.loads(payload)
    assert isinstance(parsed, dict)
    entries = cast("dict[str, object]", parsed)[key]
    assert isinstance(entries, list)
    typed = cast("list[dict[str, object]]", entries)
    assert parsed["count"] == len(typed)
    return typed


async def _call_list_tools(args: dict[str, str]) -> list[dict[str, object]]:
    """Invoke the list_tools handler and return parsed entries.

    Mirrors the Go-side ``callListTools`` helper.
    """
    response = await handle_linode_profile_list_tools(args, Config())
    assert len(response) == 1, "handler must return exactly one TextContent"
    return _parse_envelope_list(response[0].text, "tools")


def test_list_tools_registration() -> None:
    """Static contract: name, description, and CapMeta tag.

    CapMeta is what makes the tool always-available regardless of the
    active profile; a regression on the tag would silently break the
    builder UX under read-only profiles.
    """
    tool, capability = create_linode_profile_list_tools_tool()

    assert tool.name == "linode_profile_list_tools"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_list_tools_returns_all_entries_unfiltered() -> None:
    """No-filter path: every catalog entry appears with name and capability."""
    entries = await _call_list_tools({})

    assert len(entries) == 3
    by_name: dict[str, object] = {str(e["name"]): e["capability"] for e in entries}
    assert by_name["linode_instance_boot"] == "CapWrite"
    assert by_name["linode_domain_get"] == "CapRead"
    assert by_name["hello"] == "CapMeta"


@pytest.mark.asyncio
async def test_list_tools_categories_populated() -> None:
    """Categories field is the resolved categories() output, not empty.

    The model relies on this to drive follow-up ``category=`` filters.
    """
    entries = await _call_list_tools({})

    by_name: dict[str, list[str]] = {}
    for entry in entries:
        name = str(entry["name"])
        cats_raw: object = entry["categories"]
        assert isinstance(cats_raw, list)
        # Cast narrows from list[Unknown] to list[object] in one step;
        # the per-element isinstance check then narrows to str.
        cats_typed = cast("list[object]", cats_raw)
        narrowed: list[str] = []
        for cat_obj in cats_typed:
            assert isinstance(cat_obj, str)
            narrowed.append(cat_obj)
        by_name[name] = narrowed

    assert "compute" in by_name["linode_instance_boot"]
    assert "dns" in by_name["linode_domain_get"]
    assert "core" in by_name["hello"]


@pytest.mark.asyncio
async def test_list_tools_category_filter_matches() -> None:
    """Exact-match category filter narrows output to one entry."""
    entries = await _call_list_tools({_ARG_CATEGORY: _DNS_CATEGORY})

    assert len(entries) == 1
    assert entries[0]["name"] == "linode_domain_get"


@pytest.mark.asyncio
async def test_list_tools_category_filter_rejects_unknown() -> None:
    """Empty-result path: unknown category returns zero entries.

    A non-empty response would silently mask a user typo, which the
    builder UX cannot afford.
    """
    entries = await _call_list_tools({_ARG_CATEGORY: _MISSING_CATEGORY})

    assert entries == []


@pytest.mark.asyncio
async def test_list_tools_capability_filter_long_form() -> None:
    """CapXxx form matches; used by callers round-tripping prior responses."""
    entries = await _call_list_tools({_ARG_CAPABILITY: "CapWrite"})

    assert len(entries) == 1
    assert entries[0]["name"] == "linode_instance_boot"


@pytest.mark.asyncio
async def test_list_tools_capability_filter_short_form() -> None:
    """Short form ('write') matches the same set as the long form."""
    entries = await _call_list_tools({_ARG_CAPABILITY: "write"})

    assert len(entries) == 1
    assert entries[0]["name"] == "linode_instance_boot"


@pytest.mark.asyncio
async def test_list_tools_capability_filter_case_insensitive() -> None:
    """Case folding applies to both short and long forms.

    Spelled-as-typed input ('WRITE', 'Read') must work, otherwise the
    model has to remember exact casing.
    """
    upper = await _call_list_tools({_ARG_CAPABILITY: "WRITE"})
    assert len(upper) == 1
    assert upper[0]["name"] == "linode_instance_boot"

    mixed = await _call_list_tools({_ARG_CAPABILITY: "Read"})
    assert len(mixed) == 1
    assert mixed[0]["name"] == "linode_domain_get"


@pytest.mark.asyncio
async def test_list_tools_combined_filters() -> None:
    """AND semantics: a tool must match both filters to appear."""
    match_entries = await _call_list_tools(
        {_ARG_CATEGORY: _DNS_CATEGORY, _ARG_CAPABILITY: "read"}
    )
    assert len(match_entries) == 1
    assert match_entries[0]["name"] == "linode_domain_get"

    miss_entries = await _call_list_tools(
        {_ARG_CATEGORY: _DNS_CATEGORY, _ARG_CAPABILITY: "write"}
    )
    assert miss_entries == []


@pytest.mark.asyncio
async def test_list_tools_empty_catalog_returns_empty_array() -> None:
    """JSON shape on the empty path: ``[]`` not ``null``.

    The model would handle null as 'tool failed'; an empty array is
    the correct 'no tools matched'.
    """
    publish_catalog(list)

    response = await handle_linode_profile_list_tools({}, Config())

    assert _parse_envelope_list(response[0].text, "tools") == []


@pytest.mark.asyncio
async def test_list_tools_without_state_refuses() -> None:
    """A handler reached with no state refuses rather than answering empty.

    An empty catalog and an unwired server read the same to a caller, so the
    handler says which one it is. Go answers the same sentence.
    """
    token = set_builder_state(None)
    try:
        response = await handle_linode_profile_list_tools({}, Config())
    finally:
        reset_builder_state(token)

    assert response[0].text == f"Error: {BUILDER_UNCONFIGURED}"


def test_list_categories_registration() -> None:
    """Categories tool: name, description, CapMeta tag."""
    tool, capability = create_linode_profile_list_categories_tool()

    assert tool.name == "linode_profile_list_categories"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_list_categories_returns_deduplicated_counts() -> None:
    """Substantive behavior: every category appears with the right count.

    A regression where duplicates leak through (e.g. a typo in the
    accumulator key) would surface here. Python's
    ``linode_instance_boot`` carries just ``compute`` (no
    ``compute_actions`` category on the Python side), so the count
    expectations differ from the Go-side test by design.
    """
    response = await handle_linode_profile_list_categories({}, Config())
    parsed = _parse_envelope_list(response[0].text, "categories")
    counts = {str(e["name"]): e["tool_count"] for e in parsed}

    assert counts["compute"] == 1
    assert counts["dns"] == 1
    assert counts["core"] == 1
    # Python does not split compute_actions out of compute; confirming
    # the absence guards against an accidental import of the Go-side
    # taxonomy without aligning the Python catalog first.
    assert "compute_actions" not in counts


@pytest.mark.asyncio
async def test_list_tools_sorted_by_name() -> None:
    """Pin the answer's order.

    The catalog fixture is deliberately NOT in name order, so a dropped sort
    fails here rather than showing up as two languages listing one menu two
    ways: Go registers its hand-written tools first and Python registers in
    name order, which is the divergence this sort closes.
    """
    entries = await _call_list_tools({})

    names = [str(entry["name"]) for entry in entries]

    assert len(names) == len(fixture_catalog())
    assert names == sorted(names)


@pytest.mark.asyncio
async def test_list_categories_sorted_by_name() -> None:
    """Stable output: sorted ascending by name.

    A refactor that drops the sort would cause flaky cross-language
    comparison.
    """
    response = await handle_linode_profile_list_categories({}, Config())
    parsed = _parse_envelope_list(response[0].text, "categories")
    names = [str(e["name"]) for e in parsed]

    assert names == sorted(names)


@pytest.mark.asyncio
async def test_list_categories_empty_catalog_returns_empty_array() -> None:
    """Empty catalog serializes as ``[]`` not ``null``."""
    publish_catalog(list)

    response = await handle_linode_profile_list_categories({}, Config())

    assert _parse_envelope_list(response[0].text, "categories") == []
