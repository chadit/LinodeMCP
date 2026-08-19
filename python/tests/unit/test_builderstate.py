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
from linodemcp.tools.linode_profile_builder import (
    profile_list_categories_result,
    profile_list_tools_result,
)
from linodemcp.tools.linode_profile_can_run import profile_can_run_result
from linodemcp.tools.linode_profile_draft import (
    profile_draft_discard_result,
    profile_draft_new_result,
    profile_draft_show_result,
)
from linodemcp.tools.linode_profile_draft_mutate import (
    profile_draft_add_tools_result,
    profile_draft_remove_tools_result,
    profile_draft_set_result,
)
from linodemcp.tools.linode_profile_draft_save import profile_draft_save_result

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


def _answer_text(answer: Any, arguments: dict[str, Any]) -> str:
    """Invoke one migrated builder answer and return its single text item.

    draft_new is the only one that reads the configuration, so it is the only
    one handed a Config.
    """
    if answer is profile_draft_new_result:
        response = answer(arguments, Config())
    else:
        response = answer(arguments)
    text: str = response[0].text
    return text


@pytest.mark.parametrize(
    "answer",
    [
        profile_list_tools_result,
        profile_list_categories_result,
        profile_can_run_result,
        profile_draft_new_result,
        profile_draft_show_result,
        profile_draft_discard_result,
        profile_draft_add_tools_result,
        profile_draft_remove_tools_result,
        profile_draft_set_result,
        profile_draft_save_result,
    ],
)
@pytest.mark.usefixtures("unwired")
def test_builder_answers_refuse_without_state(answer: Any) -> None:
    """Every builder answer refuses when no state was published for the call.

    The whole class is generated now, so each one answers through its hook and
    the refusal is proven against the answer functions themselves. Production
    publishes the state on every dispatch, so this is the shape a broken wiring
    change would take, and Go answers the same sentence.
    """
    assert _answer_text(answer, {"calls": []}) == f"Error: {BUILDER_UNCONFIGURED}"


def test_state_from_context_answers_what_was_published() -> None:
    """What goes in comes out, and an unpublished context answers None."""
    token = set_builder_state(None)
    try:
        assert builder_state_from_context() is None
    finally:
        reset_builder_state(token)

    state = BuilderState(
        drafts=Registry(), catalog=_catalog, active_profile=_fixture_profile
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
        drafts=Registry(), catalog=_catalog, active_profile=_fixture_profile
    )
    token = set_builder_state(state)

    try:
        profile_draft_new_result({"name": _DRAFT_NAME}, Config())
        response = profile_draft_show_result({"name": _DRAFT_NAME})
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
        drafts=Registry(), catalog=_catalog, active_profile=lambda: active
    )
    token = set_builder_state(state)
    args = {"calls": [{"tool": _BOOT_TOOL}]}

    try:
        before: dict[str, Any] = json.loads(profile_can_run_result(args)[0].text)
        assert before["active_profile"] == "before"
        assert before["results"][0]["allowed"] is False

        active = Profile(name="after", description="", allowed_tools=(_BOOT_TOOL,))

        after: dict[str, Any] = json.loads(profile_can_run_result(args)[0].text)
    finally:
        reset_builder_state(token)

    assert after["active_profile"] == "after"
    assert after["results"][0]["allowed"] is True
