"""The presence readers the generated handlers call for the slot 2 cohort.

``present_text`` and ``present_bool`` answer for one argument each, and
``require_any_argument`` answers for a set of them. All three read the raw
argument map rather than the decoded message, because an implicit-presence
field reads absent and zero alike and the retired hooks told those apart.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import present_bool, present_text, require_any_argument

LABEL = "label"
PUBLIC = "public"
LABEL_REQUIRED = "label is required"
PUBLIC_BOOLEAN = "public must be a boolean"
OAUTH_ANY_OF = "at least one of label, redirect_uri, or public is required"


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, LABEL_REQUIRED),
        ({LABEL: None}, LABEL_REQUIRED),
        ({LABEL: 7}, LABEL_REQUIRED),
        ({LABEL: True}, LABEL_REQUIRED),
        ({LABEL: ""}, LABEL_REQUIRED),
        ({LABEL: "   "}, LABEL_REQUIRED),
        ({LABEL: "my-app"}, ""),
        ({LABEL: "  my-app  "}, ""),
    ],
)
def test_present_text_required_refuses_every_unusable_shape(
    arguments: dict[str, Any], expected: str
) -> None:
    """One sentence covers a missing label and a label sent as spaces alike."""
    assert present_text(arguments, LABEL, required=True) == (None, expected)


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, ""),
        ({LABEL: None}, LABEL_REQUIRED),
        ({LABEL: 7}, LABEL_REQUIRED),
        ({LABEL: ""}, LABEL_REQUIRED),
        ({LABEL: "   "}, LABEL_REQUIRED),
        ({LABEL: "renamed-app"}, ""),
    ],
)
def test_present_text_optional_refuses_only_what_was_sent(
    arguments: dict[str, Any], expected: str
) -> None:
    """Saying nothing is a legal request; only a supplied value must be usable."""
    assert present_text(arguments, LABEL, required=False) == (None, expected)


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, PUBLIC_BOOLEAN),
        ({PUBLIC: None}, PUBLIC_BOOLEAN),
        ({PUBLIC: "yes"}, PUBLIC_BOOLEAN),
        ({PUBLIC: 1}, PUBLIC_BOOLEAN),
        ({PUBLIC: False}, ""),
        ({PUBLIC: True}, ""),
    ],
)
def test_present_bool_required_reads_presence_not_truth(
    arguments: dict[str, Any], expected: str
) -> None:
    """False is an answer the caller is entitled to give, so only absence refuses."""
    assert present_bool(arguments, PUBLIC, required=True) == (None, expected)


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, ""),
        ({PUBLIC: None}, PUBLIC_BOOLEAN),
        ({PUBLIC: "yes"}, PUBLIC_BOOLEAN),
        ({PUBLIC: False}, ""),
        ({PUBLIC: True}, ""),
    ],
)
def test_present_bool_optional_refuses_only_what_was_sent(
    arguments: dict[str, Any], expected: str
) -> None:
    """An absent optional flag leaves the setting alone."""
    assert present_bool(arguments, PUBLIC, required=False) == (None, expected)


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, OAUTH_ANY_OF),
        ({"confirm": True, "dry_run": True}, OAUTH_ANY_OF),
        ({LABEL: "renamed-app"}, ""),
        ({PUBLIC: False}, ""),
        ({"redirect_uri": None}, ""),
        ({"redirect_uri": []}, ""),
        ({LABEL: 7}, ""),
    ],
)
def test_require_any_argument_asks_only_whether_one_arrived(
    arguments: dict[str, Any], expected: str
) -> None:
    """An update sends a field's zero to clear it, so presence alone satisfies."""
    assert (
        require_any_argument(arguments, OAUTH_ANY_OF, LABEL, "redirect_uri", PUBLIC)
        == expected
    )
