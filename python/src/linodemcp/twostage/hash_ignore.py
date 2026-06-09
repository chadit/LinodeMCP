"""Per-resource-type hash-ignore lists for two-stage drift detection.

Mirrors ``go/internal/twostage/hash_ignore.go``. Cosmetic fields that change
without a user-caused mutation (server-side timestamps, telemetry) are stripped
before the state is hashed, so a plan does not refuse on drift the user never
made. An unknown resource type returns an empty list and the whole state is
hashed.
"""

from __future__ import annotations

_FIELD_UPDATED = "updated"

_HASH_IGNORE_BY_TYPE: dict[str, list[str]] = {
    # Instance.updated bumps on unrelated account activity; the watchdog and
    # last-seen telemetry move without a user mutation.
    "Instance": [_FIELD_UPDATED, "last_seen_ipv4", "watchdog_enabled", "host_uuid"],
    # Volume.updated bumps on attach/detach bookkeeping unrelated to delete.
    "Volume": [_FIELD_UPDATED, "last_seen_ipv4"],
    # LKE cluster timestamps churn as nodes recycle.
    "LKECluster": [_FIELD_UPDATED, "created"],
    # Firewall.updated bumps when attached devices change state.
    "Firewall": [_FIELD_UPDATED],
    # NodeBalancer.transfer carries running bandwidth counters that move
    # continuously; updated bumps with them.
    "NodeBalancer": [_FIELD_UPDATED, "transfer"],
    # VPC.updated bumps as subnets and interfaces attach or detach.
    "VPC": [_FIELD_UPDATED],
    # Domain.updated bumps on record edits unrelated to deleting the zone.
    "Domain": [_FIELD_UPDATED],
    # StackScript.updated bumps on edits; deployments_total counts every
    # deploy by anyone, so it moves without the owner acting.
    "StackScript": [_FIELD_UPDATED, "deployments_total"],
    # Disk.updated bumps on imaging and resize bookkeeping.
    "Disk": [_FIELD_UPDATED],
    # VPC subnet timestamps move as interfaces attach and detach.
    "VPCSubnet": [_FIELD_UPDATED],
    # DNS record timestamps move on unrelated edits to the same record.
    "DomainRecord": [_FIELD_UPDATED],
    # LKE pool node list churns as nodes recycle; the pool itself is the
    # delete target, so the member nodes are telemetry, not user intent.
    "LKENodePool": ["nodes"],
}


def hash_ignore_fields(resource_type: str) -> list[str]:
    """Return the cosmetic fields stripped before hashing a resource's state."""
    return _HASH_IGNORE_BY_TYPE.get(resource_type, [])
