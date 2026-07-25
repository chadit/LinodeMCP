"""Unit tests for the shared pagination helpers in tools/helpers.py.

The cross-language contract (which values are rejected, what query string the
request carries) is pinned by the behavior fixtures both languages replay. These
cover the Python-only branches underneath it: the omit-when-unset arm of
``paginated_path`` that keeps an unset value off the query string so the API's
own default applies, and the bounds ``standard_pagination_arguments`` reads. Go
pins the same ground in pagination_query_test.go and standard_pagination_test.go.
"""

from __future__ import annotations

import pytest

from linodemcp.tools.helpers import (
    STANDARD_PAGE_SIZE_MAX,
    STANDARD_PAGE_SIZE_MIN,
    paginated_path,
    standard_pagination_arguments,
)


def test_paginated_path_omits_both_unset_values() -> None:
    """No pagination arguments means no query string, so the API default applies."""
    assert paginated_path("/regions", None, None) == "/regions"


@pytest.mark.parametrize(
    ("page", "page_size", "want"),
    [
        (2, 50, "/regions?page=2&page_size=50"),
        (2, None, "/regions?page=2"),
        (None, 50, "/regions?page_size=50"),
    ],
    ids=["both", "page-only", "page-size-only"],
)
def test_paginated_path_appends_only_the_set_values(
    page: int | None, page_size: int | None, want: str
) -> None:
    """Each value appears only when set, in the order Go's withPaginationQuery uses."""
    assert paginated_path("/regions", page, page_size) == want


def test_paginated_path_preserves_a_path_that_already_has_segments() -> None:
    """Sub-resource paths keep their ids; the query is appended, not substituted."""
    assert (
        paginated_path("/linode/instances/123/disks", 2, 50)
        == "/linode/instances/123/disks?page=2&page_size=50"
    )


def test_standard_pagination_arguments_absent_yields_none() -> None:
    """Absent arguments stay None so the caller omits them from the request."""
    assert standard_pagination_arguments({}) == (None, None)


def test_standard_pagination_arguments_reads_both() -> None:
    """A valid pair comes back parsed, not coerced to the bounds."""
    assert standard_pagination_arguments({"page": 3, "page_size": 100}) == (3, 100)


@pytest.mark.parametrize(
    "page_size",
    [STANDARD_PAGE_SIZE_MIN, STANDARD_PAGE_SIZE_MAX],
    ids=["minimum", "maximum"],
)
def test_standard_pagination_arguments_accepts_the_bounds(page_size: int) -> None:
    """Both bounds are inclusive, matching the spec's 25-500 range."""
    assert standard_pagination_arguments({"page_size": page_size})[1] == page_size


@pytest.mark.parametrize(
    "page_size",
    [STANDARD_PAGE_SIZE_MIN - 1, STANDARD_PAGE_SIZE_MAX + 1],
    ids=["below-minimum", "above-maximum"],
)
def test_standard_pagination_arguments_rejects_outside_the_bounds(
    page_size: int,
) -> None:
    """Outside the range is rejected rather than clamped, so the caller sees it."""
    with pytest.raises(ValueError, match="page_size must be an integer from"):
        standard_pagination_arguments({"page_size": page_size})


def test_standard_pagination_arguments_rejects_a_non_integer_page() -> None:
    """A non-integer page is a caller error, not a value to coerce."""
    with pytest.raises(TypeError, match="page must be an integer"):
        standard_pagination_arguments({"page": "x"})


def test_standard_pagination_arguments_rejects_page_below_one() -> None:
    """Page is one-based, so zero and negatives cannot address a page."""
    with pytest.raises(ValueError, match="page must be an integer greater than"):
        standard_pagination_arguments({"page": 0})
