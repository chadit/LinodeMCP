"""Drive the open-object walks the contract declares through the shared reader.

Each case is one a client can make and the sentence it reads back; a case
expecting ``""`` is one the walk accepts and another part of the handler owns.
The Go twin is go/internal/toolwalk/toolwalk_test.go, and both read the same
declaration out of the same descriptors.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp.tools.objectwalk import walk

CONFIG_CREATE = "linode.mcp.v1.InstanceConfigCreateInput"
CONFIG_UPDATE = "linode.mcp.v1.InstanceConfigUpdateInput"
CONTACT_CREATE = "linode.mcp.v1.ManagedContactCreateInput"
SETTINGS_UPDATE = "linode.mcp.v1.ManagedLinodeSettingsUpdateInput"
FIREWALL_UPDATE = "linode.mcp.v1.FirewallSettingsUpdateInput"
GRANTS_UPDATE = "linode.mcp.v1.AccountUserGrantsUpdateInput"

_DEVICES_REQUIRED = "devices is required"
_SLOT_NOT_OBJECT = "device sda must be an object"
_DISK_INTEGER = "invalid devices JSON: sda.disk_id must be an integer"
_SECTION_SHAPE = "linode must be an array of objects"
_SSH_PORT = "ssh.port must be an integer from 1 to 65535 or null"
_SSH_USER = "ssh.user must be a string up to 32 characters or null"
_PURPOSE_CHOICE = "interfaces[0].purpose must be one of: public, vlan, vpc"


def _slot(members: Any) -> dict[str, Any]:
    """One config device map carrying the named slot value."""
    return {"devices": {"sda": members}}


def _interfaces(entries: Any) -> dict[str, Any]:
    """A valid create carrying the given legacy interfaces argument."""
    return {"devices": {"sda": {"disk_id": 111}}, "interfaces": entries}


@pytest.mark.parametrize(
    ("message", "arguments"),
    [
        ("linode.mcp.v1.DomainGetInput", {"devices": 7}),
        ("linode.mcp.v1.NoSuchInput", {}),
        ("", {}),
    ],
)
def test_a_message_with_no_walk_is_accepted(
    message: str, arguments: dict[str, Any]
) -> None:
    """The seam is written only for a tool that declares a walk.

    A name that resolves to no declaration must answer nothing rather than
    refusing every call.
    """
    assert walk(message, arguments) == ""


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, _DEVICES_REQUIRED),
        ({"devices": None}, _DEVICES_REQUIRED),
        ({"devices": "sda"}, "devices must be a JSON object"),
        ({"devices": {}}, "devices must include at least one device slot"),
        ({"devices": {"sdz": {}}}, "device slot sdz must be one of sda through sdh"),
        (_slot(111), _SLOT_NOT_OBJECT),
        (_slot(None), _SLOT_NOT_OBJECT),
        (_slot({"cdrom": 1}), 'invalid devices JSON: unknown field "cdrom"'),
        (_slot({"disk_id": "111"}), _DISK_INTEGER),
        (_slot({"disk_id": 1.5}), _DISK_INTEGER),
        (_slot({"disk_id": True}), _DISK_INTEGER),
        (_slot({"disk_id": 0}), "device sda disk_id must be greater than 0"),
        (_slot({"volume_id": 0}), "device sda volume_id must be greater than 0"),
        (_slot({}), "device sda requires disk_id or volume_id"),
        (
            _slot({"disk_id": 111, "volume_id": 222}),
            "device sda can use disk_id or volume_id, not both",
        ),
        (_slot({"disk_id": 111}), ""),
        (_slot({"disk_id": 111.0}), ""),
    ],
)
def test_device_slots_walk_two_levels(arguments: dict[str, Any], expected: str) -> None:
    """The one walk that reads a second level: slots, then the members inside one.

    The Go twin is TestDeviceSlotsWalkTwoLevels.
    """
    assert walk(CONFIG_CREATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, ""),
        ({"devices": {}}, "devices must include at least one device slot"),
    ],
)
def test_the_update_accepts_an_absent_map(
    arguments: dict[str, Any], expected: str
) -> None:
    """The two config writes walk the same shapes.

    They differ only in whether the caller has to name a devices map at all.
    """
    assert walk(CONFIG_UPDATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"devices": None, "helpers": {"turbo": True}}, _DEVICES_REQUIRED),
        (
            {"devices": {"sda": {"disk_id": 111}}, "helpers": "distro"},
            "helpers must be a JSON object",
        ),
        (
            {"devices": {"sda": {"disk_id": 111}}, "helpers": {"turbo": True}},
            'invalid helpers JSON: unknown field "turbo"',
        ),
        ({"devices": {"sda": {"disk_id": 111}}, "helpers": {"distro": "yes"}}, ""),
    ],
)
def test_helpers_check_the_vocabulary_and_nothing_else(
    arguments: dict[str, Any], expected: str
) -> None:
    """The route reads every helper as a flag.

    The walk holds the names and never judges a value, which is what a key
    vocabulary with no value spec declares. The Go twin is
    TestHelpersCheckTheVocabularyAndNothingElse.
    """
    assert walk(CONFIG_CREATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (_interfaces("public"), ""),
        (_interfaces(["public"]), "interfaces must be an array of objects"),
        (_interfaces([{}]), _PURPOSE_CHOICE),
        (_interfaces([{"purpose": "carrier-pigeon"}]), _PURPOSE_CHOICE),
        (_interfaces([{"purpose": 7}]), _PURPOSE_CHOICE),
        (_interfaces([]), ""),
        (_interfaces([{"purpose": "vlan", "label": "vlan-1"}]), ""),
    ],
)
def test_interfaces_walk_a_legacy_list(
    arguments: dict[str, Any], expected: str
) -> None:
    """The members block that words no unknown key.

    An entry may carry anything beside the purpose it has to name. The Go twin
    is TestInterfacesWalkALegacyList.
    """
    assert walk(CONFIG_CREATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, ""),
        ({"ssh": "on"}, ""),
        (
            {"ssh": {"shell": "zsh", "agent": True}},
            "ssh contains unsupported fields: agent, shell",
        ),
        ({"ssh": {"access": "yes"}}, "ssh.access must be a boolean"),
        ({"ssh": {"access": None}}, "ssh.access must be a boolean"),
        ({"ssh": {"ip": "   "}}, "ssh.ip must be a non-empty string"),
        ({"ssh": {"ip": 7}}, "ssh.ip must be a non-empty string"),
        ({"ssh": {"port": 0}}, _SSH_PORT),
        ({"ssh": {"port": 65536}}, _SSH_PORT),
        ({"ssh": {"port": 22.5}}, _SSH_PORT),
        ({"ssh": {"port": True}}, _SSH_PORT),
        ({"ssh": {"port": None}}, ""),
        ({"ssh": {"access": True}}, ""),
        ({"ssh": {"port": 22}}, ""),
        ({"ssh": {"user": "u" * 33}}, _SSH_USER),
        ({"ssh": {"user": "u" * 32}}, ""),
        ({"ssh": {"user": None}}, ""),
    ],
)
def test_ssh_members_each_carry_their_own_kind(
    arguments: dict[str, Any], expected: str
) -> None:
    """The one walk whose members differ from each other.

    Two of them read a null as "clear this", which is what the API reads one
    as. The Go twin is TestSSHMembersEachCarryTheirOwnKind.
    """
    assert walk(SETTINGS_UPDATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, ""),
        ({"phone": "5551234"}, ""),
        ({"phone": {}}, "phone must include primary or secondary"),
        (
            {"phone": {"primary": " "}},
            "phone.primary must be a non-empty string or null",
        ),
        ({"phone": {"secondary": None}}, ""),
        ({"phone": {"primary": "5551234"}}, ""),
    ],
)
def test_phone_needs_one_number(arguments: dict[str, Any], expected: str) -> None:
    """The at-least-one arm at the top level, where no key sits above it.

    The Go twin is TestPhoneNeedsOneNumber.
    """
    assert walk(CONTACT_CREATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "default_firewall_ids is required"),
        (
            {"default_firewall_ids": 5},
            "default_firewall_ids must be a non-empty object",
        ),
        (
            {"default_firewall_ids": {}},
            "default_firewall_ids must be a non-empty object",
        ),
        (
            {"default_firewall_ids": {"volume": 5}},
            "default_firewall_ids contains unsupported key: volume",
        ),
        (
            {"default_firewall_ids": {"nodebalancer": 0}},
            "default_firewall_ids.nodebalancer must be a positive integer",
        ),
        (
            {"default_firewall_ids": {"linode": True}},
            "default_firewall_ids.linode must be a positive integer",
        ),
        ({"default_firewall_ids": {"linode": 9}}, ""),
        ({"default_firewall_ids": {"linode": 9.0}}, ""),
    ],
)
def test_firewall_assignments_share_one_member_spec(
    arguments: dict[str, Any], expected: str
) -> None:
    """Every key carries the same value, the shared-value form of a walk.

    The Go twin is TestFirewallAssignmentsShareOneMemberSpec.
    """
    assert walk(FIREWALL_UPDATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"linode": "read_only"}, _SECTION_SHAPE),
        ({"linode": ["read_only"]}, _SECTION_SHAPE),
        ({"linode": [{"permissions": "read_only"}]}, _SECTION_SHAPE),
        ({"linode": [{"id": 0, "permissions": "read_only"}]}, _SECTION_SHAPE),
        (
            {"linode": [{"id": 1, "permissions": "read_only", "label": "web"}]},
            _SECTION_SHAPE,
        ),
        (
            {"linode": [{"id": 1, "permissions": "admin"}]},
            "linode[0].permissions must be one of: read_only, read_write",
        ),
        (
            {
                "linode": [
                    {"id": 1, "permissions": "read_only"},
                    {"id": 2, "permissions": "admin"},
                ]
            },
            "linode[1].permissions must be one of: read_only, read_write",
        ),
        ({"linode": [{"id": 1, "permissions": None}]}, ""),
        ({"linode": [{"id": 1, "permissions": "read_write"}]}, ""),
    ],
)
def test_grant_sections_walk_each_entry(
    arguments: dict[str, Any], expected: str
) -> None:
    """The list form of a walk, where the index stands where a key does.

    One declaration serves all eleven sections, which is why every sentence
    names the field it is about. The Go twin is TestGrantSectionsWalkEachEntry.
    """
    assert walk(GRANTS_UPDATE, arguments) == expected


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (
            {"global": "read_only"},
            "global must be an object matching the grants schema",
        ),
        ({"global": {}}, "global must be an object matching the grants schema"),
        ({"global": {"add_pigeons": True}}, "global has unknown fields: add_pigeons"),
        ({"global": {"add_linodes": "yes"}}, "global.add_linodes must be a boolean"),
        (
            {"global": {"account_access": "admin"}},
            "global.account_access must be one of: read_only, read_write",
        ),
        (
            {"global": {"child_account_access": "yes"}},
            "global.child_account_access must be a boolean or null",
        ),
        ({"global": {"add_linodes": True}}, ""),
        ({"global": {"child_account_access": None}}, ""),
        ({"global": {"account_access": None}}, ""),
        ({"global": {"account_access": "read_only"}}, ""),
    ],
)
def test_global_grants_mix_shared_and_own_members(
    arguments: dict[str, Any], expected: str
) -> None:
    """One block declaring a shared value for twelve keys and two of its own.

    The Go twin is TestGlobalGrantsMixSharedAndOwnMembers.
    """
    assert walk(GRANTS_UPDATE, arguments) == expected
