"""Cross-language tool-capability parity gate.

``docs/tools-capabilities.txt`` pins each tool's capability tier so the Go
and Python registries agree; a tool exposed under a different capability
would land in a different profile. The Go twin
(``go/internal/server/tools_capabilities_test.go``) checks the same file.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_CAPABILITIES_PATH = (
    Path(__file__).resolve().parents[3] / "docs" / "tools-capabilities.txt"
)


def _load_capability_manifest() -> dict[str, str]:
    """Parse docs/tools-capabilities.txt into a tool -> tier map."""
    entries: dict[str, str] = {}

    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        name, sep, tier = raw_line.partition("\t")
        assert sep, (
            f"capability manifest line {raw_line!r} is not <tool>\\t<Capability>"
        )

        name = name.strip()
        assert name not in entries, f"capability manifest lists {name!r} twice"
        entries[name] = tier.strip()

    return entries


def test_tool_capabilities_match_manifest() -> None:
    """Every registered tool's capability must equal docs/tools-capabilities.txt."""
    expected = _load_capability_manifest()
    actual = {entry.name: entry.capability.name for entry in get_tool_registry()}

    mismatched = sorted(
        f"{name} (manifest {expected[name]}, registry {tier})"
        for name, tier in actual.items()
        if name in expected and tier != expected[name]
    )
    missing = sorted(set(expected) - set(actual))
    extra = sorted(set(actual) - set(expected))

    assert not mismatched, (
        "tool capabilities differ from docs/tools-capabilities.txt: "
        + ", ".join(mismatched)
    )
    assert not missing, (
        "tools in docs/tools-capabilities.txt but not registered by Python: "
        + ", ".join(missing)
    )
    assert not extra, (
        "tools registered by Python but missing from docs/tools-capabilities.txt: "
        + ", ".join(extra)
    )
