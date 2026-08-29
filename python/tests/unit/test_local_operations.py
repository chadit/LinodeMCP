"""The generated arm layer: its conformance walk and its last except arm.

Python has no compile step, so the two things Go gets from the compiler need
tests here instead. The first is that every subsystem defines the method its arm
calls; the walk is generated and this runs it. The second is what an arm does
with a failure its own declaration does not name: it must refuse to project,
because the answer beside an unnamed failure is nothing.
"""

from __future__ import annotations

from linodemcp.genlocal import (
    LocalDraftMissingError,
    LocalOperationError,
    ProfileDraftResponse,
)
from linodemcp.gentools.conformance import check_local_operations
from linodemcp.gentools.operations import run_draft_read
from linodemcp.tools.local_answer import LocalRefusal, local_response

# What a subsystem could fail with that its declaration does not list.
UNNAMED_FAILURE = "the store went away"


class UnnamedDraftRead:
    """A draft-read subsystem reporting a condition the operation misses."""

    def draft_read(self, draft: str) -> ProfileDraftResponse:
        """Report a failure LOCAL_CALL_DRAFT_READ does not declare."""
        raise LocalOperationError(f"{UNNAMED_FAILURE}: {draft}")


class MissingDraftRead:
    """A draft-read subsystem reporting the one condition it declares."""

    def draft_read(self, draft: str) -> ProfileDraftResponse:
        """Report the declared condition, naming the draft it looked for."""
        raise LocalDraftMissingError(f"no draft named {draft}")


def test_every_subsystem_defines_the_method_its_arm_calls() -> None:
    """The generated walk, which is this language's half of the Go compiler."""
    assert check_local_operations() == []


def test_generated_arm_refuses_to_project_an_unnamed_failure() -> None:
    """Without this the arm would project an answer nothing built."""
    outcome = run_draft_read(UnnamedDraftRead(), "any-draft")

    assert outcome.body == {}
    assert outcome.refusal is LocalRefusal.NONE
    assert UNNAMED_FAILURE in outcome.cause


def test_an_unnamed_failure_answers_the_disagreement_rather_than_a_half_body() -> None:
    """What a caller sees is what makes the refusal safe."""
    outcome = run_draft_read(UnnamedDraftRead(), "any-draft")

    answered = local_response("linode.mcp.v1.ProfileDraftResponse", outcome.body)

    assert "unfilled" in answered[0].text


def test_the_declared_condition_still_words_its_own_refusal() -> None:
    """The control beside the two above: the declared arm is unchanged."""
    outcome = run_draft_read(MissingDraftRead(), "any-draft")

    assert outcome.refusal is LocalRefusal.DRAFT_MISSING
