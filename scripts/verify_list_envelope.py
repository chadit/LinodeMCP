#!/usr/bin/env python3
"""List-envelope guard: no `or []` in a Python function that builds a list response.

A handful of Linode list routes name their own member instead of returning the
{data, page, pages, results} page envelope (/linode/instances/{id}/interfaces,
/profile/security-questions). Unwrapping one with `raw.get(key) or []` looks
like a null guard, but `or` fires on every falsey value: `{}`, `""`, `0`, and
`false` all become an empty list. Go rejects each of those through
listProtoElementsKeyed, and so does linodego decoding the same routes into a
[]T struct field, so the collapse ships a malformed API response to the caller
as a successful empty list in one language only.

The rule is narrow on purpose: `or []` is flagged only inside a function that
itself calls serialize_list_response or serialize_keyed_list_response. The same
idiom in a per-element shaping helper (linode_domains, linode_object_storage)
coerces a documented-nullable field and is untouched.

Use serialize_keyed_list_response, which checks the root object and hands the
member to the array and element checks unexamined.

What gets scanned comes from docs/contracts/languages.txt, not from a path
written here: each registered language is scanned under its own working dir.
COVERAGE says how, holding either a scanner for that language or the reason
the idiom cannot occur in it. A language registered without an entry there
fails the gate by name, so registering one forces the decision instead of
silently leaving the surface unguarded.

Known gaps live in docs/contracts/list-envelope-baseline.txt, a ratchet: fix the
handler and remove its line; never add a line by hand (regenerate with
--update-baseline, then attach the required acceptance annotation).

Stdlib only, so no venv is needed. Run via `make list-envelope` (in `make
check`, and so the pre-push hook and the CI gate on every branch).

Usage: verify_list_envelope.py [--update-baseline]
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import _baselines

if TYPE_CHECKING:
    from collections.abc import Callable

    # A language scanner takes that language's working dir and returns its
    # violation entries, each prefixed with the repo-relative file path.
    Scanner = Callable[[Path], list[str]]

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "list-envelope-baseline.txt"

LIST_SERIALIZERS = frozenset(
    {"serialize_list_response", "serialize_keyed_list_response"}
)

_BASELINE_HEADER = (
    "# Functions that build a list response and still collapse falsey values\n"
    "# with `or []`, hiding a malformed API response as an empty list. Ratchet:\n"
    "# switch the handler to serialize_keyed_list_response and remove its line;\n"
    "# never add a line by hand (regenerate instead, then attach the required\n"
    "# annotation).\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_list_envelope.py --update-baseline\n"
)

_FUNCTION_NODES = (ast.FunctionDef, ast.AsyncFunctionDef)


def _own_nodes(func: ast.FunctionDef | ast.AsyncFunctionDef) -> list[ast.AST]:
    """Every node in func's body except those inside a nested function.

    A handler and its inner _call are separate functions here, so a finding
    names the function that actually holds both the serializer call and the
    collapse rather than the outermost def.
    """
    own: list[ast.AST] = []
    stack: list[ast.AST] = list(func.body)
    while stack:
        node = stack.pop()
        own.append(node)
        # Stop at a nested def wherever it sits, including inside an if or a
        # with block, so its body belongs to it and not to the function here.
        if isinstance(node, _FUNCTION_NODES):
            continue
        stack.extend(ast.iter_child_nodes(node))
    return own


def _is_empty_list(node: ast.AST) -> bool:
    """True for the `[]` literal, the operand that makes `or` a silent default."""
    return isinstance(node, ast.List) and not node.elts


def _calls_list_serializer(nodes: list[ast.AST]) -> bool:
    """True when one of these nodes calls a list-response serializer by name."""
    return any(
        isinstance(node, ast.Call)
        and isinstance(node.func, ast.Name)
        and node.func.id in LIST_SERIALIZERS
        for node in nodes
    )


def _collapses_to_empty_list(nodes: list[ast.AST]) -> bool:
    """True when one of these nodes is an `X or []` expression."""
    return any(
        isinstance(node, ast.BoolOp)
        and isinstance(node.op, ast.Or)
        and _is_empty_list(node.values[-1])
        for node in nodes
    )


def module_violations(source: str, module: str) -> list[str]:
    """One entry per function in source that builds a list response with `or []`."""
    found: list[str] = []

    def visit(node: ast.AST, prefix: str) -> None:
        for child in ast.iter_child_nodes(node):
            if isinstance(child, _FUNCTION_NODES):
                qualname = f"{prefix}.{child.name}" if prefix else child.name
                nodes = _own_nodes(child)
                if _calls_list_serializer(nodes) and _collapses_to_empty_list(nodes):
                    found.append(f"{module}:{qualname}")
                visit(child, qualname)
            else:
                visit(child, prefix)

    visit(ast.parse(source), "")
    return found


def _display_path(path: Path, root: Path) -> str:
    """Repo-relative when the tree sits in the repo, root-relative otherwise.

    A registry working dir is normally inside the repo, so entries read as
    python/src/... . The fallback keeps the scanner usable against any tree
    (a test fixture, a checkout mounted elsewhere) instead of raising.
    """
    try:
        return path.relative_to(_REPO_ROOT).as_posix()
    except ValueError:
        return path.relative_to(root).as_posix()


def python_violations(root: Path) -> list[str]:
    """Scan every .py under root, skipping dot-directories (venvs, tool caches)."""
    violations: list[str] = []
    for path in sorted(root.rglob("*.py")):
        relative = path.relative_to(root)
        if any(part.startswith(".") for part in relative.parts):
            continue
        violations.extend(
            module_violations(
                path.read_text(encoding="utf-8"), _display_path(path, root)
            )
        )
    return violations


# How each registered language is covered: a scanner, or the reason the idiom
# cannot occur there. Go's entry is a claim the reviewer can check, not a
# blanket exemption.
COVERAGE: dict[str, Scanner | str] = {
    "python": python_violations,
    "go": (
        "`or []` has no Go equivalent, and both keyed routes decode through the "
        "single listProtoElementsKeyed helper rather than unwrapping per handler"
    ),
}


def registered_languages(path: Path) -> list[tuple[str, Path]]:
    """(name, working dir) per registry line, in file order."""
    languages: list[tuple[str, Path]] = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = [field.strip() for field in line.split("\t") if field.strip()]
        if len(fields) < 2:
            msg = f"unparsable {path.name} line {raw!r}"
            raise SystemExit(msg)
        languages.append((fields[0], _REPO_ROOT / fields[1]))
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)
    return languages


def undeclared_languages(languages: list[tuple[str, Path]]) -> list[str]:
    """Registered languages with neither a scanner nor a stated exemption."""
    return sorted(name for name, _ in languages if name not in COVERAGE)


def current_violations() -> list[str]:
    """Every list-building function that collapses with `or []`, across languages."""
    violations: list[str] = []
    for name, workdir in registered_languages(_LANGUAGES):
        scanner = COVERAGE.get(name)
        # A str entry is the stated reason the idiom cannot occur in that
        # language; only a callable entry has a tree to scan.
        if scanner is None or isinstance(scanner, str):
            continue
        violations.extend(scanner(workdir))
    return sorted(violations)


def main(argv: list[str]) -> int:
    languages = registered_languages(_LANGUAGES)
    undeclared = undeclared_languages(languages)
    if undeclared:
        print(
            "no list-envelope coverage declared for registered language(s): "
            f"{', '.join(undeclared)}. Add a scanner to COVERAGE in "
            "scripts/verify_list_envelope.py, or the reason the idiom cannot "
            "occur in that language.",
            file=sys.stderr,
        )
        return 1

    violations = current_violations()

    if "--update-baseline" in argv:
        _baselines.write_baseline(
            _BASELINE, _BASELINE_HEADER, violations, _baselines.read_baseline(_BASELINE)
        )
        print(f"wrote {len(violations)} list-envelope gap(s)", file=sys.stderr)
        return 0

    baseline = _baselines.read_entries(_BASELINE)
    new = [entry for entry in violations if entry not in baseline]
    fixed = sorted(baseline - set(violations))

    if new:
        print("list responses built from a falsey-collapsed member:", file=sys.stderr)
        for entry in new:
            print(f"  {entry}", file=sys.stderr)
        print(
            '\n`or []` turns {}, "", 0, and false into an empty list; Go and'
            " linodego reject all four. Unwrap the member with"
            " serialize_keyed_list_response instead.",
            file=sys.stderr,
        )
    if fixed:
        print(
            "list-envelope baseline entries are fixed; remove them: "
            f"{', '.join(fixed)}",
            file=sys.stderr,
        )

    if new or fixed:
        return 1

    print(f"list-envelope guard OK: {len(violations)} accepted gap(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
