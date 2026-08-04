"""Offline tests for the Python half of the tool generator.

scripts/toolgen_py.py reads the compiled descriptors and writes the factories
and handlers for the tools docs/contracts/generated-tools.txt names. `make
proto` runs it with nobody reading its output, so these tests pin that it
refuses by name whatever the contract cannot answer, and that what it does
write is the shape the registry and the offline gates recognize.

No tool in the cohort declares a Python hook today, so the hook seam is proved
against a stub module: a hook that is declared and not written has to fail at
emit, not when a caller reaches the tool.
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
COHORT = REPO_ROOT / "docs" / "contracts" / "generated-tools.txt"
HOOKS = REPO_ROOT / "docs" / "contracts" / "tool-hooks.txt"
LANGUAGES = REPO_ROOT / "docs" / "contracts" / "languages.txt"

# One pilot per tier the emitter covers: a resource, a top-level collection, a
# resource under a path id, and a collection under one.
DOMAIN_GET = "linode_domain_get"
DOMAIN_LIST = "linode_domain_list"
RECORD_GET = "linode_domain_record_get"
RECORD_LIST = "linode_domain_record_list"


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


@pytest.fixture(name="toolgen")
def toolgen_fixture() -> ModuleType:
    """The emitter module."""
    return _load_script("toolgen_py")


def _write(path: Path, text: str) -> Path:
    path.write_text(text, encoding="utf-8")
    return path


def test_cohort_skips_comments_and_blanks(toolgen: ModuleType, tmp_path: Path) -> None:
    cohort = _write(tmp_path / "cohort.txt", "# a note\n\nlinode_domain_get\n")
    assert toolgen.read_cohort(cohort) == [DOMAIN_GET]


def test_unknown_tool_fails_by_name(toolgen: ModuleType) -> None:
    """A cohort entry the contract does not declare stops the run.

    Silently generating nothing for a typo would leave the tool hand-written
    and the cohort claiming otherwise.
    """
    with pytest.raises(Exception, match="linode_domain_typo") as caught:
        toolgen.build_contract("linode_domain_typo")
    assert "declares no contract" in str(caught.value)


def test_get_tier_reads_a_single_resource(toolgen: ModuleType) -> None:
    """A response with no repeated message field is one resource.

    Domain repeats master_ips and tags, so the tier cannot be read off
    "repeats something": it is read off repeating a message.
    """
    built = toolgen.build_contract(DOMAIN_GET)
    assert built.tier is toolgen.Tier.GET
    assert built.element is None
    assert built.slots == ("domain_id",)


def test_list_tier_reads_a_collection(toolgen: ModuleType) -> None:
    """A response repeating a message is a collection, with no path id."""
    built = toolgen.build_contract(DOMAIN_LIST)
    assert built.tier is toolgen.Tier.LIST
    assert built.element is not None
    assert built.element.name == "Domain"
    assert built.slots == ()


def test_subresource_list_tier_reads_a_nested_collection(toolgen: ModuleType) -> None:
    """A collection addressed under a path id is the sub-resource tier."""
    built = toolgen.build_contract(RECORD_LIST)
    assert built.tier is toolgen.Tier.SUBRESOURCE_LIST
    assert built.slots == ("domain_id",)


def test_filters_come_from_the_query_arguments(toolgen: ModuleType) -> None:
    """Each non-pagination query argument becomes one filter, in field order.

    The order fixes the wording of the applied-filter echo, so it is part of
    what the contract decides rather than something the emitter sorts.
    """
    built = toolgen.build_contract(DOMAIN_LIST)
    derived = [
        (entry.argument, entry.element_field, entry.contains) for entry in built.filters
    ]
    assert derived == [
        ("domain_contains", "domain", True),
        ("type", "type", False),
    ]


def test_pagination_arguments_are_not_filters(toolgen: ModuleType) -> None:
    """page and page_size select a page rather than narrowing one."""
    built = toolgen.build_contract(RECORD_LIST)
    assert {entry.argument for entry in built.filters} == {"type", "name_contains"}


def test_get_handler_reads_its_path_arguments(toolgen: ModuleType) -> None:
    """A required path value is read and checked before the driver runs.

    The rejection sentence matters as much as the check: it is what the tool
    answers, and testdata/behavior/ pins it.
    """
    module = toolgen.render_module([toolgen.build_contract(RECORD_GET)], {})
    assert 'domain_id = arguments.get("domain_id", 0)' in module
    assert 'return error_response("domain_id is required")' in module
    assert 'return error_response("record_id is required")' in module


def test_factory_carries_the_literals_the_gates_read(toolgen: ModuleType) -> None:
    """scripts/_surface.py pairs a tool with its input message by reading both
    out of the factory source, so neither may become an expression."""
    module = toolgen.render_module([toolgen.build_contract(DOMAIN_GET)], {})
    assert 'name="linode_domain_get"' in module
    assert 'schema("linode.mcp.v1.DomainGetInput")' in module


def test_handler_names_its_tool_to_the_driver(toolgen: ModuleType) -> None:
    """The route scanner resolves a driver call through the tool it names, so
    the tool travels as a literal keyword argument."""
    module = toolgen.render_module([toolgen.build_contract(DOMAIN_LIST)], {})
    assert "run_list_tool(" in module
    assert 'tool="linode_domain_list"' in module


def test_registry_exports_every_pair(toolgen: ModuleType, tmp_path: Path) -> None:
    """The server finds a tool by its create/handle pair, so both are exported."""
    contracts = [toolgen.build_contract(name) for name in (DOMAIN_GET, DOMAIN_LIST)]
    files = toolgen.render_tree(contracts, {}, tmp_path)
    registry = files["__init__.py"]
    for tool in (DOMAIN_GET, DOMAIN_LIST):
        assert f'"create_{tool}_tool"' in registry
        assert f'"handle_{tool}"' in registry


def test_written_tree_drops_a_stale_module(toolgen: ModuleType, tmp_path: Path) -> None:
    """A module the cohort no longer produces leaves with it.

    A factory left behind would keep registering a tool the contract stopped
    describing, and the tree is gitignored, so nothing else would notice.
    """
    stale = _write(tmp_path / "old.py", "# left over\n")
    toolgen.write_tree(tmp_path, {"new.py": "# current\n"})
    assert not stale.exists()
    assert (tmp_path / "new.py").exists()


def test_hook_manifest_reports_an_unregistered_language(
    toolgen: ModuleType, tmp_path: Path
) -> None:
    """A language column naming nothing registered is refused.

    Each emitter reads only its own lines, so a typo in the other language's
    name would otherwise leave a hook nothing acts on in a file both trust.
    """
    manifest = _write(tmp_path / "hooks.txt", "linode_domain_get validate rust Check\n")
    with pytest.raises(Exception, match="rust"):
        toolgen.read_hooks(manifest, [DOMAIN_GET], {"go", "python"})


def test_hook_manifest_reports_an_ungenerated_tool(
    toolgen: ModuleType, tmp_path: Path
) -> None:
    """A hook for a tool nothing generates is refused: only generated code
    calls one, so it would read as behavior nothing reaches."""
    manifest = _write(tmp_path / "hooks.txt", "linode_domain_typo validate go Check\n")
    with pytest.raises(Exception, match="linode_domain_typo"):
        toolgen.read_hooks(manifest, [DOMAIN_GET], {"go", "python"})


def test_hook_manifest_reports_an_unknown_kind(
    toolgen: ModuleType, tmp_path: Path
) -> None:
    """A kind with no call shape is refused rather than read and ignored."""
    manifest = _write(tmp_path / "hooks.txt", "linode_domain_get shape go Check\n")
    with pytest.raises(Exception, match="shape"):
        toolgen.read_hooks(manifest, [DOMAIN_GET], {"go", "python"})


def test_python_reads_only_its_own_hook_lines(
    toolgen: ModuleType, tmp_path: Path
) -> None:
    """A Go hook is not a Python one. The one in the shipped manifest is Go's,
    and Python derives the same sentence its handler already answered."""
    manifest = _write(
        tmp_path / "hooks.txt",
        "linode_domain_get validate go GoCheck\n"
        "linode_domain_list validate python PyCheck\n",
    )
    hooks = toolgen.read_hooks(manifest, [DOMAIN_GET, DOMAIN_LIST], {"go", "python"})
    assert hooks == {DOMAIN_LIST: "PyCheck"}


def test_declared_hook_without_an_implementation_fails(toolgen: ModuleType) -> None:
    """A hook the manifest names and the module does not define stops the run.

    The emitted call names the function directly, so left to itself this would
    surface when the server imports the generated module. Failing at emit is
    the same move the Go side gets from its compiler.
    """
    module = _load_script("toolgen_py")
    with pytest.raises(Exception, match="MissingCheck"):
        toolgen.resolve_hooks({DOMAIN_GET: "MissingCheck"}, module.__name__)


def test_declared_hook_resolves_when_implemented(toolgen: ModuleType) -> None:
    """A hook the module does implement passes, so the check bites on absence
    rather than on every hook."""
    toolgen.resolve_hooks({DOMAIN_GET: "read_cohort"}, toolgen.__name__)


def test_hooked_handler_calls_the_hook_instead_of_deriving(
    toolgen: ModuleType,
) -> None:
    """A tool with a declared hook hands it the whole argument check.

    Keeping the derived checks beside it would answer the derived sentence
    first, which is exactly the wording the hook exists to replace.
    """
    built = toolgen.build_contract(RECORD_GET)
    module = toolgen.render_module([built], {RECORD_GET: "record_get_validate"})
    assert "record_get_validate(arguments)" in module
    assert 'error_response("domain_id is required")' not in module
    assert "from linodemcp.toolhooks import record_get_validate" in module

    # No tool in the cohort declares a Python hook, so this is the only place
    # the hooked shape is ever built. Compiling it is what keeps it from being
    # a template that has never been valid Python.
    compile(module, "hooked.py", "exec")


def test_shipped_manifest_declares_no_python_hook(toolgen: ModuleType) -> None:
    """The cohort's one hook is Go's.

    This is a fact about today's contract rather than a rule: when a Python
    hook is added, this test is what says the module has to exist with it.
    """
    languages = toolgen.read_languages(LANGUAGES)
    assert toolgen.read_hooks(HOOKS, toolgen.read_cohort(COHORT), languages) == {}


def test_emitted_tree_matches_the_checked_in_output(
    toolgen: ModuleType, tmp_path: Path
) -> None:
    """Running the emitter again writes what the working tree already holds.

    `make proto` writes that tree, so a difference means the emitter's output
    depends on something other than the contract, which is the one property
    that makes a gitignored tree safe to ship inside the binary.
    """
    out = REPO_ROOT / "python" / "src" / "linodemcp" / "gentools"
    contracts = [toolgen.build_contract(name) for name in toolgen.read_cohort(COHORT)]
    rendered = toolgen.render_tree(contracts, {}, out)

    for name, text in rendered.items():
        assert (out / name).read_text(encoding="utf-8") == text
