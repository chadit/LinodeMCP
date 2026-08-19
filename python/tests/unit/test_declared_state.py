"""The state a declared fetch reports, and the reader a dependency walk uses.

The Go twin is ``declared_state_test.go`` in ``go/internal/tools``: the same
projection rule and the same refusal sentence.
"""

from __future__ import annotations

import pytest

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_device_pb2,
    instance_pb2,
    lke_pool_pb2,
    monitor_stream_pb2,
    volume_pb2,
)
from linodemcp.tools.declared_state import (
    DeclaredState,
    declared_state_of,
    project_declared_state,
)


def test_projection_keeps_only_what_the_read_models() -> None:
    """A member the API sent that the read's message does not carry is dropped,
    so the preview stays inside the shape the read tool answers with."""
    state = project_declared_state(
        {"id": 42, "label": "audit log sink", "unknown_member": "dropped"},
        monitor_stream_pb2.MonitorStreamDestination.DESCRIPTOR,
    )

    assert state.fields == {"id": 42, "label": "audit log sink"}


def test_projection_invents_no_member_the_api_left_out() -> None:
    """The serializer would emit every declared member at its zero, which reads
    as a resource carrying values the API never mentioned."""
    state = project_declared_state({"id": 42}, volume_pb2.Volume.DESCRIPTOR)

    assert state.fields == {"id": 42}


def test_projection_reports_the_nulls_the_api_sent() -> None:
    """A key sent as null is the API saying there is none, which a missing key
    cannot say, and a plan hashes the two differently."""
    state = project_declared_state(
        {"id": 42, "linode_id": None}, volume_pb2.Volume.DESCRIPTOR
    )

    assert state.fields == {"id": 42, "linode_id": None}


def test_projection_reaches_a_null_under_a_nested_member() -> None:
    """Restoration used to reach one level, so a null under a sub-message came
    back missing however it was declared."""
    state = project_declared_state(
        {"id": 8, "entity": {"id": 5, "label": "web-01", "parent_entity": None}},
        firewall_device_pb2.FirewallDevice.DESCRIPTOR,
    )

    assert state.fields["entity"] == {
        "id": 5,
        "label": "web-01",
        "parent_entity": None,
    }


def test_projection_reads_each_element_of_a_repeated_member() -> None:
    """A pool's nodes are objects of their own, and a walk reads them the same
    way it reads the resource."""
    state = project_declared_state(
        {
            "id": 7,
            "nodes": [
                {"id": "node-a", "instance_id": 11, "unknown": "dropped"},
                {"id": "node-b", "instance_id": 22},
            ],
        },
        lke_pool_pb2.LKENodePool.DESCRIPTOR,
    )

    assert state.fields["nodes"] == [
        {"id": "node-a", "instance_id": 11},
        {"id": "node-b", "instance_id": 22},
    ]


def test_accessors_read_the_members_a_walk_names() -> None:
    """The three readers are the whole set the walks need."""
    state = DeclaredState(
        {
            "linode_id": 9,
            "linode_label": "web-01",
            "linodes": [{"id": 11}, {"id": 22}],
        }
    )

    assert state.number("linode_id") == 9
    assert state.text("linode_label") == "web-01"
    assert [child.number("id") for child in state.objects("linodes")] == [11, 22]


@pytest.mark.parametrize(
    ("name", "fields"),
    [
        ("absent", {}),
        ("null", {"null": None}),
        ("text", {"text": "9"}),
        ("bool", {"bool": True}),
    ],
)
def test_number_answers_nothing_for_a_member_that_is_not_a_whole_number(
    name: str, fields: dict[str, object]
) -> None:
    """A bool would otherwise read as 1 and name instance 1 as a dependency."""
    assert DeclaredState(fields).number(name) is None


def test_projection_reads_a_member_under_its_camel_case_spelling() -> None:
    """The decoder accepts both spellings, so the projection has to read both or
    it would drop a member the decode kept."""
    state = project_declared_state(
        {"id": 333, "linodeId": 9}, volume_pb2.Volume.DESCRIPTOR
    )

    assert state.fields == {"id": 333, "linode_id": 9}


def test_projection_passes_a_map_member_whole() -> None:
    """A map's keys are the API's rather than the contract's, so descending into
    it with the entry descriptor would empty it."""
    state = project_declared_state(
        {"id": 5, "devices": {"sda": {"disk_id": 1}}},
        instance_pb2.InstanceConfig.DESCRIPTOR,
    )

    assert state.fields["devices"] == {"sda": {"disk_id": 1}}


def test_projection_keeps_a_member_the_api_sent_under_the_wrong_shape() -> None:
    """The API is what decides the shape, so a member arriving as something the
    contract does not model reaches the preview as it came."""
    state = project_declared_state(
        {"id": 8, "entity": 5}, firewall_device_pb2.FirewallDevice.DESCRIPTOR
    )

    assert state.fields == {"id": 8, "entity": 5}


def test_projection_keeps_a_repeated_member_that_is_not_a_list() -> None:
    """Same case for a repeated member, which a caller reads directly."""
    state = project_declared_state(
        {"id": 7, "nodes": 5}, lke_pool_pb2.LKENodePool.DESCRIPTOR
    )

    assert state.fields == {"id": 7, "nodes": 5}


def test_projection_keeps_a_list_element_that_is_not_an_object() -> None:
    """Same case one level down, inside the list."""
    state = project_declared_state(
        {"id": 7, "nodes": [5, {"id": "node-b"}]},
        lke_pool_pb2.LKENodePool.DESCRIPTOR,
    )

    assert state.fields["nodes"] == [5, {"id": "node-b"}]


def test_objects_skips_an_element_that_is_not_an_object() -> None:
    """A walk reading a list of objects gets the objects, and nothing else in
    that list becomes one."""
    state = DeclaredState({"nodes": [5, {"id": "node-b"}]})

    nodes = state.objects("nodes")

    assert [node.text("id") for node in nodes] == ["node-b"]


def test_declared_state_of_refuses_what_no_declared_fetch_produced() -> None:
    """A walk paired with a hand-written fetch fails where the report names it,
    rather than reporting no dependencies and looking like a resource with
    none. Go's DeclaredStateOf refuses with this sentence."""
    with pytest.raises(
        TypeError, match="dependency walk received state that is not a declared fetch"
    ):
        declared_state_of({"id": 42})


def test_declared_state_of_passes_a_declared_state_through() -> None:
    """The reader is the only way a walk reaches its state, so it has to hand
    the real one back untouched."""
    state = DeclaredState({"id": 42})

    assert declared_state_of(state) is state
