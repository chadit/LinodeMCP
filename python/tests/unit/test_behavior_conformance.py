"""Cross-language behavior conformance runner.

Replays the shared fixtures in ``testdata/behavior/`` through the real server
dispatch path (registration, profile filter, handler, client) with the HTTP
transport faked at the ``httpx.AsyncClient.request`` boundary; no network, no
credentials. The Go runner
(``go/internal/server/behavior_conformance_test.go``) replays the same cases,
so a handler whose validation, coercion, error text, or outgoing HTTP request
drifts from the other language fails one of the two runners.

Outcome contract per case: ``expect_error`` is the bare validation message
(this runner strips Python's ``Error: `` framing before comparing), or
``expect_request`` is the method, path, and JSON body of the one HTTP call
the handler must make (``api_response`` is served back as the canned reply).
"""

from __future__ import annotations

import dataclasses
import json
from pathlib import Path
from typing import Any
from unittest.mock import patch

import httpx
import pytest

from linodemcp.config import BuiltinOverride, Config, EnvironmentConfig, LinodeConfig
from linodemcp.server import Server

_BEHAVIOR_DIR = Path(__file__).resolve().parents[3] / "testdata" / "behavior"
_FAKE_API_URL = "http://linode.test/v4"


def _behavior_cases() -> list[tuple[str, str, dict[str, Any]]]:
    """Load every fixture case as (tool, case name, case dict)."""
    cases: list[tuple[str, str, dict[str, Any]]] = []
    for path in sorted(_BEHAVIOR_DIR.glob("*.json")):
        fixture: dict[str, Any] = json.loads(path.read_text())
        cases.extend((fixture["tool"], case["name"], case) for case in fixture["cases"])
    return cases


def _behavior_config() -> Config:
    """Full-access config pointed at the fake API URL."""
    base = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(api_url=_FAKE_API_URL, token="test-token"),
            )
        }
    )
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={"full-access": BuiltinOverride(disabled=False)},
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("tool", "case_name", "case"),
    _behavior_cases(),
    ids=[f"{tool}/{name}" for tool, name, _ in _behavior_cases()],
)
async def test_behavior_conformance(
    tool: str, case_name: str, case: dict[str, Any]
) -> None:
    """One shared fixture case must produce its contracted outcome."""
    captured: list[tuple[str, str, Any]] = []
    api_response = case.get("api_response", {})

    async def _fake_request(
        _self: httpx.AsyncClient, method: str, url: str, **kwargs: Any
    ) -> httpx.Response:
        captured.append((method, url, kwargs.get("json")))
        return httpx.Response(
            200,
            json=api_response,
            request=httpx.Request(method, url),
        )

    srv = Server(_behavior_config())

    # autospec keeps the bound-method signature, so _fake_request receives
    # the client instance as its first argument.
    with patch.object(httpx.AsyncClient, "request", autospec=True) as mock_req:
        mock_req.side_effect = _fake_request
        result = await srv.dispatch(tool, dict(case["args"]))

    assert len(result) == 1, f"{tool}/{case_name}: expected one content item"
    text: str = result[0].text

    expect_error = case.get("expect_error")
    if expect_error is not None:
        assert text == f"Error: {expect_error}", (
            f"{tool}/{case_name}: error text {text!r}, "
            f"want {'Error: ' + expect_error!r}"
        )
        assert captured == [], f"{tool}/{case_name}: no HTTP call expected"
        return

    expect_request = case["expect_request"]
    assert not text.startswith("Error:"), f"{tool}/{case_name}: unexpected {text!r}"
    assert len(captured) == 1, (
        f"{tool}/{case_name}: captured {len(captured)} requests, want 1"
    )

    method, url, body = captured[0]
    assert method == expect_request["method"], f"{tool}/{case_name}: method {method}"
    assert url == _FAKE_API_URL + expect_request["path"], (
        f"{tool}/{case_name}: url {url}"
    )
    if "body" in expect_request:
        assert body == expect_request["body"], (
            f"{tool}/{case_name}: body {body!r}, want {expect_request['body']!r}"
        )
