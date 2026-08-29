"""The build-information envelope the CLI ``version`` verb prints.

The MCP tool reaches the same construction through the generated arm, which
projects ``build_info`` the way this does, so both surfaces answer one field
set.
"""

from typing import Any

from linodemcp.genlocal import project_version_response
from linodemcp.tools.operations import build_info


def version_response_dict() -> dict[str, Any]:
    """The canonical VersionResponse payload as a dict."""
    return project_version_response(build_info())
