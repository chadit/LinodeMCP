"""Capability enum for tool registration tagging.

Each registered tool carries a ``Capability`` value indicating the kind of
operation it performs. ``Capability.Unknown`` is the zero value: a tool
that has not yet been tagged with a real capability. During the Phase 1
rollout this is expected for not-yet-tagged tools. After the Phase 1
cleanup PR lands, any tool registering with ``Unknown`` is a bug.

Tag meanings (mirrors Go's ``profiles.Capability``):

- ``Read``    -- GET endpoints, no state change.
- ``Write``   -- POST/PUT creating or updating resources.
- ``Destroy`` -- DELETE endpoints and explicitly destructive POSTs.
- ``Admin``   -- account-level mutations.
- ``Meta``    -- touches our config or session state, never the Linode API.
"""

from __future__ import annotations

from enum import IntEnum


class Capability(IntEnum):
    """Tool capability classification.

    ``Unknown`` is the zero value on purpose. A registration that omits
    the capability argument gets ``Unknown``; the capability-and-confirm
    invariant check ignores those during the Phase 1 rollout. The Phase 1
    cleanup PR adds a stricter assertion that fails on any remaining
    ``Unknown`` registration.
    """

    Unknown = 0
    Read = 1
    Write = 2
    Destroy = 3
    Admin = 4
    Meta = 5
