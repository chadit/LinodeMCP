#!/usr/bin/env python3
"""Offline gate: every tool declares what it answers with, not just what it calls.

`make tool-routes` and `make field-location` pin the request half of the
contract. Five options pin the answer: `tool_response` (the message the handler
serializes, which the input message's name does not imply, since roughly half
the surface answers with a differently spelled message), `confirm_message` and
`success_message` (the prose a gated tool answers with unconfirmed and after a
completed mutation), `resource_type` (the two-stage hash-ignore key), and
`retry_disabled`.

A declaration nothing checks drifts, so every check fails both directions:

- a tool with no `tool_response`, and one naming a message the descriptors do
  not define. Tools answering with an open-ended object are listed by name in
  STRUCT_RESPONSE, so one of those declaring a response fails too.
- a gated tool with no `confirm_message`, and a Read or Meta tool carrying one.
- a `resource_type` outside Destroy, and one the hash-ignore table cannot
  place: the table treats an unknown type and a typo alike, so a typo would
  otherwise turn drift detection into a silent whole-state compare.
- a `success_message` placeholder naming no field of the input message, the
  response message, or one of that response's direct sub-messages.
- `retry_disabled` on a Meta tool, which reaches no route to retry.

Two ways to pass while measuring nothing are checked first: no tool declaring a
response (annotations gone, or `make proto` has not run), and a hash-ignore
table that reads back empty. That table is read from every language registered
in docs/contracts/languages.txt, which must agree on it; a registered language
with no reader fails by name.

Reading descriptors needs the generated modules, so `make proto` must have run;
scripts/_toolroutes.py falls back to python/.venv/bin/python when the running
interpreter cannot import them. Run via `make tool-response`, part of
`make check`.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import _toolroutes

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"

# Read and Meta tools never gate on confirm.
GATED = frozenset(
    {"TOOL_CAPABILITY_WRITE", "TOOL_CAPABILITY_ADMIN", "TOOL_CAPABILITY_DESTROY"}
)
DESTROY = "TOOL_CAPABILITY_DESTROY"
META = "TOOL_CAPABILITY_META"

# Tools that serialize through google.protobuf.Struct, so they have no per-tool
# message to name. Listed by name rather than skipping anything undeclared, so a
# tool that merely forgot the option cannot pass as one of these.
STRUCT_RESPONSE = {
    "linode_database_mysql_config_get",
    "linode_database_postgresql_config_get",
    "linode_managed_stats_get",
    "linode_profile_preferences_get",
}

# Gated tools that carry no single confirm sentence, with the reason each one
# cannot. Every other gated tool must declare its prose.
NO_CONFIRM_GATE = {
    "linode_instance_backup_restore": (
        "picks between two sentences at runtime, since restoring over a"
        " populated instance destroys its disks and restoring to a fresh one"
        " does not"
    ),
    "linode_managed_credential_get": (
        "reads a stored credential and gates on the Admin tier alone; neither"
        " language asks for confirm"
    ),
}

# Types the hash-ignore table has no entry for on purpose: nothing about them is
# cosmetic, so the whole state is hashed. Accepted here rather than ignored,
# since the table treats an unknown type and a misspelled one alike.
WHOLE_STATE_TYPES = {
    "IPv6Range",
    "Image",
    "InstanceIP",
    "LKENode",
    "ObjectStorageBucket",
    "ObjectStorageKey",
    "ObjectStorageSSL",
    "PlacementGroup",
    "ReservedIP",
    "SSHKey",
    "Tag",
    "VLAN",
}

# Both tables spell an entry as a quoted type name plus a colon, so one pattern
# reads either. Indent-anchored so a type named in a comment is not read as one.
_TABLE_ENTRY = re.compile(r'^\s+"([A-Za-z0-9]+)":', re.MULTILINE)

# Per-language path to the hash-ignore table, relative to that language's
# working dir. A registered language absent here fails the gate by name.
TABLE_SOURCES = {
    "go": "internal/twostage/hash_ignore.go",
    "python": "src/linodemcp/twostage/hash_ignore.py",
}

_PLACEHOLDER = re.compile(r"\{([a-z0-9_]+)\}")


def registered_languages(path: Path) -> list[tuple[str, Path]]:
    """(name, working dir) per registry line, in file order."""
    languages: list[tuple[str, Path]] = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = [field.strip() for field in line.split("\t") if field.strip()]
        if len(fields) < 2:
            msg = f"unparsable {path.name} line {raw!r}"
            raise SystemExit(msg)
        languages.append((fields[0], _REPO_ROOT / fields[1]))
    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)
    return languages


def hash_ignore_keys(languages: list[tuple[str, Path]]) -> set[str]:
    """Resource types the hash-ignore table defines, proven equal per language.

    Reading every language makes a one-sided table edit fail here instead of
    surfacing later as two clients hashing the same state differently.
    """
    per_language: dict[str, set[str]] = {}
    for name, workdir in languages:
        relative = TABLE_SOURCES.get(name)
        if relative is None:
            msg = (
                f"{name} is registered in {_LANGUAGES.name} but this gate has no"
                f" hash-ignore table path for it; add one to TABLE_SOURCES"
            )
            raise SystemExit(msg)
        source = workdir / relative
        if not source.exists():
            msg = f"{name}: no hash-ignore table at {source}"
            raise SystemExit(msg)
        per_language[name] = set(_TABLE_ENTRY.findall(source.read_text("utf-8")))

    shared = set.union(*per_language.values())
    if not shared:
        msg = (
            "the hash-ignore tables define no resource types;"
            " the gate would compare nothing"
        )
        raise SystemExit(msg)

    disagree = sorted(
        f"{key}: defined by {sorted(n for n, k in per_language.items() if key in k)}"
        for key in shared
        if any(key not in keys for keys in per_language.values())
    )
    if disagree:
        _report(
            "resource types one language's hash-ignore table defines and"
            " another does not:",
            disagree,
            "the tables are one contract; move an entry in every language or none",
        )
        raise SystemExit(1)

    return shared


def response_violations(
    declared: list[_toolroutes.ToolDeclaration], messages: dict[str, dict[str, str]]
) -> list[str]:
    """Tools with no response, an unknown one, or one they should not carry."""
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool:
            continue
        if tool in STRUCT_RESPONSE:
            if entry.response:
                found.append(
                    f"{tool}: answers with a free-form object but declares"
                    f" {entry.response}"
                )
            continue
        if not entry.response:
            found.append(f"{tool}: declares no tool_response")
        elif entry.response not in messages:
            found.append(f"{tool}: names {entry.response}, which no message defines")
    return sorted(found)


def confirm_violations(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Gated tools with no confirm prose, and ungated tools carrying some."""
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool:
            continue
        gated = entry.capability in GATED
        if gated and not entry.confirm_message and tool not in NO_CONFIRM_GATE:
            found.append(f"{tool}: {entry.capability} but declares no confirm_message")
        elif not gated and entry.confirm_message:
            found.append(
                f"{tool}: {entry.capability} tools do not gate on confirm, but it"
                f" declares confirm prose"
            )
        elif gated and entry.confirm_message and tool in NO_CONFIRM_GATE:
            found.append(
                f"{tool}: listed as carrying no single confirm sentence, but it"
                f" declares one"
            )
    return sorted(found)


def resource_type_violations(
    declared: list[_toolroutes.ToolDeclaration], known: set[str]
) -> list[str]:
    """Resource types outside Destroy, and ones the table cannot place."""
    found: list[str] = []
    accepted = known | WHOLE_STATE_TYPES
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool or not entry.resource_type:
            continue
        if entry.capability != DESTROY:
            found.append(
                f"{tool}: declares resource_type {entry.resource_type} but is"
                f" {entry.capability}, so it runs no drift hash"
            )
        if entry.resource_type not in accepted:
            found.append(
                f"{tool}: resource_type {entry.resource_type} is neither a"
                f" hash-ignore table entry nor an accepted whole-state type"
            )
    return sorted(found)


def visible_fields(
    entry: _toolroutes.ToolDeclaration, messages: dict[str, dict[str, str]]
) -> set[str]:
    """Fields a success_message placeholder may name.

    The tool's own input, its response, and that response's direct sub-messages,
    where an envelope keeps the object it wraps.
    """
    names: set[str] = set(messages.get(entry.message, {}))
    for field, submessage in messages.get(entry.response, {}).items():
        names.add(field)
        if submessage:
            names |= set(messages.get(submessage, {}))
    return names


def success_violations(
    declared: list[_toolroutes.ToolDeclaration], messages: dict[str, dict[str, str]]
) -> list[str]:
    """Placeholders that name nothing the tool can fill them from."""
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool or not entry.success_message:
            continue
        visible = visible_fields(entry, messages)
        unknown = sorted(set(_PLACEHOLDER.findall(entry.success_message)) - visible)
        if unknown:
            found.append(
                f"{tool}: success_message fills {', '.join(unknown)} from no field"
                f" of its input, its response, or that response's sub-messages"
            )
    return sorted(found)


def retry_violations(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Meta tools marked unretryable, which reach no route to retry."""
    return sorted(
        f"{entry.meta_tool}: Meta tools reach no Linode route, so retry_disabled"
        f" describes nothing"
        for entry in declared
        if entry.retry_disabled and entry.capability == META
    )


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def main() -> int:
    """Report every disagreement; zero when the whole surface declares itself."""
    keys = hash_ignore_keys(registered_languages(_LANGUAGES))
    declared = _toolroutes.declarations()
    messages = _toolroutes.message_fields()

    if not any(entry.response for entry in declared):
        _report(
            "no tool declares a response message:",
            ["the tool_response options are gone, or `make proto` has not run"],
            "without one the comparison below has nothing to measure",
        )
        return 1

    groups = (
        (
            response_violations(declared, messages),
            "tools whose declared response does not name one real message:",
            (
                "add `option (linode.mcp.v1.tool_response)` naming the message the"
                " handler serializes, then run `make proto`"
            ),
        ),
        (
            confirm_violations(declared),
            "tools whose confirm prose does not match their tier:",
            (
                "a Write, Admin, or Destroy tool declares confirm_message; a Read or"
                " Meta tool declares none"
            ),
        ),
        (
            resource_type_violations(declared, keys),
            "resource types that name nothing the drift hash can use:",
            (
                "spell the type as the hash-ignore table's key, or accept it in"
                " WHOLE_STATE_TYPES in this script"
            ),
        ),
        (
            success_violations(declared, messages),
            "success messages that fill a placeholder from no field:",
            (
                "name a field of the input, the response, or a response sub-message,"
                " or leave success_message unset"
            ),
        ),
        (
            retry_violations(declared),
            "tools marked unretryable that make no request:",
            "drop retry_disabled from the Meta tool's input",
        ),
    )

    failed = False
    for entries, header, remedy in groups:
        if entries:
            _report(header, entries, remedy)
            failed = True

    if failed:
        return 1

    responses = sum(1 for entry in declared if entry.response)
    confirms = sum(1 for entry in declared if entry.confirm_message)
    successes = sum(1 for entry in declared if entry.success_message)
    types = sum(1 for entry in declared if entry.resource_type)
    unretryable = sum(1 for entry in declared if entry.retry_disabled)
    print(
        f"tool-response gate OK: {responses} response(s) over {len(declared)} tool(s)"
        f" plus {len(STRUCT_RESPONSE)} free-form, {confirms} confirm message(s),"
        f" {successes} success message(s), {types} resource type(s) against"
        f" {len(keys)} table entr(ies), {unretryable} unretryable call(s)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
