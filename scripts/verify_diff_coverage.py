#!/usr/bin/env python3
"""Diff-aware gate: source lines added since a base rev must be covered.

The aggregate floors (coverage-floor gate, pytest --cov-fail-under) hold
totals up, but an aggregate cannot see WHICH lines a change added: a small
untested addition rides in while the total stays above the floor. Given a
base git rev, this gate intersects the working tree's added lines with each
language's coverage data and fails on any added executable line no test
executed.

It needs the artifacts a full test run writes (go/coverage.out from the go
test target, python/coverage.json from pytest-cov), so it rides in `check`
right after the suites (and so in the pre-push hook and CI). The default
base origin/main locally means "everything not yet pushed"; an unreachable
base rev (tarball checkout, shallow clone) skips loudly instead of failing
unrelated work. CI re-runs it after `make check` with the event's true
base (PR merge parent / push predecessor), because on a push to main
origin/main already equals HEAD and the in-check run sees an empty diff.

Scope is hand-written source in the two measured trees (go/, python/src/).
Skipped, and why:

- generated genpb trees: never hand-edited (and gitignored, so never in a
  diff anyway); go/cmd/: entrypoint mains and one-shot gate dump tools,
  the Go mirror of the python coverage omit for main.py (_coverage.py owns
  both exclusions).
- *_test.go and python/tests/: test code is not itself coverage-measured.
- files coverage.py omits (main.py, server/__init__.py): absent from
  coverage.json by configuration, skipped the same way.
- scripts/: the gate scripts are tested by python/tests/unit but sit
  outside the measured source tree; a known hole, named here rather than
  silent.

Go notes: with this module's Go version, `./...` profiles include packages
without test files at count 0, so a brand-new untested package fails here
line by line instead of hiding. A changed .go file absent from the profile
is declaration-only or excluded by build tags; absence is skipped, not
flagged. Python's `# pragma: no cover` stays the deliberate-exclusion
mechanism (excluded lines never reach missing_lines); Go added lines must
be tested.

Stdlib plus scripts/_coverage.py, so no venv is needed.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

import _coverage

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_PROFILE = _REPO_ROOT / "go" / "coverage.out"
_GO_MOD = _REPO_ROOT / "go" / "go.mod"
_PY_COVERAGE = _REPO_ROOT / "python" / "coverage.json"

_HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@")


def _rev_exists(rev: str) -> bool:
    """Report whether the rev resolves in this clone."""
    result = subprocess.run(
        ["git", "rev-parse", "--verify", f"{rev}^{{commit}}"],
        cwd=_REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    return result.returncode == 0


def _diff_text(base_rev: str) -> str:
    """Unified zero-context diff of the working tree against base_rev."""
    result = subprocess.run(
        [
            "git",
            "diff",
            "--no-color",
            "--find-renames",
            "-U0",
            base_rev,
            "--",
            "go",
            "python/src",
        ],
        cwd=_REPO_ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout


def added_lines_from_diff(diff_text: str) -> dict[str, set[int]]:
    """Added line numbers per repo-relative file from unified diff text."""
    added: dict[str, set[int]] = {}
    current: str | None = None
    lineno = 0
    for line in diff_text.splitlines():
        if line.startswith("+++ "):
            target = line[4:].split("\t")[0]
            current = None if target == "/dev/null" else target.removeprefix("b/")
        elif hunk := _HUNK.match(line):
            lineno = int(hunk.group(1))
        elif current and line.startswith("+"):
            added.setdefault(current, set()).add(lineno)
            lineno += 1
    return added


def _untracked_files() -> list[str]:
    """Repo-relative untracked (not ignored) files under the measured trees."""
    result = subprocess.run(
        [
            "git",
            "ls-files",
            "--others",
            "--exclude-standard",
            "--",
            "go",
            "python/src",
        ],
        cwd=_REPO_ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    return [line for line in result.stdout.splitlines() if line]


def add_untracked_lines(added: dict[str, set[int]]) -> dict[str, set[int]]:
    """Fold untracked files in as fully-added.

    `git diff` never shows untracked files, but a brand-new source file is
    exactly the "new code with no tests" case this gate exists for: in CI
    the file is committed and rides the diff, and the local run must not
    be blinder than CI. Every line counts as added; the scope filters and
    the coverage intersection then treat it like any other file.
    """
    for path in _untracked_files():
        text = (_REPO_ROOT / path).read_text(encoding="utf-8", errors="replace")
        count = text.count("\n") + (0 if text.endswith("\n") or not text else 1)
        if count:
            added.setdefault(path, set()).update(range(1, count + 1))
    return added


def _in_go_scope(path: str) -> bool:
    return (
        path.startswith("go/")
        and path.endswith(".go")
        and not path.endswith("_test.go")
        and not _coverage.go_excluded(path)
    )


def _in_python_scope(path: str) -> bool:
    return path.startswith("python/src/linodemcp/") and path.endswith(".py")


def _go_violations(added: dict[str, set[int]]) -> list[str]:
    """Added Go lines no test executed, per the coverage profile."""
    module = _coverage.go_module_name(_GO_MOD)
    blocks_by_file = _coverage.parse_go_profile(_GO_PROFILE, module, "go")
    problems: list[str] = []
    for path, lines in sorted(added.items()):
        if not _in_go_scope(path) or path not in blocks_by_file:
            continue
        uncovered = _coverage.go_uncovered_lines(blocks_by_file[path])
        problems.extend(f"{path}:{line}" for line in sorted(lines & uncovered))
    return problems


def _python_violations(added: dict[str, set[int]]) -> list[str]:
    """Added Python lines still in coverage.json's missing set."""
    missing_by_file = _coverage.python_missing_lines(_PY_COVERAGE, "python")
    problems: list[str] = []
    for path, lines in sorted(added.items()):
        if not _in_python_scope(path) or path not in missing_by_file:
            continue
        problems.extend(
            f"{path}:{line}" for line in sorted(lines & missing_by_file[path])
        )
    return problems


def _staleness_warnings(added: dict[str, set[int]]) -> list[str]:
    """Warn when a changed source file is newer than its coverage artifact.

    Advisory only: in CI the artifacts are always written after checkout,
    but locally an edit after the last test run makes the data stale, and
    a stale pass is worse than a loud hint.
    """
    warnings: list[str] = []
    checks = [
        (_GO_PROFILE, _in_go_scope, "make go-test"),
        (_PY_COVERAGE, _in_python_scope, "make python-test"),
    ]
    for artifact, in_scope, remedy in checks:
        changed = [p for p in added if in_scope(p) and (_REPO_ROOT / p).exists()]
        if not changed or not artifact.exists():
            continue
        newest = max((_REPO_ROOT / p).stat().st_mtime for p in changed)
        if newest > artifact.stat().st_mtime:
            warnings.append(
                f"warning: {artifact.name} is older than changed source;"
                f" rerun `{remedy}` for fresh data"
            )
    return warnings


def _missing_artifacts(added: dict[str, set[int]]) -> list[str]:
    """Coverage artifacts required by the diff but absent on disk."""
    problems: list[str] = []
    if any(_in_go_scope(p) for p in added) and not _GO_PROFILE.exists():
        problems.append(
            f"{_GO_PROFILE} is missing; run `make go-test` first"
            " (the test target writes it)"
        )
    if any(_in_python_scope(p) for p in added) and not _PY_COVERAGE.exists():
        problems.append(
            f"{_PY_COVERAGE} is missing; run `make python-test` first"
            " (pytest-cov writes it)"
        )
    return problems


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: verify_diff_coverage.py <base-git-rev>", file=sys.stderr)
        return 2

    base_rev = sys.argv[1]
    if not _rev_exists(base_rev):
        # A shallow clone or force-push can drop the reference point;
        # without one there is nothing to diff against, so report and
        # stand down rather than fail every unrelated change (the same
        # posture as baseline-guard).
        print(f"diff-coverage: base rev {base_rev!r} not found; skipping")
        return 0

    added = add_untracked_lines(added_lines_from_diff(_diff_text(base_rev)))
    missing = _missing_artifacts(added)
    if missing:
        print("diff-coverage gate failed:", file=sys.stderr)
        for problem in missing:
            print(f"  {problem}", file=sys.stderr)
        return 1

    for warning in _staleness_warnings(added):
        print(warning, file=sys.stderr)

    problems = _go_violations(added) + _python_violations(added)
    if problems:
        print(f"added lines vs {base_rev} with no test coverage:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        print(
            "\nEvery added executable line needs a unit test that reaches it"
            " (python may mark deliberate exclusions with `# pragma: no"
            " cover`). Add the tests; never ship the lines dark.",
            file=sys.stderr,
        )
        return 1

    checked = sum(
        len(lines)
        for path, lines in added.items()
        if _in_go_scope(path) or _in_python_scope(path)
    )
    print(f"diff-coverage OK: {checked} added source lines vs {base_rev} all covered")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
