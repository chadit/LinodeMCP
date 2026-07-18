#!/usr/bin/env python3
"""Shared readers for the repo's tool surface, used by the offline gates.

Three facts several gates need, derived from committed sources only:

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
"""

from __future__ import annotations

import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
CAPABILITIES = REPO_ROOT / "docs" / "contracts" / "tools-capabilities.txt"
PY_TOOLS = REPO_ROOT / "python" / "src" / "linodemcp" / "tools"
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


def tool_input_messages(tools_dir: Path = PY_TOOLS) -> dict[str, str]:
    """Tool name to proto input message, read from the python tool factories."""
    source = "".join(
        path.read_text(encoding="utf-8") for path in sorted(tools_dir.glob("*.py"))
    )
    return dict(_FACTORY_RE.findall(source))


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
