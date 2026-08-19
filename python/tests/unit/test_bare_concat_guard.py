"""Guard: a pinned prose sentence must be one literal, not a built string.

A prose sweep rewrites declared sentences by reading them out of the sources,
so a pin built as ``f"Failed to retrieve {tool}"`` is invisible to it. Three
monitor pins survived a 65-site sweep exactly that way, and the tests kept
passing because the built string still evaluated to the old sentence.

The Go twin is TestNoTestPinsADeclaredSentenceByConcatenation in
go/internal/tools/bare_concat_guard_test.go and reads the same shape out of the
Go test tree.
"""

from __future__ import annotations

import ast
from pathlib import Path

_TEST_TREE = Path(__file__).resolve().parents[1]

# The fewest space-separated words a literal needs before it reads as a
# sentence rather than as a token being assembled: "Bearer " builds a header
# value, "Failed to retrieve " builds a sentence a caller sees.
_PROSE_WORDS = 2


def _opens_a_sentence(text: str) -> bool:
    """Whether a literal opens prose that a value is about to be joined onto."""
    if not text.endswith(" "):
        return False
    stripped = text.lstrip()
    if not stripped or not stripped[0].isalnum():
        return False
    return len([word for word in text.split(" ") if word]) >= _PROSE_WORDS


def _prose_join(node: ast.AST) -> str | None:
    """The opening literal of a prose string built by concat or f-string."""
    if (
        isinstance(node, ast.BinOp)
        and isinstance(node.op, ast.Add)
        and isinstance(node.left, ast.Constant)
        and isinstance(node.left.value, str)
        and _opens_a_sentence(node.left.value)
    ):
        return node.left.value
    if (
        isinstance(node, ast.JoinedStr)
        and len(node.values) > 1
        and isinstance(node.values[0], ast.Constant)
        and isinstance(node.values[0].value, str)
        and _opens_a_sentence(node.values[0].value)
    ):
        return node.values[0].value
    return None


def _assert_message_nodes(tree: ast.AST) -> set[int]:
    """Nodes inside an assert's message, which is printed rather than pinned.

    A sweep never has to find the wording a failing test reports, so building
    one is fine there.
    """
    inside: set[int] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Assert) and node.msg is not None:
            inside.update(id(child) for child in ast.walk(node.msg))
    return inside


def _concat_pins(path: Path) -> list[str]:
    """Every built prose sentence one test file compares against.

    Only comparison operands count. A built string handed to a call or written
    into a fixture file is test input rather than a pin, and the sweep has no
    sentence to find in it.
    """
    tree = ast.parse(path.read_text(encoding="utf-8"), str(path))
    printed = _assert_message_nodes(tree)
    found: list[str] = []

    for node in ast.walk(tree):
        if not isinstance(node, ast.Compare):
            continue
        for side in [node.left, *node.comparators]:
            for inner in ast.walk(side):
                if id(inner) in printed or not isinstance(inner, ast.expr):
                    continue
                if _prose_join(inner) is not None:
                    found.append(f"{path}:{inner.lineno}")

    return found


def test_no_test_pins_a_declared_sentence_by_concatenation() -> None:
    """Every pinned prose sentence is spelled as a single literal."""
    scanned = 0
    found: list[str] = []

    for path in sorted(_TEST_TREE.rglob("*.py")):
        scanned += 1
        found.extend(_concat_pins(path))

    # Non-vacuity: a scan that read nothing would pass while proving nothing,
    # which is the failure mode this guard exists to close.
    assert scanned > 0, f"scanned no test files under {_TEST_TREE}"

    assert not found, (
        "these pin a sentence by building it; spell each as one literal:\n"
        + "\n".join(found)
    )
