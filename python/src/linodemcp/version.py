"""Version information for LinodeMCP."""

import platform
from dataclasses import asdict, dataclass

VERSION = "0.1.0"
API_VERSION = "0.1.0"


@dataclass
class VersionInfo:
    """Build and version information."""

    version: str
    api_version: str
    build_date: str
    git_commit: str
    git_branch: str
    python_version: str
    platform: str

    def to_dict(self) -> dict[str, str]:
        """Convert to dictionary."""
        return asdict(self)

    def __str__(self) -> str:
        """String representation."""
        return (
            f"LinodeMCP v{self.version} "
            f"(MCP: v{self.api_version}, {self.platform}, {self.git_commit})"
        )


def get_version_info(
    build_date: str = "unknown",
    git_commit: str = "unknown",
    git_branch: str = "main",
) -> VersionInfo:
    """Get version information.

    A development build knows no commit, so it says so rather than naming a
    build type nobody chose; Go's appinfo has always spelled it that way.
    """
    return VersionInfo(
        version=VERSION,
        api_version=API_VERSION,
        build_date=build_date,
        git_commit=git_commit,
        git_branch=git_branch,
        python_version=platform.python_version(),
        platform=f"{platform.system()}/{platform.machine()}",
    )
