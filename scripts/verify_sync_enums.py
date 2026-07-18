#!/usr/bin/env python3
"""Live sync gate: verify every MCP proto enum matches the live Linode API spec.

This is the scheduled-agent's drift detector, NOT part of the offline `make
check` (it needs network). It fetches the live OpenAPI spec, reads each proto
enum's value set straight from the .proto files, and diffs the two. A difference
means the API changed a value (added / removed / renamed) and the proto enum is
now stale, so a human/agent must reconcile it (update the proto, regen, fixture)
and only then move the baseline.

Two properties the user required:
  * LIVE, not vendored: the spec is fetched every run from the active repo
    (linode/linode-api-openapi); the archived linode-api-docs must never be used,
    it is frozen. There is no checked-in spec copy to rot.
  * STALENESS TRIPWIRE: even the live spec repo lags the changelog, so a passing
    diff does not by itself prove "code matches the current API". The gate also
    compares the latest commit changing openapi.json against the changelog's
    newest entry and fails closed when the source predates a release, so a green
    run is never mistaken for fully current.

It also checks the hand-maintained validation lists that cannot be proto enums
(hyphen/colon values, or map keys) the same way: it reads each hand-list from
source (Go via cmd/hand-list-dump, Python via ast) and diffs it against the
same live spec, folding the result into the same baseline.

Usage: verify_sync_enums.py [--spec PATH] [--go-lists PATH] [--update-baseline]
  --spec PATH        read the spec from a local file instead of fetching
                     (CI/offline test)
  --go-lists PATH    read the Go hand-lists from a JSON file instead of running
                     cmd/hand-list-dump (CI/offline test)
  --update-baseline  rewrite docs/contracts/enum-sync-baseline.txt from the current diff
"""

from __future__ import annotations

import ast
import json
import re
import subprocess
import sys
import urllib.request
from datetime import UTC, date, datetime
from html.parser import HTMLParser
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

REPO_ROOT = Path(__file__).resolve().parent.parent
PROTO_DIR = REPO_ROOT / "proto" / "linode" / "mcp" / "v1"
GO_DIR = REPO_ROOT / "go"
BASELINE = REPO_ROOT / "docs" / "contracts" / "enum-sync-baseline.txt"

SPEC_URL = (
    "https://raw.githubusercontent.com/linode/linode-api-openapi/main/openapi.json"
)
CHANGELOG_URL = "https://techdocs.akamai.com/linode-api/changelog"
SPEC_COMMITS_URL = (
    "https://api.github.com/repos/linode/linode-api-openapi/commits"
    "?path=openapi.json&per_page=1"
)

_CHANGELOG_ENTRY_PATH = re.compile(r"/linode-api/changelog/[^/]+/?$", re.IGNORECASE)
_CHANGELOG_PATH_DATE = re.compile(
    r"/linode-api/changelog/([a-z]+)-(\d{1,2})-(\d{4})(?:-[^/]+)?/?$",
    re.IGNORECASE,
)
_CHANGELOG_TEXT_DATE = re.compile(r"\b([a-z]+)\s+(\d{1,2}),\s*(\d{4})\b", re.IGNORECASE)
_MONTH_NAMES = (
    "january",
    "february",
    "march",
    "april",
    "may",
    "june",
    "july",
    "august",
    "september",
    "october",
    "november",
    "december",
)
_MONTH_NUMBERS = {
    alias: number
    for number, name in enumerate(_MONTH_NAMES, start=1)
    for alias in (name, name[:3])
}

# proto enum message name -> how to find its value set in the OpenAPI spec.
# ("<field>", "<path substring>") extracts that field's enum from request bodies
# on matching endpoints (unioned across oneOf variants and endpoints).
# "TOOL_DEFINED" marks an enum whose values are the MCP tool's own contract, not
# an API request field (audit export format, S3 presign method); the API side
# cannot be checked, so it is asserted stable against the baseline only.
ENUM_SPEC_MAP: dict[str, tuple[str, str] | str] = {
    "NodeBalancerProtocol": ("protocol", "/nodebalancers"),
    "NodeBalancerAlgorithm": ("algorithm", "/nodebalancers"),
    "NodeBalancerStickiness": ("stickiness", "/nodebalancers"),
    "NodeBalancerCheck": ("check", "/nodebalancers"),
    "NodeBalancerCipherSuite": ("cipher_suite", "/nodebalancers"),
    "NodeBalancerProxyProtocol": ("proxy_protocol", "/nodebalancers"),
    "NodeBalancerNodeMode": ("mode", "/nodebalancers"),
    "ManagedServiceType": ("service_type", "/managed/services"),
    "ConfigInterfacePurpose": ("purpose", "/linode/instances"),
    "ObjectStorageKeyPermission": ("permissions", "/object-storage/keys"),
    "PlacementGroupPolicy": ("placement_group_policy", "/placement/groups"),
    "ConfigRunLevel": ("run_level", "/configs"),
    "ConfigVirtMode": ("virt_mode", "/configs"),
    "FirewallPolicy": ("inbound_policy", "/networking/firewalls"),
    "FirewallDeviceType": ("type", "/networking/firewalls/"),
    "LKETier": ("tier", "/lke/clusters"),
    # FirewallTemplateSlug: the API declares slug as a free-form path parameter
    # with no OpenAPI enum, so the closed set is the MCP tool's own contract.
    "FirewallTemplateSlug": "TOOL_DEFINED",
    "InstanceIPType": ("type", "/networking/ips"),
    "GrantPermission": ("permissions", "/account/users"),
    # AuditExportFormat (json/csv/ndjson) and PresignedURLMethod (GET/PUT) are the
    # MCP tools' own contracts (local export format, S3 presign verb), not API
    # request fields, so there is no spec enum to check them against.
    "AuditExportFormat": "TOOL_DEFINED",
    "PresignedURLMethod": "TOOL_DEFINED",
}

ENUM_SENTINEL = "unspecified"
# message <Name> { enum Value { <body> } }, the enum-wrapper convention.
_ENUM_BLOCK = re.compile(
    r"message\s+(\w+)\s*\{\s*enum\s+Value\s*\{([^}]*)\}", re.MULTILINE
)
_ENUM_VALUE = re.compile(r"^\s*(\w+)\s*=\s*\d+\s*;", re.MULTILINE)


def proto_enums() -> dict[str, set[str]]:
    """Return {enum message name: {api value, ...}} from the .proto sources."""
    out: dict[str, set[str]] = {}
    for path in sorted(PROTO_DIR.glob("*.proto")):
        text = path.read_text(encoding="utf-8")
        for name, body in _ENUM_BLOCK.findall(text):
            values = {v for v in _ENUM_VALUE.findall(body) if v != ENUM_SENTINEL}
            if values:
                out[name] = values
    return out


def _resolve(doc: dict[str, Any], ref: str) -> Any:
    node: Any = doc
    for part in ref.lstrip("#/").split("/"):
        node = node[part]
    return node


def _walk(
    doc: dict[str, Any],
    schema: Any,
    field: str,
    out: set[str],
    depth: int,
    prop: str | None,
) -> None:
    if depth > 12 or not isinstance(schema, dict):
        return
    ref = schema.get("$ref")
    if isinstance(ref, str):
        _walk(doc, _resolve(doc, ref), field, out, depth + 1, prop)
        return
    if prop == field and isinstance(schema.get("enum"), list):
        out.update(str(v) for v in schema["enum"])
    for comb in ("oneOf", "anyOf", "allOf"):
        for sub in schema.get(comb, []):
            _walk(doc, sub, field, out, depth + 1, prop)
    props = schema.get("properties")
    if isinstance(props, dict):
        for name, sub in props.items():
            _walk(doc, sub, field, out, depth + 1, name)
    item = schema.get("items")
    if isinstance(item, dict):
        _walk(doc, item, field, out, depth + 1, prop)


def spec_enum(doc: dict[str, Any], field: str, path_substr: str) -> set[str]:
    """Union a field's request-body enum across matching endpoints and oneOf
    branches."""
    out: set[str] = set()
    for path, ops in doc.get("paths", {}).items():
        if path_substr not in path:
            continue
        for method, op in ops.items():
            if method not in ("post", "put", "patch") or not isinstance(op, dict):
                continue
            body = op.get("requestBody")
            if isinstance(body, dict):
                for media in body.get("content", {}).values():
                    if media.get("schema"):
                        _walk(doc, media["schema"], field, out, 0, None)
    return out


def load_spec(spec_path: str | None) -> dict[str, Any]:
    if spec_path:
        return json.loads(Path(spec_path).read_text(encoding="utf-8"))
    with urllib.request.urlopen(SPEC_URL, timeout=60) as resp:  # noqa: S310 - fixed HTTPS URL
        return json.loads(resp.read().decode("utf-8"))


class StalenessVerificationError(RuntimeError):
    """Raised when live changelog or OpenAPI source verification is inconclusive."""


class _ChangelogLinkParser(HTMLParser):
    """Collect changelog links and whether each is explicitly marked scheduled."""

    def __init__(self) -> None:
        super().__init__()
        self.links: list[tuple[str, str, bool]] = []
        self._href: str | None = None
        self._marker_parts: list[str] = []
        self._scheduled = False

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag.casefold() != "a" or self._href is not None:
            return
        href = next((value for name, value in attrs if name == "href"), None)
        if href is None:
            return
        self._href = href
        self._marker_parts = [
            value
            for name, value in attrs
            if name.casefold() in {"aria-label", "title"} and value
        ]
        self._scheduled = any(
            name.casefold() in {"data-release-status", "data-status"}
            and value is not None
            and value.strip().casefold() == "scheduled"
            for name, value in attrs
        )

    def handle_data(self, data: str) -> None:
        if self._href is not None:
            self._marker_parts.append(data)

    def handle_endtag(self, tag: str) -> None:
        if tag.casefold() != "a" or self._href is None:
            return
        marker = " ".join(self._marker_parts).strip().casefold()
        text_scheduled = marker.startswith(("scheduled:", "scheduled release"))
        self.links.append((self._href, marker, self._scheduled or text_scheduled))
        self._href = None
        self._marker_parts = []
        self._scheduled = False


def changelog_release_dates(
    html: str,
    *,
    today: date | None = None,
    include_scheduled_future: bool = True,
) -> list[date]:
    """Return valid Linode API changelog-entry dates, newest first.

    Dates elsewhere in the TechDocs page are ignored. Future entry links are
    ignored unless the link is explicitly marked as scheduled.
    """
    parser = _ChangelogLinkParser()
    parser.feed(html)
    current_date = today or datetime.now(tz=UTC).date()
    releases: set[date] = set()
    for href, marker, scheduled in parser.links:
        path = urlparse(href).path
        if _CHANGELOG_ENTRY_PATH.fullmatch(path) is None:
            continue
        match = _CHANGELOG_PATH_DATE.fullmatch(path) or _CHANGELOG_TEXT_DATE.search(
            marker
        )
        if match is None:
            continue
        month_name, day_text, year_text = match.groups()
        month = _MONTH_NUMBERS.get(month_name.casefold())
        if month is None:
            raise ValueError(f"invalid changelog entry month in {href!r}")
        try:
            release = date(int(year_text), month, int(day_text))
        except ValueError as exc:
            raise ValueError(f"invalid changelog entry date in {href!r}") from exc
        if release <= current_date or (include_scheduled_future and scheduled):
            releases.add(release)
    if not releases:
        raise ValueError("no valid Linode API changelog entry dates found")
    return sorted(releases, reverse=True)


def _fetch_staleness_source(url: str, label: str) -> str:
    request = urllib.request.Request(  # noqa: S310 - fixed HTTPS URLs
        url,
        headers={
            "Accept": "application/vnd.github+json, text/html",
            "User-Agent": "LinodeMCP-sync-enums",
        },
    )
    try:
        with urllib.request.urlopen(  # noqa: S310 - fixed HTTPS URLs
            request, timeout=60
        ) as response:
            return response.read().decode("utf-8")
    except (OSError, UnicodeError) as exc:
        raise StalenessVerificationError(f"{label} fetch failed: {exc}") from exc


def _latest_openapi_commit_date(payload: str) -> date:
    try:
        commits = json.loads(payload)
        timestamp = commits[0]["commit"]["committer"]["date"]
    except (IndexError, KeyError, TypeError, ValueError) as exc:
        raise StalenessVerificationError(
            f"OpenAPI commit verification failed: {exc}"
        ) from exc
    if (
        not isinstance(timestamp, str)
        or re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", timestamp) is None
    ):
        raise StalenessVerificationError(
            "OpenAPI commit verification failed: missing commit timestamp"
        )
    try:
        return date.fromisoformat(timestamp[:10])
    except ValueError as exc:
        raise StalenessVerificationError(
            f"OpenAPI commit verification failed: {exc}"
        ) from exc


def staleness_note(doc: dict[str, Any], *, today: date | None = None) -> str:
    """Compare the latest changelog release with the OpenAPI source commit."""
    info = doc.get("info")
    version = info.get("version") if isinstance(info, dict) else None
    if not isinstance(version, str) or not version.strip():
        raise StalenessVerificationError("OpenAPI info.version verification failed")
    version = version.strip()
    html = _fetch_staleness_source(CHANGELOG_URL, "changelog")
    try:
        newest = changelog_release_dates(
            html, today=today, include_scheduled_future=False
        )[0]
    except ValueError as exc:
        raise StalenessVerificationError(f"changelog parsing failed: {exc}") from exc
    commits = _fetch_staleness_source(SPEC_COMMITS_URL, "OpenAPI commits")
    commit_date = _latest_openapi_commit_date(commits)
    if newest >= commit_date:
        raise StalenessVerificationError(
            "OpenAPI release coverage cannot be verified: newest changelog entry "
            f"{newest} is newer than latest openapi.json commit {commit_date}"
        )
    return (
        f"spec version {version}; latest openapi.json commit {commit_date}; "
        f"newest changelog entry {newest} (release coverage verified)"
    )


def read_baseline() -> set[str]:
    if not BASELINE.exists():
        return set()
    return {
        line.strip()
        for line in BASELINE.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.startswith("#")
    }


# Hand-maintained validation value-sets that CANNOT become proto enums: their
# values are not valid proto identifiers (hyphens, colons) or they are map keys
# rather than a scalar field. They stay as hand-lists in both languages, so this
# gate reads each hand-list straight from source (Go via cmd/hand-list-dump,
# Python via ast) and diffs it against the same live spec the enum gate uses.
# The diffs fold into the same baseline.
#
# Each entry:
#   "spec": (mode, field, path_substr)
#       "field-enum"   -> the request-body enum of <field> (same as proto enums)
#       "object-props" -> the property NAMES of the object-typed <field>
#   "spec_exclude": values the API lists but the hand-list intentionally omits
#   "py": (kind, symbol, rel_path) for the Python hand-list, or None when Python
#         does not validate this today (a tracked parity gap)
# The Go side is keyed by the same logical name in cmd/hand-list-dump's output.
HAND_LIST_SPEC_MAP: dict[str, dict[str, Any]] = {
    "bucket_acl": {
        "spec": ("field-enum", "acl", "/object-storage/buckets"),
        # The access and object-acl endpoints list a 5th value, "custom", in
        # their request enum, but it is a read-back/display state, never a
        # settable input: linodego has no ACLCustom constant, and the Akamai
        # docs say Cloud Manager only DISPLAYS "custom" when a bucket carries
        # non-canned S3 grants. Both languages accept only the 4 canned ACLs on
        # input, so the gate drops "custom" from the spec side. A genuinely new
        # canned value would still trip the diff.
        "spec_exclude": {"custom"},
        "py": (
            "set",
            "_VALID_ACLS",
            "python/src/linodemcp/tools/linode_object_storage_write.py",
        ),
    },
    "placement_group_type": {
        "spec": ("field-enum", "placement_group_type", "/placement/groups"),
        "py": (
            "set",
            "_PLACEMENT_GROUP_TYPES",
            "python/src/linodemcp/tools/linode_placement_groups_write.py",
        ),
    },
    "config_device_slot": {
        # sda-sdh are the property NAMES of the config request's "devices"
        # object, not a scalar field enum, so proto cannot gate them.
        "spec": ("object-props", "devices", "/configs"),
        "py": (
            "set",
            "_VALID_DEVICE_SLOTS",
            "python/src/linodemcp/tools/linode_instance_disks.py",
        ),
    },
}


def _properties(doc: dict[str, Any], schema: Any, depth: int = 0) -> dict[str, Any]:
    """Merge a schema's property map, resolving $ref and allOf/oneOf/anyOf."""
    if depth > 12 or not isinstance(schema, dict):
        return {}
    ref = schema.get("$ref")
    if isinstance(ref, str):
        return _properties(doc, _resolve(doc, ref), depth + 1)
    props: dict[str, Any] = {}
    own = schema.get("properties")
    if isinstance(own, dict):
        props.update(own)
    for comb in ("allOf", "oneOf", "anyOf"):
        for sub in schema.get(comb, []):
            props.update(_properties(doc, sub, depth + 1))
    return props


def spec_object_props(doc: dict[str, Any], field: str, path_substr: str) -> set[str]:
    """Union the property names of an object-typed request field across endpoints."""
    out: set[str] = set()
    for path, ops in doc.get("paths", {}).items():
        if path_substr not in path or not isinstance(ops, dict):
            continue
        for method, op in ops.items():
            if method not in ("post", "put", "patch") or not isinstance(op, dict):
                continue
            body = op.get("requestBody")
            if not isinstance(body, dict):
                continue
            for media in body.get("content", {}).values():
                schema = media.get("schema") if isinstance(media, dict) else None
                if not schema:
                    continue
                field_schema = _properties(doc, schema).get(field)
                if field_schema is not None:
                    out.update(_properties(doc, field_schema).keys())
    return out


def _string_members(value: ast.expr) -> set[str]:
    """Collect string constants of a set/list/tuple literal or set()/frozenset()
    call."""
    elts: list[ast.expr] = []
    if isinstance(value, (ast.Set, ast.List, ast.Tuple)):
        elts = list(value.elts)
    elif (
        isinstance(value, ast.Call)
        and isinstance(value.func, ast.Name)
        and value.func.id in ("set", "frozenset")
        and value.args
        and isinstance(value.args[0], (ast.Set, ast.List, ast.Tuple))
    ):
        elts = list(value.args[0].elts)
    return {
        e.value
        for e in elts
        if isinstance(e, ast.Constant) and isinstance(e.value, str)
    }


def python_hand_list(rel_path: str, name: str) -> set[str]:
    """Extract the string members of a module-level set assigned to `name`.

    Raises when the name is absent or has no string members so a renamed
    constant trips the gate loudly instead of vanishing into a false green.
    """
    path = REPO_ROOT / rel_path
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    for node in ast.walk(tree):
        if isinstance(node, ast.Assign):
            targets, value = node.targets, node.value
        elif isinstance(node, ast.AnnAssign) and node.value is not None:
            targets, value = [node.target], node.value
        else:
            continue
        if any(isinstance(t, ast.Name) and t.id == name for t in targets):
            members = _string_members(value)
            if not members:
                raise ValueError(f"{name} in {rel_path}: no string members found")
            return members
    raise ValueError(f"{name} not found in {rel_path}")


def go_hand_lists(go_lists_path: str | None) -> dict[str, set[str]]:
    """Return the Go hand-list value-sets, from a JSON file or cmd/hand-list-dump.

    A non-zero exit from the extractor (a renamed or missing symbol) raises, so
    the gate fails loudly rather than skipping the Go side.
    """
    if go_lists_path:
        raw = json.loads(Path(go_lists_path).read_text(encoding="utf-8"))
    else:
        proc = subprocess.run(
            ["go", "run", "./cmd/hand-list-dump"],
            cwd=GO_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
        if proc.returncode != 0:
            raise RuntimeError(
                f"cmd/hand-list-dump failed (exit {proc.returncode}): "
                f"{proc.stderr.strip()}"
            )
        raw = json.loads(proc.stdout)
    return {str(k): {str(v) for v in vals} for k, vals in raw.items()}


def hand_list_diffs(doc: dict[str, Any], go_lists: dict[str, set[str]]) -> list[str]:
    """Diff every hand-list (Go and Python) against the live spec value-set."""
    diffs: list[str] = []
    for key, spec in HAND_LIST_SPEC_MAP.items():
        mode, field, path_substr = spec["spec"]
        exclude: set[str] = spec.get("spec_exclude", set())
        if mode == "field-enum":
            spec_vals = spec_enum(doc, field, path_substr) - exclude
        elif mode == "object-props":
            spec_vals = spec_object_props(doc, field, path_substr) - exclude
        else:
            diffs.append(f"{key}: unknown spec mode {mode!r}")
            continue
        if not spec_vals:
            diffs.append(f"{key}: spec field {field!r} not found under {path_substr!r}")
            continue

        go_vals = go_lists.get(key)
        if not go_vals:
            diffs.append(f"{key}: go hand-list empty or missing (renamed symbol?)")
        else:
            go_missing = sorted(spec_vals - go_vals)
            go_extra = sorted(go_vals - spec_vals)
            if go_missing:
                diffs.append(f"{key}: go hand-list missing API value(s): {go_missing}")
            if go_extra:
                diffs.append(f"{key}: go hand-list has value(s) not in API: {go_extra}")

        py_spec = spec["py"]
        if py_spec is None:
            diffs.append(
                f"{key}: python has no hand-list "
                "(known parity gap; Go validates, Python does not)"
            )
            continue
        _, py_symbol, py_path = py_spec
        try:
            py_vals = python_hand_list(py_path, py_symbol)
        except (OSError, ValueError) as exc:
            diffs.append(f"{key}: python extraction failed: {exc}")
            continue
        py_missing = sorted(spec_vals - py_vals)
        py_extra = sorted(py_vals - spec_vals)
        if py_missing:
            diffs.append(f"{key}: python hand-list missing API value(s): {py_missing}")
        if py_extra:
            diffs.append(f"{key}: python hand-list has value(s) not in API: {py_extra}")
        if go_vals and go_vals != py_vals:
            diffs.append(
                f"{key}: go and python hand-lists differ "
                f"(go-only={sorted(go_vals - py_vals)}, "
                f"py-only={sorted(py_vals - go_vals)})"
            )
    return diffs


def main(argv: list[str]) -> int:
    spec_path = None
    go_lists_path = None
    update = "--update-baseline" in argv
    if "--spec" in argv:
        spec_path = argv[argv.index("--spec") + 1]
    if "--go-lists" in argv:
        go_lists_path = argv[argv.index("--go-lists") + 1]

    enums = proto_enums()
    doc = load_spec(spec_path)

    diffs: list[str] = []
    mapped = set(ENUM_SPEC_MAP)
    for name, values in sorted(enums.items()):
        mapping = ENUM_SPEC_MAP.get(name)
        if mapping is None:
            diffs.append(
                f"{name}: proto enum has no ENUM_SPEC_MAP entry (add a mapping)"
            )
            continue
        if isinstance(mapping, str):
            # "TOOL_DEFINED": values are the MCP tool's contract, no API field to check.
            continue
        field, path_substr = mapping
        api = spec_enum(doc, field, path_substr)
        if not api:
            diffs.append(
                f"{name}: field {field!r} not found in spec under {path_substr!r}"
            )
            continue
        missing = api - values
        extra = values - api
        if missing:
            diffs.append(f"{name}: proto missing API value(s): {sorted(missing)}")
        if extra:
            diffs.append(f"{name}: proto has value(s) not in API: {sorted(extra)}")

    diffs.extend(
        f"{name}: mapped but no such proto enum (stale ENUM_SPEC_MAP entry)"
        for name in mapped - set(enums)
    )

    try:
        go_lists = go_hand_lists(go_lists_path)
    except (OSError, RuntimeError, json.JSONDecodeError) as exc:
        print(f"sync-enums: hand-list extraction failed: {exc}", file=sys.stderr)
        return 1
    diffs.extend(hand_list_diffs(doc, go_lists))

    # The changelog fetch is a live-only staleness tripwire; --spec means an
    # offline run (CI/tests), so skip the network call and stay hermetic. Verify
    # live source freshness before either comparison or baseline mutation.
    if spec_path is None:
        try:
            note = staleness_note(doc)
        except StalenessVerificationError as exc:
            print(
                f"sync-enums: staleness verification failed: {exc}",
                file=sys.stderr,
            )
            return 1
        print(f"sync-enums: {note}", file=sys.stderr)

    if update:
        BASELINE.parent.mkdir(parents=True, exist_ok=True)
        version = doc.get("info", {}).get("version", "unknown")
        header = (
            f"# enum-sync drift baseline (reviewed at linode-api-openapi {version})\n"
        )
        BASELINE.write_text(header + "".join(f"{d}\n" for d in diffs), encoding="utf-8")
        print(f"wrote {len(diffs)} baseline line(s)", file=sys.stderr)
        return 0
    baseline = read_baseline()
    new = [d for d in diffs if d not in baseline]
    fixed = baseline - set(diffs)
    if new:
        print(
            "enum drift vs live API "
            "(reconcile proto, regen, fixture, then rebaseline):",
            file=sys.stderr,
        )
        for d in new:
            print(f"  DRIFT {d}", file=sys.stderr)
    if fixed:
        print(
            "baseline entries no longer drifting (rebaseline to shrink):",
            file=sys.stderr,
        )
        for d in sorted(fixed):
            print(f"  FIXED {d}", file=sys.stderr)
    if new or fixed:
        return 1
    print(
        f"sync-enums OK: {len(enums)} proto enum(s) + "
        f"{len(HAND_LIST_SPEC_MAP)} hand-list(s) match the live API",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
