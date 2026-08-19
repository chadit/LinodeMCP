"""Each declared list envelope decodes the body its route really sends.

A generated list tool reads every route through run_list_tool, so which JSON
shape the body is read as comes from the tool's declared list_envelope.
Assuming the standard page for all of them answers a populated collection as an
empty one and reports success, which is the failure these pin. Go's
route_list_envelope_test.go asserts the same four shapes off the same option.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.linode.routes import ListShape, list_envelope_for
from linodemcp.tools.drivers import run_list_tool

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


@pytest.mark.asyncio
async def test_keyed_envelope_reads_the_declared_member(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """The interfaces route names its own member, not data."""
    mock_linode_client.route_raw.return_value = {"interfaces": [{"id": 11}, {"id": 12}]}

    result = await run_list_tool(
        sample_config,
        {},
        tool="linode_instance_interface_list",
        path_values={"linode_id": 7},
    )

    payload = json.loads(result[0].text)
    assert payload["count"] == 2


@pytest.mark.asyncio
async def test_bare_envelope_reads_a_top_level_array(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """The region availability route answers an array with no envelope."""
    mock_linode_client.route_raw.return_value = [
        {"region": "us-east", "plan": "g6-standard-1"}
    ]

    result = await run_list_tool(
        sample_config,
        {},
        tool="linode_region_availability_get",
        path_values={"region_id": "us-east"},
    )

    payload = json.loads(result[0].text)
    assert payload["count"] == 1


@pytest.mark.asyncio
async def test_required_data_refuses_a_page_missing_its_data_member(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """A truncated trusted-device page is a failure, not "no remembered browsers"."""
    mock_linode_client.route_raw.return_value = {"page": 1, "results": 0}

    result = await run_list_tool(sample_config, {}, tool="linode_profile_device_list")

    assert result[0].text.startswith("Failed to retrieve items: ")


@pytest.mark.asyncio
async def test_a_standard_page_still_reads_an_absent_data_member_as_empty(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """The strict reading stays scoped to the routes that declared it."""
    mock_linode_client.route_raw.return_value = {"page": 1, "results": 0}

    result = await run_list_tool(sample_config, {}, tool="linode_domain_list")

    payload = json.loads(result[0].text)
    assert payload["count"] == 0


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "tool",
    ["linode_instance_interface_list", "linode_profile_device_list"],
)
async def test_a_body_that_is_not_an_object_is_refused(
    tool: str, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """A member cannot be read out of an array, so the shape is reported."""
    mock_linode_client.route_raw.return_value = ["not", "an", "object"]

    result = await run_list_tool(
        sample_config,
        {},
        tool=tool,
        path_values={"linode_id": 7} if "instance" in tool else None,
    )

    assert result[0].text.startswith("Failed to retrieve items: ")


@pytest.mark.parametrize(
    ("tool", "shape", "member"),
    [
        ("linode_instance_interface_list", ListShape.KEYED, "interfaces"),
        (
            "linode_profile_security_question_list",
            ListShape.KEYED,
            "security_questions",
        ),
        ("linode_instance_config_interface_list", ListShape.BARE, ""),
        ("linode_region_availability_get", ListShape.BARE, ""),
        ("linode_profile_device_list", ListShape.REQUIRED_DATA, ""),
        ("linode_domain_list", ListShape.DATA, ""),
    ],
)
def test_list_envelope_for_reads_the_declared_shapes(
    tool: str, shape: ListShape, member: str
) -> None:
    """A tool losing its annotation would otherwise only show as an empty list."""
    envelope = list_envelope_for(tool)

    assert envelope.shape is shape
    assert envelope.member == member
