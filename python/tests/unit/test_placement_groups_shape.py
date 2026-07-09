"""Shaping-function coverage for placement group read tools.

``placement_group_to_response_dict`` and its member/migration helpers turn a
raw API dict into the proto-canonical shape. The handler tests exercise the
tool wiring; these pin the field defaults and the members-always-list /
migrations-omitted-when-absent rules directly.
"""

from __future__ import annotations

from linodemcp.tools.linode_placement_groups import placement_group_to_response_dict


def test_full_placement_group_shape() -> None:
    """A populated raw dict maps every field, members, and migrations."""
    raw = {
        "id": 42,
        "label": "web-affinity",
        "region": "us-east",
        "placement_group_type": "anti_affinity:local",
        "placement_group_policy": "strict",
        "is_compliant": True,
        "members": [
            {"linode_id": 101, "is_compliant": True},
            {"linode_id": 202, "is_compliant": False},
        ],
        "migrations": {
            "inbound": [{"linode_id": 303}],
            "outbound": [{"linode_id": 404}, {"linode_id": 505}],
        },
    }

    result = placement_group_to_response_dict(raw)

    assert result == {
        "id": 42,
        "label": "web-affinity",
        "region": "us-east",
        "placement_group_type": "anti_affinity:local",
        "placement_group_policy": "strict",
        "is_compliant": True,
        "members": [
            {"linode_id": 101, "is_compliant": True},
            {"linode_id": 202, "is_compliant": False},
        ],
        "migrations": {
            "inbound": [{"linode_id": 303}],
            "outbound": [{"linode_id": 404}, {"linode_id": 505}],
        },
    }


def test_empty_raw_uses_defaults_and_omits_migrations() -> None:
    """An empty raw dict yields zero-value fields, an empty members list,
    and no migrations key.
    """
    result = placement_group_to_response_dict({})

    assert result == {
        "id": 0,
        "label": "",
        "region": "",
        "placement_group_type": "",
        "placement_group_policy": "",
        "is_compliant": False,
        "members": [],
    }
    assert "migrations" not in result


def test_member_defaults_fill_missing_fields() -> None:
    """A member missing both keys defaults linode_id to 0, is_compliant False."""
    result = placement_group_to_response_dict({"members": [{}]})

    assert result["members"] == [{"linode_id": 0, "is_compliant": False}]


def test_migrations_present_but_empty_sides() -> None:
    """A migrations object with no inbound/outbound yields empty lists, not
    an omitted key.
    """
    result = placement_group_to_response_dict({"migrations": {}})

    assert result["migrations"] == {"inbound": [], "outbound": []}
