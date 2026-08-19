"""Offline tests for the README tool-count drift guard.

verify_docs_tool_count.py fails when a README line that cites
docs/contracts/tools-manifest.txt states a tool count that disagrees with the
number of tools the proto contract declares. The manifest is generated from that
contract, so the count comes from the contract itself rather than through the
file. These tests pin the parsing and the live README.
"""

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


guard = _load_script("verify_docs_tool_count")


def test_declared_total_matches_the_generated_manifest() -> None:
    """The count the guard reports is the surface the manifest was written from.

    Reading the descriptors and reading the generated file have to give the same
    answer, since one is written from the other; a difference would mean the
    manifest on disk predates the contract.
    """
    manifest = REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"
    listed = sum(
        1
        for raw in manifest.read_text(encoding="utf-8").splitlines()
        if raw.strip() and not raw.strip().startswith("#")
    )

    assert guard.declared_total() == listed


def test_readme_claims_reads_only_manifest_lines() -> None:
    text = (
        "Coverage spans 12 tools for networking.\n"
        "Pinned by [docs/contracts/tools-manifest.txt](x), which lists 460 tools.\n"
    )

    # The 12-tools line does not cite the manifest, so it is ignored.
    assert guard.readme_claims(text) == [460]


def test_readme_claims_matches_hyphenated_phrasing() -> None:
    text = "the same 454-tool surface, pinned by tools-manifest.txt today\n"

    assert guard.readme_claims(text) == [454]


def test_live_readme_matches_the_declared_surface() -> None:
    """The real README's count must equal the tools the contract declares.

    This is the drift guard itself as a test: it fails the moment the README
    prose and the proto disagree.
    """
    readme = (REPO_ROOT / "README.md").read_text(encoding="utf-8")
    claims = guard.readme_claims(readme)

    assert claims, "README states no tool count on a line citing the manifest"
    assert set(claims) == {guard.declared_total()}
