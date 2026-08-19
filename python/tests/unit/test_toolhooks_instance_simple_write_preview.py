"""The previews and the address check the three simple instance writes own.

Every rule these tools are refused by is declared on their input messages and
evaluated by ``linodemcp.tools.constraints``. What is left here is what a rule
cannot see: an address that parses or does not, and an rdns whose explicit null
is the API's spelling for "clear it" rather than an omitted argument. The Go
twin is ``instance_simple_write_preview_test.go`` in ``go/internal/toolhooks``.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

IP_PROBE_ADDRESS = "192.0.2.10"
INSTANCE_PATH = "/linode/instances/123"
DISK_PATH = "/linode/instances/123/disks/5"
IP_UPDATE_PATH = f"/linode/instances/123/ips/{IP_PROBE_ADDRESS}"
RDNS_HOST = "host.example.com"
NEW_LABEL = "newlabel"


def _stub_client(**answers: Any) -> AsyncMock:
    """A client whose read methods answer one canned body each."""
    client = AsyncMock()
    for method, body in answers.items():
        getattr(client, method).return_value = body
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None

    return client


CLONE_EFFECT = (
    'Disk "boot-disk" (25600 MB) is cloned to a new disk on the same instance, '
    "consuming 25600 MB of additional storage."
)
RESET_EFFECT = "The root password for disk 5 on instance 123 will be reset."
RESET_WARNING = "Existing disk root password access will be replaced."


async def test_instance_disk_clone_preview_names_the_storage_the_copy_takes(
    sample_config: Config,
) -> None:
    """The clone preview owes the caller the disk it copies and the storage the
    copy consumes, which only the fetched size can say.

    The Go twin is
    TestLinodeInstanceDiskClonePreviewNamesTheStorageTheCopyTakes.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client(
            get_instance_disk={"id": 5, "label": "boot-disk", "size": 25600}
        )
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_instance_disk_clone_preview(
            sample_config,
            {"linode_id": 123, "disk_id": 5, "dry_run": True},
            "POST",
            f"{DISK_PATH}/clone",
            {},
        )

    preview = json.loads(result[0].text)

    assert preview["tool"] == "linode_instance_disk_clone"
    assert preview["current_state"]["label"] == "boot-disk"
    assert preview["side_effects"] == [CLONE_EFFECT]
    mock_client.get_instance_disk.assert_awaited_once_with(123, 5)


async def test_instance_disk_clone_preview_survives_a_disk_shaped_like_nothing(
    sample_config: Config,
) -> None:
    """A read that answered with something other than a disk still names the
    copy, with the label and size it could not find left empty.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_cls.return_value = _stub_client(get_instance_disk=None)

        result = await toolhooks.linode_instance_disk_clone_preview(
            sample_config,
            {"linode_id": 123, "disk_id": 5, "dry_run": True},
            "POST",
            f"{DISK_PATH}/clone",
            {},
        )

    preview = json.loads(result[0].text)

    assert preview["side_effects"] == [
        (
            'Disk "" (0 MB) is cloned to a new disk on the same instance, '
            "consuming 0 MB of additional storage."
        )
    ]
