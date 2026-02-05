"""Unit tests for MCP tools."""

from typing import Any
from unittest.mock import AsyncMock, patch

from linodemcp.config import Config
from linodemcp.linode import (
    UDF,
    Account,
    Addons,
    Alerts,
    Backups,
    BackupsAddon,
    Domain,
    DomainRecord,
    Firewall,
    FirewallAddresses,
    FirewallRule,
    FirewallRules,
    Image,
    Instance,
    InstanceType,
    NodeBalancer,
    Price,
    Profile,
    Region,
    Resolver,
    Schedule,
    Specs,
    SSHKey,
    StackScript,
    Transfer,
    Volume,
)
from linodemcp.tools import (
    handle_hello,
    handle_linode_account,
    handle_linode_domain_get,
    handle_linode_domain_records_list,
    handle_linode_domains_list,
    handle_linode_firewalls_list,
    handle_linode_images_list,
    handle_linode_instance_get,
    handle_linode_instances_list,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancers_list,
    handle_linode_profile,
    handle_linode_regions_list,
    handle_linode_sshkeys_list,
    handle_linode_stackscripts_list,
    handle_linode_types_list,
    handle_linode_volumes_list,
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


# Stage 2 Tool Tests


async def test_handle_linode_instance_get(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_get tool."""
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
        mock_client.get_instance.return_value = mock_instance
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": "123456"}, sample_config
        )

        assert len(result) == 1
        assert "test-instance" in result[0].text
        assert "running" in result[0].text
        mock_client.get_instance.assert_called_once_with(123456)


async def test_handle_linode_instance_get_missing_id(sample_config: Config) -> None:
    """Test linode_instance_get tool with missing ID."""
    result = await handle_linode_instance_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_instance_get_invalid_id(sample_config: Config) -> None:
    """Test linode_instance_get tool with invalid ID."""
    result = await handle_linode_instance_get(
        {"instance_id": "not-a-number"}, sample_config
    )

    assert len(result) == 1
    assert "Error" in result[0].text or "integer" in result[0].text.lower()


async def test_handle_linode_account(sample_config: Config) -> None:
    """Test linode_account tool."""
    mock_account = Account(
        first_name="Test",
        last_name="User",
        email="test@example.com",
        company="TestCo",
        address_1="123 Test St",
        address_2="",
        city="Test City",
        state="TS",
        zip="12345",
        country="US",
        phone="555-1234",
        balance=100.50,
        balance_uninvoiced=50.25,
        capabilities=["Linodes", "Block Storage"],
        active_since="2020-01-01T00:00:00",
        euuid="abcd-1234",
        billing_source="linode",
        active_promotions=[],
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account.return_value = mock_account
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account({}, sample_config)

        assert len(result) == 1
        assert "Test" in result[0].text
        assert "test@example.com" in result[0].text
        mock_client.get_account.assert_called_once()


async def test_handle_linode_regions_list(sample_config: Config) -> None:
    """Test linode_regions_list tool."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes", "Block Storage"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({}, sample_config)

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "eu-west" in result[0].text
        mock_client.list_regions.assert_called_once()


async def test_handle_linode_regions_list_filter_country(sample_config: Config) -> None:
    """Test linode_regions_list tool with country filter."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="us-west",
            label="Fremont, CA",
            country="us",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.3", ipv6="2001:db8::3"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({"country": "us"}, sample_config)

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "us-west" in result[0].text
        assert "eu-west" not in result[0].text
        assert '"count": 2' in result[0].text


async def test_handle_linode_types_list(sample_config: Config) -> None:
    """Test linode_types_list tool."""
    mock_types = [
        InstanceType(
            id="g6-nanode-1",
            label="Nanode 1GB",
            class_="nanode",
            disk=25600,
            memory=1024,
            vcpus=1,
            gpus=0,
            network_out=1000,
            transfer=1000,
            price=Price(hourly=0.0075, monthly=5.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
            successor=None,
        ),
        InstanceType(
            id="g6-standard-2",
            label="Linode 4GB",
            class_="standard",
            disk=81920,
            memory=4096,
            vcpus=2,
            gpus=0,
            network_out=4000,
            transfer=4000,
            price=Price(hourly=0.03, monthly=20.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.008, monthly=5.0))),
            successor=None,
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({}, sample_config)

        assert len(result) == 1
        assert "g6-nanode-1" in result[0].text
        assert "g6-standard-2" in result[0].text
        mock_client.list_types.assert_called_once()


async def test_handle_linode_types_list_filter_class(sample_config: Config) -> None:
    """Test linode_types_list tool with class filter."""
    mock_types = [
        InstanceType(
            id="g6-nanode-1",
            label="Nanode 1GB",
            class_="nanode",
            disk=25600,
            memory=1024,
            vcpus=1,
            gpus=0,
            network_out=1000,
            transfer=1000,
            price=Price(hourly=0.0075, monthly=5.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
            successor=None,
        ),
        InstanceType(
            id="g6-standard-2",
            label="Linode 4GB",
            class_="standard",
            disk=81920,
            memory=4096,
            vcpus=2,
            gpus=0,
            network_out=4000,
            transfer=4000,
            price=Price(hourly=0.03, monthly=20.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.008, monthly=5.0))),
            successor=None,
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({"class": "standard"}, sample_config)

        assert len(result) == 1
        assert "g6-standard-2" in result[0].text
        assert "g6-nanode-1" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_volumes_list(sample_config: Config) -> None:
    """Test linode_volumes_list tool."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="eu-west",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=["backup"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" in result[0].text
        mock_client.list_volumes.assert_called_once()


async def test_handle_linode_volumes_list_filter_region(sample_config: Config) -> None:
    """Test linode_volumes_list tool with region filter."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="eu-west",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({"region": "us-east"}, sample_config)

        assert len(result) == 1
        assert "data-vol" in result[0].text
        assert "backup-vol" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_images_list(sample_config: Config) -> None:
    """Test linode_images_list tool."""
    mock_images = [
        Image(
            id="linode/ubuntu22.04",
            label="Ubuntu 22.04",
            description="Ubuntu 22.04 LTS",
            type="manual",
            is_public=True,
            deprecated=False,
            size=2500,
            vendor="linode",
            status="available",
            created="2022-04-21T00:00:00",
            created_by="linode",
            expiry=None,
            eol=None,
            capabilities=["cloud-init"],
            tags=[],
        ),
        Image(
            id="private/12345",
            label="Custom Image",
            description="My custom image",
            type="manual",
            is_public=False,
            deprecated=False,
            size=5000,
            vendor="",
            status="available",
            created="2024-01-01T00:00:00",
            created_by="user@example.com",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=["custom"],
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.return_value = mock_images
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list({}, sample_config)

        assert len(result) == 1
        assert "linode/ubuntu22.04" in result[0].text
        assert "private/12345" in result[0].text
        mock_client.list_images.assert_called_once()


async def test_handle_linode_images_list_filter_public(sample_config: Config) -> None:
    """Test linode_images_list tool with is_public filter."""
    mock_images = [
        Image(
            id="linode/ubuntu22.04",
            label="Ubuntu 22.04",
            description="Ubuntu 22.04 LTS",
            type="manual",
            is_public=True,
            deprecated=False,
            size=2500,
            vendor="linode",
            status="available",
            created="2022-04-21T00:00:00",
            created_by="linode",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=[],
        ),
        Image(
            id="private/12345",
            label="Custom Image",
            description="My custom image",
            type="manual",
            is_public=False,
            deprecated=False,
            size=5000,
            vendor="",
            status="available",
            created="2024-01-01T00:00:00",
            created_by="user@example.com",
            expiry=None,
            eol=None,
            capabilities=[],
            tags=[],
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.return_value = mock_images
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list(
            {"is_public": "false"}, sample_config
        )

        assert len(result) == 1
        assert "private/12345" in result[0].text
        assert "linode/ubuntu22.04" not in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_account_error(sample_config: Config) -> None:
    """Test linode_account tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_account.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_account({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_regions_list_error(sample_config: Config) -> None:
    """Test linode_regions_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_types_list_error(sample_config: Config) -> None:
    """Test linode_types_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_types.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_types_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_error(sample_config: Config) -> None:
    """Test linode_volumes_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_images_list_error(sample_config: Config) -> None:
    """Test linode_images_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_images.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_images_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_instance_get_error(sample_config: Config) -> None:
    """Test linode_instance_get tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_instance.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_get(
            {"instance_id": "123456"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_volumes_list_filter_label(sample_config: Config) -> None:
    """Test linode_volumes_list tool with label filter."""
    mock_volumes = [
        Volume(
            id=1,
            label="data-vol",
            status="active",
            size=100,
            region="us-east",
            linode_id=123,
            linode_label="test-instance",
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=2,
            label="backup-vol",
            status="active",
            size=50,
            region="us-east",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_backup-vol",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
        Volume(
            id=3,
            label="data-backup",
            status="active",
            size=75,
            region="us-east",
            linode_id=None,
            linode_label=None,
            filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data-backup",
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            hardware_type="hdd",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_volumes.return_value = mock_volumes
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volumes_list(
            {"label_contains": "backup"}, sample_config
        )

        assert len(result) == 1
        assert "backup-vol" in result[0].text
        assert "data-backup" in result[0].text
        assert '"count": 2' in result[0].text


async def test_handle_linode_regions_list_filter_capability(
    sample_config: Config,
) -> None:
    """Test linode_regions_list tool with capability filter."""
    mock_regions = [
        Region(
            id="us-east",
            label="Newark, NJ",
            country="us",
            capabilities=["Linodes", "Block Storage"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
            site_type="core",
        ),
        Region(
            id="eu-west",
            label="London, UK",
            country="uk",
            capabilities=["Linodes"],
            status="ok",
            resolvers=Resolver(ipv4="192.0.2.2", ipv6="2001:db8::2"),
            site_type="core",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_regions.return_value = mock_regions
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_regions_list(
            {"capability": "Block Storage"}, sample_config
        )

        assert len(result) == 1
        assert "us-east" in result[0].text
        assert "eu-west" not in result[0].text
        assert '"count": 1' in result[0].text


# Stage 3 Tool Tests


async def test_handle_linode_sshkeys_list(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool."""
    mock_keys = [
        SSHKey(
            id=1,
            label="work-laptop",
            ssh_key="ssh-rsa AAAA... user@work",
            created="2024-01-01T00:00:00",
        ),
        SSHKey(
            id=2,
            label="home-desktop",
            ssh_key="ssh-rsa BBBB... user@home",
            created="2024-01-02T00:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list({}, sample_config)

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" in result[0].text
        mock_client.list_ssh_keys.assert_called_once()


async def test_handle_linode_sshkeys_list_filter_label(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool with label filter."""
    mock_keys = [
        SSHKey(
            id=1,
            label="work-laptop",
            ssh_key="ssh-rsa AAAA... user@work",
            created="2024-01-01T00:00:00",
        ),
        SSHKey(
            id=2,
            label="home-desktop",
            ssh_key="ssh-rsa BBBB... user@home",
            created="2024-01-02T00:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list(
            {"label_contains": "work"}, sample_config
        )

        assert len(result) == 1
        assert "work-laptop" in result[0].text
        assert "home-desktop" not in result[0].text


async def test_handle_linode_sshkeys_list_error(sample_config: Config) -> None:
    """Test linode_sshkeys_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_ssh_keys.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkeys_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domains_list(sample_config: Config) -> None:
    """Test linode_domains_list tool."""
    mock_domains = [
        Domain(
            id=1,
            domain="example.com",
            type="master",
            status="active",
            soa_email="admin@example.com",
            description="Main domain",
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        Domain(
            id=2,
            domain="test.com",
            type="master",
            status="active",
            soa_email="admin@test.com",
            description="Test domain",
            tags=["staging"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domains.return_value = mock_domains
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domains_list({}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        assert "test.com" in result[0].text
        mock_client.list_domains.assert_called_once()


async def test_handle_linode_domains_list_error(sample_config: Config) -> None:
    """Test linode_domains_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domains.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domains_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_get(sample_config: Config) -> None:
    """Test linode_domain_get tool."""
    mock_domain = Domain(
        id=1,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="Main domain",
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "example.com" in result[0].text
        mock_client.get_domain.assert_called_once_with(1)


async def test_handle_linode_domain_get_missing_id(sample_config: Config) -> None:
    """Test linode_domain_get tool with missing ID."""
    result = await handle_linode_domain_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_get_error(sample_config: Config) -> None:
    """Test linode_domain_get tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_domain.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_get({"domain_id": 1}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_domain_records_list(sample_config: Config) -> None:
    """Test linode_domain_records_list tool."""
    mock_records = [
        DomainRecord(
            id=1,
            type="A",
            name="www",
            target="192.0.2.1",
            priority=0,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        DomainRecord(
            id=2,
            type="MX",
            name="",
            target="mail.example.com",
            priority=10,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.return_value = mock_records
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "mail.example.com" in result[0].text
        mock_client.list_domain_records.assert_called_once_with(1)


async def test_handle_linode_domain_records_list_filter_type(
    sample_config: Config,
) -> None:
    """Test linode_domain_records_list tool with type filter."""
    mock_records = [
        DomainRecord(
            id=1,
            type="A",
            name="www",
            target="192.0.2.1",
            priority=0,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        DomainRecord(
            id=2,
            type="MX",
            name="",
            target="mail.example.com",
            priority=10,
            weight=0,
            port=0,
            ttl_sec=300,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.return_value = mock_records
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1, "type": "A"}, sample_config
        )

        assert len(result) == 1
        assert "192.0.2.1" in result[0].text
        assert "mail.example.com" not in result[0].text


async def test_handle_linode_domain_records_list_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_domain_records_list tool with missing domain_id."""
    result = await handle_linode_domain_records_list({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_domain_records_list_error(sample_config: Config) -> None:
    """Test linode_domain_records_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_domain_records.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_records_list(
            {"domain_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_firewalls_list(sample_config: Config) -> None:
    """Test linode_firewalls_list tool."""
    mock_firewalls = [
        Firewall(
            id=1,
            label="web-firewall",
            status="enabled",
            rules=FirewallRules(
                inbound=[
                    FirewallRule(
                        action="ACCEPT",
                        protocol="TCP",
                        ports="80,443",
                        addresses=FirewallAddresses(
                            ipv4=["0.0.0.0/0"], ipv6=["::/0"]
                        ),
                        label="HTTP/HTTPS",
                        description="Allow web traffic",
                    )
                ],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list({}, sample_config)

        assert len(result) == 1
        assert "web-firewall" in result[0].text
        mock_client.list_firewalls.assert_called_once()


async def test_handle_linode_firewalls_list_filter_status(
    sample_config: Config,
) -> None:
    """Test linode_firewalls_list tool with status filter."""
    mock_firewalls = [
        Firewall(
            id=1,
            label="enabled-fw",
            status="enabled",
            rules=FirewallRules(
                inbound=[],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        Firewall(
            id=2,
            label="disabled-fw",
            status="disabled",
            rules=FirewallRules(
                inbound=[],
                outbound=[],
                inbound_policy="DROP",
                outbound_policy="ACCEPT",
            ),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.return_value = mock_firewalls
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list(
            {"status": "enabled"}, sample_config
        )

        assert len(result) == 1
        assert "enabled-fw" in result[0].text
        assert "disabled-fw" not in result[0].text


async def test_handle_linode_firewalls_list_error(sample_config: Config) -> None:
    """Test linode_firewalls_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_firewalls.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewalls_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancers_list(sample_config: Config) -> None:
    """Test linode_nodebalancers_list tool."""
    mock_nodebalancers = [
        NodeBalancer(
            id=1,
            label="web-lb",
            hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
            ipv4="192.0.2.1",
            ipv6="2001:db8::1",
            region="us-east",
            client_conn_throttle=0,
            transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
            tags=["production"],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.return_value = mock_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list({}, sample_config)

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.list_nodebalancers.assert_called_once()


async def test_handle_linode_nodebalancers_list_filter_region(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancers_list tool with region filter."""
    mock_nodebalancers = [
        NodeBalancer(
            id=1,
            label="us-lb",
            hostname="nb-1.newark.nodebalancer.linode.com",
            ipv4="192.0.2.1",
            ipv6="2001:db8::1",
            region="us-east",
            client_conn_throttle=0,
            transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
        NodeBalancer(
            id=2,
            label="eu-lb",
            hostname="nb-2.london.nodebalancer.linode.com",
            ipv4="192.0.2.2",
            ipv6="2001:db8::2",
            region="eu-west",
            client_conn_throttle=0,
            transfer=Transfer(in_=500.0, out=1000.0, total=1500.0),
            tags=[],
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.return_value = mock_nodebalancers
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list(
            {"region": "us-east"}, sample_config
        )

        assert len(result) == 1
        assert "us-lb" in result[0].text
        assert "eu-lb" not in result[0].text


async def test_handle_linode_nodebalancers_list_error(sample_config: Config) -> None:
    """Test linode_nodebalancers_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_nodebalancers.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancers_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_nodebalancer_get(sample_config: Config) -> None:
    """Test linode_nodebalancer_get tool."""
    mock_nodebalancer = NodeBalancer(
        id=1,
        label="web-lb",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2001:db8::1",
        region="us-east",
        client_conn_throttle=0,
        transfer=Transfer(in_=1000.0, out=2000.0, total=3000.0),
        tags=["production"],
        created="2024-01-01T00:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "web-lb" in result[0].text
        mock_client.get_nodebalancer.assert_called_once_with(1)


async def test_handle_linode_nodebalancer_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_get tool with missing ID."""
    result = await handle_linode_nodebalancer_get({}, sample_config)

    assert len(result) == 1
    assert "Error" in result[0].text or "required" in result[0].text.lower()


async def test_handle_linode_nodebalancer_get_error(sample_config: Config) -> None:
    """Test linode_nodebalancer_get tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_nodebalancer.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_get(
            {"nodebalancer_id": 1}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()


async def test_handle_linode_stackscripts_list(sample_config: Config) -> None:
    """Test linode_stackscripts_list tool."""
    mock_stackscripts = [
        StackScript(
            id=1,
            username="testuser",
            user_gravatar_id="abc123",
            label="my-script",
            description="Test script",
            images=["linode/ubuntu22.04"],
            deployments_total=10,
            deployments_active=5,
            is_public=False,
            mine=True,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash\necho hello",
            user_defined_fields=[
                UDF(
                    label="Username",
                    name="username",
                    example="admin",
                    oneof="",
                    default="admin",
                )
            ],
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.return_value = mock_stackscripts
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({}, sample_config)

        assert len(result) == 1
        assert "my-script" in result[0].text
        mock_client.list_stackscripts.assert_called_once()


async def test_handle_linode_stackscripts_list_filter_mine(
    sample_config: Config,
) -> None:
    """Test linode_stackscripts_list tool with mine filter."""
    mock_stackscripts = [
        StackScript(
            id=1,
            username="testuser",
            user_gravatar_id="abc123",
            label="my-script",
            description="My script",
            images=["linode/ubuntu22.04"],
            deployments_total=10,
            deployments_active=5,
            is_public=False,
            mine=True,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash",
            user_defined_fields=[],
        ),
        StackScript(
            id=2,
            username="otheruser",
            user_gravatar_id="def456",
            label="other-script",
            description="Other script",
            images=["linode/ubuntu22.04"],
            deployments_total=100,
            deployments_active=50,
            is_public=True,
            mine=False,
            created="2024-01-01T00:00:00",
            updated="2024-01-15T12:00:00",
            script="#!/bin/bash",
            user_defined_fields=[],
        ),
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.return_value = mock_stackscripts
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({"mine": "true"}, sample_config)

        assert len(result) == 1
        assert "my-script" in result[0].text
        assert "other-script" not in result[0].text


async def test_handle_linode_stackscripts_list_error(sample_config: Config) -> None:
    """Test linode_stackscripts_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_stackscripts.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_stackscripts_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text or "error" in result[0].text.lower()
