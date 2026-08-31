#!/usr/bin/env python3
"""Byte-identity harness for everything `make proto` emits.

The single-emitter work moves rendering between languages and hosts, and the
only acceptance any of it has is that the emitted bytes do not move with it. So
this script freezes a sha256 line per generated file, then reports per-file
drift against that frozen manifest: added, removed, and changed, each by name.

It regenerates nothing. Run `make proto` (or wipe the trees and run it) first,
then compare; keeping generation out of here is what lets the same manifest
answer both the in-place regen and the from-empty regen.

The cohorts are the two emitted tool trees, the Go schema data the emitter
reads back, and the two registries scripts/gen_tool_registries.py writes. All
five are gitignored build output, which is why a checksum file is the only
record of what they held before a refactor.

Usage:
    python3 scripts/generated_goldens.py freeze <manifest>
    python3 scripts/generated_goldens.py compare <manifest>
"""

from __future__ import annotations

import argparse
import hashlib
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]

# Every path `make proto` writes that a renderer refactor could move.
COHORTS: tuple[str, ...] = (
    "go/internal/gentools",
    "python/src/linodemcp/gentools",
    "go/internal/genlocal",
    "python/src/linodemcp/genlocal",
    "go/internal/toolschemas/data",
    "docs/contracts/tools-manifest.txt",
    "docs/contracts/tools-capabilities.txt",
)

# Interpreter caches sit inside the Python tool tree and are not emitted output.
EXCLUDED_DIRS = frozenset({"__pycache__"})

_HEADER = (
    "# sha256 manifest of the generated cohorts, written by "
    "scripts/generated_goldens.py.\n"
    "# One line per file, path-sorted, repo-relative. Regenerate with "
    "`freeze`, check with `compare`.\n"
)

# Read in blocks so a multi-megabyte schema bundle does not land in memory whole.
_READ_BLOCK = 1 << 20  # 1 MiB, the usual sweet spot for hashing throughput


class ManifestError(Exception):
    """A stored manifest could not be read as hash-and-path lines."""


def hash_file(path: Path) -> str:
    """The sha256 of one file, hex."""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while block := handle.read(_READ_BLOCK):
            digest.update(block)
    return digest.hexdigest()


def cohort_files(repo_root: Path, cohort: str) -> list[Path]:
    """Repo-relative paths a cohort covers, whether it names a file or a tree."""
    root = repo_root / cohort
    if root.is_file():
        return [Path(cohort)]
    if not root.is_dir():
        return []

    found: list[Path] = []
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        if EXCLUDED_DIRS.intersection(path.relative_to(root).parts):
            continue
        found.append(path.relative_to(repo_root))
    return found


def build_manifest(
    repo_root: Path, cohorts: tuple[str, ...] = COHORTS
) -> dict[str, str]:
    """Map repo-relative path to sha256 for every file the cohorts cover."""
    entries: dict[str, str] = {}
    for cohort in cohorts:
        for relative in cohort_files(repo_root, cohort):
            entries[relative.as_posix()] = hash_file(repo_root / relative)
    return entries


def format_manifest(entries: dict[str, str]) -> str:
    """Render a manifest, path-sorted so two freezes of one tree agree byte for byte."""
    lines = [f"{entries[path]}  {path}" for path in sorted(entries)]
    return _HEADER + "\n".join(lines) + "\n"


def parse_manifest(text: str) -> dict[str, str]:
    """Read a stored manifest back, rejecting a line that is not hash-and-path."""
    entries: dict[str, str] = {}
    for number, raw in enumerate(text.splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        digest, separator, path = line.partition("  ")
        if not separator or not path:
            raise ManifestError(
                f"line {number}: expected '<sha256>  <path>', got {raw!r}"
            )
        entries[path.strip()] = digest
    return entries


def drift(stored: dict[str, str], current: dict[str, str]) -> dict[str, list[str]]:
    """Per-file difference between a frozen manifest and the tree on disk."""
    return {
        "added": sorted(set(current) - set(stored)),
        "removed": sorted(set(stored) - set(current)),
        "changed": sorted(
            path
            for path, digest in current.items()
            if path in stored and stored[path] != digest
        ),
    }


def report(found: dict[str, list[str]]) -> None:
    """Print each drifted path under its kind, readable without a diff tool."""
    for kind in ("added", "removed", "changed"):
        for path in found[kind]:
            print(f"{kind}: {path}", file=sys.stderr)


def _measure(repo_root: Path, cohorts: tuple[str, ...]) -> dict[str, str]:
    """Hash the cohorts, refusing an empty result.

    Every cohort is written by `make proto`, so nothing found means the root is
    wrong or generation never ran. Answering "no drift" there would pass each
    refactor stage while comparing zero files against zero files.
    """
    entries = build_manifest(repo_root, cohorts)
    if not entries:
        raise ManifestError(
            f"no generated files under {repo_root} for cohorts {cohorts}"
        )
    return entries


def freeze(repo_root: Path, manifest: Path, cohorts: tuple[str, ...] = COHORTS) -> int:
    entries = _measure(repo_root, cohorts)
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(format_manifest(entries), encoding="utf-8")
    print(f"goldens frozen: {len(entries)} files -> {manifest}")
    return 0


def compare(repo_root: Path, manifest: Path, cohorts: tuple[str, ...] = COHORTS) -> int:
    if not manifest.is_file():
        print(f"goldens: no manifest at {manifest}; run freeze first.", file=sys.stderr)
        return 1

    stored = parse_manifest(manifest.read_text(encoding="utf-8"))
    current = _measure(repo_root, cohorts)
    found = drift(stored, current)
    total = sum(len(paths) for paths in found.values())

    if total:
        report(found)
        print(
            f"goldens DRIFT: {len(found['added'])} added, "
            f"{len(found['removed'])} removed, {len(found['changed'])} changed "
            f"(against {manifest}).",
            file=sys.stderr,
        )
        return 1

    print(f"goldens OK: {len(current)} files byte-identical to {manifest}")
    return 0


def main(argv: list[str] | None = None) -> int:
    # python -OO strips module docstrings, so bind the summary line only when
    # one survives; argparse treats a missing description the same as None.
    summary = __doc__.splitlines()[0] if __doc__ else None
    parser = argparse.ArgumentParser(description=summary)
    parser.add_argument("action", choices=("freeze", "compare"))
    parser.add_argument("manifest", type=Path)
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=REPO_ROOT,
        help="tree to hash (defaults to this checkout)",
    )
    args = parser.parse_args(argv)

    action = freeze if args.action == "freeze" else compare
    return action(args.repo_root.resolve(), args.manifest)


if __name__ == "__main__":
    raise SystemExit(main())
