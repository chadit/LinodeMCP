"""Focused tests for the behavior-fixture response-shape gate."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_gate() -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(
        "verify_response_shapes", SCRIPTS_DIR / "verify_response_shapes.py"
    )
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_gate()


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
