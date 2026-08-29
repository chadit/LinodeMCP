#!/usr/bin/env python3
"""Hard gate: no gopls analyzer finding in any registered Go tree.

gopls carries the analyzer set a developer's editor reports (modernize and
friends), and it releases ahead of the x/tools module tags golangci-lint
depends on. So a finding lands in the editor long before any linter in this
repo's gate would see it, and twenty sealed batches accumulated thirty-eight
of them without one turning `make check` red. This closes that: the analyzers
the editor runs now run in the gate too.

What is scanned: every ``*.go`` file under each registered language whose
working directory carries a go.mod, as docs/contracts/languages.txt names those
directories. Reading the registry rather than a path literal is what keeps a
renamed or newly registered Go tree in scope. Generated trees are scanned with
the rest on purpose: an emitter that starts writing code an analyzer flags is
exactly the finding this gate exists to catch, and the emitter is the only place
it can be fixed.

gopls exits zero even when it reports findings, so any output at all is the
failure. gopls is a hard requirement, not a warn-skip: a machine without it
would otherwise pass a scan CI ran. scripts/ci-setup.sh installs it.

Run via `make go-analyzers` (in `make check`, and so in the pre-push hook and
the CI gate on every branch).

Usage: verify_go_analyzers.py
"""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

import _hardgate

_ROOT = Path(__file__).resolve().parent.parent
_LANGUAGES = _ROOT / "docs" / "contracts" / "languages.txt"

_FIX = (
    "Rewrite the flagged code so the analyzer has nothing to report. For a\n"
    "finding inside a generated tree, the fix belongs in go/cmd/toolgen: the\n"
    "generated file is overwritten on the next `make proto`."
)


def _go_trees() -> list[Path]:
    """Return each registered language's working directory that holds a go.mod.

    The registry names the directory; the go.mod is what says the directory is
    a Go module, so neither half is a path literal here.
    """
    trees: list[Path] = []
    for raw in _LANGUAGES.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) < 2:
            continue
        working = _ROOT / fields[1].strip()
        if (working / "go.mod").is_file():
            trees.append(working)
    return trees


def _sources(tree: Path) -> list[Path]:
    """Return every Go source file under one tree, in a stable order."""
    return sorted(path for path in tree.rglob("*.go") if path.is_file())


def main() -> int:
    """Run gopls over every registered Go tree and report what it found."""
    if shutil.which("gopls") is None:
        raise SystemExit(
            "gopls is required (install: go install golang.org/x/tools/gopls@latest,"
            " or run scripts/ci-setup.sh)"
        )

    findings: list[str] = []
    scanned = 0
    for tree in _go_trees():
        sources = _sources(tree)
        scanned += len(sources)
        if not sources:
            continue
        result = subprocess.run(
            ["gopls", "check", *(str(path) for path in sources)],
            capture_output=True,
            text=True,
            check=False,
            cwd=tree,
        )
        findings.extend(
            line
            for line in result.stdout.splitlines() + result.stderr.splitlines()
            if line.strip()
        )

    _hardgate.measured("gopls analyzer scan", scanned)
    _hardgate.say(f"scanned {scanned} Go source file(s) with gopls")

    return _hardgate.report("gopls analyzer findings", findings, _FIX)


if __name__ == "__main__":
    sys.exit(main())
