#!/usr/bin/env python3
"""Shared coverage-artifact parsing for the coverage gates.

verify_coverage_floor.py (offline, in `make check`) and
verify_diff_coverage.py (diff-aware, beside baseline-guard) read the same
two artifacts a full test run leaves behind:

- go/coverage.out: the statement-coverage profile the Go test target writes
  (`go test -race -coverprofile=coverage.out ./...`). After the mode header,
  each line is `<import-path>/<file>.go:<sl>.<sc>,<el>.<ec> <stmts> <count>`.
- python/coverage.json: the coverage.py JSON report pytest-cov writes
  (`--cov-report=json` in pyproject addopts), keyed by path relative to the
  python/ working directory.

Both parsers normalize to repo-relative POSIX paths (go/internal/...,
python/src/...) so the gates can talk about files the way `git diff` does.
Stdlib only, so no venv is needed.
"""

from __future__ import annotations

import json
from pathlib import Path

# (start_line, end_line, statement_count, hit_count) for one profile block.
GoBlock = tuple[int, int, int, int]

# Hand-written-code scope for the Go side, shared by both gates: the
# generated genpb tree is never hand-edited (same exclusion fmt and gosec
# apply), and cmd/ holds the entrypoint mains plus one-shot gate dump
# tools, the Go mirror of the python coverage omit for main.py.
GO_EXCLUDED_PREFIXES = ("go/internal/genpb/", "go/cmd/")


def go_excluded(path: str) -> bool:
    """Report whether a repo-relative Go path is outside coverage scope."""
    return path.startswith(GO_EXCLUDED_PREFIXES)


def go_module_name(go_mod: Path) -> str:
    """The module path declared in a go.mod file."""
    for line in go_mod.read_text(encoding="utf-8").splitlines():
        if line.startswith("module "):
            return line.split()[1]
    msg = f"no module line in {go_mod}"
    raise ValueError(msg)


def parse_go_profile(
    profile: Path, module: str, module_dir: str
) -> dict[str, list[GoBlock]]:
    """Blocks per repo-relative file from a Go coverage profile.

    ``module`` is the import-path prefix to strip (from go.mod) and
    ``module_dir`` the repo directory it corresponds to (``go``). Profile
    lines for files outside the module (none in practice) are skipped
    rather than guessed at.
    """
    blocks: dict[str, list[GoBlock]] = {}
    prefix = module + "/"
    for raw in profile.read_text(encoding="utf-8").splitlines()[1:]:
        if not raw.strip():
            continue
        location, stmts, count = raw.split()
        import_path, _, span = location.rpartition(":")
        if not import_path.startswith(prefix):
            continue
        start, _, end = span.partition(",")
        rel = f"{module_dir}/{import_path[len(prefix) :]}"
        blocks.setdefault(rel, []).append(
            (
                int(start.split(".")[0]),
                int(end.split(".")[0]),
                int(stmts),
                int(count),
            )
        )
    return blocks


def go_statement_totals(
    blocks_by_file: dict[str, list[GoBlock]],
) -> tuple[int, int]:
    """(covered, total) statement counts over the in-scope files."""
    covered = 0
    total = 0
    for path, blocks in blocks_by_file.items():
        if go_excluded(path):
            continue
        for _, _, stmts, count in blocks:
            total += stmts
            if count > 0:
                covered += stmts
    return covered, total


def go_uncovered_lines(blocks: list[GoBlock]) -> set[int]:
    """Lines of one file no test executed.

    A line can sit in more than one block (a one-line ``if x { y() }``
    spans the condition block and the body block); it counts as covered
    when ANY block containing it was hit, so overlap never false-flags.
    """
    hits: dict[int, int] = {}
    for start, end, _, count in blocks:
        for line in range(start, end + 1):
            hits[line] = max(hits.get(line, 0), count)
    return {line for line, count in hits.items() if count == 0}


def python_missing_lines(coverage_json: Path, prefix: str) -> dict[str, set[int]]:
    """Uncovered executable lines per repo-relative file from coverage.json.

    coverage.py measures every file under its configured source (even ones
    no test imported), minus the omit list, so a file ABSENT from this map
    was deliberately omitted, not silently missed. ``# pragma: no cover``
    lines land in excluded_lines, not missing_lines, so they never appear
    here either.
    """
    data = json.loads(coverage_json.read_text(encoding="utf-8"))
    missing: dict[str, set[int]] = {}
    for rel, info in data["files"].items():
        missing[f"{prefix}/{Path(rel).as_posix()}"] = set(info["missing_lines"])
    return missing
