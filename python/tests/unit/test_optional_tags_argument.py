"""Unit checks for the shared tags body-argument reader.

The Go twin is optionalTagsField / tagsValueFromToolArg in
go/internal/tools/linode_images_write.go. Both accept a native array and a
JSON-encoded string, trim entries, and reject the same shapes with the same
text, so a client that reaches one implementation reaches the other the same
way.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import (
    TAGS_ENTRIES_NON_EMPTY,
    TAGS_MUST_BE_JSON_STRING_ARRAY,
    optional_tags_argument,
)


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, None),
        ({"tags": []}, []),
        ({"tags": ["prod", "web"]}, ["prod", "web"]),
        ({"tags": [" prod ", "web"]}, ["prod", "web"]),
        ({"tags": '["prod", "web"]'}, ["prod", "web"]),
        ({"tags": '  ["prod"]  '}, ["prod"]),
    ],
)
def test_accepted_tag_shapes(
    arguments: dict[str, Any], expected: list[str] | None
) -> None:
    """An absent argument, a native array, and a JSON string all parse."""
    tags, error = optional_tags_argument(arguments)

    assert error == ""
    assert tags == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"tags": 5}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": True}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": {"prod": True}}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": "not json"}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": "null"}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": '"prod"'}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": ["prod", 5]}, TAGS_MUST_BE_JSON_STRING_ARRAY),
        ({"tags": ["prod", ""]}, TAGS_ENTRIES_NON_EMPTY),
        ({"tags": ["prod", "   "]}, TAGS_ENTRIES_NON_EMPTY),
        ({"tags": '["prod", ""]'}, TAGS_ENTRIES_NON_EMPTY),
    ],
)
def test_rejected_tag_shapes(arguments: dict[str, Any], expected: str) -> None:
    """Every rejected shape returns no tags and the message Go emits."""
    tags, error = optional_tags_argument(arguments)

    assert tags is None
    assert error == expected
