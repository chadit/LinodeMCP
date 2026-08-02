"""Surface-wide guard: every paginated tool rejects a bad page before any request.

The Go twin is go/internal/server/pagination_surface_test.go. Both walk the
registered surface rather than naming tools, so a family that adopts pagination
later is covered without editing either test.

This is the guard the per-family readers used to provide implicitly: the shared
reader is the only thing validating page/page_size now, and nothing else proves
each tool still routes through it. A handler that reads the pair itself again,
or validates after the client call, fails here.
"""

from __future__ import annotations

from typing import Any, cast
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.config import Config, EnvironmentConfig, LinodeConfig
from linodemcp.server import get_tool_registry

PAGE_ARG = "page"
PAGE_SIZE_ARG = "page_size"
WANT_REJECTION = "page must be an integer"

# Tools whose other required arguments take a format this test cannot
# synthesize validate those first and report that instead. They still must
# reject without reaching the client, which is asserted for every tool. The
# floor keeps the specific-message assertion honest: if the shared reader
# stopped being wired up, the count would collapse rather than a
# hand-maintained skip list going quietly stale.
MIN_TOOLS_WITH_PAGINATION_MESSAGE = 60


def _paginated_entries() -> list[tuple[str, Any, dict[str, Any]]]:
    """(name, handler, arguments) for every tool taking page and page_size."""
    entries: list[tuple[str, Any, dict[str, Any]]] = []
    for entry in get_tool_registry():
        schema: dict[str, Any] = entry.tool.input_schema or {}
        properties: dict[str, Any] = schema.get("properties", {})
        if PAGE_ARG not in properties or PAGE_SIZE_ARG not in properties:
            continue

        arguments: dict[str, Any] = {PAGE_ARG: "x"}
        for name in schema.get("required", []):
            if name in (PAGE_ARG, PAGE_SIZE_ARG):
                continue
            prop = cast("dict[str, Any]", properties.get(name, {}))
            enum = cast("list[Any]", prop.get("enum") or [])
            if enum:
                arguments[name] = enum[0]
            elif prop.get("type") in {"integer", "number"}:
                arguments[name] = 1
            elif prop.get("type") == "boolean":
                # Never true: a paginated tool that also takes a confirm flag
                # must not be able to execute anything from this test.
                arguments[name] = False
            else:
                arguments[name] = "1"
        entries.append((entry.name, entry.handle_fn, arguments))
    return entries


PAGINATED = _paginated_entries()


def _config() -> Config:
    cfg = Config()
    cfg.environments["default"] = EnvironmentConfig(
        label="default",
        linode=LinodeConfig(api_url="https://api.invalid/v4", token="0123456789abcdef"),
    )
    return cfg


def test_the_surface_actually_has_paginated_tools() -> None:
    """A schema change dropping pagination everywhere must not silence this."""
    assert len(PAGINATED) >= MIN_TOOLS_WITH_PAGINATION_MESSAGE


@pytest.mark.parametrize(
    ("name", "handler", "arguments"),
    PAGINATED,
    ids=[name for name, _, _ in PAGINATED],
)
async def test_paginated_tool_rejects_bad_page_without_calling_the_api(
    name: str, handler: Any, arguments: dict[str, Any]
) -> None:
    """A non-integer page is rejected before any client is built."""
    with patch("linodemcp.tools.helpers.RetryableClient") as client_class:
        client_class.return_value = AsyncMock()

        result = await handler(arguments, _config())

    text = result[0].text
    assert text.startswith("Error: "), f"{name}: page='x' accepted, got {text!r}"
    assert not client_class.called, f"{name}: built a client for an invalid page"


async def test_most_paginated_tools_use_the_shared_rejection_text() -> None:
    """The shared reader must be what rejects, not a per-family reimplementation."""
    matched: list[str] = []
    for name, handler, arguments in PAGINATED:
        with patch("linodemcp.tools.helpers.RetryableClient") as client_class:
            client_class.return_value = AsyncMock()

            result = await handler(arguments, _config())

        if WANT_REJECTION in result[0].text:
            matched.append(name)

    assert len(matched) >= MIN_TOOLS_WITH_PAGINATION_MESSAGE, (
        f"{len(matched)} of {len(PAGINATED)} paginated tools reported "
        f"{WANT_REJECTION!r}; want at least {MIN_TOOLS_WITH_PAGINATION_MESSAGE}"
    )
