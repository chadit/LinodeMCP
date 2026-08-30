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
from types import MappingProxyType
from typing import TYPE_CHECKING, Final

from linodemcp.genpb.linode.mcp.v1 import options_pb2

if TYPE_CHECKING:
    from collections.abc import Mapping

# What every capability tag's spelling opens with, so cutting it leaves the
# short form a catalog filter may name instead.
CAPABILITY_PREFIX: Final = "Cap"

# What the contract puts in front of every ToolCapability member name, leaving
# the tier alone once cut.
_MEMBER_PREFIX: Final = "TOOL_CAPABILITY_"

# What ``Unknown`` shows. The contract calls its zero member
# TOOL_CAPABILITY_UNSPECIFIED, so the untagged marker is the one tag whose name
# the enum cannot supply without changing what an operator already reads.
_UNTAGGED_SPELLING: Final = f"{CAPABILITY_PREFIX}Unknown"

# What a number outside the tag set shows, naming no tier because there is none
# to name. Go's ``Capability.String`` answers the same words.
_INVALID_SPELLING: Final = "Capability(invalid)"

# Every ToolCapability member the contract declares, by number. Reading the
# generated enum is what keeps the spellings below from being a second copy of
# the names the proto already carries.
_MEMBER_NAMES: Final[Mapping[int, str]] = MappingProxyType(
    {
        number: str(options_pb2.ToolCapability.Name(number))
        for number in options_pb2.ToolCapability.values()
    }
)


class Capability(IntEnum):
    """Tool capability classification.

    ``Unknown`` is the zero value on purpose. A registration that omits
    the capability argument gets ``Unknown``; the capability-and-confirm
    invariant check ignores those during the Phase 1 rollout. The Phase 1
    cleanup PR adds a stricter assertion that fails on any remaining
    ``Unknown`` registration.

    The numbers are the ToolCapability member numbers the contract declares,
    which is what lets ``capability_spelling`` read each tier's name off the
    generated enum. ``testdata/profile/capability_spellings.json`` pins the
    pairing in every language.
    """

    Unknown = 0
    Read = 1
    Write = 2
    Destroy = 3
    Admin = 4
    Meta = 5


def capability_spelling(capability: int) -> str:
    """The spelling one capability tag shows.

    A tool answer that names a tier carries this string and a catalog filter is
    matched against it, so Go's ``Capability.String`` answers the same words for
    the same number. The argument is the number rather than the member so a tag
    the contract declares nothing for refuses here the way Go's does, instead of
    failing the enum lookup a caller made first.
    """
    if capability == Capability.Unknown:
        return _UNTAGGED_SPELLING

    tier = _MEMBER_NAMES.get(int(capability), "").removeprefix(_MEMBER_PREFIX)
    if not tier:
        return _INVALID_SPELLING

    return f"{CAPABILITY_PREFIX}{tier[:1]}{tier[1:].lower()}"
