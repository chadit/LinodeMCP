"""Focused tests for behavior-fixture response-shape selection."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import TYPE_CHECKING, cast

import pytest

if TYPE_CHECKING:
    from collections.abc import Mapping
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


gate = _load_script("verify_response_shapes")


def _violations_for_case(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, case: Mapping[str, object]
) -> list[str]:
    snapshot = tmp_path / "api-response-shapes-baseline.txt"
    snapshot.write_text("GET /widgets array\n", encoding="utf-8")
    routes = tmp_path / "tool-routes.txt"
    routes.write_text("linode_widget_list: GET /widgets\n", encoding="utf-8")
    fixtures = tmp_path / "behavior"
    fixtures.mkdir()
    (fixtures / "linode_widget_list.json").write_text(
        json.dumps({"tool": "linode_widget_list", "cases": [case]}),
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_SNAPSHOT", snapshot)
    monkeypatch.setattr(gate, "_ROUTES", routes)
    monkeypatch.setattr(gate, "_FIXTURES", fixtures)
    return cast("list[str]", gate.current_violations())


def test_expect_api_error_body_is_not_a_shape_violation(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    envelope: dict[str, object] = {
        "data": [{"id": "example"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    case: dict[str, object] = {
        "api_responses": {
            "GET /widgets": envelope,
        },
        "expect_api_error": "response",
    }

    assert _violations_for_case(tmp_path, monkeypatch, case) == []


@pytest.mark.parametrize(
    "expectation",
    [
        {"expect_result": {"ok": True}},
        {"expect_request": {"method": "GET", "path": "/widgets"}},
    ],
)
def test_mismatched_success_body_is_a_shape_violation(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    expectation: dict[str, object],
) -> None:
    case: dict[str, object] = {
        "api_response": {
            "data": [{"id": "example"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        },
        **expectation,
    }

    assert _violations_for_case(tmp_path, monkeypatch, case) == [
        "linode_widget_list: GET /widgets fixture=envelope spec=array"
    ]


def test_case_bodies_skips_non_empty_error_outcomes() -> None:
    routes = {"linode_widget_list": ("GET", "/widgets")}

    assert (
        gate._case_bodies(
            "linode_widget_list",
            {"api_response": {"data": []}, "expect_api_error": "response"},
            routes,
        )
        == []
    )
    assert (
        gate._case_bodies(
            "linode_widget_list",
            {"api_response": {"data": []}, "expect_error": "invalid input"},
            routes,
        )
        == []
    )


def test_case_bodies_checks_success_with_empty_error_fields() -> None:
    routes = {"linode_widget_list": ("GET", "/widgets")}
    body = {"data": [{"id": 1}]}

    assert gate._case_bodies(
        "linode_widget_list",
        {
            "api_response": body,
            "expect_error": "",
            "expect_api_error": "",
            "expect_result": {"widgets": [{"id": 1}]},
        },
        routes,
    ) == [("GET", "/widgets", body)]
