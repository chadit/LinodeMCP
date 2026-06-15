"""Unit tests for ``linodemcp audit`` (recent/summary/health/export).

The audit subcommands build a ``linode_audit_*`` call and drive it through the
shared dispatch, then print the result. Tests use the public synchronous entry
``run_audit_command`` with a temp config (via the config-path env override) and
a temp audit dir so a real read happens against an isolated, empty log.
"""

from __future__ import annotations

import io
import json
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli.audit import run_audit_command

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture
def audit_config_file(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Write a minimal config and point the loader at it via the env override."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestCLI\n"
        "environments:\n"
        "  default:\n"
        "    label: Default\n"
        "    linode:\n"
        "      apiUrl: https://api.linode.com/v4\n"
        "      token: test-token\n"
    )
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(config_file))
    return config_file


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Point the audit log at a temp dir so reads hit an isolated log."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def test_audit_recent_returns_json(audit_config_file: Path) -> None:
    """`audit recent` dispatches the recent tool and prints a JSON result."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["recent", "--limit", "5"], stdout, stderr)
    assert code == 0
    payload = json.loads(stdout.getvalue())
    assert "events" in payload


def test_audit_health_returns_json(audit_config_file: Path) -> None:
    """`audit health` reports the audit subsystem state as JSON."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["health"], stdout, stderr)
    assert code == 0
    payload = json.loads(stdout.getvalue())
    assert "jsonl_path" in payload


def test_audit_summary_returns_json(audit_config_file: Path) -> None:
    """`audit summary` dispatches the summary tool and prints JSON."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["summary"], stdout, stderr)
    assert code == 0
    # Summary returns a JSON document; just assert it parses.
    json.loads(stdout.getvalue())


def test_audit_export_requires_format(audit_config_file: Path) -> None:
    """`audit export` without --format is a usage error (argparse required)."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["export"], stdout, stderr)
    assert code == 2


def test_audit_export_with_format_succeeds(audit_config_file: Path) -> None:
    """`audit export --format json` dispatches and reports the export result."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["export", "--format", "json"], stdout, stderr)
    assert code == 0


def test_audit_unknown_subcommand_exits_usage_error(audit_config_file: Path) -> None:
    """An unknown audit subcommand prints usage and exits 2."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["bogus"], stdout, stderr)
    assert code == 2
    assert "unknown audit subcommand" in stderr.getvalue()


def test_audit_no_subcommand_prints_usage(audit_config_file: Path) -> None:
    """`audit` with no subcommand prints usage and exits 2."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command([], stdout, stderr)
    assert code == 2
    assert "Usage" in stderr.getvalue()


def test_audit_recent_tool_filter_folds_into_call(audit_config_file: Path) -> None:
    """`audit recent --tool glob --since TS` runs without error (flags fold in).

    The filter values are accepted by the tool's schema; the result is still a
    valid JSON document even when the filtered set is empty.
    """
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(
        [
            "recent",
            "--tool",
            "linode_instance_*",
            "--since",
            "2020-01-01T00:00:00Z",
        ],
        stdout,
        stderr,
    )
    assert code == 0
    json.loads(stdout.getvalue())


def test_audit_malformed_config_exits_usage_error(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A malformed config (not merely absent) fails the audit runtime, exit 2.

    A missing file falls back to the in-memory default; a malformed one is a
    real error that must surface.
    """
    bad = tmp_path / "bad_config.yml"
    bad.write_text("server: [not a mapping\n")
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(bad))
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["health"], stdout, stderr)
    assert code == 2
    assert "failed to start runtime" in stderr.getvalue()


def test_audit_health_offline_no_config(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """With NO config file, `audit health` runs offline via the default, exit 0.

    The audit query tools are meta tools, so the no-config fallback lets an
    operator read the log without a config present, matching the Go CLI.
    """
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(tmp_path / "absent_config.yml"))
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_audit_command(["health"], stdout, stderr)
    assert code == 0
    assert "jsonl_path" in stdout.getvalue()
