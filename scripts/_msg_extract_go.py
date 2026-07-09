#!/usr/bin/env python3
"""Extract a tool_name -> confirm-message map from the Go tools package.

Heuristic: within each top-level Go func, collect the linode_* tool-name
literals and the confirm message (RequireConfirm arg, requireDestroyConfirmation
4th arg, or ConfirmMessage: field). Pair them when a func has exactly one
distinct tool name and one message. Tool names given as consts are resolved from
a first pass over `<Const> = "linode_..."` (with optional string concatenation).
"""

import json
import re
import sys
from pathlib import Path

TOOLS_DIR = Path(sys.argv[1])
MANIFEST = Path(sys.argv[2]) if len(sys.argv) > 2 else None
VALID_TOOLS: set[str] = set()
if MANIFEST:
    for _ln in MANIFEST.read_text().split("\n"):
        _ln = _ln.strip()
        if _ln and not _ln.startswith("#"):
            VALID_TOOLS.add(_ln)

TOOL_LIT = re.compile(r'"(linode_[a-z0-9_]+)"')
# const definitions like: name = "linode_foo"  OR  name = "linode_" + "foo"
CONST_DEF = re.compile(
    r"^\s*(\w+)\s*=\s*(\"linode_[a-z0-9_]*\"(?:\s*\+\s*\"[a-z0-9_]*\")*)"
)
STR = r'"((?:[^"\\]|\\.)*)"'
MSG = re.compile(STR)
CONFIRM_MSG_FIELD = re.compile(r"ConfirmMessage:\s*" + STR)
REQUIRE_CONFIRM = re.compile(r"RequireConfirm\(request,\s*" + STR)
REQUIRE_DESTROY = re.compile(
    r"requireDestroyConfirmation\([^,]*,\s*[^,]*,\s*([^,]+),\s*" + STR
)
TOOLNAME_FIELD = re.compile(r"ToolName:\s*([^,]+),")
DESTROY_TOOLNAME_ARG = None


def resolve_const_value(expr: str) -> str:
    parts = re.findall(r'"([a-z0-9_]*)"', expr)
    return "".join(parts)


def main() -> int:
    consts: dict[str, str] = {}
    files = sorted(TOOLS_DIR.glob("linode_*.go"))
    files = [f for f in files if not f.name.endswith("_test.go")]

    # pass 1: const map
    for f in files:
        for line in f.read_text().split("\n"):
            m = CONST_DEF.match(line)
            if m:
                consts[m.group(1)] = resolve_const_value(m.group(2))

    def resolve_toolname(expr: str) -> str | None:
        expr = expr.strip()
        m = re.match(r'"(linode_[a-z0-9_]+)"', expr)
        if m:
            return m.group(1)
        if expr in consts:
            return consts[expr]
        return None

    result: dict[str, str] = {}
    conflicts: list[str] = []

    for f in files:
        lines = f.read_text().split("\n")
        i = 0
        n = len(lines)
        while i < n:
            line = lines[i]
            fm = re.match(r"^func (?:\([^)]*\)\s*)?(\w+)\(", line)
            if not fm:
                i += 1
                continue
            # collect the whole func body by brace depth
            body = [line]
            depth = line.count("{") - line.count("}")
            j = i
            while (depth > 0 or j == i) and j + 1 < n:
                j += 1
                body.append(lines[j])
                depth += lines[j].count("{") - lines[j].count("}")
                if depth <= 0 and j > i:
                    break
            text = "\n".join(body)

            tool_names: set[str] = set()
            for tm in TOOL_LIT.finditer(text):
                if not VALID_TOOLS or tm.group(1) in VALID_TOOLS:
                    tool_names.add(tm.group(1))
            for tf in TOOLNAME_FIELD.finditer(text):
                rn = resolve_toolname(tf.group(1))
                if rn and (not VALID_TOOLS or rn in VALID_TOOLS):
                    tool_names.add(rn)

            messages: list[str] = [rc.group(1) for rc in REQUIRE_CONFIRM.finditer(text)]
            messages.extend(cf.group(1) for cf in CONFIRM_MSG_FIELD.finditer(text))
            destroy_pairs: list[tuple[str | None, str]] = [
                (resolve_toolname(rd.group(1)), rd.group(2))
                for rd in REQUIRE_DESTROY.finditer(text)
            ]

            # direct destroy pairs: toolname + message both in the call
            for tn, msg in destroy_pairs:
                if tn:
                    if tn in result and result[tn] != msg:
                        conflicts.append(f"{tn}: {result[tn]!r} vs {msg!r}")
                    result[tn] = msg
                elif len(tool_names) == 1:
                    tn2 = next(iter(tool_names))
                    result[tn2] = msg

            # RequireConfirm / ConfirmMessage: pair with the sole tool name
            if messages:
                if len(tool_names) == 1 and len(set(messages)) == 1:
                    tn = next(iter(tool_names))
                    msg = messages[0]
                    if tn in result and result[tn] != msg:
                        conflicts.append(f"{tn}: {result[tn]!r} vs {msg!r}")
                    result[tn] = msg
                elif len(tool_names) == len(messages) and len(tool_names) > 1:
                    # ambiguous multi in one func; report
                    conflicts.append(
                        f"{f.name}:{fm.group(1)} MULTI "
                        f"names={sorted(tool_names)} msgs={messages}"
                    )
                elif len(tool_names) == 0:
                    pass  # helper, caller supplies
                else:
                    conflicts.append(
                        f"{f.name}:{fm.group(1)} UNPAIRED "
                        f"names={sorted(tool_names)} msgs={messages}"
                    )

            i = j + 1

    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
