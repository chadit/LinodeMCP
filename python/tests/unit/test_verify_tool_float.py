"""Focused tests for the floating-toolchain gate."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

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


gate = _load_script("verify_tool_float")


def test_dev_group_cap_is_flagged() -> None:
    """The exact shape of the PR #749 cap fails the gate."""
    pyproject = "\n".join(
        (
            "dev = [",
            '    "pytest>=9.1.1",',
            '    "ruff>=0.15.20,<0.16",',
            "]",
        )
    )
    assert gate.dev_group_violations(pyproject) == [
        "python/pyproject.toml dev group: ruff>=0.15.20,<0.16"
    ]


def test_dev_group_exact_pin_is_flagged() -> None:
    pyproject = "\n".join(("dev = [", '    "mypy==2.1.0",', "]"))
    assert gate.dev_group_violations(pyproject) == [
        "python/pyproject.toml dev group: mypy==2.1.0"
    ]


def test_dev_group_floors_pass() -> None:
    pyproject = "\n".join(
        ("dev = [", '    "ruff>=0.16.0",', '    "pytest>=9.1.1",', "]")
    )
    assert gate.dev_group_violations(pyproject) == []


def test_app_dependencies_may_pin() -> None:
    """Caps outside the dev group are app-dep policy, not this gate's."""
    pyproject = "\n".join(
        (
            "dependencies = [",
            '    "structlog>=26.1,<26.2",',
            "]",
            "dev = [",
            '    "ruff>=0.16.0",',
            "]",
        )
    )
    assert gate.dev_group_violations(pyproject) == []


def test_pinned_go_tool_is_flagged() -> None:
    text = "\tgo run github.com/example/lint/cmd/lint@v1.2.3 ./..."
    assert gate.invocation_violations(text, "go/Makefile") == [
        "go/Makefile:1: github.com/example/lint/cmd/lint@v1.2.3"
    ]


def test_latest_go_tool_passes() -> None:
    text = "\tgo run github.com/example/lint/cmd/lint@latest ./..."
    assert gate.invocation_violations(text, "go/Makefile") == []


def test_deliberate_pin_is_allowlisted() -> None:
    text = "\tgo install github.com/bufbuild/buf/cmd/buf@v1.71.0"
    assert gate.invocation_violations(text, "scripts/ci-setup.sh") == []


def test_comment_lines_are_ignored() -> None:
    text = "# go install github.com/example/lint/cmd/lint@v1.2.3"
    assert gate.invocation_violations(text, "go/Makefile") == []


def test_live_repo_is_clean() -> None:
    """The gate holds on the tree as committed; a regression fails here too."""
    assert gate.current_violations() == []
