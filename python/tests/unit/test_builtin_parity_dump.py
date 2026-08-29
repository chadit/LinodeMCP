"""Tests for the Python half of the profile-resolution dump.

The dump is what scripts/verify_profile_resolution.py diffs against Go's, so a
dump that quietly drops a tool or mistypes a tier would make the gate agree
about less than it claims to compare.
"""

from __future__ import annotations

import json

import pytest

from linodemcp.builtin_parity_dump import dump, main, read_catalog
from linodemcp.profiles.capability import Capability

_FIXTURE = json.dumps(
    [
        {"name": "hello", "capability": "Meta"},
        {"name": "linode_volume_create", "capability": "Write"},
        {"name": "linode_longview_client_get", "capability": "Read"},
    ]
)


def test_read_catalog_maps_every_tier_name_to_its_capability() -> None:
    catalog = read_catalog(_FIXTURE)

    assert [tool.capability for tool in catalog] == [
        Capability.Meta,
        Capability.Write,
        Capability.Read,
    ]


def test_read_catalog_refuses_a_tier_it_cannot_map() -> None:
    with pytest.raises(SystemExit, match="Superuser"):
        read_catalog(json.dumps([{"name": "x", "capability": "Superuser"}]))


def test_dump_names_every_catalog_tool_in_the_category_half() -> None:
    payload = json.loads(dump(read_catalog(_FIXTURE)))

    assert set(payload["categories"]) == {
        "hello",
        "linode_volume_create",
        "linode_longview_client_get",
    }
    assert payload["categories"]["linode_volume_create"] == ["block_storage"]
    assert payload["categories"]["linode_longview_client_get"] == ["longview"]


def test_dump_resolves_every_built_in_profile() -> None:
    payload = json.loads(dump(read_catalog(_FIXTURE)))

    assert set(payload["profiles"]) == {
        "default",
        "readonly-full",
        "compute-admin",
        "network-admin",
        "kubernetes-admin",
        "storage-admin",
        "iam-admin",
        "full-access",
        "emergency",
    }
    assert (
        "linode_volume_create" in payload["profiles"]["storage-admin"]["allowed_tools"]
    )
    assert "linode_volume_create" not in payload["profiles"]["default"]["allowed_tools"]


def test_main_writes_the_dump_to_stdout(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr("sys.stdin", _StubStdin(_FIXTURE))

    assert main() == 0
    assert json.loads(capsys.readouterr().out)["categories"]["hello"] == ["core"]


class _StubStdin:
    """Minimal stdin stand-in: the dump reads it once, whole."""

    def __init__(self, payload: str) -> None:
        self._payload = payload

    def read(self) -> str:
        return self._payload
