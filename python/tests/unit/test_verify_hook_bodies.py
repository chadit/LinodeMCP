"""The hook-body ratchet counts one step of one tool, in whichever language.

Both languages spell a hook name their own way and the gate reads them with one
rule, so the tests below fabricate trees in each spelling and hold them against a
fabricated contract. The half worth pinning is what happens when the two
disagree: a body no tool declares and a declaration with no body both have to be
reported by name, or the count could sit still while the surface moved.
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


gate = _load_script("verify_hook_bodies")

# One tool declaring one hook of each countable kind, in the flattened spelling
# the gate matches on.
DECLARED = {
    "linodevolumedeletefetchstate": "fetch_state",
    "linodevolumedeletedependencywalk": "dependency_walk",
    "linodedomaincreatepreview": "preview",
    "linodeprofilecanrunanswer": "answer",
}


def _write(root: Path, relative: str, text: str) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _go(tmp_path: Path, sources: dict[str, str]) -> tuple[dict[str, int], list[str]]:
    """Attribute a fabricated Go hook package against DECLARED."""
    for relative, text in sources.items():
        _write(tmp_path, f"go/internal/toolhooks/{relative}", text)
    return _attributed(tmp_path, "go", gate._TREES["go"])


def _python(
    tmp_path: Path, body: str, declared: dict[str, str] | None = None
) -> tuple[dict[str, int], list[str]]:
    """Attribute a fabricated Python hooks module against DECLARED."""
    _write(tmp_path, "python/src/linodemcp/toolhooks.py", body)
    return _attributed(tmp_path, "python", gate._TREES["python"], declared)


def _attributed(
    tmp_path: Path,
    language: str,
    tree: Any,
    declared: dict[str, str] | None = None,
) -> tuple[dict[str, int], list[str]]:
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        counts: dict[str, int]
        problems: list[str]
        counts, problems = gate.attribute(language, tree, declared or DECLARED)
    finally:
        swap._REPO_ROOT = original

    return counts, problems


def test_a_body_counts_under_the_kind_its_tool_declared(tmp_path: Path) -> None:
    """Go's four spellings each land under the kind the contract named."""
    counts, problems = _go(
        tmp_path,
        {
            "h.go": (
                "func LinodeVolumeDeleteFetchState(a any) (any, error)"
                " { return nil, nil }\n\n"
                "func LinodeVolumeDeleteDependencyWalk(a any) (any, error)"
                " { return nil, nil }\n\n"
                "func LinodeDomainCreatePreview(a any) (any, error)"
                " { return nil, nil }\n\n"
                "func LinodeProfileCanRunAnswer(a any) (any, error)"
                " { return nil, nil }\n"
            )
        },
    )

    assert problems == []
    assert counts == {
        "fetch_state": 1,
        "dependency_walk": 1,
        "preview": 1,
        "answer": 1,
    }


def test_both_languages_read_through_one_rule(tmp_path: Path) -> None:
    """The same four hooks in Python's spelling give the same four counts."""
    counts, problems = _python(
        tmp_path,
        "async def linode_volume_delete_fetch_state(a):\n    return None\n\n\n"
        "async def linode_volume_delete_dependency_walk(a):\n    return None\n\n\n"
        "async def linode_domain_create_preview(a):\n    return None\n\n\n"
        "async def linode_profile_can_run_answer(a):\n    return None\n",
    )

    assert problems == []
    assert counts == {
        "fetch_state": 1,
        "dependency_walk": 1,
        "preview": 1,
        "answer": 1,
    }


def test_a_body_no_tool_declares_is_reported_by_name(tmp_path: Path) -> None:
    """A hook nothing calls is a number that would never fall, so it fails."""
    _, problems = _python(
        tmp_path,
        "async def linode_volume_delete_fetch_state(a):\n    return None\n\n\n"
        "async def linode_volume_delete_dependency_walk(a):\n    return None\n\n\n"
        "async def linode_domain_create_preview(a):\n    return None\n\n\n"
        "async def linode_profile_can_run_answer(a):\n    return None\n\n\n"
        "async def linode_ghost_tool_preview(a):\n    return None\n",
    )

    assert len(problems) == 1
    assert "linode_ghost_tool_preview" in problems[0]


def test_a_declared_hook_with_no_body_is_reported_by_name(tmp_path: Path) -> None:
    """A tree that stopped defining a declared hook is a scan that broke."""
    _, problems = _go(
        tmp_path,
        {
            "h.go": (
                "func LinodeVolumeDeleteFetchState(a any) (any, error)"
                " { return nil, nil }\n"
            )
        },
    )

    assert len(problems) == 3
    assert any("linodedomaincreatepreview" in problem for problem in problems)
    assert all("declares a" in problem for problem in problems)


def test_test_files_are_not_counted(tmp_path: Path) -> None:
    """A table test naming hook shapes is not a second implementation."""
    counts, _ = _go(
        tmp_path,
        {
            "h.go": (
                "func LinodeVolumeDeleteFetchState(a any) (any, error)"
                " { return nil, nil }\n"
            ),
            "h_test.go": (
                "func LinodeDomainCreatePreview(a any) (any, error)"
                " { return nil, nil }\n"
            ),
        },
    )

    assert counts["fetch_state"] == 1
    assert counts["preview"] == 0


def test_methods_are_not_counted(tmp_path: Path) -> None:
    """A method on a type is not a top-level hook definition."""
    counts, _ = _go(
        tmp_path,
        {
            "h.go": (
                "func (c *Client) LinodeDomainCreatePreview(a any) (any, error)"
                " { return nil, nil }\n"
            )
        },
    )

    assert counts["preview"] == 0


def test_plumbing_is_not_counted_and_is_not_an_orphan(tmp_path: Path) -> None:
    """A named plumbing entry is support vocabulary, exempt by name not by file.

    Everything beside it in the same file still has to resolve to a declaration,
    which is the half worth pinning: the exemption cannot widen to its neighbours.
    """
    counts, problems = _go(
        tmp_path,
        {
            "h.go": (
                'func FunctionName(tool, kind string) string { return "" }\n\n'
                "func LinodeVolumeDeleteFetchState(a any) (any, error)"
                " { return nil, nil }\n\n"
                "func LinodeStrayHelper(a any) (any, error) { return nil, nil }\n"
            )
        },
    )

    assert counts["fetch_state"] == 1
    assert any("LinodeStrayHelper" in problem for problem in problems)
    assert not any("FunctionName" in problem for problem in problems)


def test_every_plumbing_name_exists_in_its_tree() -> None:
    """A plumbing entry naming nothing is an exemption with nothing to exempt.

    Without this the set could keep a name long after its function went, and the
    next function given that name would be exempt by accident.
    """
    for language, tree in gate._TREES.items():
        if not tree.plumbing:
            continue

        assert tree.plumbing <= set(gate.defined(tree)), (
            f"{language} plumbing names nothing: "
            f"{sorted(tree.plumbing - set(gate.defined(tree)))}"
        )


def test_validate_is_left_to_the_gate_that_owns_it() -> None:
    """One population under two gates is two numbers that can disagree."""
    assert "validate" in gate._OWNED_ELSEWHERE
    assert "validate" not in set(gate.declared_hooks().values())


def test_a_missing_tree_root_fails_rather_than_counting_zero(tmp_path: Path) -> None:
    """A root naming nothing would otherwise scan an empty set and pass."""
    swap: Any = gate
    original = swap._REPO_ROOT
    swap._REPO_ROOT = tmp_path
    try:
        raised = False
        try:
            gate.defined(gate._TREES["go"])
        except SystemExit:
            raised = True
    finally:
        swap._REPO_ROOT = original

    assert raised


def test_a_malformed_counts_line_fails_with_a_worded_message(tmp_path: Path) -> None:
    """A bare unpack traceback reads as a broken gate rather than a bad line."""
    for bad in ("go preview", "go preview many", "go preview 4 extra"):
        counts = tmp_path / "hook-body-counts.txt"
        counts.write_text(f"# header\n{bad}\n", encoding="utf-8")

        raised = ""
        try:
            gate.read_counts(counts)
        except SystemExit as stop:
            raised = str(stop)

        assert "hook-body-counts.txt" in raised, bad
        assert "<language> <kind> <count>" in raised, bad


def test_a_well_formed_counts_line_still_reads(tmp_path: Path) -> None:
    """The guard above must not refuse the shape the contract actually uses."""
    counts = tmp_path / "hook-body-counts.txt"
    counts.write_text("# header\ngo preview 128\n", encoding="utf-8")

    assert gate.read_counts(counts) == {("go", "preview"): 128}


def test_direction_failures_name_both_sides() -> None:
    """Above and below the recorded line both fail, naming the movement."""
    above = gate.compare({("go", "preview"): 5}, {("go", "preview"): 4})
    assert above
    assert "up from" in above[0]

    below = gate.compare({("go", "preview"): 3}, {("go", "preview"): 4})
    assert below
    assert "down from" in below[0]

    assert gate.compare({("go", "preview"): 4}, {("go", "preview"): 4}) == []


def test_a_line_naming_no_declared_kind_fails() -> None:
    """A retired cohort takes its lines with it rather than leaving a floor."""
    problems = gate.compare({("go", "preview"): 4}, {("go", "normalize"): 0})

    assert any("normalize" in problem for problem in problems)


def test_a_kind_with_no_line_fails() -> None:
    """New hand-written surface cannot arrive without a floor stating it."""
    problems = gate.compare({("go", "preview"): 4}, {})

    assert any("records no count" in problem for problem in problems)


def test_the_real_trees_match_the_recorded_counts() -> None:
    """The shipped contract states what the shipped source measures."""
    languages = gate.registered_languages(
        gate._REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )
    counts, problems = gate.measure(languages)

    assert problems == []
    assert counts == gate.read_counts(gate._COUNTS)


def test_every_registered_language_has_a_tree() -> None:
    """A registered language with no scanner goes uncounted, so it fails by name."""
    languages = gate.registered_languages(
        gate._REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )

    assert set(languages) <= set(gate._TREES)


def test_a_registered_language_with_no_tree_is_reported() -> None:
    """The gate names the language rather than skipping it."""
    _, problems = gate.measure(["go", "python", "rust"])

    assert any("rust" in problem for problem in problems)


def test_regenerating_an_unchanged_tree_rewrites_the_same_bytes(
    tmp_path: Path,
) -> None:
    """The file is a contract, so its order cannot depend on dict iteration."""
    languages = gate.registered_languages(
        gate._REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )
    counts, _ = gate.measure(languages)
    target = tmp_path / "counts.txt"

    gate.write_counts(target, gate.ordered(counts, languages))
    first = target.read_bytes()
    gate.write_counts(target, gate.ordered(counts, languages))

    assert first == target.read_bytes()
    assert first == gate._COUNTS.read_bytes()


def test_a_validate_line_is_told_which_gate_owns_it() -> None:
    """A kind owned elsewhere is not a retired cohort, so it says so instead.

    The two failures read the same to a scanner and mean opposite things: one
    asks for the line to come out, the other says the count lives in the file
    next door.
    """
    problems = gate.compare(
        {("go", "preview"): 4}, {("go", "preview"): 4, ("go", "validate"): 0}
    )

    assert len(problems) == 1
    assert "hand-validator-counts.txt" in problems[0]
    assert "retired" not in problems[0]
