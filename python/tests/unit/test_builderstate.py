"""Builder-state seam tests.

Mirrors ``go/internal/tools/builderstate_test.go``. These cover what the
behavior fixtures cannot: a handler reached with no state attached, the draft
that has to survive from one call to the next, and the profile a pre-check
reads at call time rather than at registration.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.config import Config
from linodemcp.gentools.profile_builder import (
    handle_linode_profile_can_run,
    handle_linode_profile_draft_add_tools,
    handle_linode_profile_draft_discard,
    handle_linode_profile_draft_new,
    handle_linode_profile_draft_remove_tools,
    handle_linode_profile_draft_save,
    handle_linode_profile_draft_set,
    handle_linode_profile_draft_show,
    handle_linode_profile_list_categories,
    handle_linode_profile_list_tools,
)
from linodemcp.profiles import Capability, Profile
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    BuilderState,
    builder_state_from_context,
    reset_builder_state,
    set_builder_state,
)

if TYPE_CHECKING:
    from collections.abc import Iterator

_BOOT_TOOL = "linode_instance_boot"
_DRAFT_NAME = "seam-draft"


def _catalog() -> list[ToolDescriptor]:
    return [ToolDescriptor(name=_BOOT_TOOL, capability=Capability.Write)]


def _fixture_profile() -> Profile:
    return Profile(name="p", description="", allowed_tools=())


@pytest.fixture
def unwired() -> Iterator[None]:
    """Clear the published state for the duration of one test."""
    token = set_builder_state(None)
    yield
    reset_builder_state(token)


async def _handler_text(handler: Any, arguments: dict[str, Any]) -> str:
    """Invoke one generated builder handler and return its single text item."""
    response = await handler(arguments, Config())
    text: str = response[0].text
    return text


_NAMED = {"name": _DRAFT_NAME}
# Save is gated, and the generated handler asks the gate ahead of the state
# read, so the row has to clear it to reach what the case measures.
_CONFIRMED = {"name": _DRAFT_NAME, "confirm": True}


@pytest.mark.parametrize(
    ("handler", "arguments"),
    [
        (handle_linode_profile_list_tools, {}),
        (handle_linode_profile_list_categories, {}),
        (handle_linode_profile_can_run, {"calls": []}),
        (handle_linode_profile_draft_new, _NAMED),
        (handle_linode_profile_draft_show, _NAMED),
        (handle_linode_profile_draft_discard, _NAMED),
        (handle_linode_profile_draft_add_tools, _NAMED),
        (handle_linode_profile_draft_remove_tools, _NAMED),
        (handle_linode_profile_draft_set, _NAMED),
        (handle_linode_profile_draft_save, _CONFIRMED),
    ],
)
@pytest.mark.usefixtures("unwired")
async def test_builder_tools_refuse_without_state(
    handler: Any, arguments: dict[str, Any]
) -> None:
    """Every builder tool refuses when no state was published for the call.

    Driven through the generated handler because four of the ten no longer have
    an answer function: their state read is written by the declaration. Each row
    sends the name its contract requires, since the rule check runs ahead of the
    state read and would otherwise answer first. Production publishes the state
    on every dispatch, so this is the shape a broken wiring change would take,
    and Go answers the same sentence.
    """
    assert await _handler_text(handler, arguments) == f"Error: {BUILDER_UNCONFIGURED}"


def test_state_from_context_answers_what_was_published() -> None:
    """What goes in comes out, and an unpublished context answers None."""
    token = set_builder_state(None)
    try:
        assert builder_state_from_context() is None
    finally:
        reset_builder_state(token)

    state = BuilderState(
        drafts=Registry(),
        catalog=_catalog,
        active_profile=_fixture_profile,
        config=Config(),
    )
    token = set_builder_state(state)
    try:
        assert builder_state_from_context() is state
    finally:
        reset_builder_state(token)


@pytest.mark.asyncio
async def test_draft_survives_across_calls() -> None:
    """One published state across two calls keeps the draft the first made."""
    state = BuilderState(
        drafts=Registry(),
        catalog=_catalog,
        active_profile=_fixture_profile,
        config=Config(),
    )
    token = set_builder_state(state)

    try:
        await handle_linode_profile_draft_new({"name": _DRAFT_NAME}, Config())
        response = await handle_linode_profile_draft_show(
            {"name": _DRAFT_NAME}, Config()
        )
    finally:
        reset_builder_state(token)

    payload: dict[str, Any] = json.loads(response[0].text)
    assert payload["name"] == _DRAFT_NAME


@pytest.mark.asyncio
async def test_can_run_reads_the_profile_at_call_time() -> None:
    """Swapping the profile the reader answers changes the next call's verdict.

    That freshness is what lets a reload reach an already-registered tool.
    """
    active = Profile(name="before", description="", allowed_tools=())

    state = BuilderState(
        drafts=Registry(),
        catalog=_catalog,
        active_profile=lambda: active,
        config=Config(),
    )
    token = set_builder_state(state)
    args = {"calls": [{"tool": _BOOT_TOOL}]}

    try:
        before: dict[str, Any] = json.loads(
            (await handle_linode_profile_can_run(args, Config()))[0].text
        )
        assert before["active_profile"] == "before"
        assert before["results"][0]["allowed"] is False

        active = Profile(name="after", description="", allowed_tools=(_BOOT_TOOL,))

        after: dict[str, Any] = json.loads(
            (await handle_linode_profile_can_run(args, Config()))[0].text
        )
    finally:
        reset_builder_state(token)

    assert after["active_profile"] == "after"
    assert after["results"][0]["allowed"] is True


async def test_draft_tool_patterns_survive_a_non_list() -> None:
    """A tools value that is not a list parses as no patterns, and the draft
    registry then words its own refusal."""
    state = BuilderState(
        drafts=Registry(),
        catalog=_catalog,
        active_profile=_fixture_profile,
        config=Config(),
    )
    token = set_builder_state(state)
    try:
        response = await handle_linode_profile_draft_add_tools(
            {"name": "ghost", "tools": "linode_domain_*"}, Config()
        )
    finally:
        reset_builder_state(token)
    assert "Error" in response[0].text
