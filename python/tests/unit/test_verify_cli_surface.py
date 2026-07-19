"""Offline tests for the CLI-surface parity gate.

verify_cli_surface.py diffs the verb and flag surfaces the two CLIs declare
in source. These tests pin the extraction shapes (dispatch switch, frozenset
literal, wrapped argparse calls, bindOptionalBool), each failure direction,
the extraction-went-blind guard, and the live surface.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

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


gate = _load_script("verify_cli_surface")


def test_go_verb_extraction_reads_dispatch_cases(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    main_go = tmp_path / "main.go"
    main_go.write_text(
        "func dispatch(args []string) int {\n\tswitch args[0] {\n"
        '\tcase "serve":\n\t\treturn run()\n'
        '\tcase "call":\n\t\treturn 0\n\t}\n}\n',
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_GO_MAIN", main_go)

    assert gate.go_verbs() == {"serve", "call"}


def test_python_verb_extraction_adds_the_implicit_serve(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    main_py = tmp_path / "main.py"
    main_py.write_text(
        '_CLI_SUBCOMMANDS = frozenset({"call", "tools",\n    "version"})\n',
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_PY_MAIN", main_py)

    assert gate.python_verbs() == {"call", "tools", "version", "serve"}


def test_call_flag_extraction_covers_wrapped_and_optional_bool(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Go's three registration forms and Python's wrapped add_argument."""
    call_go = tmp_path / "call.go"
    call_go.write_text(
        'flags.StringVar(&parsed.jsonArg, "json", "", "x")\n'
        'flags.Var(&kvArgs, "arg", "x")\n'
        'bindOptionalBool(flags, "dry-run", &safety.DryRun)\n',
        encoding="utf-8",
    )
    call_py = tmp_path / "call.py"
    call_py.write_text(
        'parser.add_argument("tool", help="positional stays out")\n'
        'parser.add_argument(\n    "--json", default=None\n)\n'
        'parser.add_argument("--arg", action="append")\n'
        'parser.add_argument("--dry-run", action="store_true")\n',
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_GO_CALL", call_go)
    monkeypatch.setattr(gate, "_PY_CALL", call_py)

    assert gate.go_call_flags() == {"json", "arg", "dry-run"}
    assert gate.python_call_flags() == {"json", "arg", "dry-run"}


def test_violations_name_the_one_sided_language(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(gate, "go_verbs", lambda: {"serve", "call", "extra"})
    monkeypatch.setattr(gate, "python_verbs", lambda: {"serve", "call"})
    monkeypatch.setattr(gate, "go_call_flags", lambda: {"json"})
    monkeypatch.setattr(gate, "python_call_flags", lambda: {"json", "yolo"})
    monkeypatch.setattr(gate, "go_tools_surface", lambda: ({"all"}, True))
    monkeypatch.setattr(gate, "python_tools_surface", lambda: ({"all"}, False))

    problems = gate.cli_violations()

    assert "verbs: extra exists in go only" in problems
    assert "call flags: yolo exists in python only" in problems
    assert "tools show subverb: present in go only" in problems


def test_blind_extraction_is_a_failure_not_a_pass(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An empty extraction means the source shape moved, never a clean pass."""
    monkeypatch.setattr(gate, "go_verbs", _empty_set)
    monkeypatch.setattr(gate, "python_verbs", lambda: {"serve"})
    monkeypatch.setattr(gate, "go_call_flags", lambda: {"json"})
    monkeypatch.setattr(gate, "python_call_flags", lambda: {"json"})
    monkeypatch.setattr(gate, "go_tools_surface", lambda: ({"all"}, True))
    monkeypatch.setattr(gate, "python_tools_surface", lambda: ({"all"}, True))

    problems = gate.cli_violations()

    assert any("extraction found nothing for go" in p for p in problems)


def _empty_set() -> set[str]:
    return set()


def test_live_cli_surfaces_match() -> None:
    """The gate itself as a test: both CLIs declare one surface today."""
    assert gate.cli_violations() == []
