"""The plain spelling of each argument reader, driven directly.

Every member has a plain form beside its worded one, and the emitter writes the
plain form for a declaration that words no arm. Nothing in the contract words
nothing today, so these drive them here rather than letting them ship untested
behind a declaration that does not exist yet. Go pins the same three in
``TestPlainReaderSpellingsAnswerTheMembersOwnSentences``.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import free_text, id_list, present_string


@pytest.mark.parametrize(
    ("arguments", "required", "expected"),
    [
        ({}, True, "object_storage must be a string"),
        ({}, False, ""),
        ({"object_storage": "active"}, True, ""),
        ({"object_storage": ""}, True, ""),
        ({"object_storage": 5}, True, "object_storage must be a string"),
    ],
)
def test_present_string_answers_its_own_sentences(
    arguments: dict[str, Any], required: bool, expected: str
) -> None:
    """A blank is a value here; only a non-string is refused."""
    _, message = present_string(arguments, "object_storage", required=required)

    assert message == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "linodes is required"),
        ({"linodes": "123"}, "linodes must be a JSON array of positive integers"),
        (
            {"linodes": []},
            "linodes must be a non-empty array of distinct positive integers",
        ),
        (
            {"linodes": [1, 1]},
            "linodes must be a non-empty array of distinct positive integers",
        ),
        ({"linodes": [1, 2]}, ""),
    ],
)
def test_id_list_answers_its_own_sentences(
    arguments: dict[str, Any], expected: str
) -> None:
    """The three arms an absent, unusable and unusable-shaped list reach."""
    _, message = id_list(arguments, "linodes")

    assert message == expected


@pytest.mark.parametrize(
    ("arguments", "value", "expected"),
    [
        ({}, "", "engine_id is required"),
        ({"engine_id": "  "}, "", "engine_id must be a non-empty string"),
        ({"engine_id": 8}, "", "engine_id must be a non-empty string"),
        ({"engine_id": "mysql/8"}, "mysql/8", ""),
    ],
)
def test_free_text_answers_its_own_sentences(
    arguments: dict[str, Any], value: str, expected: str
) -> None:
    """It refuses nothing a rule can see, and hands back what it read."""
    got, message = free_text(arguments, "engine_id")

    assert got == value
    assert message == expected
