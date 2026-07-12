"""Offline regression tests for sync-enum source staleness verification."""

from __future__ import annotations

import importlib.util
import json
from datetime import date
from pathlib import Path
from typing import TYPE_CHECKING, Any

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT = REPO_ROOT / "scripts" / "verify_sync_enums.py"
FIXTURE = Path(__file__).resolve().parents[1] / "fixtures" / "linode_api_changelog.html"


def _load_script() -> ModuleType:
    spec = importlib.util.spec_from_file_location("verify_sync_enums", SCRIPT)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


sync_enums = _load_script()


class _Response:
    def __init__(self, payload: bytes) -> None:
        self.payload = payload

    def __enter__(self) -> _Response:
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self) -> bytes:
        return self.payload


def _url(request: Any) -> str:
    return str(getattr(request, "full_url", request))


def _responses(
    monkeypatch: pytest.MonkeyPatch,
    changelog: bytes,
    commits: bytes,
    *,
    broken_url: str | None = None,
) -> None:
    payloads = {
        sync_enums.CHANGELOG_URL: changelog,
        sync_enums.SPEC_COMMITS_URL: commits,
    }

    def urlopen(request: Any, **_kwargs: int) -> _Response:
        request_url = _url(request)
        if request_url == broken_url:
            raise OSError("offline")
        return _Response(payloads[request_url])

    monkeypatch.setattr(sync_enums.urllib.request, "urlopen", urlopen)


def test_changelog_fixture_ignores_unrelated_future_metadata() -> None:
    html = FIXTURE.read_text(encoding="utf-8")

    dates = sync_enums.changelog_release_dates(html, today=date(2026, 7, 10))

    assert dates[:3] == [
        date(2026, 7, 1),
        date(2026, 6, 30),
        date(2026, 5, 20),
    ]
    assert date(2026, 10, 16) not in dates


def test_future_release_requires_explicit_scheduled_marker() -> None:
    html = """
    <a href="/linode-api/changelog/august-1-2026">August 1, 2026</a>
    <a data-status="scheduled" href="/linode-api/changelog/august-2-2026">
      Scheduled: August 2, 2026
    </a>
    <a href="/linode-api/changelog/july-1-2026">July 1, 2026</a>
    """

    dates = sync_enums.changelog_release_dates(html, today=date(2026, 7, 10))

    assert dates == [date(2026, 8, 2), date(2026, 7, 1)]


def test_future_release_rejects_scheduled_near_misses() -> None:
    html = """
    <a data-status="unscheduled" href="/linode-api/changelog/august-3-2026">
      Unscheduled: August 3, 2026
    </a>
    <a href="/linode-api/changelog/august-4-2026-scheduled-preview">
      August 4, 2026
    </a>
    <a href="/linode-api/changelog/august-5-2026">
      Not scheduled: August 5, 2026
    </a>
    <a href="/linode-api/changelog/july-1-2026">July 1, 2026</a>
    """

    dates = sync_enums.changelog_release_dates(html, today=date(2026, 7, 10))

    assert dates == [date(2026, 7, 1)]


def test_staleness_note_accepts_newer_openapi_commit(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    changelog = FIXTURE.read_bytes()
    commits = json.dumps(
        [{"commit": {"committer": {"date": "2026-07-02T00:00:00Z"}}}]
    ).encode()
    _responses(monkeypatch, changelog, commits)

    note = sync_enums.staleness_note(
        {"info": {"version": "4.229.1"}}, today=date(2026, 7, 10)
    )

    assert note == (
        "spec version 4.229.1; latest openapi.json commit 2026-07-02; "
        "newest changelog entry 2026-07-01 (release coverage verified)"
    )


@pytest.mark.parametrize(
    "commit_timestamp", ["2026-05-27T15:29:36Z", "2026-07-01T12:00:00Z"]
)
def test_staleness_note_fails_closed_without_strictly_newer_commit(
    monkeypatch: pytest.MonkeyPatch,
    commit_timestamp: str,
) -> None:
    commits = json.dumps(
        [{"commit": {"committer": {"date": commit_timestamp}}}]
    ).encode()
    _responses(monkeypatch, FIXTURE.read_bytes(), commits)

    with pytest.raises(
        sync_enums.StalenessVerificationError,
        match="release coverage cannot be verified",
    ):
        sync_enums.staleness_note(
            {"info": {"version": "4.229.1"}}, today=date(2026, 7, 10)
        )


def test_staleness_ignores_scheduled_future_release(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    changelog = b"""
    <a data-status="scheduled" href="/linode-api/changelog/august-2-2026">
      Scheduled: August 2, 2026
    </a>
    <a href="/linode-api/changelog/july-1-2026">July 1, 2026</a>
    """
    commits = b'[{"commit":{"committer":{"date":"2026-07-02T12:00:00Z"}}}]'
    _responses(monkeypatch, changelog, commits)

    note = sync_enums.staleness_note(
        {"info": {"version": "4.229.1"}}, today=date(2026, 7, 10)
    )

    assert "newest changelog entry 2026-07-01" in note


def test_staleness_note_fails_closed_without_info_version() -> None:
    with pytest.raises(sync_enums.StalenessVerificationError, match=r"info\.version"):
        sync_enums.staleness_note({}, today=date(2026, 7, 10))


@pytest.mark.parametrize("broken_url_name", ["CHANGELOG_URL", "SPEC_COMMITS_URL"])
def test_staleness_note_fails_closed_on_fetch_error(
    monkeypatch: pytest.MonkeyPatch,
    broken_url_name: str,
) -> None:
    commits = b'[{"commit":{"committer":{"date":"2026-05-27T15:29:36Z"}}}]'
    _responses(
        monkeypatch,
        FIXTURE.read_bytes(),
        commits,
        broken_url=getattr(sync_enums, broken_url_name),
    )

    with pytest.raises(sync_enums.StalenessVerificationError, match="fetch failed"):
        sync_enums.staleness_note(
            {"info": {"version": "4.229.1"}}, today=date(2026, 7, 10)
        )


@pytest.mark.parametrize(
    ("changelog", "commits", "message"),
    [
        (b"<html>no release links</html>", b"[]", "changelog parsing failed"),
        (
            b'<a href="/linode-api/changelog/july-1-2026">July 1</a>',
            b"[]",
            "OpenAPI commit verification failed",
        ),
    ],
)
def test_staleness_note_fails_closed_on_invalid_source_data(
    monkeypatch: pytest.MonkeyPatch,
    changelog: bytes,
    commits: bytes,
    message: str,
) -> None:
    _responses(monkeypatch, changelog, commits)

    with pytest.raises(sync_enums.StalenessVerificationError, match=message):
        sync_enums.staleness_note(
            {"info": {"version": "4.229.1"}}, today=date(2026, 7, 10)
        )


def test_main_succeeds_after_staleness_verification(
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    def load_spec(_path: str | None) -> dict[str, Any]:
        return {"info": {"version": "4.229.1"}}

    def go_hand_lists(_path: str | None) -> dict[str, set[str]]:
        return {}

    def hand_list_diffs(_doc: dict[str, Any], _lists: dict[str, set[str]]) -> list[str]:
        return []

    commits = json.dumps(
        [{"commit": {"committer": {"date": "2026-07-02T12:00:00Z"}}}]
    ).encode()
    _responses(monkeypatch, FIXTURE.read_bytes(), commits)
    monkeypatch.setattr(sync_enums, "proto_enums", dict)
    monkeypatch.setattr(sync_enums, "ENUM_SPEC_MAP", {})
    monkeypatch.setattr(sync_enums, "load_spec", load_spec)
    monkeypatch.setattr(sync_enums, "go_hand_lists", go_hand_lists)
    monkeypatch.setattr(sync_enums, "hand_list_diffs", hand_list_diffs)
    monkeypatch.setattr(sync_enums, "read_baseline", set)

    assert sync_enums.main(["verify_sync_enums.py"]) == 0
    assert "newest changelog entry 2026-07-01" in capsys.readouterr().err


def test_main_fails_closed_when_staleness_verification_fails(
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    def load_spec(_path: str | None) -> dict[str, Any]:
        return {"info": {"version": "4.229.1"}}

    def go_hand_lists(_path: str | None) -> dict[str, set[str]]:
        return {}

    def hand_list_diffs(_doc: dict[str, Any], _lists: dict[str, set[str]]) -> list[str]:
        return []

    monkeypatch.setattr(sync_enums, "proto_enums", dict)
    monkeypatch.setattr(sync_enums, "load_spec", load_spec)
    monkeypatch.setattr(sync_enums, "go_hand_lists", go_hand_lists)
    monkeypatch.setattr(sync_enums, "hand_list_diffs", hand_list_diffs)

    def fail(_doc: dict[str, Any]) -> str:
        raise sync_enums.StalenessVerificationError("changelog fetch failed: offline")

    monkeypatch.setattr(sync_enums, "staleness_note", fail)

    assert sync_enums.main(["verify_sync_enums.py"]) == 1
    assert "staleness verification failed" in capsys.readouterr().err


def test_update_baseline_fails_closed_before_write(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    def load_spec(_path: str | None) -> dict[str, Any]:
        return {"info": {"version": "4.229.1"}}

    def go_hand_lists(_path: str | None) -> dict[str, set[str]]:
        return {}

    def hand_list_diffs(_doc: dict[str, Any], _lists: dict[str, set[str]]) -> list[str]:
        return []

    baseline = tmp_path / "enum-baseline.txt"
    baseline.write_text("unchanged\n", encoding="utf-8")
    monkeypatch.setattr(sync_enums, "BASELINE", baseline)
    monkeypatch.setattr(sync_enums, "proto_enums", dict)
    monkeypatch.setattr(sync_enums, "load_spec", load_spec)
    monkeypatch.setattr(sync_enums, "go_hand_lists", go_hand_lists)
    monkeypatch.setattr(sync_enums, "hand_list_diffs", hand_list_diffs)

    commits = json.dumps(
        [{"commit": {"committer": {"date": "2026-05-27T15:29:36Z"}}}]
    ).encode()
    _responses(monkeypatch, FIXTURE.read_bytes(), commits)

    assert sync_enums.main(["verify_sync_enums.py", "--update-baseline"]) == 1
    assert baseline.read_text(encoding="utf-8") == "unchanged\n"
