"""Unit tests for the linode_profile_can_run pre-check tool."""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

from linodemcp.config import Config
from linodemcp.gentools import (
    create_linode_profile_can_run_tool,
    handle_linode_profile_can_run,
)
from linodemcp.profiles import Capability, Profile
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.builderstate import (
    BuilderState,
    reset_builder_state,
    set_builder_state,
)

if TYPE_CHECKING:
    from collections.abc import Iterator
    from typing import Any

_READ_TOOL = "linode_instance_list"
_WRITE_TOOL = "linode_instance_create"
_DESTROY_TOOL = "linode_instance_delete"
_UNKNOWN_TOOL = "linode_not_a_real_tool"


def _fixture_catalog() -> list[ToolDescriptor]:
    return [
        ToolDescriptor(name=_READ_TOOL, capability=Capability.Read),
        ToolDescriptor(name=_WRITE_TOOL, capability=Capability.Write),
        ToolDescriptor(name=_DESTROY_TOOL, capability=Capability.Destroy),
    ]


def _fixture_profile(environments: tuple[str, ...] = ("prod",)) -> Profile:
    return Profile(
        name="compute-readonly",
        description="read-only fixture",
        allowed_tools=(_READ_TOOL,),
        allowed_environments=environments,
    )


@pytest.fixture
def wired(request: pytest.FixtureRequest) -> Iterator[None]:
    """Publish the fixture state; reset it on teardown to avoid state bleed.

    The optional indirect param overrides the profile's allowed_environments.
    """
    environments: tuple[str, ...] = getattr(request, "param", ("prod",))
    token = set_builder_state(
        BuilderState(
            drafts=Registry(),
            catalog=_fixture_catalog,
            active_profile=lambda: _fixture_profile(environments),
            config=Config(),
        )
    )
    yield
    reset_builder_state(token)


async def _run(calls: list[dict[str, Any]]) -> dict[str, Any]:
    result = await handle_linode_profile_can_run({"calls": calls}, Config())
    parsed: dict[str, Any] = json.loads(result[0].text)
    return parsed


def test_schema_and_capability() -> None:
    tool, capability = create_linode_profile_can_run_tool()
    assert tool.name == "linode_profile_can_run"
    assert capability == Capability.Meta
    assert "calls" in tool.input_schema["properties"]


async def test_classifies_every_category_and_allow_path(wired: None) -> None:
    assert wired is None
    body = await _run(
        [
            {"tool": _READ_TOOL},
            {"tool": _READ_TOOL, "args": {"environment": "dev"}},
            {"tool": _WRITE_TOOL},
            {"tool": _DESTROY_TOOL},
            {"tool": _UNKNOWN_TOOL},
        ]
    )

    assert body["active_profile"] == "compute-readonly"
    results = body["results"]
    assert len(results) == 5

    assert results[0]["allowed"] is True

    assert results[1]["allowed"] is False
    assert results[1]["reason"] == "environment not permitted by profile"

    assert results[2]["allowed"] is False
    assert results[2]["reason"] == "tool not in profile's allowed_tools"

    assert results[3]["allowed"] is False
    assert "(CapDestroy)" in results[3]["reason"]

    assert results[4]["allowed"] is False
    assert results[4]["reason"] == "tool name not registered"


async def test_summary_buckets_and_invariant(wired: None) -> None:
    assert wired is None
    body = await _run(
        [
            {"tool": _READ_TOOL},
            {"tool": _READ_TOOL, "args": {"environment": "dev"}},
            {"tool": _WRITE_TOOL},
            {"tool": _DESTROY_TOOL},
            {"tool": _UNKNOWN_TOOL},
        ]
    )

    summary = body["summary"]
    assert summary["total"] == 5
    assert summary["allowed"] == 1
    assert summary["blocked"] == 4

    buckets = summary["blocked_by_reason"]
    assert buckets == {
        "unregistered": 1,
        "profile_block": 1,
        "environment_block": 1,
        "capability_block": 1,
    }
    assert sum(buckets.values()) <= summary["blocked"]


async def test_an_entry_that_names_no_call_is_dropped(wired: None) -> None:
    """A list member that is not an object names no call, so nothing counts it.

    The schema does not stop one, and reading it as a call would put a verdict
    on a tool the caller never named.
    """
    assert wired is None
    result = await handle_linode_profile_can_run(
        {"calls": [_READ_TOOL, {"tool": _READ_TOOL}, 7]}, Config()
    )
    body: dict[str, Any] = json.loads(result[0].text)

    assert len(body["results"]) == 1
    assert body["summary"]["total"] == 1


@pytest.mark.parametrize("wired", [(), ("*",)], indirect=True)
async def test_unrestricted_environments_allow_any(wired: None) -> None:
    assert wired is None
    body = await _run([{"tool": _READ_TOOL, "args": {"environment": "dev"}}])
    assert body["results"][0]["allowed"] is True


async def test_remedies_match_the_go_wording(wired: None) -> None:
    """The four remedy sentences are an exact-match contract with Go.

    The reason strings above were already pinned on both sides and the remedies
    were not, so a one-sided reword read as green in both languages.
    """
    assert wired is None
    body = await _run(
        [
            {"tool": _READ_TOOL},
            {"tool": _READ_TOOL, "args": {"environment": "dev"}},
            {"tool": _WRITE_TOOL},
            {"tool": _DESTROY_TOOL},
            {"tool": _UNKNOWN_TOOL},
        ]
    )

    results = body["results"]
    assert results[1]["remedy"] == (
        "target an environment in the profile's allowed_environments, or "
        "switch to a profile that permits this environment"
    )
    assert results[2]["remedy"] == (
        "switch to a profile that permits linode_instance_create, or add it to "
        "the current profile"
    )
    assert results[3]["remedy"] == (
        "switch to a profile that permits linode_instance_delete, or use yolo "
        "on a profile that allows it"
    )
    assert results[4]["remedy"] == (
        "check spelling or call linode_profile_list_tools to discover "
        "the registered tool surface"
    )
    assert "remedy" not in results[0]


_SHARED_FIXTURE = (
    Path(__file__).resolve().parents[3]
    / "testdata"
    / "profile"
    / "can_run_verdicts.json"
)

# The fixture names capabilities the way a profile file does; the pre-check
# reads the tags.
_FIXTURE_CAPABILITIES = {
    "read": Capability.Read,
    "write": Capability.Write,
    "destroy": Capability.Destroy,
}


async def test_verdicts_match_shared_fixture() -> None:
    """Every reason and remedy matches the shared cross-language fixture.

    A can_run reason is a member of a successful answer rather than a refusal,
    so no declaration words it and each language keeps its own copy. The
    behavior fixtures run under full-access, which permits every tool in every
    environment, so three of the four blocked categories are unreachable there:
    this fixture is what stops a one-sided reword.
    """
    fixture = json.loads(_SHARED_FIXTURE.read_text(encoding="utf-8"))
    assert fixture["calls"], (
        "the shared fixture names no calls, so this measures nothing"
    )

    catalog = [
        ToolDescriptor(
            name=entry["tool"], capability=_FIXTURE_CAPABILITIES[entry["capability"]]
        )
        for entry in fixture["catalog"]
    ]
    profile = Profile(
        name=fixture["profile"]["name"],
        description="shared verdict fixture",
        allowed_tools=tuple(fixture["profile"]["allowed_tools"]),
        allowed_environments=tuple(fixture["profile"]["allowed_environments"]),
    )

    token = set_builder_state(
        BuilderState(
            drafts=Registry(),
            catalog=lambda: catalog,
            active_profile=lambda: profile,
            config=Config(),
        )
    )
    try:
        body = await _run(fixture["calls"])
    finally:
        reset_builder_state(token)

    assert body == fixture["expect_result"]
