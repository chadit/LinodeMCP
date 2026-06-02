"""Unit tests for the linode_profile_can_run pre-check tool."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from linodemcp.profiles import Capability, Profile
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.linode_profile_can_run import (
    create_linode_profile_can_run_tool,
    handle_linode_profile_can_run,
    set_can_run_active_profile_provider,
    set_can_run_catalog_provider,
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
    """Install fixture bridges; clear them on teardown to avoid state bleed.

    The optional indirect param overrides the profile's allowed_environments.
    """
    environments: tuple[str, ...] = getattr(request, "param", ("prod",))
    set_can_run_catalog_provider(_fixture_catalog)
    set_can_run_active_profile_provider(lambda: _fixture_profile(environments))
    yield
    set_can_run_catalog_provider(None)
    set_can_run_active_profile_provider(None)


async def _run(calls: list[dict[str, Any]]) -> dict[str, Any]:
    result = await handle_linode_profile_can_run({"calls": calls})
    parsed: dict[str, Any] = json.loads(result[0].text)
    return parsed


def test_schema_and_capability() -> None:
    tool, capability = create_linode_profile_can_run_tool()
    assert tool.name == "linode_profile_can_run"
    assert capability == Capability.Meta
    assert "calls" in tool.inputSchema["properties"]


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


@pytest.mark.parametrize("wired", [(), ("*",)], indirect=True)
async def test_unrestricted_environments_allow_any(wired: None) -> None:
    assert wired is None
    body = await _run([{"tool": _READ_TOOL, "args": {"environment": "dev"}}])
    assert body["results"][0]["allowed"] is True
