"""Cross-language tool-surface parity gate.

``docs/tools-manifest.txt`` is the canonical tool surface for both
implementations. Every listed tool must exist in BOTH implementations:
this test asserts the Python registry equals exactly the full manifest
set, and the Go twin (``go/internal/server/tools_manifest_test.go``)
enforces the same. Any tab annotation on a manifest line (the retired
go-only/py-only mechanism) fails the test outright, so one-sided tools
cannot quietly return.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_MANIFEST_PATH = Path(__file__).resolve().parents[3] / "docs" / "tools-manifest.txt"


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


def test_tool_surface_matches_manifest() -> None:
    """The Python registry must equal the full manifest set exactly."""
    expected = _load_manifest()
    actual = {entry.name for entry in get_tool_registry()}

    missing = sorted(expected - actual)
    extra = sorted(actual - expected)

    assert not missing, (
        "tools in docs/tools-manifest.txt but not registered by the Python "
        f"server: {', '.join(missing)}"
    )
    assert not extra, (
        "tools registered by the Python server but missing from "
        f"docs/tools-manifest.txt: {', '.join(extra)}"
    )
