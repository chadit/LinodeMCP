"""Proto-output field-shape tests for the instance action write tools.

These tools echo the request as proto-canonical JSON. The message-substring
assertions in test_tools.py miss field-name changes, so these pin the exact
output field names and values: a regression that drops a field or reverts to the
legacy dict shape is caught here. Byte-identical output with the Go side is the
point of the proto conversion, so these mirror
go/internal/tools/instance_action_proto_output_test.go.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.config import TwoStageConfig
from linodemcp.linode import parse_instance
from linodemcp.tools import (
    handle_linode_instance_backups_cancel,
    handle_linode_instance_backups_enable,
    handle_linode_instance_boot,
    handle_linode_instance_disk_password_reset,
    handle_linode_instance_migrate,
    handle_linode_instance_password_reset,
    handle_linode_instance_reboot,
    handle_linode_instance_rescue,
    handle_linode_instance_resize,
    handle_linode_instance_shutdown,
)
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable
    from unittest.mock import AsyncMock

    from linodemcp.config import Config

pytestmark = pytest.mark.asyncio

_STRONG_PASS = "Str0ngP@ssw0rd!"


async def _run(
    handler: Callable[[dict[str, Any], Config], Awaitable[list[Any]]],
    args: dict[str, Any],
    cfg: Config,
) -> dict[str, Any]:
    """Invoke a handler and decode its single text result into a dict."""
    result = await handler(args, cfg)
    assert len(result) == 1
    decoded: dict[str, Any] = json.loads(result[0].text)
    return decoded


async def test_boot_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.boot_instance.return_value = None
    data = await _run(
        handle_linode_instance_boot,
        {"instance_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Instance 123 boot initiated successfully"
    assert data["instance_id"] == 123
    # The power tools echo instance_id, not linode_id.
    assert "linode_id" not in data


async def test_reboot_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.reboot_instance.return_value = None
    data = await _run(
        handle_linode_instance_reboot,
        {"instance_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Instance 123 reboot initiated successfully"
    assert data["instance_id"] == 123
    assert "linode_id" not in data


async def test_shutdown_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.shutdown_instance.return_value = None
    data = await _run(
        handle_linode_instance_shutdown,
        {"instance_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Instance 123 shutdown initiated successfully"
    assert data["instance_id"] == 123
    assert "linode_id" not in data


async def test_migrate_proto_output_no_region(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.migrate_instance.return_value = None
    data = await _run(
        handle_linode_instance_migrate,
        {"linode_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Migration initiated for instance 123"
    assert data["linode_id"] == 123
    # region is explicit-presence, omitted when the caller lets Linode pick.
    assert "region" not in data
    mock_linode_client.migrate_instance.assert_called_once_with(123, region=None)


async def test_rescue_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.rescue_instance.return_value = None
    data = await _run(
        handle_linode_instance_rescue,
        {"linode_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Instance 123 is booting into rescue mode"
    assert data["linode_id"] == 123


async def test_resize_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.resize_instance.return_value = None
    data = await _run(
        handle_linode_instance_resize,
        {"instance_id": 123, "type": "g6-standard-1", "confirm": True},
        sample_config,
    )
    assert (
        data["message"] == "Instance 123 resize to g6-standard-1 initiated successfully"
    )
    assert data["instance_id"] == 123
    assert data["new_type"] == "g6-standard-1"


async def test_backups_enable_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.enable_instance_backups.return_value = None
    data = await _run(
        handle_linode_instance_backups_enable,
        {"linode_id": 123, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Backup service enabled for instance 123"
    assert data["linode_id"] == 123


async def test_backups_cancel_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.cancel_instance_backups.return_value = None
    data = await _run(
        handle_linode_instance_backups_cancel,
        {"linode_id": 123, "confirm": True},
        sample_config,
    )
    assert (
        data["message"]
        == "Backup service canceled for instance 123. All backups have been deleted."
    )
    assert data["linode_id"] == 123


async def test_password_reset_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.reset_instance_password.return_value = None
    data = await _run(
        handle_linode_instance_password_reset,
        {"linode_id": 123, "root_pass": _STRONG_PASS, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Root password reset for instance 123"
    assert data["linode_id"] == 123


async def test_disk_password_reset_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.reset_instance_disk_password.return_value = None
    data = await _run(
        handle_linode_instance_disk_password_reset,
        {"linode_id": 123, "disk_id": 10, "password": _STRONG_PASS, "confirm": True},
        sample_config,
    )
    assert data["message"] == "Password reset for disk 10 on instance 123"
    assert data["linode_id"] == 123
    assert data["disk_id"] == 10


async def test_migrate_proto_output_with_region(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.migrate_instance.return_value = None
    data = await _run(
        handle_linode_instance_migrate,
        {"linode_id": 123, "region": "us-east", "confirm": True},
        sample_config,
    )
    # A caller-picked region is named in the message and echoed in the field.
    assert data["message"] == "Migration initiated for instance 123 to region us-east"
    assert data["linode_id"] == 123
    assert data["region"] == "us-east"
    mock_linode_client.migrate_instance.assert_called_once_with(123, region="us-east")


async def test_password_reset_two_stage_apply_proto_output(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    """The two-stage apply body is proto-canonical, same shape as single-step."""
    mock_linode_client.get_instance.return_value = parse_instance(
        {
            "id": 123,
            "status": "offline",
            "updated": "2026-01-01T00:00:00",
        }
    )
    mock_linode_client.reset_instance_password.return_value = None
    sample_config.two_stage = TwoStageConfig(
        opt_in={"linode_instance_password_reset": True}
    )

    args: dict[str, Any] = {"linode_id": 123, "root_pass": _STRONG_PASS}
    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan = await handle_linode_instance_password_reset(
            {**args, "mode": "plan"}, sample_config
        )
        plan_id = json.loads(plan[0].text)["plan_id"]
        assert plan_id

        applied = await handle_linode_instance_password_reset(
            {**args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        data = json.loads(applied[0].text)
        assert data["message"] == "Root password reset for instance 123"
        assert data["linode_id"] == 123
    finally:
        reset_plan_store(token)


@pytest.mark.parametrize(
    ("handler", "extra"),
    [
        (handle_linode_instance_migrate, {}),
        (handle_linode_instance_rescue, {}),
        (handle_linode_instance_password_reset, {"root_pass": _STRONG_PASS}),
        (handle_linode_instance_backups_enable, {}),
        (handle_linode_instance_backups_cancel, {}),
    ],
)
async def test_invalid_linode_id_rejected(
    handler: Callable[[dict[str, Any], Config], Awaitable[list[Any]]],
    extra: dict[str, Any],
    sample_config: Config,
) -> None:
    """A non-integer linode_id is rejected before any API call, for every tool
    that parses it through the shared _parse_instance_id helper.
    """
    result = await handler(
        {"linode_id": "not-a-number", "confirm": True, **extra}, sample_config
    )
    assert len(result) == 1
    assert "linode_id must be a valid integer" in result[0].text
