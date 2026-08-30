#!/usr/bin/env python3
"""Offline gate: every internal link in README.md, llms.txt and docs/ resolves.

Markdown links rot silently: a moved contract file or renamed doc page
leaves a dead link no test notices. This walks every relative link target
in README.md, llms.txt, docs/**/*.md, and the prose each tool project
ships beside its code under tools/ (external URLs and pure #anchors
excluded, anchors on internal links stripped before the existence check)
and fails on the first pass listing every target that does not exist on
disk.

tools/ is in scope because a tool project's own wiki cross-links its
chapters and points back at repo paths. Those links became movable the
moment the wiki came in-repo, so leaving them unwalked would carve a hole
in the one gate that notices.

llms.txt is in scope for the same reason and it is the file most exposed
to it: it is a second copy of the docs index written for agents, so every
page this repo moves rots a line there while docs/ stays green.

Stdlib only, so no venv is needed. Run via `make docs-links` (in `make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parent.parent

# Inline markdown links: capture the target up to a closing paren or an
# anchor. Reference-style definitions ("[x]: path") are rare here and out
# of scope until a doc uses one.
_LINK = re.compile(r"\]\(([^)#\s]+)(?:#[^)]*)?\)")
_EXTERNAL_PREFIXES = ("http://", "https://", "mailto:")

# The two walked files that live at the repo root rather than inside a
# globbed tree. A glob that matches nothing is invisible, so these are
# named and their absence is reported.
_ROOT_DOCS = ("README.md", "llms.txt")


def _doc_files() -> list[Path]:
    files = [_REPO_ROOT / name for name in _ROOT_DOCS]
    files.extend(sorted((_REPO_ROOT / "docs").rglob("*.md")))
    files.extend(sorted((_REPO_ROOT / "tools").rglob("*.md")))
    return [path for path in files if path.is_file()]


def missing_root_docs() -> list[str]:
    """Named root files the walk expects and did not find."""
    return [name for name in _ROOT_DOCS if not (_REPO_ROOT / name).is_file()]


def broken_links() -> list[str]:
    """Every internal link whose target does not exist, as 'file: target'."""
    problems: list[str] = []
    for doc in _doc_files():
        text = doc.read_text(encoding="utf-8")
        for target in _LINK.findall(text):
            if target.startswith(_EXTERNAL_PREFIXES):
                continue
            resolved = (doc.parent / target).resolve()
            if not resolved.exists():
                relative = doc.relative_to(_REPO_ROOT)
                problems.append(f"{relative}: {target}")
    return problems


def main() -> int:
    missing = missing_root_docs()
    if missing:
        print("docs-links gate: a file it walks by name is gone:", file=sys.stderr)
        for name in missing:
            print(f"  {name}, so its links go unwalked", file=sys.stderr)
        return 1

    problems = broken_links()
    if problems:
        print("dead internal links in the docs:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        print(
            "  (fix the path or the moved file; docs/README.md is the index"
            " when unsure where a page went)",
            file=sys.stderr,
        )
        return 1

    total = len(_doc_files())
    print(f"docs-links gate OK: internal links resolve across {total} pages")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
