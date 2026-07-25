"""Durable source evidence for Go routes assembled at runtime."""

import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]


def test_go_instance_interface_list_dynamic_route_evidence() -> None:
    """The route contract and Go package retain the dynamic interface path."""
    routes = (REPO_ROOT / "docs" / "contracts" / "tool-routes.txt").read_text(
        encoding="utf-8"
    )
    go_source = "\n".join(
        path.read_text(encoding="utf-8")
        for path in sorted((REPO_ROOT / "go" / "internal" / "linode").rglob("*.go"))
        if not path.name.endswith("_test.go")
    )

    route = "linode_instance_interface_list: GET /linode/instances/{p}/interfaces"
    assert route in routes

    function_name = "httpListInstanceInterfacesProto"
    declaration = re.search(
        rf"(?m)^func\b[^\n]*\b{function_name}\b[^\n]*$",
        go_source,
    )
    assert declaration is not None
    function_tail = go_source[declaration.end() :]
    function_source, _, _ = function_tail.partition("\nfunc ")
    assert '"/%s/interfaces"' in function_source
