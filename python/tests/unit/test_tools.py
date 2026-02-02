"""Unit tests for MCP tools."""

from typing import Any
from unittest.mock import AsyncMock, patch

from linodemcp.config import Config
from linodemcp.linode import Alerts, Backups, Instance, Profile, Schedule, Specs
from linodemcp.tools import (
    handle_hello,
    handle_linode_instances_list,
    handle_linode_profile,
    handle_version,
)


async def test_handle_hello_with_name() -> None:
    """Test hello tool with name parameter."""
    result = await handle_hello({"name": "Alice"})
    assert len(result) == 1
    assert "Hello, Alice!" in result[0].text
    assert "LinodeMCP server is running" in result[0].text


async def test_handle_hello_without_name() -> None:
    """Test hello tool without name parameter."""
    result = await handle_hello({})
    assert len(result) == 1
    assert "Hello, World!" in result[0].text


async def test_handle_version() -> None:
    """Test version tool."""
    result = await handle_version({})
    assert len(result) == 1
    assert "version" in result[0].text.lower()
    assert "0.1.0" in result[0].text


async def test_handle_linode_profile(
    sample_config: Config, sample_profile_data: dict[str, Any]
) -> None:
    """Test linode_profile tool."""
    mock_profile = Profile(
        username=sample_profile_data["username"],
        email=sample_profile_data["email"],
        timezone=sample_profile_data["timezone"],
        email_notifications=sample_profile_data["email_notifications"],
        restricted=sample_profile_data["restricted"],
        two_factor_auth=sample_profile_data["two_factor_auth"],
        uid=sample_profile_data["uid"],
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile({}, sample_config)

        assert len(result) == 1
        assert "testuser" in result[0].text
        assert "test@example.com" in result[0].text


async def test_handle_linode_profile_with_environment(sample_config: Config) -> None:
    """Test linode_profile tool with environment parameter."""
    mock_profile = Profile(
        username="envuser",
        email="env@example.com",
        timezone="UTC",
        email_notifications=True,
        restricted=False,
        two_factor_auth=False,
        uid=99999,
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_profile.return_value = mock_profile
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_profile({"environment": "default"}, sample_config)

        assert len(result) == 1
        assert "envuser" in result[0].text


async def test_handle_linode_profile_missing_environment(sample_config: Config) -> None:
    """Test linode_profile tool with missing environment."""
    result = await handle_linode_profile({"environment": "nonexistent"}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "error" in result[0].text


async def test_handle_linode_instances_list(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instances_list tool."""
    mock_instance = Instance(
        id=sample_instance_data["id"],
        label=sample_instance_data["label"],
        status=sample_instance_data["status"],
        type=sample_instance_data["type"],
        region=sample_instance_data["region"],
        image=sample_instance_data["image"],
        ipv4=sample_instance_data["ipv4"],
        ipv6=sample_instance_data["ipv6"],
        hypervisor=sample_instance_data["hypervisor"],
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(**sample_instance_data["backups"]["schedule"]),
            last_successful=None,
        ),
        created=sample_instance_data["created"],
        updated=sample_instance_data["updated"],
        group=sample_instance_data["group"],
        tags=sample_instance_data["tags"],
        watchdog_enabled=sample_instance_data["watchdog_enabled"],
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = [mock_instance]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list({}, sample_config)

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "123456" in result[0].text
        assert "running" in result[0].text


async def test_handle_linode_instances_list_with_status_filter(
    sample_config: Config,
    sample_instance_data: dict[str, Any],
) -> None:
    """Test linode_instances_list tool with status filter."""
    running_instance = Instance(
        id=123456,
        label="running-instance",
        status="running",
        type="g6-standard-1",
        region="us-east",
        image="linode/ubuntu22.04",
        ipv4=["192.0.2.1"],
        ipv6="2001:db8::1/64",
        hypervisor="kvm",
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=True,
            available=True,
            schedule=Schedule(day="Saturday", window="W22"),
            last_successful=None,
        ),
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
        group="production",
        tags=["web"],
        watchdog_enabled=True,
    )

    stopped_instance = Instance(
        id=789012,
        label="stopped-instance",
        status="stopped",
        type="g6-standard-1",
        region="us-east",
        image="linode/ubuntu22.04",
        ipv4=["192.0.2.2"],
        ipv6="2001:db8::2/64",
        hypervisor="kvm",
        specs=Specs(**sample_instance_data["specs"]),
        alerts=Alerts(**sample_instance_data["alerts"]),
        backups=Backups(
            enabled=True,
            available=True,
            schedule=Schedule(day="Saturday", window="W22"),
            last_successful=None,
        ),
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
        group="staging",
        tags=["test"],
        watchdog_enabled=False,
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.return_value = [running_instance, stopped_instance]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list(
            {"status": "running"}, sample_config
        )

        assert len(result) == 1
        assert "running-instance" in result[0].text
        assert "stopped-instance" not in result[0].text
        assert '"count": 1' in result[0].text
        assert "status=running" in result[0].text


async def test_handle_linode_instances_list_error(sample_config: Config) -> None:
    """Test linode_instances_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_instances.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instances_list({}, sample_config)

        assert len(result) == 1
        assert (
            "Failed to retrieve" in result[0].text or "error" in result[0].text.lower()
        )
