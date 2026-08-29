"""The rule a declared preview sentence is chosen and filled by.

Pinned here rather than per tool because it is one rule: every tool declaring
prose reads its wordings through this, and the fixtures pin only the wording a
caller who supplied everything reads. The Go twin is
go/internal/tools/preview_sentence_test.go.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.declared_state import DeclaredState
from linodemcp.tools.preview import (
    preview_carried,
    preview_carried_number,
    preview_changed,
    preview_differs,
    preview_guarded,
    preview_joined,
    preview_matched,
    preview_number,
    preview_per_element,
    preview_sentence,
    preview_state_number,
    preview_state_text,
    preview_text,
)

LABELLED = 'A new personal access token "{label}" will be created.'
PLAIN = "A new personal access token will be created."


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"label": "ci"}, "ci"),
        ({}, ""),
        ({"label": ""}, ""),
        ({"label": 7}, ""),
        ({"label": True}, ""),
    ],
)
def test_preview_text_reports_only_text(
    arguments: dict[str, Any], expected: str
) -> None:
    """A label that arrived as anything but text is not a label."""
    assert preview_text(arguments, "label") == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"size": 20}, "20"),
        ({"size": 20.0}, "20"),
        ({}, ""),
        ({"size": 0}, ""),
        ({"size": "20"}, ""),
        ({"size": True}, ""),
        ({"size": 20.5}, ""),
    ],
)
def test_preview_number_reports_only_a_whole_number_that_names_something(
    arguments: dict[str, Any], expected: str
) -> None:
    """Zero reads as none: the ids a sentence names are resource ids."""
    assert preview_number(arguments, "size") == expected


@pytest.mark.parametrize(
    ("values", "expected"),
    [
        ({"label": "ci"}, 'A new personal access token "ci" will be created.'),
        ({"label": ""}, PLAIN),
        ({}, PLAIN),
    ],
)
def test_preview_sentence_reports_the_first_wording_it_can_fill(
    values: dict[str, str], expected: str
) -> None:
    """The specific wording when every placeholder has a value, else the next."""
    assert preview_sentence(values, LABELLED, PLAIN) == expected


def test_preview_sentence_fills_every_placeholder_in_a_wording() -> None:
    """Several placeholders in one wording all get their value."""
    values = {"type": "g6-standard-1", "region": "us-east"}
    template = "A new {type} instance will be created in region {region}."

    assert (
        preview_sentence(values, template)
        == "A new g6-standard-1 instance will be created in region us-east."
    )


def test_preview_sentence_drops_a_line_no_wording_can_fill() -> None:
    """A line whose every wording wants a value the call did not carry.

    It is reported not at all, rather than with a gap where the value would
    have gone.
    """
    assert preview_sentence({}, "A new {type} instance was requested.") == ""


def test_preview_sentence_with_no_wording_reports_nothing() -> None:
    """Nothing declared is nothing reported."""
    assert preview_sentence({}) == ""


def test_preview_sentence_answers_a_wording_whose_brace_never_closes() -> None:
    """The contract refuses such a wording.

    This is the answer for a call that reached one anyway: the text as written
    rather than a crash or a truncated sentence.
    """
    broken = "A new {label instance was requested."

    assert preview_sentence({}, broken) == broken


@pytest.mark.parametrize(
    ("state", "path", "text", "whole"),
    [
        (DeclaredState({"label": "web-1"}), "label", "web-1", ""),
        (DeclaredState({"size": 20}), "size", "", "20"),
        (DeclaredState({"count": 0}), "count", "", ""),
        (DeclaredState({"label": "web-1"}), "nobody", "", ""),
        (
            DeclaredState({"instance": DeclaredState({"type": "g6-standard-1"})}),
            "instance.type",
            "g6-standard-1",
            "",
        ),
        ("not a declared state", "label", "", ""),
    ],
)
def test_preview_state_readers_report_what_the_resource_carries(
    state: Any, path: str, text: str, whole: str
) -> None:
    """A wording naming the resource reads a member of it.

    One level down for a composite projection, and nothing at all from an
    answer some other fetch produced, which is what lets the wording naming it
    step aside rather than report a gap. The Go twin is
    TestPreviewStateReadersReportWhatTheResourceCarries.
    """
    assert preview_state_text(state, path) == text
    assert preview_state_number(state, path) == whole


def test_preview_changed_reads_a_matching_reading_as_absent() -> None:
    """The rule turning a change wording into a set wording is one comparison.

    Pinned here rather than through every family that declares the pair. The Go
    twin is TestPreviewChangedReadsAMatchingReadingAsAbsent.
    """
    assert preview_changed("old", "new") == "old"
    assert preview_changed("same", "same") == ""


def test_preview_guard_reads_carried_rather_than_empty() -> None:
    """A guarded line asks whether the call sent the argument.

    A zero and an empty list still report, while an argument nobody sent drops
    the line. The Go twin is TestPreviewGuardReadsCarriedRatherThanEmpty.
    """
    arguments: dict[str, Any] = {"count": 0, "tags": []}

    assert preview_carried(arguments, "count")
    assert preview_carried(arguments, "tags")
    assert not preview_carried(arguments, "label")

    assert preview_carried_number(arguments, "count") == "0"
    assert preview_carried_number(arguments, "label") == ""
    # A fraction is no whole number, the reading preview_number already gives it.
    assert preview_carried_number({"count": 1.5}, "count") == ""
    assert preview_carried_number({"count": True}, "count") == ""

    assert preview_guarded(carried=True, line="reported") == "reported"
    assert preview_guarded(carried=False, line="reported") == ""


@pytest.mark.parametrize(
    ("reading", "argument", "expected"),
    [
        ("old", "new", True),
        ("same", "same", False),
        ("old", "", False),
        ("", "new", True),
    ],
)
def test_preview_differs_reports_only_a_real_change(
    reading: str, argument: str, *, expected: bool
) -> None:
    """A line naming only the new value reports nothing without a real change.

    Nothing to say when the call carries no value, and nothing to say when the
    resource already holds what the call would set. The Go twin is
    TestPreviewDiffersReportsOnlyARealChange.
    """
    assert preview_differs(reading, argument) is expected


def test_preview_matched_selects_the_arm_the_value_names() -> None:
    """The arms are a shortlist, not the whole vocabulary.

    A value none of them names falls to the wording that answers the rest, and
    an empty one drops the line. The Go twin is
    TestPreviewMatchedSelectsTheArmTheValueNames.
    """
    arms = {"disabled": "stops {status}"}

    assert (
        preview_matched({"status": "disabled"}, "disabled", arms, "starts {status}")
        == "stops disabled"
    )
    assert (
        preview_matched({"status": "enabled"}, "enabled", arms, "starts {status}")
        == "starts enabled"
    )
    assert preview_matched({"status": "enabled"}, "enabled", arms, "") == ""


def test_preview_elements_reads_every_entry_shape() -> None:
    """The entries a wording writes over are text or whole numbers.

    A list carrying anything else drops that entry rather than spelling it a
    way the other language would not. The Go twin is
    TestPreviewElementsReadsEveryEntryShape.
    """
    arguments: dict[str, Any] = {
        "tags": ["prod", 42, True, 1.5, "staging"],
        "label": "not a list",
        "region": [],
    }

    assert preview_joined(arguments, "tags", ", ") == "prod, 42, staging"
    assert preview_joined(arguments, "label", ", ") == ""
    assert preview_joined(arguments, "region", ", ") == ""
    assert preview_joined(arguments, "count", ", ") == ""


def test_preview_per_element_writes_a_line_per_entry() -> None:
    """A per-entry line binds the entry and reads the other arguments normally.

    A list the call did not send writes nothing at all. The Go twin is
    TestPreviewPerElementWritesALinePerEntry.
    """
    arguments: dict[str, Any] = {"tags": [456, 789]}
    template = "Linode {element} joins group {count}."

    assert preview_per_element(arguments, "tags", {"count": "7"}, template) == [
        "Linode 456 joins group 7.",
        "Linode 789 joins group 7.",
    ]
    assert preview_per_element(arguments, "region", {"count": "7"}, template) == []
