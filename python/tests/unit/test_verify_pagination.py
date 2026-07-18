"""Offline tests for the pagination gate and its sync snapshot extractor.

verify_pagination.py fails when a tool's GET route paginates in the spec
snapshot but the tool's proto input has no page/page_size. These tests pin the
template matcher, the spec extractor's envelope rule (including allOf
composition), and the live repo's no-drift state against the ratchet baseline.
"""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_pagination")
sync = _load_script("verify_sync_pagination")


def test_match_template_prefers_literal_segments() -> None:
    templates = {"/volumes/{volumeId}", "/volumes/types"}

    assert gate.match_template("/volumes/types", templates) == "/volumes/types"
    assert gate.match_template("/volumes/123", templates) == "/volumes/{volumeId}"
    assert gate.match_template("/linode/instances", templates) is None


def test_match_template_requires_equal_segment_count() -> None:
    templates = {"/domains/{domainId}/records"}

    assert gate.match_template("/domains/5/records", templates) == (
        "/domains/{domainId}/records"
    )
    assert gate.match_template("/domains/5", templates) is None


def test_spec_pagination_requires_params_and_envelope() -> None:
    """Paginated params plus the envelope are both required; allOf counts."""
    envelope: dict[str, Any] = {
        "properties": {"data": {}, "page": {}, "pages": {}, "results": {}},
    }
    size_schema = {"type": "integer", "minimum": 25, "maximum": 500, "default": 100}
    page_params = [
        {"name": "page", "in": "query", "schema": {"type": "integer", "minimum": 1}},
        {"name": "page_size", "in": "query", "schema": size_schema},
    ]
    doc: dict[str, Any] = {
        "paths": {
            "/{apiVersion}/widgets": {
                "get": {
                    "parameters": page_params,
                    "responses": {
                        "200": {
                            "content": {
                                "application/json": {
                                    "schema": {"allOf": [envelope, {"properties": {}}]}
                                }
                            }
                        }
                    },
                }
            },
            "/{apiVersion}/widgets/{widgetId}": {
                "get": {
                    "parameters": page_params,
                    "responses": {
                        "200": {
                            "content": {
                                "application/json": {
                                    "schema": {"properties": {"id": {}}}
                                }
                            }
                        }
                    },
                }
            },
            "/{apiVersion}/gadgets": {
                "get": {"parameters": [], "responses": {}},
            },
        }
    }

    assert sync.spec_pagination(doc) == {"GET /widgets page_size=25-500 default=100"}


def test_snapshot_routes_parses_entry_lines(tmp_path: Path) -> None:
    snapshot = tmp_path / "api-pagination-baseline.txt"
    snapshot.write_text(
        "# header\nGET /widgets page_size=25-500 default=100\n\n",
        encoding="utf-8",
    )

    assert gate.snapshot_routes(snapshot) == {"/widgets"}


def test_live_gate_has_no_drift_vs_baseline() -> None:
    """The repo's current gaps must equal the accepted ratchet entries.

    This is the gate itself as a test: a new unpaginated list tool, or a fixed
    tool whose baseline line was not removed, fails here and in make check.
    """
    violations, _unmapped = gate.current_violations()
    baselines = _load_script("_baselines")
    ratchet = REPO_ROOT / "docs" / "contracts" / "pagination-baseline.txt"

    assert set(violations) == baselines.read_entries(ratchet)


def test_tag_object_list_stays_paginated() -> None:
    """The tagged-objects tool exposes pagination and must keep doing so."""
    messages = gate.paginated_messages()

    assert messages["TaggedObjectListInput"] is True


def test_fixture_get_paths_reads_expect_request(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fixtures = tmp_path / "behavior"
    fixtures.mkdir()
    (fixtures / "linode_widget_list.json").write_text(
        json.dumps(
            {
                "tool": "linode_widget_list",
                "cases": [
                    {"expect_request": {"method": "GET", "path": "/widgets?page=2"}},
                    {"expect_error": "nope"},
                ],
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_FIXTURES", fixtures)

    assert gate.fixture_get_paths() == {"linode_widget_list": {"/widgets"}}
