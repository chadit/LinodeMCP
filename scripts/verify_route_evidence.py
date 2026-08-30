#!/usr/bin/env python3
"""Route-evidence gate: every declared route is one a client can actually build.

The proto contract records which Linode operation each tool calls, as a
`tool_route` option on the tool's input message. This gate proves each client
can build every declared route. All of that evidence is contract-derived now:
every request call site in both languages names its tool at a generated driver
primitive and carries no path of its own, so the scanners measure zero
hand-assembled URLs and zero unresolved sites. The hand-assembly resolution
stays anyway, as the tripwire: a call site that builds its URL by hand again
either resolves into evidence or fails the gate as unresolved, rather than
hiding from a catalog grep the way
fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", encodedLinodeID) once hid
from a scan for "/linode/instances/{id}/interfaces".

Each client's route surface is resolved from source and the contract is checked
against it. Go resolves through go/cmd/route-dump (AST, no build, no imports,
so the gitignored genpb tree cannot break it); Python resolves here with ast.
Both find their request primitives structurally, by the function that takes a
method and an endpoint and builds an HTTP request out of them, so a new wrapper
is picked up in either language without an edit.

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

This is a HARD gate: a contracted route a language cannot build, or a request
call site its scanner cannot follow, fails by name. There is no baseline file
and no acceptance path. A route in the contract with no code behind it is a
tool that raises the first time someone calls it, and an unresolved call site
is the gate losing its own reading of that language, so neither is a state to
sit in. A tool a language has not caught up on yet is recorded once, as an
annotated absence in docs/contracts/tool-parity-baseline.txt.

Stdlib plus scripts/_toolroutes.py, which reads the declared routes from the
generated descriptors (through python/.venv/bin/python when the running
interpreter cannot import them); the Go scanner needs the Go toolchain. Run via
`make route-evidence` (in `make check`, and so in the pre-push hook and the CI
gate on every branch).

Usage: verify_route_evidence.py [--go-routes PATH]
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import _hardgate
import _routescan
import _toolroutes

if TYPE_CHECKING:
    from collections.abc import Callable

    # A language scanner takes that language's working dir and returns the
    # routes it can build plus the call sites it could not follow.
    Scanner = Callable[[Path], _routescan.Evidence]

_REPO_ROOT = Path(__file__).resolve().parents[1]
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"


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


def contract_messages() -> dict[str, str]:
    """The declared routes keyed by input message, as {message: tool}.

    The message is named the way a call site writes it, without its package,
    because that is all a source reader can see. The scanners resolve a typed
    lookup through this map and then through contract_routes, so the message
    the call site names and the route the gate checks come from one reading of
    the descriptors.
    """
    return {
        entry.message.rsplit(".", 1)[-1]: entry.tool
        for entry in _toolroutes.tool_routes()
    }


def go_evidence(
    workdir: Path,
    dump_path: str | None = None,
    declared: dict[str, str] | None = None,
    messages: dict[str, str] | None = None,
) -> _routescan.Evidence:
    """Resolve Go's route surface through cmd/route-dump.

    A non-zero exit (a renamed request primitive, an unparsable file) raises, so
    the gate fails loudly rather than treating Go as a language with no routes
    and reporting every contracted route as missing.

    declared is the contract in the gate's own shape, {tool: "<METHOD> <path>"},
    and messages the same contract as {input message: tool}. The dump cannot
    supply either: cmd/route-dump reads source with go/ast and never imports
    the generated descriptors, so a call site that names a tool, or hands a
    message type to the typed lookup, arrives here as a name, and this is
    where it becomes a route.
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

    routes = {str(route) for route in raw.get("routes", [])}
    unresolved = [str(site) for site in raw.get("unresolved", [])]

    for entry in raw.get("contracted", []):
        site = str(entry.get("site", ""))
        tool = str(entry.get("tool", ""))
        message = str(entry.get("message", ""))
        if message:
            tool = (messages or {}).get(message, "")
            if not tool:
                unresolved.append(f"{site}: unknown message {message!r}")
                continue
        route = (declared or {}).get(tool)
        if route is None:
            unresolved.append(_undeclared(site, tool))
            continue
        routes.add(route)

    return _routescan.Evidence(routes=routes, unresolved=unresolved)


def _undeclared(site: str, tool: str) -> str:
    """Name a call site that resolves its route from a tool nothing declares.

    Its own error class rather than silence: the name is the whole route at such
    a site, so a typo is a call that raises the first time it runs, and this is
    the offline reading that can still catch it. The wording matches what
    scripts/_routescan.py emits for the same case, so both languages report it
    the same way.
    """
    return f"{site}: undeclared tool {tool!r}"


def python_evidence(workdir: Path) -> _routescan.Evidence:
    """Resolve Python's route surface from the source tree under workdir.

    The contract goes in because a call site that resolves its route from the
    proto names its tool and no path, so the declaration is where its route
    lives. Evidence for those call sites is that the tool is named and declared,
    which is all there is to be right about once the path is single-sourced. It
    goes in keyed by message too, for the call sites that name the tool's input
    message to the typed lookup rather than the tool.
    """
    return _routescan.scan_python(
        workdir, _REPO_ROOT, contract_routes(), contract_messages()
    )


def coverage(go_routes: str | None = None) -> dict[str, Scanner]:
    """How each registered language's route surface is resolved.

    Built per call rather than held as a constant so the Go scanner can be
    pointed at a recorded dump, which is what lets the gate's own tests run
    without the Go toolchain.

    Both scanners get the contract, because in both languages a call site that
    resolves its route from the proto names a tool and no path. Go takes it as
    an argument since its dump comes back from a subprocess that cannot read the
    descriptors at all.
    """
    return {
        "go": lambda workdir: go_evidence(
            workdir, go_routes, contract_routes(), contract_messages()
        ),
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


def current_gaps(
    go_routes: str | None = None, routes: dict[str, str] | None = None
) -> list[str]:
    """Every route gap across every registered language.

    routes is the declared contract, read here when a caller does not supply
    it. main passes the set it already read, since reading it means going back
    to the generated descriptors.

    Each language's resolved route surface has to be non-empty: a scanner that
    resolves nothing would otherwise report the whole contract as missing in
    that language, which is a wall of findings pointing at the scanner rather
    than at the code. It fails as the one thing it is.
    """
    if routes is None:
        routes = contract_routes()
    scanners = coverage(go_routes)

    _hardgate.measured("the proto contract's route declarations", len(routes))

    gaps: list[str] = []
    for name, workdir in registered_languages(_LANGUAGES):
        scanner = scanners.get(name)
        if scanner is None:
            continue
        evidence = scanner(workdir)
        _hardgate.measured(f"the {name} route scan of {workdir}", len(evidence.routes))
        gaps.extend(language_gaps(name, routes, evidence))

    return sorted(gaps)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
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

    declared = contract_routes()
    gaps = current_gaps(args.go_routes, declared)

    if gaps:
        print("routes with no client evidence:", file=sys.stderr)
        for entry in gaps:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nEither the route is unimplemented in that language, or it is"
            " built in a shape the scanner cannot follow. An unresolved entry"
            " names the call site to teach it.",
            file=sys.stderr,
        )
        return 1

    print(
        f"route-evidence guard OK: every declared route ({len(declared)})"
        " resolves in every scanned language"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
