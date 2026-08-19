"""The declared transforms the generated handlers call before anything reads.

Both are held to what the per-tool bodies they replaced did: text is rewritten
in place, anything else is left for the check that words it by type, and an
argument nobody sent stays absent. Go spells the same pair TrimArguments and
TrimListDropBlank.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import trim_arguments, trim_list_drop_blank

LABEL = "label"
ADDRESS = "address"
IMAGES = "images"


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (
            {LABEL: "  web  ", ADDRESS: "\t10.0.0.1\n"},
            {LABEL: "web", ADDRESS: "10.0.0.1"},
        ),
        ({LABEL: "   "}, {LABEL: ""}),
        ({LABEL: 5, ADDRESS: None}, {LABEL: 5, ADDRESS: None}),
        ({"weight": 100}, {"weight": 100}),
    ],
)
def test_trim_arguments_rewrites_every_named_text_value(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    """A padded value is trimmed, and anything that is not text is left alone."""
    supplied = dict(arguments)
    trim_arguments(supplied, LABEL, ADDRESS)

    assert supplied == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (
            {IMAGES: ["  linode/debian12  ", "   ", "\tprivate/1"]},
            {IMAGES: ["linode/debian12", "private/1"]},
        ),
        ({IMAGES: ["  "]}, {IMAGES: []}),
        ({IMAGES: ["  linode/debian12  ", 5]}, {IMAGES: ["  linode/debian12  ", 5]}),
        ({IMAGES: "linode/debian12"}, {IMAGES: "linode/debian12"}),
        ({LABEL: "web"}, {LABEL: "web"}),
    ],
)
def test_trim_list_drop_blank_rewrites_only_all_text_lists(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    """Entries are trimmed and the blanks dropped, unless one is not text."""
    supplied = dict(arguments)
    trim_list_drop_blank(supplied, IMAGES)

    assert supplied == expected


def test_transforms_rewrite_every_name_they_are_given() -> None:
    """One declaration over several arguments renders as one call."""
    supplied: dict[str, Any] = {
        "summary": "  down  ",
        "description": "  since 3am  ",
        IMAGES: ["  a  "],
        "tags": ["  b  "],
    }

    trim_arguments(supplied, "summary", "description")
    trim_list_drop_blank(supplied, IMAGES, "tags")

    assert supplied == {
        "summary": "down",
        "description": "since 3am",
        IMAGES: ["a"],
        "tags": ["b"],
    }
