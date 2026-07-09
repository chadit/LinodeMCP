#!/usr/bin/env python3
"""Extract tool -> confirm-gate message from Python handler files.

For each handle_<tool> function, find the first confirm-check line and capture
the message string(s) emitted by the following error return. Strips a leading
"Error: " so the result compares against Go's bare messages.
"""

import json
import re
import sys
from pathlib import Path

CONFIRM_CHECK = re.compile(r"\bconfirm\b")
NEG = ("is not True", "not confirm", "not arguments.get", "!= True", "is not true")
STRLIT = re.compile(r'"((?:[^"\\]|\\.)*)"')


def collect_message(lines: list[str], start: int) -> str | None:
    """From the confirm-check line at `start`, read the next return's strings."""
    n = len(lines)
    j = start + 1
    # skip to the first line that starts an error return or error assignment
    while j < n and j < start + 6:
        s = lines[j].strip()
        if (
            s.startswith(
                (
                    "return _error_response(",
                    "return error_response(",
                    "return [",
                    "error = (",
                    "error = ",
                )
            )
            or "TextContent(" in s
        ):
            break
        j += 1
    else:
        return None
    # gather string literals until the statement's closing paren/bracket
    buf: list[str] = []
    depth = 0
    k = j
    opened = False
    while k < n and k < j + 12:
        line = lines[k]
        depth += line.count("(") + line.count("[") - line.count(")") - line.count("]")
        for m in STRLIT.finditer(line):
            piece = m.group(1)
            if piece == "text":  # the TextContent(type="text", ...) field value
                continue
            buf.append(piece)
        if opened and depth <= 0:
            break
        if "(" in line or "[" in line:
            opened = True
        if not opened and (";" in line or line.strip().endswith(")")):
            break
        k += 1
    msg = "".join(buf)
    msg = msg.removeprefix("Error: ")
    return msg or None


def main() -> int:
    tools_dir = Path(sys.argv[1])
    result: dict[str, str] = {}
    for f in sorted(tools_dir.glob("linode_*.py")):
        lines = f.read_text().split("\n")
        cur = None
        i = 0
        n = len(lines)
        captured_for_cur = False
        while i < n:
            line = lines[i]
            hm = re.match(r"^(?:async )?def (handle_\w+)\(", line)
            if hm:
                cur = hm.group(1).replace("handle_", "", 1)
                captured_for_cur = False
                i += 1
                continue
            if (
                cur
                and not captured_for_cur
                and "confirm" in line
                and any(neg in line for neg in NEG)
                and line.strip().startswith("if ")
            ):
                msg = collect_message(lines, i)
                if msg:
                    result.setdefault(cur, msg)
                    captured_for_cur = True
            i += 1
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
