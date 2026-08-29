"""The two whole-collection state forms page to the end.

Go's FetchCollectionScan and the composite's list call terminate on a short
page; a resource on the second page must be found, not reported missing.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

from linodemcp.tools.drivers import (
    CompositeCall,
    read_collection_scan,
    read_composite_state,
    read_envelope_state,
)
from linodemcp.tools.helpers import STANDARD_PAGE_SIZE_MAX

_FULL_PAGE: list[dict[str, Any]] = [
    {"region": "us-east", "label": f"vl-{index}", "linodes": []}
    for index in range(STANDARD_PAGE_SIZE_MAX)
]


def _paged_client(pages: list[dict[str, Any]]) -> AsyncMock:
    client = AsyncMock()
    client.route_raw.side_effect = pages
    return client


async def test_scan_finds_a_match_past_the_first_page() -> None:
    client = _paged_client(
        [
            {"data": _FULL_PAGE, "page": 1, "pages": 2, "results": 501},
            {
                "data": [{"region": "us-east", "label": "vl-app", "linodes": [7]}],
                "page": 2,
                "pages": 2,
                "results": 501,
            },
        ]
    )

    state = await read_collection_scan(
        client,
        tool="linode_vlan_list",
        matches={"region": "us-east", "label": "vl-app"},
    )

    assert state.text("label") == "vl-app"
    assert client.route_raw.await_count == 2


async def test_composite_list_call_pages_to_the_end() -> None:
    disk_page: list[dict[str, Any]] = [
        {"id": index, "size": 100, "filesystem": "ext4"}
        for index in range(STANDARD_PAGE_SIZE_MAX)
    ]
    client = _paged_client(
        [
            {"id": 123, "type": "g6-nanode-1"},
            {"data": disk_page, "page": 1, "pages": 2, "results": 501},
            {
                "data": [{"id": 999, "size": 50, "filesystem": "ext4"}],
                "page": 2,
                "pages": 2,
                "results": 501,
            },
        ]
    )

    state = await read_composite_state(
        client,
        [
            CompositeCall(
                tool="linode_instance_get",
                member="instance",
                fields=("type",),
                values=(123,),
                is_list=False,
            ),
            CompositeCall(
                tool="linode_instance_disk_list",
                member="disks",
                fields=("id",),
                values=(123,),
                is_list=True,
            ),
        ],
    )

    disks = state.objects("disks")
    assert len(disks) == STANDARD_PAGE_SIZE_MAX + 1
    assert state.object("instance").text("type") == "g6-nanode-1"


async def test_envelope_state_reads_the_page_it_was_given() -> None:
    """A replacement previews the page its own call publishes.

    The behavior fixtures cannot see this: a preview's stub is matched on the
    path alone, so the query rides here. Go's twin is
    TestFetchEnvelopeStateReadsThePageItWasGiven.
    """
    client = _paged_client([{"data": [], "results": 0}])

    await read_envelope_state(
        client,
        123,
        tool="linode_instance_firewall_list",
        query="page=2&page_size=50",
    )

    assert client.route_raw.await_args.kwargs["query"] == "page=2&page_size=50"


async def test_envelope_state_reads_the_route_defaults_when_given_no_page() -> None:
    """A read with no controls to forward calls the route the way it always
    did, so the tag delete's own fetch is unchanged."""
    client = _paged_client([{"data": [], "results": 0}])

    await read_envelope_state(client, "prod", tool="linode_tag_object_list")

    assert "query" not in client.route_raw.await_args.kwargs
