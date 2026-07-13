"""Cross-language tool-surface parity gate.

``docs/tools-manifest.txt`` is the canonical shared tool surface for both
implementations. Every listed tool must exist in BOTH implementations; accepted
language-specific additions are recorded in ``docs/tool-parity-baseline.txt``
and remain outside the shared manifest.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_MANIFEST_PATH = Path(__file__).resolve().parents[3] / "docs" / "tools-manifest.txt"
_PARITY_BASELINE_PATH = (
    Path(__file__).resolve().parents[3] / "docs" / "tool-parity-baseline.txt"
)


def _load_manifest() -> set[str]:
    """Parse the manifest into the set of tool names, rejecting annotations."""
    entries: set[str] = set()

    for raw_line in _MANIFEST_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        assert "\t" not in raw_line, (
            f"manifest line {raw_line!r} carries a tab annotation; "
            "one-sided tools are not allowed"
        )

        assert stripped not in entries, f"manifest lists {stripped!r} twice"

        entries.add(stripped)

    return entries


def _load_python_only_tools() -> set[str]:
    """Return tools explicitly accepted as registered only in Python."""
    suffix = ": registered in Python but not Go"
    return {
        line.removesuffix(suffix)
        for line in _PARITY_BASELINE_PATH.read_text(encoding="utf-8").splitlines()
        if line.endswith(suffix)
    }


def test_tool_surface_matches_manifest() -> None:
    """The Python registry must equal the shared plus accepted Python surface."""
    expected = _load_manifest()
    actual = {entry.name for entry in get_tool_registry()}

    missing = sorted(expected - actual)
    extra = sorted(actual - expected - _load_python_only_tools())

    assert not missing, (
        "tools in docs/tools-manifest.txt but not registered by the Python "
        f"server: {', '.join(missing)}"
    )
    assert not extra, (
        "tools registered by the Python server but missing from "
        f"docs/tools-manifest.txt: {', '.join(extra)}"
    )
