"""The membership reader generated handlers hold enum-shaped arguments to.

member_choice mirrors Go's tools.MemberChoiceArgument: it reads the RAW
argument string, because an enum argument naming no member decodes to the
enum's zero, the same as an absent one, so only the argument map can tell
"left out" from "named wrong". The sentences are composed from the member
names the renderer arms pass.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.helpers import member_choice

_MEMBERS = ("master", "slave")
_MEMBERS_SENTENCE = "type must be one of: master, slave"


@pytest.mark.parametrize(
    ("arguments", "required", "members", "value", "message"),
    [
        ({}, True, _MEMBERS, "", "type is required"),
        ({}, False, _MEMBERS, "", ""),
        ({"type": 5}, True, _MEMBERS, "", "type is required"),
        ({"type": ""}, True, _MEMBERS, "", "type is required"),
        ({"type": ""}, False, _MEMBERS, "", ""),
        ({"type": "primary"}, True, _MEMBERS, "", _MEMBERS_SENTENCE),
        ({"type": "unspecified"}, False, _MEMBERS, "", _MEMBERS_SENTENCE),
        ({"type": "primary"}, False, _MEMBERS, "", _MEMBERS_SENTENCE),
        (
            {"type": "affinity"},
            True,
            ("anti_affinity:local",),
            "",
            "type must be anti_affinity:local",
        ),
        ({"type": "slave"}, True, _MEMBERS, "slave", ""),
    ],
    ids=[
        "absent required",
        "absent optional passes",
        "non-string reads as absent",
        "empty reads as absent",
        "empty optional passes",
        "outside the vocabulary",
        "sentinel is no member",
        "optional non-member still refused",
        "one-value vocabulary names the value alone",
        "member passes and answers the value",
    ],
)
def test_member_choice_holds_the_raw_argument_to_its_vocabulary(
    arguments: dict[str, Any],
    required: bool,
    members: tuple[str, ...],
    value: str,
    message: str,
) -> None:
    """Every sentence the reader answers, and the value a member passes as."""
    assert member_choice(arguments, "type", required, members) == (value, message)
