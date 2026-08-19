"""The checks and previews the config-profile and disk write tools own.

Every rule these four tools are refused by is declared on their input messages
and evaluated by ``linodemcp.tools.constraints``. What is left here is what a
rule cannot see: an open device-slot map, the boot-helper toggles, the legacy
interface purposes, and the one thing a config update alone cannot do, which is
name nothing to change. The Go twins are ``instance_config_write_test.go`` and
``instance_disk_write_test.go`` in ``go/internal/toolhooks``.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

CONFIGS_PATH = "/linode/instances/123/configs"
DISKS_PATH = "/linode/instances/123/disks"
DISK_PATH = "/linode/instances/123/disks/5"
SLOT_SDA = "sda"
SLOT_UNKNOWN = "sdz"
PURPOSE_SENTENCE = "must be one of: public, vlan, vpc"
UPDATE_NO_FIELDS = "at least one configuration field must be provided"
CONFIG_UPDATE_FIELDS = (
    "label",
    "devices",
    "kernel",
    "comments",
    "memory_limit",
    "root_device",
    "run_level",
    "virt_mode",
    "helpers",
    "interfaces",
)


def _devices() -> dict[str, Any]:
    """The one device slot a valid create carries."""
    return {SLOT_SDA: {"disk_id": 111}}


def _stub_client(**answers: Any) -> AsyncMock:
    """A client whose read methods answer one canned body each."""
    client = AsyncMock()
    for method, body in answers.items():
        getattr(client, method).return_value = body
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None

    return client


_CREATE_CASES: list[tuple[dict[str, Any], str]] = [
    ({}, "devices is required"),
    ({"devices": SLOT_SDA}, "devices must be a JSON object"),
    ({"devices": {}}, "devices must include at least one device slot"),
    (
        {"devices": {SLOT_SDA: {"disk_id": 111, "no_such_member": 1}}},
        'invalid devices JSON: unknown field "no_such_member"',
    ),
    (
        {"devices": {SLOT_SDA: {"disk_id": "111"}}},
        "invalid devices JSON: sda.disk_id must be an integer",
    ),
    (
        {"devices": {SLOT_UNKNOWN: {"disk_id": 111}}},
        "device slot sdz must be one of sda through sdh",
    ),
    ({"devices": {SLOT_SDA: None}}, "device sda must be an object"),
    ({"devices": {SLOT_SDA: {}}}, "device sda requires disk_id or volume_id"),
    (
        {"devices": {SLOT_SDA: {"disk_id": 0}}},
        "device sda disk_id must be greater than 0",
    ),
    (
        {"devices": {SLOT_SDA: {"volume_id": 0}}},
        "device sda volume_id must be greater than 0",
    ),
    (
        {"devices": {SLOT_SDA: {"disk_id": 111, "volume_id": 222}}},
        "device sda can use disk_id or volume_id, not both",
    ),
    # The slot reported is the first in sorted order, so the answer is the
    # same every run and the same one the Go hook gives.
    (
        {"devices": {"sdy": {"disk_id": 111}, SLOT_UNKNOWN: {"disk_id": 111}}},
        "device slot sdy must be one of sda through sdh",
    ),
    (
        {"devices": _devices(), "helpers": "distro"},
        "helpers must be a JSON object",
    ),
    (
        {"devices": _devices(), "helpers": {"turbo": True}},
        'invalid helpers JSON: unknown field "turbo"',
    ),
    (
        {"devices": _devices(), "interfaces": [{}]},
        f"interfaces[0].purpose {PURPOSE_SENTENCE}",
    ),
    (
        {
            "devices": _devices(),
            "interfaces": [{"purpose": "public"}, {"purpose": "carrier"}],
        },
        f"interfaces[1].purpose {PURPOSE_SENTENCE}",
    ),
    (
        {"devices": _devices(), "interfaces": ["public"]},
        "interfaces must be an array of objects",
    ),
    # The body reader answers the shape, so the hook holds its peace and
    # lets the one sentence a caller reads come from there.
    ({"devices": _devices(), "interfaces": "public"}, ""),
    (
        {
            "devices": _devices(),
            "helpers": {"distro": True},
            "interfaces": [{"purpose": "vlan"}],
        },
        "",
    ),
]


_UPDATE_CASES: list[tuple[dict[str, Any], str]] = [
    ({}, UPDATE_NO_FIELDS),
    ({"kernel": "linode/latest-64bit"}, ""),
    ({"devices": {}}, "devices must include at least one device slot"),
    (
        {"devices": {SLOT_UNKNOWN: {"disk_id": 111}}},
        "device slot sdz must be one of sda through sdh",
    ),
    ({"helpers": {"turbo": True}}, 'invalid helpers JSON: unknown field "turbo"'),
    (
        {"interfaces": [{"purpose": "carrier"}]},
        f"interfaces[0].purpose {PURPOSE_SENTENCE}",
    ),
]


_RESIZE_CASES: list[tuple[dict[str, Any], str]] = [
    (
        {"id": 5, "label": "boot-disk", "size": 1024},
        "Disk resizes from 1024 MB to 2048 MB.",
    ),
    ({"id": 5, "label": "boot-disk"}, "Disk resizes to 2048 MB."),
]


@pytest.mark.parametrize(("state", "expected"), _RESIZE_CASES)
async def test_disk_resize_preview_names_the_size_it_starts_from(
    sample_config: Config, state: dict[str, Any], expected: str
) -> None:
    """A read that answered with nothing leaves the starting size out rather
    than naming a zero.

    The Go twin is TestLinodeInstanceDiskResizePreviewNamesTheSizeItStartsFrom.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_cls.return_value = _stub_client(get_instance_disk=state)

        result = await toolhooks.linode_instance_disk_resize_preview(
            sample_config,
            {"linode_id": 123, "disk_id": 5, "size": 2048, "dry_run": True},
            "POST",
            f"{DISK_PATH}/resize",
            {"size": 2048},
        )

    preview = json.loads(result[0].text)

    assert preview["tool"] == "linode_instance_disk_resize"
    assert preview["side_effects"] == [expected]
    assert preview["warnings"] == ["The instance must be powered off to resize a disk."]
