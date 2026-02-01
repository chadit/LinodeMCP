"""Unit tests for version module."""

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
