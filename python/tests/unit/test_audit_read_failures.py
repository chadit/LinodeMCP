"""Audit query tools answer their read failures instead of raising.

Go maps every store failure in the audit query tools to a tool result the
caller can read ("failed to read audit log: ..."). Python left the same reads
outside its try, so an unreadable directory or a failing encode came back as an
unhandled exception instead of an answer. These tests hold the Python side to
the Go answer, one read at a time.

The timestamp cases sit here for the same reason: Go parses since/until as
RFC 3339 and nothing else, and Python's fromisoformat used to take spellings
Go refuses, so the same call answered a window in one language and a refusal in
the other.

Mirrors ``go/internal/tools/linode_audit_failure_test.go``.
"""

from __future__ import annotations

import json
import os
import stat
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.audit import export
from linodemcp.config import Config
from linodemcp.gentools.audit import (
    handle_linode_audit_export,
    handle_linode_audit_recent,
    handle_linode_audit_summary,
)

if TYPE_CHECKING:
    from collections.abc import Iterator
    from pathlib import Path

_READ_FAILED = "Error: failed to read audit log: "
_WRITE_FAILED = "Error: failed to write export file: "


@pytest.fixture
def unreadable_audit_dir(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> Iterator[Path]:
    """An audit directory the process cannot list, restored on teardown.

    Root ignores the mode, so the cases that need a real failure skip there
    rather than passing for the wrong reason.
    """
    if hasattr(os, "geteuid") and os.geteuid() == 0:
        pytest.skip("root can list a mode-000 directory, so nothing fails")

    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text("", encoding="utf-8")
    audit_dir.chmod(0)

    yield audit_dir

    audit_dir.chmod(stat.S_IRWXU)


@pytest.fixture
def empty_audit_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """A readable, empty audit directory the tools answer zero events from."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)

    return audit_dir


@pytest.mark.asyncio
async def test_recent_answers_a_read_failure(unreadable_audit_dir: Path) -> None:
    """An unlistable audit directory is a tool result, not an exception."""
    assert unreadable_audit_dir.exists()

    result = await handle_linode_audit_recent({}, Config())

    assert result[0].text.startswith(_READ_FAILED)


@pytest.mark.asyncio
async def test_summary_answers_a_read_failure(unreadable_audit_dir: Path) -> None:
    """The summary window read answers its failure the way Go's does."""
    assert unreadable_audit_dir.exists()

    result = await handle_linode_audit_summary({}, Config())

    assert result[0].text.startswith(_READ_FAILED)


@pytest.mark.asyncio
async def test_export_answers_a_read_failure(unreadable_audit_dir: Path) -> None:
    """The export window read answers its failure the way Go's does."""
    assert unreadable_audit_dir.exists()

    result = await handle_linode_audit_export({"format": "json"}, Config())

    assert result[0].text.startswith(_READ_FAILED)


@pytest.mark.asyncio
async def test_export_answers_an_encode_failure(
    empty_audit_dir: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """An encode that fails answers a tool result, the way Go's writer does."""
    assert empty_audit_dir.exists()

    def _refuse(*_args: Any, **_kwargs: Any) -> None:
        msg = "encoder unavailable"
        raise OSError(msg)

    monkeypatch.setattr(export, "encode_events", _refuse)

    result = await handle_linode_audit_export({"format": "json"}, Config())

    assert result[0].text.startswith(_WRITE_FAILED)


@pytest.mark.parametrize(
    "since",
    ["2026-05-19", "2026-05-19 00:00:00", "2026-05-19T00:00:00"],
)
@pytest.mark.asyncio
async def test_recent_refuses_a_non_rfc3339_since(
    empty_audit_dir: Path, since: str
) -> None:
    """A spelling Go's RFC 3339 parse refuses is refused here too."""
    assert empty_audit_dir.exists()

    result = await handle_linode_audit_recent({"since": since}, Config())

    assert result[0].text.startswith("Error: ")
    assert "since" in result[0].text


@pytest.mark.parametrize(
    "since",
    ["2026-05-19T00:00:00Z", "2026-05-19T00:00:00.500Z", "2026-05-19T00:00:00+02:00"],
)
@pytest.mark.asyncio
async def test_recent_accepts_an_rfc3339_since(
    empty_audit_dir: Path, since: str
) -> None:
    """Every spelling Go's RFC 3339 parse accepts still answers a window."""
    assert empty_audit_dir.exists()

    result = await handle_linode_audit_recent({"since": since}, Config())

    assert json.loads(result[0].text)["count"] == 0


@pytest.mark.asyncio
async def test_export_names_the_bad_bound(empty_audit_dir: Path) -> None:
    """The export refusal names which bound failed, the way its siblings do."""
    assert empty_audit_dir.exists()

    result = await handle_linode_audit_export(
        {"format": "json", "until": "not-a-timestamp"}, Config()
    )

    assert result[0].text.startswith("Error: ")
    assert "until" in result[0].text


@pytest.mark.asyncio
async def test_recent_refuses_an_impossible_date(empty_audit_dir: Path) -> None:
    """A bound that spells RFC 3339 but names no real date still refuses.

    The pattern only proves the shape, so the parse behind it is what catches a
    thirteenth month, the same way Go's time.Parse does.
    """
    assert empty_audit_dir.exists()

    result = await handle_linode_audit_recent(
        {"since": "2026-13-45T00:00:00Z"}, Config()
    )

    assert result[0].text.startswith("Error: ")
    assert "since" in result[0].text
