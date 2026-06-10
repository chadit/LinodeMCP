"""Cross-language tool-surface parity gate.

``docs/tools-manifest.txt`` is the canonical tool surface for both
implementations. This test asserts the Python registry equals exactly the
manifest's names minus the go-only lines. The Go twin
(``go/internal/server/tools_manifest_test.go``) enforces the same manifest
minus the py-only lines. Together they stop the two tool surfaces from
drifting apart again.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_MANIFEST_PATH = Path(__file__).resolve().parents[3] / "docs" / "tools-manifest.txt"

_ANNOTATION_GO_ONLY = "go-only"
_ANNOTATION_PY_ONLY = "py-only"


def _load_manifest() -> dict[str, str]:
    """Parse the manifest into name -> annotation ('' when registered in both)."""
    entries: dict[str, str] = {}

    for raw_line in _MANIFEST_PATH.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue

        name, _, annotation = line.partition("\t")
        name = name.strip()
        annotation = annotation.strip()

        assert annotation in ("", _ANNOTATION_GO_ONLY, _ANNOTATION_PY_ONLY), (
            f"manifest line {name!r} has unknown annotation {annotation!r}"
        )
        assert name not in entries, f"manifest lists {name!r} twice"

        entries[name] = annotation

    return entries


def test_tool_surface_matches_manifest() -> None:
    """The Python registry must equal the manifest minus go-only lines."""
    manifest = _load_manifest()

    expected = {
        name
        for name, annotation in manifest.items()
        if annotation != _ANNOTATION_GO_ONLY
    }
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
