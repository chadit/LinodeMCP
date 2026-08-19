"""Offline tests for the tool-response gate.

verify_tool_response.py pins the answering half of the tool contract: the
response message, the description a client picks the tool by, the confirm,
success, and failure prose, the two-stage resource type, and the retry choice.
These tests cover each violation class, the ways the gate could pass while
measuring nothing, and one that reads the real descriptors so the checked-in
surface has to stay fully declared.
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
    warning_message: str = "",
    resource_type: str = "",
    retry_disabled: bool = False,
    description: str = "Creates a DNS domain.",
    error_message: str = "Failed to create domain: {error}",
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
        warning_message=warning_message,
        resource_type=resource_type,
        retry_disabled=retry_disabled,
        description=description,
        error_message=error_message,
    )


def test_shipped_contract_declares_every_answer() -> None:
    """The gate passes on the real descriptors, which is what it exists to hold.

    A failure here means the proto lost or mis-stated one of the options, not
    that the test is wrong.
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
    found = gate.confirm_violations([_declaration(confirm_message="")], set())

    assert found == [f"{_TOOL}: {_WRITE} but declares no confirm_message"]


def test_read_tool_carrying_confirm_prose_is_reported() -> None:
    """The other direction: a Read tool gates on nothing, so prose there
    advertises a confirmation that never happens."""
    found = gate.confirm_violations([_declaration(capability=_READ)], set())

    assert found == [
        (
            f"{_TOOL}: {_READ} tools do not gate on confirm, but it declares"
            f" confirm prose"
        )
    ]


def test_meta_tool_asking_for_confirm_is_gated() -> None:
    """A tool that reaches no route still gates when its input asks for confirm.

    The profile-builder save rewrites the config file behind confirm=true in
    both languages, so its tier cannot be what decides whether the sentence
    belongs on the message.
    """
    found = gate.confirm_violations(
        [_declaration(capability=_META, route_tool="", meta_tool=_TOOL)], {_MESSAGE}
    )

    assert found == []


def test_meta_tool_asking_for_confirm_without_prose_is_reported() -> None:
    """The other direction: asking for confirm without declaring the sentence
    leaves each language inventing what the caller reads."""
    found = gate.confirm_violations(
        [
            _declaration(
                capability=_META, route_tool="", meta_tool=_TOOL, confirm_message=""
            )
        ],
        {_MESSAGE},
    )

    assert found == [f"{_TOOL}: {_META} but declares no confirm_message"]


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


def test_warning_with_no_field_to_carry_it_is_reported() -> None:
    """A notice the response cannot hold is dropped by the serializer.

    DomainWriteResponse is {message, domain}, so the sentence would reach no
    client at all. The emitter refuses the same shape at generation time.
    """
    found = gate.warning_violations(
        [_declaration(warning_message="Records are added separately.")], _MESSAGES
    )

    assert found == [
        (
            f"{_TOOL}: declares a warning_message and {_RESPONSE} has no warning"
            " field to report it in"
        )
    ]


def test_warning_placeholder_naming_no_field_is_reported() -> None:
    """The notice fills the way the sentence beside it does, so the same rule holds."""
    messages = {
        **_MESSAGES,
        _RESPONSE: {"message": "", "warning": "", "domain": "linode.mcp.v1.Domain"},
    }

    found = gate.warning_violations(
        [_declaration(warning_message="Zone '{nickname}' is not live yet")], messages
    )

    assert found == [
        (
            f"{_TOOL}: warning_message fills nickname from no field of its input,"
            f" its response, or that response's sub-messages"
        )
    ]


def test_warning_filled_from_the_wrapped_object_passes() -> None:
    """A response carrying the field it names is the shape the gate accepts."""
    messages = {
        **_MESSAGES,
        _RESPONSE: {"message": "", "warning": "", "domain": "linode.mcp.v1.Domain"},
    }

    found = gate.warning_violations(
        [_declaration(warning_message="Zone '{domain}' (ID: {id}) is new")], messages
    )

    assert found == []


def test_response_promising_a_warning_is_left_to_the_emitter() -> None:
    """A hand-written tool fills that field in code, so the gate stays quiet.

    Failing here would report the whole un-migrated warning surface as broken.
    The emitter refuses it for the tools it actually generates, which is the
    only place the notice could be dropped.
    """
    messages = {
        **_MESSAGES,
        _RESPONSE: {"message": "", "warning": "", "domain": "linode.mcp.v1.Domain"},
    }

    assert gate.warning_violations([_declaration()], messages) == []


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


def test_missing_description_is_reported() -> None:
    """A tool with no description reaches a client unlabeled, so a model has
    only the tool name to pick it by."""
    found = gate.description_violations([_declaration(description="")])

    assert found == [f"{_TOOL}: declares no tool_description"]


def test_meta_tool_without_a_description_is_reported() -> None:
    """The description check takes no exemption list: a Meta tool advertises
    itself to a client exactly like a routed one does."""
    found = gate.description_violations(
        [
            _declaration(
                route_tool="", meta_tool="hello", capability=_META, description=""
            )
        ]
    )

    assert found == ["hello: declares no tool_description"]


def test_routed_tool_without_a_failure_sentence_is_reported() -> None:
    """Leaving the failure side spelled out in each language is what lets one
    tool answer the same failure differently depending on which binary served
    it."""
    found = gate.failure_violations([_declaration(error_message="")])

    assert found == [f"{_TOOL}: declares no error_message"]


def test_collection_tool_without_a_failure_sentence_passes() -> None:
    """A page answers with one shared sentence across the whole list surface,
    so naming the tool it came from would add nothing."""
    found = gate.failure_violations(
        [
            _declaration(
                response="linode.mcp.v1.DomainListResponse",
                capability=_READ,
                confirm_message="",
                error_message="",
            )
        ]
    )

    assert found == []


def test_listed_tool_without_a_failure_sentence_passes() -> None:
    """The other side of the named list: these are the tools whose sentence a
    shared helper builds today."""
    listed = next(iter(gate.SHARED_FAILURE))
    found = gate.failure_violations([_declaration(route_tool=listed, error_message="")])

    assert found == []


def test_listed_tool_that_gained_its_own_sentence_is_reported() -> None:
    """The list records where a sentence is built, not a permanent exemption:
    a tool that grew its own has to leave the list so the list keeps stating
    the real remaining work."""
    listed = next(iter(gate.SHARED_FAILURE))
    found = gate.failure_violations(
        [_declaration(route_tool=listed, error_message="Failed to act: {error}")]
    )

    assert len(found) == 1
    assert found[0].startswith(f"{listed}: listed as answering with a shared")


def test_meta_tool_with_a_failure_sentence_is_reported() -> None:
    """A Meta tool works on local state, so it makes no call whose failure the
    sentence could describe."""
    found = gate.failure_violations(
        [
            _declaration(
                route_tool="",
                meta_tool="hello",
                capability=_META,
                confirm_message="",
                error_message="Failed to greet: {error}",
            )
        ]
    )

    assert found == [
        (
            "hello: Meta tools reach no Linode route, so there is no call whose"
            " failure error_message could describe"
        )
    ]


def test_failure_placeholder_naming_no_input_field_is_reported() -> None:
    """A failed call produced no response to read from, so a placeholder that
    names a response field would print an empty value."""
    found = gate.failure_placeholder_violations(
        [_declaration(error_message="Failed to create domain {id}: {error}")],
        _MESSAGES,
    )

    assert found == [f"{_TOOL}: error_message fills id from no field of its input"]


def test_failure_placeholder_naming_an_input_field_passes() -> None:
    """The binding the option exists for: a sentence naming the argument the
    caller passed."""
    found = gate.failure_placeholder_violations(
        [_declaration(error_message="Failed to create domain {domain}: {error}")],
        _MESSAGES,
    )

    assert found == []


def test_shared_failure_entry_naming_no_tool_is_reported() -> None:
    """An entry naming nothing exempts nothing while reading as though it does,
    which is the shape a typo or a renamed tool takes."""
    found = gate.unlisted_shared_failures([_declaration()])

    assert len(found) == len(gate.SHARED_FAILURE)
    assert all("no routed tool is called that" in entry for entry in found)


def test_gate_refuses_a_surface_that_declares_no_description(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The second way to measure nothing. With every description stripped the
    per-tool checks below would still print OK on the remaining options."""
    stripped = [_declaration(description="")]
    monkeypatch.setattr(gate._toolroutes, "declarations", lambda: stripped)
    monkeypatch.setattr(gate._toolroutes, "message_fields", lambda: _MESSAGES)

    assert gate.main() == 1


def test_gate_refuses_a_surface_that_declares_no_failure_sentence(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The third. Every routed tool losing error_message at once reads as the
    option being gone rather than as 449 separate omissions."""
    stripped = [_declaration(error_message="")]
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
