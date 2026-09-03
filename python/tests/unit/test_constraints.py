"""The buf.validate rules the contract declares, read through the seam.

Every case here is a call a client can make and the sentence it gets back, and
the whole table is the twin of Go's toolvalidate package test: a rule is one
declaration, so the two languages have to answer it the same way or the
declaration is not the source of truth it claims to be.

The cases expecting "" are the other half of the contract. The rules hold their
peace where another part of the handler owns the answer, and a rule that spoke
there would take away the sentence a caller reads today.
"""

import importlib
import pkgutil
import subprocess
import sys
from typing import Any

import pytest
from google.protobuf.descriptor import Descriptor

from linodemcp.genpb.buf.validate import validate_pb2
from linodemcp.genpb.linode.mcp import v1 as genpb
from linodemcp.tools.constraints import check, unknown_arguments

DOMAIN_CREATE = "linode.mcp.v1.DomainCreateInput"
DOMAIN_UPDATE = "linode.mcp.v1.DomainUpdateInput"

EXAMPLE_DOMAIN = "example.com"
EXAMPLE_SOA = "admin@example.com"

ERR_PATTERN = "domain must match the documented domain-name pattern"
ERR_SOA_MASTER = "soa_email is required for master domains"
ERR_MASTER_IPS = "master_ips must include at least one value for slave domains"
ERR_DOMAIN_ID = "domain_id must be a positive integer"


@pytest.mark.parametrize(
    ("message", "arguments", "want"),
    [
        pytest.param(
            DOMAIN_CREATE,
            {"type": "master", "soa_email": EXAMPLE_SOA},
            "domain is required",
            id="absent domain",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "a" * 254, "type": "master", "soa_email": EXAMPLE_SOA},
            "domain must be between 1 and 253 characters",
            id="domain longer than the documented cap",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "bad domain", "type": "master", "soa_email": EXAMPLE_SOA},
            ERR_PATTERN,
            id="domain with a space",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "localhost", "type": "master", "soa_email": EXAMPLE_SOA},
            ERR_PATTERN,
            id="domain without a tld",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "a" * 64 + ".com", "type": "master", "soa_email": EXAMPLE_SOA},
            ERR_PATTERN,
            id="domain label over 63 characters",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "example.123", "type": "master", "soa_email": EXAMPLE_SOA},
            ERR_PATTERN,
            id="numeric tld",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": "*.example.com", "type": "master", "soa_email": EXAMPLE_SOA},
            "",
            id="documented wildcard domain",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "master"},
            ERR_SOA_MASTER,
            id="master without an soa email",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "master", "soa_email": ""},
            ERR_SOA_MASTER,
            id="master with an emptied soa email",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "slave", "master_ips": []},
            ERR_MASTER_IPS,
            id="slave with an emptied master_ips",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "slave", "master_ips": ["192.0.2.20"]},
            "",
            id="slave with a master ip",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "status": "edit_mode",
            },
            "status must be one of: active, disabled",
            id="status naming no member",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "status": "active",
            },
            "",
            id="documented status",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN},
            "",
            id="absent type leaves the type rules to the hook",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "primary"},
            "",
            id="type naming no zone type leaves every later rule to the hook",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "primary", "status": "edit_mode"},
            "",
            id="type naming no zone type outranks a bad status",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "status": 1,
            },
            "",
            id="status of the wrong type is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "master", "soa_email": 1},
            "",
            id="soa email of the wrong type is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "status": True,
            },
            "",
            id="enum given a boolean is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "status": [],
            },
            "",
            id="enum given a list is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {"domain": EXAMPLE_DOMAIN, "type": "slave", "master_ips": '["192.0.2.20"]'},
            "",
            id="json encoded master_ips is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "tags": '[" prod "]',
            },
            "",
            id="json encoded tags is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "retry_sec": 1.5,
            },
            "",
            id="fractional integer is left to the body",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "master",
                "soa_email": EXAMPLE_SOA,
                "retry_sec": 1.0,
            },
            "",
            id="integral json number reads as the integer it is",
        ),
        pytest.param(
            DOMAIN_CREATE,
            {
                "domain": EXAMPLE_DOMAIN,
                "type": "slave",
                "axfr_ips": [],
                "description": "",
                "expire_sec": 604800,
                "group": "",
                "master_ips": ["192.0.2.20"],
                "refresh_sec": 14400,
                "retry_sec": 0,
                "soa_email": "",
                "status": "active",
                "tags": [],
                "ttl_sec": 300,
            },
            "",
            id="every optional supplied with its zero",
        ),
        pytest.param(
            DOMAIN_UPDATE, {"confirm": True}, ERR_DOMAIN_ID, id="absent domain id"
        ),
        pytest.param(
            DOMAIN_UPDATE, {"domain_id": 0}, ERR_DOMAIN_ID, id="zero domain id"
        ),
        pytest.param(
            DOMAIN_UPDATE,
            {"domain_id": "abc"},
            ERR_DOMAIN_ID,
            id="domain id that is not a number reads as absent",
        ),
        pytest.param(
            DOMAIN_UPDATE, {"domain_id": -5}, ERR_DOMAIN_ID, id="negative domain id"
        ),
        pytest.param(DOMAIN_UPDATE, {"domain_id": 5}, "", id="supplied domain id"),
    ],
)
def test_check_answers_the_declared_sentence(
    message: str, arguments: dict[str, Any], want: str
) -> None:
    assert check(message, arguments) == want


@pytest.mark.parametrize(
    ("supplied", "want"),
    [
        pytest.param("123/456", ERR_DOMAIN_ID, id="id carrying a path segment"),
        pytest.param("1?foo=bar", ERR_DOMAIN_ID, id="id carrying a query"),
        pytest.param("../7", ERR_DOMAIN_ID, id="id carrying a traversal"),
        pytest.param("1_000", ERR_DOMAIN_ID, id="id grouped with underscores"),
        pytest.param("+123", ERR_DOMAIN_ID, id="id with a leading plus"),
        pytest.param("007", ERR_DOMAIN_ID, id="id with leading zeros"),
        pytest.param("١٢٣", ERR_DOMAIN_ID, id="id in digits outside ascii"),
        pytest.param(" 123 ", ERR_DOMAIN_ID, id="id with surrounding space"),
        pytest.param("5", "", id="id spelled whole"),
        pytest.param("1e1", "", id="id spelled with an exponent"),
        pytest.param("5.0", "", id="id spelled with a zero fraction"),
    ],
)
def test_check_reads_only_an_integer_spelled_whole(supplied: str, want: str) -> None:
    """An integer argument arrives as a string often enough that both languages
    read one, and each JSON reader used to be lenient in its own direction: Go's
    took the 123 out of "123/456" and let the rule pass on an id the path read
    then refused in other words, while Python's read spellings of its own such as
    "1_000". Only a string spelling one whole number is read now, so every case
    here is the same sentence in both languages."""
    assert check(DOMAIN_UPDATE, {"domain_id": supplied}) == want


@pytest.mark.parametrize(
    "message",
    [
        pytest.param("linode.mcp.v1.DomainGetInput", id="message with no rules"),
        pytest.param(
            "linode.mcp.v1.NoSuchInput", id="name the contract does not define"
        ),
        pytest.param("linode.mcp.v1.FieldLocation", id="name that is not a message"),
        pytest.param("", id="empty name"),
    ],
)
def test_check_is_silent_where_nothing_is_declared(message: str) -> None:
    """A message declaring no rules is the whole rest of the surface, and the
    seam runs on every generated handler, so answering nothing there is what
    keeps it from changing any tool it was not pointed at."""
    assert check(message, {"domain": ""}) == ""


def test_check_ignores_undeclared_arguments() -> None:
    """An argument the message does not declare belongs to whatever else reads
    it, so it neither feeds a rule nor stops one from running."""
    arguments = {"domain": EXAMPLE_DOMAIN, "type": "master", "nonsense": [1, 2]}

    assert check(DOMAIN_CREATE, arguments) == ERR_SOA_MASTER


SETTINGS_UPDATE = "linode.mcp.v1.AccountSettingsUpdateInput"
SETTINGS_UPDATE_TOOL = "linode_account_settings_update"


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        pytest.param(
            {"backups_enabled": True, "confirm": True}, "", id="every argument declared"
        ),
        pytest.param(
            {"dry_run": True, "environment": "default"},
            "",
            id="system params are declared too",
        ),
        pytest.param(
            {
                "confirm_bypass_dry_run": True,
                "confirmed_dry_run": True,
                "yolo": True,
            },
            "",
            id="the engine control names pass",
        ),
        pytest.param(
            {"backups_enabled": True, "bogus": "x"},
            f"Unsupported argument(s) for {SETTINGS_UPDATE_TOOL}: bogus",
            id="one unknown",
        ),
        pytest.param(
            {"zebra": 1, "alpha": 2},
            f"Unsupported argument(s) for {SETTINGS_UPDATE_TOOL}: alpha, zebra",
            id="several unknown, sorted",
        ),
    ],
)
def test_unknown_arguments_answers_for_an_undeclared_name(
    arguments: dict[str, Any], want: str
) -> None:
    """Every tool refuses a name its input message does not declare, and the
    sentence is this engine's own. Go's refusedAsUndeclared holds the other
    copy, and the behavior fixtures are what keep the two equal. No fixture
    sends yolo, so the control-name case here is the only guard on that entry.
    """
    assert unknown_arguments(arguments, SETTINGS_UPDATE_TOOL, SETTINGS_UPDATE) == want


@pytest.mark.parametrize(
    "message",
    [
        pytest.param(
            "linode.mcp.v1.NoSuchInput", id="name the contract does not define"
        ),
        pytest.param("linode.mcp.v1.FieldLocation", id="name that is not a message"),
        pytest.param("", id="empty name"),
    ],
)
def test_unknown_arguments_refuses_nothing_without_a_descriptor(message: str) -> None:
    """A name the pool does not carry as a message leaves no allowlist to hold
    the call to, and refusing every argument there would be worse than refusing
    none."""
    assert unknown_arguments({"zebra": 1}, SETTINGS_UPDATE_TOOL, message) == ""


def _constrained_messages() -> list[Descriptor]:
    """Every contract message that declares rules, read from the generated
    modules because a pool lookup only finds what something imported."""
    found: list[Descriptor] = []

    for module in pkgutil.iter_modules(genpb.__path__):
        if not module.name.endswith("_pb2"):
            continue

        loaded: Any = importlib.import_module(f"{genpb.__name__}.{module.name}")
        found.extend(
            descriptor
            for descriptor in loaded.DESCRIPTOR.message_types_by_name.values()
            if descriptor.GetOptions().HasExtension(validate_pb2.message)
        )

    return found


def _reachable_enum(descriptor: Descriptor, seen: set[str]) -> str:
    """Name the first enum a message reaches, or "" when it reaches none.

    Well-known types are skipped because their JSON reading is their own and
    takes no member name from the contract.
    """
    if (
        descriptor.full_name.startswith("google.protobuf.")
        or descriptor.full_name in seen
    ):
        return ""

    seen.add(descriptor.full_name)

    for field in descriptor.fields:
        if field.enum_type is not None:
            return str(field.enum_type.full_name)

        if field.message_type is None:
            continue

        reached = _reachable_enum(field.message_type, seen)
        if reached:
            return reached

    return ""


def _nested_enums(descriptor: Descriptor) -> tuple[int, list[str]]:
    """Count the message-typed fields one constrained message declares and name
    every enum they reach."""
    inspected = 0
    reached: list[str] = []

    for field in descriptor.fields:
        if field.message_type is None:
            continue

        inspected += 1

        found = _reachable_enum(field.message_type, set())
        if found:
            reached.append(f"{descriptor.full_name}.{field.name} reaches enum {found}")

    return inspected, reached


def test_no_constrained_message_carries_a_nested_enum() -> None:
    """A string naming no member of an enum is coerced at the top level of a call
    only, so one inside a nested message would leave the whole call to the body
    builder and quietly take every other rule with it. No field in the contract
    can carry a nested enum today, and this names the one that would rather than
    letting the gap reopen unannounced."""
    inspected = 0
    reached: list[str] = []

    for descriptor in _constrained_messages():
        count, found = _nested_enums(descriptor)
        inspected += count
        reached.extend(found)

    assert not reached, reached
    assert inspected, "no constrained message declares a message-typed field"


def test_check_reads_a_message_nothing_else_imported() -> None:
    """A pool lookup only finds a message whose generated module was imported,
    and a handler does not import its own, so a tool nothing else pulls in
    would read as declaring no rules and skip every check it declares."""
    subprocess.run(
        [
            sys.executable,
            "-c",
            (
                "from linodemcp.tools.constraints import check;"
                " answer = check('linode.mcp.v1.AccountBetaEnrollInput', {});"
                " assert answer == 'id is required', answer"
            ),
        ],
        check=True,
    )
