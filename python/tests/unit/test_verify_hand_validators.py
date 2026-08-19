"""The hand-validator ratchet counts every non-CEL check, wherever it lives.

Hook functions and validators still inside hand-written handlers are one
population: extracting a handler validator into a hook must hold the count
flat, or migration reads as growth and the gate blocks the work it measures.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from types import ModuleType

SCRIPTS_DIR = Path(__file__).resolve().parents[3] / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_hand_validators")


def _write(root: Path, relative: str, text: str) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _counted(tmp_path: Path, hook_src: str, hand_src: str) -> int:
    """The go count over a fabricated pair of trees."""
    _write(tmp_path, "go/internal/toolhooks/h.go", hook_src)
    _write(tmp_path, "go/internal/tools/t.go", hand_src)
    hooks = gate.Tree(
        root="go/internal/toolhooks",
        suffix=".go",
        definition=gate._TREES["go"].definition,
    )
    hand = gate.Tree(
        root="go/internal/tools",
        suffix=".go",
        definition=gate._HAND_TREES["go"].definition,
    )
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        return len(gate.implemented(hooks)) + len(gate.hand_written(hand))
    finally:
        swap._REPO_ROOT = original


def _python_hand_count(tmp_path: Path, files: dict[str, str]) -> int:
    """The python handler-tree count over a fabricated tree."""
    for relative, text in files.items():
        _write(tmp_path, f"python/src/linodemcp/tools/{relative}", text)
    tree = gate.Tree(
        root="python/src/linodemcp/tools",
        suffix=".py",
        definition=gate._HAND_TREES["python"].definition,
    )
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        return len(gate.hand_written(tree))
    finally:
        swap._REPO_ROOT = original


def test_both_populations_are_counted(tmp_path: Path) -> None:
    """A hook function and a handler validator are one check each."""
    count = _counted(
        tmp_path,
        'func LinodeDomainGetValidate(a map[string]any) string { return "" }\n',
        'func validateDomainCreate(a map[string]any) string { return "" }\n',
    )

    assert count == 2


def test_extraction_is_count_neutral(tmp_path: Path) -> None:
    """Moving a validator from a handler into a hook holds the total flat."""
    before = _counted(
        tmp_path,
        "",
        'func validateDomainCreate(a map[string]any) string { return "" }\n',
    )
    extracted = _counted(
        tmp_path,
        'func LinodeDomainCreateValidate(a map[string]any) string { return "" }\n',
        "",
    )

    assert before == 1
    assert extracted == 1


def test_test_files_are_not_counted(tmp_path: Path) -> None:
    """A table test naming hook shapes is not a second implementation."""
    _write(
        tmp_path,
        "go/internal/tools/t_test.go",
        'func validatePhantom(a map[string]any) string { return "" }\n',
    )
    count = _counted(tmp_path, "", "")

    assert count == 0


def test_parse_and_validate_helpers_count(tmp_path: Path) -> None:
    """Every spelling python gives an argument check is one check."""
    count = _python_hand_count(
        tmp_path,
        {
            "a.py": (
                "def _parse_instance_id(arguments: dict) -> int:\n    return 1\n\n\n"
                "def _validate_key_label(label: str) -> str | None:\n"
                "    return None\n\n\n"
                "def _disk_create_fields_error(arguments: dict) -> str | None:\n"
                "    return None\n\n\n"
                "def _firewall_ids_argument(arguments: dict) -> list | None:\n"
                "    return None\n"
            )
        },
    )

    assert count == 4


def test_response_decoders_are_not_argument_checks(tmp_path: Path) -> None:
    """A helper that shapes a response body is not a hand-written check."""
    count = _python_hand_count(
        tmp_path,
        {
            "a.py": (
                "def _error_response(message: str) -> list:\n    return []\n\n\n"
                "def _reserved_ip_response(payload: dict) -> dict:\n"
                "    return payload\n"
            )
        },
    )

    assert count == 0


def test_repeated_copies_each_count(tmp_path: Path) -> None:
    """Two files spelling out the same check are two copies of it."""
    body = "def _parse_instance_id(arguments: dict) -> int:\n    return 1\n"
    count = _python_hand_count(tmp_path, {"a.py": body, "b.py": body})

    assert count == 2


def test_go_parse_and_validate_helpers_count(tmp_path: Path) -> None:
    """Go's handler tree spells the same check three ways, and all count."""
    count = _counted(
        tmp_path,
        "",
        'func validateDomainCreate(a map[string]any) string { return "" }\n\n'
        'func parseConfigDevices(raw any) (any, string) { return nil, "" }\n\n'
        "func domainIDFromTool(r *mcp.CallToolRequest) (int, string) "
        '{ return 0, "" }\n',
    )

    assert count == 3


def test_go_handlers_are_not_argument_checks(tmp_path: Path) -> None:
    """Ordinary handler and client functions are not hand-written checks."""
    count = _counted(
        tmp_path,
        "",
        "func handleLinodeDomainCreateRequest(a map[string]any) error "
        "{ return nil }\n\n"
        "func httpCreateDomain(a map[string]any) error { return nil }\n\n"
        "func buildDefaultRoute(ipv4, ipv6 bool) *Route { return nil }\n",
    )

    assert count == 0


def test_direction_failures_name_both_sides() -> None:
    """Above and below the recorded line both fail, naming the movement."""
    above = gate.compare({"go": 5}, {"go": 4})
    assert above
    assert "go" in above[0]

    below = gate.compare({"go": 3}, {"go": 4})
    assert below
    assert "go" in below[0]

    assert gate.compare({"go": 4}, {"go": 4}) == []


def test_the_real_trees_match_the_recorded_counts() -> None:
    """The shipped contract states what the shipped source measures."""
    languages = gate.registered_languages(
        gate._REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )
    counts, problems = gate.measure(languages)

    assert problems == []
    assert counts == gate.read_counts(gate._COUNTS)


def test_plumbing_names_are_not_counted(tmp_path: Path) -> None:
    """A name in the plumbing set is support vocabulary, not a per-tool check.

    The set is what lets one shared reader be counted in neither language
    instead of in one. Everything beside it in the same file still counts, which
    is the half worth pinning: the exemption is by name, not by file.
    """
    _write(
        tmp_path,
        "go/internal/tools/t.go",
        "func standardPaginationFromTool(r any) (int, int, string)"
        ' { return 0, 0, "" }\n\n'
        "func parseOptionalTime(v string) (any, error) { return nil, nil }\n\n"
        'func validateDomainCreate(a map[string]any) string { return "" }\n',
    )
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        found = gate.hand_written(gate._HAND_TREES["go"])
    finally:
        swap._REPO_ROOT = original

    assert [name.rsplit(":", maxsplit=1)[-1] for name in found] == [
        "validateDomainCreate"
    ]


def test_python_plumbing_name_is_not_counted(tmp_path: Path) -> None:
    """Python's shared segment reader is exempt under its natural name.

    It was spelled away from `_..._argument` to dodge this scan; the plumbing
    set is what makes the natural name safe again.
    """
    _write(
        tmp_path,
        "python/src/linodemcp/tools/a.py",
        "def _segment_argument(arguments: dict, name: str) -> tuple:\n"
        '    return "", ""\n\n\n'
        "def _firewall_ids_argument(arguments: dict) -> list | None:\n"
        "    return None\n",
    )
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        found = gate.hand_written(gate._HAND_TREES["python"])
    finally:
        swap._REPO_ROOT = original

    assert [name.rsplit(":", maxsplit=1)[-1] for name in found] == [
        "_firewall_ids_argument"
    ]


def test_every_plumbing_name_exists_in_its_tree() -> None:
    """A plumbing entry naming nothing is an exemption with nothing to exempt.

    Without this the set could keep a name long after its function went, and
    the next function to be given that name would be exempt by accident.
    """
    for language, tree in gate._HAND_TREES.items():
        if not tree.plumbing:
            continue
        root = gate._REPO_ROOT / tree.root
        defined: set[str] = set()
        for path in sorted(root.rglob("*" + tree.suffix)):
            if gate.is_test_path(path.relative_to(root)):
                continue
            defined.update(tree.definition.findall(path.read_text(encoding="utf-8")))

        assert tree.plumbing <= defined, f"{language} plumbing names nothing: " + str(
            sorted(tree.plumbing - defined)
        )


def test_a_new_hook_raises_the_count(tmp_path: Path) -> None:
    """The gate still fails loudly when surface writes a check by hand.

    Zero is only worth reaching if it cannot be passed by a tree that grew one
    back, so this drives the same comparison `make check` runs.
    """
    grown = _counted(
        tmp_path,
        'func LinodeDomainGetValidate(a map[string]any) string { return "" }\n',
        "",
    )

    assert grown == 1
    assert gate.compare({"go": grown}, {"go": 0})
