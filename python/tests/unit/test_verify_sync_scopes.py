"""Offline tests for the scope sync gate.

verify_sync_scopes.py compares the Python scope mapping against the
OpenAPI spec's per-operation security blocks, routed through the
docs/contracts/tool-routes.txt contract. These tests pin the path
normalization, the contract parser, every drift class, and the baseline
ratchet; the full gate runs live via `make sync-scopes`.
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


gate = _load_script("verify_sync_scopes")


def _spec(paths: dict[str, dict[str, Any]]) -> dict[str, Any]:
    return {"info": {"version": "4.999.0"}, "paths": paths}


def _op(scopes: list[str] | None) -> dict[str, Any]:
    """Build one operation: None means public (no security field)."""
    if scopes is None:
        return {"responses": {}}
    return {
        "security": [{"personalAccessToken": []}, {"oauth": scopes}],
        "responses": {},
    }


def _record(name: str, capability: str, scopes: list[str]) -> dict[str, Any]:
    return {"name": name, "capability": capability, "scopes": scopes}


def test_norm_template_collapses_params_and_query() -> None:
    """Placeholders and query strings never affect route matching."""
    assert gate.norm_template("/linode/instances/{linodeId}") == (
        "/linode/instances/{p}"
    )
    assert gate.norm_template("/profile/tokens?page=2") == "/profile/tokens"
    assert gate.norm_template("/a/{x}/b/{y}") == "/a/{p}/b/{p}"


def test_parse_routes_happy_path() -> None:
    """Comments and blanks are skipped; entries parse into tuples."""
    text = (
        "# header\n\nlinode_tag_list: GET /tags\n"
        "linode_tag_delete: DELETE /tags/{label}\n"
    )
    assert gate.parse_routes(text) == {
        "linode_tag_list": ("GET", "/tags"),
        "linode_tag_delete": ("DELETE", "/tags/{p}"),
    }


@pytest.mark.parametrize(
    "line",
    [
        "linode_tag_list GET /tags",
        "linode_tag_list: FETCH /tags",
        "linode_tag_list: GET tags",
        "linode_tag_list: GET",
    ],
)
def test_parse_routes_rejects_malformed_lines(line: str) -> None:
    """A broken contract aborts instead of becoming drift."""
    with pytest.raises(SystemExit):
        gate.parse_routes(line + "\n")


def test_parse_routes_rejects_duplicates() -> None:
    """Two lines for one tool is a contract bug, not a choice."""
    text = "linode_tag_list: GET /tags\nlinode_tag_list: GET /tags\n"
    with pytest.raises(SystemExit):
        gate.parse_routes(text)


def test_spec_operations_scoped_public_and_token_only() -> None:
    """Public routes and empty oauth lists both document no scope."""
    spec = _spec(
        {
            "/{apiVersion}/tags": {"get": _op(["account:read_only"])},
            "/{apiVersion}/linode/kernels": {"get": _op(None)},
            "/{apiVersion}/betas": {"get": _op([])},
        }
    )
    operations = gate.spec_operations(spec)
    assert operations["/tags"]["GET"] == ["account:read_only"]
    assert operations["/linode/kernels"]["GET"] == []
    assert operations["/betas"]["GET"] == []


def _base_fixture() -> tuple[dict[str, Any], list[dict[str, Any]], dict[str, Any]]:
    routes = gate.parse_routes(
        "linode_tag_list: GET /tags\nlinode_kernel_list: GET /linode/kernels\n"
    )
    dump = [
        _record("linode_tag_list", "Read", ["account:read_only"]),
        _record("linode_kernel_list", "Read", []),
        _record("hello", "Meta", []),
    ]
    spec = _spec(
        {
            "/{apiVersion}/tags": {"get": _op(["account:read_only"])},
            "/{apiVersion}/linode/kernels": {"get": _op(None)},
        }
    )
    return routes, dump, spec


def test_compare_clean_surface_reports_nothing() -> None:
    """Matching scopes, public routes, and meta tools produce no drift."""
    routes, dump, spec = _base_fixture()
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == []


def test_compare_flags_scope_mismatch() -> None:
    """A mapping that disagrees with the documented scopes is one line."""
    routes, dump, spec = _base_fixture()
    dump[0]["scopes"] = ["account:read_write"]
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == [
        "linode_tag_list: scopes doc=['account:read_only']"
        " mapped=['account:read_write']"
    ]


def test_compare_flags_missing_route_entry() -> None:
    """A registered tool absent from the contract is flagged."""
    routes, dump, spec = _base_fixture()
    dump.append(_record("linode_volume_list", "Read", ["volumes:read_only"]))
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == [
        "linode_volume_list: no route entry"
    ]


def test_compare_flags_stale_route_entry() -> None:
    """A contract line for an unregistered tool is flagged."""
    routes, dump, spec = _base_fixture()
    routes["linode_gone_tool"] = ("GET", "/tags")
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == [
        "linode_gone_tool: route entry but tool not registered"
    ]


def test_compare_flags_route_missing_from_spec() -> None:
    """A route upstream never documented is its own drift class."""
    routes, dump, spec = _base_fixture()
    routes["linode_tag_list"] = ("GET", "/tags/nowhere")
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == [
        "linode_tag_list: route GET /tags/nowhere not in spec"
    ]


def _write_env(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    baseline: str | None,
) -> tuple[Path, Path]:
    """Point the module at tmp contract/baseline/spec/dump files."""
    routes_file = tmp_path / "tool-routes.txt"
    routes_file.write_text(
        "linode_tag_list: GET /tags\n",
        encoding="utf-8",
    )
    baseline_file = tmp_path / "scope-sync-baseline.txt"
    if baseline is not None:
        baseline_file.write_text(baseline, encoding="utf-8")
    monkeypatch.setattr(gate, "_ROUTES", routes_file)
    monkeypatch.setattr(gate, "_BASELINE", baseline_file)

    spec_file = tmp_path / "spec.json"
    spec_file.write_text(
        json.dumps(_spec({"/{apiVersion}/tags": {"get": _op(["account:read_only"])}})),
        encoding="utf-8",
    )
    dump_file = tmp_path / "dump.json"
    dump_file.write_text(
        json.dumps([_record("linode_tag_list", "Read", ["account:read_write"])]),
        encoding="utf-8",
    )
    return spec_file, dump_file


def test_main_reports_new_drift(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Un-baselined drift exits non-zero and prints DRIFT lines."""
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline=None)
    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])
    out = capsys.readouterr().out
    assert rc == 1
    assert "DRIFT linode_tag_list: scopes" in out


def test_main_baseline_suppresses_known_drift(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """An annotated baseline entry keeps the gate green."""
    baseline = (
        "linode_tag_list: scopes doc=['account:read_only']"
        " mapped=['account:read_write']"
        "  # accepted 2026-07-21 https://example.test/issues/1\n"
    )
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline=baseline)
    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])
    out = capsys.readouterr().out
    assert rc == 0
    assert "sync-scopes OK" in out


def test_main_unannotated_baseline_entry_fails(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """A baseline line with no acceptance annotation is rejected."""
    baseline = (
        "linode_tag_list: scopes doc=['account:read_only']"
        " mapped=['account:read_write']\n"
    )
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline=baseline)
    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])
    out = capsys.readouterr().out
    assert rc == 1
    assert "missing a valid annotation" in out


def test_main_stale_baseline_entry_fails(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """A fixed deviation must leave the baseline (shrink-only ratchet)."""
    baseline = (
        "linode_gone: scopes doc=['a:read_only'] mapped=[]"
        "  # accepted 2026-07-21 https://example.test/issues/1\n"
        "linode_tag_list: scopes doc=['account:read_only']"
        " mapped=['account:read_write']"
        "  # accepted 2026-07-21 https://example.test/issues/1\n"
    )
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline=baseline)
    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])
    out = capsys.readouterr().out
    assert rc == 1
    assert "FIXED since baseline" in out
    assert "linode_gone" in out


def test_main_update_baseline_preserves_annotations(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Regeneration keeps the audit trail of surviving entries."""
    annotation = "accepted 2026-07-21 https://example.test/issues/1"
    baseline = (
        "linode_tag_list: scopes doc=['account:read_only']"
        f" mapped=['account:read_write']  # {annotation}\n"
    )
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline=baseline)
    rc = gate.main(
        ["--spec", str(spec_file), "--dump", str(dump_file), "--update-baseline"]
    )
    assert rc == 0
    rewritten = (tmp_path / "scope-sync-baseline.txt").read_text(encoding="utf-8")
    assert annotation in rewritten
    assert "baseline updated: 1 accepted deviation(s)" in capsys.readouterr().out


def test_live_contract_files_are_coherent() -> None:
    """The checked-in contract parses and covers the checked-in baseline.

    Full spec comparison needs the network, but the contract file's format
    and the baseline's annotations are verifiable offline, so a malformed
    line never waits for the Monday cron to surface.
    """
    routes = gate.parse_routes(
        (REPO_ROOT / "docs" / "contracts" / "tool-routes.txt").read_text(
            encoding="utf-8"
        )
    )
    assert len(routes) > 400

    baselines = _load_script("_baselines")
    stored = baselines.read_baseline(
        REPO_ROOT / "docs" / "contracts" / "scope-sync-baseline.txt"
    )
    assert not baselines.unannotated(set(stored), stored)

    # Every accepted deviation names a tool that has a route line, so a
    # renamed tool cannot leave an orphaned baseline entry behind.
    baseline_tools = {entry.split(":", 1)[0] for entry in stored}
    assert baseline_tools <= set(routes)
