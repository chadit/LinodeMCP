"""The hand-code detector: nothing per-tool lives beside the generated tree.

Both languages spell a function their own way and the gate reads them with one
rule, so the tests below fabricate a repo root with a registry, an ignore file
and a tree per language, and hold it against a fabricated contract. The half
worth pinning is that there is no allowed set left: a function named after a
tool is reported whatever it is called and wherever it sits, because nothing
derives a hand-written name from a tool any more, and a string literal that
spells a tool name is reported by file and line for the same reason.

The single-word case is pinned too, and it pins a HOLE rather than a guarantee:
`version` prefixes ordinary vocabulary, so neither arm can claim it. The trees
that call tools by name are pinned as a scope rather than an exemption: the
literal arm leaves them out, the definition arm still reads them, and a tree
whose path merely starts the same way is scanned by both.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

import pytest

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


gate = _load_script("verify_hand_code")

# Two registered languages, in the working directories the real registry names.
LANGUAGES = [gate.Language("go", "go"), gate.Language("python", "python")]

# A contract of five tools: three compound meta tools, one single-word meta
# tool, and one compound name that is a prefix of another.
DECLARED = gate.Declared(
    tools={
        "linodeauditexport": "linode_audit_export",
        "linodeprofilecanrun": "linode_profile_can_run",
        "linodevolumecreate": "linode_volume_create",
        "linodevolume": "linode_volume",
        "version": "version",
    }
)

# A clean tree: nothing in it is named after a tool.
GO_CLEAN = (
    "package tools\n\n"
    "func RunLocalCall(a any) (any, error) { return nil, nil }\n\n"
    "func decodePage(a any) error { return nil }\n"
)
PY_CLEAN = (
    "async def run_local_call(a):\n    return None\n\n\n"
    "def _decode_page(a):\n    return None\n"
)
GITIGNORE = "/go/internal/gentools/\n/python/src/linodemcp/gentools/\n.venv/\n"

# Hand-coded tool code, in each language's spelling.
STRAY_GO = (
    "package x\n\n"
    "func LinodeVolumeCreatePreview(a any) (any, error) { return nil, nil }\n"
)
STRAY_PY = "def linode_volume_create_preview(a):\n    return None\n"

# Tool names spelled in string literals, in the two shapes the tree once held:
# a route lookup keyed on the name, and a comma-joined hand list of names.
LITERAL_GO = (
    "package linode\n\n"
    "func (c *Client) httpGetVolume(ctx context.Context) error {\n"
    '\treturn c.makeRouteRequest(ctx, "linode_volume_create", nil)\n'
    "}\n"
)
LITERAL_PY = (
    "FEATURE_TOOLS_LIST = (\n"
    '    "hello,version,linode_audit_export,"\n'
    "    'linode_profile_can_run'\n"
    ")\n"
)


def _write(root: Path, relative: str, text: str) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _repo(tmp_path: Path, extra: dict[str, str] | None = None) -> Path:
    """A fabricated repo root whose hand-written trees name no tool."""
    _write(tmp_path, ".gitignore", GITIGNORE)
    _write(tmp_path, "go/internal/tools/engine.go", GO_CLEAN)
    _write(tmp_path, "python/src/linodemcp/tools/engine.py", PY_CLEAN)
    for relative, text in (extra or {}).items():
        _write(tmp_path, relative, text)
    return tmp_path


def _measure(root: Path) -> list[str]:
    problems: list[str] = gate.measure(root, LANGUAGES, DECLARED)
    return problems


def test_a_tree_that_names_no_tool_is_clean(tmp_path: Path) -> None:
    """The engine functions both languages keep are not per-tool code."""
    assert _measure(_repo(tmp_path)) == []


def test_a_function_named_after_a_tool_fails_by_name(tmp_path: Path) -> None:
    """Hand-coded tool code beside the generated tree is refused whatever it does."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/tools/stray.go": STRAY_GO,
                "python/src/linodemcp/stray.py": (
                    "def linode_audit_export_helper(a):\n    return None\n"
                ),
            },
        )
    )

    assert len(problems) == 2
    assert "stray.go defines LinodeVolumeCreatePreview" in problems[0]
    assert "linode_volume_create" in problems[0]
    assert "stray.py defines linode_audit_export_helper" in problems[1]
    assert "linode_audit_export" in problems[1]


def test_no_suffix_is_allowed_any_more(tmp_path: Path) -> None:
    """There is no kind vocabulary left, so every tool-named spelling fails."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/tools/stray.go": (
                    "package tools\n\n"
                    "func LinodeProfileCanRunAnswer() {}\n\n"
                    "func LinodeProfileCanRunWhateverThisIs() {}\n"
                ),
            },
        )
    )

    assert len(problems) == 2
    assert all("linode_profile_can_run" in problem for problem in problems)


def test_the_longest_matching_tool_name_owns_the_definition(tmp_path: Path) -> None:
    """A tool whose name prefixes another must not steal the other's finding."""
    problems = _measure(_repo(tmp_path, {"go/internal/tools/stray.go": STRAY_GO}))

    assert len(problems) == 1
    assert "linode_volume_create" in problems[0]


def test_a_single_word_tool_is_not_matched(tmp_path: Path) -> None:
    """The known hole: `version` prefixes ordinary vocabulary, so it is skipped.

    Pinned rather than fixed. An exact-name rule would flag the tracing helper
    the real tree already defines, and an exemption list is the one thing that
    could make this gate lie.
    """
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/cli/version_cmd.go": (
                    'package cli\n\nfunc VersionLine() string { return "" }\n'
                ),
                "go/internal/tools/version.go": (
                    "package tools\n\nfunc VersionAnswer() {}\n"
                ),
                "python/src/linodemcp/tui/extras.py": (
                    "def version_rows():\n    return []\n"
                ),
            },
        )
    )

    assert problems == []
    assert gate.classify("VersionAnswer", DECLARED) is None


def test_generated_trees_tests_and_dot_directories_are_not_scanned(
    tmp_path: Path,
) -> None:
    """A generated handler, a test naming a tool, and a venv are all outside."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/gentools/volume.gen.go": STRAY_GO + LITERAL_GO,
                "go/internal/tools/volume_test.go": STRAY_GO + LITERAL_GO,
                "python/src/linodemcp/gentools/volume.py": STRAY_PY + LITERAL_PY,
                "python/tests/unit/test_volume.py": STRAY_PY + LITERAL_PY,
                "python/.venv/lib/site.py": STRAY_PY + LITERAL_PY,
            },
        )
    )

    assert problems == []


def test_a_tool_name_in_a_string_literal_fails_by_name_and_line(
    tmp_path: Path,
) -> None:
    """A route lookup keyed on a tool name and a hand list of names both fail.

    The list is the shape that hid sixty names in one string: every name on
    every line of it is reported, not the string as a whole.
    """
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/linode/methods_storage.go": LITERAL_GO,
                "python/src/linodemcp/version.py": LITERAL_PY,
            },
        )
    )

    assert len(problems) == 3
    assert "methods_storage.go:4 spells linode_volume_create" in problems[0]
    assert "version.py:2 spells linode_audit_export" in problems[1]
    assert "version.py:3 spells linode_profile_can_run" in problems[2]
    assert all("in a string literal" in problem for problem in problems)


def test_a_literal_matches_a_whole_tool_name_only(tmp_path: Path) -> None:
    """A longer identifier, an unquoted name and a single-word tool are not hits.

    The single-word case pins the same hole the definition arm has: "version"
    is the JSON key every version answer carries.
    """
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/tools/names.go": (
                    "package tools\n\n"
                    'const preview = "linode_volume_create_preview"\n'
                    'const key = "version"\n'
                    "var bare = linode_volume_create\n"
                ),
                "python/src/linodemcp/names.py": (
                    "PREVIEW = 'linode_volume_create_preview'\nKEY = \"hello\"\n"
                ),
            },
        )
    )

    assert problems == []


def test_the_trees_that_call_tools_by_name_are_left_out_of_the_literal_arm_only(
    tmp_path: Path,
) -> None:
    """The CLI and TUI spell tool names to invoke them; nothing else may.

    The definition arm still reads those trees, and a tree whose path merely
    starts with a caller's path is not a caller.
    """
    callers = _measure(
        _repo(
            tmp_path / "callers",
            {
                "go/internal/cli/audit_cmd.go": (
                    'package cli\n\nvar tools = []string{"linode_audit_export"}\n'
                ),
                "python/src/linodemcp/cli/audit.py": (
                    'TOOLS = ("linode_audit_export",)\n'
                ),
                "python/src/linodemcp/tui/app.py": (
                    'SCREEN = "linode_profile_can_run"\n'
                ),
            },
        )
    )
    assert callers == []

    defined = _measure(
        _repo(
            tmp_path / "defined",
            {
                "go/internal/cli/stray.go": (
                    "package cli\n\nfunc LinodeVolumeCreateRun() {}\n"
                ),
            },
        )
    )
    assert len(defined) == 1
    assert "stray.go defines LinodeVolumeCreateRun" in defined[0]

    neighbour = _measure(
        _repo(
            tmp_path / "neighbour",
            {
                "go/internal/client/routes.go": (
                    'package client\n\nconst route = "linode_volume_create"\n'
                ),
                "python/src/linodemcp/tuix/app.py": (
                    'SCREEN = "linode_profile_can_run"\n'
                ),
            },
        )
    )
    assert len(neighbour) == 2
    assert "client/routes.go:3 spells linode_volume_create" in neighbour[0]
    assert "tuix/app.py:1 spells linode_profile_can_run" in neighbour[1]


def test_tools_named_reads_every_quoted_string_on_a_line() -> None:
    """Every quote kind, an escaped quote inside, and a list all read through."""
    compound = gate.compound_tools(DECLARED)

    assert compound == frozenset(
        {
            "linode_audit_export",
            "linode_profile_can_run",
            "linode_volume_create",
            "linode_volume",
        }
    )
    line = (
        "a = \"linode_volume\" + 'x linode_volume_create'"
        ' + "it\\"s linode_audit_export"'
    )
    assert gate.tools_named(line, compound) == [
        "linode_volume",
        "linode_volume_create",
        "linode_audit_export",
    ]
    assert gate.tools_named("b = linode_volume_create", compound) == []
    assert gate.tools_named('c = "version,hello"', compound) == []
    assert gate.tools_named("d := `linode_volume`", compound) == ["linode_volume"]


def test_the_scan_roots_come_from_the_registry(tmp_path: Path) -> None:
    """A working directory the registry does not name is not scanned."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "rust/src/stray.rs": "fn linode_volume_create_preview() {}\n",
                "elsewhere/stray.go": STRAY_GO,
            },
        )
    )

    assert problems == []


def test_a_registered_language_with_no_scanner_arm_is_reported(tmp_path: Path) -> None:
    """The gate names the language rather than skipping it."""
    root = _repo(tmp_path, {"rust/src/lib.rs": "fn main() {}\n"})

    registered = [*LANGUAGES, gate.Language("rust", "rust")]

    problems = gate.measure(root, registered, DECLARED)

    assert any("rust is registered" in p and "no scanner arm" in p for p in problems)


def test_a_language_whose_scan_finds_nothing_is_reported(tmp_path: Path) -> None:
    """A tree with no definition at all is a scan that broke, not a clean one."""
    root = _repo(tmp_path)
    (root / "go/internal/tools/engine.go").write_text(
        "package tools\n", encoding="utf-8"
    )

    problems = _measure(root)

    assert any("go: no definition found under go" in p for p in problems)


def test_a_missing_working_directory_fails_rather_than_passing_empty(
    tmp_path: Path,
) -> None:
    """A root naming nothing would otherwise scan an empty set and pass."""
    _write(tmp_path, ".gitignore", GITIGNORE)

    with pytest.raises(SystemExit, match="go is not a directory"):
        gate.measure(tmp_path, LANGUAGES, DECLARED)


def test_a_contract_declaring_no_tool_is_reported(tmp_path: Path) -> None:
    """An empty contract means the descriptors did not load."""
    empty = gate.Declared(tools={})

    problems = gate.measure(_repo(tmp_path), LANGUAGES, empty)

    assert any("the contract declares no tools" in p for p in problems)


def test_the_argument_check_population_is_held_to_its_own_contract_file(
    tmp_path: Path,
) -> None:
    """A hand-validator line above zero, or a missing one, fails here too."""
    counts = tmp_path / "hand-validator-counts.txt"

    counts.write_text("# header\ngo 0\npython 0\n", encoding="utf-8")
    assert gate.validator_problems(counts, LANGUAGES) == []

    counts.write_text("# header\ngo 2\n", encoding="utf-8")
    problems = gate.validator_problems(counts, LANGUAGES)
    assert len(problems) == 2
    assert "go: hand-validator-counts.txt reads 2" in problems[0]
    assert "python: hand-validator-counts.txt records no count" in problems[1]


def test_gitignore_directory_patterns_are_read_both_ways(tmp_path: Path) -> None:
    """Anchored patterns match one path; bare names match at any depth."""
    ignore = tmp_path / ".gitignore"
    ignore.write_text(
        "# comment\n*.pyc\n/go/internal/gentools/\nvendor/\n!/docs/\n",
        encoding="utf-8",
    )

    ignored = gate.ignored_directories(ignore)

    assert ignored == (frozenset({"go/internal/gentools"}), frozenset({"vendor"}))
    assert gate.is_ignored(Path("go/internal/gentools/a.go"), ignored)
    assert gate.is_ignored(Path("python/vendor/x/y.py"), ignored)
    assert not gate.is_ignored(Path("go/internal/gentools_more/a.go"), ignored)
    assert not gate.is_ignored(Path("go/internal/tools/vendor.go"), ignored)


def test_the_real_trees_hold_no_hand_coded_tool_code() -> None:
    """The shipped source is clean, which is what this campaign was for."""
    languages = gate.registered_languages(gate._LANGUAGES)

    assert gate.measure(gate._REPO_ROOT, languages, gate.declared_contract()) == []
    assert gate.validator_problems(gate._VALIDATOR_COUNTS, languages) == []


def test_the_real_contract_still_declares_tools() -> None:
    """The whole scan compares against this set, so an empty one measures nothing."""
    assert len(gate.declared_contract().tools) > 1


def test_every_registered_language_has_a_scanner_arm() -> None:
    """A registered language with no arm goes unscanned, so it fails by name."""
    names = {language.name for language in gate.registered_languages(gate._LANGUAGES)}

    assert names <= set(gate._ARMS)


def test_the_registry_is_read_as_a_name_and_a_working_directory(
    tmp_path: Path,
) -> None:
    """Both fields travel, since one names the failure and the other the scan."""
    registry = tmp_path / "languages.txt"
    registry.write_text(
        "# registry\ngo\tgo\tdump\npython\tpython\tdump\n", encoding="utf-8"
    )

    assert gate.registered_languages(registry) == LANGUAGES


def test_a_registry_line_without_a_working_directory_fails(tmp_path: Path) -> None:
    """A language with no tree to scan is a registry defect, named as one."""
    registry = tmp_path / "languages.txt"
    registry.write_text("go\n", encoding="utf-8")

    with pytest.raises(SystemExit, match=r"languages\.txt line is not"):
        gate.registered_languages(registry)


def test_main_answers_clean_and_exit_zero(
    capsys: pytest.CaptureFixture[str], monkeypatch: pytest.MonkeyPatch
) -> None:
    """The entry point runs the real scan and reports what it found."""
    monkeypatch.setattr(sys, "argv", ["verify_hand_code.py"])

    assert gate.main() == 0

    out: Any = capsys.readouterr().out
    assert out.startswith("hand-code: ")
    assert "no hand-coded tool code across 2 languages" in out
