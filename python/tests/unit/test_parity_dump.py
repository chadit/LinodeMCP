"""In-process tests for the Python parity dumper's record contract.

The tool-parity gate runs the dumper as a subprocess, which coverage cannot
see, so these tests import it directly: they pin the record fields the
cross-language comparison relies on, including the scopes surface.
"""

from __future__ import annotations

from linodemcp.parity_dump import dump_records


def test_dump_records_carry_contract_fields_including_scopes() -> None:
    records = dump_records()

    assert len(records) >= 400
    by_name = {str(record["name"]): record for record in records}

    listing = by_name["linode_networking_reserved_ip_list"]
    assert listing["capability"] == "Read"
    assert listing["scopes"] == ["ips:read_only"]
    assert "page" in listing["params"]

    # Rebuild documents linodes:read_write alone; image access is
    # grant-enforced by the API at request time.
    rebuild = by_name["linode_instance_rebuild"]
    assert rebuild["scopes"] == ["linodes:read_write"]

    # Attach carries the documented dual scope, proving multi-scope
    # tools survive the dump's sort.
    attach = by_name["linode_volume_attach"]
    assert attach["scopes"] == ["linodes:read_write", "volumes:read_write"]

    assert by_name["version"]["scopes"] == []


def test_dump_records_scopes_are_sorted_for_stable_comparison() -> None:
    records = dump_records()

    assert all(record["scopes"] == sorted(record["scopes"]) for record in records)
