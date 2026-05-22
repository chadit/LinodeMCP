"""Audit reports config tests.

Mirrors ``go/internal/config/reports_test.go``. Covers report parsing,
the default output mode, and the filter-grammar validation cases.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from linodemcp.config import (
    REPORT_OUTPUT_SUMMARY,
    ConfigInvalidError,
    load_from_file,
)

if TYPE_CHECKING:
    from pathlib import Path

_MINIMAL = """
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
audit:
  reports:
"""


def _write(tmp_path: Path, reports_block: str) -> Path:
    """Write a minimal config with the supplied reports block appended."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL + reports_block, encoding="utf-8")
    return path


def test_reports_parse(tmp_path: Path) -> None:
    """A custom report round-trips from YAML into the typed config."""
    path = _write(
        tmp_path,
        """    daily-destroys:
      description: "Destructive ops in the last 24h"
      filter:
        capability: "destroy"
        since_offset: "24h"
      group_by: ["tool", "environment"]
      output: "summary"
""",
    )

    cfg = load_from_file(path)
    report = cfg.audit.reports["daily-destroys"]

    assert report.description == "Destructive ops in the last 24h"
    assert report.filter.capability == "destroy"
    assert report.filter.since_offset == "24h"
    assert report.group_by == ["tool", "environment"]
    assert report.output == REPORT_OUTPUT_SUMMARY


def test_reports_default_output(tmp_path: Path) -> None:
    """An omitted output defaults to summary."""
    path = _write(
        tmp_path,
        """    no-output:
      filter:
        capability: "read"
""",
    )

    cfg = load_from_file(path)
    assert cfg.audit.reports["no-output"].output == REPORT_OUTPUT_SUMMARY


@pytest.mark.parametrize(
    ("reports_block", "match"),
    [
        (
            """    bad-out:
      output: "csv"
""",
            "output must be",
        ),
        (
            """    both-cap:
      output: "list"
      filter:
        capability: "destroy"
        capability_in: ["read", "write"]
""",
            "both capability and capability_in",
        ),
        (
            """    bad-dur:
      output: "summary"
      filter:
        since_offset: "yesterday"
""",
            "since_offset is not a valid duration",
        ),
        (
            """    bad-ts:
      output: "summary"
      filter:
        since: "not-a-date"
""",
            "not a valid RFC 3339",
        ),
    ],
)
def test_reports_validation(tmp_path: Path, reports_block: str, match: str) -> None:
    """Malformed report grammar is rejected with the expected reason."""
    path = _write(tmp_path, reports_block)

    with pytest.raises(ConfigInvalidError, match=match):
        load_from_file(path)
