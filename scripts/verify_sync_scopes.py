#!/usr/bin/env python3
"""LIVE-check per-tool OAuth scopes against the Linode API spec.

The tool-parity gate proves every registered language maps the same
scopes per tool, but it cannot see when ALL languages drift from the API
docs together: that is exactly how 88 scope divergences and a further 16
silently-empty families accumulated before issue 1056. This gate closes
that hole by comparing the mapping against the spec's own per-operation
security blocks, the same source techdocs renders.

Only the Python mapping is compared here, on purpose: tool-parity pins
Go (and any future language) equal to Python per tool, so python-vs-spec
plus parity transitively pins every language against the docs without
this script growing a per-language dumper matrix.

The comparison needs to know which route each tool calls, and the tool
registry does not record that. The proto contract does: every non-meta
tool's *Input message carries a `tool_route` option naming the tool, the
method, and the path template, which makes the message the equivalent of
an OpenAPI operation object. verify_tool_routes.py pins those options
from both sides offline, so this gate reads them rather than re-checking
them.

The spec is NOT the route oracle here. It demonstrably lags techdocs, so
a route it has never published (the reserved-ip family) is a gap in the
spec rather than a defect in the contract; a tool whose route the spec
documents no operation for is counted and skipped. Whether a client can
actually build a declared route is verify_route_evidence.py's job, and
it runs offline on every push rather than weekly.

Drift classes reported, each one line, each baselineable:
- `<tool>: no route entry` - registry grew a tool with no annotation.
- `<tool>: route entry but tool not registered` - stale annotation.
- `<tool>: scopes doc=[...] mapped=[...]` - the mapping disagrees with
  the documented scopes. Deliberate deviations live in the baseline with
  an annotation naming the tracking issue; see the scopeOverrides
  docstrings in the per-language mapping files for the rationale.
- `<tool>: stale fixup, ...` - upstream changed a route whose documented
  scope this script pins in _UPSTREAM_SCOPE_FIXUPS (typos and
  non-grantable names); the entry must be dropped or updated so the
  fixup can never mask a real upstream change.

Deliberately NOT part of `make check`: it fetches the live OpenAPI spec,
so it is non-deterministic and offline-hostile. Run on a cron / by the
sync agent (`make sync-scopes`). Stdlib-only, but the tool dump comes
from `python -m linodemcp.parity_dump`, so the venv must exist unless
--dump supplies a saved dump file.

Usage: verify_sync_scopes.py [--spec PATH] [--dump PATH] [--update-baseline]

  --spec PATH        read the OpenAPI document from PATH instead of fetching
  --dump PATH        read the tool dump JSON from PATH instead of running
                     the Python parity dumper
  --update-baseline  rewrite docs/contracts/scope-sync-baseline.txt from the
                     current drift set, preserving existing annotations
"""

from __future__ import annotations

import json
import subprocess
import sys
import urllib.request
from pathlib import Path
from typing import Any

import _baselines
import _toolroutes

_REPO_ROOT = Path(__file__).resolve().parents[1]
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "scope-sync-baseline.txt"

SPEC_URL = (
    "https://raw.githubusercontent.com/linode/linode-api-openapi/main/openapi.json"
)

_METHODS = ("get", "post", "put", "delete")

# Documented scope values that cannot be encoded in the language
# mappings, pinned with the exact upstream value so a fixup goes stale
# loudly the moment the spec changes. Two classes only: malformed scope
# strings (a permission level or category name that does not exist),
# and names absent from every grantable-scope registry (the spec's own
# OAuth catalog and the techdocs scope list); requiring one of those
# would make profiles unsatisfiable for real tokens. Each entry maps
# (METHOD, template) to (documented value, effective value): the
# comparison substitutes the effective value only while the spec still
# documents exactly the pinned value, and reports a stale-fixup drift
# line otherwise.
_UPSTREAM_SCOPE_FIXUPS: dict[tuple[str, str], tuple[list[str], list[str]]] = {
    # "ips:read" is not a permission level; the family uses read_only.
    ("GET", "/networking/ipv6/ranges/{p}"): (["ips:read"], ["ips:read_only"]),
    # "linode" is a typo for the "linodes" category.
    ("GET", "/linode/instances/{p}/interfaces/settings"): (
        ["linode:read_only"],
        ["linodes:read_only"],
    ),
    # "placement" appears in no grantable-scope registry; the rest of
    # the placement-group family is documented under linodes:*.
    ("GET", "/placement/groups"): (["placement:read_only"], ["linodes:read_only"]),
    # "child_account" appears in no grantable-scope registry; the
    # parent-account routes otherwise sit under account:*.
    ("GET", "/account/child-accounts"): (
        ["child_account:read_only"],
        ["account:read_only"],
    ),
    ("GET", "/account/child-accounts/{p}"): (
        ["child_account:read_only"],
        ["account:read_only"],
    ),
    ("POST", "/account/child-accounts/{p}/token"): (
        ["child_account:read_write"],
        ["account:read_write"],
    ),
}

_EXEMPT = _REPO_ROOT / "docs" / "contracts" / "scope-sync-exempt.txt"


def _exempt_deviations() -> set[str]:
    """The deviations no work here can close, so the ratchet never holds them.

    Each line is <deviation>\\t<reason>; the reason is mandatory documentation
    but only the deviation matters for matching.
    """
    exempt: set[str] = set()

    if not _EXEMPT.exists():
        return exempt

    for raw in _EXEMPT.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            exempt.add(stripped.split("\t")[0].strip())

    return exempt


_BASELINE_HEADER = (
    "# Accepted (known) deviations between the tool scope mapping and the\n"
    "# Linode OpenAPI spec's per-operation security blocks. Ratchet: fix\n"
    "# one and remove its line; never add a line by hand (regenerate\n"
    "# instead, then attach the required annotation). Regenerate with:\n"
    "#   python3 scripts/verify_sync_scopes.py --update-baseline\n"
    "# Every entry MUST carry an annotation naming when it was accepted\n"
    "# and the tracking issue that will close it:\n"
    "#   <entry>  # accepted YYYY-MM-DD <tracking-issue URL>\n"
)


def norm_template(path: str) -> str:
    """Normalize a path to a placeholder template: '/a/{p}/b'.

    Parameter NAMES differ between the spec and the handlers (linodeId vs
    encoded_instance_id), so every `{...}` segment collapses to `{p}` and
    matching happens on shape. Shared with the other gates rather than written
    twice, since both sides of this comparison have to be shaped identically.
    """
    return _toolroutes.norm_template(path)


def contract_routes() -> dict[str, tuple[str, str]]:
    """The declared routes as {tool: (METHOD, template)}.

    Templates are normalized rather than trusted, so this side of the
    comparison and the spec side are shaped by the same function.
    """
    return {
        tool: (method, norm_template(path))
        for tool, (method, path) in _toolroutes.routes().items()
    }


def spec_operations(spec: dict[str, Any]) -> dict[str, dict[str, list[str]]]:
    """Map each spec path template to {METHOD: sorted documented scopes}.

    A route with no security requirement (public) and a route whose
    oauth scope list is empty (any authenticated token) both document
    "no scope required", so both collapse to []. That mirrors what the
    mapping expresses: an empty scope list.
    """
    operations: dict[str, dict[str, list[str]]] = {}
    for path, item in spec.get("paths", {}).items():
        template = norm_template(path.removeprefix("/{apiVersion}"))
        for method in _METHODS:
            operation = item.get(method)
            if not isinstance(operation, dict):
                continue
            scopes: list[str] = []
            for entry in operation.get("security") or []:
                oauth = entry.get("oauth")
                if oauth:
                    scopes = sorted(str(scope) for scope in oauth)
            operations.setdefault(template, {})[method.upper()] = scopes
    return operations


def compare(
    routes: dict[str, tuple[str, str]],
    dump: list[dict[str, Any]],
    operations: dict[str, dict[str, list[str]]],
) -> tuple[list[str], list[str]]:
    """One sorted drift line per disagreement, plus the routes the spec skips."""
    problems: list[str] = []
    undocumented: list[str] = []
    registered: set[str] = set()

    for record in dump:
        name = str(record["name"])
        if record.get("capability") == "Meta":
            continue
        registered.add(name)

        route = routes.get(name)
        if route is None:
            problems.append(f"{name}: no route entry")
            continue

        method, template = route
        documented = operations.get(template, {}).get(method)
        if documented is None:
            # The spec lags techdocs, so a route it never published documents
            # no scopes to compare against. That is a hole in the spec, not
            # drift here, and route-evidence proves the route is real.
            undocumented.append(f"{name}: {method} {template}")
            continue

        fixup = _UPSTREAM_SCOPE_FIXUPS.get((method, template))
        if fixup is not None:
            pinned, effective = fixup
            if documented == pinned:
                documented = effective
            else:
                problems.append(
                    f"{name}: stale fixup, doc changed from {pinned}"
                    f" to {documented}; drop or update the fixup entry"
                )
                continue

        mapped = sorted(str(scope) for scope in record.get("scopes") or [])
        if mapped != documented:
            problems.append(f"{name}: scopes doc={documented} mapped={mapped}")

    problems.extend(
        f"{tool}: route entry but tool not registered"
        for tool in routes
        if tool not in registered
    )

    return sorted(problems), sorted(undocumented)


def load_dump(path: str | None) -> list[dict[str, Any]]:
    """Load the tool dump from a file, or run the Python parity dumper."""
    if path is not None:
        parsed = json.loads(Path(path).read_text(encoding="utf-8"))
        return list(parsed)

    python = _REPO_ROOT / "python" / ".venv" / "bin" / "python"
    if not python.exists():
        raise SystemExit(
            "python/.venv missing; run `make -C python install-dev` or pass --dump"
        )
    # Fixed argv, no shell involved; the interpreter path is repo-owned.
    result = subprocess.run(
        [str(python), "-m", "linodemcp.parity_dump"],
        capture_output=True,
        text=True,
        check=False,
        cwd=_REPO_ROOT / "python",
    )
    if result.returncode != 0:
        raise SystemExit(f"parity dump failed:\n{result.stderr}")
    return list(json.loads(result.stdout))


def load_spec(path: str | None) -> dict[str, Any]:
    """Load the OpenAPI document from a file or the live repository."""
    if path is not None:
        loaded: dict[str, Any] = json.loads(Path(path).read_text(encoding="utf-8"))
        return loaded
    with urllib.request.urlopen(SPEC_URL, timeout=60) as resp:
        fetched: dict[str, Any] = json.load(resp)
    return fetched


def _flag_value(argv: list[str], flag: str) -> str | None:
    """Return the value following ``flag`` in argv, or None."""
    if flag not in argv:
        return None
    index = argv.index(flag)
    if index + 1 >= len(argv):
        raise SystemExit(f"{flag} requires a value")
    return argv[index + 1]


def main(argv: list[str]) -> int:
    routes = contract_routes()
    dump = load_dump(_flag_value(argv, "--dump"))
    spec = load_spec(_flag_value(argv, "--spec"))
    version = str(spec.get("info", {}).get("version", "unknown"))

    exempt = _exempt_deviations()
    drift, undocumented = compare(routes, dump, spec_operations(spec))
    generated = set(drift)
    # Exempt deviations leave the ratchet entirely: they are upstream facts, not
    # work waiting on someone here, so a baseline line would be a promise with
    # nothing behind it.
    current = generated - exempt
    stale_exemptions = sorted(exempt - generated)
    stored = _baselines.read_baseline(_BASELINE)
    baseline = set(stored)

    if "--update-baseline" in argv:
        _BASELINE.parent.mkdir(parents=True, exist_ok=True)
        _baselines.write_baseline(_BASELINE, _BASELINE_HEADER, current, stored)
        print(f"baseline updated: {len(current)} accepted deviation(s)")
        pending = _baselines.unannotated(current, stored)
        if pending:
            print("annotate these lines (accepted YYYY-MM-DD <issue URL>):")
            for line in pending:
                print(f"  {line}")
        return 0

    new = sorted(current - baseline)
    fixed = sorted(baseline - current)
    pending = _baselines.unannotated(current & baseline, stored)

    if undocumented:
        # Named rather than only counted: these tools get no scope check at
        # all, so the set has to stay visible as it changes.
        print(f"routes the spec documents no operation for ({len(undocumented)}):")
        for line in undocumented:
            print(f"  {line}")

    if not new and not fixed and not pending and not stale_exemptions:
        print(
            f"sync-scopes OK: {len(routes) - len(undocumented)} of "
            f"{len(routes)} declared route(s) compared against spec "
            f"{version} ({len(undocumented)} the spec documents no operation "
            f"for), {len(baseline)} accepted deviation(s) unchanged, "
            f"{len(exempt)} exemption(s)"
        )
        return 0

    if new:
        print(f"NEW scope drift vs spec {version} ({len(new)}):")
        for line in new:
            print(f"  DRIFT {line}")
    if fixed:
        print(f"\nFIXED since baseline ({len(fixed)}) - remove these lines:")
        for line in fixed:
            print(f"  {line}")
        print("\nRun: python3 scripts/verify_sync_scopes.py --update-baseline")
    if pending:
        print("\nbaseline lines missing a valid annotation:")
        for line in pending:
            print(f"  {line}")
    if stale_exemptions:
        print(
            f"\nEXEMPTIONS that no longer apply ({len(stale_exemptions)}) - "
            "remove these lines from docs/contracts/scope-sync-exempt.txt:"
        )
        for line in stale_exemptions:
            print(f"  {line}")
        print(
            "\nThe spec now covers the route, so the exemption claims an"
            " absence that ended."
        )
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
