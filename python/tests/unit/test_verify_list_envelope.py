"""Focused tests for the list-envelope falsey-collapse guard."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

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
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_list_envelope")


def test_flags_collapse_in_the_nested_call_that_builds_the_list() -> None:
    """The finding names the inner function holding both the collapse and the call."""
    source = """
async def handle_widget_list(arguments, cfg):
    async def _call(client):
        raw = await client.list_widgets()
        items = raw.get("widgets") or []
        return serialize_list_response({"data": items}, "widgets", WidgetList())

    return await execute_tool(cfg, arguments, "list widgets", _call)
"""
    assert gate.module_violations(source, "linode_widgets.py") == [
        "linode_widgets.py:handle_widget_list._call"
    ]


def test_flags_collapse_beside_the_keyed_serializer_too() -> None:
    """Reaching for the keyed helper does not license the collapse beside it."""
    source = """
def build(raw):
    return serialize_keyed_list_response(raw.get("page") or [], "widgets", WidgetList())
"""
    assert gate.module_violations(source, "linode_widgets.py") == [
        "linode_widgets.py:build"
    ]


def test_attributes_a_def_nested_inside_a_block_to_itself() -> None:
    """Descent stops at a nested def wherever it sits, not just at the body top."""
    source = """
def outer(flag):
    if flag:
        def build(raw):
            items = raw.get("widgets") or []
            return serialize_list_response({"data": items}, "widgets", WidgetList())

        return build
    return None
"""
    assert gate.module_violations(source, "linode_widgets.py") == [
        "linode_widgets.py:outer.build"
    ]


def test_ignores_a_clean_keyed_handler() -> None:
    """The fixed shape (no `or []`, root check inside the helper) is not a finding."""
    source = """
async def handle_widget_list(arguments, cfg):
    async def _call(client):
        raw = await client.list_widgets()
        return serialize_keyed_list_response(raw, "widgets", WidgetList())

    return await execute_tool(cfg, arguments, "list widgets", _call)
"""
    assert gate.module_violations(source, "linode_widgets.py") == []


def test_ignores_element_shaping_helpers() -> None:
    """`or []` coercing a documented-nullable field is untouched by the rule."""
    source = """
def widget_to_dict(widget):
    return {"tags": widget.get("tags") or [], "ips": widget.get("ips") or []}


async def handle_widget_list(arguments, cfg):
    async def _call(client):
        widgets = await client.list_widgets()
        return serialize_list_response({"data": widgets}, "widgets", WidgetList())

    return await execute_tool(cfg, arguments, "list widgets", _call)
"""
    assert gate.module_violations(source, "linode_widgets.py") == []


def test_ignores_a_collapse_with_no_list_response_in_the_same_function() -> None:
    """The rule is scoped to list building, not to `or []` everywhere."""
    source = """
def summarize(raw):
    return {"tags": raw.get("tags") or []}
"""
    assert gate.module_violations(source, "linode_widgets.py") == []


def test_ignores_a_non_empty_list_default() -> None:
    """`or [fallback]` supplies a real default; only `or []` hides a bad shape."""
    source = """
def build(raw):
    items = raw.get("widgets") or [DEFAULT]
    return serialize_list_response({"data": items}, "widgets", WidgetList())
"""
    assert gate.module_violations(source, "linode_widgets.py") == []


def test_baseline_entries_are_still_real() -> None:
    """A baseline line that no longer matches the tree is stale and must be dropped."""
    path = REPO_ROOT / "docs" / "contracts" / "list-envelope-baseline.txt"
    baseline = {
        line.split("  # ", 1)[0].strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.startswith("#")
    }
    assert baseline <= set(gate.current_violations())


def test_every_registered_language_declares_coverage() -> None:
    """A registered language with no scanner and no exemption is unguarded surface."""
    languages = gate.registered_languages(
        REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )
    assert [name for name, _ in languages]
    assert gate.undeclared_languages(languages) == []


def test_undeclared_language_is_reported_by_name(tmp_path: Path) -> None:
    """Registering a language without deciding its coverage fails, and says which."""
    registry = tmp_path / "languages.txt"
    registry.write_text("# comment\ngo\tgo\tdump\nrust\trust\tdump\n", encoding="utf-8")

    languages = gate.registered_languages(registry)

    assert [name for name, _ in languages] == ["go", "rust"]
    assert gate.undeclared_languages(languages) == ["rust"]


def test_registry_scope_comes_from_the_working_dir(tmp_path: Path) -> None:
    """The scanned tree is the registry's working dir, not a path in the script."""
    root = tmp_path / "python"
    (root / "src" / "deep").mkdir(parents=True)
    (root / "src" / "deep" / "handler.py").write_text(
        "def build(raw):\n"
        '    items = raw.get("widgets") or []\n'
        '    return serialize_list_response({"data": items}, "widgets", W())\n',
        encoding="utf-8",
    )

    found = gate.python_violations(root)

    assert [entry.rsplit("/", 1)[-1] for entry in found] == ["handler.py:build"]


def test_python_scan_skips_dot_directories(tmp_path: Path) -> None:
    """A venv or tool cache under the working dir is not repository source."""
    root = tmp_path / "python"
    (root / ".venv" / "lib").mkdir(parents=True)
    (root / ".venv" / "lib" / "vendored.py").write_text(
        "def build(raw):\n"
        '    items = raw.get("widgets") or []\n'
        '    return serialize_list_response({"data": items}, "widgets", W())\n',
        encoding="utf-8",
    )

    assert gate.python_violations(root) == []


def test_rule_targets_serializers_that_exist() -> None:
    """A renamed serializer would leave the rule matching nothing, silently."""
    from linodemcp.tools import proto_response

    assert gate.LIST_SERIALIZERS
    for name in sorted(gate.LIST_SERIALIZERS):
        assert callable(getattr(proto_response, name, None)), name


def test_go_exemption_still_matches_the_go_source() -> None:
    """Go's stated exemption names a helper that must still exist to hold."""
    helper = "listProtoElementsKeyed"
    reason = gate.COVERAGE["go"]
    assert isinstance(reason, str)
    assert helper in reason

    workdirs = dict(
        gate.registered_languages(REPO_ROOT / "docs" / "contracts" / "languages.txt")
    )
    sources = list(workdirs["go"].rglob("*.go"))
    assert sources
    assert any(helper in path.read_text(encoding="utf-8") for path in sources)
