"""Behavioral tests for previously-untested helpers in tools/helpers.py.

Exercises the public helpers whose branches were never hit: the billing_delta
arm of the dry-run envelope, the dataclass JSON fallback used when a dry-run
current_state is a dataclass model, truncate_string, the live-config hot-reload
bridge, and the shared execute_tool_list / execute_dry_run environment + error
handling paths.
"""

from __future__ import annotations

import dataclasses
import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.tools.helpers import build_dry_run_response, truncate_string

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.config import Config


@dataclasses.dataclass
class _Sample:
    """Tiny dataclass standing in for a fetched read-model current_state."""

    a: int
    b: str


def test_build_dry_run_response_includes_billing_delta() -> None:
    """A supplied billing_delta lands in the serialized dry-run envelope.

    The v0 wire shape hides the field when unset; this checks the populated
    arm survives the DryRunResponse proto round-trip with both sub-fields.
    """
    result = build_dry_run_response(
        "linode_volume_resize",
        "prod",
        "PUT",
        "/volumes/1",
        None,
        billing_delta={"monthly_change_usd": "5.00", "note": "prorated"},
    )

    payload = json.loads(result[0].text)
    assert payload["billing_delta"] == {
        "monthly_change_usd": "5.00",
        "note": "prorated",
    }


def test_build_dry_run_response_omits_empty_billing_delta() -> None:
    """An empty billing_delta is dropped rather than serialized as ``{}``."""
    result = build_dry_run_response(
        "linode_volume_resize", "prod", "PUT", "/volumes/1", None, billing_delta={}
    )

    payload = json.loads(result[0].text)
    assert "billing_delta" not in payload


def test_build_dry_run_response_serializes_dataclass_current_state() -> None:
    """A dataclass current_state is flattened to a dict via the JSON fallback.

    Without the dataclass default, json.dumps would raise on the model; this
    confirms the fallback turns it into the plain object the proto envelope
    accepts.
    """
    result = build_dry_run_response(
        "linode_instance_resize", "prod", "PUT", "/x/1", _Sample(1, "x")
    )

    payload = json.loads(result[0].text)
    assert payload["current_state"] == {"a": 1, "b": "x"}


def test_build_dry_run_response_rejects_unserializable_current_state() -> None:
    """A non-dataclass, non-JSON current_state surfaces as a TypeError.

    The JSON fallback only handles dataclasses; anything else is a bug in the
    caller and must not be silently swallowed.
    """
    with pytest.raises(TypeError, match="not JSON serializable"):
        build_dry_run_response(
            "linode_instance_resize", "prod", "PUT", "/x/1", object()
        )


def test_truncate_string_appends_ellipsis_over_limit() -> None:
    """A value longer than the limit is cut and suffixed with an ellipsis."""
    assert truncate_string("abcdefgh", 3) == "abc..."


def test_truncate_string_returns_value_at_or_under_limit() -> None:
    """A value within the limit is returned unchanged, no ellipsis."""
    assert truncate_string("abc", 3) == "abc"
    assert truncate_string("ab", 5) == "ab"


def _cm_client() -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


def _capture_retry_max(captured: dict[str, int]) -> Callable[..., AsyncMock]:
    """A RetryableClient stand-in that records the max_retries of the
    RetryConfig it was built with, then behaves as the async-cm client."""

    def _factory(api_url: str, token: str, retry_config: Any) -> AsyncMock:
        captured["max_retries"] = retry_config.max_retries
        return _cm_client()

    return _factory


async def test_live_config_source_overrides_snapshot(sample_config: Config) -> None:
    """A registered live source wins over the passed snapshot: a tool builds
    its client from the live config's resilience, not the snapshot's."""
    from linodemcp.tools import helpers

    live = dataclasses.replace(
        sample_config,
        resilience=dataclasses.replace(sample_config.resilience, max_retries=99),
    )
    captured: dict[str, int] = {}

    async def _callback(client: object) -> list[dict[str, Any]]:
        return []

    helpers.set_live_config_source(lambda: live)
    try:
        with patch(
            "linodemcp.tools.helpers.RetryableClient",
            side_effect=_capture_retry_max(captured),
        ):
            await helpers.execute_tool_list(sample_config, {}, "list things", _callback)
    finally:
        helpers.set_live_config_source(None)

    assert captured["max_retries"] == 99
    assert sample_config.resilience.max_retries != 99


async def test_resolve_config_uses_snapshot_without_live_source(
    sample_config: Config,
) -> None:
    """With no live source registered, the client is built from the passed
    snapshot's resilience settings."""
    from linodemcp.tools import helpers

    captured: dict[str, int] = {}

    async def _callback(client: object) -> list[dict[str, Any]]:
        return []

    helpers.set_live_config_source(None)
    with patch(
        "linodemcp.tools.helpers.RetryableClient",
        side_effect=_capture_retry_max(captured),
    ):
        await helpers.execute_tool_list(sample_config, {}, "list things", _callback)

    assert captured["max_retries"] == sample_config.resilience.max_retries


async def test_execute_tool_list_returns_json_on_success(
    sample_config: Config,
) -> None:
    """A successful callback result is JSON-serialized into the TextContent."""
    from linodemcp.tools import helpers

    async def _callback(client: object) -> list[dict[str, Any]]:
        return [{"id": 1, "label": "web"}]

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_tool_list(
            sample_config, {}, "list things", _callback
        )

    assert json.loads(result[0].text) == [{"id": 1, "label": "web"}]


async def test_execute_tool_list_reports_missing_environment(
    sample_config: Config,
) -> None:
    """An unknown environment short-circuits to the user-facing Error path."""
    from linodemcp.tools import helpers

    async def _callback(client: object) -> list[dict[str, Any]]:
        return []

    result = await helpers.execute_tool_list(
        sample_config, {"environment": "nope"}, "list things", _callback
    )

    assert result[0].text.startswith("Error:")
    assert "nope" in result[0].text


async def test_execute_tool_list_wraps_api_error(sample_config: Config) -> None:
    """An APIError raised by the callback becomes a 'Failed to ...' message."""
    from linodemcp.linode import APIError
    from linodemcp.tools import helpers

    async def _callback(client: object) -> list[dict[str, Any]]:
        raise APIError(500, "upstream boom")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_tool_list(
            sample_config, {}, "list volumes", _callback
        )

    assert result[0].text.startswith("Failed to list volumes")
    assert "upstream boom" in result[0].text


async def test_execute_tool_list_reports_unexpected_error(
    sample_config: Config,
) -> None:
    """An unexpected exception is logged and surfaced as a failure message."""
    from linodemcp.tools import helpers

    async def _callback(client: object) -> list[dict[str, Any]]:
        msg = "kaboom"
        raise RuntimeError(msg)

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_tool_list(
            sample_config, {}, "list volumes", _callback
        )

    assert result[0].text.startswith("Failed to list volumes")
    assert "kaboom" in result[0].text


async def test_execute_dry_run_wraps_api_error(sample_config: Config) -> None:
    """An APIError while fetching state maps to the dry-run failure message."""
    from linodemcp.linode import APIError
    from linodemcp.tools import helpers

    async def _fetch(client: object) -> object:
        raise APIError(500, "state fetch boom")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_dry_run(
            sample_config, {}, "linode_x_delete", "DELETE", "/x/1", _fetch
        )

    assert result[0].text.startswith("Failed to fetch state for dry-run")
    assert "state fetch boom" in result[0].text


async def test_execute_dry_run_reports_unexpected_error(
    sample_config: Config,
) -> None:
    """An unexpected fetch error is logged and surfaced as a dry-run failure."""
    from linodemcp.tools import helpers

    async def _fetch(client: object) -> object:
        msg = "unexpected"
        raise RuntimeError(msg)

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_dry_run(
            sample_config, {}, "linode_x_delete", "DELETE", "/x/1", _fetch
        )

    assert result[0].text.startswith("Failed to fetch state for dry-run")
    assert "unexpected" in result[0].text
