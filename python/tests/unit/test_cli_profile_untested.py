"""Behavioral tests for the untested branches of ``cli/profile.py``.

The suite in ``test_cli_profile.py`` exercises the pure catalog helpers and
``test_cli_mutators.py`` / ``test_cli_clone_delete.py`` cover the mutator happy
paths. What's left uncovered is the command dispatcher, the ``list``/``show``
read-only bodies (which read via ``get_config_path`` rather than an injected
path), the config load/write failure branches, and the non-empty
allowed-environments formatting. These tests drive those real code paths and
assert on the produced stdout/stderr and exit codes.
"""

from __future__ import annotations

import io
from typing import TYPE_CHECKING

from linodemcp.cli import (
    EXIT_USAGE_ERROR,
    print_profile_detail,
    run_profile_clone,
    run_profile_command,
    run_profile_delete,
    run_profile_enable,
    run_profile_list,
    run_profile_show,
    run_profile_use,
)
from linodemcp.cli import profile as profile_module
from linodemcp.config import load_from_file
from linodemcp.profiles import Profile

if TYPE_CHECKING:
    from pathlib import Path

    import pytest

    from linodemcp.config import Config


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

# An unterminated flow sequence: exists on disk but fails to parse, so
# load_from_file raises and the load-failure branches fire.
_MALFORMED_YAML = "server: [this is not a mapping\n"


def _write_config(tmp_path: Path, body: str) -> Path:
    """Stage a config file with ``body`` and return its path."""
    path = tmp_path / "config.yml"
    path.write_text(body)
    return path


def _raise_write_error(path: Path, cfg: Config) -> None:
    """Stand in for ``write_atomic`` and fail like a full/unwritable disk."""
    msg = f"simulated write failure for {path}"
    raise OSError(msg)


def test_command_dispatches_read_only_list(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """``profile list`` routed through the dispatcher renders the full table.

    Covers the read-only dispatch branch plus the whole ``run_profile_list``
    body, including the DISABLED/YES formatting on the emergency built-in and
    the active marker on the default profile.
    """
    cfg_path = _write_config(tmp_path, _MINIMAL_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_command(["list"], stdout, stderr)

    assert rc == 0, f"list must succeed: {stderr.getvalue()}"
    out = stdout.getvalue()
    lines = out.splitlines()

    header, data_rows = lines[0], lines[1:]
    assert "name" in header
    assert "yolo" in header
    assert "state" in header
    assert "tools" in header

    emergency_line = next(line for line in data_rows if "emergency" in line)
    assert "DISABLED" in emergency_line, "emergency ships disabled by default"
    assert "YES" in emergency_line, "emergency allows yolo"

    active_line = next(line for line in data_rows if line.startswith("*"))
    assert "default" in active_line, "default is the active profile marker row"


def test_command_dispatches_mutator_use(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """``profile use`` routed through the dispatcher persists the switch.

    Covers the mutator dispatch branch, which passes ``config_path=None`` so the
    handler resolves the path via ``get_config_path``.
    """
    cfg_path = _write_config(tmp_path, _MINIMAL_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_command(["use", "readonly-full"], stdout, stderr)

    assert rc == 0, f"use must succeed: {stderr.getvalue()}"
    assert "active profile switched to readonly-full" in stdout.getvalue()
    assert load_from_file(cfg_path).active_profile == "readonly-full"


def test_list_rejects_extra_arguments() -> None:
    """``profile list`` takes no positional arguments."""
    stderr = io.StringIO()
    rc = run_profile_list(["unexpected"], io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "takes no arguments" in stderr.getvalue()


def test_list_load_failure_returns_one(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A malformed config makes ``list`` report the load error and exit 1."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_list([], stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == "", "no table should print when the load fails"


def test_show_known_profile_prints_detail(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """``show default`` prints the profile detail and marks it active."""
    cfg_path = _write_config(tmp_path, _MINIMAL_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_show(["default"], stdout, stderr)

    assert rc == 0, f"show must succeed: {stderr.getvalue()}"
    out = stdout.getvalue()
    assert "Profile: default (active)" in out
    assert "Allowed tools (" in out


def test_show_unknown_profile_lists_available(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """An unknown name exits 1 and prints the sorted list of valid names."""
    cfg_path = _write_config(tmp_path, _MINIMAL_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_show(["no-such-profile"], stdout, stderr)

    assert rc == 1
    err = stderr.getvalue()
    assert 'profile "no-such-profile" not found.' in err
    assert "Available profiles:" in err
    assert "  compute-admin" in err, "the recovery list must name the built-ins"
    assert stdout.getvalue() == ""


def test_show_load_failure_returns_one(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A malformed config makes ``show`` report the load error and exit 1."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)
    monkeypatch.setattr(profile_module, "get_config_path", lambda: cfg_path)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_show(["default"], stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == ""


def test_use_load_failure_returns_one(tmp_path: Path) -> None:
    """``use`` on a malformed config reports the load error without writing."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_use(["readonly-full"], cfg_path, stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == ""


def test_clone_load_failure_returns_one(tmp_path: Path) -> None:
    """``clone`` reaches the load step after the dst guards, then exits 1."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_clone(["default", "fresh-name"], cfg_path, stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == ""


def test_delete_load_failure_returns_one(tmp_path: Path) -> None:
    """``delete`` of a non-built-in name reaches the load step, then exits 1."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_delete(["some-user-profile"], cfg_path, stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == ""


def test_toggle_load_failure_returns_one(tmp_path: Path) -> None:
    """``enable`` on a malformed config reports the load error and exits 1."""
    cfg_path = _write_config(tmp_path, _MALFORMED_YAML)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_enable(["compute-admin"], cfg_path, stdout, stderr)

    assert rc == 1
    assert "load config from" in stderr.getvalue()
    assert stdout.getvalue() == ""


def test_write_failure_reports_and_returns_one(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A failed atomic write is reported to stderr with exit 1, no success line.

    The config loads and validates fine, so the command reaches the write step;
    forcing ``write_atomic`` to raise exercises the error branch that would
    otherwise report a bogus success.
    """
    cfg_path = _write_config(tmp_path, _MINIMAL_YAML)
    monkeypatch.setattr(profile_module, "write_atomic", _raise_write_error)

    stdout, stderr = io.StringIO(), io.StringIO()
    rc = run_profile_use(["readonly-full"], cfg_path, stdout, stderr)

    assert rc == 1
    assert "write config to" in stderr.getvalue()
    assert stdout.getvalue() == "", "no success line when the write fails"


def test_print_detail_lists_named_environments() -> None:
    """A profile scoped to environments prints them joined, not ``<all>``."""
    prof = Profile(
        name="scoped",
        description="restricted to two environments",
        allowed_tools=(),
        allowed_environments=("prod", "staging"),
        required_token_scopes=(),
        allow_yolo=False,
        disabled=False,
    )

    buf = io.StringIO()
    print_profile_detail(buf, prof, "default")

    out = buf.getvalue()
    assert "Allowed environments: prod, staging" in out
    assert "<all>" not in out, "named environments must replace the <all> marker"
