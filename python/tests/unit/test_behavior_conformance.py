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
request. ``api_response_raw`` serves exact bytes instead, including empty or
malformed JSON, and ``api_status`` overrides HTTP 200 in single-response mode.
A case whose args include ``dry_run: true`` additionally asserts
every captured request is a GET: a dry run may read whatever it needs for
its preview but must never mutate.
"""

from __future__ import annotations

import dataclasses
import json
from contextlib import asynccontextmanager
from pathlib import Path
from typing import TYPE_CHECKING, Any
from unittest.mock import patch

import httpx
import pytest

from linodemcp.config import (
    BuiltinOverride,
    Config,
    EnvironmentConfig,
    LinodeConfig,
    ObjectStorageConfig,
)
from linodemcp.linode.routes import DEFAULT_SURFACE_SEGMENT
from linodemcp.server import Server

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator

_BEHAVIOR_DIR = Path(__file__).resolve().parents[3] / "testdata" / "behavior"
_FAKE_ORIGIN = "http://linode.test"
_FAKE_API_URL = f"{_FAKE_ORIGIN}/{DEFAULT_SURFACE_SEGMENT}"


def _split_surface(url: str) -> tuple[str, str]:
    """Take the surface segment and the path off a request URL.

    The fake base ends in a version segment the way a real apiUrl does, so every
    request carries one and a tool that moved surfaces is visible here. Splitting
    rather than stripping a fixed prefix is what keeps "/v4beta/locks" from
    reading as the path "beta/locks". The Go runner splits the same way.
    """
    rest = url.removeprefix(_FAKE_ORIGIN).removeprefix("/")
    surface, _, path = rest.partition("/")
    return surface, "/" + path


def _behavior_cases() -> list[tuple[str, str, dict[str, Any]]]:
    """Load every fixture case as (tool, case name, case dict)."""
    cases: list[tuple[str, str, dict[str, Any]]] = []
    for path in sorted(_BEHAVIOR_DIR.glob("*.json")):
        fixture: dict[str, Any] = json.loads(path.read_text())
        cases.extend((fixture["tool"], case["name"], case) for case in fixture["cases"])
    return cases


def _behavior_config(case: dict[str, Any]) -> Config:
    """Full-access config pointed at the fake API URL.

    A case may overlay the Object Storage data-plane block by name, which is how
    the over-ceiling case runs against a 2 KB file instead of a 5 GiB one. Only
    that block is reachable: a general overlay would let a fixture change the
    profile or the auth it is being tested under.
    """
    overlay = case.get("config", {}).get("object_storage", {})
    object_storage = ObjectStorageConfig()
    if "max_single_part_bytes" in overlay:
        object_storage = dataclasses.replace(
            object_storage, max_single_part_bytes=overlay["max_single_part_bytes"]
        )

    base = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(api_url=_FAKE_API_URL, token="test-token"),
            )
        },
        object_storage=object_storage,
    )
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={"full-access": BuiltinOverride(disabled=False)},
    )


def _materialize_files(case: dict[str, Any], tmp_path: Path) -> dict[str, Any]:
    """Write the case's declared files and resolve {{file:name}} in its args.

    A tool whose whole job is reading a local file cannot be pinned by a fixture
    that never puts one on disk. ``content`` writes literal text; ``size`` with
    ``fill`` generates a file too large to spell out in the fixture.

    ``{{file_dir}}`` names the directory itself, which is what a tool that
    WRITES a file needs: a destination inside a directory the case owns and that
    no file occupies yet.
    """
    declared: dict[str, Any] = case.get("files", {})

    paths: dict[str, str] = {}
    for name, spec in declared.items():
        target = tmp_path / name
        if spec.get("size"):
            target.write_bytes(spec.get("fill", "a").encode() * spec["size"])
        else:
            target.write_text(spec.get("content", ""))
        paths[name] = str(target)

    resolved: dict[str, Any] = {}
    for key, value in case["args"].items():
        if not isinstance(value, str):
            resolved[key] = value
            continue
        substituted = value
        for name, path in paths.items():
            substituted = substituted.replace(f"{{{{file:{name}}}}}", path)
        resolved[key] = substituted.replace("{{file_dir}}", str(tmp_path))

    return resolved


def _assert_case_shape(tool: str, case_name: str, case: dict[str, Any]) -> None:
    """Refuse a fixture case that asserts nothing or contradicts itself."""
    outcome_count = _behavior_outcome_count(case)
    assert outcome_count == 1, (
        f"{tool}/{case_name}: {outcome_count} outcome fields set; "
        "want exactly 1 non-empty outcome"
    )
    assert not ("api_response" in case and "api_response_raw" in case), (
        f"{tool}/{case_name}: api_response and api_response_raw are mutually exclusive"
    )
    assert not (
        "api_responses" in case
        and any(
            name in case for name in ("api_response", "api_response_raw", "api_status")
        )
    ), f"{tool}/{case_name}: api_responses cannot use single-response fields"


async def _drain(content: Any) -> None:
    """Consume a streamed request body so the bytes actually move.

    The patch replaces ``request`` before httpx ever reads the body, so a
    streaming upload's iterator would otherwise be dropped unread and the
    transfer would report a byte count nothing produced.
    """
    if content is None or isinstance(content, bytes | str):
        return

    async for _ in content:
        pass


def _resolve_api_base(api_responses: dict[str, Any] | None) -> dict[str, Any] | None:
    """Point {{api_base}} at the fake API.

    The Go runner substitutes its httptest port here. Python's fake origin is
    fixed, but the substitution has to exist in both so one fixture can mint a
    presigned URL that points back at the fake API.
    """
    if api_responses is None:
        return None

    return dict(
        json.loads(json.dumps(api_responses).replace("{{api_base}}", _FAKE_API_URL))
    )


def _substitute_file_paths(text: str, tmp_path: Path, files: dict[str, Any]) -> str:
    """Resolve the file tokens in one string.

    The expected error text needs them as much as the arguments do: a refusal
    that names the local path it refused cannot be pinned any other way,
    because the path is a per-run temp directory.
    """
    for name in files:
        text = text.replace(f"{{{{file:{name}}}}}", str(tmp_path / name))

    return text.replace("{{file_dir}}", str(tmp_path))


async def _dispatch_faked(
    srv: Server,
    tool: str,
    arguments: dict[str, Any],
    fake_request: Any,
    fake_stream: Any,
) -> list[Any]:
    """Dispatch one call with both httpx entry points faked.

    autospec keeps the bound-method signature, so each fake receives the client
    instance as its first argument. Both are patched because httpx routes
    .stream() through send() rather than request(), so a streaming transfer
    would otherwise escape the fake and reach the network.
    """
    with (
        patch.object(httpx.AsyncClient, "request", autospec=True) as mock_req,
        patch.object(httpx.AsyncClient, "stream", autospec=True) as mock_stream,
    ):
        mock_req.side_effect = fake_request
        mock_stream.side_effect = fake_stream
        return await srv.dispatch(tool, arguments)


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
    api_response_present: bool,
    api_response_raw: str | None,
    api_status: int | None,
    method: str,
    url: str,
    unmatched: list[str],
) -> tuple[int, bytes]:
    """Pick the fake reply for one request.

    Routed mode (``api_responses``) matches on "METHOD /path" with the query
    string stripped, mirroring the Go runner. A miss is recorded in
    ``unmatched`` so the test fails loudly, and served as a 404.
    """
    if api_responses is None:
        if api_response_raw is not None:
            body = api_response_raw.encode()
        elif api_response_present:
            body = json.dumps(api_response).encode()
        else:
            body = b"{}"
        return 200 if api_status is None else api_status, body

    _, path = _split_surface(url.split("?", 1)[0])
    key = f"{method} {path}"
    if key not in api_responses:
        unmatched.append(key)
        return 404, b"{}"

    return 200, json.dumps(api_responses[key]).encode()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("tool", "case_name", "case"),
    _behavior_cases(),
    ids=[f"{tool}/{name}" for tool, name, _ in _behavior_cases()],
)
async def test_behavior_conformance(
    tool: str, case_name: str, case: dict[str, Any], tmp_path: Path
) -> None:
    """One shared fixture case must produce its contracted outcome."""
    _assert_case_shape(tool, case_name, case)

    captured: list[tuple[str, str, Any]] = []
    unmatched: list[str] = []
    api_response = case.get("api_response")
    api_responses = _resolve_api_base(case.get("api_responses"))
    api_response_headers: dict[str, Any] = case.get("api_response_headers", {})

    async def _fake_request(
        _self: httpx.AsyncClient, method: str, url: str, **kwargs: Any
    ) -> httpx.Response:
        await _drain(kwargs.get("content"))
        captured.append((method, url, kwargs.get("json")))
        status, content = _resolve_response(
            api_responses,
            api_response,
            "api_response" in case,
            case.get("api_response_raw"),
            case.get("api_status"),
            method,
            url,
            unmatched,
        )
        _, path = _split_surface(url.split("?", 1)[0])
        headers = {"content-type": "application/json"}
        headers.update(api_response_headers.get(f"{method} {path}", {}))
        return httpx.Response(
            status,
            content=content,
            request=httpx.Request(method, url),
            headers=headers,
        )

    @asynccontextmanager
    async def _fake_stream(
        _self: httpx.AsyncClient, method: str, url: str, **kwargs: Any
    ) -> AsyncGenerator[httpx.Response]:
        """Intercept streamed reads the way _fake_request intercepts buffered ones.

        httpx routes .stream() through send() rather than request(), so a
        streaming download would otherwise escape the fake and reach the
        network. The Go runner needs no equivalent: its fake is a real httptest
        server, so every verb already lands on it.
        """
        yield await _fake_request(_self, method, url, **kwargs)

    result = await _dispatch_faked(
        Server(_behavior_config(case)),
        tool,
        _materialize_files(case, tmp_path),
        _fake_request,
        _fake_stream,
    )

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
        expect_error = _substitute_file_paths(
            expect_error, tmp_path, case.get("files", {})
        )
        # Python prefixes a local validation failure with "Error: " and Go does
        # not, so the prefix is framing rather than contract. A hook that
        # refuses before any call reports the tool's own declared sentence with
        # no prefix at all, and both shapes are the same fact: this text, and no
        # HTTP call. Comparing with the prefix stripped covers both.
        assert text.removeprefix("Error: ") == expect_error, (
            f"{tool}/{case_name}: error text {text!r}, want {expect_error!r}"
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
    surface, path = _split_surface(url)
    # Absent means v4, so a tool that moved surfaces without its fixture moving
    # with it fails here rather than passing quietly.
    want_surface = expect_request.get("api_surface", DEFAULT_SURFACE_SEGMENT)
    assert surface == want_surface, (
        f"{tool}/{case_name}: api surface {surface}, want {want_surface}"
    )
    assert path == expect_request["path"], f"{tool}/{case_name}: path {path}"
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


@pytest.mark.parametrize(
    ("raw", "status", "expected_body", "expected_status"),
    [
        ("", 400, b"", 400),
        ('{"id":', None, b'{"id":', 200),
    ],
    ids=["empty-error", "malformed-success"],
)
def test_resolve_response_preserves_raw_body_and_status(
    raw: str,
    status: int | None,
    expected_body: bytes,
    expected_status: int,
) -> None:
    """Single-response mode preserves non-JSON bodies and non-200 status."""
    got_status, got_body = _resolve_response(
        None,
        None,
        False,
        raw,
        status,
        "POST",
        _FAKE_API_URL + "/domains",
        [],
    )
    assert got_status == expected_status
    assert got_body == expected_body
