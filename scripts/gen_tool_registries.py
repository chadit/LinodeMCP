#!/usr/bin/env python3
"""Emit the tool registries the proto contract already states.

docs/contracts/tools-manifest.txt lists every tool any language serves and
docs/contracts/tools-capabilities.txt gives each one its tier. Both were kept by
hand, and both restate something the descriptors already carry: a tool's name
rides in `tool_route` or `tool_meta`, and its tier in `tool_capability`. Two
copies of one fact is a drift vector, and `make tool-capability` existed only to
watch the copies for drift.

So they are emitted here instead, by `make proto`, into the same gitignored
category the generated code is in. Adding a tool means adding its proto message;
nothing else. The gates that read these files keep reading them. The comparison
gate itself is retired: a file rewritten from the descriptors on every regen
cannot drift from them, staleness is what the proto stamp governs, and the
per-language registry tests still pin that each client serves what the files
say.

The formats are unchanged, headers apart, because roughly twenty consumers parse
them: the manifest and capability tests in each language, the offline gates, and
the parity dumpers.

Reading descriptors needs the protobuf runtime and the generated modules, which
scripts/_toolroutes.py resolves through python/.venv/bin/python when the running
interpreter cannot import them.

Usage: gen_tool_registries.py [-manifest PATH] [-capabilities PATH]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

import _toolroutes

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"

_DEFAULT_MANIFEST = _CONTRACTS / "tools-manifest.txt"
_DEFAULT_CAPABILITIES = _CONTRACTS / "tools-capabilities.txt"

# Proto enum value to the spelling the capability manifest uses. Mapped
# explicitly because the two vocabularies are independent: deriving one from the
# other would mistranslate the first tier whose name takes more than one word.
_TIERS = {
    "TOOL_CAPABILITY_READ": "Read",
    "TOOL_CAPABILITY_WRITE": "Write",
    "TOOL_CAPABILITY_DESTROY": "Destroy",
    "TOOL_CAPABILITY_ADMIN": "Admin",
    "TOOL_CAPABILITY_META": "Meta",
}

_MANIFEST_HEADER = """\
# Canonical LinodeMCP tool surface: one tool name per line, sorted.
# Written from the `tool_route` and `tool_meta` options by
# scripts/gen_tool_registries.py under `make proto`. Gitignored and rewritten
# whole every run, so edit the proto, not this file. A language landing behind
# the others records the gap under docs/parity.md's accepted-absence flow.
# Enforced by:
#   go/internal/server/tools_manifest_test.go
#   python/tests/unit/test_tools_manifest.py
"""

_CAPABILITIES_HEADER = """\
# Canonical LinodeMCP tool capability tags: "<tool>\tCapability" per line,
# sorted by tool name, one of Read, Write, Destroy, Admin, Meta.
# Written from the `tool_capability` option by scripts/gen_tool_registries.py
# under `make proto`. Gitignored and rewritten whole every run, so edit the
# proto, not this file. What each tier covers lives on ToolCapability in
# proto/linode/mcp/v1/options.proto.
# Enforced by:
#   go/internal/server/tools_capabilities_test.go
#   python/tests/unit/test_tools_capabilities.py
"""


class GenError(RuntimeError):
    """The descriptors cannot answer something a registry line needs."""


def tool_tiers() -> dict[str, str]:
    """Tool name to manifest-spelled tier, for every tool the proto declares.

    A message naming its tool in both markers, in neither, or under a tier with
    no spelling here stops the run. Writing a registry from a contradictory
    declaration would file the tool under a name that may not be its own, and
    the gates downstream would then report the resulting mess as drift.
    """
    tiers: dict[str, str] = {}

    for entry in _toolroutes.declarations():
        short = entry.message.removeprefix("linode.mcp.v1.")

        if entry.route_tool and entry.meta_tool:
            msg = (
                f"{short}: names {entry.route_tool} as a routed tool and"
                f" {entry.meta_tool} as a meta tool"
            )
            raise GenError(msg)

        name = entry.route_tool or entry.meta_tool
        if not name:
            msg = f"{short}: declares a capability but no marker names its tool"
            raise GenError(msg)

        tier = _TIERS.get(entry.capability)
        if tier is None:
            msg = f"{short}: {name} declares {entry.capability}, which has no tier"
            raise GenError(msg)

        if name in tiers:
            msg = f"{name} is declared by more than one message"
            raise GenError(msg)

        tiers[name] = tier

    return tiers


def render_manifest(tiers: dict[str, str]) -> str:
    """The tool surface, one name per line."""
    return _MANIFEST_HEADER + "".join(f"{name}\n" for name in sorted(tiers))


def render_capabilities(tiers: dict[str, str]) -> str:
    """Each tool's tier, tab-separated, in the same order as the manifest."""
    return _CAPABILITIES_HEADER + "".join(
        f"{name}\t{tiers[name]}\n" for name in sorted(tiers)
    )


def generate(manifest: Path, capabilities: Path) -> int:
    """Write both registries, answering how many tools they hold."""
    tiers = tool_tiers()

    manifest.write_text(render_manifest(tiers), encoding="utf-8")
    capabilities.write_text(render_capabilities(tiers), encoding="utf-8")

    return len(tiers)


def main(argv: list[str] | None = None) -> int:
    """Emit both registries, or report the one thing that stopped it."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-manifest", type=Path, default=_DEFAULT_MANIFEST)
    parser.add_argument("-capabilities", type=Path, default=_DEFAULT_CAPABILITIES)
    args = parser.parse_args(argv)

    try:
        count = generate(args.manifest, args.capabilities)
    except GenError as exc:
        sys.stderr.write(f"gen_tool_registries: {exc}\n")
        return 1

    sys.stderr.write(f"gen_tool_registries: wrote {count} tool(s)\n")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
