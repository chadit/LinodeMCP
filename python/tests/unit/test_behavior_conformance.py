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
import re
from contextlib import asynccontextmanager
from datetime import datetime
from pathlib import Path
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import patch

import httpx
import pytest

from linodemcp.audit import (
    ACTIVE_LOG_FILE_NAME,
    USER_AUDIT_DIR_RELATIVE,
    SQLiteSink,
)
from linodemcp.config import (
    DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS,
    AuditConfig,
    AuditSQLiteConfig,
    BuiltinOverride,
    Config,
    EnvironmentConfig,
    LinodeConfig,
    ObjectStorageConfig,
    ReportConfig,
    ReportFilter,
    write_atomic,
)
from linodemcp.genlocal import audit_event_from_record
from linodemcp.linode.routes import DEFAULT_SURFACE_SEGMENT
from linodemcp.server import Server
from linodemcp.tools.clock import reset_clock, set_clock

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator, Callable

    from _pytest.monkeypatch import MonkeyPatch

_BEHAVIOR_DIR = Path(__file__).resolve().parents[3] / "testdata" / "behavior"
_FAKE_ORIGIN = "http://linode.test"
_FAKE_API_URL = f"{_FAKE_ORIGIN}/{DEFAULT_SURFACE_SEGMENT}"

# Every key a case may carry. A field one runner honors and the other ignores is
# exactly the drift these fixtures exist to stop, so an unrecognized key fails
# the case in both languages rather than passing quietly in one. The Go runner
# holds the same set in behaviorCaseFields().
_CASE_FIELDS = frozenset(
    {
        "name",
        "args",
        "api_response",
        "api_response_raw",
        "api_status",
        "api_responses",
        "api_response_headers",
        "files",
        "config",
        "expect_api_error",
        "expect_error",
        "expect_request",
        "expect_requests",
        "expect_result",
        "setup_calls",
        "audit_store",
        "now",
        "host_decided",
    }
)

# Type names a host-decided field may declare, and the two audit backends a
# seeded store may choose.
_HOST_DECIDED_STRING = "string"
_HOST_DECIDED_INTEGER = "integer"
_STORE_JSONL = "jsonl"
_STORE_SQLITE = "sqlite"

# The fields that make a case stateful: a prior call against the same server, a
# seeded store, or a fixed clock. Those cases redirect process-global paths, so
# each one owns its own audit directory and config file.
_STATEFUL_FIELDS = ("setup_calls", "audit_store", "now")


@dataclasses.dataclass(frozen=True)
class _RunnerPaths:
    """Per-run locations the runner owns and a fixture names through a token.

    Neither can be spelled in a shared contract file, so the runner picks them
    and resolves the token in the expected result.
    """

    audit_dir: str = ""
    sqlite_path: str = ""


# What a stateless case runs under: the ambient environment, no seeded store.
_AMBIENT_PATHS = _RunnerPaths()


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


def _behavior_config(
    case: dict[str, Any], paths: _RunnerPaths = _AMBIENT_PATHS
) -> Config:
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

    # The seeded database is what turns the sink on for the read path; nothing
    # in a fixture may reach the audit config directly.
    sqlite = AuditSQLiteConfig(
        enabled=bool(paths.sqlite_path),
        path=paths.sqlite_path,
        busy_timeout_ms=DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS,
    )

    base = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(api_url=_FAKE_API_URL, token="test-token"),
            )
        },
        object_storage=object_storage,
        audit=AuditConfig(reports=_behavior_reports(case), sqlite=sqlite),
    )
    return dataclasses.replace(
        base,
        active_profile="full-access",
        profiles_builtin_overrides={"full-access": BuiltinOverride(disabled=False)},
    )


def _behavior_reports(case: dict[str, Any]) -> dict[str, ReportConfig]:
    """Build the named-report catalog a case declared.

    Only the report catalog is reachable from the audit block: it is the one
    audit setting whose absence makes a tool unpinnable, and it cannot change
    the profile or the auth the case runs under.
    """
    audit_overlay: dict[str, Any] = case.get("config", {}).get("audit", {})
    declared: dict[str, Any] = audit_overlay.get("reports", {})

    return {
        name: ReportConfig(
            description=spec.get("description", ""),
            filter=ReportFilter(**spec.get("filter", {})),
            group_by=list(spec.get("group_by", [])),
            output=spec.get("output", ""),
            limit=spec.get("limit", 0),
        )
        for name, spec in declared.items()
    }


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


def _is_stateful(case: dict[str, Any]) -> bool:
    """Report whether the case needs the process-scoped setup."""
    return any(case.get(field) for field in _STATEFUL_FIELDS)


def _prepare_state(
    case: dict[str, Any], tmp_path: Path, monkeypatch: MonkeyPatch
) -> _RunnerPaths:
    """Redirect the process-global paths the meta tools read and seed the store.

    A stateless case keeps the ambient environment, which is what every existing
    fixture already runs under. A stateful one gets its own audit directory and
    its own config file: a draft save writes ``get_config_path()`` for real, so
    the redirect is what keeps a success fixture off the operator's config.
    Returns the locations the case's expected result can name.
    """
    if not _is_stateful(case):
        return _RunnerPaths()

    state_home = tmp_path / "state"
    monkeypatch.setenv("XDG_STATE_HOME", str(state_home))
    config_path = tmp_path / "config.yaml"
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(config_path))
    _seed_config(config_path)

    audit_dir = state_home / USER_AUDIT_DIR_RELATIVE
    store = case.get("audit_store")
    sqlite_path = ""
    if store is not None:
        sqlite_path = _seed_audit_store(audit_dir, tmp_path, store)

    return _RunnerPaths(audit_dir=str(audit_dir), sqlite_path=sqlite_path)


def _seed_config(path: Path) -> None:
    """Put a loadable config at the redirected path.

    A draft save reads the file before it merges, so the redirect alone would
    answer a missing-file refusal instead of the success the fixture pins.
    Writing through the config package's own writer is what makes the seed
    loadable by definition rather than by a hand-kept literal.
    """
    write_atomic(path, _behavior_config({}))


def _seed_audit_store(audit_dir: Path, tmp_path: Path, store: dict[str, Any]) -> str:
    """Write the case's events into the backend it declared.

    Answers the database path when that backend is SQLite, "" otherwise.
    """
    backend = store.get("backend", _STORE_JSONL)
    if backend == _STORE_JSONL:
        _seed_jsonl_store(audit_dir, store)
        return ""

    assert backend == _STORE_SQLITE, (
        f"audit_store backend {backend!r}, want jsonl or sqlite"
    )
    return _seed_sqlite_store(tmp_path, store)


def _seed_sqlite_store(tmp_path: Path, store: dict[str, Any]) -> str:
    """Write the case's events through the sink's own writer.

    The seeded schema is the shipped schema rather than a hand-kept copy of it.
    The database sits outside the audit directory because the JSONL footprint
    counts every file beside the log: a database in there would make disk_bytes
    driver decided too, and the point of the backend is to keep exactly one
    field that way.
    """
    store_dir = tmp_path / "store"
    store_dir.mkdir(parents=True, exist_ok=True)
    path = store_dir / "audit.db"

    sink = SQLiteSink(str(path), DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS)
    try:
        for event in store.get("events", []):
            sink.write(
                audit_event_from_record(
                    json.dumps(
                        event, sort_keys=True, separators=(",", ":"), ensure_ascii=False
                    )
                )
            )
    finally:
        sink.close()

    return str(path)


def _seed_jsonl_store(audit_dir: Path, store: dict[str, Any]) -> None:
    """Write the case's events into the active JSONL log the readers walk.

    Both runners re-encode each event the same way (sorted keys, compact
    separators, no escaping), so the file's byte size is part of the shared
    contract rather than an accident of whichever language wrote it.
    """
    audit_dir.mkdir(parents=True, exist_ok=True)
    lines = [
        json.dumps(event, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
        for event in store.get("events", [])
    ]
    (audit_dir / ACTIVE_LOG_FILE_NAME).write_text(
        "".join(f"{line}\n" for line in lines), encoding="utf-8"
    )


def _substitute_runner_paths(want: Any, paths: _RunnerPaths) -> Any:
    """Resolve {{audit_dir}} and {{sqlite_path}} in an expected result.

    Both live in a per-run temp directory the runner picked, so an answer that
    reports its own store cannot be pinned any other way.
    """
    text = json.dumps(want)
    if paths.audit_dir:
        text = text.replace("{{audit_dir}}", paths.audit_dir)
    if paths.sqlite_path:
        text = text.replace("{{sqlite_path}}", paths.sqlite_path)

    return json.loads(text)


def _assert_host_decided_shape(tool: str, case_name: str, case: dict[str, Any]) -> None:
    """Refuse a declaration that would weaken the pin.

    A named field with no constraint is an ignored field, and a case with no
    expect_result has no answer to name a field in.
    """
    declared: list[dict[str, Any]] = case.get("host_decided", [])
    if not declared:
        return

    assert "expect_result" in case, (
        f"{tool}/{case_name}: host_decided needs expect_result: "
        "there is no answer to name a field in"
    )

    for field in declared:
        assert _host_decided_entry_valid(field), (
            f"{tool}/{case_name}: host_decided {field.get('path', '')!r} needs a "
            "reason, and must be a string with a pattern or an integer with a min"
        )


def _host_decided_entry_valid(field: dict[str, Any]) -> bool:
    """Report whether one declaration still asserts something.

    A string carrying a pattern, or an integer carrying a floor. A type with
    neither would name a field and check nothing about it. The Go runner holds
    the same rule in hostDecidedEntryValid().
    """
    if not field.get("path") or not field.get("reason"):
        return False

    string_shape = (
        field.get("type") == _HOST_DECIDED_STRING
        and bool(field.get("pattern"))
        and "min" not in field
    )
    integer_shape = (
        field.get("type") == _HOST_DECIDED_INTEGER
        and "min" in field
        and not field.get("pattern")
    )

    return string_shape or integer_shape


def _take_host_decided(answer: Any, path: str) -> tuple[Any, bool]:
    """Remove the field at a dotted path and return it.

    Everything left keeps its literal comparison.
    """
    segments = path.split(".")
    node: Any = answer
    for segment in segments[:-1]:
        if not isinstance(node, dict) or segment not in node:
            return None, False
        node = cast("dict[str, Any]", node)[segment]

    leaf = segments[-1]
    if not isinstance(node, dict) or leaf not in node:
        return None, False

    return cast("dict[str, Any]", node).pop(leaf), True


def _assert_host_decided_value(
    tool: str, case_name: str, field: dict[str, Any], value: Any
) -> None:
    """Assert the declared type, and the pattern or the floor beside it."""
    path = field["path"]
    if field["type"] == _HOST_DECIDED_STRING:
        assert isinstance(value, str), (
            f"{tool}/{case_name}: host-decided {path!r} = {value!r}, want a string"
        )
        assert re.search(field["pattern"], value), (
            f"{tool}/{case_name}: host-decided {path!r} = {value!r}, "
            f"want a match for {field['pattern']!r}"
        )
        return

    assert not isinstance(value, bool), (
        f"{tool}/{case_name}: host-decided {path!r} = {value!r}, want an integer"
    )
    assert isinstance(value, int), (
        f"{tool}/{case_name}: host-decided {path!r} = {value!r}, want an integer"
    )
    assert value >= field["min"], (
        f"{tool}/{case_name}: host-decided {path!r} = {value}, "
        f"want at least {field['min']}"
    )


def _take_all_host_decided(
    tool: str, case_name: str, case: dict[str, Any], got: Any, want: Any
) -> None:
    """Assert each named field and lift it out of the answer.

    A name the answer does not carry fails here rather than passing quietly,
    which is what stops a renamed field from turning a pin into a no-op, and a
    name expect_result also spells fails too: one owner per field.
    """
    for field in case.get("host_decided", []):
        path = field["path"]
        _, spelled = _take_host_decided(want, path)
        assert not spelled, (
            f"{tool}/{case_name}: host-decided {path!r} is also spelled in "
            "expect_result"
        )
        value, present = _take_host_decided(got, path)
        assert present, (
            f"{tool}/{case_name}: host-decided {path!r} is not in the answer"
        )
        _assert_host_decided_value(tool, case_name, field, value)


def _assert_case_shape(tool: str, case_name: str, case: dict[str, Any]) -> None:
    """Refuse a fixture case that asserts nothing or contradicts itself."""
    unknown = sorted(set(case) - _CASE_FIELDS)
    assert not unknown, (
        f"{tool}/{case_name}: case declares unknown field(s): {', '.join(unknown)}"
    )
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
    _assert_host_decided_shape(tool, case_name, case)


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
    calls: list[tuple[str, dict[str, Any]]],
    fake_request: Any,
    fake_stream: Any,
) -> list[Any]:
    """Dispatch the case's calls in order and answer the last one's result.

    Every call goes to the SAME server, which is what makes a builder success
    path reachable at all: the draft a show or a save acts on exists because an
    earlier call in this list made it. A setup call that refuses fails the case.

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

        result: list[Any] = []
        for index, (tool, arguments) in enumerate(calls):
            result = await srv.dispatch(tool, arguments)
            if index < len(calls) - 1:
                text = result[0].text
                assert not text.startswith("Error:"), (
                    f"setup call {index} ({tool}) failed: {text}"
                )

        return result


def _assert_expected_requests(
    tool: str, case_name: str, case: dict[str, Any], captured: list[Any]
) -> None:
    """Hold the calls a case pins, in order, beside whichever outcome it asserts.

    ``expect_request`` holds a tool to exactly one call, which a two-leg tool
    cannot satisfy: the Object Storage transfers presign first and then move the
    bytes, so the body the presign carries has no other place to be pinned.
    """
    expected = case.get("expect_requests")
    if not expected:
        return

    assert len(captured) == len(expected), (
        f"{tool}/{case_name}: captured {len(captured)} requests, want {len(expected)}"
    )

    for index, want in enumerate(expected):
        method, url, body = captured[index]
        _, path = _split_surface(url)
        assert method == want["method"], (
            f"{tool}/{case_name}: request {index} method {method}"
        )
        assert path == want["path"], f"{tool}/{case_name}: request {index} path {path}"
        if "body" in want:
            assert body == want["body"], (
                f"{tool}/{case_name}: request {index} body {body!r},"
                f" want {want['body']!r}"
            )


def _behavior_clock(case: dict[str, Any]) -> Callable[[], datetime] | None:
    """Answer the case's fixed clock, or None for the wall clock."""
    fixed = case.get("now")
    if not fixed:
        return None

    parsed = datetime.fromisoformat(fixed)
    return lambda: parsed


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


@dataclasses.dataclass(frozen=True)
class _CaseOutcome:
    """What one dispatched case produced, beside the paths it ran under."""

    text: str
    captured: list[tuple[str, str, Any]]
    tmp_path: Path
    paths: _RunnerPaths


def _assert_case_outcome(
    tool: str,
    case_name: str,
    case: dict[str, Any],
    outcome: _CaseOutcome,
) -> None:
    """Assert the one outcome the case contracted."""
    text = outcome.text
    captured = outcome.captured
    tmp_path = outcome.tmp_path
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
        expect_result = _substitute_runner_paths(case["expect_result"], outcome.paths)
        assert not text.startswith("Error:"), f"{tool}/{case_name}: unexpected {text!r}"
        answer = json.loads(text)
        _take_all_host_decided(tool, case_name, case, answer, expect_result)
        assert answer == expect_result, (
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


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("tool", "case_name", "case"),
    _behavior_cases(),
    ids=[f"{tool}/{name}" for tool, name, _ in _behavior_cases()],
)
async def test_behavior_conformance(
    tool: str,
    case_name: str,
    case: dict[str, Any],
    tmp_path: Path,
    monkeypatch: MonkeyPatch,
) -> None:
    """One shared fixture case must produce its contracted outcome."""
    _assert_case_shape(tool, case_name, case)
    paths = _prepare_state(case, tmp_path, monkeypatch)

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

    cfg = _behavior_config(case, paths)
    clock_token = set_clock(_behavior_clock(case))

    calls = [(setup["tool"], setup["args"]) for setup in case.get("setup_calls", [])]
    calls.append((tool, _materialize_files(case, tmp_path)))

    try:
        result = await _dispatch_faked(Server(cfg), calls, _fake_request, _fake_stream)
    finally:
        reset_clock(clock_token)

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

    _assert_expected_requests(tool, case_name, case, captured)
    _assert_case_outcome(
        tool,
        case_name,
        case,
        _CaseOutcome(text=text, captured=captured, tmp_path=tmp_path, paths=paths),
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


@pytest.mark.parametrize(
    ("path", "expected"),
    [
        ("platform", ("darwin/arm64", True)),
        ("sqlite.db_bytes", (4096, True)),
        ("sqlite.size_bytes", (None, False)),
        ("store.db_bytes", (None, False)),
        ("platform.db_bytes", (None, False)),
    ],
    ids=[
        "top-level",
        "nested",
        "renamed-leaf",
        "renamed-parent",
        "leaf-is-not-an-object",
    ],
)
def test_take_host_decided(path: str, expected: tuple[Any, bool]) -> None:
    """A dotted path resolves, and a name the answer lacks reports absent."""
    answer = {"platform": "darwin/arm64", "sqlite": {"db_bytes": 4096}}
    assert _take_host_decided(answer, path) == expected


def test_take_host_decided_leaves_the_rest_alone() -> None:
    """Taking one field must not disturb the literal half of the answer."""
    answer = {"platform": "darwin/arm64", "commit": "unknown"}
    assert _take_host_decided(answer, "platform") == ("darwin/arm64", True)
    assert answer == {"commit": "unknown"}


@pytest.mark.parametrize(
    ("field", "expected"),
    [
        (
            {
                "path": "path",
                "type": "string",
                "pattern": "^/",
                "reason": "temp dir",
            },
            True,
        ),
        (
            {
                "path": "db_bytes",
                "type": "integer",
                "min": 1,
                "reason": "driver",
            },
            True,
        ),
        ({"path": "path", "type": "string", "reason": "temp dir"}, False),
        ({"path": "db_bytes", "type": "integer", "reason": "driver"}, False),
        ({"path": "path", "type": "string", "pattern": "^/"}, False),
        ({"type": "string", "pattern": "^/", "reason": "temp dir"}, False),
        (
            {
                "path": "path",
                "type": "any",
                "pattern": "^/",
                "reason": "temp dir",
            },
            False,
        ),
    ],
    ids=[
        "string-with-a-pattern",
        "integer-with-a-floor",
        "string-with-no-pattern",
        "integer-with-no-floor",
        "no-reason",
        "no-path",
        "unknown-type",
    ],
)
def test_host_decided_entry_valid(field: dict[str, Any], expected: bool) -> None:
    """A declaration that checks nothing is refused before the case runs."""
    assert _host_decided_entry_valid(field) is expected
