#!/usr/bin/env python3
"""Cross-language tool-parity verifier.

Dumps every language's tool registry and compares, per tool, the observable
contract that a client or model sees: capability tier, the set of input
parameters, each parameter's JSON-Schema type, the required set, and the
required OAuth scopes. Descriptions are intentionally ignored, since wording
is allowed to differ. Scopes are compared because the tool-to-scope mapping
is hand-written per language and nothing else diffs it: a one-sided scope
change would make the same profile allow a tool in one language and deny it
in the other, silently.

The languages come from docs/contracts/languages.txt: one dumper command per
implementation, each printing the same JSON record shape (see
go/cmd/parity-dump and python -m linodemcp.parity_dump). Registering a
language there is what turns this gate on for it, so a freshly added
language immediately fails with one "missing in <language>" line per tool it
has not implemented yet. Those absences are accepted into the baseline with
a tracking annotation and driven to zero; `make parity-todo` renders the
per-language remaining-work view.

Run it directly, via ``make tool-parity`` (root Makefile), or as a pre-commit
hook. Exits non-zero and prints every divergence when the implementations
disagree, so a tool cannot drift in shape between languages.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "tool-parity-baseline.txt"

_ABSENCE_MARK = ": missing in "

_BASELINE_HEADER = (
    "# Accepted (known) cross-language tool-parity divergences. Ratchet:\n"
    "# fix one and remove its line; never add a line by hand (regenerate\n"
    "# instead, then attach the required annotation). Regenerate with:\n"
    "#   python scripts/verify_tool_parity.py --update-baseline\n"
    '# Every "missing in <language>" entry MUST carry an annotation naming\n'
    "# when it was accepted and the tracking issue that will close it:\n"
    "#   <entry>  # accepted YYYY-MM-DD <tracking-issue URL>\n"
)


def _load_languages() -> list[tuple[str, Path, list[str]]]:
    """Parse docs/contracts/languages.txt into (name, cwd, argv) triples in file order.

    File order matters: the first language that implements a tool is the
    reference its contract is diffed against.
    """
    languages: list[tuple[str, Path, list[str]]] = []

    for raw in _LANGUAGES.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue

        parts = stripped.split("\t")
        expected_fields = 3
        if len(parts) != expected_fields:
            msg = (
                f"docs/contracts/languages.txt line {raw!r}"
                " is not <name>\\t<dir>\\t<command>"
            )
            raise SystemExit(msg)

        name, workdir, command = parts
        languages.append((name, _REPO_ROOT / workdir, command.split()))

    if not languages:
        msg = "docs/contracts/languages.txt registers no languages"
        raise SystemExit(msg)

    return languages


def _dump_surface(name: str, cwd: Path, argv: list[str]) -> dict[str, dict[str, Any]]:
    """Run one language's dumper and return {tool_name: normalized_record}."""
    result = subprocess.run(
        argv,
        cwd=cwd,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"{name} dumper failed (exit {result.returncode})"
        raise SystemExit(msg)

    records = json.loads(result.stdout)
    return {rec["name"]: _normalize(rec) for rec in records}


# Go's mcp-go only offers WithNumber (emits "number"); Python uses "integer".
# They are equivalent for the integer ids/pages that dominate, and the Linode
# API rejects non-integers anyway, so collapse them. A real float-vs-int bug
# would instead surface as number-vs-string or a missing param.
_TYPE_ALIASES = {"integer": "number"}


def _canon_type(typ: str) -> str:
    """Canonicalize a JSON-Schema type so integer and number compare equal."""
    return _TYPE_ALIASES.get(typ, typ)


def _normalize(rec: dict[str, Any]) -> dict[str, Any]:
    """Sort the list fields and canonicalize param types for comparison."""
    params = {
        name: _canon_type(str(typ)) for name, typ in (rec.get("params") or {}).items()
    }
    return {
        "capability": rec["capability"],
        "params": params,
        "required": sorted(rec.get("required") or []),
        "scopes": sorted(rec.get("scopes") or []),
    }


def _compare(
    surfaces: dict[str, dict[str, dict[str, Any]]], order: list[str]
) -> list[str]:
    """Return human-readable divergence lines across every language.

    Absences are reported per language against the union surface, so a tool
    only one implementation registers still names every language that lacks
    it. Contract fields are diffed against the first registered language
    that has the tool.
    """
    problems: list[str] = []

    union: set[str] = set()
    for tools in surfaces.values():
        union |= set(tools)

    for tool in sorted(union):
        present = [lang for lang in order if tool in surfaces[lang]]
        problems.extend(
            f"{tool}{_ABSENCE_MARK.rstrip()} {lang}"
            for lang in order
            if tool not in surfaces[lang]
        )

        reference = present[0]
        for lang in present[1:]:
            problems.extend(
                _compare_one(
                    tool,
                    reference,
                    surfaces[reference][tool],
                    lang,
                    surfaces[lang][tool],
                )
            )

    return problems


def _compare_one(
    name: str,
    ref_lang: str,
    ref: dict[str, Any],
    other_lang: str,
    other: dict[str, Any],
) -> list[str]:
    """Compare one tool's capability, params, types, and required set."""
    out: list[str] = []

    if ref["capability"] != other["capability"]:
        out.append(
            f"{name}: capability {ref_lang}={ref['capability']} "
            f"{other_lang}={other['capability']}"
        )

    ref_params, other_params = ref["params"], other["params"]

    out.extend(
        f"{name}: param '{param}' in {ref_lang} but not {other_lang}"
        for param in sorted(set(ref_params) - set(other_params))
    )
    out.extend(
        f"{name}: param '{param}' in {other_lang} but not {ref_lang}"
        for param in sorted(set(other_params) - set(ref_params))
    )
    out.extend(
        f"{name}: param '{param}' type "
        f"{ref_lang}={ref_params[param] or '?'} "
        f"{other_lang}={other_params[param] or '?'}"
        for param in sorted(set(ref_params) & set(other_params))
        if ref_params[param] != other_params[param]
    )

    if ref["required"] != other["required"]:
        out.append(
            f"{name}: required {ref_lang}={ref['required']} "
            f"{other_lang}={other['required']}"
        )

    if ref["scopes"] != other["scopes"]:
        out.append(
            f"{name}: scopes {ref_lang}={ref['scopes']} {other_lang}={other['scopes']}"
        )

    return out


def _is_absence(entry: str) -> bool:
    """Report whether a divergence entry records a per-language absence."""
    return _ABSENCE_MARK in entry


def _report_unannotated(pending: list[str]) -> None:
    """Explain the annotation an accepted absence entry must carry."""
    print(f"\nabsence entries missing a tracking annotation ({len(pending)}):")
    for entry in pending:
        print(f"  {entry}")
    print(
        "\nEach accepted absence documents who will close it. Append to the"
        " line in docs/contracts/tool-parity-baseline.txt:"
        "\n  <entry>  # accepted YYYY-MM-DD <tracking-issue URL>"
    )


def _require_scope_signal(surfaces: dict[str, dict[str, dict[str, Any]]]) -> None:
    """Fail when a language's dump carries no scopes at all.

    Per-tool empty scopes are legitimate (Meta tools touch no Linode API),
    but a whole surface without a single scope means the dumper stopped
    emitting the field, and comparing empty-to-empty would pass silently.
    """
    for lang, tools in surfaces.items():
        if not any(rec["scopes"] for rec in tools.values()):
            msg = (
                f"{lang} dump carries no scopes for any tool;"
                " its parity dumper stopped emitting the scopes field"
            )
            raise SystemExit(msg)


def main() -> int:
    languages = _load_languages()
    order = [name for name, _, _ in languages]
    surfaces = {name: _dump_surface(name, cwd, argv) for name, cwd, argv in languages}
    _require_scope_signal(surfaces)

    union: set[str] = set()
    for tools in surfaces.values():
        union |= set(tools)

    current = set(_compare(surfaces, order))
    stored = _baselines.read_baseline(_BASELINE)
    baseline = set(stored)

    # The gate is a ratchet: the baseline can only shrink. New divergences
    # (not in the baseline) and stale baseline entries (fixed, so no longer
    # diverging) both fail, so the file stays accurate and the count drops.
    # Accepted absences additionally require a tracking annotation, so a
    # language-specific gap is a visible commitment rather than silent debt.
    if "--update-baseline" in sys.argv:
        _baselines.write_baseline(_BASELINE, _BASELINE_HEADER, current, stored)
        print(f"baseline updated: {len(current)} accepted divergence(s)")

        pending = _baselines.unannotated(filter(_is_absence, current), stored)
        if pending:
            _report_unannotated(pending)

        return 0

    new = sorted(current - baseline)
    fixed = sorted(baseline - current)
    pending = _baselines.unannotated(filter(_is_absence, current & baseline), stored)

    if not new and not fixed and not pending:
        print(
            f"tool parity OK: {len(union)} tools across "
            f"{len(order)} language(s), "
            f"{len(baseline)} known divergence(s) unchanged"
        )
        return 0

    if new:
        print(f"NEW tool-parity divergence(s) ({len(new)}):")
        for line in new:
            print(f"  {line}")
    if fixed:
        print(f"\nFIXED since baseline ({len(fixed)}) - remove these lines:")
        for line in fixed:
            print(f"  {line}")
        print("\nRun: python scripts/verify_tool_parity.py --update-baseline")
    if pending:
        _report_unannotated(pending)

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
