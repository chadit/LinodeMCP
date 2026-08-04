"""Wire-level checks that the newly paginated list tools send page/page_size.

The Go twin is go/internal/linode/pagination_query_test.go. A handler that
advertises page and page_size in its input schema but drops them between the
arguments and the request would return page one forever, which no caller can
see from the response.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import patch

import httpx
import pytest

from linodemcp.gentools import (
    handle_linode_domain_list,
    handle_linode_domain_record_list,
)
from linodemcp.linode import RetryableClient
from linodemcp.tools.linode_firewalls import handle_linode_firewall_list
from linodemcp.tools.linode_nodebalancers import handle_linode_nodebalancer_list
from linodemcp.tools.linode_sshkeys import handle_linode_sshkey_list
from linodemcp.tools.linode_stackscripts import handle_linode_stackscript_list
from linodemcp.tools.linode_volumes import handle_linode_volume_list
from linodemcp.tools.linode_vpc import (
    handle_linode_vpc_ip_all_list,
    handle_linode_vpc_ip_list,
    handle_linode_vpc_list,
    handle_linode_vpc_subnet_list,
)

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from mcp.types import TextContent

    from linodemcp.config import Config

_VPC_ID = 123
_DOMAIN_ID = 123


def _capturing_client_factory(seen: list[httpx.Request]) -> Callable[..., Any]:
    """A RetryableClient factory whose transport records the request it sent.

    Patching the class the handler constructs (rather than the handler's own
    call) keeps the whole path under test: argument parsing, query encoding,
    and route resolution all run for real.
    """

    def handle(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200, json={"data": [], "page": 1, "pages": 1, "results": 0}
        )

    def factory(*args: Any, **kwargs: Any) -> RetryableClient:
        client = RetryableClient(*args, **kwargs)
        client.client.client = httpx.AsyncClient(transport=httpx.MockTransport(handle))
        return client

    return factory


async def _run(
    handler: Callable[[dict[str, Any], Config], Awaitable[list[TextContent]]],
    arguments: dict[str, Any],
    cfg: Config,
) -> httpx.Request:
    """Run one list handler and return the single request it put on the wire."""
    seen: list[httpx.Request] = []
    with patch(
        "linodemcp.tools.helpers.RetryableClient", _capturing_client_factory(seen)
    ):
        result = await handler(arguments, cfg)

    assert not result[0].text.startswith("Error"), result[0].text
    assert not result[0].text.startswith("Failed"), result[0].text
    assert len(seen) == 1
    return seen[0]


_PAGINATED_LISTS: list[tuple[str, Any, dict[str, Any], str]] = [
    ("domains", handle_linode_domain_list, {}, "/v4/domains"),
    (
        "domain records",
        handle_linode_domain_record_list,
        {"domain_id": _DOMAIN_ID},
        "/v4/domains/123/records",
    ),
    ("firewalls", handle_linode_firewall_list, {}, "/v4/networking/firewalls"),
    ("nodebalancers", handle_linode_nodebalancer_list, {}, "/v4/nodebalancers"),
    ("ssh keys", handle_linode_sshkey_list, {}, "/v4/profile/sshkeys"),
    ("stackscripts", handle_linode_stackscript_list, {}, "/v4/linode/stackscripts"),
    ("volumes", handle_linode_volume_list, {}, "/v4/volumes"),
    ("vpcs", handle_linode_vpc_list, {}, "/v4/vpcs"),
    ("vpc ips across all vpcs", handle_linode_vpc_ip_all_list, {}, "/v4/vpcs/ips"),
    (
        "vpc ips",
        handle_linode_vpc_ip_list,
        {"vpc_id": _VPC_ID},
        "/v4/vpcs/123/ips",
    ),
    (
        "vpc subnets",
        handle_linode_vpc_subnet_list,
        {"vpc_id": _VPC_ID},
        "/v4/vpcs/123/subnets",
    ),
]


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("handler", "arguments", "path"),
    [case[1:] for case in _PAGINATED_LISTS],
    ids=[case[0] for case in _PAGINATED_LISTS],
)
async def test_page_pair_reaches_the_query_string(
    handler: Any,
    arguments: dict[str, Any],
    path: str,
    sample_config: Config,
) -> None:
    """A supplied page pair must arrive as page/page_size on the request."""
    request = await _run(
        handler, {**arguments, "page": 2, "page_size": 50}, sample_config
    )

    assert request.method == "GET"
    assert request.url.path == path
    assert request.url.query == b"page=2&page_size=50"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("handler", "arguments", "path"),
    [case[1:] for case in _PAGINATED_LISTS],
    ids=[case[0] for case in _PAGINATED_LISTS],
)
async def test_unset_page_pair_leaves_the_query_empty(
    handler: Any,
    arguments: dict[str, Any],
    path: str,
    sample_config: Config,
) -> None:
    """With no page pair the request stays byte-identical to the older one."""
    request = await _run(handler, dict(arguments), sample_config)

    assert request.url.path == path
    assert request.url.query == b""


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
        ({"page": "2"}, "page must be an integer"),
    ],
)
async def test_out_of_range_page_pair_is_rejected_before_the_request(
    arguments: dict[str, Any], message: str, sample_config: Config
) -> None:
    """Bounds rejection matches Go's standardPaginationFromTool text."""
    seen: list[httpx.Request] = []
    with patch(
        "linodemcp.tools.helpers.RetryableClient", _capturing_client_factory(seen)
    ):
        result = await handle_linode_volume_list(arguments, sample_config)

    assert result[0].text == f"Error: {message}"
    assert seen == []
