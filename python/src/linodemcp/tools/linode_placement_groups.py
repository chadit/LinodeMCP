"""Placement group READ tools for LinodeMCP."""

from __future__ import annotations

from typing import Any


def _pg_member_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group member to proto-canonical form."""
    return {
        "linode_id": raw.get("linode_id", 0),
        "is_compliant": raw.get("is_compliant", False),
    }


def _pg_migration_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group migration to proto-canonical form."""
    return {"linode_id": raw.get("linode_id", 0)}


def _pg_migrations_to_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape raw placement group migrations to proto-canonical form."""
    return {
        "inbound": [_pg_migration_to_dict(m) for m in raw.get("inbound", [])],
        "outbound": [_pg_migration_to_dict(m) for m in raw.get("outbound", [])],
    }


def placement_group_to_response_dict(raw: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw placement group API dict to proto-canonical form.

    members is always emitted as a list; migrations is omitted when absent,
    matching the proto linode.mcp.v1.PlacementGroup serialization.
    """
    body: dict[str, Any] = {
        "id": raw.get("id", 0),
        "label": raw.get("label", ""),
        "region": raw.get("region", ""),
        "placement_group_type": raw.get("placement_group_type", ""),
        "placement_group_policy": raw.get("placement_group_policy", ""),
        "is_compliant": raw.get("is_compliant", False),
        "members": [_pg_member_to_dict(m) for m in raw.get("members", [])],
    }
    migrations = raw.get("migrations")
    if migrations is not None:
        body["migrations"] = _pg_migrations_to_dict(migrations)
    return body
