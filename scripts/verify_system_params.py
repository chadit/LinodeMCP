#!/usr/bin/env python3
"""Offline gate: every MCP system param carries a trailing `// system param`.

A handful of proto input fields are the server's own plumbing rather than
Linode API parameters: `environment` picks the credential profile from MCP
config, `confirm` is the write-confirmation gate, `dry_run` is the preview
flag, and `mode`/`plan_id` drive the two-stage plan/apply flow. Nothing in the
proto files says so, so a reader cannot tell them apart from the API params
sitting next to them, and a spec-parity pass has no way to skip them.

The marker is a TRAILING comment on the field line:

    optional bool dry_run = 5; // system param

Trailing comments do not reach the generated JSON Schema. A proto LEADING
comment becomes the `description` an MCP client shows the model, so putting
the marker in the leading block would rewrite 970 user-facing descriptions.
Keep it on the field line.

docs/contracts/system-params.txt holds the name-and-type set, not this script,
so widening the set is a contract edit plus the matching proto markers. The
type column is part of the key: `mode` is `optional string` for the two-stage
selector and `optional NodeBalancerNodeMode.Value` for the NodeBalancer backend
node mode, which is a real API body param.

Both directions fail. A field matching the contract without the marker fails,
and a field carrying the marker that the contract does not name fails. Markers
are required only inside *Input messages, since response and nested item
messages reuse the same names for real API values; a stray marker in one of
those still fails.

Stdlib only, so no venv is needed. Run via `make system-params` (in `make
check`, and so in the pre-push hook and the CI gate on every branch).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import NamedTuple

_REPO_ROOT = Path(__file__).resolve().parent.parent
_CONTRACT = _REPO_ROOT / "docs" / "contracts" / "system-params.txt"
_PROTO_DIR = _REPO_ROOT / "proto" / "linode" / "mcp" / "v1"

MARKER = "// system param"

_MESSAGE = re.compile(r"^message\s+([A-Za-z_]\w*)\s*\{")
# A field line: optional cardinality, a type (map<...> counts as one token), the
# field name, the tag number, then whatever trails the semicolon.
_FIELD = re.compile(
    r"^\s*(?:(?:optional|repeated)\s+)?"
    r"(map<[^>]+>|[A-Za-z_][\w.]*)\s+"
    r"([a-z_][a-z0-9_]*)\s*=\s*\d+\s*;"
    r"(.*)$"
)


class Field(NamedTuple):
    """One proto field, located well enough to print a file:line violation."""

    path: str
    line: int
    message: str
    name: str
    type: str
    marked: bool

    def where(self) -> str:
        """`file:line message.field`, the anchor every violation prints."""
        return f"{self.path}:{self.line} {self.message}.{self.name}"


def contract_entries() -> set[tuple[str, str]]:
    """The pinned (field name, proto type) system-param set."""
    entries: set[tuple[str, str]] = set()
    for raw in _CONTRACT.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = [field.strip() for field in line.split("\t") if field.strip()]
        if len(fields) != 2:
            msg = f"unparsable {_CONTRACT.name} line {raw!r}"
            raise SystemExit(msg)
        entries.add((fields[0], fields[1]))
    if not entries:
        msg = f"{_CONTRACT.name} names no system params"
        raise SystemExit(msg)
    return entries


def parse_fields(source: str, path: str) -> list[Field]:
    """Every field of every top-level message in one proto file.

    Enum values inside the enum-wrapper messages carry no type token, so the
    field pattern skips them; a `oneof` block only shifts brace depth, and its
    arms stay attributed to the enclosing message.
    """
    fields: list[Field] = []
    message: str | None = None
    depth = 0

    for number, line in enumerate(source.splitlines(), start=1):
        opened = _MESSAGE.match(line)
        if opened:
            message = opened.group(1)
            depth = line.count("{") - line.count("}")
            # `message Foo {}` opens and closes on one line and holds nothing.
            if depth <= 0:
                message = None
            continue
        if message is None:
            continue

        found = _FIELD.match(line)
        if found:
            field_type, name, trailer = found.groups()
            fields.append(
                Field(
                    path=path,
                    line=number,
                    message=message,
                    name=name,
                    type=field_type,
                    marked=trailer.strip() == MARKER,
                )
            )

        depth += line.count("{") - line.count("}")
        if depth <= 0:
            message = None

    return fields


def all_fields() -> list[Field]:
    """Every field of every message under proto/linode/mcp/v1/, in file order."""
    fields: list[Field] = []
    for path in sorted(_PROTO_DIR.glob("*.proto")):
        display = path.relative_to(_REPO_ROOT).as_posix()
        fields.extend(parse_fields(path.read_text(encoding="utf-8"), display))
    return fields


def violations(
    fields: list[Field], entries: set[tuple[str, str]]
) -> tuple[list[Field], list[Field]]:
    """(system params missing the marker, fields marked that should not be)."""
    missing: list[Field] = []
    unexpected: list[Field] = []
    for field in fields:
        wanted = field.message.endswith("Input") and (field.name, field.type) in entries
        if wanted and not field.marked:
            missing.append(field)
        elif field.marked and not wanted:
            unexpected.append(field)
    return missing, unexpected


def _report(missing: list[Field], unexpected: list[Field]) -> None:
    """Print each violation with its file:line and the fix for that direction."""
    if missing:
        print("system params missing the trailing marker:", file=sys.stderr)
        for field in missing:
            print(f"  {field.where()}", file=sys.stderr)
        print(
            f"  (append ` {MARKER}` to the field line, after the semicolon;"
            " never add it to the leading comment block)",
            file=sys.stderr,
        )
    if unexpected:
        print(f"fields marked `{MARKER}` that are not system params:", file=sys.stderr)
        for field in unexpected:
            print(f"  {field.where()} ({field.type})", file=sys.stderr)
        print(
            "  (drop the marker, or add the field's name and type to"
            " docs/contracts/system-params.txt; a marker outside an *Input"
            " message is always wrong)",
            file=sys.stderr,
        )


def main() -> int:
    entries = contract_entries()
    fields = all_fields()
    missing, unexpected = violations(fields, entries)

    if missing or unexpected:
        _report(missing, unexpected)
        return 1

    marked = sum(1 for field in fields if field.marked)
    print(
        f"system-params gate OK: {marked} marked field(s)"
        f" across {len(entries)} contracted param(s)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
