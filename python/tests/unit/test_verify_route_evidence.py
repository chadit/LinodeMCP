"""Focused tests for the route-evidence gate and its Python route scanner."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"

# The request primitive every fixture needs. Without a call reaching the HTTP
# library the scanner hard-fails, which is its own test below. Fixture sources
# quote with single quotes so they can nest inside these docstrings.
PRIMITIVE = """
class Client:
    async def make_request(self, method, endpoint, body=None):
        url = self.base_url + endpoint
        return await self.client.request(method, url, headers={}, json=body)
"""


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_route_evidence")
routescan = _load_script("_routescan")


def _scan(tmp_path: Path, source: str) -> Any:
    """Scan one fixture module written into its own tree."""
    (tmp_path / "client.py").write_text(PRIMITIVE + source, encoding="utf-8")
    return routescan.scan_python(tmp_path, tmp_path)


def test_resolves_every_endpoint_shape(tmp_path: Path) -> None:
    """Each shape the real client builds resolves to the route it requests.

    Named shapes rather than a count, so a resolver that stops understanding one
    of them fails with that shape named.
    """
    evidence = _scan(
        tmp_path,
        """
def paginated_path(path, page, page_size):
    # Pagination is a query string, so it is not part of the route.
    if page is None:
        return path
    return path + '?page=1'


class Handlers(Client):
    async def literal(self):
        return await self.make_request('GET', '/things')

    async def interpolated(self, thing_id):
        endpoint = f'/things/{thing_id}/parts'
        return await self.make_request('DELETE', endpoint)

    async def concatenated(self, name):
        endpoint = '/things/named/' + quote(name)
        return await self.make_request('PUT', endpoint)

    async def appended_query(self, params):
        endpoint = '/things/queried'
        endpoint += f'?{urlencode(params)}'
        return await self.make_request('GET', endpoint)

    async def conditional(self, query):
        endpoint = f'/things/maybe?{query}' if query else '/things/maybe'
        return await self.make_request('GET', endpoint)

    async def through_helper(self, page):
        return await self.make_request('GET', paginated_path('/things/paged', page, 1))
""",
    )

    assert evidence.routes == {
        "GET /things",
        "DELETE /things/{p}/parts",
        "PUT /things/named/{p}",
        "GET /things/queried",
        "GET /things/maybe",
        "GET /things/paged",
    }
    assert evidence.unresolved == []


def test_resolves_a_wrapper_that_forwards_its_endpoint(tmp_path: Path) -> None:
    """A wrapper taking an endpoint is discovered, so its callers resolve too.

    This is what keeps the scanner from needing a hand-maintained list of
    wrappers, which would go stale the first time someone added one.
    """
    evidence = _scan(
        tmp_path,
        """
class Raw(Client):
    async def get_raw(self, endpoint):
        return await self.make_request('GET', endpoint)


class Tools:
    async def list_regions(self, client):
        return await client.get_raw('/regions')
""",
    )

    assert evidence.routes == {"GET /regions"}
    assert evidence.unresolved == []


def test_resolves_a_twin_that_forwards_by_name_alone(tmp_path: Path) -> None:
    """The retry twin hands its callee off as a value rather than calling it.

    Only the shared name ties it to the plain wrapper, which is why declarations
    are pooled by name.
    """
    evidence = _scan(
        tmp_path,
        """
class Raw(Client):
    async def get_raw(self, endpoint):
        return await self.make_request('GET', endpoint)


class Retryable:
    async def get_raw(self, endpoint):
        return await self._execute_with_retry(self.client.get_raw, endpoint)


class Tools:
    async def list_domains(self, client):
        return await client.get_raw('/domains')
""",
    )

    assert evidence.routes == {"GET /domains"}


def test_real_python_tree_resolves_domain_create_route() -> None:
    """The registered domain-create call chain proves POST /domains exists."""
    evidence = gate.python_evidence(REPO_ROOT / "python")

    assert "POST /domains" in evidence.routes
    assert not any(
        "python/src/linodemcp/tools/linode_domains_write.py:" in site
        and "handle_linode_domain_create" in site
        for site in evidence.unresolved
    )


def test_resolves_a_request_that_skips_the_shared_primitive(tmp_path: Path) -> None:
    """A method reaching the HTTP library directly still yields its route.

    The thumbnail routes send raw bytes and build their own URL, so seeding at
    the library call rather than at make_request is what covers them.
    """
    evidence = _scan(
        tmp_path,
        """
class Thumbnails(Client):
    async def update_thumbnail(self, client_id, png):
        endpoint = f'/account/oauth-clients/{quote(client_id)}/thumbnail'
        url = self.base_url + endpoint
        return await self.client.request('PUT', url, headers={}, content=png)
""",
    )

    assert evidence.routes == {"PUT /account/oauth-clients/{p}/thumbnail"}
    assert evidence.unresolved == []


def test_reports_what_it_cannot_follow(tmp_path: Path) -> None:
    """An endpoint the scanner cannot read is named rather than dropped.

    Dropping it would read as a client that does not build the route, which is
    the false negative the gate exists to remove.
    """
    evidence = _scan(
        tmp_path,
        """
class Handlers(Client):
    async def known(self):
        return await self.make_request('GET', '/known')

    async def mystery(self):
        return await self.make_request('GET', elsewhere.endpoint())
""",
    )

    assert evidence.routes == {"GET /known"}
    assert len(evidence.unresolved) == 1
    assert "mystery" in evidence.unresolved[0]
    assert "unresolved path" in evidence.unresolved[0]


def test_skips_test_code(tmp_path: Path) -> None:
    """A fixture endpoint in a test never counts as evidence of a route."""
    (tmp_path / "client.py").write_text(
        PRIMITIVE
        + """
class Handlers(Client):
    async def real(self):
        return await self.make_request('GET', '/real')
""",
        encoding="utf-8",
    )
    tests = tmp_path / "tests"
    tests.mkdir()
    (tests / "test_client.py").write_text(
        """
class Fake(Client):
    async def fake(self):
        return await self.make_request('GET', '/only-in-a-test')
""",
        encoding="utf-8",
    )

    evidence = routescan.scan_python(tmp_path, tmp_path)

    assert evidence.routes == {"GET /real"}


def test_a_tree_that_reaches_no_http_library_is_a_hard_fail(tmp_path: Path) -> None:
    """An empty result is a broken scanner, never a client with no routes.

    Returning it would report every contracted route as missing at once, which
    reads like a contract break instead of a scanner that stopped working.
    """
    (tmp_path / "client.py").write_text(
        "def unrelated():\n    return 1\n", encoding="utf-8"
    )

    with pytest.raises(routescan.ScannerError):
        routescan.scan_python(tmp_path, tmp_path)


def test_contract_routes_rejects_an_unparsable_line(tmp_path: Path) -> None:
    """A malformed contract line fails the gate instead of being skipped."""
    path = tmp_path / "tool-routes.txt"
    path.write_text("# comment\nlinode_thing_get GET /things\n", encoding="utf-8")

    with pytest.raises(SystemExit):
        gate.contract_routes(path)


def test_language_gaps_names_the_tool_and_the_route() -> None:
    """A gap says which tool contracted the route, not only which route."""
    routes = {"linode_thing_get": "GET /things/{p}", "linode_thing_list": "GET /things"}
    evidence = routescan.Evidence(routes={"GET /things"}, unresolved=["client.py:1 f"])

    assert gate.language_gaps("go", routes, evidence) == [
        "go missing linode_thing_get: GET /things/{p}",
        "go unresolved client.py:1 f",
    ]


def test_a_recorded_go_dump_is_read_instead_of_running_the_command(
    tmp_path: Path,
) -> None:
    """The recorded-dump path is what keeps the gate's tests off the toolchain."""
    dump = tmp_path / "routes.json"
    dump.write_text(
        json.dumps({"routes": ["GET /things"], "unresolved": ["a.go:1 f: x"]}),
        encoding="utf-8",
    )

    evidence = gate.go_evidence(tmp_path, str(dump))

    assert evidence.routes == {"GET /things"}
    assert evidence.unresolved == ["a.go:1 f: x"]


def test_every_registered_language_has_a_route_scanner() -> None:
    """A registered language with no scanner is unguarded surface."""
    languages = gate.registered_languages(
        REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )

    assert [name for name, _ in languages]
    assert gate.undeclared_languages(languages) == []


def test_baseline_entries_are_still_real() -> None:
    """A baseline line that no longer matches the tree is stale and must go.

    Only the Python half is recomputed here; Go's surface is pinned by the
    command's own tests, so this checks its entries name contracted routes
    rather than re-running the toolchain.
    """
    baseline = {
        line.split("  # ", 1)[0].strip()
        for line in (REPO_ROOT / "docs" / "contracts" / "route-evidence-baseline.txt")
        .read_text(encoding="utf-8")
        .splitlines()
        if line.strip() and not line.startswith("#")
    }
    routes = gate.contract_routes(REPO_ROOT / "docs" / "contracts" / "tool-routes.txt")
    contracted = {f"go missing {tool}: {route}" for tool, route in routes.items()}
    python_gaps = set(
        gate.language_gaps("python", routes, gate.python_evidence(REPO_ROOT / "python"))
    )

    assert {entry for entry in baseline if entry.startswith("python ")} <= python_gaps
    assert {entry for entry in baseline if entry.startswith("go ")} <= contracted
