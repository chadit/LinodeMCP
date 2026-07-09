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

from linodemcp.profiles import Capability
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.linode_profile_builder import (
    create_linode_profile_list_categories_tool,
    create_linode_profile_list_tools_tool,
    handle_linode_profile_list_categories,
    handle_linode_profile_list_tools,
    set_tool_catalog_provider,
)

if TYPE_CHECKING:
    from collections.abc import Iterator

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
        ToolDescriptor(name="linode_instance_boot", capability=Capability.Write),
        ToolDescriptor(name="linode_domain_get", capability=Capability.Read),
        ToolDescriptor(name="hello", capability=Capability.Meta),
    ]


@pytest.fixture(autouse=True)
def install_fixture_catalog() -> Iterator[None]:
    """Install the reproducible catalog and reset the bridge afterward.

    The module-level bridge state would otherwise bleed across tests
    (and across files, since the production server module touches the
    same singleton). ``yield`` shape per ruff PT021. No leading
    underscore so pyright doesn't flag the auto-applied fixture as an
    unused private function.
    """
    set_tool_catalog_provider(fixture_catalog)
    yield
    set_tool_catalog_provider(None)


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
    response = await handle_linode_profile_list_tools(args)
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
    set_tool_catalog_provider(list)

    response = await handle_linode_profile_list_tools({})

    assert _parse_envelope_list(response[0].text, "tools") == []


@pytest.mark.asyncio
async def test_list_tools_no_bridge_returns_empty() -> None:
    """When no bridge is installed the handler returns an empty list.

    Documented contract: the handler doesn't raise if the server hasn't
    wired the catalog provider yet, which simplifies test setup and
    ensures bare imports of the module don't break.
    """
    set_tool_catalog_provider(None)

    response = await handle_linode_profile_list_tools({})

    assert _parse_envelope_list(response[0].text, "tools") == []


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
    response = await handle_linode_profile_list_categories({})
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
async def test_list_categories_sorted_by_name() -> None:
    """Stable output: sorted ascending by name.

    A refactor that drops the sort would cause flaky cross-language
    comparison.
    """
    response = await handle_linode_profile_list_categories({})
    parsed = _parse_envelope_list(response[0].text, "categories")
    names = [str(e["name"]) for e in parsed]

    assert names == sorted(names)


@pytest.mark.asyncio
async def test_list_categories_empty_catalog_returns_empty_array() -> None:
    """Empty catalog serializes as ``[]`` not ``null``."""
    set_tool_catalog_provider(list)

    response = await handle_linode_profile_list_categories({})

    assert _parse_envelope_list(response[0].text, "categories") == []
