"""Offline tests for the tool-response gate.

verify_tool_response.py pins the answering half of the tool contract: the
response message, the confirm and success prose, the two-stage resource type,
and the retry choice. These tests cover each violation class, the ways the gate
could pass while measuring nothing, and one that reads the real descriptors so
the checked-in surface has to stay fully declared.
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
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_tool_response")
reader = _load_script("_toolroutes")

_TOOL = "linode_domain_create"
_MESSAGE = "linode.mcp.v1.DomainCreateInput"
_RESPONSE = "linode.mcp.v1.DomainWriteResponse"
_DESTROY_TOOL = "linode_domain_delete"

_READ = "TOOL_CAPABILITY_READ"
_WRITE = "TOOL_CAPABILITY_WRITE"
_DESTROY = "TOOL_CAPABILITY_DESTROY"
_META = "TOOL_CAPABILITY_META"

# One envelope wrapping one object, which is the shape every write response has
# and the reason a placeholder may reach one level down.
_MESSAGES = {
    _MESSAGE: {"domain": "", "confirm": ""},
    _RESPONSE: {"message": "", "domain": "linode.mcp.v1.Domain"},
    "linode.mcp.v1.Domain": {"id": "", "domain": ""},
}


def _declaration(
    *,
    message: str = _MESSAGE,
    route_tool: str = _TOOL,
    meta_tool: str = "",
    capability: str = _WRITE,
    response: str = _RESPONSE,
    confirm_message: str = "Set confirm=true to proceed.",
    success_message: str = "",
    resource_type: str = "",
    retry_disabled: bool = False,
) -> object:
    """One declaration as the reader hands it over."""
    return reader.ToolDeclaration(
        message=message,
        route_tool=route_tool,
        meta_tool=meta_tool,
        capability=capability,
        response=response,
        confirm_message=confirm_message,
        success_message=success_message,
        resource_type=resource_type,
        retry_disabled=retry_disabled,
    )


def test_shipped_contract_declares_every_answer() -> None:
    """The gate passes on the real descriptors, which is what it exists to hold.

    A failure here means the proto lost or mis-stated one of the five options,
    not that the test is wrong.
    """
    assert gate.main() == 0


def test_missing_response_is_reported() -> None:
    """A tool that says which route it calls but not what comes back leaves
    every language naming the response type again at its own marshal call."""
    found = gate.response_violations([_declaration(response="")], _MESSAGES)

    assert found == [f"{_TOOL}: declares no tool_response"]


def test_response_naming_no_message_is_reported() -> None:
    """A typo or a renamed message has to fail here rather than at the first
    attempt to serialize through it."""
    found = gate.response_violations(
        [_declaration(response="linode.mcp.v1.NoSuchResponse")], _MESSAGES
    )

    assert found == [
        (f"{_TOOL}: names linode.mcp.v1.NoSuchResponse, which no message defines")
    ]


def test_free_form_tool_declaring_a_response_is_reported() -> None:
    """The list is an exemption from having a message, not a licence to name
    one: a tool that gained a proto model should leave the list."""
    free_form = next(iter(gate.STRUCT_RESPONSE))
    found = gate.response_violations(
        [_declaration(route_tool=free_form, response=_RESPONSE)], _MESSAGES
    )

    assert found == [
        (f"{free_form}: answers with a free-form object but declares {_RESPONSE}")
    ]


def test_free_form_tool_without_a_response_passes() -> None:
    """The other side of the same list: these four are the only tools allowed
    to declare nothing."""
    free_form = next(iter(gate.STRUCT_RESPONSE))
    found = gate.response_violations(
        [_declaration(route_tool=free_form, response="")], _MESSAGES
    )

    assert found == []


def test_gated_tool_without_confirm_prose_is_reported() -> None:
    """A Write tool with no gate text would leave each language inventing the
    sentence a user reads before approving a mutation."""
    found = gate.confirm_violations([_declaration(confirm_message="")])

    assert found == [f"{_TOOL}: {_WRITE} but declares no confirm_message"]


def test_read_tool_carrying_confirm_prose_is_reported() -> None:
    """The other direction: a Read tool gates on nothing, so prose there
    advertises a confirmation that never happens."""
    found = gate.confirm_violations([_declaration(capability=_READ)])

    assert found == [
        (
            f"{_TOOL}: {_READ} tools do not gate on confirm, but it declares"
            f" confirm prose"
        )
    ]


def test_listed_gateless_tool_that_declares_prose_is_reported() -> None:
    """A tool accepted as carrying no single sentence has to leave that list
    once it does, or the reason recorded there goes stale unnoticed."""
    listed = next(iter(gate.NO_CONFIRM_GATE))
    found = gate.confirm_violations([_declaration(route_tool=listed)])

    assert found == [
        (
            f"{listed}: listed as carrying no single confirm sentence, but it"
            f" declares one"
        )
    ]


def test_listed_gateless_tool_without_prose_passes() -> None:
    """The two tools that cannot carry one sentence stay accepted."""
    listed = next(iter(gate.NO_CONFIRM_GATE))
    found = gate.confirm_violations(
        [_declaration(route_tool=listed, confirm_message="")]
    )

    assert found == []


def test_resource_type_outside_destroy_is_reported() -> None:
    """Only a Destroy runs the two-stage drift hash, so a type anywhere else
    describes a step the tool never takes."""
    found = gate.resource_type_violations(
        [_declaration(resource_type="Domain")], {"Domain"}
    )

    assert found == [
        (
            f"{_TOOL}: declares resource_type Domain but is {_WRITE}, so it runs no"
            f" drift hash"
        )
    ]


def test_unknown_resource_type_is_reported() -> None:
    """The table returns nothing for a type it has no entry for, so a misspelled
    key reads as "hash everything" instead of failing. This is what catches it."""
    found = gate.resource_type_violations(
        [
            _declaration(
                route_tool=_DESTROY_TOOL, capability=_DESTROY, resource_type="Domian"
            )
        ],
        {"Domain"},
    )

    assert found == [
        (
            f"{_DESTROY_TOOL}: resource_type Domian is neither a hash-ignore table"
            f" entry nor an accepted whole-state type"
        )
    ]


def test_accepted_whole_state_type_passes() -> None:
    """A type deliberately absent from the table hashes the whole state, which
    is legal once it is written down."""
    accepted = next(iter(gate.WHOLE_STATE_TYPES))
    found = gate.resource_type_violations(
        [
            _declaration(
                route_tool=_DESTROY_TOOL, capability=_DESTROY, resource_type=accepted
            )
        ],
        {"Domain"},
    )

    assert found == []


def test_success_placeholder_naming_no_field_is_reported() -> None:
    """A placeholder is a promise that the value can be filled; one naming no
    field would render literally in the message a user reads."""
    found = gate.success_violations(
        [_declaration(success_message="Domain '{nickname}' created")], _MESSAGES
    )

    assert found == [
        (
            f"{_TOOL}: success_message fills nickname from no field of its input, its"
            f" response, or that response's sub-messages"
        )
    ]


def test_success_placeholder_reaching_into_the_wrapped_object_passes() -> None:
    """The write envelope carries the API object, so the fields a message
    reports live one level below the response itself."""
    found = gate.success_violations(
        [_declaration(success_message="Domain '{domain}' (ID: {id}) created")],
        _MESSAGES,
    )

    assert found == []


def test_success_placeholder_naming_an_input_field_passes() -> None:
    """A message may also report what the caller asked for."""
    found = gate.success_violations(
        [_declaration(success_message="Confirmed {confirm}")], _MESSAGES
    )

    assert found == []


def test_retry_disabled_on_a_meta_tool_is_reported() -> None:
    """Meta tools work on local state, so there is no request to replay."""
    found = gate.retry_violations(
        [
            _declaration(
                route_tool="",
                meta_tool="version",
                capability=_META,
                confirm_message="",
                retry_disabled=True,
            )
        ]
    )

    assert found == [
        (
            "version: Meta tools reach no Linode route, so retry_disabled describes"
            " nothing"
        )
    ]


def test_retry_disabled_on_a_routed_tool_passes() -> None:
    """The case the option exists for: a create that must not be replayed."""
    assert gate.retry_violations([_declaration(retry_disabled=True)]) == []


def test_hash_ignore_tables_agree_across_languages() -> None:
    """Both clients hash the same state, so a one-sided edit to the table is a
    drift-detection difference waiting to happen."""
    keys = gate.hash_ignore_keys(gate.registered_languages(gate._LANGUAGES))

    assert "Instance" in keys


def test_registered_language_with_no_table_path_fails_by_name(
    tmp_path: Path,
) -> None:
    """Registering a language must force the decision rather than leave half the
    contract unchecked."""
    registry = tmp_path / "languages.txt"
    registry.write_text("rust\trust\tcargo run\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="rust is registered"):
        gate.hash_ignore_keys(gate.registered_languages(registry))


def test_disagreeing_tables_are_reported(tmp_path: Path) -> None:
    """A type one language strips and another does not."""
    go_dir = tmp_path / "go" / "internal" / "twostage"
    py_dir = tmp_path / "python" / "src" / "linodemcp" / "twostage"
    go_dir.mkdir(parents=True)
    py_dir.mkdir(parents=True)
    (go_dir / "hash_ignore.go").write_text('\t\t"Instance": {},\n', encoding="utf-8")
    (py_dir / "hash_ignore.py").write_text(
        '    "Instance": [],\n    "Volume": [],\n', encoding="utf-8"
    )

    with pytest.raises(SystemExit):
        gate.hash_ignore_keys(
            [("go", tmp_path / "go"), ("python", tmp_path / "python")]
        )


def test_empty_table_refuses_to_measure_nothing(tmp_path: Path) -> None:
    """A table that reads back empty would accept any resource_type at all, so
    it fails rather than passing an unchecked contract."""
    go_dir = tmp_path / "go" / "internal" / "twostage"
    py_dir = tmp_path / "python" / "src" / "linodemcp" / "twostage"
    go_dir.mkdir(parents=True)
    py_dir.mkdir(parents=True)
    (go_dir / "hash_ignore.go").write_text("// no entries\n", encoding="utf-8")
    (py_dir / "hash_ignore.py").write_text("# no entries\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="define no resource types"):
        gate.hash_ignore_keys(
            [("go", tmp_path / "go"), ("python", tmp_path / "python")]
        )


def test_gate_refuses_a_surface_that_declares_no_response(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The guard that measures nothing. With the option stripped from every
    message the per-tool checks would still report the free-form tools as fine
    and print OK, so the whole scan is checked for emptiness first."""
    stripped = [_declaration(response="", confirm_message="")]
    monkeypatch.setattr(gate._toolroutes, "declarations", lambda: stripped)
    monkeypatch.setattr(gate._toolroutes, "message_fields", lambda: _MESSAGES)

    assert gate.main() == 1


def test_reader_refuses_an_empty_message_set(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Field lookups against no messages would call every placeholder valid by
    finding nothing to contradict it."""
    monkeypatch.setattr(reader, "_read_message_fields", dict)

    with pytest.raises(SystemExit, match="no messages found"):
        reader.message_fields()
