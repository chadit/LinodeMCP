"""The hand-arms detector: no arm lives beside the generated tree.

Both languages spell an arm their own way and the gate reads each with that
language's own rule, so the tests below fabricate a repo root with a registry,
an ignore file and a tree per language, and hold it against a fabricated
operation vocabulary. The half worth pinning is that there is no allowed set at
all: the contract file that used to list the hand-written ones is gone, and a
function opening with an arm's spelling is reported wherever it sits.

The precision case is pinned too, and it pins the reason the rule is
per-language: the CLI defines runAuditRecent for its own subcommand, which a
case-flattened rule would report as an arm for LOCAL_CALL_AUDIT_RECENT.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

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


gate = _load_script("verify_hand_arms")

# Two registered languages, in the working directories the real registry names.
LANGUAGES = [gate.Language("go", "go"), gate.Language("python", "python")]

# Four operations: one two-sided, one whose stem opens another's, and two
# ordinary ones.
STEMS = {
    "audit_recent": "LOCAL_CALL_AUDIT_RECENT",
    "catalog_can_run": "LOCAL_CALL_CATALOG_CAN_RUN",
    "draft_tools": "LOCAL_CALL_DRAFT_TOOLS",
    "draft": "LOCAL_CALL_DRAFT",
}

# A clean tree: nothing in it opens with an arm's spelling.
GO_CLEAN = (
    "package tools\n\n"
    "func AuditRecent(a any) (any, error) { return nil, nil }\n\n"
    "func runAuditRecent(a any) error { return nil }\n"
)
PY_CLEAN = (
    "def audit_recent(a):\n    return None\n\n\ndef _run_helper(a):\n    return None\n"
)
GITIGNORE = "/go/internal/gentools/\n/python/src/linodemcp/gentools/\n.venv/\n"

# A hand-written arm, in each language's spelling.
STRAY_GO = (
    "package x\n\nfunc RunCatalogCanRun(a any) (any, error) { return nil, nil }\n"
)
STRAY_PY = "def run_catalog_can_run(a):\n    return None\n"


def _write(root: Path, relative: str, text: str) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _repo(tmp_path: Path, extra: dict[str, str] | None = None) -> Path:
    """A fabricated repo root whose hand-written trees carry no arm."""
    _write(tmp_path, ".gitignore", GITIGNORE)
    _write(tmp_path, "go/internal/tools/engine.go", GO_CLEAN)
    _write(tmp_path, "python/src/linodemcp/tools/engine.py", PY_CLEAN)
    for relative, text in (extra or {}).items():
        _write(tmp_path, relative, text)
    return tmp_path


def _measure(root: Path) -> list[str]:
    problems: list[str] = gate.measure(root, LANGUAGES, STEMS)
    return problems


def test_a_tree_carrying_no_arm_is_clean(tmp_path: Path) -> None:
    """The subsystem methods and the CLI runner both languages keep are not arms."""
    assert _measure(_repo(tmp_path)) == []


def test_an_arm_beside_the_generated_tree_fails_by_name(tmp_path: Path) -> None:
    """A hand-written arm is refused whatever it does and wherever it sits."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/tools/stray.go": STRAY_GO,
                "python/src/linodemcp/stray.py": STRAY_PY,
            },
        )
    )

    assert len(problems) == 2
    assert "stray.go defines RunCatalogCanRun" in problems[0]
    assert "LOCAL_CALL_CATALOG_CAN_RUN" in problems[0]
    assert "stray.py defines run_catalog_can_run" in problems[1]
    assert "LOCAL_CALL_CATALOG_CAN_RUN" in problems[1]


def test_the_cli_runner_is_not_an_arm(tmp_path: Path) -> None:
    """Go's unexported runAuditRecent is the CLI's own, and the case says so.

    This is why the rule is each language's own spelling rather than one
    flattened form: flattened, the CLI subcommand reads as an arm for
    LOCAL_CALL_AUDIT_RECENT and the gate reports code that is fine.
    """
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/cli/audit_cmd.go": (
                    "package cli\n\nfunc runAuditRecent(a any) error { return nil }\n"
                ),
            },
        )
    )

    assert problems == []
    spelled = gate.declared_arms(gate.arm_for("go"), STEMS)
    assert gate.classify("runAuditRecent", spelled) is None


def test_both_sides_of_a_two_sided_operation_are_arms(tmp_path: Path) -> None:
    """The opening matches, so a direction suffix needs no vocabulary here."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/tools/stray.go": (
                    "package tools\n\n"
                    "func RunDraftToolsAdd() {}\n\n"
                    "func RunDraftToolsRemove() {}\n"
                ),
            },
        )
    )

    assert len(problems) == 2
    assert all("LOCAL_CALL_DRAFT_TOOLS" in problem for problem in problems)


def test_the_longest_matching_stem_owns_the_definition(tmp_path: Path) -> None:
    """An operation whose stem opens another's must not steal the finding."""
    problems = _measure(
        _repo(
            tmp_path,
            {"go/internal/tools/stray.go": "package x\n\nfunc RunDraftToolsAdd() {}\n"},
        )
    )

    assert len(problems) == 1
    assert "LOCAL_CALL_DRAFT_TOOLS" in problems[0]


def test_generated_trees_tests_and_dot_directories_are_not_scanned(
    tmp_path: Path,
) -> None:
    """The emitted arms, a test naming one, and a venv are all outside."""
    problems = _measure(
        _repo(
            tmp_path,
            {
                "go/internal/gentools/operations.gen.go": STRAY_GO,
                "go/internal/tools/canrun_test.go": STRAY_GO,
                "python/src/linodemcp/gentools/operations.py": STRAY_PY,
                "python/tests/unit/test_canrun.py": STRAY_PY,
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
                "rust/src/stray.rs": "fn run_catalog_can_run() {}\n",
                "elsewhere/stray.go": STRAY_GO,
            },
        )
    )

    assert problems == []


def test_a_registered_language_with_no_scanner_arm_is_reported(tmp_path: Path) -> None:
    """The gate names the language rather than skipping it."""
    root = _repo(tmp_path, {"rust/src/lib.rs": "fn main() {}\n"})

    registered = [*LANGUAGES, gate.Language("rust", "rust")]

    problems = gate.measure(root, registered, STEMS)

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
        gate.measure(tmp_path, LANGUAGES, STEMS)


def test_a_contract_declaring_no_operation_is_reported(tmp_path: Path) -> None:
    """An empty vocabulary means the descriptors did not load."""
    problems = gate.measure(_repo(tmp_path), LANGUAGES, {})

    assert any("declares no local operations" in p for p in problems)


def test_the_real_trees_hold_no_hand_written_arm() -> None:
    """The measurement itself, over the repository as it stands.

    The population is zero and stays zero: this is the case that fails the day
    someone writes an arm beside the generated ones.
    """
    root = Path(__file__).resolve().parents[3]

    registered = gate.registered_languages(gate.registry_path())

    assert gate.measure(root, registered, gate.declared_stems()) == []
