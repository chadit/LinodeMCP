#!/usr/bin/env python3
"""Shared readers for the repo's tool surface, used by the offline gates.

Three facts several gates need:

- which capability each tool carries (docs/contracts/tools-capabilities.txt);
- which proto input message each tool uses (the python factories declare
  name and schema together, and both languages generate from the same proto,
  so the factory is a language-neutral map);
- each proto message's body text, for field-presence checks.

The factory matcher is tempered so it can never read past the next factory:
a Tool( block whose input schema is not proto-generated must not cause its
neighbor's message to be paired with the wrong tool. The proto reader walks
braces instead of pattern-matching to the first closing brace, so messages
containing nested blocks still parse.

Factories come from two trees now: the hand-written one and the gitignored one
scripts/toolgen_py.py emits. Reading only the committed tree would have left a
generated tool out of every gate that consults this, and a gate that measures
less than it did still prints OK, so the cohort file (which is committed) is
what holds the generated half to being present.
"""

from __future__ import annotations

import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
CAPABILITIES = REPO_ROOT / "docs" / "contracts" / "tools-capabilities.txt"
GENERATED_TOOLS = REPO_ROOT / "docs" / "contracts" / "generated-tools.txt"
PY_TOOLS = REPO_ROOT / "python" / "src" / "linodemcp" / "tools"
PY_GENTOOLS = REPO_ROOT / "python" / "src" / "linodemcp" / "gentools"
PROTO_DIR = REPO_ROOT / "proto" / "linode" / "mcp" / "v1"

# Tempered dot: anything except the start of another factory's name=.
_FACTORY_RE = re.compile(
    r'name="([a-z0-9_]+)",(?:(?!name=")[\s\S])*?'
    r'schema\(\s*"linode\.mcp\.v1\.(\w+)"\s*\)'
)
_MESSAGE_START_RE = re.compile(r"^message (\w+) \{", re.MULTILINE)


def read_capabilities(path: Path = CAPABILITIES) -> dict[str, str]:
    """Tool name to capability tier from the capabilities contract."""
    out: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        tool, _, capability = stripped.partition("\t")
        if tool and capability:
            out[tool] = capability
    return out


def tool_input_messages(tools_dir: Path | None = None) -> dict[str, str]:
    """Tool name to proto input message, read from the python tool factories.

    Called with nothing, it reads this repo's whole factory surface: the
    hand-written tree and the generated one beside it, then holds the result to
    naming every tool docs/contracts/generated-tools.txt lists. Without that
    check a checkout where `make proto` has not run would answer with a map
    missing the generated tools, and every gate reading it would pass while
    checking less than it did.

    Called with a directory, it reads that directory and nothing else, which is
    what the tests of the matcher itself need: a tree of their own is not this
    repo's surface, so the contract above says nothing about it.
    """
    if tools_dir is not None:
        return _factories([tools_dir])

    found = _factories([PY_TOOLS, PY_GENTOOLS])

    missing = sorted(set(generated_tools()) - set(found))
    if missing:
        names = ", ".join(missing)
        msg = (
            f"no factory found for generated tool(s) {names};"
            f" {PY_GENTOOLS} is written by `make proto`"
        )
        raise SystemExit(msg)

    return found


def _factories(dirs: list[Path]) -> dict[str, str]:
    """Every factory's tool name and input message across the given trees."""
    source = "".join(
        path.read_text(encoding="utf-8")
        for directory in dirs
        for path in sorted(directory.glob("*.py"))
    )
    return dict(_FACTORY_RE.findall(source))


def generated_tools(path: Path = GENERATED_TOOLS) -> list[str]:
    """The tools whose factories the emitters write, in file order."""
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]


def proto_message_bodies(proto_dir: Path = PROTO_DIR) -> dict[str, str]:
    """Proto message name to its brace-balanced body text."""
    out: dict[str, str] = {}
    for path in sorted(proto_dir.glob("*.proto")):
        source = path.read_text(encoding="utf-8")
        for match in _MESSAGE_START_RE.finditer(source):
            body = _balanced_body(source, match.end())
            if body is not None:
                out[match.group(1)] = body
    return out


def _balanced_body(source: str, start: int) -> str | None:
    """Body text from just after an opening brace to its matching close."""
    depth = 1
    for index in range(start, len(source)):
        char = source[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[start:index]
    return None


def message_has_field(body: str, field: str) -> bool:
    """Whether a message body declares the named scalar field."""
    return re.search(rf"\b{re.escape(field)}\b\s*=", body) is not None
