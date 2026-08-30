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

# Every fixture needs this: a tree reaching no HTTP library hard-fails the
# scanner. Fixtures single-quote so they nest inside these docstrings.
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


def _scan(
    tmp_path: Path,
    source: str,
    declared: dict[str, str] | None = None,
    messages: dict[str, str] | None = None,
) -> Any:
    """Scan one fixture module written into its own tree."""
    (tmp_path / "client.py").write_text(PRIMITIVE + source, encoding="utf-8")
    return routescan.scan_python(tmp_path, tmp_path, declared, messages)


# What a client method looks like once its route comes from the proto instead of
# from a path written at the call site. The builder itself writes no path, so
# the scanner reads the tool name and the contract, not this body.
ROUTE_BUILDER = """
class Routed(Client):
    async def make_route_request(self, tool, *values, body=None):
        route = route_for(tool)
        endpoint = route.endpoint(*values)
        if body is None:
            return await self.make_request(route.method, endpoint)
        return await self.make_request(route.method, endpoint, body)
"""


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
    params = {}
    if page is not None:
        params['page'] = page
    if page_size is not None:
        params['page_size'] = page_size
    if not params:
        return path
    return f'{path}?{urlencode(params)}'


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


def test_resolves_a_call_site_that_names_its_tool(tmp_path: Path) -> None:
    """A call site whose route comes from the proto is evidence for that route.

    Nothing at the call site spells the path out, so the contract is what the
    tool name resolves through. The builder's own body contributes nothing,
    which is what keeps its two hand-off calls from reading as unresolved.
    """
    evidence = _scan(
        tmp_path,
        ROUTE_BUILDER
        + """
class Handlers(Routed):
    async def delete_domain(self, domain_id):
        return await self.make_route_request('linode_domain_delete', domain_id)
""",
        {"linode_domain_delete": "DELETE /domains/{p}"},
    )

    assert evidence.routes == {"DELETE /domains/{p}"}
    assert evidence.unresolved == []


def test_reports_a_tool_the_contract_never_declared(tmp_path: Path) -> None:
    """A misspelled tool name is a call that raises, caught here while offline."""
    evidence = _scan(
        tmp_path,
        ROUTE_BUILDER
        + """
class Handlers(Routed):
    async def delete_domain(self, domain_id):
        return await self.make_route_request('linode_domian_delete', domain_id)
""",
        {"linode_domain_delete": "DELETE /domains/{p}"},
    )

    assert evidence.routes == set()
    assert len(evidence.unresolved) == 1
    assert "undeclared tool 'linode_domian_delete'" in evidence.unresolved[0]


def test_reports_a_route_call_that_does_not_name_its_tool(tmp_path: Path) -> None:
    """A tool computed per call leaves no route to read, so it is named."""
    evidence = _scan(
        tmp_path,
        ROUTE_BUILDER
        + """
class Handlers(Routed):
    async def delete_anything(self, tool, resource_id):
        return await self.make_route_request(tool, resource_id)
""",
        {"linode_domain_delete": "DELETE /domains/{p}"},
    )

    assert evidence.routes == set()
    assert len(evidence.unresolved) == 1
    assert "delete_anything: unnamed tool" in evidence.unresolved[0]


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


def test_contract_routes_reads_the_scanners_shape() -> None:
    """Declared routes reach the scanners as the "<METHOD> <path>" they emit."""
    assert gate.contract_routes()["linode_tag_list"] == "GET /tags"


def test_contract_routes_drops_declared_parameter_names() -> None:
    """A declared name never reaches the comparison.

    Scanners resolve routes from code that builds URLs out of variables, so
    they emit the placeholder and nothing else. A name that survived to here
    would report every parameterized route as missing evidence.
    """
    routes = gate.contract_routes()

    assert routes["linode_tag_delete"] == "DELETE /tags/{p}"
    assert [
        route for route in routes.values() if "{" in route and "{p}" not in route
    ] == []


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


def test_a_go_call_site_that_names_its_tool_resolves_through_the_contract(
    tmp_path: Path,
) -> None:
    """The Go twin of the Python route-builder case.

    cmd/route-dump reads source and never imports the descriptors, so it hands
    over the tool name and the gate turns it into the declared route.
    """
    dump = tmp_path / "routes.json"
    dump.write_text(
        json.dumps(
            {
                "routes": [],
                "contracted": [{"tool": "linode_tag_delete", "site": "tags.go:79 f"}],
                "unresolved": [],
            }
        ),
        encoding="utf-8",
    )

    evidence = gate.go_evidence(
        tmp_path, str(dump), {"linode_tag_delete": "DELETE /tags/{p}"}
    )

    assert evidence.routes == {"DELETE /tags/{p}"}
    assert evidence.unresolved == []


def test_a_go_call_site_naming_an_undeclared_tool_is_reported(tmp_path: Path) -> None:
    """Same failure, same wording as the Python scanner, with the call site."""
    dump = tmp_path / "routes.json"
    dump.write_text(
        json.dumps(
            {
                "routes": [],
                "contracted": [{"tool": "linode_tag_delte", "site": "tags.go:79 f"}],
                "unresolved": [],
            }
        ),
        encoding="utf-8",
    )

    evidence = gate.go_evidence(
        tmp_path, str(dump), {"linode_tag_delete": "DELETE /tags/{p}"}
    )

    assert evidence.routes == set()
    assert evidence.unresolved == ["tags.go:79 f: undeclared tool 'linode_tag_delte'"]


def test_every_registered_language_has_a_route_scanner() -> None:
    """A registered language with no scanner is unguarded surface."""
    languages = gate.registered_languages(
        REPO_ROOT / "docs" / "contracts" / "languages.txt"
    )

    assert [name for name, _ in languages]
    assert gate.undeclared_languages(languages) == []


def test_python_builds_every_contracted_route_and_keeps_no_baseline() -> None:
    """Python must resolve the whole contract: there is no accepted-gap file.

    Only the Python half runs here; Go's surface comes from cmd/route-dump,
    which its own tests pin, and re-running the toolchain from a unit test
    would be the slow way to learn the same thing.
    """
    routes = gate.contract_routes()
    gaps = gate.language_gaps(
        "python", routes, gate.python_evidence(REPO_ROOT / "python")
    )

    assert gaps == []
    assert not (
        REPO_ROOT / "docs" / "contracts" / "route-evidence-baseline.txt"
    ).exists()


def test_a_scanner_that_resolves_nothing_fails(monkeypatch: pytest.MonkeyPatch) -> None:
    """An empty route surface would report the whole contract as missing.

    That is a wall of findings pointing at the scanner rather than at the code,
    so it fails as the one thing it is. The Python scanner refuses a tree with
    no HTTP call of its own (ScannerError), which leaves this for the scanners
    that answer without raising, such as a recorded Go dump.
    """
    empty: Any = routescan.Evidence(routes=set(), unresolved=[])

    def scan_nothing(_workdir: Path) -> Any:
        return empty

    def one_empty_scanner(_go_routes: str | None = None) -> dict[str, Any]:
        return {"go": scan_nothing}

    monkeypatch.setattr(gate, "coverage", one_empty_scanner)

    with pytest.raises(SystemExit, match="covered nothing"):
        gate.current_gaps(routes={"linode_tag_list": "GET /tags"})


def test_resolves_a_route_raw_call_that_names_its_tool(tmp_path: Path) -> None:
    """The raw JSON twin is a route builder too: callers hand it a tool only.

    Its body resolves the route and hands off to make_route_request, so the
    scanner must skip that body and read the tool name at each call site, the
    same treatment make_route_request gets. Before route_raw joined the
    builder set, the hand-off inside its body read as an unnamed tool.
    """
    evidence = _scan(
        tmp_path,
        ROUTE_BUILDER
        + """
class Raw(Routed):
    async def route_raw(self, tool, *values, body=None, query=None):
        response = await self.make_route_request(tool, *values, body=body)
        return response.json()

class Handlers(Raw):
    async def get_domain(self, domain_id):
        return await self.route_raw('linode_domain_get', domain_id)
""",
        {"linode_domain_get": "GET /domains/{p}"},
    )

    assert evidence.routes == {"GET /domains/{p}"}
    assert evidence.unresolved == []


def test_resolves_a_get_driver_call_that_names_its_tool(tmp_path: Path) -> None:
    """The single-resource driver is a route builder one level up.

    A generated handler names its tool and its path values and nothing else, so
    the route lives in the contract alone. The scanner has to read the tool off
    the `tool=` keyword and skip the driver body, which names no tool of its
    own, the same treatment the list and write drivers already get.
    """
    evidence = _scan(
        tmp_path,
        ROUTE_BUILDER
        + """
class Raw(Routed):
    async def route_raw(self, tool, *values, body=None, query=None):
        response = await self.make_route_request(tool, *values, body=body)
        return response.json()

async def run_get_tool(cfg, arguments, *, tool, path_values=None):
    client = Raw()
    return await client.route_raw(tool, *(path_values or {}).values())

async def handle_linode_domain_get(arguments, cfg):
    return await run_get_tool(
        cfg,
        arguments,
        tool='linode_domain_get',
        path_values={'domain_id': 5},
    )
""",
        {"linode_domain_get": "GET /domains/{p}"},
    )

    assert evidence.routes == {"GET /domains/{p}"}
    assert evidence.unresolved == []


# The typed shape the scope validator has: the call site names the tool's input
# message to the lookup and spells no tool at all.
TYPED_LOOKUP = (
    ROUTE_BUILDER
    + """
class Raw(Routed):
    async def route_raw(self, tool, *values, body=None, query=None):
        response = await self.make_route_request(tool, *values, body=body)
        return response.json()

class Validator:
    async def read_domain(self, client):
        return await client.route_raw(tool_of(DomainGetInput))

    async def read_any(self, client, message):
        return await client.route_raw(tool_of(message))
"""
)


def test_resolves_a_typed_lookup_call_through_the_contract(tmp_path: Path) -> None:
    """A call site that names a message to the lookup is evidence for its tool.

    The message maps to the tool and the tool to the route, both through the
    contract, so nothing at the site spells either. A lookup handed a parameter
    reads as a message of that name, which the contract does not know, so it is
    reported by that name rather than dropped.
    """
    evidence = _scan(
        tmp_path,
        TYPED_LOOKUP,
        {"linode_domain_get": "GET /domains/{p}"},
        {"DomainGetInput": "linode_domain_get"},
    )

    assert evidence.routes == {"GET /domains/{p}"}
    assert len(evidence.unresolved) == 1
    assert "read_any: unknown message 'message'" in evidence.unresolved[0]


def test_reports_a_typed_lookup_of_a_message_the_contract_never_declared(
    tmp_path: Path,
) -> None:
    """A message no tool declares is named, the way an undeclared tool is."""
    evidence = _scan(
        tmp_path,
        TYPED_LOOKUP,
        {"linode_domain_get": "GET /domains/{p}"},
        {},
    )

    assert evidence.routes == set()
    assert any(
        "read_domain: unknown message 'DomainGetInput'" in entry
        for entry in evidence.unresolved
    )


def test_contract_messages_keys_the_same_contract_by_input_message() -> None:
    """The typed lookup resolves through the message-to-tool half of the contract."""
    messages = gate.contract_messages()

    assert messages["ProfileGetInput"] == "linode_profile_get"
    assert messages["ProfileGrantsGetInput"] == "linode_profile_grants_get"
    assert set(messages.values()) == set(gate.contract_routes())


def test_a_go_typed_lookup_resolves_through_the_contract(tmp_path: Path) -> None:
    """The Go twin: the dump hands over the message and the gate resolves it."""
    dump = tmp_path / "routes.json"
    dump.write_text(
        json.dumps(
            {
                "routes": [],
                "contracted": [
                    {"message": "TagDeleteInput", "site": "validator.go:12 f"},
                    {"message": "TagDelteInput", "site": "validator.go:30 g"},
                ],
                "unresolved": [],
            }
        ),
        encoding="utf-8",
    )

    evidence = gate.go_evidence(
        tmp_path,
        str(dump),
        {"linode_tag_delete": "DELETE /tags/{p}"},
        {"TagDeleteInput": "linode_tag_delete"},
    )

    assert evidence.routes == {"DELETE /tags/{p}"}
    assert evidence.unresolved == ["validator.go:30 g: unknown message 'TagDelteInput'"]
