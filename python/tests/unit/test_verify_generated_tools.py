"""Offline tests for the generated-tools ratchet gate.

verify_generated_tools.py holds the tool generator to owning its whole cohort in
every registered language, refuses a hand-written factory left behind for a
generated tool, and counts what each language still serves by hand against
docs/contracts/generated-tools-counts.txt. Those counts only ever fall.

Every failure path has a test below, including the four ways the gate could run
while measuring nothing. The last test measures the real trees, so the committed
counts cannot go stale.

Fixture sources use single quotes so they can nest inside these docstrings.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    # Registered before it runs: @dataclass resolves a field annotation through
    # sys.modules[cls.__module__], which is absent for a spec-loaded module.
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_generated_tools")

# Fixture surface: four tools, two of which the cohort claims.
MANIFEST = (
    "# surface\n"
    "linode_other_get\n"
    "linode_thing_delete\n"
    "linode_thing_get\n"
    "linode_thing_list\n"
)
COHORT = "# cohort\nlinode_thing_get\nlinode_thing_list\n"

# One factory spells its name plainly, one joins two literals with +, the way the
# real client spells one of its own. A scan reading source verbatim would miss the
# second and report that tool as served by nothing.
GO_HAND = """
package tools

func NewLinodeThingDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability) {
	return mcp.NewToolWithRawSchema(
		'linode_thing_delete',
		'Deletes a thing.',
		toolschemas.Schema('linode.mcp.v1.ThingDeleteInput'),
	)
}

const otherGetToolName = 'linode_other_' + 'get'

func NewLinodeOtherGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability) {
	return mcp.NewToolWithRawSchema(
		otherGetToolName,
		'Gets the other thing.',
		toolschemas.Schema('linode.mcp.v1.OtherGetInput'),
	)
}
""".replace("'", '"')

# The Go tests name every tool, generated ones included. Counting a test would
# report a leftover factory for a tool whose factory moved.
GO_TEST = """
package tools_test

func TestFactories(t *testing.T) {
	for _, name := range []string{'linode_thing_get', 'linode_thing_list'} {
		_ = name
	}
}
""".replace("'", '"')

GO_GENERATED = """
package gentools

func NewLinodeThingGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability) {
	return mcp.NewToolWithRawSchema(
		'linode_thing_get',
		'Gets a thing.',
		toolschemas.Schema('linode.mcp.v1.ThingGetInput'),
	)
}

func NewLinodeThingListTool(cfg *config.Config) (mcp.Tool, profiles.Capability) {
	return mcp.NewToolWithRawSchema(
		'linode_thing_list',
		'Lists things.',
		toolschemas.Schema('linode.mcp.v1.ThingListInput'),
	)
}
""".replace("'", '"')

PY_HAND = """
def create_linode_thing_delete_tool():
    return Tool(
        name='linode_thing_delete',
        description='Deletes a thing.',
        input_schema=schema('linode.mcp.v1.ThingDeleteInput'),
    )


def create_linode_other_get_tool():
    return Tool(
        name='linode_other_get',
        description='Gets the other thing.',
        input_schema=schema('linode.mcp.v1.OtherGetInput'),
    )
"""

# The re-export module names its factories, not its tools. A bare substring match
# would read 'create_linode_thing_get_tool' as the tool still being named here.
PY_EXPORTS = """
__all__ = [
    'create_linode_thing_get_tool',
    'create_linode_thing_list_tool',
]
"""

PY_GENERATED = """
def create_linode_thing_get_tool():
    return Tool(
        name='linode_thing_get',
        description='Gets a thing.',
        input_schema=schema('linode.mcp.v1.ThingGetInput'),
    )


def create_linode_thing_list_tool():
    return Tool(
        name='linode_thing_list',
        description='Lists things.',
        input_schema=schema('linode.mcp.v1.ThingListInput'),
    )
"""

# Only the first generated factory, for the test that takes half a cohort away.
PY_GENERATED_PARTIAL = PY_GENERATED.partition("def create_linode_thing_list_tool")[0]

# Each fixture tree serves two tools by hand alongside the two the cohort names.
HAND_TOOLS = 2
MEASURED_COUNTS = f"go {HAND_TOOLS}\npython {HAND_TOOLS}\n"

REGISTRY = "# registry\ngo\tgo\tdump\npython\tpython\tdump\n"

GO_HAND_DIR = "go/internal/tools"
GO_GEN_DIR = "go/internal/gentools"
PY_HAND_DIR = "python/src/linodemcp/tools"
PY_GEN_DIR = "python/src/linodemcp/gentools"


def _write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _fixture_repo(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, counts: str = MEASURED_COUNTS
) -> Path:
    """Point the gate at a two-language repo with both trees and a counts file.

    The test files are there to be ignored: each names a generated tool, so a
    scan that stopped skipping them would report a leftover hand-written factory
    that is not there.
    """
    _write(tmp_path / GO_HAND_DIR / "linode_thing.go", GO_HAND)
    _write(tmp_path / GO_HAND_DIR / "linode_thing_test.go", GO_TEST)
    _write(tmp_path / GO_GEN_DIR / "thing.gen.go", GO_GENERATED)
    _write(tmp_path / PY_HAND_DIR / "linode_thing.py", PY_HAND)
    _write(tmp_path / PY_HAND_DIR / "__init__.py", PY_EXPORTS)
    _write(tmp_path / PY_HAND_DIR / "tests" / "test_thing.py", PY_GENERATED)
    _write(tmp_path / PY_GEN_DIR / "thing.py", PY_GENERATED)
    _write(tmp_path / "languages.txt", REGISTRY)
    _write(tmp_path / "tools-manifest.txt", MANIFEST)
    _write(tmp_path / "generated-tools.txt", COHORT)

    counts_path = tmp_path / "generated-tools-counts.txt"
    _write(counts_path, counts)

    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(gate, "_LANGUAGES", tmp_path / "languages.txt")
    monkeypatch.setattr(gate, "_MANIFEST", tmp_path / "tools-manifest.txt")
    monkeypatch.setattr(gate, "_COHORT", tmp_path / "generated-tools.txt")
    monkeypatch.setattr(gate, "_COUNTS", counts_path)

    return counts_path


def _error(capsys: pytest.CaptureFixture[str]) -> str:
    return capsys.readouterr().err


def test_scan_splits_the_two_trees(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fixture_repo(tmp_path, monkeypatch)

    found = gate.scan("go", gate._TREES["go"], gate.read_names(gate._MANIFEST))

    # Finding linode_other_get proves the + concatenation is read before the scan.
    assert found.hand == frozenset({"linode_thing_delete", "linode_other_get"})
    assert found.generated == frozenset({"linode_thing_get", "linode_thing_list"})


def test_python_scan_reads_whole_literals_not_substrings(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fixture_repo(tmp_path, monkeypatch)

    found = gate.scan("python", gate._TREES["python"], gate.read_names(gate._MANIFEST))

    assert "linode_thing_get" not in found.hand


def test_the_happy_path_passes_and_says_what_it_measured(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch)

    assert gate.main([]) == 0
    assert "2 tool(s) generated in every language" in capsys.readouterr().out


def test_a_cohort_tool_one_language_does_not_generate_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """Half a cohort is a tool served by one binary and not the other."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / PY_GEN_DIR / "thing.py", PY_GENERATED_PARTIAL)

    assert gate.main([]) == 1

    error = _error(capsys)
    assert "python generates no factory for linode_thing_list" in error


def test_a_generated_tool_outside_the_cohort_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """The cohort file is the contract, so the tree may not exceed it either."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(
        tmp_path / GO_GEN_DIR / "thing.gen.go",
        GO_GENERATED + GO_HAND.replace("package tools", ""),
    )

    assert gate.main([]) == 1
    assert "go generates linode_other_get, which the cohort does not" in _error(capsys)


def test_a_leftover_hand_written_factory_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """Both languages register by scanning, so a leftover stages the tool twice."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / GO_HAND_DIR / "linode_thing.go", GO_HAND + GO_GENERATED)

    assert gate.main([]) == 1

    error = _error(capsys)
    assert f"go still names linode_thing_get in {GO_HAND_DIR}" in error


def test_an_empty_cohort_fails_instead_of_passing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """Every cohort check is scoped to the cohort, so an empty one proves nothing."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / "generated-tools.txt", "# nothing listed yet\n")

    assert gate.main([]) == 1
    assert "cohort is empty" in _error(capsys)


def test_an_empty_generated_tree_fails_instead_of_passing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """A tree holding nothing would satisfy the leftover check by measuring nothing.

    Checked ahead of the cohort comparison so the report names the cause rather
    than one line per tool the generator did not write.
    """
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / GO_GEN_DIR / "thing.gen.go", "package gentools\n")

    assert gate.main([]) == 1
    assert f"go: {GO_GEN_DIR} names no tool" in _error(capsys)


def test_an_absent_generated_tree_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The tree is gitignored, so its absence means `make proto` has not run."""
    _fixture_repo(tmp_path, monkeypatch)
    (tmp_path / GO_GEN_DIR / "thing.gen.go").unlink()
    (tmp_path / GO_GEN_DIR).rmdir()

    with pytest.raises(SystemExit, match="no generated tool tree"):
        gate.main([])


def test_a_manifest_tool_no_tree_names_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """A factory the scan cannot see would quietly lower the count by one."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(
        tmp_path / GO_HAND_DIR / "linode_thing.go",
        GO_HAND.replace('"linode_other_" + "get"', "buildOtherName()"),
    )

    assert gate.main([]) == 1
    assert "go names linode_other_get in neither" in _error(capsys)


def test_a_grown_count_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, f"go {HAND_TOOLS - 1}\npython {HAND_TOOLS}\n")

    assert gate.main([]) == 1

    error = _error(capsys)
    assert "hand-written tools have grown" in error
    assert f"go: {HAND_TOOLS} hand-written tool(s), {HAND_TOOLS - 1} recorded" in error


def test_a_fallen_count_fails_until_the_line_follows_it(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """The file states the real remaining work, not an old high-water mark."""
    _fixture_repo(tmp_path, monkeypatch, f"go {HAND_TOOLS + 3}\npython {HAND_TOOLS}\n")

    assert gate.main([]) == 1
    assert "below the recorded count" in _error(capsys)


def test_a_registered_language_with_no_tree_arm_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """A language cannot join the surface with its codegen migration unwatched."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / "languages.txt", REGISTRY + "rust\trust\tdump\n")

    assert gate.main([]) == 1
    assert "no generated-tools scanner declared" in _error(capsys)


def test_a_registered_language_with_no_recorded_line_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, f"go {HAND_TOOLS}\n")

    assert gate.main([]) == 1
    assert "python is registered but has no line" in _error(capsys)


def test_a_recorded_line_without_a_registered_language_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS + "rust 4\n")

    assert gate.main([]) == 1
    assert "rust has a line" in _error(capsys)


def test_a_leftover_factory_fails_before_the_ratchet(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """The leftover also raises the count, and the gate names the cause instead."""
    _fixture_repo(tmp_path, monkeypatch)
    _write(tmp_path / GO_HAND_DIR / "linode_thing.go", GO_HAND + GO_GENERATED)

    assert gate.main([]) == 1

    error = _error(capsys)
    assert "hand-written factories for generated tool(s)" in error
    assert "have grown" not in error


def test_update_baseline_writes_the_measured_counts_in_registry_order(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    counts_path = _fixture_repo(tmp_path, monkeypatch, "go 99\npython 99\n")
    _write(tmp_path / "languages.txt", "python\tpython\tdump\ngo\tgo\tdump\n")

    assert gate.main(["--update-baseline"]) == 0

    written = counts_path.read_text(encoding="utf-8")
    assert written.startswith("# Generated-tools counts:")
    assert written.endswith(f"python {HAND_TOOLS}\ngo {HAND_TOOLS}\n")


def test_recorded_counts_match_the_real_trees() -> None:
    languages = gate.registered_languages(gate._LANGUAGES)
    manifest = gate.read_names(gate._MANIFEST)

    assert gate.missing_arms(languages) == []

    measured = {
        name: gate.scan(name, gate._TREES[name], manifest) for name in languages
    }

    assert gate.cohort_problems(gate.read_names(gate._COHORT), measured) == []
    assert gate.leftovers(gate.read_names(gate._COHORT), measured) == []
    assert gate.unlocatable(gate.read_names(gate._COHORT), manifest, measured) == []
    assert {name: len(found.hand) for name, found in measured.items()} == (
        gate.read_counts(gate._COUNTS)
    )
