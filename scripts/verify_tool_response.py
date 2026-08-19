#!/usr/bin/env python3
"""Offline gate: every tool declares what it answers with, not just what it calls.

`make tool-routes` and `make field-location` pin the request half of the
contract. Seven options pin the answer: `tool_response` (the message the handler
serializes, which the input message's name does not imply, since roughly half
the surface answers with a differently spelled message), `confirm_message` and
`success_message` (the prose a gated tool answers with unconfirmed and after a
completed mutation), `tool_description` (the sentence a client picks the tool
by) and `error_message` (the prose a failed call reports), `resource_type` (the
two-stage hash-ignore key), and `retry_disabled`.

A declaration nothing checks drifts, so every check fails both directions:

- a tool with no `tool_response`, and one naming a message the descriptors do
  not define. Tools answering with an open-ended object are listed by name in
  STRUCT_RESPONSE, so one of those declaring a response fails too.
- a gated tool with no `confirm_message`, and an ungated one carrying it. A
  tool gates when its tier mutates or when its input declares the confirm
  argument, which is what the emitter reads: the profile-builder save reaches
  no route and still asks for confirm before it rewrites the config file. This
  check takes no exemption list: every gated tool on the surface declares its
  own sentence.
- a tool with no `tool_description`. Every tool has one, so this check takes no
  exemption list: an unlabeled tool is one a model has to guess at.
- a routed tool with no `error_message`, unless SHARED_FAILURE names it or it
  answers with a collection. A tool on that list that gained a sentence of its
  own fails too, so the list shrinks as the surface is single-sourced.
- an `error_message` placeholder naming no field of the input message, and one
  on a Meta tool, which makes no call that can fail.
- a `resource_type` outside Destroy, and one the hash-ignore table cannot
  place: the table treats an unknown type and a typo alike, so a typo would
  otherwise turn drift detection into a silent whole-state compare.
- a `success_message` placeholder naming no field of the input message, the
  response message, or one of that response's direct sub-messages.
- `retry_disabled` on a Meta tool, which reaches no route to retry.

Four ways to pass while measuring nothing are checked first: no tool declaring a
response, none declaring a description, none declaring a failure sentence (any
of which means the annotations are gone or `make proto` has not run), and a
hash-ignore table that reads back empty. That table is read from every language
registered in docs/contracts/languages.txt, which must agree on it; a registered
language with no reader fails by name.

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

# The tiers that gate on confirm by virtue of mutating a Linode resource. A
# tool outside them gates only when its input asks for confirm.
GATED = frozenset(
    {"TOOL_CAPABILITY_WRITE", "TOOL_CAPABILITY_ADMIN", "TOOL_CAPABILITY_DESTROY"}
)
# The system argument a tool takes when the caller has to confirm before it
# acts, and the location it is always annotated with.
CONFIRM_ARGUMENT = "confirm"
LOCAL_LOCATION = "FIELD_LOCATION_LOCAL"
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

# The suffix every collection response is named with. A tool answering with one
# is on the list tier, where both clients answer a failed fetch with one shared
# sentence rather than a per-tool one: naming the tool a page came from adds
# nothing a caller does not already know. The emitter cross-checks this name
# against the message's shape, so the two spellings of "collection" cannot
# drift apart without `make proto` failing.
COLLECTION_SUFFIX = "ListResponse"

# Why a tool that answers one resource still declares no failure sentence. Each
# reason is a fact about where the sentence it answers with is built, read off
# the handlers rather than assumed.
_SHARED = (
    "its handler builds no failure sentence of its own; the one it answers with"
    " comes from a helper several tools share"
)
_UNBOUND = (
    "its sentence names a value the input message has no field for, so binding"
    " it would put a name in the contract that nothing fills"
)

# Routed tools with no per-tool failure sentence, by name and reason. This list
# only SHRINKS: an entry goes away when that tool's sentence becomes its own,
# and the gate refuses a tool that is on the list and declares one anyway.
SHARED_FAILURE = {
    "linode_database_mysql_instance_delete": _SHARED,
    "linode_database_postgresql_instance_delete": _SHARED,
    "linode_domain_delete": _SHARED,
    "linode_domain_record_delete": _SHARED,
    "linode_firewall_delete": _SHARED,
    "linode_firewall_device_delete": _SHARED,
    "linode_image_delete": _SHARED,
    "linode_image_sharegroup_delete": _SHARED,
    "linode_image_sharegroup_image_delete": _SHARED,
    "linode_image_sharegroup_member_token_delete": _SHARED,
    "linode_image_sharegroup_token_delete": _SHARED,
    "linode_instance_backups_cancel": _SHARED,
    "linode_instance_delete": _SHARED,
    "linode_instance_disk_delete": _SHARED,
    "linode_instance_ip_delete": _SHARED,
    "linode_instance_password_reset": _SHARED,
    "linode_instance_rebuild": _SHARED,
    "linode_ipv6_range_delete": _SHARED,
    "linode_lke_cluster_delete": _SHARED,
    "linode_lke_cluster_recycle": _SHARED,
    "linode_lke_cluster_regenerate": _SHARED,
    "linode_lke_kubeconfig_delete": _SHARED,
    "linode_lke_node_delete": _SHARED,
    "linode_lke_node_recycle": _SHARED,
    "linode_lke_pool_delete": _SHARED,
    "linode_lke_pool_recycle": _SHARED,
    "linode_lke_service_token_delete": _SHARED,
    "linode_monitor_service_alert_definition_delete": _SHARED,
    "linode_networking_reserved_ip_delete": _SHARED,
    "linode_nodebalancer_delete": _SHARED,
    "linode_object_storage_bucket_delete": _SHARED,
    "linode_object_storage_key_delete": _SHARED,
    "linode_object_storage_ssl_delete": _SHARED,
    "linode_placement_group_delete": _SHARED,
    "linode_sshkey_delete": _SHARED,
    "linode_stackscript_delete": _SHARED,
    "linode_tag_delete": _SHARED,
    "linode_vlan_delete": _SHARED,
    "linode_volume_delete": _SHARED,
    "linode_vpc_delete": _SHARED,
    "linode_vpc_subnet_delete": _SHARED,
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

# A placeholder can carry a form after the colon, a zero-padded width
# ({month:02}) or a repeated field's entry count ({linodes:len}), which both
# renderer arms and the Python driver render. The name is what this gate checks, so
# the form is matched and discarded rather than read as part of the name.
_PLACEHOLDER = re.compile(r"\{([a-z0-9_]+)(?::(?:0\d+|len))?\}")

# The response member a declared warning_message is reported in. Every mutation
# that carries a notice calls it this, which is what binds the two.
_WARNING_FIELD = "warning"


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


def confirm_argument_messages() -> set[str]:
    """Input messages that ask the caller to confirm before the tool acts.

    Read from the field annotations rather than the tier, because the tier
    cannot say it for a tool that reaches no route: a Meta tool that rewrites
    local state gates on the same argument a mutation does, and both renderer
    arms key on this same field.
    """
    return {
        message
        for message, fields in _toolroutes.field_locations().items()
        if fields.get(CONFIRM_ARGUMENT) == LOCAL_LOCATION
    }


def confirm_violations(
    declared: list[_toolroutes.ToolDeclaration], asks_confirm: set[str]
) -> list[str]:
    """Gated tools with no confirm prose, and ungated tools carrying some.

    asks_confirm names the input messages that take the confirm argument, which
    is what makes a tool gated when its tier does not say so.
    """
    found: list[str] = []

    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool:
            continue
        gated = entry.capability in GATED or entry.message in asks_confirm
        if gated and not entry.confirm_message:
            found.append(f"{tool}: {entry.capability} but declares no confirm_message")
        elif not gated and entry.confirm_message:
            found.append(
                f"{tool}: {entry.capability} tools do not gate on confirm, but it"
                f" declares confirm prose"
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


def warning_violations(
    declared: list[_toolroutes.ToolDeclaration], messages: dict[str, dict[str, str]]
) -> list[str]:
    """Declared warnings the tool's answer cannot carry or cannot fill.

    The notice is filled the way the success sentence beside it is, so the same
    placeholder rule applies, and a sentence with no warning field to report it
    in is dropped by the serializer.

    The other half of the pairing, a response promising a warning field while
    its tool declares no sentence, is the emitter's to refuse rather than this
    gate's. A hand-written tool fills that field in code today, so failing it
    here would report the whole un-migrated warning surface as broken; a
    generated one has nothing to fill it from, which is what the emitter stops
    on.
    """
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool or not entry.warning_message:
            continue

        if _WARNING_FIELD not in messages.get(entry.response, {}):
            named = entry.response or "its response"
            found.append(
                f"{tool}: declares a warning_message and {named} has no warning"
                " field to report it in"
            )

        unknown = sorted(
            set(_PLACEHOLDER.findall(entry.warning_message))
            - visible_fields(entry, messages)
        )
        if unknown:
            found.append(
                f"{tool}: warning_message fills {', '.join(unknown)} from no field"
                f" of its input, its response, or that response's sub-messages"
            )
    return sorted(found)


def description_violations(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Tools a client would see unlabeled.

    No exemption list: a description is the one piece of prose every tool has,
    and the message comment above it in the proto documents the input contract
    rather than the sentence a model chooses the tool by.
    """
    return sorted(
        f"{entry.route_tool or entry.meta_tool}: declares no tool_description"
        for entry in declared
        if (entry.route_tool or entry.meta_tool) and not entry.description
    )


def failure_violations(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Failure sentences that are missing, unnecessary, or fill from no field."""
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool:
            continue
        found.extend(failure_faults(entry, tool))
    return sorted(found)


def failure_faults(entry: _toolroutes.ToolDeclaration, tool: str) -> list[str]:
    """Everything wrong with one tool's failure sentence."""
    if not entry.route_tool:
        if entry.error_message:
            return [
                (
                    f"{tool}: Meta tools reach no Linode route, so there is no"
                    f" call whose failure error_message could describe"
                )
            ]
        return []

    listed = tool in SHARED_FAILURE
    collection = entry.response.endswith(COLLECTION_SUFFIX)

    if not entry.error_message:
        if listed or collection:
            return []
        return [f"{tool}: declares no error_message"]

    if listed:
        return [
            (
                f"{tool}: listed as answering with a shared sentence"
                f" ({SHARED_FAILURE[tool]}), but it declares one of its own"
            )
        ]
    return []


def failure_placeholder_violations(
    declared: list[_toolroutes.ToolDeclaration], messages: dict[str, dict[str, str]]
) -> list[str]:
    """Placeholders a failed call has nothing to fill from.

    Only the input message is in scope, unlike success_message: a call that
    failed produced no response to read a value out of, so {error} and the
    arguments the caller passed are all the sentence can name.
    """
    found: list[str] = []
    for entry in declared:
        tool = entry.route_tool or entry.meta_tool
        if not tool or not entry.error_message:
            continue
        visible = set(messages.get(entry.message, {})) | {"error"}
        unknown = sorted(set(_PLACEHOLDER.findall(entry.error_message)) - visible)
        if unknown:
            found.append(
                f"{tool}: error_message fills {', '.join(unknown)} from no field"
                f" of its input"
            )
    return sorted(found)


def unlisted_shared_failures(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Names on SHARED_FAILURE that no tool answers to.

    An entry naming nothing exempts nothing while reading as though it does,
    which is the shape a typo or a renamed tool takes.
    """
    known = {entry.route_tool for entry in declared if entry.route_tool}
    return sorted(
        f"{tool}: SHARED_FAILURE names it, but no routed tool is called that"
        for tool in SHARED_FAILURE
        if tool not in known
    )


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

    unread = [
        option
        for option, values in (
            ("tool_response", [entry.response for entry in declared]),
            ("tool_description", [entry.description for entry in declared]),
            ("error_message", [entry.error_message for entry in declared]),
        )
        if not any(values)
    ]
    if unread:
        _report(
            "options this gate reads that no tool declares:",
            unread,
            "they are gone, or `make proto` has not run; either way the"
            " comparison below would measure nothing",
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
            confirm_violations(declared, confirm_argument_messages()),
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
            warning_violations(declared, messages),
            "warnings the tool's answer cannot carry or cannot fill:",
            (
                "declare warning_message only on a tool whose response carries a"
                " warning field, and name a field it can fill from"
            ),
        ),
        (
            retry_violations(declared),
            "tools marked unretryable that make no request:",
            "drop retry_disabled from the Meta tool's input",
        ),
        (
            description_violations(declared),
            "tools a client would see unlabeled:",
            (
                "add `option (linode.mcp.v1.tool_description)` carrying the"
                " sentence the tool advertises, then run `make proto`"
            ),
        ),
        (
            failure_violations(declared) + unlisted_shared_failures(declared),
            "failure sentences that do not match how the tool answers:",
            (
                "declare error_message, or name the tool in SHARED_FAILURE in"
                " this script with the reason its sentence is not per-tool"
            ),
        ),
        (
            failure_placeholder_violations(declared, messages),
            "failure sentences that fill a placeholder from no field:",
            (
                "a failed call has no response to read from, so name a field of"
                " the input message or {error}"
            ),
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
    descriptions = sum(1 for entry in declared if entry.description)
    failures = sum(1 for entry in declared if entry.error_message)
    print(
        f"tool-response gate OK: {responses} response(s) over {len(declared)} tool(s)"
        f" plus {len(STRUCT_RESPONSE)} free-form, {descriptions} description(s),"
        f" {confirms} confirm message(s), {successes} success message(s),"
        f" {failures} failure sentence(s) plus {len(SHARED_FAILURE)} answering"
        f" with a shared one, {types} resource type(s) against"
        f" {len(keys)} table entr(ies), {unretryable} unretryable call(s)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
