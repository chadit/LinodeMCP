"""Offline tests for the dry-run setup gate and the shared surface readers.

verify_dryrun.py is a hard gate: every mutating tool advertises dry_run in
its proto input, no read-only tool does, and every tool maps to its input
message. The _surface tests pin the two parsing hazards found while building
it: a factory without a proto schema swallowing its neighbor's mapping, and
proto messages that wrap the schema name or nest braces.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

    import pytest

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


surface = _load_script("_surface")
gate = _load_script("verify_dryrun")


def test_factory_map_survives_a_schemaless_neighbor(tmp_path: Path) -> None:
    """A factory with a hand-written schema must not steal its neighbor's map.

    The tempered matcher stops at the next name=, so tool_a (no proto schema)
    maps to nothing instead of swallowing tool_b's message.
    """
    (tmp_path / "tools.py").write_text(
        'Tool(\n    name="tool_a",\n    description="x",\n'
        "    inputSchema={'type': 'object'},\n)\n"
        'Tool(\n    name="tool_b",\n    description="y",\n'
        '    inputSchema=schema("linode.mcp.v1.BInput"),\n)\n',
        encoding="utf-8",
    )

    assert surface.tool_input_messages(tmp_path) == {"tool_b": "BInput"}


def test_factory_map_matches_wrapped_schema_call(tmp_path: Path) -> None:
    """Long message names wrap schema( across lines and must still map."""
    (tmp_path / "tools.py").write_text(
        'Tool(\n    name="tool_long",\n    description="x",\n'
        "    inputSchema=schema(\n"
        '        "linode.mcp.v1.VeryLongMessageNameInput"\n'
        "    ),\n)\n",
        encoding="utf-8",
    )

    assert surface.tool_input_messages(tmp_path) == {
        "tool_long": "VeryLongMessageNameInput"
    }


def test_proto_bodies_handle_nested_braces(tmp_path: Path) -> None:
    (tmp_path / "x.proto").write_text(
        "message Outer {\n"
        "  message Inner {\n    string a = 1;\n  }\n"
        "  optional bool dry_run = 2;\n"
        "}\n"
        "message Plain {\n  string b = 1;\n}\n",
        encoding="utf-8",
    )

    bodies = surface.proto_message_bodies(tmp_path)

    assert set(bodies) == {"Outer", "Plain"}
    assert surface.message_has_field(bodies["Outer"], "dry_run")
    assert not surface.message_has_field(bodies["Plain"], "dry_run")


def test_message_has_field_ignores_prefix_names() -> None:
    body = "  optional int32 page_size = 4;\n"

    assert surface.message_has_field(body, "page_size")
    assert not surface.message_has_field(body, "page")


def test_violations_are_detected_per_direction(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Each failure direction fires: unmapped, missing dry_run, extra dry_run."""
    monkeypatch.setattr(
        gate._surface,
        "read_capabilities",
        lambda: {"t_write": "Write", "t_read": "Read", "t_lost": "Destroy"},
    )
    monkeypatch.setattr(
        gate._surface,
        "tool_input_messages",
        lambda: {"t_write": "WriteInput", "t_read": "ReadInput"},
    )
    monkeypatch.setattr(
        gate._surface,
        "proto_message_bodies",
        lambda: {
            "WriteInput": "  string label = 1;\n",
            "ReadInput": "  optional bool dry_run = 2;\n",
        },
    )

    unmapped, missing, extra = gate.dryrun_violations()

    assert unmapped == ["t_lost (Destroy)"]
    assert missing == ["t_write (Write)"]
    assert extra == ["t_read (Read)"]


def test_live_surface_is_fully_mapped_and_compliant() -> None:
    """The gate itself as a test: setup must hold across all 460 tools.

    Every tool maps to a proto input; every Write/Admin/Destroy input carries
    dry_run; no Read/Meta input does. A regression in any of the three fails
    here and in make check.
    """
    unmapped, missing, extra = gate.dryrun_violations()

    assert unmapped == []
    assert missing == []
    assert extra == []
