"""Cross-language capability spelling gate.

``testdata/profile/capability_spellings.json`` pins the string every capability
tag shows, and the Go twin
(``go/internal/profiles/capability_test.go``) asserts against the same file. A
tool answer that names a tier carries this string and a catalog filter is
matched against it, so a tier spelled two ways would answer two ways depending
on which binary served the call.
"""

from __future__ import annotations

import json
from pathlib import Path

from linodemcp.genpb.linode.mcp.v1 import options_pb2
from linodemcp.profiles import Capability, capability_spelling

# The tag each contract member declares the tier for. ``capability_spelling``
# reads a tier's name out of the generated enum by the tag's own number, so this
# table is what holds that numbering: without it, a renumbered member would
# spell as its neighbor and nothing would say so.
_CAPABILITY_TAGS = {
    "TOOL_CAPABILITY_UNSPECIFIED": Capability.Unknown,
    "TOOL_CAPABILITY_READ": Capability.Read,
    "TOOL_CAPABILITY_WRITE": Capability.Write,
    "TOOL_CAPABILITY_DESTROY": Capability.Destroy,
    "TOOL_CAPABILITY_ADMIN": Capability.Admin,
    "TOOL_CAPABILITY_META": Capability.Meta,
}

_SHARED_FIXTURE = (
    Path(__file__).resolve().parents[3]
    / "testdata"
    / "profile"
    / "capability_spellings.json"
)


def _fixture_rows() -> list[dict[str, object]]:
    """The shared fixture's spelling rows."""
    fixture = json.loads(_SHARED_FIXTURE.read_text(encoding="utf-8"))
    rows = fixture["spellings"]
    assert rows, "the shared fixture names no spellings, so this measures nothing"

    return list(rows)


def test_spellings_match_shared_fixture() -> None:
    """Every tag spells its tier the way the shared fixture pins it.

    Each row also pins the contract member the tag selects, which is the pairing
    that lets the spelling come off the generated enum rather than a second copy
    of the names.
    """
    for row in _fixture_rows():
        member = str(row["member"])
        value = int(str(row["value"]))
        assert options_pb2.ToolCapability.Name(value) == member

        tag = _CAPABILITY_TAGS[member]
        assert int(tag) == value, f"the tag for {member} is {int(tag)}, want {value}"
        assert capability_spelling(tag) == row["spelling"]


def test_shared_fixture_covers_every_declared_member() -> None:
    """A member the contract declares and the fixture skips fails here.

    Without this, a tier added to the proto could reach a tool answer with a
    spelling no language ever agreed on.
    """
    pinned = {int(str(row["value"])) for row in _fixture_rows()}

    for number in options_pb2.ToolCapability.values():
        member = options_pb2.ToolCapability.Name(number)
        assert number in pinned, (
            f"the contract declares {member} = {number} and the shared fixture "
            "pins no spelling for it"
        )


def test_spelling_refuses_numbers_outside_the_tag_set() -> None:
    """A number the contract declares no member for names no tier.

    Go's ``Capability.String`` answers the same words, so a tag read off a
    corrupt config says it names nothing rather than borrowing a neighbor's
    spelling.
    """
    assert capability_spelling(99) == "Capability(invalid)"
    assert capability_spelling(-1) == "Capability(invalid)"
    # Wider than the int32 keys Go's generated name map uses. Go narrowed the
    # tag to index that map once and spelled these as whatever tier their low
    # 32 bits landed on, so both languages pin the refusal.
    assert capability_spelling(1 << 32) == "Capability(invalid)"
    assert capability_spelling(1 << 32 | 1) == "Capability(invalid)"
