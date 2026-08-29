"""The hand-validator ratchet counts every non-CEL check, wherever it lives.

An argument check is named after what it CHECKS, so no tool name appears in it.
That is why this gate exists beside `make hand-code`, which fails by name on a
function named after a TOOL and cannot see this population at all. The last test
here holds the two gates against one planted check and pins that split, because
it is the only reason two contract files are worth keeping.
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


def _rooted(tmp_path: Path, tree: Any) -> list[str]:
    """The sites one tree yields with the repo root pointed at a fabrication."""
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        found: list[str] = gate.hand_written(tree)
    finally:
        swap._REPO_ROOT = original

    return found


def _go_count(tmp_path: Path, source: str) -> int:
    """The go count over a fabricated hand-written tree."""
    _write(tmp_path, "go/internal/tools/t.go", source)

    return len(_rooted(tmp_path, gate._TREES["go"]))


def _python_count(tmp_path: Path, files: dict[str, str]) -> int:
    """The python count over a fabricated hand-written tree."""
    for relative, text in files.items():
        _write(tmp_path, f"python/src/linodemcp/tools/{relative}", text)

    return len(_rooted(tmp_path, gate._TREES["python"]))


def test_a_hand_written_check_is_counted(tmp_path: Path) -> None:
    """One check written out in a handler is one unit of the debt."""
    count = _go_count(
        tmp_path, 'func validateDomainCreate(a map[string]any) string { return "" }\n'
    )

    assert count == 1


def test_test_files_are_not_counted(tmp_path: Path) -> None:
    """A table test naming a check shape is not a second implementation."""
    _write(
        tmp_path,
        "go/internal/tools/t_test.go",
        'func validatePhantom(a map[string]any) string { return "" }\n',
    )

    assert _go_count(tmp_path, "package tools\n") == 0


def test_parse_and_validate_helpers_count(tmp_path: Path) -> None:
    """Every spelling python gives an argument check is one check."""
    count = _python_count(
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
    count = _python_count(
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
    count = _python_count(tmp_path, {"a.py": body, "b.py": body})

    assert count == 2


def test_go_parse_and_validate_helpers_count(tmp_path: Path) -> None:
    """Go's hand-written tree spells the same check three ways, and all count."""
    count = _go_count(
        tmp_path,
        'func validateDomainCreate(a map[string]any) string { return "" }\n\n'
        'func parseConfigDevices(raw any) (any, string) { return nil, "" }\n\n'
        "func domainIDFromTool(r *mcp.CallToolRequest) (int, string) "
        '{ return 0, "" }\n',
    )

    assert count == 3


def test_go_handlers_are_not_argument_checks(tmp_path: Path) -> None:
    """Ordinary handler and client functions are not hand-written checks."""
    count = _go_count(
        tmp_path,
        "func handleLinodeDomainCreateRequest(a map[string]any) error "
        "{ return nil }\n\n"
        "func httpCreateDomain(a map[string]any) error { return nil }\n\n"
        "func buildDefaultRoute(ipv4, ipv6 bool) *Route { return nil }\n",
    )

    assert count == 0


def test_direction_failures_name_both_sides() -> None:
    """Above and below the recorded line both fail, naming the movement."""
    sites = {"go": ["go/internal/tools/t.go:validateDomainCreate"]}

    above = gate.compare({"go": 5}, {"go": 4}, sites)
    assert above
    assert "up from" in above[0]

    below = gate.compare({"go": 3}, {"go": 4}, sites)
    assert below
    assert "down from" in below[0]

    assert gate.compare({"go": 4}, {"go": 4}, sites) == []


def test_a_count_over_its_line_names_the_sites_that_pushed_it_there() -> None:
    """A bare number does not say which check to move onto a message."""
    sites = {"go": ["go/internal/tools/t.go:validateDomainCreate"]}

    problems = gate.compare({"go": 1}, {"go": 0}, sites)

    assert problems
    assert "go/internal/tools/t.go:validateDomainCreate" in problems[0]


def test_the_real_trees_match_the_recorded_counts() -> None:
    """The shipped contract states what the shipped source measures."""
    languages = gate.registered_languages(
        gate._REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )
    sites: dict[str, list[str]] = {}
    counts, problems = gate.measure(languages, sites)

    assert problems == []
    assert counts == gate.read_counts(gate._COUNTS)


def test_a_tree_that_holds_no_source_file_is_reported(tmp_path: Path) -> None:
    """At zero, an empty scan and a clean tree read the same without this."""
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        _, problems = gate.measure(["go"], {})
    finally:
        swap._REPO_ROOT = original

    assert any("no source file found under" in problem for problem in problems)


def test_a_registered_language_with_no_tree_is_reported() -> None:
    """The gate names the language rather than counting it as zero."""
    _, problems = gate.measure(["rust"], {})

    assert any("rust is registered" in problem for problem in problems)


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
        'func validateDomainCreate(a map[string]any) string { return "" }\n',
    )

    found = _rooted(tmp_path, gate._TREES["go"])

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

    found = _rooted(tmp_path, gate._TREES["python"])

    assert [name.rsplit(":", maxsplit=1)[-1] for name in found] == [
        "_firewall_ids_argument"
    ]


def test_every_plumbing_name_exists_in_its_tree() -> None:
    """A plumbing entry naming nothing is an exemption with nothing to exempt.

    Without this the set could keep a name long after its function went, and
    the next function to be given that name would be exempt by accident.
    """
    for language, tree in gate._TREES.items():
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


def test_a_new_check_raises_the_count(tmp_path: Path) -> None:
    """The gate still fails loudly when surface writes a check by hand.

    Zero is only worth reaching if it cannot be passed by a tree that grew one
    back, so this drives the same comparison `make check` runs.
    """
    grown = _go_count(
        tmp_path, 'func validateDomainCreate(a map[string]any) string { return "" }\n'
    )

    assert grown == 1
    assert gate.compare({"go": grown}, {"go": 0}, {"go": ["t.go:validateDomainCreate"]})


def test_the_hand_code_detector_cannot_see_this_population(tmp_path: Path) -> None:
    """Why both contract files exist, held as a fact rather than a claim.

    One planted check, named after what it checks rather than after a tool:
    `make hand-code` reports nothing and this gate counts it. Retiring either
    file would leave that check with no gate at all.
    """
    detector = _load_script("verify_hand_code")

    _write(tmp_path, ".gitignore", "/go/internal/gentools/\n")
    _write(
        tmp_path,
        "go/internal/tools/t.go",
        "package tools\n\n"
        'func validateResizeTarget(a map[string]any) string { return "" }\n',
    )

    languages = [detector.Language("go", "go")]
    declared = detector.Declared(tools={"linodevolumeresize": "linode_volume_resize"})

    assert detector.measure(tmp_path, languages, declared) == []
    assert _rooted(tmp_path, gate._TREES["go"]) == [
        "go/internal/tools/t.go:validateResizeTarget"
    ]
