"""Offline tests for the diff-coverage gate.

verify_diff_coverage.py intersects the added lines of a diff with each
language's coverage data. These tests pin the unified-diff parsing, the
per-language violation logic (incl. every scope exclusion), the
missing-artifact failure, and the skip posture for an unknown base rev.
"""

from __future__ import annotations

import importlib.util
import json
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


gate = _load_script("verify_diff_coverage")

_DIFF_TEXT = """\
diff --git a/go/internal/foo/bar.go b/go/internal/foo/bar.go
index 1111111..2222222 100644
--- a/go/internal/foo/bar.go
+++ b/go/internal/foo/bar.go
@@ -13,0 +14,2 @@ func x() {
+	if err != nil {
+		return err
@@ -20 +23 @@ func y() {
+	done()
diff --git a/go/internal/foo/old.go b/go/internal/foo/old.go
deleted file mode 100644
--- a/go/internal/foo/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-gone()
-gone()
-gone()
diff --git a/python/src/linodemcp/tools/new.py b/python/src/linodemcp/tools/new.py
new file mode 100644
--- /dev/null
+++ b/python/src/linodemcp/tools/new.py
@@ -0,0 +1,2 @@
+def fresh() -> int:
+    return 1
"""


def test_added_lines_parses_hunks_new_files_and_skips_deletions() -> None:
    added = gate.added_lines_from_diff(_DIFF_TEXT)

    assert added == {
        "go/internal/foo/bar.go": {14, 15, 23},
        "python/src/linodemcp/tools/new.py": {1, 2},
    }


def _write_go_fixtures(tmp_path: Path) -> tuple[Path, Path]:
    profile = tmp_path / "coverage.out"
    profile.write_text(
        "mode: atomic\n"
        "github.com/chadit/LinodeMCP/go/internal/foo/bar.go:10.2,14.16 2 1\n"
        "github.com/chadit/LinodeMCP/go/internal/foo/bar.go:15.2,16.10 2 0\n"
        "github.com/chadit/LinodeMCP/go/cmd/linodemcp/main.go:8.1,20.2 6 0\n",
        encoding="utf-8",
    )
    go_mod = tmp_path / "go.mod"
    go_mod.write_text(
        "module github.com/chadit/LinodeMCP/go\n\ngo 1.26\n", encoding="utf-8"
    )
    return profile, go_mod


def test_go_violations_flag_only_uncovered_in_scope_lines(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Covered lines, test files, cmd/, and unprofiled files never flag."""
    profile, go_mod = _write_go_fixtures(tmp_path)
    monkeypatch.setattr(gate, "_GO_PROFILE", profile)
    monkeypatch.setattr(gate, "_GO_MOD", go_mod)
    added = {
        "go/internal/foo/bar.go": {14, 15},
        "go/internal/foo/bar_test.go": {5},
        "go/cmd/linodemcp/main.go": {9},
        "go/internal/foo/decl_only.go": {3},
    }

    assert gate._go_violations(added) == ["go/internal/foo/bar.go:15"]


def test_python_violations_flag_missing_lines_and_skip_omitted(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    coverage_json = tmp_path / "coverage.json"
    coverage_json.write_text(
        json.dumps(
            {
                "files": {
                    "src/linodemcp/tools/foo.py": {
                        "executed_lines": [1, 2],
                        "missing_lines": [12, 30],
                    }
                }
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setattr(gate, "_PY_COVERAGE", coverage_json)
    added = {
        "python/src/linodemcp/tools/foo.py": {2, 12},
        "python/src/linodemcp/main.py": {4},
        "python/tests/unit/test_foo.py": {9},
    }

    assert gate._python_violations(added) == ["python/src/linodemcp/tools/foo.py:12"]


def test_missing_artifacts_named_only_when_the_diff_needs_them(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(gate, "_GO_PROFILE", tmp_path / "absent.out")
    monkeypatch.setattr(gate, "_PY_COVERAGE", tmp_path / "absent.json")

    go_only = gate._missing_artifacts({"go/internal/foo/bar.go": {1}})
    neither = gate._missing_artifacts({"docs/README.md": {1}})

    assert len(go_only) == 1
    assert "make go-test" in go_only[0]
    assert neither == []


def test_untracked_files_count_as_fully_added(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """git diff never shows untracked files; the gate must still see them."""
    probe = tmp_path / "python" / "src" / "linodemcp" / "new.py"
    probe.parent.mkdir(parents=True)
    probe.write_text("def fresh() -> int:\n    return 1\n", encoding="utf-8")
    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(
        gate, "_untracked_files", lambda: ["python/src/linodemcp/new.py"]
    )

    added = gate.add_untracked_lines({"go/internal/foo/bar.go": {7}})

    assert added == {
        "go/internal/foo/bar.go": {7},
        "python/src/linodemcp/new.py": {1, 2},
    }


def test_unknown_base_rev_skips_instead_of_failing(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """Mirrors baseline-guard: no reference point means stand down loudly."""
    monkeypatch.setattr(
        sys, "argv", ["verify_diff_coverage.py", "definitely-not-a-rev"]
    )

    assert gate.main() == 0
    assert "skipping" in capsys.readouterr().out
