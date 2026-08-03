"""Offline tests for the scope sync gate.

verify_sync_scopes.py compares the Python scope mapping against the
OpenAPI spec's per-operation security blocks, routed through the
tool_route options the proto contract carries. These tests pin the path
normalization, the route reader, every drift class, and the baseline
ratchet; the full gate runs live via `make sync-scopes`.
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


def test_contract_routes_normalizes_what_the_proto_declares(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Declared paths pass through the same shaping the spec side gets.

    Both sides of the comparison have to be normalized by one function, or a
    parameter the proto happened to name would never match the spec's own
    name for it.
    """
    monkeypatch.setattr(
        gate._toolroutes,
        "routes",
        lambda: {
            "linode_tag_list": ("GET", "/tags"),
            "linode_tag_delete": ("DELETE", "/tags/{label}"),
        },
    )

    assert gate.contract_routes() == {
        "linode_tag_list": ("GET", "/tags"),
        "linode_tag_delete": ("DELETE", "/tags/{p}"),
    }


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
    routes = {
        "linode_tag_list": ("GET", "/tags"),
        "linode_kernel_list": ("GET", "/linode/kernels"),
    }
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
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == ([], [])


def test_compare_flags_scope_mismatch() -> None:
    """A mapping that disagrees with the documented scopes is one line."""
    routes, dump, spec = _base_fixture()
    dump[0]["scopes"] = ["account:read_write"]
    drift, undocumented = gate.compare(routes, dump, gate.spec_operations(spec))
    assert drift == [
        (
            "linode_tag_list: scopes doc=['account:read_only']"
            " mapped=['account:read_write']"
        )
    ]
    assert undocumented == []


def test_compare_flags_missing_route_entry() -> None:
    """A registered tool absent from the contract is flagged."""
    routes, dump, spec = _base_fixture()
    dump.append(_record("linode_volume_list", "Read", ["volumes:read_only"]))
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == (
        ["linode_volume_list: no route entry"],
        [],
    )


def test_compare_flags_stale_route_entry() -> None:
    """A contract line for an unregistered tool is flagged."""
    routes, dump, spec = _base_fixture()
    routes["linode_gone_tool"] = ("GET", "/tags")
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == (
        ["linode_gone_tool: route entry but tool not registered"],
        [],
    )


def test_compare_applies_upstream_fixup() -> None:
    """A pinned fixup substitutes the effective value while doc matches.

    GET /placement/groups documents placement:read_only, a scope no
    grantable registry defines; the fixup maps it to the family's
    linodes:read_only, so a mapping carrying that value is clean.
    """
    routes = {"linode_placement_group_list": ("GET", "/placement/groups")}
    dump = [_record("linode_placement_group_list", "Read", ["linodes:read_only"])]
    spec = _spec(
        {"/{apiVersion}/placement/groups": {"get": _op(["placement:read_only"])}}
    )
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == ([], [])


def test_compare_reports_stale_upstream_fixup() -> None:
    """A fixup whose pinned doc value no longer matches fails loudly.

    If upstream fixes the placement scope, the fixup must be dropped
    rather than silently rewriting the new documented value.
    """
    routes = {"linode_placement_group_list": ("GET", "/placement/groups")}
    dump = [_record("linode_placement_group_list", "Read", ["linodes:read_only"])]
    spec = _spec(
        {"/{apiVersion}/placement/groups": {"get": _op(["linodes:read_only"])}}
    )
    problems, _ = gate.compare(routes, dump, gate.spec_operations(spec))
    assert len(problems) == 1
    assert problems[0].startswith("linode_placement_group_list: stale fixup")


def test_compare_skips_a_route_the_spec_never_documented() -> None:
    """A route the spec has no operation for is counted, never drift.

    The spec lags techdocs, so its silence about a route says nothing about
    the mapping. Failing on it would make the gate report a defect here for
    a hole upstream, which is what the exempt file used to paper over.
    """
    routes, dump, spec = _base_fixture()
    routes["linode_tag_list"] = ("GET", "/tags/nowhere")
    assert gate.compare(routes, dump, gate.spec_operations(spec)) == (
        [],
        ["linode_tag_list: GET /tags/nowhere"],
    )


def _write_env(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    baseline: str | None,
    exempt: str = "",
) -> tuple[Path, Path]:
    """Point the module at a tiny route set and tmp baseline/exempt/spec/dump.

    The exempt file is redirected even when empty: left pointing at the real
    one, every deviation it lists would read as an exemption that no longer
    applies against this fixture's tiny route set.
    """
    baseline_file = tmp_path / "scope-sync-baseline.txt"
    if baseline is not None:
        baseline_file.write_text(baseline, encoding="utf-8")
    exempt_file = tmp_path / "scope-sync-exempt.txt"
    exempt_file.write_text(exempt, encoding="utf-8")
    monkeypatch.setattr(
        gate, "contract_routes", lambda: {"linode_tag_list": ("GET", "/tags")}
    )
    monkeypatch.setattr(gate, "_BASELINE", baseline_file)
    monkeypatch.setattr(gate, "_EXEMPT", exempt_file)

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
    """The declared routes resolve and cover the checked-in baseline.

    Full spec comparison needs the network, but the proto's declarations and
    the baseline's annotations are verifiable offline, so an orphaned entry
    never waits for the Monday cron to surface.
    """
    routes = gate.contract_routes()
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


def test_main_exempt_deviation_leaves_the_ratchet(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """An exempt deviation needs no baseline line and no tracking issue.

    A ratchet line is a promise to come back. A route upstream has not published
    is not work waiting on anyone here, so holding it in the ratchet forces an
    annotation that cites an issue nothing can close.
    """
    exempt = (
        "linode_tag_list: scopes doc=['account:read_only']"
        " mapped=['account:read_write']\tupstream gates the route\n"
    )
    spec_file, dump_file = _write_env(tmp_path, monkeypatch, baseline="", exempt=exempt)

    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])

    out = capsys.readouterr().out
    assert rc == 0
    assert "1 exemption(s)" in out


def test_main_stale_exemption_fails(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """An exemption whose deviation stopped being generated has to go.

    Otherwise the file keeps claiming an absence that ended, which is the same
    silent-debt failure the ratchet exists to prevent.
    """
    exempt = "linode_gone: route GET /gone not in spec\tupstream never published it\n"
    spec_file, dump_file = _write_env(
        tmp_path,
        monkeypatch,
        baseline=(
            "linode_tag_list: scopes doc=['account:read_only']"
            " mapped=['account:read_write']"
            "  # accepted 2026-07-21 https://example.test/issues/1\n"
        ),
        exempt=exempt,
    )

    rc = gate.main(["--spec", str(spec_file), "--dump", str(dump_file)])

    out = capsys.readouterr().out
    assert rc == 1
    assert "EXEMPTIONS that no longer apply" in out
    assert "linode_gone" in out


def test_live_exemptions_are_real_deviations() -> None:
    """Every exempt line names a tool the route contract still knows.

    A renamed tool would otherwise leave an exemption suppressing nothing.
    """
    exempt_path = REPO_ROOT / "docs" / "contracts" / "scope-sync-exempt.txt"
    routes = gate.contract_routes()

    for raw in exempt_path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue

        deviation = line.split("\t")[0]
        assert deviation.split(":", 1)[0] in routes
