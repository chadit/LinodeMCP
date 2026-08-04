"""Offline tests for the route-source ratchet gate.

verify_route_source.py counts the request call sites that still build their
endpoint by hand, per registered language, and holds each count to
docs/contracts/route-source-counts.txt. Counts only ever fall, so the gate
fails a count that grew and a count that fell without the recorded line
following it. Also pinned: a language with no scanner arm or no recorded line,
a line naming no registered language, and a missing primitive declaration that
would otherwise empty a count and read as a finished migration. The last test
measures the real trees, so the committed counts cannot go stale.

Fixture sources use single quotes so they can nest inside these docstrings.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    # @dataclass resolves field annotations through sys.modules[cls.__module__],
    # which is absent for a module loaded straight from a spec.
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_route_source")

# A client whose three primitives all reach each other. Every call in here is
# plumbing, so a scan of this file alone counts nothing.
GO_CLIENT = """
package linode

// makeRequest sends a request built from a method and an endpoint. A doc
// mention of c.makeRequest(ctx) must not count as a call site.
func (c *Client) makeRequest(ctx, method, endpoint, payload any) error {
	return c.makeRequestWithContentType(ctx, method, endpoint, payload)
}

func (c *Client) makeRequestWithContentType(ctx, method, endpoint, body any) error {
	return c.send(ctx, method, endpoint, body)
}

func (c *Client) makeRouteRequest(ctx, tool, payload any, values ...any) error {
	method, endpoint, err := linoderoute.Resolve(tool, values...)
	if err != nil {
		return err
	}

	return c.makeRequest(ctx, method, endpoint, payload)
}

// makeRouteRequestQuery is the second routed primitive. It has to reach
// makeRequest to send anything, which is the hop the routed tuple exists to
// keep out of the count.
func (c *Client) makeRouteRequestQuery(
	ctx, tool, rawQuery, payload any, values ...any,
) error {
	method, endpoint, err := routedRequest(tool, rawQuery, values)
	if err != nil {
		return err
	}

	return c.makeRequest(ctx, method, endpoint, payload)
}

// makeRouteRequestContentType is the third routed primitive. It reaches the
// content-type hand-built one rather than makeRequest, so the tuple has to keep
// that hop out of the count as well.
func (c *Client) makeRouteRequestContentType(
	ctx, tool, contentType, body any, values ...any,
) error {
	method, endpoint, err := routedRequest(tool, "", values)
	if err != nil {
		return err
	}

	return c.makeRequestWithContentType(ctx, method, endpoint, body)
}
"""

# Two hand-built call sites (one of them through the content-type primitive)
# and one that names its tool instead.
GO_METHODS = """
package linode

func (c *Client) getThing(ctx context.Context, id int) error {
	return c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("/things/%d", id), nil)
}

func (c *Client) uploadThing(ctx context.Context, body io.Reader) error {
	// c.makeRequest(ctx, http.MethodPost, "/things", nil) was the old shape.
	return c.makeRequestWithContentType(ctx, http.MethodPost, "/things", body)
}

func (c *Client) deleteThing(ctx context.Context, id int) error {
	return c.makeRouteRequest(ctx, "linode_thing_delete", nil, id)
}

func (c *Client) listThings(ctx context.Context, page int) error {
	return c.makeRouteRequestQuery(ctx, "linode_thing_list", "page=1", nil)
}

func (c *Client) uploadThumbnail(ctx context.Context, id int, body io.Reader) error {
	return c.makeRouteRequestContentType(
		ctx, "linode_thing_thumbnail_update", "image/png", body, id,
	)
}
"""

GO_TEST = """
package linode_test

func TestGetThing(t *testing.T) {
	_ = c.makeRequest(ctx, http.MethodGet, "/things", nil)
}
"""

PY_CLIENT = """
class Client:
    async def make_request(self, method, endpoint, body=None):
        return await self.client.request(method, self.base_url + endpoint, json=body)

    async def make_file_request(self, method, endpoint, blob):
        return await self.client.request(method, endpoint, content=blob)

    async def make_route_request(self, tool, *values, body=None):
        method, endpoint = resolve(tool, *values)
        if body is None:
            return await self.make_request(method, endpoint)
        return await self.make_request(method, endpoint, body)
"""

PY_METHODS = """
class Things(Client):
    async def get_thing(self, thing_id):
        return await self.make_request('GET', f'/things/{thing_id}')

    async def upload_thing(self, thing_id, blob):
        return await self.make_file_request('POST', f'/things/{thing_id}', blob)

    async def get_thumbnail(self, thing_id):
        # self.make_request('GET', ...) was the old shape.
        return await self.client.request('GET', f'/things/{thing_id}/thumbnail')

    async def delete_thing(self, thing_id):
        return await self.make_route_request('linode_thing_delete', thing_id)
"""

# What the two fixture clients measure: the calls in the methods files only.
GO_HANDBUILT = 2
GO_ROUTED = 3
PY_HANDBUILT = 3
MEASURED_COUNTS = f"go {GO_HANDBUILT}\npython {PY_HANDBUILT}\n"

REGISTRY = "# registry\ngo\tgoclient\tdump\npython\tpyclient\tdump\n"


def _write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _fixture_repo(tmp_path: Path, monkeypatch: pytest.MonkeyPatch, counts: str) -> Path:
    """Point the gate at a two-language repo with a registry and a counts file.

    The generated tree, test file, and dot-directory hold the same calls as the
    source beside them, so a scan that stopped skipping them would come back
    with a different number.
    """
    _write(tmp_path / "goclient" / "client.go", GO_CLIENT)
    _write(tmp_path / "goclient" / "methods.go", GO_METHODS)
    _write(tmp_path / "goclient" / "methods_test.go", GO_TEST)
    _write(tmp_path / "goclient" / "genpb" / "generated.go", GO_METHODS)
    _write(tmp_path / "pyclient" / "client.py", PY_CLIENT)
    _write(tmp_path / "pyclient" / "methods.py", PY_METHODS)
    _write(tmp_path / "pyclient" / "tests" / "test_methods.py", PY_METHODS)
    _write(tmp_path / "pyclient" / ".venv" / "vendored.py", PY_METHODS)
    _write(tmp_path / "languages.txt", REGISTRY)

    counts_path = tmp_path / "route-source-counts.txt"
    _write(counts_path, counts)

    monkeypatch.setattr(gate, "_REPO_ROOT", tmp_path)
    monkeypatch.setattr(gate, "_LANGUAGES", tmp_path / "languages.txt")
    monkeypatch.setattr(gate, "_COUNTS", counts_path)

    return counts_path


def _scan(tmp_path: Path, language: str, directory: str) -> Any:
    """Scan one fixture tree with the real client shape for that language."""
    return gate.scan(gate._CLIENTS[language], tmp_path / directory)


def test_call_pattern_matches_the_call_not_the_declaration() -> None:
    pattern = gate.call_pattern(("make_request", "client.request"))

    assert pattern.search("        await self.make_request('GET', endpoint)")
    assert pattern.search("        await self.client.request(method, url)")
    assert pattern.search("    async def make_request(self, method, endpoint):") is None


def test_scan_counts_go_call_sites_and_skips_plumbing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)

    counts = _scan(tmp_path, "go", "goclient")

    # Three primitive bodies reach makeRequest and none of them counts: a routed
    # primitive has to reach the hand-built one to send anything, and counting
    # that hop would leave a remainder no migration could clear.
    assert counts.handbuilt == GO_HANDBUILT
    assert counts.routed == GO_ROUTED
    assert counts.undeclared == ()


def test_scan_counts_python_call_sites_and_skips_plumbing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)

    counts = _scan(tmp_path, "python", "pyclient")

    assert counts.handbuilt == PY_HANDBUILT
    assert counts.routed == 1
    assert counts.undeclared == ()


def test_scan_names_a_primitive_with_no_declaration(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A renamed primitive must not read as a client with nothing left to do."""
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)
    # The declaration rather than the bare name, which is a prefix of the
    # query primitive's and would rename both.
    _write(
        tmp_path / "goclient" / "client.go",
        GO_CLIENT.replace(
            "func (c *Client) makeRouteRequest(",
            "func (c *Client) makeContractedRequest(",
        ),
    )

    counts = _scan(tmp_path, "go", "goclient")

    assert counts.undeclared == ("makeRouteRequest",)


def test_scan_names_a_renamed_query_primitive(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Every routed name is checked, not just the first one in the tuple.

    Miss a rename and the primitive's own hop to makeRequest starts counting as
    a hand-built call site, so the count climbs while the client gets more
    migrated, not less.
    """
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)
    _write(
        tmp_path / "goclient" / "client.go",
        GO_CLIENT.replace("makeRouteRequestQuery", "makeRouteRequestFiltered"),
    )

    counts = _scan(tmp_path, "go", "goclient")

    assert counts.undeclared == ("makeRouteRequestQuery",)


def test_scan_names_a_renamed_content_type_primitive(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The content-type primitive is checked like the other two.

    Its hop lands on makeRequestWithContentType rather than makeRequest, so a
    rename that slipped past would count that hop on the upload routes, where
    the count is small enough to look like noise.
    """
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)
    _write(
        tmp_path / "goclient" / "client.go",
        GO_CLIENT.replace("makeRouteRequestContentType", "makeRouteRequestUpload"),
    )

    counts = _scan(tmp_path, "go", "goclient")

    assert counts.undeclared == ("makeRouteRequestContentType",)


def test_each_language_declares_the_routed_primitives_it_counts() -> None:
    """Pin the routed set per language, so a dropped entry fails by name here.

    Python is a single name on purpose: make_route_request took a query keyword
    rather than growing a second primitive, and route_raw reaches it rather than
    make_request, so it adds no primitive call site to skip.
    """
    assert gate._CLIENTS["go"].routed == (
        "makeRouteRequest",
        "makeRouteRequestQuery",
        "makeRouteRequestContentType",
    )
    assert gate._CLIENTS["python"].routed == ("make_route_request",)


def test_read_counts_skips_comments_and_parses_values(tmp_path: Path) -> None:
    contract = tmp_path / "route-source-counts.txt"
    contract.write_text("# header\n\ngo 416\npython 447\n", encoding="utf-8")

    assert gate.read_counts(contract) == {"go": 416, "python": 447}


def test_read_counts_of_a_missing_file_is_empty(tmp_path: Path) -> None:
    assert gate.read_counts(tmp_path / "absent.txt") == {}


def test_read_counts_rejects_a_line_that_is_not_a_count(tmp_path: Path) -> None:
    contract = tmp_path / "route-source-counts.txt"
    contract.write_text("go many\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="unparsable"):
        gate.read_counts(contract)


def test_registered_languages_rejects_an_unparsable_line(tmp_path: Path) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text("go\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="unparsable"):
        gate.registered_languages(registry)


def test_registered_languages_rejects_an_empty_registry(tmp_path: Path) -> None:
    registry = tmp_path / "languages.txt"
    registry.write_text("# nothing but a comment\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="registers no languages"):
        gate.registered_languages(registry)


def test_unchanged_counts_pass(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)

    assert gate.main([]) == 0
    assert "route-source gate OK" in capsys.readouterr().out


def test_a_new_hand_built_call_site_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, f"go {GO_HANDBUILT - 1}\npython 3\n")

    assert gate.main([]) == 1

    error = capsys.readouterr().err
    assert "have grown" in error
    assert "go: 2 hand-built call site(s), 1 recorded (+1)" in error


def test_a_migrated_call_site_fails_until_the_line_is_lowered(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """Shrinking below the line fails the same way a fixed ratchet entry does."""
    _fixture_repo(tmp_path, monkeypatch, f"go {GO_HANDBUILT + 3}\npython 3\n")

    assert gate.main([]) == 1

    error = capsys.readouterr().err
    assert "below the recorded count" in error
    assert "go: 2 hand-built call site(s), 5 recorded (-3)" in error
    assert "lower the line" in error


def test_registered_language_without_a_scanner_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)
    _write(tmp_path / "languages.txt", REGISTRY + "rust\trustclient\tdump\n")

    assert gate.main([]) == 1

    error = capsys.readouterr().err
    assert "no route-source scanner declared" in error
    assert "rust" in error


def test_registered_language_without_a_recorded_line_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, f"go {GO_HANDBUILT}\n")

    assert gate.main([]) == 1
    assert "python is registered but has no line" in capsys.readouterr().err


def test_recorded_line_without_a_registered_language_fails_by_name(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS + "rust 4\n")

    assert gate.main([]) == 1
    assert "rust has a line" in capsys.readouterr().err


def test_a_renamed_primitive_fails_before_the_ratchet(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """The rename empties the go count, and the gate says so instead of that."""
    _fixture_repo(tmp_path, monkeypatch, MEASURED_COUNTS)
    _write(
        tmp_path / "goclient" / "client.go",
        GO_CLIENT.replace("makeRequestWithContentType", "sendWithContentType"),
    )

    assert gate.main([]) == 1

    error = capsys.readouterr().err
    assert "no declaration in their own tree" in error
    assert "makeRequestWithContentType" in error
    assert "below the recorded count" not in error


def test_update_baseline_writes_the_measured_counts_in_registry_order(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    counts_path = _fixture_repo(tmp_path, monkeypatch, "go 99\npython 99\n")
    _write(
        tmp_path / "languages.txt",
        "python\tpyclient\tdump\ngo\tgoclient\tdump\n",
    )

    assert gate.main(["--update-baseline"]) == 0

    written = counts_path.read_text(encoding="utf-8")
    assert written.startswith("# Route-source counts:")
    assert written.endswith(f"python {PY_HANDBUILT}\ngo {GO_HANDBUILT}\n")


def test_recorded_counts_match_the_real_trees() -> None:
    """The committed counts are what the repo measures right now."""
    languages = gate.registered_languages(gate._LANGUAGES)

    assert gate.missing_arms(languages) == []

    measured = {
        name: gate.scan(gate._CLIENTS[name], workdir) for name, workdir in languages
    }

    assert gate.renamed_primitives(measured) == []
    assert {name: counts.handbuilt for name, counts in measured.items()} == (
        gate.read_counts(gate._COUNTS)
    )
