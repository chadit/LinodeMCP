"""The hand-code detector: nothing per-tool lives beside the generated tree.

Both languages spell a function their own way and the gate reads them with one
rule, so the tests below fabricate a repo root with a registry, an ignore file
and a tree per language, and hold it against a fabricated contract. The half
worth pinning is that there is no allowed set left: a function named after a
tool is reported whatever it is called and wherever it sits, because nothing
derives a hand-written name from a tool any more.

The single-word case is pinned too, and it pins a HOLE rather than a guarantee:
`version` prefixes ordinary vocabulary, so the scan cannot claim it.
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
                "go/internal/gentools/volume.gen.go": STRAY_GO,
                "go/internal/tools/volume_test.go": STRAY_GO,
                "python/src/linodemcp/gentools/volume.py": STRAY_PY,
                "python/tests/unit/test_volume.py": STRAY_PY,
                "python/.venv/lib/site.py": STRAY_PY,
            },
        )
    )

    assert problems == []


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
