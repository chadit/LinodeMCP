"""Phase 7c clone + delete subcommand tests.

Mirrors ``go/internal/cli/clone_delete_test.go``. Each test owns a
tempdir config file so the round-trip exercises ``write_atomic``
end-to-end.
"""

from __future__ import annotations

import io
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli import (
    EXIT_USAGE_ERROR,
    run_profile_clone,
    run_profile_delete,
    run_profile_use,
)
from linodemcp.config import load_from_file

if TYPE_CHECKING:
    from pathlib import Path


_MINIMAL_YAML = """\
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


@pytest.fixture
def config_path(tmp_path: Path) -> Path:
    """Stage a minimal config file and return its path."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL_YAML)
    return path


def test_run_profile_clone_copies_builtin_into_user_defined(
    config_path: Path,
) -> None:
    """Happy path: cloning a built-in into a new user-defined name persists."""
    stdout = io.StringIO()
    stderr = io.StringIO()

    rc = run_profile_clone(["compute-admin", "my-compute"], config_path, stdout, stderr)

    assert rc == 0, f"clone must succeed: {stderr.getvalue()}"
    assert "profile my-compute cloned from compute-admin" in stdout.getvalue()

    reloaded = load_from_file(config_path)
    assert "my-compute" in reloaded.profiles, (
        "cloned profile must appear in user profiles after reload"
    )
    assert reloaded.profiles["my-compute"].allowed_tools, (
        "cloned profile must carry the source's allowed_tools"
    )


def test_run_profile_clone_refuses_builtin_destination_name(
    config_path: Path,
) -> None:
    """Cloning to a built-in name is refused to prevent shadowing."""
    stderr = io.StringIO()

    rc = run_profile_clone(
        ["compute-admin", "network-admin"], config_path, io.StringIO(), stderr
    )

    assert rc == 1
    assert "built-in profile name" in stderr.getvalue()


def test_run_profile_clone_refuses_empty_destination(config_path: Path) -> None:
    """Empty dst would yield a blank YAML key; refuse at command time."""
    stderr = io.StringIO()

    rc = run_profile_clone(["default", ""], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "cannot be empty" in stderr.getvalue()


def test_run_profile_clone_refuses_unknown_source(config_path: Path) -> None:
    """Cloning from a nonexistent name produces a friendly error."""
    stderr = io.StringIO()

    rc = run_profile_clone(
        ["nonexistent-source", "my-clone"], config_path, io.StringIO(), stderr
    )

    assert rc == 1
    assert "source profile" in stderr.getvalue()

    reloaded = load_from_file(config_path)
    assert "my-clone" not in reloaded.profiles, (
        "failed clone must not leave a stub on disk"
    )


def test_run_profile_clone_refuses_existing_user_defined(
    config_path: Path,
) -> None:
    """Second clone to the same dst must be refused (no silent overwrite)."""
    assert (
        run_profile_clone(
            ["default", "my-prof"], config_path, io.StringIO(), io.StringIO()
        )
        == 0
    )

    stderr = io.StringIO()
    rc = run_profile_clone(
        ["compute-admin", "my-prof"], config_path, io.StringIO(), stderr
    )

    assert rc == 1
    assert "already exists" in stderr.getvalue()


def test_run_profile_clone_zero_args_returns_usage() -> None:
    """clone requires exactly two positional arguments."""
    stderr = io.StringIO()

    rc = run_profile_clone([], None, io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()


def test_run_profile_delete_removes_user_defined(config_path: Path) -> None:
    """Happy path: a user-defined profile gets removed."""
    assert (
        run_profile_clone(
            ["default", "to-delete"], config_path, io.StringIO(), io.StringIO()
        )
        == 0
    )

    stdout = io.StringIO()
    stderr = io.StringIO()

    rc = run_profile_delete(["to-delete"], config_path, stdout, stderr)

    assert rc == 0, f"delete must succeed: {stderr.getvalue()}"
    assert "profile to-delete deleted" in stdout.getvalue()

    reloaded = load_from_file(config_path)
    assert "to-delete" not in reloaded.profiles


def test_run_profile_delete_refuses_builtin(config_path: Path) -> None:
    """Built-ins live in code; delete on a built-in is refused."""
    stderr = io.StringIO()

    rc = run_profile_delete(["compute-admin"], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "built-in" in stderr.getvalue()
    assert "disable" in stderr.getvalue()


def test_run_profile_delete_refuses_unknown(config_path: Path) -> None:
    """Deleting a name that's neither built-in nor user-defined exits 1."""
    stderr = io.StringIO()

    rc = run_profile_delete(["nonexistent-profile"], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "not found" in stderr.getvalue()


def test_run_profile_delete_refuses_active_profile(config_path: Path) -> None:
    """Deleting the active profile is refused (server would not start)."""
    assert (
        run_profile_clone(
            ["default", "active-clone"], config_path, io.StringIO(), io.StringIO()
        )
        == 0
    )
    assert (
        run_profile_use(["active-clone"], config_path, io.StringIO(), io.StringIO())
        == 0
    )

    stderr = io.StringIO()
    rc = run_profile_delete(["active-clone"], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "active profile" in stderr.getvalue()

    reloaded = load_from_file(config_path)
    assert "active-clone" in reloaded.profiles, (
        "refused delete must not remove the entry from disk"
    )


def test_run_profile_delete_zero_args_returns_usage() -> None:
    """delete requires exactly one positional argument."""
    stderr = io.StringIO()

    rc = run_profile_delete([], None, io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()
