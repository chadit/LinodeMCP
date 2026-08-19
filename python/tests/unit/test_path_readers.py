"""The readers a generated handler reads a path argument with.

path_int is Go's request.GetInt and path_str its request.GetString, which the
read and write tiers read with: a value of the wrong type has to answer the
absent value rather than raise or pass through, since raising left the caller
with an unhandled failure in Python where Go answered the tool's own "<name> is
required" sentence.

destroy_id is Go's tools.DestroyID, which the removal tier reads with instead.
It refuses what path_int reads past, because truncating 456.5 or coercing "456"
onto a live path removes a resource the caller never named.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import destroy_id, path_int, path_str


@pytest.mark.parametrize(
    ("value", "want"),
    [
        (7, 7),
        ("7", 7),
        ("+7", 7),
        ("007", 7),
        ("-3", -3),
        (-3, -3),
        (0, 0),
        ("abc", 0),
        ("", 0),
        ("7.5", 0),
        # strconv.Atoi is the whole legal set Go reads a string id with: it
        # takes neither surrounding space, nor a digit separator, nor a digit
        # outside ASCII, so neither does this.
        ("  7  ", 0),
        ("7_0", 0),
        (chr(0xFF17), 0),
        # Go GetInt truncates a float64 with int(v), so parity truncates too:
        # 42.0 must address resource 42 in both languages, and 7.5 reads 7.
        (7.5, 7),
        (42.0, 42),
        (-3.5, -3),
        # Go's JSON decoder refuses these outright, so only Python can hand one
        # to a reader, where int() raised rather than answering.
        (float("inf"), 0),
        (float("nan"), 0),
        (None, 0),
        ([], 0),
        ({}, 0),
        # Python counts a bool as an int, so True would otherwise address
        # resource 1 rather than reading as no id at all.
        (True, 0),
        (False, 0),
    ],
)
def test_path_int_reads_only_whole_numbers(value: Any, want: int) -> None:
    """Anything that is not a whole number reads as the absent value."""
    assert path_int(value) == want


@pytest.mark.parametrize(
    ("value", "want"),
    [
        ("us-east", "us-east"),
        ("", ""),
        # Go's GetString answers its default for a value of any other type, so
        # a number the caller sent never becomes a path segment.
        (5, ""),
        (5.0, ""),
        (True, ""),
        (None, ""),
        ([], ""),
        ({}, ""),
    ],
)
def test_path_str_reads_only_text(value: Any, want: str) -> None:
    """Anything that is not text reads as the absent value."""
    assert path_str(value) == want


REQUIRED = "widget_id is required"
POSITIVE = "widget_id must be a positive integer"


@pytest.mark.parametrize(
    ("arguments", "want_id", "want_message"),
    [
        ({}, 0, REQUIRED),
        ({"widget_id": 7}, 7, ""),
        ({"widget_id": 0}, 0, REQUIRED),
        ({"widget_id": -3}, 0, POSITIVE),
        ({"widget_id": 42.0}, 42, ""),
        ({"widget_id": 0.0}, 0, REQUIRED),
        ({"widget_id": -1.0}, 0, POSITIVE),
        # The class this tier exists for: path_int would read 5 out of both and
        # remove resource 5.
        ({"widget_id": 5.7}, 0, POSITIVE),
        ({"widget_id": "5"}, 0, POSITIVE),
        ({"widget_id": float("inf")}, 0, POSITIVE),
        ({"widget_id": float("nan")}, 0, POSITIVE),
        ({"widget_id": "abc"}, 0, POSITIVE),
        ({"widget_id": ""}, 0, POSITIVE),
        # A present null reaches the number check rather than the absent one,
        # which is where Go's DestroyID reaches for it too.
        ({"widget_id": None}, 0, POSITIVE),
        ({"widget_id": True}, 0, POSITIVE),
        ({"widget_id": False}, 0, POSITIVE),
        ({"widget_id": []}, 0, POSITIVE),
        ({"widget_id": {}}, 0, POSITIVE),
    ],
)
def test_destroy_id_refuses_every_id_that_is_not_a_whole_positive_number(
    arguments: dict[str, Any], want_id: int, want_message: str
) -> None:
    """Only an absent or zero id reads as missing; the rest are refused."""
    assert destroy_id(arguments, "widget_id") == (want_id, want_message)
