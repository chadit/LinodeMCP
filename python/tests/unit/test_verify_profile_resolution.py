"""Offline tests for the per-profile resolution gate.

verify_profile_resolution.py runs each registered language's resolver over one
catalog and diffs two halves: what every built-in profile allows, and the
category each tool falls in. These tests pin the language scoping, the two
diffs, the category-less mutator check, and the blindness guards that stop the
gate reporting OK while comparing nothing. The full gate runs live via
`make profile-resolution`.
"""

from __future__ import annotations

import importlib.util
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


gate = _load_script("verify_profile_resolution")


def _profile(name: str, **fields: Any) -> dict[str, Any]:
    """One resolved profile record with every compared field present."""
    record: dict[str, Any] = {
        "name": name,
        "allowed_tools": [],
        "required_token_scopes": [],
        "elevated": False,
        "allow_yolo": False,
        "disabled": False,
    }
    record.update(fields)
    return record


def _dump(
    profiles: list[dict[str, Any]], categories: dict[str, list[str]]
) -> dict[str, Any]:
    return {"profiles": profiles, "categories": categories}


def test_registered_languages_reads_names_in_file_order(tmp_path: Path) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text(
        "# comment\n\ngo\tgo\tgo run ./x\npython\tpython\t.venv/bin/python -m y\n",
        encoding="utf-8",
    )

    assert gate.registered_languages(registry) == ["go", "python"]


def test_registered_languages_refuses_an_empty_registry(tmp_path: Path) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text("# only a comment\n", encoding="utf-8")

    with pytest.raises(SystemExit):
        gate.registered_languages(registry)


def test_catalog_fixture_refuses_an_empty_capability_manifest() -> None:
    with pytest.raises(SystemExit):
        gate.catalog_fixture({})


def test_catalog_fixture_sorts_the_surface_by_name() -> None:
    fixture = gate.catalog_fixture({"b_tool": "Read", "a_tool": "Write"})

    assert fixture == [
        {"name": "a_tool", "capability": "Write"},
        {"name": "b_tool", "capability": "Read"},
    ]


def test_profiles_by_name_reads_both_export_shapes() -> None:
    listed = gate.profiles_by_name(_dump([_profile("default")], {}))
    mapped = gate.profiles_by_name(
        {"profiles": {"default": _profile("default")}, "categories": {}}
    )

    assert listed == mapped


def test_blind_spots_flags_a_dump_that_resolved_no_profiles() -> None:
    problems = gate.blind_spots("go", _dump([], {"a": ["compute"]}), ["a"])

    assert any("no profiles" in problem for problem in problems)


def test_blind_spots_flags_profiles_that_allow_nothing() -> None:
    problems = gate.blind_spots(
        "go", _dump([_profile("default")], {"a": ["compute"]}), ["a"]
    )

    assert any("empty tool list" in problem for problem in problems)


def test_blind_spots_flags_a_tool_the_dump_never_categorized() -> None:
    dump = _dump([_profile("default", allowed_tools=["a"])], {"a": ["compute"]})

    problems = gate.blind_spots("go", dump, ["a", "b"])

    assert any("b" in problem for problem in problems)


def test_blind_spots_passes_a_dump_that_resolved_the_whole_catalog() -> None:
    dump = _dump([_profile("default", allowed_tools=["a"])], {"a": ["compute"]})

    assert gate.blind_spots("go", dump, ["a"]) == []


def test_diff_profiles_names_a_profile_only_one_language_ships() -> None:
    problems = gate.diff_profiles(
        "go",
        "python",
        {"default": _profile("default"), "emergency": _profile("emergency")},
        {"default": _profile("default")},
    )

    assert problems == ["emergency: go ships this profile and python does not"]


def test_diff_profiles_names_the_tool_and_the_language_that_allows_it() -> None:
    problems = gate.diff_profiles(
        "go",
        "python",
        {"network-admin": _profile("network-admin", allowed_tools=["ip_create"])},
        {"network-admin": _profile("network-admin", allowed_tools=[])},
    )

    assert problems == [
        "network-admin: go allows ip_create in allowed_tools, python does not"
    ]


def test_diff_profiles_reports_a_scalar_flag_that_differs() -> None:
    problems = gate.diff_profiles(
        "go",
        "python",
        {"emergency": _profile("emergency", allow_yolo=True)},
        {"emergency": _profile("emergency", allow_yolo=False)},
    )

    assert problems == [
        "emergency: allow_yolo is True in go and False in python",
    ]


def test_diff_profiles_ignores_description_wording() -> None:
    left = _profile("default")
    left["description"] = "one wording"
    right = _profile("default")
    right["description"] = "another wording"

    assert (
        gate.diff_profiles("go", "python", {"default": left}, {"default": right}) == []
    )


def test_diff_categories_reports_a_tool_filed_two_ways() -> None:
    problems = gate.diff_categories(
        "go",
        "python",
        {"linode_longview_client_get": ["monitor"]},
        {"linode_longview_client_get": ["longview"]},
    )

    assert problems == [
        (
            "linode_longview_client_get: go files it under ['monitor'],"
            " python under ['longview']"
        )
    ]


def test_diff_categories_ignores_the_order_categories_come_back_in() -> None:
    problems = gate.diff_categories(
        "go",
        "python",
        {"tool": ["compute", "compute_deep"]},
        {"tool": ["compute_deep", "compute"]},
    )

    assert problems == []


def test_homeless_mutators_flags_a_write_in_no_category() -> None:
    dump = _dump([], {"thing_create": [], "thing_list": []})

    problems = gate.homeless_mutators(
        "go", dump, {"thing_create": "Write", "thing_list": "Read"}
    )

    assert problems == ["go: thing_create is a Write tool in no category"]


def test_homeless_mutators_leaves_admin_and_meta_alone() -> None:
    dump = _dump([], {"account_thing": [], "hello": []})

    problems = gate.homeless_mutators(
        "go", dump, {"account_thing": "Admin", "hello": "Meta"}
    )

    assert problems == []


def test_a_registered_language_with_no_resolver_fails_by_name(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text("rust\trust\tcargo run\n", encoding="utf-8")
    monkeypatch.setattr(gate, "_LANGUAGES", registry)

    assert gate.main() == 1
    assert "rust" in capsys.readouterr().err
