"""Version tool - server version and build information."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import version_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.proto_response import proto_to_canonical_dict
from linodemcp.tools.toolschemas import schema
from linodemcp.version import get_version_info

# Machine names Python's platform.machine() reports mapped to Go's GOARCH so the
# version output's platform value matches the Go server on the same host.
_ARCH_ALIASES = {
    "x86_64": "amd64",
    "aarch64": "arm64",
    "i386": "386",
    "i686": "386",
}


def create_version_tool() -> tuple[Tool, Capability]:
    """Create the version tool."""
    return Tool(
        name="version",
        description="Returns LinodeMCP server version and build information",
        inputSchema=schema("linode.mcp.v1.VersionInput"),
    ), Capability.Meta


def _normalized_platform(raw: str) -> str:
    """Normalize a "system/machine" string to Go's runtime.GOOS/GOARCH naming.

    Python reports "Darwin/arm64" or "Linux/x86_64"; Go reports "darwin/arm64"
    or "linux/amd64". Lowercasing the OS and aliasing the common arch names lines
    the two servers' version output up on the same host.
    """
    os_name, _, arch = raw.partition("/")
    return f"{os_name.lower()}/{_ARCH_ALIASES.get(arch, arch)}"


def version_response_dict() -> dict[str, Any]:
    """The canonical VersionResponse payload as a dict.

    The one construction of the version envelope: the tool handler and the
    CLI ``version`` verb both serialize this proto message, so every path in
    every language emits the field set version.proto pins (the shape the
    testdata/conformance/version_response.json fixture locks).
    """
    info = get_version_info()
    message = version_pb2.VersionResponse(
        version=info.version,
        api_version=info.api_version,
        build_date=info.build_date,
        commit=info.git_commit,
        platform=_normalized_platform(info.platform),
    )
    return proto_to_canonical_dict(message)


async def handle_version(_arguments: dict[str, Any]) -> list[TextContent]:
    """Handle version tool request."""
    return [
        TextContent(type="text", text=json.dumps(version_response_dict(), indent=2))
    ]
