"""The declared transforms the generated handlers call before anything reads.

Both are held to what the per-tool bodies they replaced did: text is rewritten
in place, anything else is left for the check that words it by type, and an
argument nobody sent stays absent. Go spells the same pair TrimArguments and
TrimListDropBlank.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import (
    fold_int_list,
    trim_arguments,
    trim_list,
    trim_list_drop_blank,
    uppercase_arguments,
)

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


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({IMAGES: ["  us-east  ", "eu-west"]}, {IMAGES: ["us-east", "eu-west"]}),
        ({IMAGES: ["   ", "us-east"]}, {IMAGES: ["", "us-east"]}),
        ({IMAGES: [" us-east ", 3]}, {IMAGES: ["us-east", 3]}),
        ({IMAGES: "us-east"}, {IMAGES: "us-east"}),
    ],
)
def test_trim_list_trims_entries_and_keeps_blanks(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    trim_list(arguments, IMAGES)
    assert arguments == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({LABEL: "get"}, {LABEL: "GET"}),
        ({LABEL: "Put"}, {LABEL: "PUT"}),
        ({LABEL: 7}, {LABEL: 7}),
        ({ADDRESS: "x"}, {ADDRESS: "x"}),
    ],
)
def test_uppercase_arguments_folds_only_text(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    uppercase_arguments(arguments, LABEL)
    assert arguments == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"linode_ids": [123, 456]}, {"entities": {"linodes": [123, 456]}}),
        ({"linode_ids": [1.0]}, {"entities": {"linodes": [1]}}),
        (
            {"linode_ids": [123], "entities": {"volumes": [9]}},
            {"entities": {"volumes": [9]}},
        ),
        ({"linode_ids": "123"}, {"linode_ids": "123"}),
        ({"linode_ids": [True]}, {"linode_ids": [True]}),
        (
            {"linode_ids": [1], "entities": "linodes"},
            {"linode_ids": [1], "entities": "linodes"},
        ),
        ({}, {}),
    ],
)
def test_fold_int_list_folds_drops_and_refuses_like_the_hook(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    fold_int_list(arguments, "linode_ids", "entities", "linodes")
    assert arguments == expected
