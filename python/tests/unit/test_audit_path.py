"""Audit directory resolution tests.

Mirrors ``go/internal/audit/path_test.go``. The system-service branch
is UID-gated and out of scope here (the test box is interactive), so
assertions stay permissive about the system path.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit import (
    SYSTEM_AUDIT_DIR,
    USER_AUDIT_DIR_RELATIVE,
    resolve_default_audit_dir,
)

if TYPE_CHECKING:
    import pytest


def test_resolve_honors_xdg_state_home(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An explicit XDG_STATE_HOME wins over the UID heuristic."""
    custom_state = tmp_path / "state"
    monkeypatch.setenv("XDG_STATE_HOME", str(custom_state))

    got = resolve_default_audit_dir()

    assert got == str(custom_state / USER_AUDIT_DIR_RELATIVE)


def test_resolve_falls_back_to_home_dir(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """With XDG unset, the path is system or home-relative, nothing else."""
    monkeypatch.delenv("XDG_STATE_HOME", raising=False)
    monkeypatch.setenv("HOME", str(tmp_path))

    got = resolve_default_audit_dir()

    home_path = str(tmp_path / ".local" / "state" / USER_AUDIT_DIR_RELATIVE)
    assert got in (SYSTEM_AUDIT_DIR, home_path)


def test_user_audit_dir_relative_constant_value() -> None:
    """Pin the directory name; a rename orphans existing logs on upgrade."""
    assert USER_AUDIT_DIR_RELATIVE == "linodemcp"


def test_system_audit_dir_constant_value() -> None:
    """Pin the system path; a change breaks system-service deployments."""
    assert SYSTEM_AUDIT_DIR == "/var/log/linodemcp"


def test_resolve_uses_home_when_not_system_service(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A non-system UID with XDG unset lands on the home-relative state dir.

    Pins the UID heuristic to a normal interactive UID so the resolution
    doesn't depend on whatever UID the test box happens to run under.
    """
    monkeypatch.delenv("XDG_STATE_HOME", raising=False)
    monkeypatch.setattr(os, "geteuid", lambda: 1000)
    monkeypatch.setenv("HOME", str(tmp_path))

    got = resolve_default_audit_dir()

    assert got == str(tmp_path / ".local" / "state" / USER_AUDIT_DIR_RELATIVE)


def test_resolve_treats_missing_geteuid_as_normal_user(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A platform without os.geteuid (Windows) lands on the user path.

    The UID heuristic degrades to "normal user" rather than the system
    dir, so with XDG unset resolution falls through to the home-relative
    state directory.
    """
    monkeypatch.delenv("XDG_STATE_HOME", raising=False)
    monkeypatch.delattr(os, "geteuid", raising=False)
    monkeypatch.setenv("HOME", str(tmp_path))

    got = resolve_default_audit_dir()

    assert got == str(tmp_path / ".local" / "state" / USER_AUDIT_DIR_RELATIVE)


def test_resolve_falls_back_to_cwd_without_home(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An unresolvable home directory falls back to the working directory.

    Pins the UID to a normal interactive value so resolution reaches the
    home fallback instead of short-circuiting to the system dir on a low
    test-box UID, then breaks Path.home so the cwd arm runs.
    """
    monkeypatch.delenv("XDG_STATE_HOME", raising=False)
    monkeypatch.setattr(os, "geteuid", lambda: 1000)

    def _no_home() -> Path:
        msg = "no home directory"
        raise RuntimeError(msg)

    monkeypatch.setattr(Path, "home", _no_home)

    got = resolve_default_audit_dir()

    assert got == str(Path.cwd() / USER_AUDIT_DIR_RELATIVE)
