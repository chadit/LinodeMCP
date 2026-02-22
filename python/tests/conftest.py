"""Shared test fixtures for LinodeMCP."""

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from pathlib import Path

import pytest
import yaml

from linodemcp.config import (
    Config,
    EnvironmentConfig,
    LinodeConfig,
    MetricsConfig,
    ResilienceConfig,
    ServerConfig,
    TracingConfig,
)


@pytest.fixture
def sample_config_data() -> dict[str, Any]:
    """Sample configuration data."""
    return {
        "server": {
            "name": "TestLinodeMCP",
            "logLevel": "debug",
            "transport": "stdio",
            "host": "127.0.0.1",
            "port": 8080,
        },
        "metrics": {
            "enabled": True,
            "port": 9090,
            "path": "/metrics",
        },
        "tracing": {
            "enabled": False,
            "exporter": "otlp",
            "endpoint": "localhost:4317",
            "sampleRate": 1.0,
        },
        "resilience": {
            "rateLimitPerMinute": 700,
            "circuitBreakerThreshold": 5,
            "circuitBreakerTimeout": 30,
            "maxRetries": 3,
            "baseRetryDelay": 1,
            "maxRetryDelay": 30,
        },
        "environments": {
            "default": {
                "label": "Default",
                "linode": {
                    "apiUrl": "https://api.linode.com/v4",
                    "token": "test-token-123",
                },
            },
        },
    }


@pytest.fixture
def sample_config() -> Config:
    """Sample Config object."""
    return Config(
        server=ServerConfig(
            name="TestLinodeMCP",
            log_level="debug",
            transport="stdio",
            host="127.0.0.1",
            port=8080,
        ),
        metrics=MetricsConfig(
            enabled=True,
            port=9090,
            path="/metrics",
        ),
        tracing=TracingConfig(
            enabled=False,
            exporter="otlp",
            endpoint="localhost:4317",
            sample_rate=1.0,
        ),
        resilience=ResilienceConfig(
            rate_limit_per_minute=700,
            circuit_breaker_threshold=5,
            circuit_breaker_timeout=30,
            max_retries=3,
            base_retry_delay=1,
            max_retry_delay=30,
        ),
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token-123",
                ),
            ),
        },
    )


@pytest.fixture
def temp_config_file(tmp_path: Path, sample_config_data: dict[str, Any]) -> Path:
    """Create a temporary config file."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(yaml.dump(sample_config_data))
    return config_file


@pytest.fixture
def sample_profile_data() -> dict[str, Any]:
    """Sample Linode profile data."""
    return {
        "username": "testuser",
        "email": "test@example.com",
        "timezone": "UTC",
        "email_notifications": True,
        "restricted": False,
        "two_factor_auth": True,
        "uid": 12345,
    }


@pytest.fixture
def sample_instance_data() -> dict[str, Any]:
    """Sample Linode instance data."""
    return {
        "id": 123456,
        "label": "test-instance",
        "status": "running",
        "type": "g6-standard-1",
        "region": "us-east",
        "image": "linode/ubuntu22.04",
        "ipv4": ["192.0.2.1"],
        "ipv6": "2001:db8::1/64",
        "hypervisor": "kvm",
        "specs": {
            "disk": 51200,
            "memory": 2048,
            "vcpus": 1,
            "gpus": 0,
            "transfer": 2000,
        },
        "alerts": {
            "cpu": 90,
            "network_in": 10,
            "network_out": 10,
            "transfer_quota": 80,
            "io": 10000,
        },
        "backups": {
            "enabled": True,
            "available": True,
            "schedule": {
                "day": "Saturday",
                "window": "W22",
            },
            "last_successful": None,
        },
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
        "group": "production",
        "tags": ["web", "production"],
        "watchdog_enabled": True,
        "host_uuid": "test-host-uuid-123",
    }
