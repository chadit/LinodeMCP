#!/usr/bin/env python3
"""Route-evidence gate: every declared route is one a client can actually build.

The proto contract records which Linode operation each tool calls, as a
`tool_route` option on the tool's input message. Nothing checked those
declarations against the clients. A route could sit in the contract with no code
behind it in one language, and the catalog scan that goes looking finds nothing
when the path is assembled rather than written out: no grep for
"/linode/instances/{id}/interfaces" matches
fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", encodedLinodeID). Disproving
one of those false positives costs a full investigation, and the last one ended
in a hand-written test pinning a single route by name.

This resolves each client's route surface from source instead and checks the
contract against it. Go resolves through go/cmd/route-dump (AST, no build, no
imports, so the gitignored genpb tree cannot break it); Python resolves here
with ast. Both find their request primitives structurally, by the function that
takes a method and an endpoint and builds an HTTP request out of them, so a new
wrapper is picked up in either language without an edit.

What gets scanned comes from docs/contracts/languages.txt rather than a path
written here. COVERAGE says how each registered language is resolved, and a
language registered without an entry fails the gate by name.

Scope differs between the two because the clients do. Go builds every route
inside its client package. Python also builds them in the tool layer, through
get_raw/post_raw/put_raw, so the Python scan covers the whole source tree.

The gate proves a client can build a route, not that a tool exposes it. Tool
coverage belongs to verify_tool_parity.py, and the two disagree on purpose: Go
has a GetReservedIP client method with no tool in front of it, which counts as
evidence here and stays a parity gap there.

Known gaps live in docs/contracts/route-evidence-baseline.txt, a ratchet: build
the route and remove its line; never add a line by hand (regenerate with
--update-baseline, then attach the required acceptance annotation).

Stdlib plus scripts/_toolroutes.py, which reads the declared routes from the
generated descriptors (through python/.venv/bin/python when the running
interpreter cannot import them); the Go scanner needs the Go toolchain. Run via
`make route-evidence` (in `make check`, and so in the pre-push hook and the CI
gate on every branch).

Usage: verify_route_evidence.py [--update-baseline] [--go-routes PATH]
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import _baselines
import _routescan
import _toolroutes

if TYPE_CHECKING:
    from collections.abc import Callable

    # A language scanner takes that language's working dir and returns the
    # routes it can build plus the call sites it could not follow.
    Scanner = Callable[[Path], _routescan.Evidence]

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "route-evidence-baseline.txt"

_BASELINE_HEADER = (
    "# Contracted routes no client builds, and request call sites a scanner\n"
    "# could not follow. Ratchet: build the route (or teach the scanner the\n"
    "# shape) and remove its line; never add a line by hand (regenerate\n"
    "# instead, then attach the required annotation).\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_route_evidence.py --update-baseline\n"
)


def contract_routes() -> dict[str, str]:
    """The declared routes as {tool: "<METHOD> <path>"}, the scanners' shape.

    A declared path names its parameters; a resolved one cannot, because the
    scanners read code that builds URLs out of variables. Names are normalized
    off here, at the comparison, rather than being kept out of the contract:
    the gate is about whether a client can build a route of that shape, and the
    name a route gives a segment was never part of that question.
    """
    return {
        tool: f"{method} {_toolroutes.norm_template(path)}"
        for tool, (method, path) in _toolroutes.routes().items()
    }


def go_evidence(workdir: Path, dump_path: str | None = None) -> _routescan.Evidence:
    """Resolve Go's route surface through cmd/route-dump.

    A non-zero exit (a renamed request primitive, an unparsable file) raises, so
    the gate fails loudly rather than treating Go as a language with no routes
    and reporting every contracted route as missing.
    """
    if dump_path:
        raw = json.loads(Path(dump_path).read_text(encoding="utf-8"))
    else:
        # Fixed argv, no shell, no input from outside the repo.
        proc = subprocess.run(
            ["go", "run", "./cmd/route-dump"],
            cwd=workdir,
            capture_output=True,
            text=True,
            check=False,
        )
        if proc.returncode != 0:
            msg = (
                f"cmd/route-dump failed (exit {proc.returncode}): {proc.stderr.strip()}"
            )
            raise RuntimeError(msg)
        raw = json.loads(proc.stdout)

    return _routescan.Evidence(
        routes={str(route) for route in raw.get("routes", [])},
        unresolved=[str(site) for site in raw.get("unresolved", [])],
    )


def python_evidence(workdir: Path) -> _routescan.Evidence:
    """Resolve Python's route surface from the source tree under workdir."""
    return _routescan.scan_python(workdir, _REPO_ROOT)


def coverage(go_routes: str | None = None) -> dict[str, Scanner]:
    """How each registered language's route surface is resolved.

    Built per call rather than held as a constant so the Go scanner can be
    pointed at a recorded dump, which is what lets the gate's own tests run
    without the Go toolchain.
    """
    return {
        "go": lambda workdir: go_evidence(workdir, go_routes),
        "python": python_evidence,
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

    if not languages:
        msg = f"{path.name} registers no languages"
        raise SystemExit(msg)

    return languages


def undeclared_languages(languages: list[tuple[str, Path]]) -> list[str]:
    """Registered languages with no route scanner."""
    scanners = coverage()
    return sorted(name for name, _ in languages if name not in scanners)


def language_gaps(
    language: str, routes: dict[str, str], evidence: _routescan.Evidence
) -> list[str]:
    """Every gap one language has: a contracted route it cannot build, and a
    request call site the scanner could not follow.

    Both are reported because both hide the same thing. A route with no evidence
    is either unimplemented or built in a shape the scanner cannot read, and an
    unresolved call site is where that shape lives.
    """
    gaps = [
        f"{language} missing {tool}: {route}"
        for tool, route in sorted(routes.items())
        if route not in evidence.routes
    ]
    gaps += [f"{language} unresolved {site}" for site in evidence.unresolved]

    return gaps


def current_gaps(go_routes: str | None = None) -> list[str]:
    """Every route gap across every registered language."""
    routes = contract_routes()
    scanners = coverage(go_routes)

    gaps: list[str] = []
    for name, workdir in registered_languages(_LANGUAGES):
        scanner = scanners.get(name)
        if scanner is None:
            continue
        gaps.extend(language_gaps(name, routes, scanner(workdir)))

    return sorted(gaps)


def _report(new: list[str], fixed: list[str]) -> None:
    """Print what changed against the baseline, in the gate's own vocabulary."""
    if new:
        print("routes with no client evidence:", file=sys.stderr)
        for entry in new:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nEither the route is unimplemented in that language, or it is"
            " built in a shape the scanner cannot follow. An unresolved entry"
            " names the call site to teach it.",
            file=sys.stderr,
        )

    if fixed:
        print(
            "route-evidence baseline entries are fixed; remove them: "
            f"{', '.join(fixed)}",
            file=sys.stderr,
        )


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--update-baseline", action="store_true")
    parser.add_argument(
        "--go-routes",
        help="read Go's route surface from a recorded cmd/route-dump JSON file",
    )
    args = parser.parse_args(argv)

    undeclared = undeclared_languages(registered_languages(_LANGUAGES))
    if undeclared:
        print(
            "no route scanner declared for registered language(s): "
            f"{', '.join(undeclared)}. Add one to coverage() in "
            "scripts/verify_route_evidence.py.",
            file=sys.stderr,
        )
        return 1

    gaps = current_gaps(args.go_routes)

    if args.update_baseline:
        _baselines.write_baseline(
            _BASELINE, _BASELINE_HEADER, gaps, _baselines.read_baseline(_BASELINE)
        )
        print(f"wrote {len(gaps)} route-evidence gap(s)", file=sys.stderr)
        return 0

    baseline = _baselines.read_entries(_BASELINE)
    new = [entry for entry in gaps if entry not in baseline]
    fixed = sorted(baseline - set(gaps))

    _report(new, fixed)

    if new or fixed:
        return 1

    print(f"route-evidence guard OK: {len(gaps)} accepted gap(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
