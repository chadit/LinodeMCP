#!/usr/bin/env python3
"""Offline gate: the CLI surface matches across languages.

The MCP tool surface has the manifest and tool-parity gates; the CLI had
nothing, and help wording aside, nothing stopped a verb or flag landing in
one binary only. This gate extracts, from source:

- the top-level verb set (Go: the dispatch switch in cmd/linodemcp/main.go;
  Python: the _CLI_SUBCOMMANDS frozenset in main.py, plus "serve", which
  falls through to the server runtime by design),
- the `call` flag set (Go: flag registrations incl. bindOptionalBool;
  Python: argparse add_argument), where the safety semantics live,
- the `tools` flag set and its `show` subverb,
- the `audit` subverb and flag sets (Go: the dispatch switch and flag
  registrations in audit_cmd.go; Python: the _AUDIT_TOOLS map and
  argparse add_argument in cli/audit.py),
- the `profile` subverb and flag sets (Go: the dispatch switch in
  profile.go; Python: the handler-map keys in cli/profile.py). Profile
  takes only positionals today, so its flag comparison accepts an
  empty-on-both-sides extraction instead of treating it as pattern rot.

and fails on any per-language difference.

Stdlib only, so no venv is needed. Run via `make cli-surface` (in `make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parent.parent

_GO_MAIN = _REPO_ROOT / "go" / "cmd" / "linodemcp" / "main.go"
_GO_CALL = _REPO_ROOT / "go" / "internal" / "cli" / "call.go"
_GO_TOOLS = _REPO_ROOT / "go" / "internal" / "cli" / "tools_cmd.go"
_GO_AUDIT = _REPO_ROOT / "go" / "internal" / "cli" / "audit_cmd.go"
_GO_PROFILE = _REPO_ROOT / "go" / "internal" / "cli" / "profile.go"
_PY_MAIN = _REPO_ROOT / "python" / "src" / "linodemcp" / "main.py"
_PY_CALL = _REPO_ROOT / "python" / "src" / "linodemcp" / "cli" / "call.py"
_PY_TOOLS = _REPO_ROOT / "python" / "src" / "linodemcp" / "cli" / "tools.py"
_PY_AUDIT = _REPO_ROOT / "python" / "src" / "linodemcp" / "cli" / "audit.py"
_PY_PROFILE = _REPO_ROOT / "python" / "src" / "linodemcp" / "cli" / "profile.py"

_GO_CASE = re.compile(r'^\tcase "([a-z]+)":', re.MULTILINE)
_PY_SUBCOMMANDS = re.compile(r"_CLI_SUBCOMMANDS = frozenset\(\{(.*?)\}\)", re.DOTALL)
_QUOTED_NAME = re.compile(r'"([a-z][a-z0-9-]*)"')
_GO_FLAG_REGISTRATIONS = (
    re.compile(r'\.StringVar\(&[^,]+,\s*"([a-z][a-z0-9-]*)"'),
    # Int/Bool registrations exist for the audit surface (--limit,
    # --include-meta); call.go itself never uses them.
    re.compile(r'\.IntVar\(&[^,]+,\s*"([a-z][a-z0-9-]*)"'),
    re.compile(r'\.BoolVar\(&[^,]+,\s*"([a-z][a-z0-9-]*)"'),
    re.compile(r'\.Var\(&[^,]+,\s*"([a-z][a-z0-9-]*)"'),
    re.compile(r'bindOptionalBool\(flags,\s*"([a-z][a-z0-9-]*)"'),
)
_PY_ADD_ARGUMENT = re.compile(r'add_argument\(\s*"--([a-z][a-z0-9-]*)"')
_DOUBLE_DASH = re.compile(r'"--([a-z][a-z0-9-]*)"')
_PY_AUDIT_TOOLS = re.compile(r"_AUDIT_TOOLS = \{(.*?)\}", re.DOTALL)
_PY_DICT_KEY = re.compile(r'"([a-z][a-z0-9-]*)"\s*:')
_PY_PROFILE_HANDLER = re.compile(r'"([a-z][a-z0-9-]*)": run_profile_')


def go_verbs() -> set[str]:
    """Verbs the Go dispatch switch routes (serve included explicitly)."""
    return set(_GO_CASE.findall(_GO_MAIN.read_text(encoding="utf-8")))


def python_verbs() -> set[str]:
    """Python's _CLI_SUBCOMMANDS plus serve, which falls through by design."""
    match = _PY_SUBCOMMANDS.search(_PY_MAIN.read_text(encoding="utf-8"))
    if match is None:
        return set()
    return set(_QUOTED_NAME.findall(match.group(1))) | {"serve"}


def go_call_flags() -> set[str]:
    """Flags call.go registers on its flag set, bindOptionalBool included."""
    text = _GO_CALL.read_text(encoding="utf-8")
    found: set[str] = set()
    for pattern in _GO_FLAG_REGISTRATIONS:
        found.update(pattern.findall(text))
    return found


def python_call_flags() -> set[str]:
    """Flags call.py's argparse parser declares (positionals excluded)."""
    return set(_PY_ADD_ARGUMENT.findall(_PY_CALL.read_text(encoding="utf-8")))


def go_tools_surface() -> tuple[set[str], bool]:
    """(tools flags, has-show-subverb) for the Go tools command."""
    text = _GO_TOOLS.read_text(encoding="utf-8")
    return set(_DOUBLE_DASH.findall(text)), '"show"' in text


def python_tools_surface() -> tuple[set[str], bool]:
    text = _PY_TOOLS.read_text(encoding="utf-8")
    return set(_DOUBLE_DASH.findall(text)), '"show"' in text


def go_audit_surface() -> tuple[set[str], set[str]]:
    """(subverbs, flags) from the Go audit dispatch switch and flag sets."""
    text = _GO_AUDIT.read_text(encoding="utf-8")
    flags: set[str] = set()
    for pattern in _GO_FLAG_REGISTRATIONS:
        flags.update(pattern.findall(text))
    return set(_GO_CASE.findall(text)), flags


def python_audit_surface() -> tuple[set[str], set[str]]:
    """(subverbs, flags) from the _AUDIT_TOOLS keys and argparse flags."""
    text = _PY_AUDIT.read_text(encoding="utf-8")
    match = _PY_AUDIT_TOOLS.search(text)
    subverbs = set(_PY_DICT_KEY.findall(match.group(1))) if match else set()
    return subverbs, set(_PY_ADD_ARGUMENT.findall(text))


def go_profile_surface() -> tuple[set[str], set[str]]:
    """(subverbs, flags) from the Go profile dispatch switch and flag sets."""
    text = _GO_PROFILE.read_text(encoding="utf-8")
    flags: set[str] = set()
    for pattern in _GO_FLAG_REGISTRATIONS:
        flags.update(pattern.findall(text))
    return set(_GO_CASE.findall(text)), flags


def python_profile_surface() -> tuple[set[str], set[str]]:
    """(subverbs, flags) from the handler-map keys and argparse flags."""
    text = _PY_PROFILE.read_text(encoding="utf-8")
    subverbs = set(_PY_PROFILE_HANDLER.findall(text))
    return subverbs, set(_PY_ADD_ARGUMENT.findall(text))


def cli_violations() -> list[str]:
    """Human-readable differences between the two CLI surfaces."""
    problems: list[str] = []

    go_audit = go_audit_surface()
    python_audit = python_audit_surface()
    go_profile = go_profile_surface()
    python_profile = python_profile_surface()

    # allow_empty marks surfaces where an empty extraction is a real
    # state rather than pattern rot: profile takes only positionals
    # today, so both sides legitimately register zero flags.
    surfaces = [
        ("verbs", go_verbs(), python_verbs(), False),
        ("call flags", go_call_flags(), python_call_flags(), False),
        ("tools flags", go_tools_surface()[0], python_tools_surface()[0], False),
        ("audit subverbs", go_audit[0], python_audit[0], False),
        ("audit flags", go_audit[1], python_audit[1], False),
        ("profile subverbs", go_profile[0], python_profile[0], False),
        ("profile flags", go_profile[1], python_profile[1], True),
    ]
    for label, go_side, python_side, allow_empty in surfaces:
        if not allow_empty and (not go_side or not python_side):
            problems.append(
                f"{label}: extraction found nothing for"
                f" {'go' if not go_side else 'python'};"
                " the source shape changed, update this gate's patterns"
            )
            continue
        problems.extend(
            f"{label}: {name} exists in go only"
            for name in sorted(go_side - python_side)
        )
        problems.extend(
            f"{label}: {name} exists in python only"
            for name in sorted(python_side - go_side)
        )

    go_show = go_tools_surface()[1]
    python_show = python_tools_surface()[1]
    if go_show != python_show:
        problems.append(
            f"tools show subverb: present in {'go' if go_show else 'python'} only"
        )
    return problems


def main() -> int:
    problems = cli_violations()
    if problems:
        print("CLI surface diverges between languages:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        print(
            "  (implement the verb/flag in every language in the same"
            " change; the CLI is one surface, not per-language)",
            file=sys.stderr,
        )
        return 1

    verb_count = len(go_verbs())
    flag_count = len(go_call_flags())
    audit_count = len(go_audit_surface()[0])
    profile_count = len(go_profile_surface()[0])
    print(
        f"cli-surface gate OK: {verb_count} verbs, {flag_count} call flags,"
        f" {audit_count} audit subverbs, and {profile_count} profile"
        " subverbs match across languages"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
