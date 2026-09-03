#!/usr/bin/env python3
"""Rendered-TechDocs to LinodeMCP-proto contract comparison.

Read-only by construction. It never imports the LinodeMCP Go or Python
implementations, never writes into the repository, and never opens issues or
schedules anything. Buf compiles proto source into a JSON FileDescriptorSet,
and that descriptor data is compared against the contract parsed solely from
rendered TechDocs pages. Tool name, HTTP method, and path come only from the
protobuf ``linode.mcp.v1.tool_route`` message option. A missing or malformed
option fails closed; there is no route-map fallback. API parameter location is
matched only when a normalized name is unique on the TechDocs operation;
ambiguous names are reported.

With no arguments it fetches rendered TechDocs, generates a candidate TechDocs
proto and both descriptor sets, and builds the repo descriptor from the working
tree this file lives in. ``--github-source`` swaps that for an immutable GitHub
archive resolved with ``gh``, which is what a run needs when it must name a
commit. Evidence lands under a host-local directory outside the repository
(see ``evidence_root``), as dated runs plus a ``latest.json`` handoff, swept by
``--retention-days``.

Scraping needs the network and therefore runs only from a schedule or by hand.
``--self-test`` is the offline arm, and it is what ``make techdocs-proof``
runs inside ``make check``.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import contextlib
import hashlib
import html
import inspect
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from datetime import UTC, date, datetime
from pathlib import Path
from typing import TYPE_CHECKING, Any, TypeIs

if TYPE_CHECKING:
    from collections.abc import Generator, Mapping

TECHDOCS_AUTHORITY = "techdocs-rendered-pages"
# JSON Schema spells a masked string field "password". It is a documented
# format name and never a value, but a secret scanner reads the literal
# table row as a hardcoded credential, so the spelling is named once here.
MASKED_STRING_FORMAT = "password"
SYSTEM_PARAMETER_MARKER = "System parameter:"
# LinodeMCP marks server-injected parameters with a TRAILING proto comment
# (bool dry_run = 19; // system param). Trailing is deliberate there: a
# leading comment becomes the user-facing JSON Schema description, so the
# marker would leak into every tool description. Both spellings are accepted
# so a system parameter is never reported as missing from TechDocs.
SYSTEM_PARAMETER_MARKERS = (SYSTEM_PARAMETER_MARKER.lower(), "system param")


def is_system_parameter(comment: str) -> bool:
    """True when a field comment marks the parameter as MCP-internal."""
    text = comment.strip().lower()
    return any(text.startswith(marker) for marker in SYSTEM_PARAMETER_MARKERS)


INVESTIGATION_GUIDANCE = (
    "This proto parameter is not present in the rendered TechDocs contract. "
    "Investigate whether it is an internal MCP/system parameter. If it is "
    "internal, add a proto field comment beginning with `System parameter:` "
    "(or the trailing `// system param` marker LinodeMCP uses) "
    "that explains its purpose and confirms it is not sent to the Linode API. "
    "Otherwise, reconcile the proto field with the rendered TechDocs contract."
)

DEFAULT_GITHUB_REPOSITORY = "chadit/LinodeMCP"
DEFAULT_GITHUB_REF = "main"
# <repo>/tools/techdocs-proof/src/techdocs_proof/proof.py, so the tool project
# is two parents up and the repository two above that.
PACKAGE_ROOT = Path(__file__).resolve().parents[2]
REPO_ROOT = PACKAGE_ROOT.parents[1]
LEDGER_PATH = PACKAGE_ROOT / "data" / "known-divergences.json"
# Run evidence stays out of the repository so a checkout is never dirtied by a
# comparator run and any host can invoke the repo command directly (REQ-D10).
EVIDENCE_ROOT_ENV = "LINODEMCP_TECHDOCS_PROOF_ROOT"
EVIDENCE_ROOT_SUFFIX = Path("linodemcp") / "techdocs-proof"
DOCS_ROOT = "https://techdocs.akamai.com/linode-api/reference/api"
SITEMAP_URL = "https://techdocs.akamai.com/sitemap.xml"
USER_AGENT = "LinodeMCP-TechDocs-Proto-Proof/1.0"
TOOL_ROUTE_OPTION = "[linode.mcp.v1.tool_route]"
TOOL_ROUTE_EXTENSION = ".linode.mcp.v1.tool_route"
TOOL_API_SURFACE_OPTION = "[linode.mcp.v1.tool_api_surface]"
TOOL_API_SURFACE_EXTENSION_NAME = "tool_api_surface"
TOOL_API_SURFACE_EXTENSION_NUMBER = 50025
# LinodeMCP omits the option on every tool that calls the default surface, so an
# absent or unspecified value means v4 and not "unknown".
API_SURFACE_VALUES = {
    "": "v4",
    "API_SURFACE_UNSPECIFIED": "v4",
    "API_SURFACE_V4": "v4",
    "API_SURFACE_V4BETA": "v4beta",
}
# How a documented route answers the apiVersion path parameter. Kept separate
# from the single api_version string because a route that accepts both surfaces
# used to flatten into v4 and hide every legitimate v4beta tool declaration.
SURFACE_V4_ONLY = "v4_only"
SURFACE_V4BETA_ONLY = "v4beta_only"
SURFACE_BOTH = "both"
API_SURFACE_CLASSIFICATIONS = (SURFACE_V4_ONLY, SURFACE_V4BETA_ONLY, SURFACE_BOTH)
FIELD_LOCATION_OPTION = "[linode.mcp.v1.field_location]"
# The BODY string argument whose wire member is the array of its
# comma-separated segments, which is the type the reference page documents.
BODY_COMMA_LIST_OPTION = "[linode.mcp.v1.body_comma_list]"
# LinodeMCP names where an input field travels in the request it builds. That
# makes the repo side of a location comparison machine-readable instead of
# inferred from a unique parameter name.
FIELD_LOCATION_VALUES = {
    "FIELD_LOCATION_PATH": "path",
    "FIELD_LOCATION_QUERY": "query",
    "FIELD_LOCATION_BODY": "body",
    "FIELD_LOCATION_LOCAL": "local",
    "FIELD_LOCATION_TOOL": "tool",
}
# A tool argument that reaches no Linode route: meta-tool domain arguments and
# the local file paths the transfer tools read. Counted apart from the system
# parameters because LOCAL is locked to the `// system param` marker.
TOOL_ARGUMENT_LOCATION = "tool"
VALIDATE_MESSAGE_OPTION = "[buf.validate.message]"
# A rule is named after the field and the constraint, so the identifier is
# where a declared requiredness is readable.
VALIDATE_REQUIRED_SUFFIX = "required"
# A `<tool>.<field>.known` rule holds a scalar field to its documented value
# set, written as a CEL membership list so both languages run the one list.
VALIDATE_KNOWN_SUFFIX = "known"
# The vocabulary a string field's ENUM_MEMBER reader is held to, which is where
# a set lives when the field carries a reader instead of a rule.
READER_VALUES_OPTION = "[linode.mcp.v1.reader_values]"
TECHDOCS_OPERATION_OPTION = "[techdocs.linode.api.operation]"
TECHDOCS_PARAMETER_OPTION = "[techdocs.linode.api.api_parameter]"
TECHDOCS_PROTO_FILE = "techdocs_contract.proto"

TYPE_MAP = {
    "TYPE_BOOL": "boolean",
    "TYPE_BYTES": "string",
    "TYPE_DOUBLE": "number",
    "TYPE_ENUM": "string",
    "TYPE_FIXED32": "integer",
    "TYPE_FIXED64": "integer",
    "TYPE_FLOAT": "number",
    "TYPE_INT32": "integer",
    "TYPE_INT64": "integer",
    "TYPE_MESSAGE": "object",
    "TYPE_SFIXED32": "integer",
    "TYPE_SFIXED64": "integer",
    "TYPE_SINT32": "integer",
    "TYPE_SINT64": "integer",
    "TYPE_STRING": "string",
    "TYPE_UINT32": "integer",
    "TYPE_UINT64": "integer",
}


class ProofError(RuntimeError):
    """The proof could not produce trustworthy current findings."""


def utc_now() -> str:
    return datetime.now(UTC).isoformat()


def read_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ProofError(f"required file is missing: {path}") from exc
    except json.JSONDecodeError as exc:
        raise ProofError(f"invalid JSON in {path}: {exc}") from exc


def is_json_object(value: Any) -> TypeIs[dict[str, Any]]:
    """True when a decoded JSON value is an object.

    Descriptor, ledger, and evidence reads all arrive as Any out of
    json.load, and a bare isinstance guard narrows one to a mapping of
    unknowns that carries the unknown into every later read. JSON keys are
    strings by definition, so the narrowed shape is declared once here and
    every caller keeps the refusal it already had.
    """
    return isinstance(value, dict)


def is_json_array(value: Any) -> TypeIs[list[Any]]:
    """True when a decoded JSON value is an array. See is_json_object."""
    return isinstance(value, list)


def write_json(path: Path | None, payload: Any) -> None:
    text = json.dumps(payload, indent=2, sort_keys=True) + "\n"
    if path is None:
        sys.stdout.write(text)
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(text, encoding="utf-8")
    temporary.replace(path)


def normalize_comment(value: str | None) -> str:
    return " ".join((value or "").split())


def normalize_name(value: str) -> str:
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", value)
    value = re.sub(r"[^A-Za-z0-9]+", "_", value)
    return value.strip("_").lower()


def normalize_semantic_type(value: str) -> str:
    normalized = value.strip().lower()
    normalized = re.sub(r"\s*,\s*unique$", "", normalized)
    normalized = re.sub(r"\s*(?:\|\s*null|or null)$", "", normalized)
    if normalized == "array" or normalized.startswith("array of "):
        return "array"
    # The proto side carries only the scalar, so a format token is the same
    # semantic type.
    return {
        "date-time": "string",
        MASKED_STRING_FORMAT: "string",
        "url": "string",
        "uuid": "string",
    }.get(normalized, normalized)


PATH_PLACEHOLDER_RE = re.compile(r"\{([^{}]*)\}")


def normalize_path(value: str) -> str:
    """Canonicalize a route path while keeping every placeholder name intact.

    The placeholder text is the join key for path parameters, so collapsing it
    would throw away the only evidence that says which slot a parameter fills.
    Shape collapsing happens later in path_shape, where losing the names is the
    point rather than an accident.
    """
    path = value.strip()
    if not path.startswith("/"):
        path = "/" + path
    path = re.sub(r"^/\{apiVersion\}(?=/|$)", "", path)
    path = re.sub(r"^/v4(?:beta)?(?=/|$)", "", path, flags=re.IGNORECASE)
    path = re.sub(r"/{2,}", "/", path)
    if len(path) > 1:
        path = path.rstrip("/")
    return path or "/"


def path_shape(path: str) -> str:
    """Collapse placeholder names so two spellings of one route still join."""
    return PATH_PLACEHOLDER_RE.sub("{}", path)


def path_placeholders(path: str) -> list[str]:
    return [match.group(1) for match in PATH_PLACEHOLDER_RE.finditer(path)]


def route_key(method: str, path: str) -> tuple[str, str]:
    return method.upper().strip(), path_shape(normalize_path(path))


def strip_segmentation(value: str) -> str:
    """Fold a name to its letters and digits so word breaks stop mattering.

    LinodeMCP writes nodebalancer_id where TechDocs writes nodeBalancerId. Both
    are conformant snake_case once normalized, so a difference here is a
    spelling variant and not a missing parameter.
    """
    return normalize_name(value).replace("_", "")


def normalized_json_value(value: Any) -> Any:
    if is_json_array(value):
        return sorted(value, key=lambda item: json.dumps(item, sort_keys=True))
    return value


def normalize_doc_path(path: str) -> str:
    cleaned = path.replace("{apiVersion}", "v4")
    cleaned = re.sub(r"\s+", "", cleaned)
    if not cleaned.startswith("/"):
        cleaned = "/" + cleaned
    normalized = re.sub(r"\{[^}]+\}", "{param}", cleaned)
    return re.sub(r"/+", "/", normalized)


def slugify_url(url: str) -> str:
    parsed = urllib.parse.urlparse(url)
    path = parsed.path.strip("/") or "index"
    slug = re.sub(r"[^A-Za-z0-9._/-]+", "-", path).strip("-").replace("/", "__")
    digest = hashlib.sha256(url.encode("utf-8")).hexdigest()[:10]
    return f"{slug}__{digest}"


def fetch_text(url: str, timeout: int = 30, attempts: int = 3) -> str | None:
    # The page list comes from a fetched sitemap, so the scheme is checked
    # rather than assumed: urlopen would otherwise honour file: and ftp:.
    if not url.startswith("https://"):
        raise ProofError(f"only https TechDocs URLs are fetched, got {url!r}")
    for attempt in range(1, max(1, min(attempts, 3)) + 1):
        request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                charset: str = response.headers.get_content_charset() or "utf-8"
                payload: bytes = response.read()
                return payload.decode(charset, errors="replace")
        except urllib.error.HTTPError as error:
            transient = error.code in {408, 429, 500, 502, 503, 504}
            if transient and attempt < attempts:
                time.sleep(attempt)
                continue
            return None
        except urllib.error.URLError, TimeoutError:
            if attempt < attempts:
                time.sleep(attempt)
                continue
            return None
    return None


VARIANT_SELECT_RE = re.compile(
    r'(?is)<select\b[^>]*data-testid="OneOfMultiSchema-trigger"[^>]*>(.*?)</select>'
)
VARIANT_OPTION_RE = re.compile(r"(?is)<option\b([^>]*)>(.*?)</option>")
VARIANT_LABEL_SEPARATORS_RE = re.compile(r"[,;\[\]]")


def variant_marker(match: re.Match[str]) -> str:
    """Keep a schema switcher's labels as a line the parser can read.

    The switcher is client-side and the fetched HTML carries one variant's
    fields, so the labels are the page's only published evidence that the other
    variants exist at all.
    """
    labels: list[str] = []
    rendered = ""
    for attributes, body in VARIANT_OPTION_RE.findall(match.group(1)):
        label = html.unescape(re.sub(r"<[^>]+>", " ", body))
        label = re.sub(r"\s+", " ", VARIANT_LABEL_SEPARATORS_RE.sub(" ", label)).strip()
        if not label:
            continue
        labels.append(label)
        if not rendered and re.search(r"(?i)(^|\s)selected(\s|=|/|>|$)", attributes):
            rendered = label
    if not labels:
        return "\n"
    return f"\n[variants: {', '.join(labels)}; rendered: {rendered}]\n"


def html_to_markdownish(raw: str) -> str:
    text = re.sub(r"(?is)<script[^>]*>.*?</script>", "\n", raw)
    text = re.sub(r"(?is)<style[^>]*>.*?</style>", "\n", text)
    text = re.sub(r"(?is)<noscript[^>]*>.*?</noscript>", "\n", text)
    text = VARIANT_SELECT_RE.sub(variant_marker, text)
    text = re.sub(r"(?i)</(h[1-6]|p|div|section|article|li|tr|pre|code)>", "\n", text)
    text = re.sub(r"(?i)<br\s*/?>", "\n", text)
    text = re.sub(
        r'(?i)<a\s+[^>]*href=["\']([^"\']+)["\'][^>]*>',
        r" [link: \1] ",
        text,
    )
    text = html.unescape(re.sub(r"<[^>]+>", " ", text))
    lines: list[str] = []
    for raw_line in text.replace("\r\n", "\n").replace("\r", "\n").split("\n"):
        line = re.sub(r"[ \t]+", " ", raw_line).strip()
        if not line:
            continue
        lowered = line.lower()
        if lowered.startswith(("sign in", "log in")):
            continue
        if "powered by" in lowered and "readme" in lowered:
            continue
        lines.append(line)
    return redact_credentials("\n".join(lines) + "\n")


def redact_credentials(text: str) -> str:
    text = re.sub(
        r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----.*?"
        r"-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----",
        "[REDACTED]",
        text,
        flags=re.DOTALL,
    )
    text = re.sub(
        r"(?i)(Authorization:\s*Bearer\s+)[A-Za-z0-9._~+/=-]{20,}",
        r"\1[REDACTED]",
        text,
    )
    text = re.sub(r"\bgh[pousr]_[A-Za-z0-9]{20,}\b", "[REDACTED]", text)
    return re.sub(
        r"\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b",
        "[REDACTED]",
        text,
    )


def discover_docs_pages(max_pages: int | None = None) -> list[str]:
    urls: set[str] = set()
    root = fetch_text(DOCS_ROOT)
    if root:
        for href in re.findall(r'href=["\']([^"\']+)["\']', root):
            full = urllib.parse.urljoin(DOCS_ROOT, href)
            if "/linode-api/reference/" in full:
                urls.add(full.split("#", 1)[0])
    sitemap = fetch_text(SITEMAP_URL)
    if sitemap:
        for location in re.findall(r"<loc>([^<]+)</loc>", sitemap):
            url = urllib.parse.unquote(location.strip())
            if "techdocs.akamai.com/linode-api/reference/" in url:
                urls.add(url.split("#", 1)[0])
    urls = {
        url
        for url in urls
        if "${" not in url
        and "%7B" not in url
        and not re.search(r"\.(?:css|js|png|jpg|jpeg|gif|svg|ico|webp|woff2?)$", url)
        and not re.search(
            r"/linode-api/reference/[0-9]{3}$", urllib.parse.urlparse(url).path
        )
    }
    result = sorted(urls)
    if max_pages:
        result = result[:max_pages]
    if not result:
        raise ProofError("rendered TechDocs discovery returned no pages")
    return result


# The monitor metrics read from their own host, so one pinned host loses them.
TECHDOCS_ROUTE_RE = re.compile(
    r"^(get|post|put|patch|delete)(?:\s+(deprecated))?\s+"
    r"https://(?:api|monitor-api)\.linode\.com\s*/\s*\{apiVersion\}\s*(.+?)\s*$",
    re.IGNORECASE,
)
TECHDOCS_SECTION_RE = re.compile(
    r"(?:\[link:\s*#[^]]+\]\s*)?(Path Params|Query Params|Body Params|Responses)$",
    re.IGNORECASE,
)
# A documented type token: collection prefix, both nullable spellings, and the
# trailing item qualifier arrays render.
TECHDOCS_TYPE_PHRASE = (
    r"(?:(?:array|map) of )?"
    r"(?:strings?|objects?|integers?|numbers?|booleans?|uuids?|urls?|passwords?"
    r"|date-times?|array)"
    r"(?:\s*(?:\|\s*null|or null))?"
    r"(?:\s*,\s*unique)?"
)
TECHDOCS_PARAM_RE = re.compile(
    rf"^([A-Za-z_][A-Za-z0-9_.-]*)\s+({TECHDOCS_TYPE_PHRASE})$",
    re.IGNORECASE,
)
# A switcher renders no type token, so the marker line below the bare name is
# what identifies it.
TECHDOCS_VARIANT_PARAM_RE = re.compile(
    r"^([A-Za-z_][A-Za-z0-9_.-]*)(?:\s+(required))?$"
)
TECHDOCS_VARIANTS_RE = re.compile(r"^\[variants:\s*(.*?);\s*rendered:\s*(.*?)\]$")
TECHDOCS_ENUM_VALUE_RE = re.compile(r"^[A-Za-z0-9_.:/+@-]+$")
# ReadMe renders a switcher only for a choice between object shapes.
TECHDOCS_VARIANT_TYPE = "object"


def parse_techdocs_scalar(raw: str) -> Any:
    value = raw.strip().rstrip(".")
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        return value


def techdocs_section_ranges(
    lines: list[str], route_index: int
) -> dict[str, tuple[int, int]]:
    markers: list[tuple[str, int]] = []
    for index in range(route_index + 1, len(lines)):
        match = TECHDOCS_SECTION_RE.search(lines[index].strip())
        if not match:
            continue
        name = match.group(1).casefold().replace(" params", "")
        markers.append((name, index))
        if name == "responses":
            break
    ranges: dict[str, tuple[int, int]] = {}
    for position, (name, index) in enumerate(markers):
        end = markers[position + 1][1] if position + 1 < len(markers) else len(lines)
        ranges[name] = (index + 1, end)
    return ranges


def parse_techdocs_variants(line: str) -> tuple[list[str], str] | None:
    """Read a rendered schema switcher back out of its marker line."""
    match = TECHDOCS_VARIANTS_RE.match(line)
    if match is None:
        return None
    labels = [item.strip() for item in match.group(1).split(",") if item.strip()]
    return labels, match.group(2).strip()


def techdocs_parameter_heads(
    lines: list[str], start: int, end: int
) -> list[tuple[int, dict[str, Any]]]:
    """Find every parameter head in a section, with the type each one renders.

    A head carries its documented type token, except where the schema is a
    switcher: there the site renders the name alone and the switcher below it,
    so the marker line is what separates that head from ordinary prose.
    """
    heads: list[tuple[int, dict[str, Any]]] = []
    for index in range(start, end):
        line = lines[index].strip()
        typed = TECHDOCS_PARAM_RE.match(line)
        if typed:
            heads.append(
                (
                    index,
                    {
                        "name": typed.group(1),
                        "type": re.sub(r"\s+", " ", typed.group(2).casefold()),
                        "required": False,
                    },
                )
            )
            continue
        if index + 1 >= end:
            continue
        switcher = parse_techdocs_variants(lines[index + 1].strip())
        untyped = TECHDOCS_VARIANT_PARAM_RE.match(line)
        if switcher is None or untyped is None:
            continue
        heads.append(
            (
                index,
                {
                    "name": untyped.group(1),
                    "type": TECHDOCS_VARIANT_TYPE,
                    "required": bool(untyped.group(2)),
                },
            )
        )
    return heads


def techdocs_section_variants(
    lines: list[str], start: int, end: int
) -> tuple[list[str], str]:
    """Read the switcher that governs a whole section rather than one parameter.

    ReadMe renders a section-level switcher with no name above it, so a marker
    that no parameter head introduces is the section's own.
    """
    heads = {index for index, _ in techdocs_parameter_heads(lines, start, end)}
    for index in range(start, end):
        switcher = parse_techdocs_variants(lines[index].strip())
        if switcher is not None and index - 1 not in heads:
            return switcher
    return [], ""


def parse_techdocs_parameters(
    lines: list[str], start: int, end: int
) -> list[dict[str, Any]]:
    starts = techdocs_parameter_heads(lines, start, end)
    parameters: list[dict[str, Any]] = []
    seen: set[str] = set()
    for position, (index, head) in enumerate(starts):
        name = head["name"]
        if name in seen:
            continue
        seen.add(name)
        block_end = starts[position + 1][0] if position + 1 < len(starts) else end
        block = [item.strip() for item in lines[index + 1 : block_end] if item.strip()]
        default_documented = False
        default: Any = None
        enum_values: list[str] = []
        for block_index, item in enumerate(block):
            default_match = re.match(r"^Defaults to\s+(.+)$", item, re.IGNORECASE)
            if default_match:
                default_documented = True
                default = parse_techdocs_scalar(default_match.group(1))
            allowed_match = re.match(r"^Allowed:\s*(.*)$", item, re.IGNORECASE)
            if not allowed_match:
                continue
            candidates = [allowed_match.group(1).strip()]
            for following in block[block_index + 1 :]:
                if not TECHDOCS_ENUM_VALUE_RE.fullmatch(following):
                    break
                candidates.append(following)
            enum_values = sorted({candidate for candidate in candidates if candidate})
        parameter = {
            "name": name,
            "type": head["type"],
            "required": head["required"]
            or any(item.casefold() == "required" for item in block),
            "default": default,
            "enum": enum_values,
            "deprecated": any(item.casefold() == "deprecated" for item in block),
        }
        if default_documented:
            parameter["default_documented"] = True
        parameters.append(parameter)
    return parameters


API_VERSION_PATH_PREFIX_RE = re.compile(r"^/(v4beta|v4)(?=/|$)", re.IGNORECASE)


def classify_api_surface(allowed_values: list[str], raw_path: str) -> str:
    """Say which API surfaces a documented route accepts.

    The apiVersion path parameter carries the answer whenever the page renders
    its Allowed list, and the both-surfaces case is the one a single api_version
    string used to flatten into v4. A page without the list falls back to the
    version the URL itself pins.
    """
    allowed = {value.strip().casefold() for value in allowed_values if value.strip()}
    serves_v4 = "v4" in allowed
    serves_beta = "v4beta" in allowed
    if serves_v4 and serves_beta:
        return SURFACE_BOTH
    if serves_beta:
        return SURFACE_V4BETA_ONLY
    if serves_v4:
        return SURFACE_V4_ONLY
    match = API_VERSION_PATH_PREFIX_RE.match(raw_path.strip())
    if match and match.group(1).casefold() == "v4beta":
        return SURFACE_V4BETA_ONLY
    return SURFACE_V4_ONLY


def endpoint_api_surface(endpoint: dict[str, Any]) -> str:
    """Read a parsed route's surface, empty when the endpoint predates it.

    An endpoint index saved before this classification existed cannot say
    whether a route also serves v4beta, and deriving it from api_version would
    invent beta-only routes, so an unclassified endpoint turns the surface
    comparison off instead of guessing.
    """
    surface = str(endpoint.get("api_surface") or "")
    return surface if surface in API_SURFACE_CLASSIFICATIONS else ""


def parse_techdocs_endpoint(
    text: str, source_url: str, page_file: str
) -> dict[str, Any] | None:
    lines = [line.strip() for line in text.splitlines()]
    route_index = -1
    route_match: re.Match[str] | None = None
    for index, line in enumerate(lines):
        match = TECHDOCS_ROUTE_RE.match(line)
        if match:
            route_index, route_match = index, match
            break
    if route_match is None:
        return None
    method = route_match.group(1).upper()
    deprecated = bool(route_match.group(2))
    suffix = re.sub(r"\s+", "", route_match.group(3))
    raw_path = "/{apiVersion}" + (suffix if suffix.startswith("/") else "/" + suffix)
    path = normalize_doc_path(raw_path)
    ranges = techdocs_section_ranges(lines, route_index)
    parameters: dict[str, list[dict[str, Any]]] = {"path": [], "query": [], "body": []}
    for location in parameters:
        if location in ranges:
            parameters[location] = parse_techdocs_parameters(lines, *ranges[location])
    body_variants, rendered_body_variant = techdocs_section_variants(
        lines, *ranges.get("body", (0, 0))
    )
    version_parameter = next(
        (item for item in parameters["path"] if item["name"] == "apiVersion"), None
    )
    api_surface = classify_api_surface(
        list((version_parameter or {}).get("enum") or []), raw_path
    )
    api_version = "v4beta" if api_surface == SURFACE_V4BETA_ONLY else "v4"
    parameters["path"] = [
        item for item in parameters["path"] if item["name"] != "apiVersion"
    ]
    defaults: dict[str, Any] = {}
    enums: dict[str, list[str]] = {}
    deprecated_parameters: list[str] = []
    for location, items in parameters.items():
        for item in items:
            key = f"{location}.{item['name']}"
            if item.pop("default_documented", False):
                defaults[key] = item["default"]
            if item["enum"]:
                enums[key] = item["enum"]
            if item["deprecated"]:
                deprecated_parameters.append(key)
    response_range = ranges.get("responses")
    status_codes: list[int] = []
    if response_range:
        status_codes = sorted(
            {
                int(item)
                for item in lines[response_range[0] : response_range[1]]
                if re.fullmatch(r"2\d\d", item)
            }
        )
    title = ""
    if route_index >= 2 and lines[route_index - 1].casefold() == "copy page":
        title = lines[route_index - 2]
    return {
        "api_surface": api_surface,
        "api_version": api_version,
        "body_variants": body_variants,
        "deprecated": deprecated,
        "deprecated_parameters": sorted(deprecated_parameters),
        "docs_url": source_url,
        "method": method,
        "operation_id": urllib.parse.urlparse(source_url)
        .path.rstrip("/")
        .rsplit("/", 1)[-1],
        "parameter_defaults": dict(sorted(defaults.items())),
        "parameter_enums": dict(sorted(enums.items())),
        "parameters": parameters,
        "path": path,
        "raw_path": raw_path,
        "rendered_body_variant": rendered_body_variant,
        "source_page": page_file,
        "status_codes": status_codes,
        "title": title,
    }


def build_techdocs_contract_snapshot(endpoints: list[dict[str, Any]]) -> dict[str, Any]:
    routes: list[dict[str, Any]] = []
    parameters: list[dict[str, Any]] = []
    defaults: list[dict[str, Any]] = []
    enums: list[dict[str, Any]] = []
    deprecated_routes: list[dict[str, Any]] = []
    deprecated_parameters: list[dict[str, Any]] = []
    for endpoint in endpoints:
        base = {
            "api_version": endpoint["api_version"],
            "method": endpoint["method"],
            "path": endpoint["path"],
        }
        route = {
            **base,
            "api_surface": endpoint_api_surface(endpoint),
            "deprecated": bool(endpoint["deprecated"]),
            "source_url": endpoint["docs_url"],
        }
        routes.append(route)
        if route["deprecated"]:
            deprecated_routes.append(route)
        for location, items in endpoint["parameters"].items():
            parameters.extend({**base, "location": location, **item} for item in items)
        for key, value in endpoint["parameter_defaults"].items():
            location, name = key.split(".", 1)
            defaults.append(
                {**base, "location": location, "name": name, "value": value}
            )
        for key, values in endpoint["parameter_enums"].items():
            location, name = key.split(".", 1)
            enums.append({**base, "location": location, "name": name, "values": values})
        for key in endpoint["deprecated_parameters"]:
            location, name = key.split(".", 1)
            deprecated_parameters.append({**base, "location": location, "name": name})

    def stable(values: list[dict[str, Any]]) -> list[dict[str, Any]]:
        """Order by canonical JSON so a snapshot diff shows content, not order."""
        return sorted(
            values,
            key=lambda value: json.dumps(value, sort_keys=True, separators=(",", ":")),
        )

    return {
        "source_authority": TECHDOCS_AUTHORITY,
        "routes": stable(routes),
        "parameters": stable(parameters),
        "defaults": stable(defaults),
        "enums": stable(enums),
        "deprecated_routes": stable(deprecated_routes),
        "deprecated_parameters": stable(deprecated_parameters),
    }


ROUTE_SNAPSHOT_NAME = "route-snapshot.txt"
ROUTE_SNAPSHOT_HEADER = """\
# Rendered-TechDocs route snapshot (harvested {harvested}, {count} routes)
# One line per rendered route:
#   METHOD /path/shape surface=<both|v4_only|v4beta_only> status=<active|deprecated>
# Placeholder names collapse to {{}}: TechDocs writes clusterId, the proto
# cluster_id; the shape is the only join.
# Generated; regenerate with:
#   PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof \\
#     --techdocs-contract <run>/techdocs-contracts.json \\
#     --emit-route-snapshot docs/contracts/api-techdocs-routes-baseline.txt
# Read offline by scripts/verify_techdocs_routes.py (make techdocs-routes);
# .github/workflows/techdocs-drift.yml reports the weekly diff.
"""


def route_snapshot_lines(contract: Mapping[str, Any]) -> list[str]:
    """One line per rendered-TechDocs route, in the snapshot's own vocabulary.

    Routes collapse to their path shape because that is the only key the proto
    and the site spell the same way. Two operations that collapse together
    agree on surface and status, so the set stays one line per route.
    """
    lines: set[str] = set()
    for route in contract.get("routes", []):
        method = str(route.get("method") or "").upper().strip()
        shape = path_shape(normalize_path(str(route.get("path") or "")))
        surface = str(route.get("api_surface") or "")
        status = "deprecated" if route.get("deprecated") else "active"
        lines.add(f"{method} {shape} surface={surface} status={status}")
    return sorted(lines)


def route_snapshot_text(contract: Mapping[str, Any], harvested: str) -> str:
    """The snapshot file's whole text, header included."""
    lines = route_snapshot_lines(contract)
    if not lines:
        raise ProofError(
            "the TechDocs contract states no route, so a snapshot written from "
            "it would gate nothing"
        )
    header = ROUTE_SNAPSHOT_HEADER.format(harvested=harvested, count=len(lines))
    return header + "".join(f"{line}\n" for line in lines)


def write_route_snapshot(
    contract: Mapping[str, Any], destination: Path, harvested: str
) -> int:
    """Write the snapshot and answer how many routes it states."""
    text = route_snapshot_text(contract, harvested)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(text, encoding="utf-8")
    return len(text.splitlines()) - len(ROUTE_SNAPSHOT_HEADER.splitlines())


def scrape_techdocs(
    run_dir: Path, workers: int, max_pages: int | None
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    pages_dir = run_dir / "pages"
    endpoints_dir = run_dir / "endpoints"
    pages_dir.mkdir(parents=True, exist_ok=True)
    endpoints_dir.mkdir(parents=True, exist_ok=True)
    urls = discover_docs_pages(max_pages)

    def fetch_one(item: tuple[int, str]) -> dict[str, Any] | None:
        index, url = item
        raw = fetch_text(url)
        if raw is None:
            return None
        content = f"---\nsource_url: {url}\n---\n\n{html_to_markdownish(raw)}"
        relative = Path("pages") / f"{slugify_url(url)}.md"
        (run_dir / relative).write_text(content, encoding="utf-8")
        return {
            "url": url,
            "file": relative.as_posix(),
            "bytes": len(content),
            "index": index,
        }

    page_index: list[dict[str, Any]] = []
    with concurrent.futures.ThreadPoolExecutor(
        max_workers=max(1, min(workers, 32))
    ) as executor:
        futures = [executor.submit(fetch_one, item) for item in enumerate(urls, 1)]
        for count, future in enumerate(concurrent.futures.as_completed(futures), 1):
            record = future.result()
            if record:
                page_index.append(record)
            if count % 50 == 0:
                print(f"fetched {count}/{len(urls)} pages", file=sys.stderr)
    if len(page_index) != len(urls):
        raise ProofError(
            f"incomplete TechDocs snapshot: fetched={len(page_index)} "
            f"discovered={len(urls)}"
        )
    page_index.sort(key=lambda item: item["url"])
    endpoints: list[dict[str, Any]] = []
    seen: set[tuple[str, str, str]] = set()
    for item in page_index:
        endpoint = parse_techdocs_endpoint(
            (run_dir / item["file"]).read_text(encoding="utf-8", errors="replace"),
            item["url"],
            item["file"],
        )
        if endpoint is None:
            continue
        key = (endpoint["api_version"], endpoint["method"], endpoint["path"])
        if key in seen:
            raise ProofError(f"duplicate rendered TechDocs route contract: {key}")
        seen.add(key)
        endpoints.append(endpoint)
        name = re.sub(r"[^A-Za-z0-9]+", "_", "_".join(key)).strip("_").lower()
        write_json(endpoints_dir / f"{name}.json", endpoint)
    if not endpoints:
        raise ProofError("rendered TechDocs pages produced no endpoint contracts")
    endpoints.sort(key=lambda item: (item["path"], item["method"], item["api_version"]))
    contract = build_techdocs_contract_snapshot(endpoints)
    write_json(run_dir / "url-index.json", page_index)
    write_json(run_dir / "endpoint-index.json", endpoints)
    write_json(run_dir / "techdocs-contracts.json", contract)
    return contract, endpoints


def validate_tool_route_extension(descriptor: dict[str, Any]) -> None:
    matches: list[dict[str, Any]] = []
    for file_descriptor in descriptor.get("file", []):
        package = str(file_descriptor.get("package") or "")
        for extension in file_descriptor.get("extension", []):
            qualified = "." + ".".join(
                part for part in (package, str(extension.get("name") or "")) if part
            )
            if qualified == TOOL_ROUTE_EXTENSION:
                matches.append(extension)
    if len(matches) != 1:
        raise ProofError(
            "protobuf descriptor must define exactly one "
            f"{TOOL_ROUTE_EXTENSION} extension; found {len(matches)}"
        )
    extension = matches[0]
    expected = {
        "number": 50001,
        "type": "TYPE_MESSAGE",
        "typeName": ".linode.mcp.v1.ToolRoute",
        "extendee": ".google.protobuf.MessageOptions",
    }
    for key, value in expected.items():
        if extension.get(key) != value:
            raise ProofError(
                f"protobuf {TOOL_ROUTE_EXTENSION} extension has invalid {key}: "
                f"expected {value!r}, got {extension.get(key)!r}"
            )


def validate_tool_api_surface_extension(descriptor: dict[str, Any]) -> None:
    """Check the surface extension's shape, but only once it exists.

    A descriptor built before LinodeMCP declared tool_api_surface is still a
    valid input; it simply has no surface evidence. Absence therefore passes and
    a wrong declaration fails, so a renumbered or re-extended option can never
    be read as a silently empty surface contract.
    """
    matches: list[dict[str, Any]] = []
    for file_descriptor in descriptor.get("file", []):
        matches.extend(
            extension
            for extension in file_descriptor.get("extension", [])
            if str(extension.get("name") or "") == TOOL_API_SURFACE_EXTENSION_NAME
        )
    if not matches:
        return
    if len(matches) != 1:
        raise ProofError(
            f"protobuf descriptor declares {len(matches)} "
            f"{TOOL_API_SURFACE_EXTENSION_NAME} extensions; expected one"
        )
    extension = matches[0]
    # The enum's own type name is deliberately unchecked: the comparison reads
    # value names, and pinning the message path here would fail a descriptor
    # that only moved the enum between files.
    expected = {
        "number": TOOL_API_SURFACE_EXTENSION_NUMBER,
        "type": "TYPE_ENUM",
        "extendee": ".google.protobuf.MessageOptions",
    }
    for key, value in expected.items():
        if extension.get(key) != value:
            raise ProofError(
                f"protobuf {TOOL_API_SURFACE_EXTENSION_NAME} extension has invalid "
                f"{key}: expected {value!r}, got {extension.get(key)!r}"
            )


def tool_api_surface(
    options: Any, file_name: str, message_name: str
) -> tuple[str, str | None]:
    """Resolve one Input message's declared API surface from its options.

    Returns the surface the tool calls and the raw declaration, where a missing
    declaration is the default surface with no raw value. An unrecognized value
    fails closed rather than defaulting to v4, because reading a beta tool as v4
    would hide the exact mismatch this comparison exists to find.
    """
    raw = options.get(TOOL_API_SURFACE_OPTION) if is_json_object(options) else None
    if raw is None:
        return API_SURFACE_VALUES[""], None
    declared = str(raw)
    surface = API_SURFACE_VALUES.get(declared)
    if surface is None:
        raise ProofError(
            f"protobuf {TOOL_API_SURFACE_OPTION} has unknown value on "
            f"{file_name}:{message_name}: {declared!r}"
        )
    return surface, declared


def run_buf_descriptor(
    repo: Path, buf_command: str, output_path: Path | None = None
) -> dict[str, Any]:
    executable = find_executable(buf_command)

    def build(output: Path) -> dict[str, Any]:
        output.parent.mkdir(parents=True, exist_ok=True)
        process = subprocess.run(
            [
                executable,
                "build",
                "--as-file-descriptor-set",
                "--output",
                str(output),
            ],
            cwd=repo,
            text=True,
            capture_output=True,
            check=False,
            timeout=300,
        )
        if process.returncode != 0:
            error = (process.stderr or process.stdout or "unknown Buf error").strip()
            raise ProofError(
                f"Buf descriptor build failed with {process.returncode}: {error}"
            )
        descriptor = read_json(output)
        if not is_json_object(descriptor):
            raise ProofError(f"Buf wrote a descriptor that is not an object: {output}")
        return descriptor

    if output_path is None:
        with tempfile.TemporaryDirectory(prefix="techdocs-proto-proof-") as directory:
            descriptor = build(Path(directory) / "descriptor.json")
    else:
        descriptor = build(output_path)
    # build already refused a non-object descriptor, so the only thing left to
    # prove here is that the object carries files.
    files = descriptor.get("file")
    if not isinstance(files, list) or not files:
        raise ProofError("Buf output contains no descriptor files")
    return descriptor


def source_comments(file_descriptor: dict[str, Any]) -> dict[tuple[int, ...], str]:
    source = file_descriptor.get("sourceCodeInfo")
    locations = source.get("location") if is_json_object(source) else None
    if not is_json_array(locations) or not locations:
        raise ProofError(
            "descriptor source comments are missing for "
            f"{file_descriptor.get('name', '<unknown>')}"
        )
    comments: dict[tuple[int, ...], str] = {}
    for location in locations:
        path = location.get("path")
        if not is_json_array(path):
            continue
        leading = normalize_comment(location.get("leadingComments"))
        trailing = normalize_comment(location.get("trailingComments"))
        # Trailing comments carry the system-parameter marker in LinodeMCP, so
        # both are kept. Trailing goes first: the marker has to be able to start
        # the string for the prefix test, and a leading comment is prose.
        merged = " ".join(part for part in (trailing, leading) if part)
        if merged:
            comments[tuple(int(item) for item in path)] = merged
    return comments


def descriptor_indexes(
    descriptor: dict[str, Any],
) -> tuple[dict[str, dict[str, Any]], dict[str, list[str]]]:
    messages: dict[str, dict[str, Any]] = {}
    enums: dict[str, list[str]] = {}

    def add_message(package: str, parent: str, message: dict[str, Any]) -> None:
        name = str(message.get("name") or "")
        qualified = "." + ".".join(part for part in (package, parent, name) if part)
        messages[qualified] = message
        nested_parent = ".".join(part for part in (parent, name) if part)
        for enum in message.get("enumType", []):
            enum_name = str(enum.get("name") or "")
            enum_key = "." + ".".join(
                part for part in (package, nested_parent, enum_name) if part
            )
            enums[enum_key] = [
                str(value.get("name") or "")
                for value in enum.get("value", [])
                if str(value.get("name") or "").lower() != "unspecified"
            ]
        for nested in message.get("nestedType", []):
            # Synthetic MapEntry messages belong in the index too. Skipping them
            # left is_map_field unable to resolve a map field's type name, so
            # every map read as a plain repeated field and reported as an array.
            add_message(package, nested_parent, nested)

    for file_descriptor in descriptor.get("file", []):
        package = str(file_descriptor.get("package") or "")
        for enum in file_descriptor.get("enumType", []):
            enum_name = str(enum.get("name") or "")
            enum_key = "." + ".".join(part for part in (package, enum_name) if part)
            enums[enum_key] = [
                str(value.get("name") or "")
                for value in enum.get("value", [])
                if str(value.get("name") or "").lower() != "unspecified"
            ]
        for message in file_descriptor.get("messageType", []):
            add_message(package, "", message)
    return messages, enums


def field_enum_values(
    field: dict[str, Any],
    messages: dict[str, dict[str, Any]],
    enums: dict[str, list[str]],
) -> list[str]:
    type_name = str(field.get("typeName") or "")
    if field.get("type") == "TYPE_ENUM":
        return sorted(value for value in enums.get(type_name, []) if value)
    if field.get("type") != "TYPE_MESSAGE":
        return []
    message = messages.get(type_name)
    if not message:
        return []
    for enum in message.get("enumType", []):
        if enum.get("name") != "Value":
            continue
        return sorted(
            str(value.get("name") or "")
            for value in enum.get("value", [])
            if str(value.get("name") or "").lower() != "unspecified"
        )
    return []


def is_map_field(field: dict[str, Any], messages: dict[str, dict[str, Any]]) -> bool:
    if field.get("type") != "TYPE_MESSAGE":
        return False
    message = messages.get(str(field.get("typeName") or ""))
    return bool(message and message.get("options", {}).get("mapEntry"))


def declared_required_fields(message: dict[str, Any]) -> set[str]:
    """Fields the input message declares required through a validation rule.

    proto3 presence says whether a field can be omitted on the wire, not whether
    the tool accepts the call without it. The rule is what the handler runs, so
    the rule is what the documented contract has to be compared against. A rule
    counts only when its identifier names the field it constrains and its
    expression reads that same field, which keeps a member rule such as
    `saml.entity_id.required` off the parent.
    """
    options: dict[str, Any] = message.get("options") or {}
    validation: dict[str, Any] = options.get(VALIDATE_MESSAGE_OPTION) or {}
    rules: list[dict[str, Any]] = validation.get("cel") or []
    required: set[str] = set()
    for rule in rules:
        identifier = str(rule.get("id") or "").split(".")
        if len(identifier) < 2 or identifier[-1] != VALIDATE_REQUIRED_SUFFIX:
            continue
        field_name = identifier[-2]
        expression = str(rule.get("expression") or "")
        if re.search(rf"\bthis\.{re.escape(field_name)}\b", expression):
            required.add(field_name)
    return required


def membership_literals(list_body: str) -> list[str]:
    """The members of one CEL list literal, each rendered as the page would.

    A string member drops its quotes and an integer member keeps its digits,
    so `[1, 2, 3]` and `['a', 'b']` both compare against the rendered
    `Allowed:` list as strings. Anything else in the list is a shape this
    reader does not understand, and the whole set is left undeclared rather
    than half read.
    """
    values: list[str] = []
    for item in list_body.split(","):
        token = item.strip()
        if len(token) >= 2 and token[0] == token[-1] and token[0] in "'\"":
            values.append(token[1:-1])
        elif re.fullmatch(r"-?\d+", token):
            values.append(token)
        else:
            return []
    return values


def declared_value_sets(message: dict[str, Any]) -> dict[str, list[str]]:
    """Fields the input message holds to a value set through a validation rule.

    A scalar field cannot carry an enum descriptor without changing the wire
    type, so LinodeMCP declares its documented values as a `.known` rule whose
    expression tests `this.<field> in [...]`. The rule is what the handler
    runs, so the list is what the rendered `Allowed:` values are compared
    against. A rule counts only when its identifier names the field and the
    membership test reads that same field, which keeps a range rule and a
    member rule such as `saml.identity_element.known` from landing a set on a
    field they do not constrain.
    """
    options: dict[str, Any] = message.get("options") or {}
    validation: dict[str, Any] = options.get(VALIDATE_MESSAGE_OPTION) or {}
    rules: list[dict[str, Any]] = validation.get("cel") or []
    value_sets: dict[str, list[str]] = {}
    for rule in rules:
        identifier = str(rule.get("id") or "").split(".")
        if len(identifier) < 2 or identifier[-1] != VALIDATE_KNOWN_SUFFIX:
            continue
        field_name = identifier[-2]
        expression = str(rule.get("expression") or "")
        membership = re.search(
            rf"\bthis\.{re.escape(field_name)} in \[([^\]]*)\]", expression
        )
        if membership is None:
            continue
        values = membership_literals(membership.group(1))
        if values:
            value_sets[field_name] = values
    return value_sets


def field_reader_values(field: dict[str, Any]) -> list[str]:
    """The vocabulary an ENUM_MEMBER reader holds a string field to.

    The option is a repeated string, and it is the last proto side of the enum
    comparison: it is read only when the field carries no enum descriptor and
    no `.known` rule.
    """
    options: dict[str, Any] = field.get("options", {})
    raw: list[Any] = options.get(READER_VALUES_OPTION) or []
    return [str(value) for value in raw if str(value)]


def extract_proto_tools(
    descriptor: dict[str, Any],
) -> tuple[dict[str, dict[str, Any]], list[dict[str, Any]]]:
    validate_tool_route_extension(descriptor)
    validate_tool_api_surface_extension(descriptor)
    messages, enums = descriptor_indexes(descriptor)
    tools: dict[str, dict[str, Any]] = {}
    extraction_findings: list[dict[str, Any]] = []

    for file_descriptor in descriptor.get("file", []):
        file_name = str(file_descriptor.get("name") or "")
        if not file_name.startswith("linode/mcp/v1/"):
            continue
        comments = source_comments(file_descriptor)
        package = str(file_descriptor.get("package") or "")
        for message_index, message in enumerate(file_descriptor.get("messageType", [])):
            message_name = str(message.get("name") or "")
            options = message.get("options")
            route = options.get(TOOL_ROUTE_OPTION) if is_json_object(options) else None
            if route is not None and not message_name.endswith("Input"):
                raise ProofError(
                    f"protobuf {TOOL_ROUTE_OPTION} is attached to non-Input message "
                    f"{file_name}:{message_name}"
                )
            if not message_name.endswith("Input") or route is None:
                continue  # Meta/local input message with no Linode API route.
            if not is_json_object(route):
                raise ProofError(
                    f"protobuf {TOOL_ROUTE_OPTION} is malformed on "
                    f"{file_name}:{message_name}"
                )
            tool = str(route.get("tool") or "")
            method = str(route.get("method") or "")
            raw_path = str(route.get("path") or "")
            if not re.fullmatch(r"linode_[a-z0-9_]+", tool):
                raise ProofError(
                    f"protobuf tool route has invalid tool on "
                    f"{file_name}:{message_name}: {tool!r}"
                )
            if method not in {"DELETE", "GET", "PATCH", "POST", "PUT"}:
                raise ProofError(
                    f"protobuf tool route has invalid method on "
                    f"{file_name}:{message_name}: {method!r}"
                )
            if not raw_path.startswith("/") or " " in raw_path:
                raise ProofError(
                    f"protobuf tool route has invalid path on "
                    f"{file_name}:{message_name}: {raw_path!r}"
                )
            method = method.upper().strip()
            canonical_path = normalize_path(raw_path)
            surface, declared_surface = tool_api_surface(
                options, file_name, message_name
            )
            if tool in tools:
                raise ProofError(
                    f"duplicate protobuf tool route for {tool!r}: "
                    f"{tools[tool]['input_message']} and {message_name}"
                )
            qualified_message = ".".join(
                part for part in (package, message_name) if part
            )
            parameters: list[dict[str, Any]] = []
            required_by_rule = declared_required_fields(message)
            value_sets = declared_value_sets(message)
            for field_index, field in enumerate(message.get("field", [])):
                comment = comments.get((4, message_index, 2, field_index), "")
                repeated = field.get("label") == "LABEL_REPEATED"
                map_field = is_map_field(field, messages)
                field_type = (
                    "object"
                    if map_field
                    else TYPE_MAP.get(str(field.get("type")), "unknown")
                )
                if repeated and not map_field:
                    field_type = "array"
                # body_comma_list splits the argument into the array the API
                # reads, so the array is the contract fact the page documents;
                # the string is only how a caller types the value.
                if field.get("options", {}).get(BODY_COMMA_LIST_OPTION):
                    field_type = "array"
                required: bool | None
                # A singular message field tracks presence without `optional`,
                # so reading it as required overstates the contract.
                singular_message = (
                    str(field.get("type")) == "TYPE_MESSAGE"
                    and not repeated
                    and not map_field
                )
                if str(field.get("name") or "") in required_by_rule:
                    required = True
                elif field.get("proto3Optional") or singular_message:
                    required = False
                elif repeated or map_field:
                    required = None
                else:
                    required = True
                raw_location = field.get("options", {}).get(FIELD_LOCATION_OPTION)
                location = FIELD_LOCATION_VALUES.get(str(raw_location or ""))
                field_name = str(field.get("name") or "")
                parameters.append(
                    {
                        "name": field_name,
                        "json_name": str(field.get("jsonName") or field_name),
                        "normalized_name": normalize_name(field_name),
                        "segmented_name": strip_segmentation(field_name),
                        "location": location,
                        "declared_location": str(raw_location or "") or None,
                        "type": field_type,
                        "proto_type": str(field.get("type") or ""),
                        "required": required,
                        "repeated": repeated,
                        "map": map_field,
                        # An enum descriptor is the field's own vocabulary; a
                        # scalar reads its declared rule, then its reader.
                        "enum": (
                            field_enum_values(field, messages, enums)
                            or value_sets.get(field_name)
                            or field_reader_values(field)
                        ),
                        "deprecated": bool(field.get("options", {}).get("deprecated")),
                        "default": field.get("defaultValue"),
                        "comment": comment,
                        "system_parameter": (
                            location == "local"
                            if location is not None
                            else is_system_parameter(comment)
                        ),
                    }
                )
            tools[tool] = {
                "tool": tool,
                "input_message": qualified_message,
                "file": file_name,
                "api_surface": surface,
                "declared_api_surface": declared_surface,
                "method": method,
                "path": canonical_path,
                "route_shape": path_shape(canonical_path),
                "placeholders": path_placeholders(canonical_path),
                "parameters": parameters,
            }
    if not tools:
        raise ProofError(
            f"protobuf descriptor contains no {TOOL_ROUTE_OPTION} message annotations"
        )
    return tools, extraction_findings


def validate_techdocs_extensions(descriptor: dict[str, Any]) -> None:
    """Fail closed unless the generated TechDocs proto declares both extensions.

    Reading the generated descriptor is only trustworthy when the annotations
    the comparison depends on are actually present, so their absence has to be
    an error rather than a silently empty TechDocs side.
    """
    expected = {
        "operation": (
            51001,
            ".techdocs.linode.api.OperationContract",
            ".google.protobuf.MessageOptions",
        ),
        "api_parameter": (
            51002,
            ".techdocs.linode.api.ParameterContract",
            ".google.protobuf.FieldOptions",
        ),
    }
    found: dict[str, tuple[Any, Any, Any]] = {}
    for file_descriptor in descriptor.get("file", []):
        for extension in file_descriptor.get("extension", []):
            name = str(extension.get("name") or "")
            if name in expected:
                found[name] = (
                    extension.get("number"),
                    extension.get("typeName"),
                    extension.get("extendee"),
                )
    for name, signature in expected.items():
        if found.get(name) != signature:
            raise ProofError(
                f"generated TechDocs proto extension {name!r} is missing or invalid: "
                f"expected {signature!r}, got {found.get(name)!r}"
            )


def techdocs_indexes(
    descriptor: dict[str, Any],
) -> tuple[
    dict[tuple[str, str], dict[str, Any]],
    dict[tuple[str, str], list[dict[str, Any]]],
    list[dict[str, Any]],
]:
    """Index the TechDocs side straight out of the generated proto descriptor.

    The scraped page dictionary feeds proto generation and nothing else. Every
    fact compared below is read back from the compiled descriptor, so a claim
    that survives here is a claim the generated proto really encodes.
    """
    validate_techdocs_extensions(descriptor)
    routes: dict[tuple[str, str], dict[str, Any]] = {}
    parameters: dict[tuple[str, str], list[dict[str, Any]]] = defaultdict(list)
    duplicate_findings: list[dict[str, Any]] = []

    for file_descriptor in descriptor.get("file", []):
        if not str(file_descriptor.get("name") or "").endswith(TECHDOCS_PROTO_FILE):
            continue
        for message in file_descriptor.get("messageType", []):
            options = message.get("options")
            operation = (
                options.get(TECHDOCS_OPERATION_OPTION)
                if is_json_object(options)
                else None
            )
            if not is_json_object(operation):
                continue  # OperationContract and ParameterContract carry no route.
            # Buf hands the option back as decoded JSON, so the contract is read
            # through one declared shape rather than off an untyped mapping.
            contract: dict[str, Any] = operation
            method = str(contract.get("method") or "").upper().strip()
            canonical_path = normalize_path(str(contract.get("path") or ""))
            key = (method, path_shape(canonical_path))
            declared_surface = str(contract.get("apiSurface") or "")
            variant_labels: list[Any] = contract.get("bodyVariants") or []
            status_codes: list[Any] = contract.get("statusCodes") or []
            route: dict[str, Any] = {
                "api_version": str(contract.get("apiVersion") or ""),
                "api_surface": (
                    declared_surface
                    if declared_surface in API_SURFACE_CLASSIFICATIONS
                    else ""
                ),
                "method": method,
                "path": canonical_path,
                "route_shape": key[1],
                "placeholders": path_placeholders(canonical_path),
                "deprecated": bool(contract.get("isDeprecated")),
                "source_url": str(contract.get("sourceUrl") or ""),
                "status_codes": list(status_codes),
                "message": str(message.get("name") or ""),
                "body_variants": [str(label) for label in variant_labels],
                "rendered_body_variant": str(contract.get("renderedBodyVariant") or ""),
            }
            prior = routes.get(key)
            if prior and prior["message"] != route["message"]:
                duplicate_findings.append(
                    {
                        "kind": "techdocs_route_ambiguous",
                        "severity": "medium",
                        "method": key[0],
                        "path": canonical_path,
                        "route_shape": key[1],
                        "messages": sorted({prior["message"], route["message"]}),
                        "source_urls": sorted(
                            {prior["source_url"], route["source_url"]} - {""}
                        ),
                    }
                )
                continue
            routes[key] = route
            for field in message.get("field", []):
                parameters[key].append(techdocs_parameter(field))

    if not routes:
        raise ProofError(
            f"generated TechDocs descriptor contains no {TECHDOCS_OPERATION_OPTION} "
            "annotations"
        )
    return routes, parameters, duplicate_findings


def techdocs_parameter(field: dict[str, Any]) -> dict[str, Any]:
    """Read one documented parameter back out of a generated proto field."""
    options: dict[str, Any] = field.get("options", {})
    contract: dict[str, Any] = options.get(TECHDOCS_PARAMETER_OPTION) or {}
    field_name = str(field.get("name") or "")
    name = str(contract.get("documentedName") or "") or field_name
    documented_type = str(contract.get("documentedType") or "")
    enum_values: list[Any] = contract.get("enumValues") or []
    default_json = contract.get("defaultJson")
    default: Any = None
    if contract.get("hasDefault"):
        try:
            default = json.loads(str(default_json))
        except TypeError, ValueError:
            default = default_json
    return {
        "name": name,
        "generated_field_name": field_name,
        "normalized_name": normalize_name(name),
        "segmented_name": strip_segmentation(name),
        "location": str(contract.get("location") or "") or None,
        "raw_type": documented_type,
        "type": normalize_semantic_type(documented_type),
        "proto_type": str(field.get("type") or ""),
        "repeated": field.get("label") == "LABEL_REPEATED",
        "required": bool(contract.get("isRequired")),
        "has_default": bool(contract.get("hasDefault")),
        "default": default,
        "enum": sorted(str(value) for value in enum_values),
        "deprecated": bool(contract.get("isDeprecated")),
    }


# Repo fields that are the documented parameter under a different name. Keyed by
# route and location so a name that means something else elsewhere is unaffected.
# A rename here produces a real match, so type and requiredness are still compared.
KNOWN_RENAMES: dict[tuple[str, str, str, str], dict[str, str]] = {
    ("POST", "/linode/instances/{}/backups/{}/restore", "body", "linode_id"): {
        "repo_name": "target_linode_id",
        "reason": "The repo names the restore target explicitly because the "
        "route already has a linode_id path parameter.",
    },
    ("PUT", "/account/users/{}", "body", "username"): {
        "repo_name": "new_username",
        "reason": "The repo names the replacement username explicitly because "
        "the route already has a username path parameter.",
    },
    ("POST", "/linode/instances/{}/upgrade-interfaces", "body", "dry_run"): {
        "repo_name": "api_dry_run",
        "reason": "The repo reserves dry_run for its own two-stage preview, so "
        "the API's dry_run body field carries an api_ prefix.",
    },
    ("POST", "/linode/instances/{}/clone", "body", "linode_id"): {
        "repo_name": "target_linode_id",
        "reason": "The repo names the clone target explicitly, matching the "
        "backup-restore precedent, because the route already has a linode_id "
        "path parameter.",
    },
}

# Divergences a triage confirmed are not repo defects. Every entry names the
# exact route, location, and parameter it covers, so a name that is a real
# field on some other route keeps its finding. The ledger is data, not source:
# entries are review material, and helper scripts once patched them into this
# file.
LEDGER_FIELDS = (
    "category",
    "kind",
    "method",
    "shape",
    "location",
    "parameter",
    "reason",
)
# Keys are written out rather than pattern-matched: a pattern absorbs whatever
# starts matching later and never goes stale, which is a suppression.
LEDGER_CLASS_FIELDS = ("category", "kind", "reason", "entries")
LEDGER_CLASS_ENTRY_FIELDS = ("method", "shape", "location", "parameter")


def json_list(value: Any, message: str) -> list[Any]:
    """Read a decoded JSON value as a list, refusing anything else.

    is_json_array declares the shape so a read off the list is typed; this
    pairs that with the caller's own failure text, which is what turns a
    wrong ledger or index into a named refusal instead of a later crash.
    """
    if not is_json_array(value):
        raise ProofError(message)
    return value


def expand_ledger_class(position: int, item: dict[str, Any]) -> list[dict[str, str]]:
    """Turn one class rule into the per-key entries the comparison matches."""
    failure = f"exclusion ledger class {position} must list at least one entry"
    rows = json_list(item["entries"], failure)
    if not rows:
        raise ProofError(failure)
    expanded: list[dict[str, str]] = []
    for offset, row in enumerate(rows):
        if not is_json_object(row) or set(row) != set(LEDGER_CLASS_ENTRY_FIELDS):
            raise ProofError(
                f"exclusion ledger class {position} entry {offset} must carry "
                f"exactly {list(LEDGER_CLASS_ENTRY_FIELDS)}"
            )
        expanded.append(
            {
                "category": str(item["category"]),
                "kind": str(item["kind"]),
                "reason": str(item["reason"]),
                **{field: str(row[field]) for field in LEDGER_CLASS_ENTRY_FIELDS},
            }
        )
    return expanded


def load_known_divergences(path: Path = LEDGER_PATH) -> tuple[dict[str, str], ...]:
    """Read the exclusion ledger, refusing an entry the comparison cannot key."""
    raw = json_list(read_json(path), f"exclusion ledger must hold a JSON list: {path}")
    entries: list[dict[str, str]] = []
    for position, item in enumerate(raw):
        if is_json_object(item) and set(item) == set(LEDGER_CLASS_FIELDS):
            entries.extend(expand_ledger_class(position, item))
            continue
        if not is_json_object(item) or set(item) != set(LEDGER_FIELDS):
            raise ProofError(
                f"exclusion ledger entry {position} must carry exactly "
                f"{list(LEDGER_FIELDS)} or {list(LEDGER_CLASS_FIELDS)}"
            )
        entries.append({field: str(item[field]) for field in LEDGER_FIELDS})
    return tuple(entries)


KNOWN_DIVERGENCES: tuple[dict[str, str], ...] = load_known_divergences()


def known_divergence_index() -> dict[tuple[str, ...], dict[str, str]]:
    index: dict[tuple[str, ...], dict[str, str]] = {}
    for entry in KNOWN_DIVERGENCES:
        shape = entry["shape"]
        if shape != path_shape(shape):
            raise ProofError(
                f"known divergence route must be a collapsed shape, got {shape!r}"
            )
        # A kind the comparison never raises can never match, so the entry would
        # sit in known_divergences_unmatched reading like upstream drift. The
        # severity table is the list of kinds a run can produce.
        if entry["kind"] not in FINDING_SEVERITY:
            raise ProofError(
                f"known divergence names a kind the comparison cannot raise: "
                f"{entry['kind']!r}"
            )
        key = (
            entry["kind"],
            entry["method"],
            shape,
            entry["location"],
            entry["parameter"],
        )
        # A duplicate was visible in review while the ledger was source; as
        # data it would silently shadow the earlier entry's reason instead.
        if key in index:
            raise ProofError(f"duplicate known divergence entry: {key}")
        index[key] = entry
    return index


# high      : TechDocs documents a route or parameter the proto does not carry.
# medium    : both sides carry it and disagree on a fact.
# known     : triaged divergence with a recorded reason, not a repo defect.
# limitation: proto3 cannot express the fact, so nothing can judge it.
# info      : spelling difference or an observation, not a contract gap.
FINDING_SEVERITY = {
    "route_missing_from_proto": "high",
    "techdocs_parameter_missing_from_proto": "high",
    "path_parameter_count_mismatch": "high",
    "deprecated_replacement_missing_from_proto": "high",
    "route_surface_mismatch": "high",
    "parameter_type_mismatch": "medium",
    "parameter_requiredness_mismatch": "medium",
    "parameter_enum_mismatch": "medium",
    "parameter_deprecation_mismatch": "medium",
    "parameter_default_mismatch": "medium",
    "parameter_location_mismatch": "medium",
    "parameter_location_ambiguous": "medium",
    "proto_parameter_not_in_techdocs": "medium",
    "proto_route_absent_from_techdocs": "medium",
    "proto_field_location_missing": "medium",
    "deprecated_route_still_in_proto": "medium",
    "deprecated_replacement_also_deprecated": "medium",
    "deprecated_replacement_missing_from_techdocs_snapshot": "medium",
    "deprecated_replacement_ambiguous": "medium",
    "techdocs_route_ambiguous": "medium",
    "parameter_name_segmentation_variant": "info",
    "path_parameter_name_differs": "info",
    "route_surface_downgrade_available": "info",
    "proto_requiredness_ambiguous": "limitation",
    "parameter_default_unrepresented_in_proto": "limitation",
    "techdocs_body_variant_unrendered": "limitation",
}

SEVERITY_ORDER = ("high", "medium", "known", "limitation", "info")


def apply_known_divergences(
    findings: list[dict[str, Any]],
) -> tuple[list[dict[str, Any]], list[dict[str, str]]]:
    """Move triaged divergences into the known bucket and report stale entries.

    An entry that stops matching means the documentation moved underneath it, so
    the unmatched list is reported rather than silently carried.
    """
    index = known_divergence_index()
    matched: set[tuple[str, ...]] = set()
    for finding in findings:
        lookup = (
            str(finding.get("kind") or ""),
            str(finding.get("method") or ""),
            str(finding.get("route_shape") or ""),
            str(finding.get("location") or ""),
            str(finding.get("parameter") or ""),
        )
        entry = index.get(lookup)
        if entry is None:
            continue
        matched.add(lookup)
        finding["severity"] = "known"
        finding["known_category"] = entry["category"]
        finding["known_reason"] = entry["reason"]
    unused = [
        {
            "kind": entry["kind"],
            "method": entry["method"],
            "shape": entry["shape"],
            "location": entry["location"],
            "parameter": entry["parameter"],
            "category": entry["category"],
        }
        for lookup, entry in index.items()
        if lookup not in matched
    ]
    return findings, unused


def finding_base(
    kind: str,
    key: tuple[str, str],
    route: dict[str, Any] | None = None,
    tool: dict[str, Any] | None = None,
) -> dict[str, Any]:
    display = (route or {}).get("path") or (tool or {}).get("path") or key[1]
    result: dict[str, Any] = {
        "kind": kind,
        "severity": FINDING_SEVERITY.get(kind, "medium"),
        "method": key[0],
        "path": display,
        "route_shape": key[1],
    }
    if route and route.get("source_url"):
        result["source_url"] = route["source_url"]
    if route and route.get("path") and tool and tool.get("path") != route.get("path"):
        result["proto_path"] = tool.get("path")
    if tool:
        result["tool"] = tool.get("tool")
        result["input_message"] = tool.get("input_message")
    return result


def order_path_slots(
    path: str, params: list[dict[str, Any]]
) -> tuple[list[dict[str, Any] | None], list[dict[str, Any]]]:
    """Line path parameters up with the placeholders they fill, left to right.

    Binding by placeholder name first keeps the slots honest when a side lists
    its path parameters out of order; declaration order only fills the slots
    that no name resolved.
    """
    by_normalized: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_segmented: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for param in params:
        by_normalized[str(param.get("normalized_name") or "")].append(param)
        by_segmented[str(param.get("segmented_name") or "")].append(param)
    consumed: set[int] = set()
    slots: list[dict[str, Any] | None] = []
    for token in path_placeholders(path):
        chosen: dict[str, Any] | None = None
        for index, name in (
            (by_normalized, normalize_name(token)),
            (by_segmented, strip_segmentation(token)),
        ):
            chosen = next(
                (item for item in index.get(name, []) if id(item) not in consumed), None
            )
            if chosen is not None:
                break
        if chosen is not None:
            consumed.add(id(chosen))
        slots.append(chosen)
    unbound = [param for param in params if id(param) not in consumed]
    for position, slot in enumerate(slots):
        if slot is None and unbound:
            slots[position] = unbound.pop(0)
    return slots, unbound


def unrendered_body_variants(route: dict[str, Any], location: str) -> list[str]:
    """Body-schema variants the page named and the fetched HTML left out.

    A non-empty answer means the documented body is one branch of a choice, so
    the missing branches, not the repo, explain a field or an allowed value the
    rendered side does not carry.
    """
    if location != "body":
        return []
    rendered = str(route.get("rendered_body_variant") or "")
    labels: list[Any] = route.get("body_variants") or []
    return [str(label) for label in labels if label != rendered]


def body_variant_finding(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    parameter: str,
    observation: str,
    unrendered: list[str],
) -> dict[str, Any]:
    finding = finding_base("techdocs_body_variant_unrendered", key, route, tool)
    finding.update(
        {
            "parameter": parameter,
            "location": "body",
            "observation": observation,
            "techdocs": {
                "body_variants": list(route.get("body_variants") or []),
                "rendered_body_variant": route.get("rendered_body_variant") or "",
                "unrendered_body_variants": unrendered,
            },
        }
    )
    return finding


def compare_parameter(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    proto_parameter: dict[str, Any],
    tech_parameter: dict[str, Any],
    matched_by: str = "name",
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    base = finding_base("", key, route, tool)
    base.update(
        {
            "parameter": proto_parameter["name"],
            "techdocs_parameter": tech_parameter.get("name"),
            "location": tech_parameter.get("location"),
            "proto_location": proto_parameter.get("location"),
            "matched_by": matched_by,
        }
    )

    proto_normalized = str(proto_parameter.get("normalized_name") or "")
    tech_normalized = str(tech_parameter.get("normalized_name") or "")
    if tech_parameter.get("renamed_to") == proto_normalized:
        base["matched_by"] = "known_rename"
        base["known_reason"] = tech_parameter.get("rename_reason")
    elif proto_normalized != tech_normalized:
        same_letters = proto_parameter.get("segmented_name") == tech_parameter.get(
            "segmented_name"
        )
        finding = dict(base)
        finding.update(
            {
                "kind": (
                    "parameter_name_segmentation_variant"
                    if same_letters
                    else "path_parameter_name_differs"
                ),
                "techdocs": tech_parameter.get("name"),
                "proto": proto_parameter.get("name"),
            }
        )
        finding["severity"] = FINDING_SEVERITY[finding["kind"]]
        findings.append(finding)

    proto_location = proto_parameter.get("location")
    tech_location = tech_parameter.get("location")
    if proto_location and tech_location and proto_location != tech_location:
        finding = dict(base)
        finding.update(
            {
                "kind": "parameter_location_mismatch",
                "severity": FINDING_SEVERITY["parameter_location_mismatch"],
                "techdocs": tech_location,
                "proto": proto_location,
            }
        )
        findings.append(finding)

    if proto_parameter.get("type") != tech_parameter.get("type"):
        finding = dict(base)
        finding.update(
            {
                "kind": "parameter_type_mismatch",
                "techdocs": tech_parameter.get("type"),
                "proto": proto_parameter.get("type"),
                "techdocs_documented_type": tech_parameter.get("raw_type"),
                "techdocs_proto_type": tech_parameter.get("proto_type"),
                "proto_field_type": proto_parameter.get("proto_type"),
            }
        )
        findings.append(finding)

    proto_required = proto_parameter.get("required")
    tech_required = bool(tech_parameter.get("required"))
    if proto_required is None:
        finding = dict(base)
        finding.update(
            {
                "kind": "proto_requiredness_ambiguous",
                "techdocs": tech_required,
                "proto": None,
            }
        )
        findings.append(finding)
    elif bool(proto_required) != tech_required:
        finding = dict(base)
        finding.update(
            {
                "kind": "parameter_requiredness_mismatch",
                "techdocs": tech_required,
                "proto": bool(proto_required),
            }
        )
        findings.append(finding)

    tech_default = tech_parameter.get("default")
    if tech_default is not None:
        proto_default = proto_parameter.get("default")
        if proto_default is None:
            finding = dict(base)
            finding.update(
                {
                    "kind": "parameter_default_unrepresented_in_proto",
                    "techdocs": tech_default,
                    "proto": None,
                }
            )
            findings.append(finding)
        elif str(proto_default) != str(tech_default):
            finding = dict(base)
            finding.update(
                {
                    "kind": "parameter_default_mismatch",
                    "techdocs": tech_default,
                    "proto": proto_default,
                }
            )
            findings.append(finding)

    tech_enum = sorted(str(value) for value in tech_parameter.get("enum", []))
    proto_enum = sorted(str(value) for value in proto_parameter.get("enum", []))
    if tech_enum != proto_enum and (tech_enum or proto_enum):
        unrendered = unrendered_body_variants(
            route, str(tech_parameter.get("location") or "")
        )
        if unrendered and set(tech_enum) < set(proto_enum):
            findings.append(
                body_variant_finding(
                    key,
                    route,
                    tool,
                    str(proto_parameter["name"]),
                    "the rendered variant lists fewer allowed values than the proto",
                    unrendered,
                )
            )
        else:
            finding = dict(base)
            finding.update(
                {
                    "kind": "parameter_enum_mismatch",
                    "techdocs": tech_enum,
                    "proto": proto_enum,
                }
            )
            findings.append(finding)

    tech_deprecated = bool(tech_parameter.get("deprecated"))
    proto_deprecated = bool(proto_parameter.get("deprecated"))
    if tech_deprecated != proto_deprecated:
        finding = dict(base)
        finding.update(
            {
                "kind": "parameter_deprecation_mismatch",
                "techdocs": tech_deprecated,
                "proto": proto_deprecated,
            }
        )
        findings.append(finding)
    for finding in findings:
        finding["severity"] = FINDING_SEVERITY.get(str(finding["kind"]), "medium")
    return findings


def missing_from_proto_finding(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    tech_parameter: dict[str, Any],
) -> dict[str, Any]:
    finding = finding_base("techdocs_parameter_missing_from_proto", key, route, tool)
    finding.update(
        {
            "parameter": tech_parameter.get("name"),
            "location": tech_parameter.get("location"),
            "techdocs": {
                "type": tech_parameter.get("type"),
                "documented_type": tech_parameter.get("raw_type"),
                "required": bool(tech_parameter.get("required")),
                "default": tech_parameter.get("default"),
                "enum": tech_parameter.get("enum", []),
            },
            "proto": None,
        }
    )
    return finding


def extra_in_proto_finding(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    proto_parameter: dict[str, Any],
) -> dict[str, Any]:
    finding = finding_base("proto_parameter_not_in_techdocs", key, route, tool)
    finding.update(
        {
            "parameter": proto_parameter.get("name"),
            "location": proto_parameter.get("location"),
            "proto_comment": proto_parameter.get("comment") or None,
            "techdocs": None,
            "proto": {
                "type": proto_parameter.get("type"),
                "required": proto_parameter.get("required"),
                "location": proto_parameter.get("location"),
            },
            "investigation_guidance": INVESTIGATION_GUIDANCE,
        }
    )
    return finding


def compare_path_parameters(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    proto_path: list[dict[str, Any]],
    tech_path: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    """Compare path parameters by the slot each one fills, never by name.

    A path slot is positional in the route both sides already agreed on, so
    position is the reliable join. Name matching here would report every
    word-segmentation difference as a missing parameter.
    """
    findings: list[dict[str, Any]] = []
    proto_slots, proto_unbound = order_path_slots(
        str(tool.get("path") or ""), proto_path
    )
    tech_slots, tech_unbound = order_path_slots(str(route.get("path") or ""), tech_path)

    if len(proto_slots) != len(tech_slots):
        finding = finding_base("path_parameter_count_mismatch", key, route, tool)
        finding.update(
            {
                "techdocs": path_placeholders(str(route.get("path") or "")),
                "proto": path_placeholders(str(tool.get("path") or "")),
            }
        )
        findings.append(finding)

    for position in range(max(len(proto_slots), len(tech_slots))):
        proto_parameter = proto_slots[position] if position < len(proto_slots) else None
        tech_parameter = tech_slots[position] if position < len(tech_slots) else None
        if proto_parameter is not None and tech_parameter is not None:
            for finding in compare_parameter(
                key,
                route,
                tool,
                proto_parameter,
                tech_parameter,
                matched_by="path_slot",
            ):
                finding["path_slot"] = position
                findings.append(finding)
        elif tech_parameter is not None:
            finding = missing_from_proto_finding(key, route, tool, tech_parameter)
            finding["path_slot"] = position
            findings.append(finding)
        elif proto_parameter is not None:
            finding = extra_in_proto_finding(key, route, tool, proto_parameter)
            finding["path_slot"] = position
            findings.append(finding)

    findings.extend(
        missing_from_proto_finding(key, route, tool, parameter)
        for parameter in tech_unbound
    )
    findings.extend(
        extra_in_proto_finding(key, route, tool, parameter)
        for parameter in proto_unbound
    )
    return findings


def take_candidate(
    index: dict[tuple[str, ...], list[dict[str, Any]]],
    lookup: tuple[str, ...],
    consumed: set[int],
) -> tuple[dict[str, Any] | None, int]:
    """Return the one unconsumed candidate under lookup, plus how many there were."""
    available = [item for item in index.get(lookup, []) if id(item) not in consumed]
    if len(available) != 1:
        return None, len(available)
    return available[0], 1


def compare_wire_parameters(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    proto_wire: list[dict[str, Any]],
    tech_wire: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    """Compare query and body parameters by name, tolerating word segmentation."""
    findings: list[dict[str, Any]] = []
    by_location_name: dict[tuple[str, ...], list[dict[str, Any]]] = defaultdict(list)
    by_location_letters: dict[tuple[str, ...], list[dict[str, Any]]] = defaultdict(list)
    by_name: dict[tuple[str, ...], list[dict[str, Any]]] = defaultdict(list)
    by_letters: dict[tuple[str, ...], list[dict[str, Any]]] = defaultdict(list)
    for parameter in tech_wire:
        location = str(parameter.get("location") or "")
        normalized = str(parameter.get("normalized_name") or "")
        letters = str(parameter.get("segmented_name") or "")
        rename = KNOWN_RENAMES.get((key[0], key[1], location, normalized))
        if rename is not None:
            parameter["renamed_to"] = rename["repo_name"]
            parameter["rename_reason"] = rename["reason"]
            normalized = rename["repo_name"]
            letters = strip_segmentation(normalized)
        by_location_name[(location, normalized)].append(parameter)
        by_location_letters[(location, letters)].append(parameter)
        by_name[(normalized,)].append(parameter)
        by_letters[(letters,)].append(parameter)

    consumed: set[int] = set()
    for proto_parameter in proto_wire:
        location = str(proto_parameter.get("location") or "")
        normalized = str(proto_parameter.get("normalized_name") or "")
        letters = str(proto_parameter.get("segmented_name") or "")
        attempts: list[
            tuple[dict[tuple[str, ...], list[dict[str, Any]]], tuple[str, ...]]
        ] = []
        if location:
            attempts.append((by_location_name, (location, normalized)))
            attempts.append((by_location_letters, (location, letters)))
        attempts.append((by_name, (normalized,)))
        attempts.append((by_letters, (letters,)))

        match: dict[str, Any] | None = None
        widest = 0
        for index, lookup in attempts:
            match, count = take_candidate(index, lookup, consumed)
            widest = max(widest, count)
            if match is not None:
                break
        if match is None:
            if widest > 1:
                finding = finding_base("parameter_location_ambiguous", key, route, tool)
                finding.update(
                    {
                        "parameter": proto_parameter.get("name"),
                        "proto_location": proto_parameter.get("location"),
                        "techdocs_locations": sorted(
                            str(candidate.get("location") or "")
                            for candidate in by_name.get((normalized,), [])
                            or by_letters.get((letters,), [])
                        ),
                    }
                )
                findings.append(finding)
                continue
            unrendered = unrendered_body_variants(route, location)
            if unrendered:
                findings.append(
                    body_variant_finding(
                        key,
                        route,
                        tool,
                        str(proto_parameter.get("name") or ""),
                        "the rendered variant declares no such body parameter",
                        unrendered,
                    )
                )
                continue
            findings.append(extra_in_proto_finding(key, route, tool, proto_parameter))
            continue
        consumed.add(id(match))
        findings.extend(compare_parameter(key, route, tool, proto_parameter, match))

    findings.extend(
        missing_from_proto_finding(key, route, tool, parameter)
        for parameter in tech_wire
        if id(parameter) not in consumed
    )
    return findings


def compare_tool_parameters(
    key: tuple[str, str],
    route: dict[str, Any],
    tool: dict[str, Any],
    tech_parameters: list[dict[str, Any]],
) -> tuple[list[dict[str, Any]], int, int]:
    findings: list[dict[str, Any]] = []
    system_count = 0
    tool_argument_count = 0
    proto_path: list[dict[str, Any]] = []
    proto_wire: list[dict[str, Any]] = []

    for parameter in tool.get("parameters", []):
        if parameter.get("system_parameter"):
            system_count += 1
            continue
        if parameter.get("location") == TOOL_ARGUMENT_LOCATION:
            tool_argument_count += 1
            continue
        if parameter.get("location") is None:
            finding = finding_base("proto_field_location_missing", key, route, tool)
            finding["parameter"] = parameter.get("name")
            findings.append(finding)
            continue
        if parameter["location"] == "path":
            proto_path.append(parameter)
        else:
            proto_wire.append(parameter)

    tech_path = [item for item in tech_parameters if item.get("location") == "path"]
    tech_wire = [item for item in tech_parameters if item.get("location") != "path"]

    findings.extend(compare_path_parameters(key, route, tool, proto_path, tech_path))
    findings.extend(compare_wire_parameters(key, route, tool, proto_wire, tech_wire))
    return findings, system_count, tool_argument_count


def compare_route_surface(
    key: tuple[str, str], route: dict[str, Any], tool: dict[str, Any]
) -> list[dict[str, Any]]:
    """Compare the surface a tool calls with the surfaces the route serves.

    A route that serves both surfaces answers either declaration, so the only
    report there is the note that the default surface would also work. The two
    real defects are a tool on the default surface calling a beta-only route,
    which fails at runtime, and a tool pinned to beta on a route that never
    served beta.
    """
    documented = str(route.get("api_surface") or "")
    if documented not in API_SURFACE_CLASSIFICATIONS:
        return []
    declared = str(tool.get("api_surface") or "")
    calls_beta = declared == "v4beta"
    if documented == SURFACE_BOTH:
        kind = "route_surface_downgrade_available" if calls_beta else ""
    elif documented == SURFACE_V4BETA_ONLY:
        kind = "" if calls_beta else "route_surface_mismatch"
    else:
        kind = "route_surface_mismatch" if calls_beta else ""
    if not kind:
        return []
    finding = finding_base(kind, key, route, tool)
    finding.update(
        {
            "techdocs": documented,
            "proto": declared,
            "declared_api_surface": tool.get("declared_api_surface"),
        }
    )
    return [finding]


def load_page_index(docs_repo: Path) -> dict[str, Path]:
    index_path = docs_repo / "url-index.json"
    raw = read_json(index_path)
    if not is_json_array(raw):
        raise ProofError(f"expected list in {index_path}")
    result: dict[str, Path] = {}
    for item in raw:
        if not is_json_object(item):
            continue
        url = str(item.get("url") or "")
        file_name = str(item.get("file") or "")
        if url and file_name:
            result[url] = docs_repo / file_name
    return result


def replacement_urls(source_url: str, page_index: dict[str, Path]) -> list[str]:
    page = page_index.get(source_url)
    if not page or not page.is_file():
        return []
    lines = page.read_text(encoding="utf-8", errors="replace").splitlines()
    declaration = None
    for index, line in enumerate(lines):
        if re.search(
            r"\b(?:get|post|put|delete)\s+deprecated\s+https://api\.linode\.com\b",
            line,
            re.IGNORECASE,
        ):
            declaration = index
            break
    if declaration is None:
        return []
    section: list[str] = []
    for line in lines[declaration + 1 : declaration + 80]:
        if line.strip() == "Permissions and scopes":
            break
        section.append(line)
    candidates: set[str] = set()
    for line in section:
        lowered = line.lower()
        if not (
            ("use the" in lowered and "instead" in lowered)
            or "replacement" in lowered
            or "replaced by" in lowered
        ):
            continue
        for slug in re.findall(
            r"\[link:\s*/linode-api/reference/([a-z0-9-]+)\]", line, re.IGNORECASE
        ):
            url = f"https://techdocs.akamai.com/linode-api/reference/{slug}"
            if url != source_url:
                candidates.add(url)
    return sorted(candidates)


def compare_contracts(
    techdocs_descriptor: dict[str, Any],
    proto_tools: dict[str, dict[str, Any]],
    extraction_findings: list[dict[str, Any]],
    docs_repo: Path | None,
) -> dict[str, Any]:
    routes, parameters, duplicate_findings = techdocs_indexes(techdocs_descriptor)
    findings: list[dict[str, Any]] = list(extraction_findings) + duplicate_findings
    proto_by_route: dict[tuple[str, str], list[dict[str, Any]]] = defaultdict(list)
    for tool in proto_tools.values():
        proto_by_route[(tool["method"], tool["route_shape"])].append(tool)
    for tools in proto_by_route.values():
        tools.sort(key=lambda item: str(item.get("tool") or ""))

    active_routes = {
        key: route for key, route in routes.items() if not bool(route.get("deprecated"))
    }
    deprecated_routes = {
        key: route for key, route in routes.items() if bool(route.get("deprecated"))
    }
    system_count = 0
    tool_argument_count = 0
    # A descriptor where no tool declares a surface predates the option, and
    # reading every tool there as v4 would report each beta-only route as a
    # repo defect. One declaration anywhere means absence is a real v4 claim.
    surface_declared = any(
        tool.get("declared_api_surface") for tool in proto_tools.values()
    )

    for key, route in sorted(active_routes.items()):
        tools = proto_by_route.get(key, [])
        if not tools:
            findings.append(finding_base("route_missing_from_proto", key, route))
            continue
        for tool in tools:
            if surface_declared:
                findings.extend(compare_route_surface(key, route, tool))
            parameter_findings, count, arguments = compare_tool_parameters(
                key, route, tool, parameters.get(key, [])
            )
            system_count += count
            tool_argument_count += arguments
            findings.extend(parameter_findings)

    all_tech_keys = set(routes)
    for key, tools in sorted(proto_by_route.items()):
        if key in all_tech_keys:
            continue
        findings.extend(
            finding_base("proto_route_absent_from_techdocs", key, None, tool)
            for tool in tools
        )

    page_index = load_page_index(docs_repo) if docs_repo else {}
    replacements_checked = 0
    for key, route in sorted(deprecated_routes.items()):
        findings.extend(
            finding_base("deprecated_route_still_in_proto", key, route, tool)
            for tool in proto_by_route.get(key, [])
        )
        source_url = str(route.get("source_url") or "")
        candidates = replacement_urls(source_url, page_index) if source_url else []
        for replacement_url in candidates:
            replacements_checked += 1
            replacement_records = [
                (candidate_key, candidate_route)
                for candidate_key, candidate_route in routes.items()
                if candidate_route.get("source_url") == replacement_url
            ]
            if not replacement_records:
                finding = finding_base(
                    "deprecated_replacement_missing_from_techdocs_snapshot", key, route
                )
                finding["replacement_url"] = replacement_url
                findings.append(finding)
                continue
            if len(replacement_records) != 1:
                finding = finding_base("deprecated_replacement_ambiguous", key, route)
                finding["replacement_url"] = replacement_url
                finding["candidate_routes"] = [
                    {"method": candidate_key[0], "path": candidate_key[1]}
                    for candidate_key, _ in replacement_records
                ]
                findings.append(finding)
                continue
            replacement_key, replacement_route = replacement_records[0]
            if replacement_route.get("deprecated"):
                finding = finding_base(
                    "deprecated_replacement_also_deprecated",
                    replacement_key,
                    replacement_route,
                )
                finding["deprecated_source_url"] = source_url
                findings.append(finding)
            if replacement_key not in proto_by_route:
                finding = finding_base(
                    "deprecated_replacement_missing_from_proto",
                    replacement_key,
                    replacement_route,
                )
                finding["deprecated_source_url"] = source_url
                findings.append(finding)

    def sort_key(finding: dict[str, Any]) -> tuple[str, ...]:
        return (
            str(finding.get("severity") or ""),
            str(finding.get("kind") or ""),
            str(finding.get("method") or ""),
            str(finding.get("path") or ""),
            str(finding.get("location") or ""),
            str(finding.get("parameter") or ""),
            str(finding.get("tool") or ""),
        )

    for finding in findings:
        finding.setdefault(
            "severity", FINDING_SEVERITY.get(str(finding.get("kind") or ""), "medium")
        )
    findings, unused_known = apply_known_divergences(findings)
    findings.sort(key=sort_key)
    # The exit code reads the count this set produces, so the two tiers named
    # here are the ones a scheduled run is allowed to fail on.
    actionable = {"high", "medium"}
    finding_routes = {
        (str(item.get("method") or ""), str(item.get("route_shape") or ""))
        for item in findings
    }
    exact_matches = sum(
        1
        for key in active_routes
        if key in proto_by_route and key not in finding_routes
    )
    actionable_routes = {
        (str(item.get("method") or ""), str(item.get("route_shape") or ""))
        for item in findings
        if item.get("severity") in actionable
    }
    contract_matches = sum(
        1
        for key in active_routes
        if key in proto_by_route and key not in actionable_routes
    )
    kind_counts: dict[str, int] = defaultdict(int)
    severity_counts: dict[str, int] = defaultdict(int)
    kinds_by_severity: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
    for finding in findings:
        kind = str(finding.get("kind") or "unknown")
        severity = str(finding.get("severity") or "medium")
        kind_counts[kind] += 1
        severity_counts[severity] += 1
        kinds_by_severity[severity][kind] += 1

    return {
        "source_authority": TECHDOCS_AUTHORITY,
        "status": "ok",
        "comparison_mode": "generated-techdocs-descriptor-vs-linodemcp-descriptor",
        "summary": {
            "techdocs_operations": len(routes),
            "active_routes_compared": len(active_routes),
            "deprecated_routes_checked": len(deprecated_routes),
            "replacement_routes_checked": replacements_checked,
            "proto_tools_compared": len(proto_tools),
            "surface_comparison_enabled": surface_declared,
            "techdocs_routes_with_surface_evidence": sum(
                1 for item in routes.values() if item.get("api_surface")
            ),
            # Counted rather than reported: a both-surfaces route answers either
            # declaration, so one finding per correct route would bury the two
            # surface defects under a page of rows that say nothing is wrong.
            "techdocs_routes_serving_both_surfaces": sum(
                1 for item in routes.values() if item.get("api_surface") == SURFACE_BOTH
            ),
            "proto_tools_declaring_v4beta": sum(
                1
                for item in proto_tools.values()
                if item.get("api_surface") == "v4beta"
            ),
            "exact_route_contract_matches": exact_matches,
            "routes_without_actionable_findings": contract_matches,
            "system_parameters": system_count,
            # Counted rather than reported: a tool-location argument reaches no
            # Linode route, so there is nothing on the documented side to join.
            "tool_arguments": tool_argument_count,
            "mismatches": len(findings),
            "actionable_findings": sum(
                1 for item in findings if item.get("severity") in actionable
            ),
            "known_divergences_unmatched": unused_known,
            "severities": dict(sorted(severity_counts.items())),
            "finding_kinds": dict(sorted(kind_counts.items())),
            "finding_kinds_by_severity": {
                severity: dict(sorted(kinds.items()))
                for severity, kinds in sorted(kinds_by_severity.items())
            },
        },
        "comparison_limits": [
            (
                "Both sides of every comparison are compiled protobuf descriptors. The "
                "scraped TechDocs pages feed proto generation only and are never "
                "compared directly."
            ),
            (
                "Tool name and HTTP method/path association come only from the "
                "linode.mcp.v1.tool_route protobuf message option."
            ),
            (
                "Path parameters are matched by slot position within the matched "
                "route, so a name difference in a path slot is reported as a spelling "
                "variant rather than a missing parameter."
            ),
            (
                "Query and body parameters are matched by name, falling back to a "
                "comparison with word separators removed; those matches are reported "
                "as segmentation variants at info severity."
            ),
            (
                "Divergences a triage confirmed are not repo defects carry severity "
                "known and a recorded reason; each entry is route-scoped and the "
                "summary lists any entry that no longer matches. A class rule "
                "states one reason over a written-out list of finding keys, so a "
                "key that stops appearing is reported the same way."
            ),
            (
                "Requiredness on the repo side reads the input message's declared "
                "buf.validate rule first and proto3 presence only where no rule "
                "names the field. A singular message field is presence-tracked, so "
                "an unruled one is read as optional the way the generated body "
                "builder treats it."
            ),
            (
                "Where the Body Params section renders a schema switcher, the "
                "fetched HTML carries one variant. The labels of the rest are "
                "recorded, and a body field or allowed value only the unrendered "
                "variants could document is reported at limitation severity."
            ),
            (
                "Proto3 repeated/map requiredness and documented defaults are reported "
                "at limitation severity: proto3 cannot express either, so the "
                "comparison cannot judge them."
            ),
            (
                "API surface is compared only where both sides state it: the "
                "documented apiVersion Allowed list classifies the route as v4_only, "
                "v4beta_only, or both, and the linode.mcp.v1.tool_api_surface message "
                "option states what the tool calls. A page without an Allowed list and "
                "a descriptor where no tool declares a surface both produce no surface "
                "findings."
            ),
            (
                "Response status comparison is not proven because LinodeMCP proto "
                "operation metadata does not encode HTTP responses."
            ),
        ],
        "findings": findings,
    }


def git_head(repo: Path) -> str | None:
    process = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
        timeout=30,
    )
    return process.stdout.strip() if process.returncode == 0 else None


def find_executable(command: str) -> str:
    """Resolve a tool name on PATH, or accept an explicit path as given."""
    executable = shutil.which(command) if "/" not in command else command
    if not executable or not Path(executable).is_file():
        raise ProofError(f"executable not found: {command}")
    return str(executable)


@contextlib.contextmanager
def resolve_linodemcp_source(
    args: argparse.Namespace,
) -> Generator[tuple[Path, dict[str, Any]]]:
    if not args.github_source:
        repo = args.linodemcp_repo.expanduser().resolve()
        if not (repo / "buf.yaml").is_file() or not (repo / "proto").is_dir():
            raise ProofError(f"local LinodeMCP proto source is incomplete: {repo}")
        status = subprocess.run(
            ["git", "status", "--porcelain", "--", "proto", "buf.yaml", "buf.lock"],
            cwd=repo,
            text=True,
            capture_output=True,
            check=False,
            timeout=30,
        )
        if status.returncode != 0:
            raise ProofError(
                f"cannot inspect local LinodeMCP source: {status.stderr.strip()}"
            )
        yield (
            repo,
            {
                "kind": "local-working-tree",
                "path": str(repo),
                "commit_sha": git_head(repo),
                "dirty": bool(status.stdout.strip()),
            },
        )
        return

    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.github_repository):
        raise ProofError(f"invalid GitHub repository: {args.github_repository!r}")
    gh = find_executable(args.gh)
    commit = subprocess.run(
        [
            gh,
            "api",
            f"repos/{args.github_repository}/commits/{args.github_ref}",
            "--jq",
            ".sha",
        ],
        text=True,
        capture_output=True,
        check=False,
        timeout=120,
    )
    sha = commit.stdout.strip()
    if commit.returncode != 0 or not re.fullmatch(r"[0-9a-f]{40}", sha):
        detail = (commit.stderr or commit.stdout or "invalid commit response").strip()
        raise ProofError(f"cannot resolve GitHub source commit: {detail}")

    with tempfile.TemporaryDirectory(prefix="linodemcp-github-source-") as directory:
        temporary = Path(directory)
        archive = temporary / "source.tar.gz"
        with archive.open("wb") as output:
            fetched = subprocess.run(
                [gh, "api", f"repos/{args.github_repository}/tarball/{sha}"],
                stdout=output,
                stderr=subprocess.PIPE,
                check=False,
                timeout=300,
            )
        if fetched.returncode != 0:
            raise ProofError(
                "cannot fetch immutable GitHub source archive: "
                + fetched.stderr.decode("utf-8", errors="replace").strip()
            )
        extracted = temporary / "source"
        extracted.mkdir()
        try:
            with tarfile.open(archive, "r:gz") as bundle:
                bundle.extractall(extracted, filter="data")
        except (tarfile.TarError, OSError) as exc:
            raise ProofError(f"cannot extract GitHub source archive: {exc}") from exc
        roots = [path for path in extracted.iterdir() if path.is_dir()]
        if len(roots) != 1:
            raise ProofError(
                f"GitHub source archive has {len(roots)} roots; expected one"
            )
        source = roots[0]
        if not (source / "buf.yaml").is_file() or not (source / "proto").is_dir():
            raise ProofError("GitHub source archive has no complete Buf proto module")
        yield (
            source,
            {
                "kind": "github-archive",
                "repository": args.github_repository,
                "ref": args.github_ref,
                "commit_sha": sha,
                "archive_sha256": hashlib.sha256(archive.read_bytes()).hexdigest(),
                "archive_bytes": archive.stat().st_size,
            },
        )


def prebuilt_descriptor_metadata(path: Path) -> dict[str, Any]:
    """Describe a descriptor built elsewhere, so the run still records provenance.

    A descriptor built from a working tree cannot be pinned to a commit, so the
    record says so rather than implying an immutable source.
    """
    resolved = path.expanduser().resolve()
    if not resolved.is_file():
        raise ProofError(f"prebuilt LinodeMCP descriptor is missing: {resolved}")
    payload = resolved.read_bytes()
    return {
        "kind": "prebuilt-descriptor",
        "path": str(resolved),
        "commit_sha": None,
        "descriptor_sha256": hashlib.sha256(payload).hexdigest(),
        "descriptor_bytes": len(payload),
    }


def preserve_proto_source(source: Path, destination: Path) -> dict[str, Any]:
    if destination.exists():
        raise ProofError(f"proto source destination already exists: {destination}")
    destination.mkdir(parents=True)
    shutil.copytree(source / "proto", destination / "proto")
    copied = ["buf.yaml"]
    shutil.copy2(source / "buf.yaml", destination / "buf.yaml")
    if (source / "buf.lock").is_file():
        shutil.copy2(source / "buf.lock", destination / "buf.lock")
        copied.append("buf.lock")
    proto_files = sorted((destination / "proto").rglob("*.proto"))
    if not proto_files:
        raise ProofError("preserved LinodeMCP source contains no .proto files")
    return {
        "directory": str(destination),
        "module_files": copied,
        "proto_files": len(proto_files),
    }


def proto_quote(value: Any) -> str:
    return json.dumps(str(value), ensure_ascii=True)


def proto_message_name(endpoint: dict[str, Any], used: set[str]) -> str:
    raw = f"{endpoint['method']}_{endpoint['operation_id']}"
    name = "".join(
        part.capitalize() for part in re.split(r"[^A-Za-z0-9]+", raw) if part
    )
    if not name or name[0].isdigit():
        name = "Operation" + name
    candidate = name + "Request"
    if candidate in used:
        digest = hashlib.sha256(
            f"{endpoint['method']} {endpoint['path']}".encode()
        ).hexdigest()[:8]
        candidate += digest.capitalize()
    used.add(candidate)
    return candidate


def proto_field_spec(raw_type: str) -> tuple[str, bool]:
    normalized = re.sub(r"\s*,\s*unique$", "", raw_type.strip().lower())
    normalized = re.sub(r"\s*(?:\|\s*null|or null)$", "", normalized)
    repeated = normalized == "array" or normalized.startswith("array of ")
    item = normalized.removeprefix("array of ").rstrip("s") if repeated else normalized
    scalar = {
        "boolean": "bool",
        "date-time": "string",
        "integer": "int64",
        "number": "double",
        MASKED_STRING_FORMAT: "string",
        "string": "string",
        "url": "string",
        "uuid": "string",
    }.get(item, "google.protobuf.Value")
    return scalar, repeated


def proto_field_name(name: str, location: str, used: set[str]) -> str:
    candidate = normalize_name(name) or "parameter"
    if candidate[0].isdigit():
        candidate = "parameter_" + candidate
    if candidate in {
        "package",
        "message",
        "enum",
        "service",
        "option",
        "import",
        "reserved",
        "repeated",
        "optional",
        "required",
        "extensions",
        "extend",
        "returns",
        "rpc",
        "stream",
        "syntax",
        "to",
        "max",
    }:
        candidate += "_parameter"
    if candidate in used:
        candidate += "_" + normalize_name(location)
    suffix = 2
    unique = candidate
    while unique in used:
        unique = f"{candidate}_{suffix}"
        suffix += 1
    used.add(unique)
    return unique


def render_techdocs_proto(endpoints: list[dict[str, Any]]) -> str:
    lines = [
        'syntax = "proto3";',
        "",
        "package techdocs.linode.api;",
        "",
        'import "google/protobuf/descriptor.proto";',
        'import "google/protobuf/struct.proto";',
        "",
        "message OperationContract {",
        "  string api_version = 1;",
        "  string method = 2;",
        "  string path = 3;",
        "  bool is_deprecated = 4;",
        "  string source_url = 5;",
        "  repeated int32 status_codes = 6;",
        "  // Which surfaces the documented apiVersion parameter allows:",
        "  // v4_only, v4beta_only, or both. Empty when the page never rendered",
        "  // an Allowed list to classify.",
        "  string api_surface = 7;",
        "  // Labels of the schema switcher over the whole Body Params section,",
        "  // and the one label the fetched HTML carried fields for. The rest",
        "  // are named by the page and rendered only in a browser.",
        "  repeated string body_variants = 8;",
        "  string rendered_body_variant = 9;",
        "}",
        "",
        "message ParameterContract {",
        "  string location = 1;",
        "  string documented_type = 2;",
        "  bool is_required = 3;",
        "  bool has_default = 4;",
        "  string default_json = 5;",
        "  repeated string enum_values = 6;",
        "  bool is_deprecated = 7;",
        "  // The name as the documentation spells it. The field name above it",
        "  // may have been rewritten to dodge a proto keyword or a collision",
        "  // between two locations, and comparing that rewritten name would",
        "  // report the generator's own mangling as a repo divergence.",
        "  string documented_name = 8;",
        "}",
        "",
        "extend google.protobuf.MessageOptions {",
        "  OperationContract operation = 51001;",
        "}",
        "",
        "extend google.protobuf.FieldOptions {",
        "  ParameterContract api_parameter = 51002;",
        "}",
        "",
    ]
    used_messages: set[str] = set()
    for endpoint in sorted(
        endpoints, key=lambda item: (item["api_version"], item["path"], item["method"])
    ):
        message_name = proto_message_name(endpoint, used_messages)
        lines.append(f"message {message_name} {{")
        lines.extend(
            [
                "  option (operation) = {",
                f"    api_version: {proto_quote(endpoint['api_version'])}",
                f"    method: {proto_quote(endpoint['method'])}",
                # raw_path keeps the documented placeholder names; endpoint
                # ["path"] has already collapsed them to {param}, which would
                # leave the generated proto with no way to say which slot a
                # path parameter fills.
                f"    path: {proto_quote(normalize_path(endpoint['raw_path']))}",
                f"    is_deprecated: {str(bool(endpoint['deprecated'])).lower()}",
                f"    source_url: {proto_quote(endpoint['docs_url'])}",
            ]
        )
        surface = endpoint_api_surface(endpoint)
        if surface:
            lines.append(f"    api_surface: {proto_quote(surface)}")
        variant_labels: list[Any] = endpoint.get("body_variants") or []
        body_variants = [str(label) for label in variant_labels]
        lines.extend(
            f"    body_variants: {proto_quote(label)}" for label in body_variants
        )
        if body_variants:
            rendered = str(endpoint.get("rendered_body_variant") or "")
            lines.append(f"    rendered_body_variant: {proto_quote(rendered)}")
        lines.extend(
            f"    status_codes: {int(status)}"
            for status in endpoint.get("status_codes", [])
        )
        lines.extend(["  };", ""])
        field_number = 1
        used_fields: set[str] = set()
        defaults = endpoint.get("parameter_defaults", {})
        for location in ("path", "query", "body"):
            for parameter in endpoint["parameters"].get(location, []):
                field_name = proto_field_name(parameter["name"], location, used_fields)
                scalar, repeated = proto_field_spec(parameter["type"])
                qualifier = (
                    "repeated "
                    if repeated
                    else ("" if parameter["required"] else "optional ")
                )
                default_key = f"{location}.{parameter['name']}"
                has_default = default_key in defaults
                documented_name = proto_quote(normalize_name(parameter["name"]))
                option_parts = [
                    f"location: {proto_quote(location)}",
                    f"documented_name: {documented_name}",
                    f"documented_type: {proto_quote(parameter['type'])}",
                    f"is_required: {str(bool(parameter['required'])).lower()}",
                    f"has_default: {str(has_default).lower()}",
                ]
                if has_default:
                    default_json = json.dumps(
                        defaults[default_key], sort_keys=True, separators=(",", ":")
                    )
                    option_parts.append(f"default_json: {proto_quote(default_json)}")
                option_parts.extend(
                    f"enum_values: {proto_quote(value)}"
                    for value in parameter.get("enum", [])
                )
                option_parts.append(
                    f"is_deprecated: {str(bool(parameter.get('deprecated'))).lower()}"
                )
                lines.append(
                    f"  {qualifier}{scalar} {field_name} = {field_number} "
                    f"[(api_parameter) = {{ {' '.join(option_parts)} }}];"
                )
                field_number += 1
        lines.extend(["}", ""])
    return "\n".join(lines)


def build_techdocs_proto_artifacts(
    run_dir: Path, endpoints: list[dict[str, Any]], buf_command: str
) -> tuple[dict[str, Any], dict[str, Any]]:
    """Generate, compile, and hand back the TechDocs descriptor for comparison.

    Returning the descriptor is the whole point: an earlier version compiled it
    and then compared the scraped dictionary instead, so nothing ever checked
    what the generated proto actually said.
    """
    proto_dir = run_dir / "generated-proto"
    proto_dir.mkdir(parents=True, exist_ok=True)
    proto_path = proto_dir / TECHDOCS_PROTO_FILE
    proto_path.write_text(render_techdocs_proto(endpoints), encoding="utf-8")
    (proto_dir / "buf.yaml").write_text(
        "version: v2\nmodules:\n  - path: .\n", encoding="utf-8"
    )
    descriptor_path = run_dir / "techdocs-descriptor.json"
    descriptor = run_buf_descriptor(proto_dir, buf_command, descriptor_path)
    return {
        "proto": str(proto_path),
        "descriptor": str(descriptor_path),
        "descriptor_files": len(descriptor.get("file", [])),
    }, descriptor


def evidence_root(
    explicit: Path | None = None,
    environ: Mapping[str, str] | None = None,
) -> Path:
    """Host-local directory holding dated runs, latest.json, and the sweep.

    A run writes tens of megabytes of scraped pages and descriptors. Landing
    that in the repository would dirty the very tree the comparison measures,
    so a root inside the repository is refused rather than defaulted away
    from. Only reviewed harvest snapshots belong in the repository.
    """
    env = os.environ if environ is None else environ
    if explicit is not None:
        root = explicit
    elif env.get(EVIDENCE_ROOT_ENV):
        root = Path(env[EVIDENCE_ROOT_ENV])
    else:
        root = Path(env.get("XDG_DATA_HOME") or "~/.local/share") / EVIDENCE_ROOT_SUFFIX
    resolved = root.expanduser().resolve()
    if resolved == REPO_ROOT or REPO_ROOT in resolved.parents:
        raise ProofError(
            f"evidence root must sit outside the repository, got {resolved}"
        )
    return resolved


def cleanup_old_runs(root: Path, retention_days: int, today: datetime) -> list[str]:
    removed: list[str] = []
    cutoff = today.date().toordinal() - retention_days
    if not root.exists():
        return removed
    for child in sorted(root.iterdir()):
        if not child.is_dir() or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", child.name):
            continue
        try:
            run_date = date.fromisoformat(child.name)
        except ValueError:
            continue
        if run_date.toordinal() < cutoff:
            shutil.rmtree(child)
            removed.append(str(child))
    return removed


def create_run_directory(root: Path, now: datetime) -> Path:
    date_dir = root / now.strftime("%Y-%m-%d")
    date_dir.mkdir(parents=True, exist_ok=True)
    stem = now.strftime("%Y%m%dT%H%M%SZ")
    candidate = date_dir / stem
    suffix = 2
    while candidate.exists():
        candidate = date_dir / f"{stem}-{suffix}"
        suffix += 1
    candidate.mkdir()
    return candidate


def write_artifact_checksums(run_dir: Path) -> Path:
    output = run_dir / "checksums.sha256"
    records: list[str] = []
    for path in sorted(run_dir.rglob("*")):
        if not path.is_file() or path.name in {"checksums.sha256", "run.json"}:
            continue
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        records.append(f"{digest}  {path.relative_to(run_dir).as_posix()}")
    output.write_text("\n".join(records) + "\n", encoding="utf-8")
    return output


def read_endpoint_index(path: Path) -> list[Any]:
    """Parsed endpoints from a saved index, refusing an empty or wrong shape."""
    endpoints = read_json(path)
    if not is_json_array(endpoints) or not endpoints:
        raise ProofError(f"endpoint index is empty: {path}")
    return endpoints


def findings_exit_code(fail_on_findings: bool, summary: dict[str, Any]) -> int:
    """Exit 3 for the tiers a caller can act on, and only those.

    known, limitation, and info each record a disagreement someone already read
    and accepted, so a gate that counted them would be red every week. The count
    comes from the summary the run publishes, the same number the scheduled job
    prints as ``actionable (medium or high) findings``, so the exit code and
    the report cannot disagree. Note what the count leaves out: a ledger entry
    that stopped matching lands in ``known_divergences_unmatched`` and reaches
    no finding, so it never reaches this exit code either.
    """
    if not fail_on_findings:
        return 0
    return 3 if summary["actionable_findings"] else 0


def run_scrape_mode(args: argparse.Namespace) -> int:
    if args.retention_days < 0:
        raise ProofError("--retention-days must be non-negative")
    if args.workers < 1:
        raise ProofError("--workers must be positive")
    if args.max_pages is not None and args.max_pages < 1:
        raise ProofError("--max-pages must be positive")
    now = datetime.now(UTC).replace(microsecond=0)
    root = evidence_root(args.evidence_root)
    root.mkdir(parents=True, exist_ok=True)
    removed = cleanup_old_runs(root, args.retention_days, now)
    run_dir = create_run_directory(root, now)
    manifest: dict[str, Any] = {
        "schema": "linodeapi.techdocs_proto_proof.run.v1",
        "status": "running",
        "started_at": now.isoformat(),
        "run_dir": str(run_dir),
        "removed_old_runs": removed,
    }
    write_json(run_dir / "run.json", manifest)
    write_json(root / "latest.json", manifest)
    try:
        linodemcp_descriptor_path = run_dir / "linodemcp-descriptor.json"
        preserved_source: Path | None = None
        if args.linodemcp_descriptor is not None:
            source_metadata = prebuilt_descriptor_metadata(args.linodemcp_descriptor)
            descriptor = read_json(args.linodemcp_descriptor)
            write_json(linodemcp_descriptor_path, descriptor)
        else:
            with resolve_linodemcp_source(args) as (source, metadata):
                source_metadata = metadata
                preserved_source = run_dir / "linodemcp-proto-source"
                source_metadata["preserved"] = preserve_proto_source(
                    source, preserved_source
                )
            descriptor = run_buf_descriptor(
                preserved_source, args.buf, linodemcp_descriptor_path
            )
        source_metadata["resolved_at"] = utc_now()
        write_json(run_dir / "linodemcp-source.json", source_metadata)
        tools, extraction_findings = extract_proto_tools(descriptor)
        if args.endpoint_index is not None:
            # Replay: the saved endpoints already are the parsed pages, so proto
            # generation can run without refetching the whole documentation site.
            endpoints = read_endpoint_index(args.endpoint_index)
            pages_root = args.endpoint_index.expanduser().resolve().parent
        else:
            _contract, endpoints = scrape_techdocs(
                run_dir, args.workers, args.max_pages
            )
            pages_root = run_dir
        techdocs_proto, techdocs_descriptor = build_techdocs_proto_artifacts(
            run_dir, endpoints, args.buf
        )
        result = compare_contracts(
            techdocs_descriptor, tools, extraction_findings, pages_root
        )
        result.update(
            {
                "started_at": manifest["started_at"],
                "ran_at": utc_now(),
                "repo_sha": source_metadata.get("commit_sha"),
                "execution": {
                    "mode": "replay" if args.endpoint_index else "scrape",
                    "host": socket.gethostname(),
                    "python": sys.version.split()[0],
                    "buf": args.buf,
                    "linodemcp_source": source_metadata,
                },
                "artifacts": {
                    "run_dir": str(run_dir),
                    "techdocs_contract": str(run_dir / "techdocs-contracts.json"),
                    "techdocs_proto": techdocs_proto,
                    "linodemcp_source": str(run_dir / "linodemcp-source.json"),
                    "linodemcp_proto_source": (
                        str(preserved_source) if preserved_source else None
                    ),
                    "linodemcp_descriptor": str(linodemcp_descriptor_path),
                    "route_snapshot": str(run_dir / ROUTE_SNAPSHOT_NAME),
                    "comparison": str(run_dir / "comparison.json"),
                    "checksums": str(run_dir / "checksums.sha256"),
                },
            }
        )
        write_json(run_dir / "comparison.json", result)
        # The candidate snapshot rides along with the run so the scheduled job
        # has a refresh to diff and upload without a second invocation.
        write_route_snapshot(
            read_json(run_dir / "techdocs-contracts.json"),
            run_dir / ROUTE_SNAPSHOT_NAME,
            now.date().isoformat(),
        )
        write_artifact_checksums(run_dir)
        manifest.update(
            {
                "status": "ok",
                "completed_at": result["ran_at"],
                "report": str(run_dir / "comparison.json"),
                "repo_sha": result["repo_sha"],
                "summary": result["summary"],
                "artifacts": result["artifacts"],
            }
        )
        write_json(run_dir / "run.json", manifest)
        write_json(root / "latest.json", manifest)
        if args.output:
            write_json(args.output, result)
        print(json.dumps(manifest, indent=2, sort_keys=True))
        return findings_exit_code(args.fail_on_findings, result["summary"])
    except Exception as exc:
        manifest.update(
            {
                "status": "error",
                "completed_at": utc_now(),
                "error": str(exc),
            }
        )
        write_json(run_dir / "run.json", manifest)
        write_json(root / "latest.json", manifest)
        raise


def run_self_test() -> None:
    # Every check below is an assertion, so -O would leave a gate that
    # verifies nothing and still prints SELF_TEST_OK.
    if not __debug__:
        raise ProofError("self-test needs assertions enabled; rerun without -O")

    descriptor_fixture = {
        "file": [
            {
                "name": "linode/mcp/v1/options.proto",
                "package": "linode.mcp.v1",
                "extension": [
                    {
                        "name": "tool_route",
                        "number": 50001,
                        "type": "TYPE_MESSAGE",
                        "typeName": ".linode.mcp.v1.ToolRoute",
                        "extendee": ".google.protobuf.MessageOptions",
                    }
                ],
                "sourceCodeInfo": {"location": [{"path": [7, 0]}]},
            },
            {
                "name": "linode/mcp/v1/widget.proto",
                "package": "linode.mcp.v1",
                "messageType": [
                    {
                        "name": "WidgetGetInput",
                        "options": {
                            TOOL_ROUTE_OPTION: {
                                "tool": "linode_widget_get",
                                "method": "GET",
                                "path": "/widgets/{widget_id}",
                            }
                        },
                        "field": [
                            {
                                "name": "widget_id",
                                "jsonName": "widgetId",
                                "number": 1,
                                "label": "LABEL_OPTIONAL",
                                "type": "TYPE_INT32",
                                "options": {
                                    FIELD_LOCATION_OPTION: "FIELD_LOCATION_PATH"
                                },
                            },
                            {
                                "name": "environment",
                                "jsonName": "environment",
                                "number": 2,
                                "label": "LABEL_OPTIONAL",
                                "type": "TYPE_STRING",
                                "proto3Optional": True,
                                "options": {
                                    FIELD_LOCATION_OPTION: "FIELD_LOCATION_LOCAL"
                                },
                            },
                        ],
                    }
                ],
                "sourceCodeInfo": {
                    "location": [
                        {
                            "path": [4, 0],
                            "leadingComments": "Input contract.",
                        },
                        {
                            "path": [4, 0, 2, 0],
                            "leadingComments": "Widget ID.",
                        },
                    ]
                },
            },
        ]
    }
    tools, extraction_findings = extract_proto_tools(descriptor_fixture)
    assert not extraction_findings
    tool = tools["linode_widget_get"]
    assert tool["method"] == "GET"
    assert tool["path"] == "/widgets/{widget_id}", tool["path"]
    assert tool["route_shape"] == "/widgets/{}", tool["route_shape"]
    assert tool["parameters"][0]["json_name"] == "widgetId"
    assert tool["parameters"][0]["location"] == "path"
    assert tool["parameters"][1]["system_parameter"] is True

    # A field with no field_location option falls back to the comment marker,
    # which is the only path that reads is_system_parameter. LinodeMCP writes
    # that marker trailing so it stays out of the generated tool description.
    marker_descriptor = json.loads(json.dumps(descriptor_fixture))
    marker_file = marker_descriptor["file"][1]
    del marker_file["messageType"][0]["field"][1]["options"]
    marker_file["sourceCodeInfo"]["location"].append(
        {"path": [4, 0, 2, 1], "trailingComments": " system param"}
    )
    marker_tools, _ = extract_proto_tools(marker_descriptor)
    marker_parameter = marker_tools["linode_widget_get"]["parameters"][1]
    assert marker_parameter["system_parameter"] is True, marker_parameter
    assert marker_parameter["location"] is None, marker_parameter

    unmarked_descriptor = json.loads(json.dumps(marker_descriptor))
    unmarked_file = unmarked_descriptor["file"][1]
    unmarked_file["sourceCodeInfo"]["location"][-1] = {
        "path": [4, 0, 2, 1],
        "trailingComments": " the caller picks the environment",
    }
    unmarked_tools, _ = extract_proto_tools(unmarked_descriptor)
    unmarked_parameter = unmarked_tools["linode_widget_get"]["parameters"][1]
    assert unmarked_parameter["system_parameter"] is False, unmarked_parameter

    assert is_system_parameter("System parameter: injected by the server")
    assert is_system_parameter("system param")
    assert not is_system_parameter("the caller sets this")

    assert normalize_path("/v4/widgets/{widgetId}") == "/widgets/{widgetId}"
    assert path_shape("/widgets/{widgetId}") == "/widgets/{}"
    assert strip_segmentation("nodeBalancerId") == strip_segmentation("nodebalancer_id")

    # A tool with no surface option calls the default surface, and the option is
    # read back exactly as the descriptor spells it.
    assert tool["api_surface"] == "v4", tool
    assert tool["declared_api_surface"] is None, tool
    beta_descriptor = json.loads(json.dumps(descriptor_fixture))
    beta_message = beta_descriptor["file"][1]["messageType"][0]
    beta_message["options"][TOOL_API_SURFACE_OPTION] = "API_SURFACE_V4BETA"
    beta_tools, _ = extract_proto_tools(beta_descriptor)
    assert beta_tools["linode_widget_get"]["api_surface"] == "v4beta", beta_tools
    assert (
        beta_tools["linode_widget_get"]["declared_api_surface"] == "API_SURFACE_V4BETA"
    ), beta_tools

    unspecified_descriptor = json.loads(json.dumps(descriptor_fixture))
    unspecified_descriptor["file"][1]["messageType"][0]["options"][
        TOOL_API_SURFACE_OPTION
    ] = "API_SURFACE_UNSPECIFIED"
    unspecified_tools, _ = extract_proto_tools(unspecified_descriptor)
    assert unspecified_tools["linode_widget_get"]["api_surface"] == "v4"

    # An unreadable surface value fails closed. Defaulting it to v4 would hide
    # the beta tools this comparison exists to check.
    unknown_surface = json.loads(json.dumps(descriptor_fixture))
    unknown_surface["file"][1]["messageType"][0]["options"][TOOL_API_SURFACE_OPTION] = (
        "API_SURFACE_V5"
    )
    try:
        extract_proto_tools(unknown_surface)
    except ProofError:
        pass
    else:
        raise AssertionError("an unknown tool_api_surface value must fail")

    # A renumbered surface extension fails rather than reading as no surface.
    wrong_extension = json.loads(json.dumps(descriptor_fixture))
    wrong_extension["file"][0]["extension"].append(
        {
            "name": TOOL_API_SURFACE_EXTENSION_NAME,
            "number": 50026,
            "type": "TYPE_ENUM",
            "extendee": ".google.protobuf.MessageOptions",
        }
    )
    try:
        extract_proto_tools(wrong_extension)
    except ProofError:
        pass
    else:
        raise AssertionError("a misnumbered tool_api_surface extension must fail")

    def techdocs_fixture(
        parameters: list[dict[str, Any]],
        path: str = "/v4/widgets/{widgetId}",
        api_surface: str | None = None,
    ) -> dict[str, Any]:
        operation: dict[str, Any] = {
            "apiVersion": "v4",
            "method": "GET",
            "path": path,
            "sourceUrl": "https://techdocs.akamai.com/linode-api/reference/get-widget",
        }
        if api_surface is not None:
            operation["apiSurface"] = api_surface
        return {
            "file": [
                {
                    "name": "techdocs_contract.proto",
                    "package": "techdocs.linode.api",
                    "extension": [
                        {
                            "name": "operation",
                            "number": 51001,
                            "typeName": ".techdocs.linode.api.OperationContract",
                            "extendee": ".google.protobuf.MessageOptions",
                        },
                        {
                            "name": "api_parameter",
                            "number": 51002,
                            "typeName": ".techdocs.linode.api.ParameterContract",
                            "extendee": ".google.protobuf.FieldOptions",
                        },
                    ],
                    "messageType": [
                        {
                            "name": "GetGetWidgetRequest",
                            "options": {TECHDOCS_OPERATION_OPTION: operation},
                            "field": parameters,
                        }
                    ],
                }
            ]
        }

    # The documented path parameter is spelled widgetId where the repo spells it
    # widget_id, and it fills the same slot, so slot matching must find it.
    path_parameter = {
        "name": "widget_id",
        "jsonName": "widgetId",
        "number": 1,
        "label": "LABEL_OPTIONAL",
        "type": "TYPE_INT64",
        "options": {
            TECHDOCS_PARAMETER_OPTION: {
                "location": "path",
                "documentedType": "integer",
                "isRequired": True,
            }
        },
    }
    result = compare_contracts(techdocs_fixture([path_parameter]), tools, [], None)
    assert result["summary"]["mismatches"] == 0, result["findings"]
    assert result["summary"]["system_parameters"] == 1

    # A body parameter that differs only in word segmentation is a variant, and
    # never a parameter the repo proto is missing.
    segmented = json.loads(json.dumps(tools))
    segmented["linode_widget_get"]["parameters"].append(
        {
            "name": "nodebalancer_id",
            "json_name": "nodebalancerId",
            "normalized_name": "nodebalancer_id",
            "segmented_name": "nodebalancerid",
            "location": "body",
            "type": "integer",
            "proto_type": "TYPE_INT64",
            "required": True,
            "repeated": False,
            "map": False,
            "enum": [],
            "deprecated": False,
            "default": None,
            "comment": "",
            "system_parameter": False,
        }
    )
    body_parameter = {
        "name": "node_balancer_id",
        "jsonName": "nodeBalancerId",
        "number": 2,
        "label": "LABEL_OPTIONAL",
        "type": "TYPE_INT64",
        "options": {
            TECHDOCS_PARAMETER_OPTION: {
                "location": "body",
                "documentedType": "integer",
                "isRequired": True,
            }
        },
    }
    result = compare_contracts(
        techdocs_fixture([path_parameter, body_parameter]), segmented, [], None
    )
    kinds = {item["kind"] for item in result["findings"]}
    assert kinds == {"parameter_name_segmentation_variant"}, result["findings"]
    assert result["summary"]["severities"] == {"info": 1}, result["summary"]
    assert not [
        item
        for item in result["findings"]
        if item["kind"] == "techdocs_parameter_missing_from_proto"
    ]

    # A documented parameter with no proto field at all is still a real gap.
    extra = {
        "name": "region",
        "jsonName": "region",
        "number": 3,
        "label": "LABEL_OPTIONAL",
        "type": "TYPE_STRING",
        "options": {
            TECHDOCS_PARAMETER_OPTION: {
                "location": "query",
                "documentedType": "string",
                "enumValues": ["us-east", "us-west"],
            }
        },
    }
    result = compare_contracts(
        techdocs_fixture([path_parameter, extra]), tools, [], None
    )
    missing = [
        item
        for item in result["findings"]
        if item["kind"] == "techdocs_parameter_missing_from_proto"
    ]
    assert len(missing) == 1, result["findings"]
    assert missing[0]["severity"] == "high"
    assert missing[0]["techdocs"]["enum"] == ["us-east", "us-west"]

    # A proto field the documentation does not mention keeps its guidance text.
    unknown = json.loads(json.dumps(tools))
    unknown["linode_widget_get"]["parameters"][1]["system_parameter"] = False
    unknown["linode_widget_get"]["parameters"][1]["location"] = "query"
    result = compare_contracts(techdocs_fixture([path_parameter]), unknown, [], None)
    findings = [
        item
        for item in result["findings"]
        if item["kind"] == "proto_parameter_not_in_techdocs"
    ]
    assert len(findings) == 1, result["findings"]
    assert findings[0]["investigation_guidance"] == INVESTIGATION_GUIDANCE

    # The page rendered one body variant, so the documented side never carried
    # the others and the comparison says so instead of calling the field extra.
    variant_body = json.loads(json.dumps(unknown))
    variant_body["linode_widget_get"]["parameters"][1]["location"] = "body"
    variant_body["linode_widget_get"]["parameters"][1]["enum"] = ["one", "two"]
    variant_fixture = techdocs_fixture([path_parameter])
    variant_operation = variant_fixture["file"][0]["messageType"][0]["options"][
        TECHDOCS_OPERATION_OPTION
    ]
    variant_operation["bodyVariants"] = ["UDP", "TCP"]
    variant_operation["renderedBodyVariant"] = "UDP"
    result = compare_contracts(variant_fixture, variant_body, [], None)
    assert [item["kind"] for item in result["findings"]] == [
        "techdocs_body_variant_unrendered"
    ], result["findings"]
    assert result["findings"][0]["severity"] == "limitation"
    assert result["findings"][0]["techdocs"]["unrendered_body_variants"] == ["TCP"]

    # A query field on the same route keeps its finding: the switcher governs
    # the body section only.
    query_on_variant_route = json.loads(json.dumps(unknown))
    result = compare_contracts(variant_fixture, query_on_variant_route, [], None)
    assert [item["kind"] for item in result["findings"]] == [
        "proto_parameter_not_in_techdocs"
    ], result["findings"]

    # On a switched section the unrendered variants explain a longer proto
    # list; a value the page never names anywhere is still a disagreement.
    enum_tool = json.loads(json.dumps(unknown))
    enum_tool["linode_widget_get"]["parameters"][1]["location"] = "body"
    enum_tool["linode_widget_get"]["parameters"][1]["enum"] = ["a", "b"]
    for documented, expected in (
        (["a"], "techdocs_body_variant_unrendered"),
        (["a", "b", "c"], "parameter_enum_mismatch"),
    ):
        documented_parameter = json.loads(json.dumps(body_parameter))
        documented_parameter["name"] = "environment"
        documented_parameter["jsonName"] = "environment"
        contract = documented_parameter["options"][TECHDOCS_PARAMETER_OPTION]
        contract["documentedType"] = "string"
        contract["enumValues"] = documented
        contract["isRequired"] = False
        enum_fixture = techdocs_fixture([path_parameter, documented_parameter])
        enum_operation = enum_fixture["file"][0]["messageType"][0]["options"][
            TECHDOCS_OPERATION_OPTION
        ]
        enum_operation["bodyVariants"] = ["UDP", "TCP"]
        enum_operation["renderedBodyVariant"] = "UDP"
        result = compare_contracts(enum_fixture, enum_tool, [], None)
        assert [item["kind"] for item in result["findings"]] == [expected], result[
            "findings"
        ]

    # A map field is an object, not an array. Its MapEntry message is repeated in
    # the descriptor, so dropping MapEntry from the message index made every map
    # read as a plain repeated field.
    map_descriptor = json.loads(json.dumps(descriptor_fixture))
    map_message = map_descriptor["file"][1]["messageType"][0]
    map_message["nestedType"] = [
        {
            "name": "MetadataEntry",
            "options": {"mapEntry": True},
            "field": [
                {
                    "name": "key",
                    "number": 1,
                    "label": "LABEL_OPTIONAL",
                    "type": "TYPE_STRING",
                },
                {
                    "name": "value",
                    "number": 2,
                    "label": "LABEL_OPTIONAL",
                    "type": "TYPE_STRING",
                },
            ],
        }
    ]
    map_message["field"].extend(
        [
            {
                "name": "metadata",
                "jsonName": "metadata",
                "number": 3,
                "label": "LABEL_REPEATED",
                "type": "TYPE_MESSAGE",
                "typeName": ".linode.mcp.v1.WidgetGetInput.MetadataEntry",
                "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
            },
            {
                "name": "tags",
                "jsonName": "tags",
                "number": 4,
                "label": "LABEL_REPEATED",
                "type": "TYPE_STRING",
                "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
            },
        ]
    )
    map_tools, _ = extract_proto_tools(map_descriptor)
    mapped = {
        item["name"]: item for item in map_tools["linode_widget_get"]["parameters"]
    }
    assert mapped["metadata"]["map"] is True, mapped["metadata"]
    assert mapped["metadata"]["type"] == "object", mapped["metadata"]
    assert mapped["metadata"]["required"] is None, mapped["metadata"]
    assert mapped["tags"]["map"] is False, mapped["tags"]
    assert mapped["tags"]["type"] == "array", mapped["tags"]

    # body_comma_list is the only way a singular string reaches the wire as an
    # array, so reading the field type alone would report a mismatch the
    # request body does not have.
    comma_descriptor = json.loads(json.dumps(descriptor_fixture))
    comma_message = comma_descriptor["file"][1]["messageType"][0]
    comma_message["field"].extend(
        [
            {
                "name": "authorized_keys",
                "jsonName": "authorizedKeys",
                "number": 5,
                "label": "LABEL_OPTIONAL",
                "type": "TYPE_STRING",
                "proto3Optional": True,
                "options": {
                    FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY",
                    BODY_COMMA_LIST_OPTION: True,
                },
            },
            {
                "name": "root_pass",
                "jsonName": "rootPass",
                "number": 6,
                "label": "LABEL_OPTIONAL",
                "type": "TYPE_STRING",
                "proto3Optional": True,
                "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
            },
        ]
    )
    comma_tools, _ = extract_proto_tools(comma_descriptor)
    comma = {
        item["name"]: item for item in comma_tools["linode_widget_get"]["parameters"]
    }
    assert comma["authorized_keys"]["type"] == "array", comma["authorized_keys"]
    assert comma["root_pass"]["type"] == "string", comma["root_pass"]

    # A singular message field is presence tracked without the optional keyword,
    # which the body builder reads, so only a declared rule makes it required.
    required_descriptor = json.loads(json.dumps(descriptor_fixture))
    required_message = required_descriptor["file"][1]["messageType"][0]
    required_message["nestedType"] = [
        {
            "name": "Placement",
            "field": [
                {
                    "name": "id",
                    "number": 1,
                    "label": "LABEL_OPTIONAL",
                    "type": "TYPE_INT32",
                }
            ],
        }
    ]
    for number, name in ((3, "placement_group"), (4, "saml")):
        required_message["field"].append(
            {
                "name": name,
                "jsonName": name,
                "number": number,
                "label": "LABEL_OPTIONAL",
                "type": "TYPE_MESSAGE",
                "typeName": ".linode.mcp.v1.WidgetGetInput.Placement",
                "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
            }
        )
    required_message["field"].append(
        {
            "name": "label",
            "jsonName": "label",
            "number": 5,
            "label": "LABEL_OPTIONAL",
            "type": "TYPE_STRING",
            "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
        }
    )
    required_message["options"][VALIDATE_MESSAGE_OPTION] = {
        "cel": [
            {
                "id": "widget_get.saml.required",
                "message": "saml is required",
                "expression": "has(this.saml)",
            },
            {
                "id": "widget_get.saml.entity_id.required",
                "message": "saml.entity_id is required",
                "expression": "this.saml.entity_id != ''",
            },
            {
                "id": "widget_get.fields.required",
                "message": "at least one of placement_group or saml is required",
                "expression": "has(this.placement_group) || has(this.saml)",
            },
        ]
    }
    required_tools, _ = extract_proto_tools(required_descriptor)
    by_name = {
        item["name"]: item for item in required_tools["linode_widget_get"]["parameters"]
    }
    assert by_name["placement_group"]["required"] is False, by_name["placement_group"]
    assert by_name["saml"]["required"] is True, by_name["saml"]
    assert by_name["label"]["required"] is True, by_name["label"]
    # A member rule names the member, not the parent, and an at-least-one rule
    # names no field of the message, so neither makes anything required.
    assert declared_required_fields(required_message) == {"saml"}

    # A `.known` rule is the value set the handler holds a scalar to: the proto
    # side of an enum comparison, as `.required` is of requiredness. Only a
    # membership list on the named field is readable; a range, a member rule,
    # or an unknown list shape leaves the field with no declared set.
    value_set_descriptor = json.loads(json.dumps(required_descriptor))
    value_set_message = value_set_descriptor["file"][1]["messageType"][0]
    for number, name, field_type in (
        (6, "policy", "TYPE_STRING"),
        (7, "size", "TYPE_INT32"),
        (8, "level", "TYPE_INT32"),
        (9, "kind", "TYPE_STRING"),
        (10, "mode", "TYPE_STRING"),
    ):
        value_set_message["field"].append(
            {
                "name": name,
                "jsonName": name,
                "number": number,
                "label": "LABEL_OPTIONAL",
                "type": field_type,
                "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
            }
        )
    value_set_message["field"][-2]["options"][READER_VALUES_OPTION] = [
        "anti_affinity:local"
    ]
    value_set_message["options"][VALIDATE_MESSAGE_OPTION]["cel"].extend(
        [
            {
                "id": "widget_get.policy.known",
                "message": "policy must be one of: linode/migrate, linode/power_off_on",
                "expression": (
                    "!has(this.policy) || this.policy in "
                    "['linode/migrate', 'linode/power_off_on']"
                ),
            },
            {
                "id": "widget_get.size.known",
                "message": "size must be one of: 1, 2, 3",
                "expression": "this.size in [1,2,3]",
            },
            {
                "id": "widget_get.level.known",
                "message": "level must be one of: 0, 1, 2, 3",
                "expression": "this.level >= 0 && this.level <= 3",
            },
            {
                "id": "widget_get.mode.known",
                "message": "mode must be one of: a, b",
                "expression": "this.mode in [this.label, 'b']",
            },
            {
                "id": "widget_get.saml.identity_element.known",
                "message": "saml.identity_element must be one of: NAME_ID",
                "expression": "this.saml.identity_element in ['NAME_ID']",
            },
        ]
    )
    value_set_tools, _ = extract_proto_tools(value_set_descriptor)
    declared = {
        item["name"]: item["enum"]
        for item in value_set_tools["linode_widget_get"]["parameters"]
    }
    assert declared["policy"] == ["linode/migrate", "linode/power_off_on"], declared
    assert declared["size"] == ["1", "2", "3"], declared
    assert declared["kind"] == ["anti_affinity:local"], declared
    assert declared["level"] == [], declared
    assert declared["mode"] == [], declared
    assert declared["saml"] == [], declared
    assert declared["label"] == [], declared

    # A tool-location argument reaches no Linode route, so it is counted and
    # left out of the comparison rather than reported as an unreadable location.
    tool_argument_descriptor = json.loads(json.dumps(descriptor_fixture))
    tool_argument_descriptor["file"][1]["messageType"][0]["field"][1]["options"] = {
        FIELD_LOCATION_OPTION: "FIELD_LOCATION_TOOL"
    }
    tool_argument_tools, _ = extract_proto_tools(tool_argument_descriptor)
    argument = tool_argument_tools["linode_widget_get"]["parameters"][1]
    assert argument["location"] == TOOL_ARGUMENT_LOCATION, argument
    assert argument["system_parameter"] is False, argument
    result = compare_contracts(
        techdocs_fixture([path_parameter]), tool_argument_tools, [], None
    )
    assert result["summary"]["mismatches"] == 0, result["findings"]
    assert result["summary"]["tool_arguments"] == 1, result["summary"]
    assert result["summary"]["system_parameters"] == 0, result["summary"]

    # An unmapped field_location value is still unreadable and still reported,
    # so the new mapping did not turn the location check off.
    unmapped = json.loads(json.dumps(descriptor_fixture))
    unmapped["file"][1]["messageType"][0]["field"][1]["options"] = {
        FIELD_LOCATION_OPTION: "FIELD_LOCATION_ELSEWHERE"
    }
    unmapped_tools, _ = extract_proto_tools(unmapped)
    result = compare_contracts(
        techdocs_fixture([path_parameter]), unmapped_tools, [], None
    )
    assert [item["kind"] for item in result["findings"]] == [
        "proto_field_location_missing"
    ], result["findings"]

    # Known divergences are route-scoped, so the same parameter name on another
    # route keeps its finding. An entry that stops matching is reported rather
    # than silently carried, and an uncollapsed shape fails instead of never
    # matching anything.
    triaged = [
        {
            "kind": "techdocs_parameter_missing_from_proto",
            "severity": "high",
            "method": "GET",
            "route_shape": "/widgets/{}",
            "location": "path",
            "parameter": "accept",
        },
        {
            "kind": "techdocs_parameter_missing_from_proto",
            "severity": "high",
            "method": "GET",
            "route_shape": "/gadgets/{}",
            "location": "path",
            "parameter": "accept",
        },
    ]
    saved_divergences = KNOWN_DIVERGENCES
    try:
        globals()["KNOWN_DIVERGENCES"] = (
            {
                "category": "header_parameter",
                "kind": "techdocs_parameter_missing_from_proto",
                "method": "GET",
                "shape": "/widgets/{}",
                "location": "path",
                "parameter": "accept",
                "reason": "Accept is an HTTP header rendered inside the path table.",
            },
            {
                "category": "header_parameter",
                "kind": "techdocs_parameter_missing_from_proto",
                "method": "GET",
                "shape": "/widgets/{}",
                "location": "body",
                "parameter": "content_type",
                "reason": "Entry that matches nothing in this fixture.",
            },
        )
        demoted, unused = apply_known_divergences(triaged)
        assert demoted[0]["severity"] == "known", demoted[0]
        assert demoted[0]["known_category"] == "header_parameter", demoted[0]
        assert demoted[1]["severity"] == "high", demoted[1]
        assert [entry["parameter"] for entry in unused] == ["content_type"], unused

        globals()["KNOWN_DIVERGENCES"] = (
            {
                "category": "header_parameter",
                "kind": "techdocs_parameter_missing_from_proto",
                "method": "GET",
                "shape": "/widgets/{widgetId}",
                "location": "path",
                "parameter": "accept",
                "reason": "Route is not a collapsed shape.",
            },
        )
        try:
            known_divergence_index()
        except ProofError:
            pass
        else:
            raise AssertionError("an uncollapsed known-divergence shape must fail")
    finally:
        globals()["KNOWN_DIVERGENCES"] = saved_divergences

    # The apiVersion path parameter classifies the route. Its Allowed list is
    # rendered as the first value on the Allowed line with any remaining values
    # on the lines below it, which is why both are read as one enum.
    def surface_page(version_block: list[str]) -> str:
        page = [
            "Get a widget",
            "Copy Page",
            "get https://api.linode.com/{apiVersion}/widgets/{widgetId}",
            "Path Params",
        ]
        page.extend(version_block)
        page.extend(["widgetId integer", "required", "Responses", "200"])
        return "\n".join(page) + "\n"

    both_block = [
        "apiVersion string",
        "enum",
        "required",
        "Enum Call either the v4",
        "URL, or v4beta",
        "for operations still in Beta.",
        "v4 v4beta",
        "Allowed: v4",
        "v4beta",
    ]
    both_endpoint = parse_techdocs_endpoint(
        surface_page(both_block),
        "https://techdocs.akamai.com/linode-api/reference/get-widget",
        "pages/get-widget.md",
    )
    assert both_endpoint is not None
    assert both_endpoint["api_surface"] == SURFACE_BOTH, both_endpoint
    # The single api_version string still reads v4 for a both-surfaces route, so
    # route keys and generated message names do not move.
    assert both_endpoint["api_version"] == "v4", both_endpoint
    assert [item["name"] for item in both_endpoint["parameters"]["path"]] == [
        "widgetId"
    ]

    beta_only_endpoint = parse_techdocs_endpoint(
        surface_page(["apiVersion string", "enum", "required", "Allowed: v4beta"]),
        "https://techdocs.akamai.com/linode-api/reference/get-widget",
        "pages/get-widget.md",
    )
    assert beta_only_endpoint is not None
    assert beta_only_endpoint["api_surface"] == SURFACE_V4BETA_ONLY, beta_only_endpoint
    assert beta_only_endpoint["api_version"] == "v4beta", beta_only_endpoint

    v4_only_endpoint = parse_techdocs_endpoint(
        surface_page(["apiVersion string", "enum", "required", "Allowed: v4"]),
        "https://techdocs.akamai.com/linode-api/reference/get-widget",
        "pages/get-widget.md",
    )
    assert v4_only_endpoint is not None
    assert v4_only_endpoint["api_surface"] == SURFACE_V4_ONLY, v4_only_endpoint

    # A page that documents no apiVersion parameter falls back to the URL.
    unversioned_endpoint = parse_techdocs_endpoint(
        surface_page([]),
        "https://techdocs.akamai.com/linode-api/reference/get-widget",
        "pages/get-widget.md",
    )
    assert unversioned_endpoint is not None
    assert unversioned_endpoint["api_surface"] == SURFACE_V4_ONLY, unversioned_endpoint
    assert classify_api_surface([], "/v4beta/widgets") == SURFACE_V4BETA_ONLY
    # An endpoint index written before classification existed says nothing, and
    # the generated proto leaves the field off rather than guessing v4_only.
    assert endpoint_api_surface({"api_version": "v4beta"}) == ""
    assert 'api_surface: "both"' in render_techdocs_proto([both_endpoint])
    assert "api_surface:" not in render_techdocs_proto(
        [{**both_endpoint, "api_surface": ""}]
    )

    # The monitor metrics operation is served from its own host. A route regex
    # pinned to one host reads the page as documenting no operation at all, so
    # the whole route drops out of the comparison rather than one parameter.
    second_host = parse_techdocs_endpoint(
        "Read metrics\nCopy Page\n"
        "post https://monitor-api.linode.com / {apiVersion} /monitor/services/"
        "{serviceType} /metrics\nPath Params\nserviceType string\nrequired\n"
        "Responses\n200\n",
        "https://techdocs.akamai.com/linode-api/reference/post-read-metric",
        "pages/post-read-metric.md",
    )
    assert second_host is not None, "the monitor host must parse as a route"
    assert second_host["method"] == "POST", second_host
    # The generated proto carries the raw path, so that is the spelling the
    # route key the proto tool has to join to is built from.
    assert path_shape(normalize_path(second_host["raw_path"])) == (
        "/monitor/services/{}/metrics"
    )

    # Parameter heads the site renders with a format token, a nullable array
    # with its item qualifier, and a switcher in place of a type. Each one used
    # to skip the whole block, taking its required and Allowed lines with it.
    typed_page = (
        "Create a client\nCopy Page\n"
        "post https://api.linode.com / {apiVersion} /account/oauth-clients\n"
        "Body Params\n"
        "redirect_uri url\nrequired\nWhere a successful log in lands.\n"
        "interfaces array of objects or null, unique\nlength <= 3\n"
        "details\n[variants: Object Storage, Custom HTTPS; rendered: Object Storage]\n"
        "label string\nrequired\n"
        "Responses\n200\n"
    )
    typed_endpoint = parse_techdocs_endpoint(
        typed_page,
        "https://techdocs.akamai.com/linode-api/reference/post-client",
        "pages/post-client.md",
    )
    assert typed_endpoint is not None
    typed_body = {item["name"]: item for item in typed_endpoint["parameters"]["body"]}
    assert sorted(typed_body) == ["details", "interfaces", "label", "redirect_uri"]
    assert typed_body["redirect_uri"]["required"] is True, typed_body["redirect_uri"]
    assert typed_body["interfaces"]["type"] == "array of objects or null, unique"
    assert typed_body["details"]["type"] == TECHDOCS_VARIANT_TYPE
    assert normalize_semantic_type("url") == "string"
    assert normalize_semantic_type("array of objects or null, unique") == "array"
    # The switcher belongs to the details parameter above it, so it is not the
    # whole section's, and label keeps the required line the block boundary now
    # leaves in its own block.
    assert typed_endpoint["body_variants"] == [], typed_endpoint
    assert typed_body["label"]["required"] is True, typed_body["label"]

    # A switcher with no parameter head above it governs the whole section, and
    # the fetched HTML carries only the variant it names as rendered.
    section_page = (
        "Create a config\nCopy Page\n"
        "post https://api.linode.com / {apiVersion} /widgets\n"
        "Body Params\n"
        "[variants: UDP, TCP, HTTP, HTTPS; rendered: UDP]\n"
        "protocol string\nenum\nAllowed: udp\n"
        "Responses\n200\n"
    )
    section_endpoint = parse_techdocs_endpoint(
        section_page,
        "https://techdocs.akamai.com/linode-api/reference/post-widget",
        "pages/post-widget.md",
    )
    assert section_endpoint is not None
    assert section_endpoint["body_variants"] == ["UDP", "TCP", "HTTP", "HTTPS"]
    assert section_endpoint["rendered_body_variant"] == "UDP"
    rendered_proto = render_techdocs_proto([section_endpoint])
    assert 'body_variants: "TCP"' in rendered_proto
    assert 'rendered_body_variant: "UDP"' in rendered_proto
    assert "body_variants:" not in render_techdocs_proto([both_endpoint])

    # The switcher marker comes from the published select element, so the
    # labels survive the text normalization the page evidence is written in.
    switcher_html = (
        '<div><span class="Param-nameU7">details</span></div>'
        '<select class="Select" data-testid="OneOfMultiSchema-trigger">'
        '<option value="0" selected="">Akamai Object Storage</option>'
        '<option value="1">Custom HTTPS</option></select>'
    )
    assert (
        "[variants: Akamai Object Storage, Custom HTTPS; rendered: Akamai Object "
        "Storage]" in html_to_markdownish(switcher_html)
    )
    assert parse_techdocs_variants("[variants: A, B; rendered: B]") == (["A", "B"], "B")
    assert parse_techdocs_variants("UDP TCP HTTP HTTPS") is None

    # A route serving both surfaces answers either declaration, so a beta tool
    # there is an observation and not a defect.
    both_route = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_BOTH),
        beta_tools,
        [],
        None,
    )
    assert [item["kind"] for item in both_route["findings"]] == [
        "route_surface_downgrade_available"
    ], both_route["findings"]
    assert both_route["findings"][0]["severity"] == "info"
    assert both_route["findings"][0]["techdocs"] == SURFACE_BOTH
    assert both_route["findings"][0]["proto"] == "v4beta"
    assert both_route["summary"]["proto_tools_declaring_v4beta"] == 1
    assert both_route["summary"]["techdocs_routes_with_surface_evidence"] == 1
    assert both_route["summary"]["techdocs_routes_serving_both_surfaces"] == 1

    agreed = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_V4BETA_ONLY),
        beta_tools,
        [],
        None,
    )
    assert agreed["summary"]["mismatches"] == 0, agreed["findings"]

    # A tool pinned to beta on a route that never served beta.
    beta_on_v4 = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_V4_ONLY),
        beta_tools,
        [],
        None,
    )
    assert [item["kind"] for item in beta_on_v4["findings"]] == [
        "route_surface_mismatch"
    ], beta_on_v4["findings"]
    assert beta_on_v4["findings"][0]["severity"] == "high"

    # A tool on the default surface calling a beta-only route, declared either
    # explicitly or by leaving the option off once the contract is in play.
    explicit_v4 = json.loads(json.dumps(descriptor_fixture))
    explicit_v4["file"][1]["messageType"][0]["options"][TOOL_API_SURFACE_OPTION] = (
        "API_SURFACE_V4"
    )
    explicit_v4_tools, _ = extract_proto_tools(explicit_v4)
    v4_on_beta = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_V4BETA_ONLY),
        explicit_v4_tools,
        [],
        None,
    )
    assert [item["kind"] for item in v4_on_beta["findings"]] == [
        "route_surface_mismatch"
    ], v4_on_beta["findings"]
    assert v4_on_beta["findings"][0]["severity"] == "high"
    assert v4_on_beta["findings"][0]["proto"] == "v4"

    mixed = json.loads(json.dumps(descriptor_fixture))
    gadget_message = json.loads(json.dumps(mixed["file"][1]["messageType"][0]))
    gadget_message["name"] = "GadgetGetInput"
    gadget_message["options"][TOOL_ROUTE_OPTION] = {
        "tool": "linode_gadget_get",
        "method": "GET",
        "path": "/gadgets/{gadget_id}",
    }
    gadget_message["options"][TOOL_API_SURFACE_OPTION] = "API_SURFACE_V4BETA"
    mixed["file"][1]["messageType"].append(gadget_message)
    mixed_tools, _ = extract_proto_tools(mixed)
    assert mixed_tools["linode_widget_get"]["declared_api_surface"] is None
    undeclared_on_beta = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_V4BETA_ONLY),
        mixed_tools,
        [],
        None,
    )
    beta_gaps = [
        item
        for item in undeclared_on_beta["findings"]
        if item["kind"] == "route_surface_mismatch"
    ]
    assert len(beta_gaps) == 1, undeclared_on_beta["findings"]
    assert beta_gaps[0]["tool"] == "linode_widget_get"
    assert beta_gaps[0]["declared_api_surface"] is None

    # Neither side alone can produce a surface finding: a descriptor where no
    # tool declares a surface, and a route the pages never classified.
    undeclared = compare_contracts(
        techdocs_fixture([path_parameter], api_surface=SURFACE_V4BETA_ONLY),
        tools,
        [],
        None,
    )
    assert undeclared["summary"]["surface_comparison_enabled"] is False
    assert undeclared["summary"]["mismatches"] == 0, undeclared["findings"]
    unclassified = compare_contracts(
        techdocs_fixture([path_parameter]), beta_tools, [], None
    )
    assert unclassified["summary"]["techdocs_routes_with_surface_evidence"] == 0
    assert unclassified["summary"]["mismatches"] == 0, unclassified["findings"]

    # Evidence placement is a refusal rather than a default. A root inside the
    # repository would dirty the tree the comparison measures.
    for inside in (REPO_ROOT, REPO_ROOT / "docs" / "contracts"):
        try:
            evidence_root(inside, {})
        except ProofError:
            pass
        else:
            raise AssertionError(f"evidence root inside the repo must fail: {inside}")

    data_home = evidence_root(environ={"XDG_DATA_HOME": "/nonexistent-xdg-root"})
    assert data_home.parts[-2:] == ("linodemcp", "techdocs-proof"), data_home
    from_env = evidence_root(
        environ={
            EVIDENCE_ROOT_ENV: "/nonexistent-env-root",
            "XDG_DATA_HOME": "/nonexistent-xdg-root",
        }
    )
    assert from_env == Path("/nonexistent-env-root"), from_env
    from_flag = evidence_root(
        Path("/nonexistent-flag-root"),
        {EVIDENCE_ROOT_ENV: "/nonexistent-env-root"},
    )
    assert from_flag == Path("/nonexistent-flag-root"), from_flag

    # The shipped ledger is data now, so the gate reads it rather than trusting
    # that a review would have caught a bad entry in source.
    assert len(known_divergence_index()) == len(KNOWN_DIVERGENCES)
    assert all(entry["reason"].strip() for entry in KNOWN_DIVERGENCES)

    saved_kind_ledger = KNOWN_DIVERGENCES
    try:
        mistyped = dict(saved_kind_ledger[0])
        mistyped["kind"] = "parameter_requiredness_mismatchh"
        globals()["KNOWN_DIVERGENCES"] = (mistyped,)
        try:
            known_divergence_index()
        except ProofError:
            pass
        else:
            raise AssertionError("a ledger kind the comparison cannot raise must fail")
    finally:
        globals()["KNOWN_DIVERGENCES"] = saved_kind_ledger

    saved_ledger = KNOWN_DIVERGENCES
    try:
        globals()["KNOWN_DIVERGENCES"] = (dict(saved_ledger[0]), dict(saved_ledger[0]))
        try:
            known_divergence_index()
        except ProofError:
            pass
        else:
            raise AssertionError("a duplicate known-divergence entry must fail")
    finally:
        globals()["KNOWN_DIVERGENCES"] = saved_ledger

    with tempfile.TemporaryDirectory(prefix="techdocs-proof-ledger-") as directory:
        incomplete = Path(directory) / "known-divergences.json"
        incomplete.write_text(
            json.dumps([{"kind": "route_missing_from_proto"}]), encoding="utf-8"
        )
        try:
            load_known_divergences(incomplete)
        except ProofError:
            pass
        else:
            raise AssertionError("an incomplete ledger entry must fail")

        # A class rule is one approval item over a written-out key list, and it
        # expands into the same per-key entries the stale report already walks,
        # so a key that stops matching still surfaces.
        classed = Path(directory) / "class-rule.json"
        classed.write_text(
            json.dumps(
                [
                    {
                        "category": "list_pagination_undocumented",
                        "kind": "proto_parameter_not_in_techdocs",
                        "reason": "The page renders no Query Params section.",
                        "entries": [
                            {
                                "method": "GET",
                                "shape": "/widgets",
                                "location": "query",
                                "parameter": "page",
                            },
                            {
                                "method": "GET",
                                "shape": "/widgets",
                                "location": "query",
                                "parameter": "page_size",
                            },
                        ],
                    }
                ]
            ),
            encoding="utf-8",
        )
        expanded = load_known_divergences(classed)
        assert len(expanded) == 2, expanded
        assert {entry["parameter"] for entry in expanded} == {"page", "page_size"}
        assert all(entry["reason"] for entry in expanded), expanded

        empty_class = Path(directory) / "empty-class.json"
        empty_class.write_text(
            json.dumps(
                [
                    {
                        "category": "list_pagination_undocumented",
                        "kind": "proto_parameter_not_in_techdocs",
                        "reason": "A rule that absorbs nothing named.",
                        "entries": [],
                    }
                ]
            ),
            encoding="utf-8",
        )
        try:
            load_known_divergences(empty_class)
        except ProofError:
            pass
        else:
            raise AssertionError("a class rule with no keys must fail")

    saved_class_ledger = KNOWN_DIVERGENCES
    try:
        globals()["KNOWN_DIVERGENCES"] = tuple(expanded)
        classed_findings = [
            {
                "kind": "proto_parameter_not_in_techdocs",
                "severity": "medium",
                "method": "GET",
                "route_shape": "/widgets",
                "location": "query",
                "parameter": "page",
            }
        ]
        demoted, unused = apply_known_divergences(classed_findings)
        assert demoted[0]["severity"] == "known", demoted[0]
        assert demoted[0]["known_category"] == "list_pagination_undocumented"
        # page_size stopped appearing, and the class rule reports it exactly as
        # a per-key entry would. A pattern rule could not report this at all.
        assert [entry["parameter"] for entry in unused] == ["page_size"], unused
    finally:
        globals()["KNOWN_DIVERGENCES"] = saved_class_ledger

    # The exit code is the only thing the scheduled job reads. These cases run
    # the real descriptor entry point end to end: it reads two JSON files, so
    # it needs neither buf nor the network.
    def exit_code_for(
        repo_descriptor: dict[str, Any], docs_descriptor: dict[str, Any]
    ) -> int:
        with tempfile.TemporaryDirectory(prefix="techdocs-proof-exit-") as directory:
            root = Path(directory)
            write_json(root / "linodemcp-descriptor.json", repo_descriptor)
            write_json(root / "techdocs-descriptor.json", docs_descriptor)
            saved_argv = sys.argv
            sys.argv = [
                "techdocs_proof",
                "--techdocs-descriptor",
                str(root / "techdocs-descriptor.json"),
                "--linodemcp-descriptor",
                str(root / "linodemcp-descriptor.json"),
                "--output",
                str(root / "comparison.json"),
                "--fail-on-findings",
            ]
            # main() reports each run on stderr, and the gate prints one line, so
            # these six probe runs report into the temporary directory instead.
            try:
                with (
                    (root / "stderr.log").open("w", encoding="utf-8") as report,
                    contextlib.redirect_stderr(report),
                ):
                    return main()
            finally:
                sys.argv = saved_argv

    clean_docs = techdocs_fixture([path_parameter])
    assert exit_code_for(descriptor_fixture, clean_docs) == 0

    # info: a body field differing from the documented name by word segmentation
    # is a spelling variant, so it must not fail the run.
    segmented_descriptor = json.loads(json.dumps(descriptor_fixture))
    segmented_descriptor["file"][1]["messageType"][0]["field"].append(
        {
            "name": "nodebalancer_id",
            "jsonName": "nodebalancerId",
            "number": 3,
            "label": "LABEL_OPTIONAL",
            "type": "TYPE_INT64",
            "options": {FIELD_LOCATION_OPTION: "FIELD_LOCATION_BODY"},
        }
    )
    assert (
        exit_code_for(
            segmented_descriptor, techdocs_fixture([path_parameter, body_parameter])
        )
        == 0
    )

    # limitation: the page rendered one body variant, so a member the rendered
    # variant never carried has nothing to match.
    variant_descriptor = json.loads(json.dumps(descriptor_fixture))
    variant_descriptor["file"][1]["messageType"][0]["field"][1]["options"][
        FIELD_LOCATION_OPTION
    ] = "FIELD_LOCATION_BODY"
    variant_docs = techdocs_fixture([path_parameter])
    variant_docs["file"][0]["messageType"][0]["options"][
        TECHDOCS_OPERATION_OPTION
    ].update({"bodyVariants": ["UDP", "TCP"], "renderedBodyVariant": "UDP"})
    assert exit_code_for(variant_descriptor, variant_docs) == 0

    # medium: a proto query field the documentation never mentions.
    query_descriptor = json.loads(json.dumps(descriptor_fixture))
    query_descriptor["file"][1]["messageType"][0]["field"][1]["options"][
        FIELD_LOCATION_OPTION
    ] = "FIELD_LOCATION_QUERY"
    assert exit_code_for(query_descriptor, clean_docs) == 3

    # known: the same row, demoted by a ledger entry that matches it.
    saved_exit_ledger = KNOWN_DIVERGENCES
    try:
        globals()["KNOWN_DIVERGENCES"] = (
            {
                "category": "probe",
                "kind": "proto_parameter_not_in_techdocs",
                "method": "GET",
                "shape": "/widgets/{}",
                "location": "query",
                "parameter": "environment",
                "reason": "The triage that demotes the medium row above.",
            },
        )
        assert exit_code_for(query_descriptor, clean_docs) == 0
    finally:
        globals()["KNOWN_DIVERGENCES"] = saved_exit_ledger

    # high: a documented query parameter with no proto field at all.
    assert (
        exit_code_for(descriptor_fixture, techdocs_fixture([path_parameter, extra]))
        == 3
    )

    # A scrape-mode run needs buf and the network, so the offline gate cannot
    # reach its return. Proving both entry points read the one helper is what
    # keeps the scrape path from drifting back to counting every tier.
    for entry_point in (run_scrape_mode, main):
        assert "findings_exit_code(" in inspect.getsource(entry_point), (
            entry_point.__name__
        )

    # The route snapshot is the artifact scripts/verify_techdocs_routes.py gates
    # proto/ against, so the shape it is written in is a contract of its own.
    snapshot_contract = {
        "routes": [
            {
                "method": "get",
                "path": "/v4/widgets/{widgetId}",
                "api_surface": "both",
                "deprecated": False,
            },
            {
                "method": "DELETE",
                "path": "/v4beta/widgets/{widgetId}",
                "api_surface": "v4beta_only",
                "deprecated": True,
            },
        ]
    }
    assert route_snapshot_lines(snapshot_contract) == [
        "DELETE /widgets/{} surface=v4beta_only status=deprecated",
        "GET /widgets/{} surface=both status=active",
    ], route_snapshot_lines(snapshot_contract)

    snapshot_text = route_snapshot_text(snapshot_contract, "2026-01-02")
    assert "harvested 2026-01-02, 2 routes" in snapshot_text, snapshot_text
    assert snapshot_text.endswith("status=active\n"), snapshot_text
    assert "verify_techdocs_routes.py" in snapshot_text, snapshot_text

    # A contract that parsed to no route would write a snapshot the offline gate
    # reads as an empty upstream, so it fails here instead.
    try:
        route_snapshot_text({"routes": []}, "2026-01-02")
    except ProofError:
        pass
    else:
        raise AssertionError("a routeless contract must not write a snapshot")

    with tempfile.TemporaryDirectory(prefix="techdocs-proof-snapshot-") as directory:
        written = Path(directory) / "nested" / ROUTE_SNAPSHOT_NAME
        assert write_route_snapshot(snapshot_contract, written, "2026-01-02") == 2
        assert written.read_text(encoding="utf-8") == snapshot_text

    print("SELF_TEST_OK")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--techdocs-descriptor",
        type=Path,
        help=(
            "Compare an already generated TechDocs descriptor instead of "
            "scraping a new snapshot"
        ),
    )
    parser.add_argument(
        "--endpoint-index",
        type=Path,
        help=(
            "Regenerate the TechDocs proto from a saved endpoint index instead of "
            "refetching pages"
        ),
    )
    parser.add_argument(
        "--docs-repo",
        type=Path,
        help="Scraped page directory for the --techdocs-descriptor mode; the "
        "scrape and replay modes read the run's own pages",
    )
    parser.add_argument(
        "--github-repository",
        default=DEFAULT_GITHUB_REPOSITORY,
        help="GitHub owner/repository containing the authoritative proto source",
    )
    parser.add_argument(
        "--github-ref",
        default=DEFAULT_GITHUB_REF,
        help="GitHub branch, tag, or commit to resolve to an immutable SHA",
    )
    parser.add_argument(
        "--gh", default="gh", help="GitHub CLI executable or command name"
    )
    parser.add_argument(
        "--linodemcp-repo",
        type=Path,
        default=REPO_ROOT,
        help="Working tree holding the proto source to compare; defaults to "
        "the repository this command lives in",
    )
    parser.add_argument(
        "--github-source",
        action="store_true",
        help="Resolve an immutable GitHub archive with gh instead of reading "
        "the local working tree; needs the network",
    )
    parser.add_argument(
        "--linodemcp-descriptor",
        type=Path,
        help=(
            "Compare against a descriptor built elsewhere, for a working tree GitHub "
            "cannot serve"
        ),
    )
    parser.add_argument("--buf", default="buf", help="Buf executable or command name")
    parser.add_argument(
        "--evidence-root",
        type=Path,
        help=f"Directory for dated runs and latest.json; defaults to "
        f"${EVIDENCE_ROOT_ENV} or the user data directory, and must sit "
        f"outside the repository",
    )
    parser.add_argument(
        "--retention-days",
        type=int,
        default=5,
        help="Remove dated run directories older than this many days",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=16,
        help="Concurrent rendered-TechDocs fetch workers",
    )
    parser.add_argument(
        "--max-pages",
        type=int,
        help=(
            "Limit TechDocs pages for parser development only; omit for a complete "
            "proof"
        ),
    )

    parser.add_argument(
        "--techdocs-contract",
        type=Path,
        help="Run's techdocs-contracts.json to read for --emit-route-snapshot",
    )
    parser.add_argument(
        "--emit-route-snapshot",
        type=Path,
        help=(
            "Write the reviewed route snapshot from --techdocs-contract and "
            "exit; offline, and it writes only where it is pointed"
        ),
    )
    parser.add_argument("--output", type=Path, help="Write JSON here; default stdout")
    parser.add_argument(
        "--fail-on-findings",
        action="store_true",
        help=(
            "Return exit 3 when the completed comparison carries a finding at "
            "medium or high; known, limitation, and info never reach the exit code"
        ),
    )
    parser.add_argument("--self-test", action="store_true")
    return parser.parse_args()


def run_route_snapshot_mode(args: argparse.Namespace) -> int:
    """Render one run's route snapshot. Offline: it reads a contract and writes."""
    if args.techdocs_contract is None:
        raise ProofError("--emit-route-snapshot needs --techdocs-contract")
    contract = read_json(args.techdocs_contract)
    if not is_json_object(contract):
        raise ProofError(f"{args.techdocs_contract} is not a TechDocs contract")
    harvested = datetime.now(UTC).date().isoformat()
    count = write_route_snapshot(contract, args.emit_route_snapshot, harvested)
    print(
        f"ROUTE_SNAPSHOT_OK routes={count} path={args.emit_route_snapshot}",
        file=sys.stderr,
    )
    return 0


def main() -> int:
    args = parse_args()
    if args.self_test:
        run_self_test()
        return 0
    if args.emit_route_snapshot is not None:
        try:
            return run_route_snapshot_mode(args)
        except (ProofError, OSError) as exc:
            print(f"PROOF_ERROR {exc}", file=sys.stderr)
            return 1
    started_at = utc_now()
    try:
        if args.techdocs_descriptor is None:
            return run_scrape_mode(args)
        techdocs_descriptor = read_json(args.techdocs_descriptor)
        if args.linodemcp_descriptor is not None:
            source_metadata = prebuilt_descriptor_metadata(args.linodemcp_descriptor)
            descriptor = read_json(args.linodemcp_descriptor)
            tools, extraction_findings = extract_proto_tools(descriptor)
        else:
            with resolve_linodemcp_source(args) as (source, source_metadata):
                descriptor = run_buf_descriptor(source, args.buf)
                tools, extraction_findings = extract_proto_tools(descriptor)
        result = compare_contracts(
            techdocs_descriptor,
            tools,
            extraction_findings,
            args.docs_repo,
        )
        result["started_at"] = started_at
        result["ran_at"] = utc_now()
        result["repo_sha"] = source_metadata.get("commit_sha")
        result["inputs"] = {
            "techdocs_descriptor": str(args.techdocs_descriptor),
            "docs_repo": str(args.docs_repo) if args.docs_repo else None,
            "linodemcp_source": source_metadata,
            "buf": args.buf,
        }
        write_json(args.output, result)
        summary = result["summary"]
        print(
            "PROOF_OK "
            f"techdocs={summary['techdocs_operations']} "
            f"proto_tools={summary['proto_tools_compared']} "
            f"deprecated={summary['deprecated_routes_checked']} "
            f"replacements={summary['replacement_routes_checked']} "
            f"findings={summary['mismatches']}",
            file=sys.stderr,
        )
        return findings_exit_code(args.fail_on_findings, summary)
    except (ProofError, OSError, subprocess.SubprocessError) as exc:
        error = {
            "source_authority": TECHDOCS_AUTHORITY,
            "status": "error",
            "started_at": started_at,
            "ran_at": utc_now(),
            "error": str(exc),
        }
        write_json(args.output, error)
        print(f"PROOF_ERROR {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
