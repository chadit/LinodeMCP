"""Shared helpers for the non-interactive CLI subcommands.

Three concerns live here so ``call``, ``tools``, and ``audit`` share one
implementation:

- exit-code constants (success / tool-error / usage-error),
- argument building: ``--json`` parsing, ``--arg key=value`` coercion by the
  tool's input-schema property type, and folding the safety flags into the
  arguments dict under the same keys the MCP fields use,
- result rendering: pulling the text payload out of the dispatch result and
  either printing it verbatim (``--output json``) or as a simple text table.

None of this reimplements tool logic. Coercion only decides whether a CLI
string becomes a number, bool, or string before it reaches the schema the
handler already validates.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

if TYPE_CHECKING:
    from mcp.types import Tool

# Exit codes. 0 success, 1 the tool ran but returned an error result, 2 a
# usage problem caught before dispatch (unknown tool, bad flags, json parse,
# pre-dispatch schema validation). Matches the Go CLI and sysexits' EX_USAGE.
EXIT_SUCCESS = 0
EXIT_TOOL_ERROR = 1
EXIT_USAGE_ERROR = 2

# The tool-result error convention this codebase uses: handlers signal a
# tool-level failure by returning a TextContent whose text starts with one of
# these prefixes (see tools/helpers.py error_response / execute_tool). The CLI
# reads that to decide exit 1 vs exit 0 without a separate isError channel.
_ERROR_PREFIXES = ("Error:", "Failed to ")


class ArgError(ValueError):
    """A CLI argument could not be parsed or coerced.

    Raised by ``build_arguments`` for a malformed ``--json``, a ``--arg``
    without ``=``, or a value that does not fit its schema type. The command
    layer catches it and exits with ``EXIT_USAGE_ERROR``.
    """


def schema_properties(tool: Tool) -> dict[str, dict[str, Any]]:
    """Return the ``properties`` map from a tool's input schema.

    ``Tool.inputSchema`` is an untyped ``dict`` at runtime, so the access is
    cast for the strict type checkers. Returns an empty dict when the tool
    declares no properties.
    """
    schema: dict[str, Any] = tool.inputSchema
    return cast("dict[str, dict[str, Any]]", schema.get("properties", {}))


def required_args(tool: Tool) -> list[str]:
    """Return the tool's required argument names from its input schema."""
    schema: dict[str, Any] = tool.inputSchema
    req = cast("list[Any]", schema.get("required", []))
    return [str(name) for name in req]


def _coerce_scalar(key: str, raw: str, json_type: str) -> Any:
    """Coerce one ``--arg`` string to the type its schema property declares.

    ``number`` and ``integer`` parse as float/int; ``boolean`` accepts the
    usual truthy/falsy words; everything else (including an unknown or absent
    type) stays a string. A value that does not fit raises ``ArgError`` so the
    caller can exit 2 rather than send a wrong-typed value into dispatch.
    """
    if json_type == "integer":
        try:
            return int(raw)
        except ValueError as exc:
            msg = f"argument {key!r} expects an integer, got {raw!r}"
            raise ArgError(msg) from exc
    if json_type == "number":
        try:
            return float(raw)
        except ValueError as exc:
            msg = f"argument {key!r} expects a number, got {raw!r}"
            raise ArgError(msg) from exc
    if json_type == "boolean":
        lowered = raw.strip().lower()
        if lowered in ("true", "1", "yes", "y", "on"):
            return True
        if lowered in ("false", "0", "no", "n", "off"):
            return False
        msg = f"argument {key!r} expects a boolean, got {raw!r}"
        raise ArgError(msg)
    return raw


def coerce_arg_pairs(
    pairs: list[str],
    properties: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    """Turn ``key=value`` strings into a typed arguments dict.

    Each value is coerced by the schema property's ``type``. Keys absent from
    the schema stay strings (the handler's own validation rejects unknown keys
    if it cares). A pair without ``=`` is a usage error.
    """
    out: dict[str, Any] = {}
    for pair in pairs:
        if "=" not in pair:
            msg = f"--arg expects key=value, got {pair!r}"
            raise ArgError(msg)
        key, raw = pair.split("=", 1)
        prop = properties.get(key, {})
        json_type = str(prop.get("type", "string"))
        out[key] = _coerce_scalar(key, raw, json_type)
    return out


def parse_json_args(blob: str) -> dict[str, Any]:
    """Parse a ``--json`` argument blob into a dict.

    The top-level value must be a JSON object; a list or scalar is a usage
    error since tool arguments are always keyed.
    """
    try:
        parsed = json.loads(blob)
    except json.JSONDecodeError as exc:
        msg = f"--json is not valid JSON: {exc}"
        raise ArgError(msg) from exc
    if not isinstance(parsed, dict):
        msg = "--json must be a JSON object"
        raise ArgError(msg)
    return cast("dict[str, Any]", parsed)


def build_arguments(
    tool: Tool,
    *,
    json_args: str | None,
    arg_pairs: list[str],
    dry_run: bool,
    confirm: bool,
    mode: str | None,
    plan_id: str | None,
    confirmed_dry_run: bool,
    yolo: bool,
    environment: str | None,
) -> dict[str, Any]:
    """Compose the full dispatch arguments dict for a ``call`` invocation.

    Starts from either the ``--json`` object or the coerced ``--arg`` pairs
    (the two are mutually exclusive; the caller enforces that), then folds the
    safety flags on top. Raises ``ArgError`` on any malformed input so the
    command layer can exit with ``EXIT_USAGE_ERROR``.
    """
    if json_args is not None:
        base = parse_json_args(json_args)
    else:
        base = coerce_arg_pairs(arg_pairs, schema_properties(tool))
    return fold_safety_flags(
        base,
        dry_run=dry_run,
        confirm=confirm,
        mode=mode,
        plan_id=plan_id,
        confirmed_dry_run=confirmed_dry_run,
        yolo=yolo,
        environment=environment,
    )


def fold_safety_flags(
    arguments: dict[str, Any],
    *,
    dry_run: bool,
    confirm: bool,
    mode: str | None,
    plan_id: str | None,
    confirmed_dry_run: bool,
    yolo: bool,
    environment: str | None,
) -> dict[str, Any]:
    """Fold the safety flags into the arguments dict under the MCP keys.

    Only flags the user actually set are written, so a bare ``call`` does not
    inject ``dry_run: false`` and trip a handler that distinguishes absent from
    false. The keys match exactly what an MCP ``tools/call`` would carry, so
    the same dispatch gate (dry-run, two-stage, confirm, yolo) applies.
    """
    merged = dict(arguments)
    if dry_run:
        merged["dry_run"] = True
    if confirm:
        merged["confirm"] = True
    if mode is not None:
        merged["mode"] = mode
    if plan_id is not None:
        merged["plan_id"] = plan_id
    if confirmed_dry_run:
        merged["confirmed_dry_run"] = True
    if yolo:
        merged["yolo"] = True
    if environment is not None:
        merged["environment"] = environment
    return merged


def result_text(result: list[Any]) -> str:
    """Join the text payloads of a dispatch result into one string.

    Dispatch returns a list of content blocks (TextContent today). Each block's
    ``.text`` is concatenated; blocks without ``.text`` are skipped. This is the
    raw payload printed under ``--output json``.
    """
    parts: list[str] = []
    for block in result:
        text = getattr(block, "text", None)
        if isinstance(text, str):
            parts.append(text)
    return "\n".join(parts)


def is_error_result(text: str) -> bool:
    """Report whether a result payload is a tool-level error.

    Uses the codebase's error-text convention (``Error:`` / ``Failed to``). A
    payload that parses as JSON is always a success even if it happens to start
    with one of those words inside a string value, so JSON is checked first.
    """
    stripped = text.lstrip()
    if not stripped:
        return False
    try:
        json.loads(stripped)
    except json.JSONDecodeError:
        return stripped.startswith(_ERROR_PREFIXES)
    return False


def render_output(text: str, output: str) -> str:
    """Render the result payload for the chosen ``--output`` mode.

    ``json`` prints the payload verbatim. ``table`` pretty-prints a JSON list
    or object as a simple text table; non-JSON or scalar payloads fall back to
    the verbatim text so ``table`` never hides output.
    """
    if output != "table":
        return text
    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        return text
    return _to_table(parsed)


def _to_table(value: Any) -> str:
    """Render a parsed JSON value as a simple aligned text table.

    A list of objects renders as columns keyed by the union of object keys; a
    single object renders as key/value rows; anything else falls back to
    indented JSON so the caller still sees the data.
    """
    if isinstance(value, list):
        return _table_from_list(value)
    if isinstance(value, dict):
        return _table_from_object(value)
    return json.dumps(value, indent=2)


def _cell(value: Any) -> str:
    """Format one table cell. Scalars print bare; containers compact-JSON."""
    if isinstance(value, (dict, list)):
        return json.dumps(value, separators=(",", ":"))
    if value is None:
        return ""
    return str(value)


def _table_from_object(value: Any) -> str:
    """Render a single object as aligned ``key  value`` rows.

    Takes ``Any`` (the value comes from ``json.loads``) and casts to a concrete
    dict here so neither strict type checker sees a partially-unknown local.
    """
    obj = cast("dict[str, Any]", value)
    if not obj:
        return "(empty)"
    width = max(len(str(k)) for k in obj)
    lines = [f"{k!s:<{width}}  {_cell(v)}" for k, v in obj.items()]
    return "\n".join(lines)


def _table_from_list(value: Any) -> str:
    """Render a list as a column table (objects) or one value per line.

    A list of objects becomes columns over the union of keys, ordered by first
    appearance. A list of scalars prints one per line. An empty list renders a
    placeholder so the output is never silently blank. Takes ``Any`` and casts
    to a concrete list here for the same reason as ``_table_from_object``.
    """
    rows = cast("list[Any]", value)
    if not rows:
        return "(no rows)"
    if not all(isinstance(r, dict) for r in rows):
        return "\n".join(_cell(r) for r in rows)

    objects = cast("list[dict[str, Any]]", rows)
    columns: list[str] = []
    for obj in objects:
        for key in obj:
            if key not in columns:
                columns.append(key)

    widths = {
        col: max(len(col), *(len(_cell(obj.get(col))) for obj in objects))
        for col in columns
    }
    header = "  ".join(f"{col:<{widths[col]}}" for col in columns)
    sep = "  ".join("-" * widths[col] for col in columns)
    body = [
        "  ".join(f"{_cell(obj.get(col)):<{widths[col]}}" for col in columns)
        for obj in objects
    ]
    return "\n".join([header, sep, *body])
