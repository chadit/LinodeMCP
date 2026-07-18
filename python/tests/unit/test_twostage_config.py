"""Tests for the operator-tunable two-stage config (TTL + opt-in overrides).

Covers the Settings resolver directly and the end-to-end effect of a
``two_stage`` config block on the instance_delete handler. Mirrors the Go
TestSettings* and TestTwoStageConfig* tests.
"""

from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import TYPE_CHECKING

import pytest

from linodemcp.config import TwoStageConfig
from linodemcp.linode import parse_instance
from linodemcp.profiles import Capability
from linodemcp.tools.linode_instance_write import handle_linode_instance_delete
from linodemcp.twostage import (
    DEFAULT_PLAN_TTL,
    Settings,
    reset_plan_store,
    set_plan_store,
)
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

    from linodemcp.config import Config


@pytest.fixture(autouse=True)
def stub_instance_walk(mock_linode_client: AsyncMock) -> None:
    """Stub the volume and IP sub-fetches the instance plan-time walk makes so
    the walk runs cleanly; the config tests only care about plan TTL.
    """
    mock_linode_client.list_volumes.return_value = []
    mock_linode_client.list_instance_ips.return_value = {"ipv4": {"public": []}}


def test_settings_opted_in_honors_override() -> None:
    settings = Settings(opt_in={"linode_destroyer": False, "linode_writer_in": True})

    assert settings.opted_in("linode_destroyer", Capability.Destroy) is False
    assert settings.opted_in("linode_writer_in", Capability.Write) is True
    assert settings.opted_in("linode_other", Capability.Destroy) is True
    assert settings.opted_in("linode_other", Capability.Write) is False


def test_settings_plan_ttl_precedence() -> None:
    settings = Settings(
        default_ttl=timedelta(minutes=2),
        tool_ttl={
            "linode_slow": timedelta(minutes=30),
            "linode_zeroed": timedelta(0),
        },
    )

    assert settings.plan_ttl("linode_slow") == timedelta(minutes=30)
    assert settings.plan_ttl("linode_other") == timedelta(minutes=2)
    # A non-positive per-tool override falls back to the default.
    assert settings.plan_ttl("linode_zeroed") == timedelta(minutes=2)


def test_settings_defaults_match_builtin() -> None:
    settings = Settings()

    assert settings.plan_ttl("anything") == DEFAULT_PLAN_TTL
    assert settings.opted_in("anything", Capability.Destroy) is True


async def test_config_ttl_override_drives_plan_lifetime(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = parse_instance(
        {"id": 123, "status": "running"}
    )
    sample_config.two_stage = TwoStageConfig(default_plan_ttl_seconds=60)

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        body = json.loads(result[0].text)
        created = datetime.fromisoformat(body["created_at"])
        expires = datetime.fromisoformat(body["expires_at"])
        assert expires - created == timedelta(seconds=60)
    finally:
        reset_plan_store(token)


async def test_config_per_tool_ttl_override(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = parse_instance(
        {"id": 123, "status": "running"}
    )
    sample_config.two_stage = TwoStageConfig(
        tool_ttl_seconds={"linode_instance_delete": 120}
    )

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        body = json.loads(result[0].text)
        created = datetime.fromisoformat(body["created_at"])
        expires = datetime.fromisoformat(body["expires_at"])
        assert expires - created == timedelta(seconds=120)
    finally:
        reset_plan_store(token)


async def test_config_opt_out_falls_through(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance.return_value = parse_instance(
        {"id": 123, "status": "running"}
    )
    sample_config.two_stage = TwoStageConfig(opt_in={"linode_instance_delete": False})

    store = PlanStore()
    token = set_plan_store(store)
    try:
        result = await handle_linode_instance_delete(
            {"instance_id": 123, "mode": "plan"}, sample_config
        )
        # Opted out: two-stage returns None, the handler falls through to the
        # normal flow, which refuses without confirm. No plan is stored.
        assert await store.length() == 0
        mock_linode_client.delete_instance.assert_not_awaited()
        assert "confirm" in result[0].text.lower()
    finally:
        reset_plan_store(token)
