"""Cross-language behavior conformance runner.

Replays the shared fixtures in ``testdata/behavior/`` through the real server
dispatch path (registration, profile filter, handler, client) with the HTTP
transport faked at the ``httpx.AsyncClient.request`` boundary; no network, no
credentials. The Go runner
(``go/internal/server/behavior_conformance_test.go``) replays the same cases,
so a handler whose validation, coercion, error text, or outgoing HTTP request
drifts from the other language fails one of the two runners.

Outcome contract per case, exactly one of: ``expect_error`` is the exact bare
validation message and forbids an HTTP call; ``expect_api_error`` is a shared
substring in an error produced after at least one HTTP call; ``expect_request``
is the method, path, and JSON body of the one HTTP call the handler must make;
``expect_result`` is the successful response content, compared as parsed JSON
so formatting is irrelevant. Substring matching for ``expect_api_error`` keeps
language-specific error framing outside the shared contract.

The fake API answers from ``api_responses`` when present: keys are
"METHOD /path" with the query string stripped (per-language pagination
params must not fragment the routing), values are the JSON bodies to serve.
A request with no matching key fails the case, but an unused key does not:
implementations may fetch equivalent data from different endpoints, and the
contract these fixtures pin is the OUTPUT, not the fetch pattern. Without
``api_responses`` the single ``api_response`` (or ``{}``) answers every
request. A case whose args include ``dry_run: true`` additionally asserts
every captured request is a GET: a dry run may read whatever it needs for
its preview but must never mutate.
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


def _behavior_outcome_count(case: dict[str, Any]) -> int:
    """Count usable outcome assertions, excluding empty error strings."""
    return sum(
        (
            bool(case.get("expect_error")),
            bool(case.get("expect_api_error")),
            case.get("expect_request") is not None,
            "expect_result" in case,
        )
    )


def _resolve_response(
    api_responses: dict[str, Any] | None,
    api_response: Any,
    method: str,
    url: str,
    unmatched: list[str],
) -> tuple[int, Any]:
    """Pick the fake reply for one request.

    Routed mode (``api_responses``) matches on "METHOD /path" with the query
    string stripped, mirroring the Go runner. A miss is recorded in
    ``unmatched`` so the test fails loudly, and served as a 404.
    """
    if api_responses is None:
        return 200, api_response

    path = url.removeprefix(_FAKE_API_URL).split("?", 1)[0]
    key = f"{method} {path}"
    if key not in api_responses:
        unmatched.append(key)
        return 404, {}

    return 200, api_responses[key]


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
    outcome_count = _behavior_outcome_count(case)
    assert outcome_count == 1, (
        f"{tool}/{case_name}: {outcome_count} outcome fields set; "
        "want exactly 1 non-empty outcome"
    )

    captured: list[tuple[str, str, Any]] = []
    unmatched: list[str] = []
    api_response = case.get("api_response", {})
    api_responses: dict[str, Any] | None = case.get("api_responses")

    async def _fake_request(
        _self: httpx.AsyncClient, method: str, url: str, **kwargs: Any
    ) -> httpx.Response:
        captured.append((method, url, kwargs.get("json")))
        status, body = _resolve_response(
            api_responses, api_response, method, url, unmatched
        )
        return httpx.Response(
            status,
            content=json.dumps(body).encode(),
            headers={"content-type": "application/json"},
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

    assert not unmatched, (
        f"{tool}/{case_name}: requests with no api_responses entry: "
        f"{', '.join(unmatched)}"
    )

    if case["args"].get("dry_run") is True:
        non_get = sorted({method for method, _, _ in captured if method != "GET"})
        assert not non_get, (
            f"{tool}/{case_name}: dry-run issued {', '.join(non_get)}; "
            "only GET is allowed"
        )

    expect_error = case.get("expect_error")
    if expect_error:
        assert text == f"Error: {expect_error}", (
            f"{tool}/{case_name}: error text {text!r}, "
            f"want {'Error: ' + expect_error!r}"
        )
        assert captured == [], f"{tool}/{case_name}: no HTTP call expected"
        return

    expect_api_error = case.get("expect_api_error")
    if expect_api_error:
        # Go receives an MCP isError bit; direct Python dispatch exposes only
        # TextContent, so validate Python's local error framing before the
        # shared, language-independent substring.
        assert text.startswith(("Error: ", "Failed to ")), (
            f"{tool}/{case_name}: expected an API error, got {text!r}"
        )
        assert expect_api_error in text, (
            f"{tool}/{case_name}: error text {text!r} does not contain "
            f"{expect_api_error!r}"
        )
        assert captured, f"{tool}/{case_name}: at least one HTTP call expected"
        return

    if "expect_result" in case:
        expect_result = case["expect_result"]
        assert not text.startswith("Error:"), f"{tool}/{case_name}: unexpected {text!r}"
        assert json.loads(text) == expect_result, (
            f"{tool}/{case_name}: result mismatch\ngot:\n{text}\n"
            f"want:\n{json.dumps(expect_result, indent=2)}"
        )
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


@pytest.mark.parametrize(
    ("case", "expected"),
    [
        ({"expect_api_error": ""}, 0),
        ({"expect_api_error": "response", "expect_result": False}, 2),
        ({"expect_result": False}, 1),
        ({"expect_result": None}, 1),
        ({"expect_error": "", "expect_api_error": "", "expect_result": False}, 1),
    ],
    ids=[
        "empty-api-error",
        "multiple-outcomes",
        "false-result",
        "null-result",
        "empty-errors-with-result",
    ],
)
def test_behavior_outcome_count(case: dict[str, Any], expected: int) -> None:
    """Outcome validation rejects empty and ambiguous fixture contracts."""
    assert _behavior_outcome_count(case) == expected
