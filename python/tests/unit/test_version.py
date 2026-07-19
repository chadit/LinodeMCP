"""Unit tests for version module."""

import io
import json

from linodemcp.main import print_version
from linodemcp.version import VERSION, get_version_info


def test_version_constant() -> None:
    """Test version constant."""
    assert VERSION == "0.1.0"


def test_get_version_info() -> None:
    """Test getting version info."""
    info = get_version_info()
    assert info.version == "0.1.0"
    assert info.api_version == "0.1.0"
    assert info.git_commit == "dev"
    assert info.git_branch == "main"
    assert info.build_date == "unknown"
    assert info.python_version
    assert info.platform
    assert "hello" in info.features["tools"]
    assert "linode_managed_linode_settings_get" in info.features["tools"]
    assert "linode_managed_service_list" in info.features["tools"]
    assert "linode_managed_service_delete" in info.features["tools"]
    assert "linode_managed_service_disable" in info.features["tools"]
    assert "linode_managed_service_enable" in info.features["tools"]
    assert "linode_networking_ip_share" in info.features["tools"]


def test_version_info_with_custom_values() -> None:
    """Test version info with custom build values."""
    info = get_version_info(
        build_date="2024-01-15",
        git_commit="abc123",
        git_branch="feature/test",
    )
    assert info.build_date == "2024-01-15"
    assert info.git_commit == "abc123"
    assert info.git_branch == "feature/test"


def test_version_info_to_dict() -> None:
    """Test converting version info to dictionary."""
    info = get_version_info()
    data = info.to_dict()
    assert isinstance(data, dict)
    assert data["version"] == "0.1.0"
    assert data["api_version"] == "0.1.0"
    assert "features" in data


def test_version_info_str() -> None:
    """Test version info string representation."""
    info = get_version_info()
    str_repr = str(info)
    assert "LinodeMCP" in str_repr
    assert "0.1.0" in str_repr
    assert info.platform in str_repr
    assert info.git_commit in str_repr


def test_cli_version_verb_emits_the_proto_field_set() -> None:
    """The CLI ``version`` verb prints exactly the VersionResponse fields.

    version.proto pins {version, api_version, build_date, commit, platform}
    as the envelope every language and every path emits; the legacy
    ``to_dict()`` shape (git_commit, git_branch, features) must never come
    back on the CLI path.
    """
    stream = io.StringIO()
    assert print_version(stream) == 0
    payload = json.loads(stream.getvalue())

    assert sorted(payload) == [
        "api_version",
        "build_date",
        "commit",
        "platform",
        "version",
    ]
    os_name, _, arch = payload["platform"].partition("/")
    assert os_name == os_name.lower()
    assert arch not in {"x86_64", "aarch64", "i386", "i686"}
