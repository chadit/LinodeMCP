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
    create_linode_lke_cluster_create_tool,
    create_linode_lke_cluster_delete_tool,
    create_linode_lke_cluster_get_tool,
    create_linode_lke_clusters_list_tool,
    create_linode_vpc_create_tool,
    create_linode_vpc_delete_tool,
    create_linode_vpc_get_tool,
    create_linode_vpc_subnet_create_tool,
    create_linode_vpc_subnet_delete_tool,
    create_linode_vpcs_list_tool,
    handle_hello,
    handle_linode_account,
    handle_linode_domain_create,
    handle_linode_domain_delete,
    handle_linode_domain_get,
    handle_linode_domain_record_create,
    handle_linode_domain_record_delete,
    handle_linode_domain_record_update,
    handle_linode_domain_records_list,
    handle_linode_domain_update,
    handle_linode_domains_list,
    handle_linode_firewall_create,
    handle_linode_firewall_delete,
    handle_linode_firewall_update,
    handle_linode_firewalls_list,
    handle_linode_images_list,
    handle_linode_instance_boot,
    handle_linode_instance_create,
    handle_linode_instance_delete,
    handle_linode_instance_get,
    handle_linode_instance_reboot,
    handle_linode_instance_resize,
    handle_linode_instance_shutdown,
    handle_linode_instances_list,
    handle_linode_lke_acl_delete,
    handle_linode_lke_acl_get,
    handle_linode_lke_acl_update,
    handle_linode_lke_api_endpoints_list,
    handle_linode_lke_cluster_create,
    handle_linode_lke_cluster_delete,
    handle_linode_lke_cluster_get,
    handle_linode_lke_cluster_recycle,
    handle_linode_lke_cluster_regenerate,
    handle_linode_lke_cluster_update,
    handle_linode_lke_clusters_list,
    handle_linode_lke_dashboard_get,
    handle_linode_lke_kubeconfig_delete,
    handle_linode_lke_kubeconfig_get,
    handle_linode_lke_node_delete,
    handle_linode_lke_node_get,
    handle_linode_lke_node_recycle,
    handle_linode_lke_pool_create,
    handle_linode_lke_pool_delete,
    handle_linode_lke_pool_get,
    handle_linode_lke_pool_recycle,
    handle_linode_lke_pool_update,
    handle_linode_lke_pools_list,
    handle_linode_lke_service_token_delete,
    handle_linode_lke_tier_versions_list,
    handle_linode_lke_types_list,
    handle_linode_lke_version_get,
    handle_linode_lke_versions_list,
    handle_linode_nodebalancer_create,
    handle_linode_nodebalancer_delete,
    handle_linode_nodebalancer_get,
    handle_linode_nodebalancer_update,
    handle_linode_nodebalancers_list,
    handle_linode_object_storage_bucket_access_get,
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_bucket_contents,
    handle_linode_object_storage_bucket_create,
    handle_linode_object_storage_bucket_delete,
    handle_linode_object_storage_bucket_get,
    handle_linode_object_storage_buckets_list,
    handle_linode_object_storage_clusters_list,
    handle_linode_object_storage_key_create,
    handle_linode_object_storage_key_delete,
    handle_linode_object_storage_key_get,
    handle_linode_object_storage_key_update,
    handle_linode_object_storage_keys_list,
    handle_linode_object_storage_object_acl_get,
    handle_linode_object_storage_object_acl_update,
    handle_linode_object_storage_presigned_url,
    handle_linode_object_storage_ssl_delete,
    handle_linode_object_storage_ssl_get,
    handle_linode_object_storage_transfer,
    handle_linode_object_storage_types_list,
    handle_linode_profile,
    handle_linode_regions_list,
    handle_linode_sshkey_create,
    handle_linode_sshkey_delete,
    handle_linode_sshkeys_list,
    handle_linode_stackscripts_list,
    handle_linode_types_list,
    handle_linode_volume_attach,
    handle_linode_volume_create,
    handle_linode_volume_delete,
    handle_linode_volume_detach,
    handle_linode_volume_resize,
    handle_linode_volumes_list,
    handle_linode_vpc_create,
    handle_linode_vpc_delete,
    handle_linode_vpc_get,
    handle_linode_vpc_ip_list,
    handle_linode_vpc_ips_list,
    handle_linode_vpc_subnet_create,
    handle_linode_vpc_subnet_delete,
    handle_linode_vpc_subnet_get,
    handle_linode_vpc_subnet_update,
    handle_linode_vpc_subnets_list,
    handle_linode_vpc_update,
    handle_linode_vpcs_list,
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

        result = await handle_linode_images_list({"is_public": "false"}, sample_config)

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
                        addresses=FirewallAddresses(ipv4=["0.0.0.0/0"], ipv6=["::/0"]),
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


# Stage 4: Write operations tests


async def test_handle_linode_sshkey_create(sample_config: Config) -> None:
    """Test linode_sshkey_create tool."""
    mock_key = SSHKey(
        id=12345,
        label="my-key",
        ssh_key="ssh-rsa AAAA...",
        created="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_ssh_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_create(
            {"label": "my-key", "ssh_key": "ssh-rsa AAAA..."}, sample_config
        )

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert "12345" in result[0].text


async def test_handle_linode_sshkey_create_missing_params(
    sample_config: Config,
) -> None:
    """Test linode_sshkey_create tool with missing parameters."""
    result = await handle_linode_sshkey_create({"label": "test"}, sample_config)
    assert len(result) == 1
    assert "Error" in result[0].text


async def test_handle_linode_sshkey_delete(sample_config: Config) -> None:
    """Test linode_sshkey_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_ssh_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_sshkey_delete({"ssh_key_id": 12345}, sample_config)

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_instance_boot(sample_config: Config) -> None:
    """Test linode_instance_boot tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.boot_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_boot(
            {"instance_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "boot" in result[0].text.lower()


async def test_handle_linode_instance_reboot(sample_config: Config) -> None:
    """Test linode_instance_reboot tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.reboot_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_reboot(
            {"instance_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "reboot" in result[0].text.lower()


async def test_handle_linode_instance_shutdown(sample_config: Config) -> None:
    """Test linode_instance_shutdown tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.shutdown_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_shutdown(
            {"instance_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "shutdown" in result[0].text.lower()


async def test_handle_linode_instance_create_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_create tool without confirmation."""
    result = await handle_linode_instance_create(
        {"region": "us-east", "type": "g6-nanode-1"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_create(
    sample_config: Config, sample_instance_data: dict[str, Any]
) -> None:
    """Test linode_instance_create tool."""
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
        specs=Specs(
            disk=sample_instance_data["specs"]["disk"],
            memory=sample_instance_data["specs"]["memory"],
            vcpus=sample_instance_data["specs"]["vcpus"],
            transfer=sample_instance_data["specs"]["transfer"],
            gpus=sample_instance_data["specs"]["gpus"],
        ),
        alerts=Alerts(
            cpu=sample_instance_data["alerts"]["cpu"],
            network_in=sample_instance_data["alerts"]["network_in"],
            network_out=sample_instance_data["alerts"]["network_out"],
            transfer_quota=sample_instance_data["alerts"]["transfer_quota"],
            io=sample_instance_data["alerts"]["io"],
        ),
        backups=Backups(
            enabled=sample_instance_data["backups"]["enabled"],
            available=sample_instance_data["backups"]["available"],
            schedule=Schedule(day="Saturday", window="W0"),
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
        mock_client.create_instance.return_value = mock_instance
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_create(
            {
                "region": "us-east",
                "type": "g6-nanode-1",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "created" in result[0].text.lower()


async def test_handle_linode_instance_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_delete tool without confirmation."""
    result = await handle_linode_instance_delete({"instance_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_delete(sample_config: Config) -> None:
    """Test linode_instance_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_delete(
            {"instance_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_instance_resize_no_confirm(sample_config: Config) -> None:
    """Test linode_instance_resize tool without confirmation."""
    result = await handle_linode_instance_resize(
        {"instance_id": 12345, "type": "g6-standard-1"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_instance_resize(sample_config: Config) -> None:
    """Test linode_instance_resize tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resize_instance.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_instance_resize(
            {"instance_id": 12345, "type": "g6-standard-1", "confirm": True},
            sample_config,
        )

        assert len(result) == 1
        assert "resize" in result[0].text.lower()


async def test_handle_linode_firewall_create(sample_config: Config) -> None:
    """Test linode_firewall_create tool."""
    mock_firewall = Firewall(
        id=12345,
        label="my-firewall",
        status="enabled",
        rules=FirewallRules(
            inbound=[],
            inbound_policy="ACCEPT",
            outbound=[],
            outbound_policy="ACCEPT",
        ),
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_firewall.return_value = mock_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_create(
            {"label": "my-firewall"}, sample_config
        )

        assert len(result) == 1
        assert "my-firewall" in result[0].text


async def test_handle_linode_firewall_update(sample_config: Config) -> None:
    """Test linode_firewall_update tool."""
    mock_firewall = Firewall(
        id=12345,
        label="updated-firewall",
        status="enabled",
        rules=FirewallRules(
            inbound=[],
            inbound_policy="ACCEPT",
            outbound=[],
            outbound_policy="ACCEPT",
        ),
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_firewall.return_value = mock_firewall
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_update(
            {"firewall_id": 12345, "label": "updated-firewall"}, sample_config
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_firewall_delete(sample_config: Config) -> None:
    """Test linode_firewall_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_firewall.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_firewall_delete(
            {"firewall_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_domain_create(sample_config: Config) -> None:
    """Test linode_domain_create tool."""
    mock_domain = Domain(
        id=12345,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_create(
            {"domain": "example.com", "soa_email": "admin@example.com"}, sample_config
        )

        assert len(result) == 1
        assert "example.com" in result[0].text


async def test_handle_linode_domain_update(sample_config: Config) -> None:
    """Test linode_domain_update tool."""
    mock_domain = Domain(
        id=12345,
        domain="example.com",
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="Updated",
        tags=[],
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_domain.return_value = mock_domain
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_update(
            {"domain_id": 12345, "description": "Updated"}, sample_config
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_domain_delete(sample_config: Config) -> None:
    """Test linode_domain_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_domain.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_delete(
            {"domain_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_domain_record_create(sample_config: Config) -> None:
    """Test linode_domain_record_create tool."""
    mock_record = DomainRecord(
        id=12345,
        type="A",
        name="www",
        target="192.0.2.1",
        priority=0,
        weight=0,
        port=0,
        ttl_sec=300,
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_domain_record.return_value = mock_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_create(
            {
                "domain_id": 12345,
                "type": "A",
                "name": "www",
                "target": "192.0.2.1",
            },
            sample_config,
        )

        assert len(result) == 1
        assert "www" in result[0].text


async def test_handle_linode_domain_record_update(sample_config: Config) -> None:
    """Test linode_domain_record_update tool."""
    mock_record = DomainRecord(
        id=12345,
        type="A",
        name="www",
        target="192.0.2.2",
        priority=0,
        weight=0,
        port=0,
        ttl_sec=300,
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_domain_record.return_value = mock_record
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_update(
            {"domain_id": 12345, "record_id": 12345, "target": "192.0.2.2"},
            sample_config,
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_domain_record_delete(sample_config: Config) -> None:
    """Test linode_domain_record_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_domain_record.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_domain_record_delete(
            {"domain_id": 12345, "record_id": 12345}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_volume_create_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_create tool without confirmation."""
    result = await handle_linode_volume_create(
        {"label": "my-volume", "region": "us-east"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_create(sample_config: Config) -> None:
    """Test linode_volume_create tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="creating",
        size=20,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_create(
            {"label": "my-volume", "region": "us-east", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "my-volume" in result[0].text


async def test_handle_linode_volume_attach(sample_config: Config) -> None:
    """Test linode_volume_attach tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="active",
        size=20,
        region="us-east",
        linode_id=54321,
        linode_label="my-linode",
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.attach_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_attach(
            {"volume_id": 12345, "linode_id": 54321}, sample_config
        )

        assert len(result) == 1
        assert "attached" in result[0].text.lower()


async def test_handle_linode_volume_detach(sample_config: Config) -> None:
    """Test linode_volume_detach tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.detach_volume.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_detach({"volume_id": 12345}, sample_config)

        assert len(result) == 1
        assert "detach" in result[0].text.lower()


async def test_handle_linode_volume_resize_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_resize tool without confirmation."""
    result = await handle_linode_volume_resize(
        {"volume_id": 12345, "size": 40}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_resize(sample_config: Config) -> None:
    """Test linode_volume_resize tool."""
    mock_volume = Volume(
        id=12345,
        label="my-volume",
        status="resizing",
        size=40,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_my-volume",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        tags=[],
        hardware_type="nvme",
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.resize_volume.return_value = mock_volume
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_resize(
            {"volume_id": 12345, "size": 40, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "resize" in result[0].text.lower()


async def test_handle_linode_volume_delete_no_confirm(sample_config: Config) -> None:
    """Test linode_volume_delete tool without confirmation."""
    result = await handle_linode_volume_delete({"volume_id": 12345}, sample_config)

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_volume_delete(sample_config: Config) -> None:
    """Test linode_volume_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_volume.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_volume_delete(
            {"volume_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_handle_linode_nodebalancer_create_no_confirm(
    sample_config: Config,
) -> None:
    """Test linode_nodebalancer_create tool without confirmation."""
    result = await handle_linode_nodebalancer_create(
        {"region": "us-east"}, sample_config
    )

    assert len(result) == 1
    assert "confirm" in result[0].text.lower()


async def test_handle_linode_nodebalancer_create(sample_config: Config) -> None:
    """Test linode_nodebalancer_create tool."""
    mock_nodebalancer = NodeBalancer(
        id=12345,
        label="my-nodebalancer",
        region="us-east",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2600:3c03::1",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T10:00:00",
        client_conn_throttle=0,
        transfer=Transfer(in_=100, out=200, total=300),
        tags=[],
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.create_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_create(
            {"region": "us-east", "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "my-nodebalancer" in result[0].text or "12345" in result[0].text


async def test_handle_linode_nodebalancer_update(sample_config: Config) -> None:
    """Test linode_nodebalancer_update tool."""
    mock_nodebalancer = NodeBalancer(
        id=12345,
        label="updated-nodebalancer",
        region="us-east",
        hostname="nb-192-0-2-1.newark.nodebalancer.linode.com",
        ipv4="192.0.2.1",
        ipv6="2600:3c03::1",
        created="2024-01-15T10:00:00",
        updated="2024-01-15T12:00:00",
        client_conn_throttle=5,
        transfer=Transfer(in_=100, out=200, total=300),
        tags=[],
    )

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_nodebalancer.return_value = mock_nodebalancer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_update(
            {"nodebalancer_id": 12345, "label": "updated-nodebalancer"}, sample_config
        )

        assert len(result) == 1
        assert "updated" in result[0].text.lower()


async def test_handle_linode_nodebalancer_delete(sample_config: Config) -> None:
    """Test linode_nodebalancer_delete tool."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.delete_nodebalancer.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_nodebalancer_delete(
            {"nodebalancer_id": 12345, "confirm": True}, sample_config
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


# Object Storage tools


async def test_handle_linode_object_storage_buckets_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_buckets_list tool."""
    mock_buckets = [
        {
            "label": "my-bucket",
            "region": "us-east-1",
            "hostname": "my-bucket.us-east-1.linodeobjects.com",
            "created": "2024-01-01T00:00:00",
            "objects": 42,
            "size": 1024000,
            "cluster": "us-east-1",
        },
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets.return_value = mock_buckets
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_list({}, sample_config)

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_buckets.assert_called_once()


async def test_handle_linode_object_storage_buckets_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_buckets_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_buckets.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_buckets_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_bucket_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get tool."""
    mock_bucket = {
        "label": "my-bucket",
        "region": "us-east-1",
        "hostname": "my-bucket.us-east-1.linodeobjects.com",
        "created": "2024-01-01T00:00:00",
        "objects": 42,
        "size": 1024000,
    }

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket.return_value = mock_bucket
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "my-bucket" in result[0].text
        mock_client.get_object_storage_bucket.assert_called_once_with(
            "us-east-1", "my-bucket"
        )


async def test_handle_linode_object_storage_bucket_get_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get with missing region."""
    result = await handle_linode_object_storage_bucket_get(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_bucket_get_missing_label(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_get with missing label."""
    result = await handle_linode_object_storage_bucket_get(
        {"region": "us-east-1"}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_object_storage_bucket_contents(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents tool."""
    mock_response = {
        "data": [
            {
                "name": "photos/cat.jpg",
                "etag": "abc123",
                "last_modified": "2024-06-01T00:00:00",
                "owner": "user",
                "size": 512000,
                "is_prefix": False,
            },
        ],
        "is_truncated": False,
        "next_marker": "",
    }

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_bucket_contents.return_value = mock_response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_contents(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "cat.jpg" in result[0].text
        assert '"count": 1' in result[0].text


async def test_handle_linode_object_storage_bucket_contents_with_prefix(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents with prefix filter."""
    mock_response = {
        "data": [
            {
                "name": "images/logo.png",
                "etag": "def456",
                "last_modified": "2024-06-01T00:00:00",
                "owner": "user",
                "size": 256000,
                "is_prefix": False,
            },
        ],
        "is_truncated": True,
        "next_marker": "images/next.png",
    }

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_bucket_contents.return_value = mock_response
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_contents(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "prefix": "images/",
                "delimiter": "/",
            },
            sample_config,
        )

        assert len(result) == 1
        assert "logo.png" in result[0].text
        assert "next_marker" in result[0].text
        assert "prefix=images/" in result[0].text


async def test_handle_linode_object_storage_bucket_contents_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_contents with missing region."""
    result = await handle_linode_object_storage_bucket_contents(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_clusters_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_clusters_list tool."""
    mock_clusters = [
        {
            "id": "us-east-1",
            "region": "us-east",
            "domain": "us-east-1.linodeobjects.com",
            "status": "available",
            "static_site": {"domain": "website-us-east-1.linodeobjects.com"},
        },
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_clusters.return_value = mock_clusters
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_clusters_list({}, sample_config)

        assert len(result) == 1
        assert "us-east-1" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_clusters.assert_called_once()


async def test_handle_linode_object_storage_clusters_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_clusters_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_clusters.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_clusters_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_types_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_types_list tool."""
    mock_types = [
        {
            "id": "objectstorage",
            "label": "Object Storage",
            "price": {"hourly": 0.02, "monthly": 5.0},
            "transfer": 1000,
            "region": "us-east",
        },
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_types.return_value = mock_types
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_types_list({}, sample_config)

        assert len(result) == 1
        assert "objectstorage" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_types.assert_called_once()


async def test_handle_linode_object_storage_types_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_types_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_types.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_types_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


# Phase 2: Access Key & Transfer Tests


async def test_handle_linode_object_storage_keys_list(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_keys_list tool."""
    mock_keys = [
        {
            "id": 1,
            "label": "my-key",
            "access_key": "AKIAIOSFODNN7EXAMPLE",
            "secret_key": "[REDACTED]",
            "limited": False,
            "bucket_access": None,
        },
    ]

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_keys.return_value = mock_keys
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_keys_list({}, sample_config)

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert '"count": 1' in result[0].text
        mock_client.list_object_storage_keys.assert_called_once()


async def test_handle_linode_object_storage_keys_list_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_keys_list tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.list_object_storage_keys.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_keys_list({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_key_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_get tool."""
    mock_key = {
        "id": 42,
        "label": "my-key",
        "access_key": "AKIAIOSFODNN7EXAMPLE",
        "secret_key": "[REDACTED]",
        "limited": True,
        "bucket_access": [
            {
                "bucket_name": "my-bucket",
                "region": "us-east-1",
                "permissions": "read_only",
            },
        ],
    }

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_key.return_value = mock_key
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_key_get(
            {"key_id": 42}, sample_config
        )

        assert len(result) == 1
        assert "my-key" in result[0].text
        assert "my-bucket" in result[0].text
        mock_client.get_object_storage_key.assert_called_once_with(42)


async def test_handle_linode_object_storage_key_get_missing_id(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_key_get with missing key_id."""
    result = await handle_linode_object_storage_key_get({}, sample_config)

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_handle_linode_object_storage_transfer(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_transfer tool."""
    mock_transfer = {"used": 1073741824}

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_transfer.return_value = mock_transfer
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer({}, sample_config)

        assert len(result) == 1
        assert "1073741824" in result[0].text
        mock_client.get_object_storage_transfer.assert_called_once()


async def test_handle_linode_object_storage_transfer_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_transfer tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_transfer.side_effect = Exception("API error")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_transfer({}, sample_config)

        assert len(result) == 1
        assert "Failed" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get tool."""
    mock_access = {"acl": "public-read", "cors_enabled": True}

    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.return_value = mock_access
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "public-read" in result[0].text
        mock_client.get_object_storage_bucket_access.assert_called_once_with(
            "us-east-1", "my-bucket"
        )


async def test_handle_linode_object_storage_bucket_access_get_missing_region(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get with missing region."""
    result = await handle_linode_object_storage_bucket_access_get(
        {"label": "my-bucket"}, sample_config
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get_missing_label(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get with missing label."""
    result = await handle_linode_object_storage_bucket_access_get(
        {"region": "us-east-1"}, sample_config
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_handle_linode_object_storage_bucket_access_get_error(
    sample_config: Config,
) -> None:
    """Test linode_object_storage_bucket_access_get tool error handling."""
    with patch("linodemcp.tools.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_object_storage_bucket_access.side_effect = Exception(
            "API error"
        )
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_get(
            {"region": "us-east-1", "label": "my-bucket"}, sample_config
        )

        assert len(result) == 1
        assert "Failed" in result[0].text


# Phase 3: Object Storage Write Bucket Tool Tests


async def test_handle_object_storage_bucket_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket create requires confirm=true."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "my-bucket", "region": "us-east-1"},
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_create_invalid_label(
    sample_config: Config,
) -> None:
    """Test bucket create rejects invalid label."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "AB", "region": "us-east-1", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "at least 3" in result[0].text


async def test_handle_object_storage_bucket_create_invalid_acl(
    sample_config: Config,
) -> None:
    """Test bucket create rejects invalid ACL."""
    result = await handle_linode_object_storage_bucket_create(
        {
            "label": "my-bucket",
            "region": "us-east-1",
            "acl": "bad-acl",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_handle_object_storage_bucket_create_missing_region(
    sample_config: Config,
) -> None:
    """Test bucket create requires region."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "my-bucket", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_object_storage_bucket_create_success(
    sample_config: Config,
) -> None:
    """Test bucket create success."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_object_storage_bucket.return_value = {
            "label": "my-bucket",
            "region": "us-east-1",
            "created": "2024-01-01T00:00:00",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_create(
            {
                "label": "my-bucket",
                "region": "us-east-1",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "created successfully" in result[0].text


async def test_handle_object_storage_bucket_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket delete requires confirm=true."""
    result = await handle_linode_object_storage_bucket_delete(
        {"region": "us-east-1", "label": "my-bucket"},
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_delete_missing_region(
    sample_config: Config,
) -> None:
    """Test bucket delete requires region."""
    result = await handle_linode_object_storage_bucket_delete(
        {"label": "my-bucket", "confirm": True},
        sample_config,
    )

    assert len(result) == 1
    assert "region is required" in result[0].text


async def test_handle_object_storage_bucket_delete_success(
    sample_config: Config,
) -> None:
    """Test bucket delete success."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_object_storage_bucket.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "deleted successfully" in result[0].text


async def test_handle_object_storage_bucket_access_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Test bucket access update requires confirm."""
    result = await handle_linode_object_storage_bucket_access_update(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "public-read",
        },
        sample_config,
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_handle_object_storage_bucket_access_update_invalid_acl(
    sample_config: Config,
) -> None:
    """Test bucket access update rejects invalid ACL."""
    result = await handle_linode_object_storage_bucket_access_update(
        {
            "region": "us-east-1",
            "label": "my-bucket",
            "acl": "bad-acl",
            "confirm": True,
        },
        sample_config,
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_handle_object_storage_bucket_access_update_success(
    sample_config: Config,
) -> None:
    """Test bucket access update success."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_storage_bucket_access.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_object_storage_bucket_access_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "acl": "public-read",
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert "updated successfully" in result[0].text


# Phase 4: Object Storage Access Key Write Tool Tests


async def test_object_storage_key_create_requires_confirm(
    sample_config: Config,
) -> None:
    """Key create should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "my-key"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text
    assert "confirm=true" in result[0].text
    assert "secret_key" in result[0].text


async def test_object_storage_key_create_empty_label(
    sample_config: Config,
) -> None:
    """Key create should reject empty label."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label is required" in result[0].text


async def test_object_storage_key_create_label_too_long(
    sample_config: Config,
) -> None:
    """Key create should reject label over 50 chars."""
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "a" * 51, "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "50 characters" in result[0].text


async def test_object_storage_key_create_invalid_json(
    sample_config: Config,
) -> None:
    """Key create should reject invalid bucket_access JSON."""
    result = list(
        await handle_linode_object_storage_key_create(
            {
                "label": "my-key",
                "bucket_access": "not-valid-json",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "Invalid bucket_access JSON" in result[0].text


async def test_object_storage_key_create_invalid_permissions(
    sample_config: Config,
) -> None:
    """Key create should reject invalid permissions."""
    import json

    bucket_access = json.dumps(
        [
            {
                "bucket_name": "mybucket",
                "region": "us-east-1",
                "permissions": "admin",
            }
        ]
    )
    result = list(
        await handle_linode_object_storage_key_create(
            {
                "label": "my-key",
                "bucket_access": bucket_access,
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "read_only" in result[0].text


async def test_object_storage_key_create_success(
    sample_config: Config,
) -> None:
    """Key create should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_object_storage_key.return_value = {
            "id": 42,
            "label": "my-key",
            "access_key": "AKIAIOSFODNN7EXAMPLE",
            "secret_key": "wJalrXUtnFEMI/bPxRfiCYEXAMPLEKEY",
            "limited": False,
            "bucket_access": [],
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_create(
                {"label": "my-key", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "created successfully" in result[0].text
        assert "IMPORTANT" in result[0].text
        assert "ONLY ONCE" in result[0].text


async def test_object_storage_key_create_missing_env() -> None:
    """Key create should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_key_create(
            {"label": "my-key", "confirm": True},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_object_storage_key_update_requires_confirm(
    sample_config: Config,
) -> None:
    """Key update should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_update(
            {"key_id": 42, "label": "new-label"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_storage_key_update_invalid_key_id(
    sample_config: Config,
) -> None:
    """Key update should reject invalid key_id."""
    result = list(
        await handle_linode_object_storage_key_update(
            {"key_id": 0, "label": "new-label", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_object_storage_key_update_success(
    sample_config: Config,
) -> None:
    """Key update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_storage_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_update(
                {
                    "key_id": 42,
                    "label": "updated-key",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated successfully" in result[0].text


async def test_object_storage_key_delete_requires_confirm(
    sample_config: Config,
) -> None:
    """Key delete should require confirm=true."""
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": 42},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_storage_key_delete_invalid_key_id(
    sample_config: Config,
) -> None:
    """Key delete should reject invalid key_id."""
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": -1, "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "key_id is required" in result[0].text


async def test_object_storage_key_delete_success(
    sample_config: Config,
) -> None:
    """Key delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_object_storage_key.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_key_delete(
                {"key_id": 42, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "revoked successfully" in result[0].text


async def test_object_storage_key_delete_missing_env() -> None:
    """Key delete should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_key_delete(
            {"key_id": 42, "confirm": True},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


# Phase 5: Presigned URLs, Object ACL & SSL Tool Tests


async def test_presigned_url_missing_name(
    sample_config: Config,
) -> None:
    """Presigned URL should fail when name is missing."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {"region": "us-east-1", "label": "my-bucket", "method": "GET"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "name" in result[0].text


async def test_presigned_url_invalid_method(
    sample_config: Config,
) -> None:
    """Presigned URL should fail with invalid method."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "DELETE",
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "GET" in result[0].text
    assert "PUT" in result[0].text


async def test_presigned_url_invalid_expires(
    sample_config: Config,
) -> None:
    """Presigned URL should fail with out of range expires_in."""
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "GET",
                "expires_in": 700000,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "604800" in result[0].text


async def test_presigned_url_success(
    sample_config: Config,
) -> None:
    """Presigned URL should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_presigned_url.return_value = {
            "url": "https://bucket.example.com/photo.jpg?signed=abc",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_presigned_url(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                    "method": "GET",
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "signed=abc" in result[0].text


async def test_presigned_url_missing_env() -> None:
    """Presigned URL should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_presigned_url(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "method": "GET",
            },
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_object_acl_get_missing_name(
    sample_config: Config,
) -> None:
    """Object ACL get should fail when name is missing."""
    result = list(
        await handle_linode_object_storage_object_acl_get(
            {"region": "us-east-1", "label": "my-bucket"},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "name" in result[0].text


async def test_object_acl_get_success(
    sample_config: Config,
) -> None:
    """Object ACL get should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_acl.return_value = {
            "acl": "public-read",
            "acl_xml": "<AccessControlPolicy>...</AccessControlPolicy>",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_object_acl_get(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "public-read" in result[0].text


async def test_object_acl_update_confirm_required(
    sample_config: Config,
) -> None:
    """Object ACL update should require confirm=true."""
    result = list(
        await handle_linode_object_storage_object_acl_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "acl": "public-read",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_object_acl_update_invalid_acl(
    sample_config: Config,
) -> None:
    """Object ACL update should fail with invalid ACL."""
    result = list(
        await handle_linode_object_storage_object_acl_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "acl": "invalid-acl",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "acl must be one of" in result[0].text


async def test_object_acl_update_success(
    sample_config: Config,
) -> None:
    """Object ACL update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_object_acl.return_value = {
            "acl": "public-read",
            "acl_xml": "<AccessControlPolicy>...</AccessControlPolicy>",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_object_acl_update(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "name": "photo.jpg",
                    "acl": "public-read",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "public-read" in result[0].text


async def test_ssl_get_success(
    sample_config: Config,
) -> None:
    """SSL get should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_bucket_ssl.return_value = {
            "ssl": True,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_get(
                {"region": "us-east-1", "label": "my-bucket"},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "true" in result[0].text


async def test_ssl_get_missing_env() -> None:
    """SSL get should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_ssl_get(
            {"region": "us-east-1", "label": "my-bucket"},
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


async def test_ssl_delete_confirm_required(
    sample_config: Config,
) -> None:
    """SSL delete should require confirm=true."""
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_ssl_delete_success(
    sample_config: Config,
) -> None:
    """SSL delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_bucket_ssl.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_object_storage_ssl_delete(
                {
                    "region": "us-east-1",
                    "label": "my-bucket",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "SSL certificate deleted" in result[0].text


async def test_ssl_delete_missing_env() -> None:
    """SSL delete should fail with missing environment."""
    cfg = Config(environments={})
    result = list(
        await handle_linode_object_storage_ssl_delete(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "confirm": True,
            },
            cfg,
        )
    )

    assert len(result) == 1
    assert "Error" in result[0].text


# ── LKE Tool Tests ──────────────────────────────────────────────


async def test_lke_clusters_list_tool_definition() -> None:
    """LKE clusters list tool should have correct name."""
    tool = create_linode_lke_clusters_list_tool()
    assert tool.name == "linode_lke_clusters_list"


async def test_lke_cluster_get_tool_definition() -> None:
    """LKE cluster get tool should require cluster_id."""
    tool = create_linode_lke_cluster_get_tool()
    assert tool.name == "linode_lke_cluster_get"
    assert "cluster_id" in (tool.inputSchema.get("required") or [])


async def test_lke_cluster_create_tool_definition() -> None:
    """LKE cluster create tool should require label, region, k8s_version."""
    tool = create_linode_lke_cluster_create_tool()
    assert tool.name == "linode_lke_cluster_create"
    required = tool.inputSchema.get("required") or []
    assert "label" in required
    assert "region" in required
    assert "k8s_version" in required


async def test_lke_cluster_delete_tool_definition() -> None:
    """LKE cluster delete tool should require cluster_id and confirm."""
    tool = create_linode_lke_cluster_delete_tool()
    assert tool.name == "linode_lke_cluster_delete"
    required = tool.inputSchema.get("required") or []
    assert "cluster_id" in required
    assert "confirm" in required


async def test_lke_clusters_list(sample_config: Config) -> None:
    """LKE clusters list should return cluster data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_clusters.return_value = [
            {"id": 1, "label": "my-cluster", "region": "us-east", "status": "ready"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_clusters_list({}, sample_config))

        assert len(result) == 1
        assert "my-cluster" in result[0].text


async def test_lke_cluster_get(sample_config: Config) -> None:
    """LKE cluster get should return cluster details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.return_value = {
            "id": 1,
            "label": "my-cluster",
            "region": "us-east",
            "k8s_version": "1.29",
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "my-cluster" in result[0].text


async def test_lke_cluster_get_missing_id(sample_config: Config) -> None:
    """LKE cluster get should fail without cluster_id."""
    result = list(await handle_linode_lke_cluster_get({}, sample_config))

    assert len(result) == 1
    assert "cluster_id" in result[0].text.lower()


async def test_lke_cluster_create_confirm_required(sample_config: Config) -> None:
    """LKE cluster create should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_create(
            {
                "label": "new-cluster",
                "region": "us-east",
                "k8s_version": "1.29",
                "node_pools": [{"type": "g6-standard-1", "count": 3}],
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_create_missing_label(sample_config: Config) -> None:
    """LKE cluster create should fail without label."""
    result = list(
        await handle_linode_lke_cluster_create(
            {
                "region": "us-east",
                "k8s_version": "1.29",
                "node_pools": [{"type": "g6-standard-1", "count": 3}],
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_lke_cluster_create_success(sample_config: Config) -> None:
    """LKE cluster create should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_lke_cluster.return_value = {
            "id": 10,
            "label": "new-cluster",
            "region": "us-east",
            "k8s_version": "1.29",
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_create(
                {
                    "label": "new-cluster",
                    "region": "us-east",
                    "k8s_version": "1.29",
                    "node_pools": [{"type": "g6-standard-1", "count": 3}],
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-cluster" in result[0].text


async def test_lke_cluster_update_confirm_required(sample_config: Config) -> None:
    """LKE cluster update should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_update(
            {"cluster_id": 1, "label": "updated", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_update_success(sample_config: Config) -> None:
    """LKE cluster update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_cluster.return_value = {
            "id": 1,
            "label": "updated",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_update(
                {"cluster_id": 1, "label": "updated", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated" in result[0].text


async def test_lke_cluster_delete_confirm_required(sample_config: Config) -> None:
    """LKE cluster delete should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_delete_success(sample_config: Config) -> None:
    """LKE cluster delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_cluster_recycle_confirm_required(sample_config: Config) -> None:
    """LKE cluster recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_recycle(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_recycle_success(sample_config: Config) -> None:
    """LKE cluster recycle should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_recycle(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_cluster_regenerate_confirm_required(
    sample_config: Config,
) -> None:
    """LKE cluster regenerate should require confirm=true."""
    result = list(
        await handle_linode_lke_cluster_regenerate(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_cluster_regenerate_success(sample_config: Config) -> None:
    """LKE cluster regenerate should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.regenerate_lke_cluster.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_cluster_regenerate(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "regenerat" in result[0].text.lower()


async def test_lke_pools_list(sample_config: Config) -> None:
    """LKE pools list should return pool data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_node_pools.return_value = [
            {"id": 100, "type": "g6-standard-1", "count": 3},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pools_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_get(sample_config: Config) -> None:
    """LKE pool get should return pool details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node_pool.return_value = {
            "id": 100,
            "type": "g6-standard-1",
            "count": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_get(
                {"cluster_id": 1, "pool_id": 100}, sample_config
            )
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_create_confirm_required(sample_config: Config) -> None:
    """LKE pool create should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_create(
            {
                "cluster_id": 1,
                "type": "g6-standard-1",
                "count": 3,
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_create_success(sample_config: Config) -> None:
    """LKE pool create should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_lke_node_pool.return_value = {
            "id": 200,
            "type": "g6-standard-1",
            "count": 3,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_create(
                {
                    "cluster_id": 1,
                    "type": "g6-standard-1",
                    "count": 3,
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_pool_update_confirm_required(sample_config: Config) -> None:
    """LKE pool update should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_update(
            {"cluster_id": 1, "pool_id": 100, "count": 5, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_update_success(sample_config: Config) -> None:
    """LKE pool update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_node_pool.return_value = {
            "id": 100,
            "type": "g6-standard-1",
            "count": 5,
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_update(
                {"cluster_id": 1, "pool_id": 100, "count": 5, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "100" in result[0].text


async def test_lke_pool_delete_confirm_required(sample_config: Config) -> None:
    """LKE pool delete should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_delete(
            {"cluster_id": 1, "pool_id": 100, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_delete_success(sample_config: Config) -> None:
    """LKE pool delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_node_pool.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_delete(
                {"cluster_id": 1, "pool_id": 100, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_pool_recycle_confirm_required(sample_config: Config) -> None:
    """LKE pool recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_pool_recycle(
            {"cluster_id": 1, "pool_id": 100, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_pool_recycle_success(sample_config: Config) -> None:
    """LKE pool recycle should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_node_pool.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_pool_recycle(
                {"cluster_id": 1, "pool_id": 100, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_node_get(sample_config: Config) -> None:
    """LKE node get should return node details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_node.return_value = {
            "id": "lke-node-abc",
            "instance_id": 555,
            "status": "ready",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_get(
                {"cluster_id": 1, "node_id": "lke-node-abc"}, sample_config
            )
        )

        assert len(result) == 1
        assert "lke-node-abc" in result[0].text


async def test_lke_node_get_missing_node_id(sample_config: Config) -> None:
    """LKE node get should fail without node_id."""
    result = list(await handle_linode_lke_node_get({"cluster_id": 1}, sample_config))

    assert len(result) == 1
    assert "node_id" in result[0].text.lower()


async def test_lke_node_delete_confirm_required(sample_config: Config) -> None:
    """LKE node delete should require confirm=true."""
    result = list(
        await handle_linode_lke_node_delete(
            {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_node_delete_success(sample_config: Config) -> None:
    """LKE node delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_node.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_delete(
                {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_node_recycle_confirm_required(sample_config: Config) -> None:
    """LKE node recycle should require confirm=true."""
    result = list(
        await handle_linode_lke_node_recycle(
            {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_node_recycle_success(sample_config: Config) -> None:
    """LKE node recycle should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.recycle_lke_node.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_node_recycle(
                {"cluster_id": 1, "node_id": "lke-node-abc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "recycle" in result[0].text.lower()


async def test_lke_kubeconfig_get(sample_config: Config) -> None:
    """LKE kubeconfig get should return kubeconfig data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_kubeconfig.return_value = {
            "kubeconfig": "YXBpVmVyc2lvbjogdjEK",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_kubeconfig_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "kubeconfig" in result[0].text.lower()


async def test_lke_kubeconfig_delete_confirm_required(
    sample_config: Config,
) -> None:
    """LKE kubeconfig delete should require confirm=true."""
    result = list(
        await handle_linode_lke_kubeconfig_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_kubeconfig_delete_success(sample_config: Config) -> None:
    """LKE kubeconfig delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_kubeconfig.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_kubeconfig_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "regenerated" in result[0].text.lower()


async def test_lke_dashboard_get(sample_config: Config) -> None:
    """LKE dashboard get should return dashboard URL."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_dashboard.return_value = {
            "url": "https://dashboard.example.com",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_dashboard_get({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "dashboard" in result[0].text.lower()


async def test_lke_api_endpoints_list(sample_config: Config) -> None:
    """LKE API endpoints list should return endpoint data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_api_endpoints.return_value = [
            {"endpoint": "https://api.lke.example.com"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_api_endpoints_list({"cluster_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "endpoint" in result[0].text.lower()


async def test_lke_service_token_delete_confirm_required(
    sample_config: Config,
) -> None:
    """LKE service token delete should require confirm=true."""
    result = list(
        await handle_linode_lke_service_token_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_service_token_delete_success(sample_config: Config) -> None:
    """LKE service token delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_service_token.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_service_token_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_acl_get(sample_config: Config) -> None:
    """LKE ACL get should return ACL data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_control_plane_acl.return_value = {
            "acl": {
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": []},
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_acl_get({"cluster_id": 1}, sample_config))

        assert len(result) == 1
        assert "acl" in result[0].text.lower()


async def test_lke_acl_update_confirm_required(sample_config: Config) -> None:
    """LKE ACL update should require confirm=true."""
    result = list(
        await handle_linode_lke_acl_update(
            {
                "cluster_id": 1,
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"]},
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_acl_update_success(sample_config: Config) -> None:
    """LKE ACL update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_lke_control_plane_acl.return_value = {
            "acl": {
                "enabled": True,
                "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": []},
            },
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_acl_update(
                {
                    "cluster_id": 1,
                    "enabled": True,
                    "addresses": {"ipv4": ["10.0.0.0/8"]},
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "acl" in result[0].text.lower()


async def test_lke_acl_delete_confirm_required(sample_config: Config) -> None:
    """LKE ACL delete should require confirm=true."""
    result = list(
        await handle_linode_lke_acl_delete(
            {"cluster_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_lke_acl_delete_success(sample_config: Config) -> None:
    """LKE ACL delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_lke_control_plane_acl.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_acl_delete(
                {"cluster_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_lke_versions_list(sample_config: Config) -> None:
    """LKE versions list should return version data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_versions.return_value = [
            {"id": "1.29"},
            {"id": "1.28"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_versions_list({}, sample_config))

        assert len(result) == 1
        assert "1.29" in result[0].text


async def test_lke_version_get(sample_config: Config) -> None:
    """LKE version get should return version details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_version.return_value = {"id": "1.29"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_lke_version_get({"version_id": "1.29"}, sample_config)
        )

        assert len(result) == 1
        assert "1.29" in result[0].text


async def test_lke_version_get_missing_id(sample_config: Config) -> None:
    """LKE version get should fail without version_id."""
    result = list(await handle_linode_lke_version_get({}, sample_config))

    assert len(result) == 1
    assert "version_id" in result[0].text.lower()


async def test_lke_types_list(sample_config: Config) -> None:
    """LKE types list should return type data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_types.return_value = [
            {"id": "g6-standard-1", "label": "Linode 2GB"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_types_list({}, sample_config))

        assert len(result) == 1
        assert "g6-standard-1" in result[0].text


async def test_lke_tier_versions_list(sample_config: Config) -> None:
    """LKE tier versions list should return tier version data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_lke_tier_versions.return_value = [
            {"id": "1.29", "tier": "standard"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_lke_tier_versions_list({}, sample_config))

        assert len(result) == 1
        assert "1.29" in result[0].text


# VPC tool definition tests


async def test_vpcs_list_tool_definition() -> None:
    """VPCs list tool should have correct name."""
    tool = create_linode_vpcs_list_tool()
    assert tool.name == "linode_vpcs_list"


async def test_vpc_get_tool_definition() -> None:
    """VPC get tool should require vpc_id."""
    tool = create_linode_vpc_get_tool()
    assert tool.name == "linode_vpc_get"
    assert "vpc_id" in (tool.inputSchema.get("required") or [])


async def test_vpc_create_tool_definition() -> None:
    """VPC create tool should require label, region, confirm."""
    tool = create_linode_vpc_create_tool()
    assert tool.name == "linode_vpc_create"
    required = tool.inputSchema.get("required") or []
    assert "label" in required
    assert "region" in required
    assert "confirm" in required


async def test_vpc_delete_tool_definition() -> None:
    """VPC delete tool should require vpc_id and confirm."""
    tool = create_linode_vpc_delete_tool()
    assert tool.name == "linode_vpc_delete"
    required = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "confirm" in required


async def test_vpc_subnet_create_tool_definition() -> None:
    """VPC subnet create tool should require vpc_id, label, ipv4, confirm."""
    tool = create_linode_vpc_subnet_create_tool()
    assert tool.name == "linode_vpc_subnet_create"
    required = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "label" in required
    assert "ipv4" in required
    assert "confirm" in required


async def test_vpc_subnet_delete_tool_definition() -> None:
    """VPC subnet delete tool should require vpc_id, subnet_id, confirm."""
    tool = create_linode_vpc_subnet_delete_tool()
    assert tool.name == "linode_vpc_subnet_delete"
    required = tool.inputSchema.get("required") or []
    assert "vpc_id" in required
    assert "subnet_id" in required
    assert "confirm" in required


# VPC handler tests


async def test_vpcs_list(sample_config: Config) -> None:
    """VPCs list should return VPC data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpcs.return_value = [
            {"id": 1, "label": "my-vpc", "region": "us-east"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpcs_list({}, sample_config))

        assert len(result) == 1
        assert "my-vpc" in result[0].text


async def test_vpc_get(sample_config: Config) -> None:
    """VPC get should return VPC details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc.return_value = {
            "id": 1,
            "label": "my-vpc",
            "region": "us-east",
            "description": "test vpc",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_get({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        assert "my-vpc" in result[0].text


async def test_vpc_get_missing_id(sample_config: Config) -> None:
    """VPC get should fail without vpc_id."""
    result = list(await handle_linode_vpc_get({}, sample_config))

    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()


async def test_vpc_create_confirm_required(sample_config: Config) -> None:
    """VPC create should require confirm=true."""
    result = list(
        await handle_linode_vpc_create(
            {
                "label": "new-vpc",
                "region": "us-east",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_create_missing_label(sample_config: Config) -> None:
    """VPC create should fail without label."""
    result = list(
        await handle_linode_vpc_create(
            {"region": "us-east", "confirm": True},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_vpc_create_success(sample_config: Config) -> None:
    """VPC create should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_vpc.return_value = {
            "id": 10,
            "label": "new-vpc",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_create(
                {
                    "label": "new-vpc",
                    "region": "us-east",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-vpc" in result[0].text


async def test_vpc_update_confirm_required(sample_config: Config) -> None:
    """VPC update should require confirm=true."""
    result = list(
        await handle_linode_vpc_update(
            {"vpc_id": 1, "label": "updated", "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_update_success(sample_config: Config) -> None:
    """VPC update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_vpc.return_value = {
            "id": 1,
            "label": "updated-vpc",
            "region": "us-east",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_update(
                {"vpc_id": 1, "label": "updated-vpc", "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated-vpc" in result[0].text


async def test_vpc_delete_confirm_required(sample_config: Config) -> None:
    """VPC delete should require confirm=true."""
    result = list(
        await handle_linode_vpc_delete(
            {"vpc_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_delete_success(sample_config: Config) -> None:
    """VPC delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_vpc.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_delete(
                {"vpc_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()


async def test_vpc_ips_list(sample_config: Config) -> None:
    """VPC IPs list should return IP data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_ips.return_value = [
            {"address": "10.0.0.1", "vpc_id": 1, "subnet_id": 1},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ips_list({}, sample_config))

        assert len(result) == 1
        assert "10.0.0.1" in result[0].text


async def test_vpc_ip_list(sample_config: Config) -> None:
    """VPC IP list should return IPs for a specific VPC."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_ip.return_value = [
            {"address": "10.0.0.2", "vpc_id": 1, "subnet_id": 1},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(await handle_linode_vpc_ip_list({"vpc_id": 1}, sample_config))

        assert len(result) == 1
        assert "10.0.0.2" in result[0].text


async def test_vpc_ip_list_missing_id(sample_config: Config) -> None:
    """VPC IP list should fail without vpc_id."""
    result = list(await handle_linode_vpc_ip_list({}, sample_config))

    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()


async def test_vpc_subnets_list(sample_config: Config) -> None:
    """VPC subnets list should return subnet data."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.list_vpc_subnets.return_value = [
            {"id": 1, "label": "my-subnet", "ipv4": "10.0.0.0/24"},
        ]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnets_list({"vpc_id": 1}, sample_config)
        )

        assert len(result) == 1
        assert "my-subnet" in result[0].text


async def test_vpc_subnet_get(sample_config: Config) -> None:
    """VPC subnet get should return subnet details."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_vpc_subnet.return_value = {
            "id": 1,
            "label": "my-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_get(
                {"vpc_id": 1, "subnet_id": 1}, sample_config
            )
        )

        assert len(result) == 1
        assert "my-subnet" in result[0].text


async def test_vpc_subnet_get_missing_ids(sample_config: Config) -> None:
    """VPC subnet get should fail without required IDs."""
    result = list(await handle_linode_vpc_subnet_get({}, sample_config))
    assert len(result) == 1
    assert "vpc_id" in result[0].text.lower()

    result = list(await handle_linode_vpc_subnet_get({"vpc_id": 1}, sample_config))
    assert len(result) == 1
    assert "subnet_id" in result[0].text.lower()


async def test_vpc_subnet_create_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet create should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {
                "vpc_id": 1,
                "label": "new-subnet",
                "ipv4": "10.0.0.0/24",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_create_missing_label(
    sample_config: Config,
) -> None:
    """VPC subnet create should fail without label."""
    result = list(
        await handle_linode_vpc_subnet_create(
            {
                "vpc_id": 1,
                "ipv4": "10.0.0.0/24",
                "confirm": True,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "label" in result[0].text.lower()


async def test_vpc_subnet_create_success(sample_config: Config) -> None:
    """VPC subnet create should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.create_vpc_subnet.return_value = {
            "id": 5,
            "label": "new-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_create(
                {
                    "vpc_id": 1,
                    "label": "new-subnet",
                    "ipv4": "10.0.0.0/24",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "new-subnet" in result[0].text


async def test_vpc_subnet_update_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet update should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_update(
            {
                "vpc_id": 1,
                "subnet_id": 1,
                "label": "updated",
                "confirm": False,
            },
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_update_success(sample_config: Config) -> None:
    """VPC subnet update should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.update_vpc_subnet.return_value = {
            "id": 1,
            "label": "updated-subnet",
            "ipv4": "10.0.0.0/24",
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_update(
                {
                    "vpc_id": 1,
                    "subnet_id": 1,
                    "label": "updated-subnet",
                    "confirm": True,
                },
                sample_config,
            )
        )

        assert len(result) == 1
        assert "updated-subnet" in result[0].text


async def test_vpc_subnet_delete_confirm_required(
    sample_config: Config,
) -> None:
    """VPC subnet delete should require confirm=true."""
    result = list(
        await handle_linode_vpc_subnet_delete(
            {"vpc_id": 1, "subnet_id": 1, "confirm": False},
            sample_config,
        )
    )

    assert len(result) == 1
    assert "confirm=true" in result[0].text


async def test_vpc_subnet_delete_success(sample_config: Config) -> None:
    """VPC subnet delete should succeed with valid input."""
    with patch("linodemcp.tools.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.delete_vpc_subnet.return_value = None
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = list(
            await handle_linode_vpc_subnet_delete(
                {"vpc_id": 1, "subnet_id": 1, "confirm": True},
                sample_config,
            )
        )

        assert len(result) == 1
        assert "deleted" in result[0].text.lower()
