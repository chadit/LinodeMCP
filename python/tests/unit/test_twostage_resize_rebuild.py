"""Two-stage tests for the resize/rebuild tier.

instance_rebuild is CapDestroy, so it opts in by default. instance_resize is
CapWrite, so it stays single-step until an operator enables it via the
two_stage config. Both carry their dependency walk into the plan body.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from linodemcp.config import TwoStageConfig
from linodemcp.tools.linode_instance_actions import handle_linode_instance_rebuild
from linodemcp.tools.linode_instance_write import handle_linode_instance_resize
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

    from linodemcp.config import Config


async def test_rebuild_plan_then_apply(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = {"id": 123, "status": "offline"}
    mock_linode_client.list_instance_disks.return_value = []

    rebuild_args: dict[str, Any] = {
        "linode_id": 123,
        "image": "linode/ubuntu24.04",
        "root_pass": "Abcdefgh1234",
    }

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan = await handle_linode_instance_rebuild(
            {**rebuild_args, "mode": "plan"}, sample_config
        )
        body = json.loads(plan[0].text)
        plan_id = body["plan_id"]
        assert plan_id
        # The rebuild walk runs at plan time, so the body reads like a preview.
        assert body["warnings"]
        mock_linode_client.rebuild_instance.assert_not_awaited()

        result = await handle_linode_instance_rebuild(
            {**rebuild_args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "Error" not in result[0].text
        mock_linode_client.rebuild_instance.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


async def test_resize_plan_then_apply_opted_in(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = {"id": 123, "type": "g6-nanode-1"}
    mock_linode_client.list_instance_disks.return_value = []
    sample_config.two_stage = TwoStageConfig(opt_in={"linode_instance_resize": True})

    resize_args: dict[str, Any] = {"instance_id": 123, "type": "g6-standard-1"}

    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan = await handle_linode_instance_resize(
            {**resize_args, "mode": "plan"}, sample_config
        )
        body = json.loads(plan[0].text)
        plan_id = body["plan_id"]
        assert plan_id
        # The resize walk's side-effect names both the current and target type.
        effect = body["side_effects"][0]
        assert "g6-nanode-1" in effect
        assert "g6-standard-1" in effect
        mock_linode_client.resize_instance.assert_not_awaited()

        result = await handle_linode_instance_resize(
            {**resize_args, "mode": "apply", "plan_id": plan_id}, sample_config
        )
        assert "Error" not in result[0].text
        # The apply body is proto-canonical, same shape as the single-step path.
        apply_body = json.loads(result[0].text)
        assert (
            apply_body["message"]
            == "Instance 123 resize to g6-standard-1 initiated successfully"
        )
        assert apply_body["instance_id"] == 123
        assert apply_body["new_type"] == "g6-standard-1"
        mock_linode_client.resize_instance.assert_awaited_once()
        assert await store.length() == 0
    finally:
        reset_plan_store(token)


async def test_resize_default_off_falls_through(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    # Without a config opt-in, a mode:"plan" resize call must NOT produce a plan;
    # CapWrite does not opt in by default.
    mock_linode_client.get_instance.return_value = {"id": 123, "type": "g6-nanode-1"}
    mock_linode_client.list_instance_disks.return_value = []

    store = PlanStore()
    token = set_plan_store(store)
    try:
        await handle_linode_instance_resize(
            {"instance_id": 123, "type": "g6-standard-1", "mode": "plan"},
            sample_config,
        )
        assert await store.length() == 0
        mock_linode_client.resize_instance.assert_not_awaited()
    finally:
        reset_plan_store(token)
