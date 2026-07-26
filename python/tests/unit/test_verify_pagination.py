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

import pytest

if TYPE_CHECKING:
    from types import ModuleType

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


def _write_registry(path: Path, *names: str) -> Path:
    """A languages.txt whose entries point at sibling working dirs."""
    registry = path / "languages.txt"
    registry.write_text(
        "# comment\n" + "".join(f"{n}\t{n}\tdump\n" for n in names), encoding="utf-8"
    )
    return registry


def test_snapshot_bounds_collapses_one_shared_pair() -> None:
    """Every route sharing 25-500 reads as a single pair, which is the premise."""
    bounds = gate.snapshot_bounds(
        REPO_ROOT / "docs" / "contracts" / "api-pagination-baseline.txt"
    )

    assert bounds == {(25, 500)}


def test_declared_bounds_finds_constants_and_skips_generated_trees(
    tmp_path: Path,
) -> None:
    """Generated code and dot-directories are not places a bound is declared."""
    workdir = tmp_path / "go"
    (workdir / "internal" / "tools").mkdir(parents=True)
    (workdir / "internal" / "tools" / "helpers.go").write_text(
        "const (\n\tstandardPageSizeMin = 25\n\tstandardPageSizeMax = 500\n)\n",
        encoding="utf-8",
    )
    (workdir / "genpb").mkdir()
    (workdir / "genpb" / "gen.go").write_text(
        "const generatedPageSizeMin = 1\n", encoding="utf-8"
    )
    (workdir / ".cache").mkdir()
    (workdir / ".cache" / "stale.go").write_text(
        "const cachedPageSizeMax = 9\n", encoding="utf-8"
    )

    found = gate.declared_bounds("go", workdir)

    assert sorted(name for _, name, _ in found) == [
        "standardPageSizeMax",
        "standardPageSizeMin",
    ]


def test_bound_violations_passes_when_code_matches_the_snapshot() -> None:
    """The live repo is the case that must stay green."""
    assert gate.bound_violations() == []


def test_bound_violations_reports_a_constant_that_drifted(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A hand-edited bound is exactly the drift this check exists to catch."""
    snapshot = tmp_path / "snapshot.txt"
    snapshot.write_text("GET /widgets page_size=25-500 default=100\n", encoding="utf-8")
    workdir = tmp_path / "go"
    workdir.mkdir()
    (workdir / "tools.go").write_text(
        "const widgetPageSizeMax = 501\n", encoding="utf-8"
    )

    monkeypatch.setattr(gate, "_SNAPSHOT", snapshot)
    monkeypatch.setattr(gate, "_LANGUAGES", _write_registry(tmp_path, "go"))
    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)

    problems = gate.bound_violations()

    assert len(problems) == 1
    assert "widgetPageSizeMax = 501" in problems[0]
    assert "snapshot says 500" in problems[0]


def test_bound_violations_refuses_a_single_pair_when_routes_disagree(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Two distinct spec pairs mean no single standard pair can be correct."""
    snapshot = tmp_path / "snapshot.txt"
    snapshot.write_text(
        "GET /widgets page_size=25-500 default=100\n"
        "GET /gadgets page_size=1-100 default=25\n",
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_SNAPSHOT", snapshot)

    problems = gate.bound_violations()

    assert len(problems) == 1
    assert "2 distinct page_size bound pairs" in problems[0]


def test_bound_violations_names_a_language_with_no_pattern(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Registering a language must force a decision, not silently skip it."""
    snapshot = tmp_path / "snapshot.txt"
    snapshot.write_text("GET /widgets page_size=25-500 default=100\n", encoding="utf-8")
    (tmp_path / "rust").mkdir()

    monkeypatch.setattr(gate, "_SNAPSHOT", snapshot)
    monkeypatch.setattr(gate, "_LANGUAGES", _write_registry(tmp_path, "rust"))
    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)

    problems = gate.bound_violations()

    assert len(problems) == 1
    assert problems[0].startswith("rust: no page_size bound pattern declared")


def test_registered_languages_rejects_an_unparsable_line(tmp_path: Path) -> None:
    """A malformed registry must fail loudly rather than scan nothing."""
    registry = tmp_path / "languages.txt"
    registry.write_text("go\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="unparsable"):
        gate.registered_languages(registry)


def test_main_fails_when_a_bound_drifts(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """main must turn a drifted bound into a non-zero exit, not just report it."""
    snapshot = tmp_path / "snapshot.txt"
    snapshot.write_text("GET /widgets page_size=25-500 default=100\n", encoding="utf-8")
    workdir = tmp_path / "go"
    workdir.mkdir()
    (workdir / "tools.go").write_text(
        "const widgetPageSizeMin = 10\n", encoding="utf-8"
    )

    monkeypatch.setattr(gate, "_SNAPSHOT", snapshot)
    monkeypatch.setattr(gate, "_LANGUAGES", _write_registry(tmp_path, "go"))

    assert gate.main([]) == 1


def test_main_passes_on_the_live_repo() -> None:
    """The committed state is the case that must stay green."""
    assert gate.main([]) == 0
