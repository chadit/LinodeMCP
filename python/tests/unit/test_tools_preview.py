"""The rule a declared preview sentence is chosen and filled by.

Pinned here rather than per tool because it is one rule: every tool declaring
prose reads its wordings through this, and the fixtures pin only the wording a
caller who supplied everything reads. The Go twin is
go/internal/tools/preview_sentence_test.go.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.preview import preview_number, preview_sentence, preview_text

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
