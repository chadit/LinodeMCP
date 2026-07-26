#!/usr/bin/env python3
"""Offline gate: list tools must paginate when their route paginates in the spec.

The Linode API is the reference. docs/contracts/api-pagination-baseline.txt is
the reviewed snapshot of every GET route the spec paginates (written by the
scheduled scripts/verify_sync_pagination.py, so this gate stays hermetic). For
each tool whose behavior fixture issues a GET to one of those routes, the
tool's proto input message must expose page and page_size; a client otherwise
has no way to reach past the API's default first page.

Known gaps live in docs/contracts/pagination-baseline.txt, a ratchet: fix a
tool and remove its line; never add a line by hand (regenerate with
--update-baseline, then attach the required acceptance annotation).

Stdlib only, so no venv is needed. Run via `make pagination` (in `make check`).

Usage: verify_pagination.py [--update-baseline]
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

import _baselines
import _surface

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SNAPSHOT = _REPO_ROOT / "docs" / "contracts" / "api-pagination-baseline.txt"
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "pagination-baseline.txt"
_FIXTURES = _REPO_ROOT / "testdata" / "behavior"

_BASELINE_HEADER = (
    "# Tools whose GET route paginates in the Linode API spec but whose input\n"
    "# contract has no page/page_size. Ratchet: add pagination to the tool in\n"
    "# every language (proto first) and remove its line; never add a line by\n"
    "# hand (regenerate instead, then attach the required annotation).\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_pagination.py --update-baseline\n"
    "# Spec side comes from docs/contracts/api-pagination-baseline.txt\n"
    "# (scripts/verify_sync_pagination.py owns that snapshot).\n"
)


def snapshot_routes(path: Path) -> set[str]:
    """Template paths of paginated GET routes from the sync snapshot."""
    routes: set[str] = set()
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split()
        if len(parts) >= 2 and parts[0] == "GET":
            routes.add(parts[1])
    return routes


def fixture_get_paths() -> dict[str, set[str]]:
    """Tool name to the GET paths its behavior fixture pins via expect_request."""
    out: dict[str, set[str]] = {}
    for fixture in sorted(_FIXTURES.glob("*.json")):
        doc = json.loads(fixture.read_text(encoding="utf-8"))
        tool = doc.get("tool")
        if not isinstance(tool, str):
            continue
        for case in doc.get("cases", []):
            request = case.get("expect_request")
            if isinstance(request, dict) and request.get("method") == "GET":
                path = str(request.get("path", "")).split("?")[0]
                if path:
                    out.setdefault(tool, set()).add(path)
    return out


def match_template(concrete: str, templates: set[str]) -> str | None:
    """Best template for a concrete path: literal segments beat placeholders."""
    concrete_parts = concrete.strip("/").split("/")
    best: str | None = None
    best_score = -1
    for template in templates:
        template_parts = template.strip("/").split("/")
        if len(template_parts) != len(concrete_parts):
            continue
        score = 0
        for concrete_part, template_part in zip(
            concrete_parts, template_parts, strict=True
        ):
            if template_part.startswith("{"):
                continue
            if template_part != concrete_part:
                score = -1
                break
            score += 1
        if score > best_score:
            best = template
            best_score = score
    return best if best_score >= 0 else None


def tool_input_messages() -> dict[str, str]:
    """Tool name to proto input message, via the shared surface reader."""
    return _surface.tool_input_messages()


def paginated_messages() -> dict[str, bool]:
    """Proto message name to whether it declares page and page_size fields."""
    return {
        name: _surface.message_has_field(body, "page")
        and _surface.message_has_field(body, "page_size")
        for name, body in _surface.proto_message_bodies().items()
    }


def current_violations() -> tuple[list[str], int]:
    """Violation entries plus how many snapshot routes have no fixture-mapped tool."""
    routes = snapshot_routes(_SNAPSHOT)
    tool_messages = tool_input_messages()
    messages = paginated_messages()

    violations: list[str] = []
    covered: set[str] = set()
    for tool, paths in sorted(fixture_get_paths().items()):
        for concrete in sorted(paths):
            template = match_template(concrete, routes)
            if template is None:
                continue
            covered.add(template)
            message = tool_messages.get(tool)
            if message is None or not messages.get(message, False):
                violations.append(f"{tool}: GET {template} unpaginated")
            break
    return sorted(set(violations)), len(routes - covered)


# How each registered language declares the standard page_size bounds. The
# pattern's two groups are the constant name and its value. A registered
# language missing an entry fails by name, so registering one forces the
# decision rather than leaving its bounds unchecked.
_BOUND_PATTERNS: dict[str, str] = {
    "go": r"(\w*PageSize(?:Min|Max))\s*=\s*(\d+)",
    "python": r"([A-Z_]*PAGE_SIZE_(?:MIN|MAX))\s*=\s*(\d+)",
}

_SOURCE_SUFFIXES = {"go": ".go", "python": ".py"}


def snapshot_bounds(path: Path) -> set[tuple[int, int]]:
    """Every distinct (min, max) page_size pair the spec snapshot records."""
    return {
        (int(m.group(1)), int(m.group(2)))
        for m in re.finditer(r"page_size=(\d+)-(\d+)", path.read_text(encoding="utf-8"))
    }


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
    return languages


def declared_bounds(name: str, workdir: Path) -> list[tuple[str, str, int]]:
    """(file, constant, value) for every page_size bound one language declares."""
    pattern = re.compile(_BOUND_PATTERNS[name])
    suffix = _SOURCE_SUFFIXES[name]
    found: list[tuple[str, str, int]] = []
    for path in sorted(workdir.rglob(f"*{suffix}")):
        relative = path.relative_to(workdir)
        if (
            any(part.startswith(".") for part in relative.parts)
            or "genpb" in relative.parts
        ):
            continue
        # Reported as "<language>/<path under its workdir>", which reconstructs
        # the repo-relative path without depending on where the tree sits.
        display = f"{name}/{relative.as_posix()}"
        found.extend(
            (display, match.group(1), int(match.group(2)))
            for match in pattern.finditer(path.read_text(encoding="utf-8"))
        )
    return found


def bound_violations() -> list[str]:
    """Bounds hardcoded in a language that disagree with the spec snapshot.

    The snapshot is generated from the live spec, so it is the only place these
    two numbers are a fact rather than a copy. Every route in it currently
    shares one pair; if that ever stops being true, a single standard pair can
    no longer be right for every tool and this says so instead of comparing
    against an arbitrary one.
    """
    bounds = snapshot_bounds(_SNAPSHOT)
    if len(bounds) != 1:
        return [
            (
                f"the spec snapshot records {len(bounds)} distinct page_size bound"
                f" pairs ({sorted(bounds)}); a single standard pair can no longer"
                " cover every route, so the per-route bound must reach the code"
            )
        ]

    minimum, maximum = next(iter(bounds))
    problems: list[str] = []
    for name, workdir in registered_languages(_LANGUAGES):
        if name not in _BOUND_PATTERNS:
            problems.append(
                f"{name}: no page_size bound pattern declared; add one to"
                " _BOUND_PATTERNS in scripts/verify_pagination.py"
            )
            continue
        for file, constant, value in declared_bounds(name, workdir):
            want = minimum if constant.upper().endswith("MIN") else maximum
            if value != want:
                problems.append(
                    f"{file}: {constant} = {value}, but the spec snapshot says {want}"
                )
    return problems


def main(argv: list[str]) -> int:
    drift = bound_violations()
    if drift:
        print("page_size bounds disagree with the spec snapshot:", file=sys.stderr)
        for entry in drift:
            print(f"  {entry}", file=sys.stderr)
        return 1

    violations, unmapped = current_violations()

    if "--update-baseline" in argv:
        _baselines.write_baseline(
            _BASELINE, _BASELINE_HEADER, violations, _baselines.read_baseline(_BASELINE)
        )
        print(f"wrote {len(violations)} pagination gap(s)", file=sys.stderr)
        return 0

    baseline = _baselines.read_entries(_BASELINE)
    new = [entry for entry in violations if entry not in baseline]
    fixed = sorted(baseline - set(violations))

    if unmapped:
        print(
            f"note: {unmapped} paginated spec route(s) have no fixture-mapped tool;"
            " new-route coverage belongs to the api-alignment flow, not this gate."
        )
    if new:
        print("tools missing pagination for a spec-paginated route:", file=sys.stderr)
        for entry in new:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nExpose page/page_size on the tool's proto input in every language"
            " (docs/parity.md), or regenerate the baseline and annotate the"
            " accepted gap.",
            file=sys.stderr,
        )
    if fixed:
        print("pagination gaps fixed; remove their baseline lines:", file=sys.stderr)
        for entry in fixed:
            print(f"  {entry}", file=sys.stderr)
    if new or fixed:
        return 1

    print(
        f"pagination gate OK: {len(violations)} accepted gap(s),"
        f" no drift vs {_BASELINE.name}, bounds match the spec snapshot"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
