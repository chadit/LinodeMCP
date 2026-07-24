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

The comparison needs to know which route each tool calls, and nothing in
the tool registry records that. docs/contracts/tool-routes.txt is that
contract: one `<tool>: <METHOD> <path-template>` line per non-meta tool.
It is hand-maintained but machine-checked on every run from both sides
(every registered tool must have a line, every line must name a
registered tool and a route that exists in the spec), so it cannot rot
silently.

Drift classes reported, each one line, each baselineable:
- `<tool>: no route entry` - registry grew a tool the contract missed.
- `<tool>: route entry but tool not registered` - stale contract line.
- `<tool>: route <METHOD> <path> not in spec` - upstream removed or has
  never documented the route (the reserved-ip family today).
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

_REPO_ROOT = Path(__file__).resolve().parents[1]
_ROUTES = _REPO_ROOT / "docs" / "contracts" / "tool-routes.txt"
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
    matching happens on shape. Query strings never participate.
    """
    bare = path.split("?", maxsplit=1)[0]
    segments = [
        "{p}" if segment.startswith("{") else segment
        for segment in bare.strip("/").split("/")
    ]
    return "/" + "/".join(segments)


def parse_routes(text: str) -> dict[str, tuple[str, str]]:
    """Parse tool-routes.txt into {tool: (METHOD, template)}.

    Malformed or duplicate lines are a broken contract, not drift, so
    they abort instead of becoming baseline entries.
    """
    routes: dict[str, tuple[str, str]] = {}
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        tool, sep, rest = line.partition(": ")
        parts = rest.split()
        expected_parts = 2
        if not sep or len(parts) != expected_parts:
            raise SystemExit(f"tool-routes.txt: malformed line: {line!r}")
        method, path = parts
        if method not in {m.upper() for m in _METHODS}:
            raise SystemExit(f"tool-routes.txt: unknown method in: {line!r}")
        if not path.startswith("/"):
            raise SystemExit(f"tool-routes.txt: path must start with /: {line!r}")
        if tool in routes:
            raise SystemExit(f"tool-routes.txt: duplicate entry for {tool}")
        routes[tool] = (method, norm_template(path))
    return routes


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
) -> list[str]:
    """Return one sorted drift line per disagreement."""
    problems: list[str] = []
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
            problems.append(f"{name}: route {method} {template} not in spec")
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

    return sorted(problems)


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
    routes = parse_routes(_ROUTES.read_text(encoding="utf-8"))
    dump = load_dump(_flag_value(argv, "--dump"))
    spec = load_spec(_flag_value(argv, "--spec"))
    version = str(spec.get("info", {}).get("version", "unknown"))

    current = set(compare(routes, dump, spec_operations(spec)))
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

    if not new and not fixed and not pending:
        print(
            f"sync-scopes OK: {len(routes)} route(s) checked against spec "
            f"{version}, {len(baseline)} accepted deviation(s) unchanged"
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
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
