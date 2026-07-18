"""Cross-language tool-capability parity gate.

``docs/contracts/tools-capabilities.txt`` pins each tool's capability tier so the Go
and Python registries agree; a tool exposed under a different capability
would land in a different profile. The Go twin
(``go/internal/server/tools_capabilities_test.go``) checks the same file.
"""

from __future__ import annotations

from pathlib import Path

from linodemcp.server import get_tool_registry

_CAPABILITIES_PATH = (
    Path(__file__).resolve().parents[3]
    / "docs"
    / "contracts"
    / "tools-capabilities.txt"
)


def _load_capability_manifest() -> dict[str, str]:
    """Parse docs/contracts/tools-capabilities.txt into a tool -> tier map."""
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


def _load_missing_in_language(language: str) -> set[str]:
    """Return the tools whose absence in ``language`` is an accepted divergence.

    Same parse as test_tools_manifest.py: baseline lines read
    ``<tool>: missing in <language>`` with an optional trailing
    ``  # accepted ...`` annotation. The capability manifest keeps listing
    those tools so the other languages' tiers stay pinned.
    """
    baseline = _CAPABILITIES_PATH.with_name("tool-parity-baseline.txt")
    suffix = f": missing in {language}"
    tools: set[str] = set()

    for raw_line in baseline.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        entry = stripped.split("  # ", 1)[0].strip()
        if entry.endswith(suffix):
            tools.add(entry.removesuffix(suffix))

    return tools


def test_tool_capabilities_match_manifest() -> None:
    """Each tool's capability must equal docs/contracts/tools-capabilities.txt."""
    expected = _load_capability_manifest()
    actual = {entry.name: entry.capability.name for entry in get_tool_registry()}

    mismatched = sorted(
        f"{name} (manifest {expected[name]}, registry {tier})"
        for name, tier in actual.items()
        if name in expected and tier != expected[name]
    )
    missing = sorted(set(expected) - set(actual) - _load_missing_in_language("python"))
    extra = sorted(set(actual) - set(expected))

    assert not mismatched, (
        "tool capabilities differ from docs/contracts/tools-capabilities.txt: "
        + ", ".join(mismatched)
    )
    assert not missing, (
        "tools in docs/contracts/tools-capabilities.txt but not registered by Python: "
        + ", ".join(missing)
    )
    assert not extra, (
        "tools registered by Python but missing from "
        "docs/contracts/tools-capabilities.txt: " + ", ".join(extra)
    )
