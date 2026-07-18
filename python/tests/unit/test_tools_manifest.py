"""Cross-language tool-surface parity gate.

``docs/contracts/tools-manifest.txt`` is the full canonical tool surface: every tool any
registered language implements. A language that has not caught up on a listed
tool records that absence in ``docs/contracts/tool-parity-baseline.txt`` as
``<tool>: missing in <language>`` with a tracking annotation, so this test
skips exactly that accepted set in its missing check. Extra tools are never
excused: a tool cannot register here without entering the manifest. The Go
twin (``go/internal/server/tools_manifest_test.go``) enforces the same
contract for its side.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_MANIFEST_PATH = (
    Path(__file__).resolve().parents[3] / "docs" / "contracts" / "tools-manifest.txt"
)
_PARITY_BASELINE_PATH = (
    Path(__file__).resolve().parents[3]
    / "docs"
    / "contracts"
    / "tool-parity-baseline.txt"
)


def _load_manifest() -> set[str]:
    """Parse the manifest into the set of tool names, rejecting annotations."""
    entries: set[str] = set()

    for raw_line in _MANIFEST_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        assert "\t" not in raw_line, (
            f"manifest line {raw_line!r} carries a tab annotation; per-language "
            "absences belong in docs/contracts/tool-parity-baseline.txt"
        )

        assert stripped not in entries, f"manifest lists {stripped!r} twice"

        entries.add(stripped)

    return entries


def _load_missing_in_language(language: str) -> set[str]:
    """Return the tools whose absence in ``language`` is an accepted divergence.

    Baseline lines read ``<tool>: missing in <language>`` with an optional
    trailing ``  # accepted ...`` annotation; scripts/verify_tool_parity.py
    enforces the annotation's presence and format.
    """
    suffix = f": missing in {language}"
    tools: set[str] = set()

    for raw_line in _PARITY_BASELINE_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        entry = stripped.split("  # ", 1)[0].strip()
        if entry.endswith(suffix):
            tools.add(entry.removesuffix(suffix))

    return tools


def test_tool_surface_matches_manifest() -> None:
    """The Python registry must equal the manifest minus accepted absences."""
    expected = _load_manifest()
    actual = {entry.name for entry in get_tool_registry()}

    missing = sorted(expected - actual - _load_missing_in_language("python"))
    extra = sorted(actual - expected)

    assert not missing, (
        "tools in docs/contracts/tools-manifest.txt but not registered by the Python "
        f"server: {', '.join(missing)}"
    )
    assert not extra, (
        "tools registered by the Python server but missing from "
        f"docs/contracts/tools-manifest.txt: {', '.join(extra)}"
    )
