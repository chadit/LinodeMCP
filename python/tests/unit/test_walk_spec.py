"""The declared walk engine, held to the same behaviors Go's table pins.

The expected sentences here are byte-identical to walk_spec_test.go's, which
is what keeps the two interpreters wording one blast radius.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock

from linodemcp.tools.declared_state import DeclaredState
from linodemcp.tools.walk_spec import (
    BillingSpec,
    WalkEmit,
    WalkEnrich,
    WalkFilter,
    WalkSpec,
    WalkWarning,
    run_billing_delta,
    run_dependency_walks,
)

if TYPE_CHECKING:
    from linodemcp.tools.helpers import DryRunDetails


async def _run(
    specs: list[WalkSpec],
    state: DeclaredState,
    arguments: dict[str, Any] | None = None,
) -> DryRunDetails:
    details: DryRunDetails = await run_dependency_walks(
        AsyncMock(), specs, state, arguments or {}
    )
    return details


async def test_emits_state_members_with_warnings() -> None:
    state = DeclaredState(
        {
            "label": "pg-app",
            "members": [{"linode_id": 123}, {"linode_id": 456}],
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="members",
                emit=WalkEmit(
                    kind="instance",
                    id_field="linode_id",
                    action="detached",
                    note=(
                        "Linode is removed from the placement group; "
                        "the instance is not deleted."
                    ),
                ),
                warnings=(
                    WalkWarning(
                        template=(
                            "Deleting this placement group detaches {count} "
                            "Linode(s); the instances are not deleted."
                        ),
                        when_any=True,
                    ),
                ),
            )
        ],
        state,
    )

    assert [dep["id"] for dep in details.get("dependencies", [])] == [123, 456]
    assert details.get("warnings") == [
        (
            "Deleting this placement group detaches 2 Linode(s); "
            "the instances are not deleted."
        )
    ]


async def test_says_nothing_over_an_empty_member() -> None:
    details = await _run(
        [
            WalkSpec(
                state_member="members",
                emit=WalkEmit(kind="instance", action="detached"),
                warnings=(WalkWarning(template="detaches {count}", when_any=True),),
            )
        ],
        DeclaredState({"members": []}),
    )

    assert details == {}


async def test_scalar_emits_on_positive_alone() -> None:
    spec = WalkSpec(
        state_scalar="linode_id",
        emit=WalkEmit(
            kind="instance",
            id_field="linode_id",
            action="detached",
            note="Volume is attached; it detaches from this instance before deletion.",
        ),
    )

    attached = await _run([spec], DeclaredState({"linode_id": 123}))
    assert len(attached.get("dependencies", [])) == 1

    detached = await _run([spec], DeclaredState({}))
    assert detached == {}


async def test_filters_folded_and_counts_the_total() -> None:
    state = DeclaredState(
        {
            "records": [
                {"type": "ns", "name": "a"},
                {"type": "NS", "name": "b"},
                {"type": "A", "name": "c"},
            ]
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="records",
                filter=WalkFilter(field="type", value="NS", fold=True),
                emit=WalkEmit(
                    kind="domain_record",
                    action="cascade_deleted",
                    note="NS record for {name}",
                ),
                warnings=(
                    WalkWarning(
                        template=(
                            "Deleting this domain destroys {total} DNS record(s), "
                            "including {count} NS record(s)."
                        ),
                        when_any=True,
                    ),
                ),
            )
        ],
        state,
    )

    assert len(details.get("dependencies", [])) == 2
    assert details.get("warnings") == [
        "Deleting this domain destroys 3 DNS record(s), including 2 NS record(s)."
    ]


async def test_truncation_and_conditional_warnings() -> None:
    state = DeclaredState(
        {
            "status": "Running",
            "data": [{"type": "linode"}],
            "results": 9,
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="data",
                emit=WalkEmit(
                    kind_field="type",
                    kind_fallback="resource",
                    action="removed",
                    note="Loses this tag; the resource itself is not deleted.",
                ),
                warnings=(
                    WalkWarning(
                        template="removes it from {max_results} tagged object(s)",
                        when_any=True,
                    ),
                    WalkWarning(
                        template=(
                            "Only the first {count} tagged object(s) are itemized."
                        ),
                        when_truncated=True,
                    ),
                    WalkWarning(
                        template="Instance is currently running.",
                        when_field="status",
                        when_equals="running",
                    ),
                    WalkWarning(
                        template="never said",
                        when_field="status",
                        when_equals="offline",
                    ),
                ),
            )
        ],
        state,
    )

    assert details.get("warnings") == [
        "removes it from 9 tagged object(s)",
        "Only the first 1 tagged object(s) are itemized.",
        "Instance is currently running.",
    ]


async def test_drops_a_note_no_placeholder_fills() -> None:
    details = await _run(
        [
            WalkSpec(
                state_member="disks",
                emit=WalkEmit(side_effects=True, note='Disk "{label}" is erased.'),
            )
        ],
        DeclaredState({"disks": [{"size": 100}]}),
    )

    assert details == {}


async def test_route_walk_words_its_failure() -> None:
    client = AsyncMock()
    client.route_raw.side_effect = RuntimeError("boom")

    details = await run_dependency_walks(
        client,
        [
            WalkSpec(
                list_tool="linode_firewall_device_list",
                values=(123,),
                emit=WalkEmit(kind="device", action="removed"),
                list_error_warning="Could not list firewall devices: {error}",
            )
        ],
        DeclaredState({}),
        {},
    )

    assert details.get("warnings") == ["Could not list firewall devices: boom"]
    assert "dependencies" not in details


async def test_guards_on_named_aggregates() -> None:
    state = DeclaredState(
        {
            "count": 2,
            "pools": [
                {"count": 3, "linodes": [{"id": 1}]},
                {"count": 0, "linodes": []},
            ],
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="pools",
                emit=WalkEmit(kind="pool", action="cascade_deleted"),
                warnings=(
                    WalkWarning(
                        template="destroys {count} pool(s) and {sum:count} node(s)",
                        when_positive="sum:count",
                    ),
                    WalkWarning(
                        template="{sum:len:linodes} interface(s) detach",
                        when_positive="sum:len:linodes",
                    ),
                    WalkWarning(
                        template="never: state count",
                        when_positive="state:absent",
                    ),
                ),
            )
        ],
        state,
    )

    assert details.get("warnings") == [
        "destroys 2 pool(s) and 3 node(s)",
        "1 interface(s) detach",
    ]


async def test_presence_pair_selects_one_wording() -> None:
    def spec() -> WalkSpec:
        return WalkSpec(
            state_member="disks",
            emit=WalkEmit(side_effects=True, note="Disk {label} is erased."),
            warnings=(
                WalkWarning(
                    template='Rebuild replaces the current image "{state:image}".',
                    when_present="image",
                ),
                WalkWarning(
                    template="Rebuild destroys all data on the instance.",
                    when_absent="image",
                ),
            ),
        )

    imaged = await _run(
        [spec()], DeclaredState({"image": "linode/ubuntu24.04", "disks": []})
    )
    assert imaged.get("warnings") == [
        'Rebuild replaces the current image "linode/ubuntu24.04".'
    ]

    bare = await _run([spec()], DeclaredState({"disks": []}))
    assert bare.get("warnings") == ["Rebuild destroys all data on the instance."]


async def test_reads_dotted_kind_and_label() -> None:
    state = DeclaredState(
        {
            "devices": [
                {"entity": {"type": "linode", "id": 5, "label": "web-01"}},
            ]
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="devices",
                emit=WalkEmit(
                    kind_field="entity.type",
                    kind_fallback="resource",
                    id_field="entity.id",
                    label_field="entity.label",
                    action="removed",
                    note="Loses this firewall.",
                ),
            )
        ],
        state,
    )

    dependencies = details.get("dependencies", [])
    assert len(dependencies) == 1
    dependency = dependencies[0]
    assert dependency["kind"] == "linode"
    assert dependency["label"] == "web-01"
    assert dependency["id"] == 5


async def test_note_reads_the_elements_own_count() -> None:
    """The declared collision rule: inside a note the element's own count
    wins, inside a warning the emitted aggregate does, and a warning with no
    predicate always speaks.
    """
    details = await _run(
        [
            WalkSpec(
                state_member="pools",
                emit=WalkEmit(
                    kind="pool",
                    action="cascade_deleted",
                    note="{count} node(s) ride along.",
                ),
                warnings=(
                    WalkWarning(template="destroys {count} pool(s)", when_any=True),
                    WalkWarning(template="the walk always says this one"),
                ),
            )
        ],
        DeclaredState({"pools": [{"count": 3}]}),
    )

    dependencies = details.get("dependencies", [])
    assert [dep["note"] for dep in dependencies] == ["3 node(s) ride along."]
    assert details.get("warnings") == [
        "destroys 1 pool(s)",
        "the walk always says this one",
    ]


async def test_filters_by_argument_through_open_objects() -> None:
    state = DeclaredState(
        {
            "data": [
                {"id": 1, "devices": {"sda": {"disk_id": 77}}},
                {"id": 2, "devices": {"sdb": {"disk_id": 88}}},
                {"id": 3, "devices": {"note": "raw"}},
            ]
        }
    )

    details = await _run(
        [
            WalkSpec(
                state_member="data",
                filter=WalkFilter(field="devices.*.disk_id", argument="disk_id"),
                emit=WalkEmit(
                    kind="config",
                    id_field="id",
                    action="removed",
                    note="references disk {arg:disk_id}",
                ),
            )
        ],
        state,
        {"disk_id": 77},
    )

    dependencies = details.get("dependencies", [])
    assert len(dependencies) == 1
    assert dependencies[0]["note"] == "references disk 77"


async def test_folds_a_dotted_filter_comparison() -> None:
    details = await _run(
        [
            WalkSpec(
                state_member="data",
                filter=WalkFilter(field="entity.type", value="linode", fold=True),
                emit=WalkEmit(kind="instance", action="detached"),
            )
        ],
        DeclaredState(
            {
                "data": [
                    {"entity": {"type": "LINODE"}},
                    {"entity": {"type": "volume"}},
                ]
            }
        ),
    )

    assert len(details.get("dependencies", [])) == 1


async def test_says_a_side_effect_line_that_fills() -> None:
    details = await _run(
        [
            WalkSpec(
                state_member="disks",
                emit=WalkEmit(
                    side_effects=True,
                    note='Disk "{label}" (encrypted {encrypted}) is erased.',
                ),
            )
        ],
        DeclaredState(
            {
                "disks": [
                    {"label": "boot", "encrypted": True},
                    {"label": "scratch", "encrypted": False},
                ]
            }
        ),
    )

    assert details.get("side_effects") == [
        'Disk "boot" (encrypted true) is erased.',
        'Disk "scratch" (encrypted false) is erased.',
    ]


async def test_falls_back_for_an_untyped_element_and_an_unsummed_note() -> None:
    """Sums are warning vocabulary, so a note naming one reads empty rather
    than reading the element.
    """
    details = await _run(
        [
            WalkSpec(
                state_member="data",
                emit=WalkEmit(
                    kind_field="type",
                    kind_fallback="resource",
                    action="removed",
                    note="loses {sum:count}",
                ),
            )
        ],
        DeclaredState({"data": [{}]}),
    )

    dependencies = details.get("dependencies", [])
    assert len(dependencies) == 1
    assert dependencies[0]["kind"] == "resource"
    assert dependencies[0]["note"] == "loses "


async def test_enriches_templates_from_a_read() -> None:
    def spec() -> WalkSpec:
        return WalkSpec(
            state_scalar="linode_id",
            enrich=WalkEnrich(
                tool="linode_instance_get", fields=("label",), values=(5,)
            ),
            emit=WalkEmit(
                kind="instance",
                id_field="linode_id",
                action="detached",
                note="attached to {enrich:label}",
            ),
        )

    state = DeclaredState({"linode_id": 5})

    client = AsyncMock()
    client.route_raw.return_value = {"label": "web-01"}
    details = await run_dependency_walks(client, [spec()], state, {})
    dependencies = details.get("dependencies", [])
    assert [dep["note"] for dep in dependencies] == ["attached to web-01"]

    broken = AsyncMock()
    broken.route_raw.side_effect = RuntimeError("boom")
    bare = await run_dependency_walks(broken, [spec()], state, {})
    dependencies = bare.get("dependencies", [])
    assert [dep["note"] for dep in dependencies] == ["attached to "]


async def test_counts_a_nested_list_in_a_note() -> None:
    details = await _run(
        [
            WalkSpec(
                state_member="linodes",
                emit=WalkEmit(
                    kind="instance",
                    id_field="id",
                    action="detached",
                    note="{len:interfaces} interface(s) in this subnet are detached.",
                ),
            )
        ],
        DeclaredState({"linodes": [{"id": 9, "interfaces": [{}, {}]}]}),
    )

    dependencies = details.get("dependencies", [])
    assert len(dependencies) == 1
    assert dependencies[0]["note"] == ("2 interface(s) in this subnet are detached.")


async def test_falls_back_to_the_enriched_label() -> None:
    client = AsyncMock()
    client.route_raw.return_value = {"label": "web-01"}

    details = await run_dependency_walks(
        client,
        [
            WalkSpec(
                state_scalar="linode_id",
                enrich=WalkEnrich(
                    tool="linode_instance_get", fields=("label",), values=(5,)
                ),
                emit=WalkEmit(
                    kind="instance",
                    id_field="linode_id",
                    label_field="linode_label",
                    label_fallback="{enrich:label}",
                    action="detached",
                ),
            )
        ],
        DeclaredState({"linode_id": 5}),
        {},
    )

    dependencies = details.get("dependencies", [])
    assert [dep.get("label") for dep in dependencies] == ["web-01"]


async def test_skips_the_enrichment_over_nothing_kept() -> None:
    client = AsyncMock()

    details = await run_dependency_walks(
        client,
        [
            WalkSpec(
                state_member="members",
                enrich=WalkEnrich(
                    tool="linode_instance_get", fields=("label",), values=(5,)
                ),
                emit=WalkEmit(kind="instance", action="detached"),
                warnings=(WalkWarning(template="in {enrich:label}", when_any=True),),
            )
        ],
        DeclaredState({"members": []}),
        {},
    )

    assert details == {}
    client.route_raw.assert_not_awaited()


async def test_walks_a_nested_list_member() -> None:
    client = AsyncMock()
    client.route_raw.return_value = {
        "ipv4": {
            "public": [{"address": "203.0.113.10"}],
            "reserved": [{"address": "203.0.113.99"}],
        }
    }

    spec = WalkSpec(
        list_tool="linode_instance_ip_list",
        list_member="ipv4.public",
        values=(123,),
        emit=WalkEmit(kind="public_ip", label_field="address", action="released"),
        list_error_warning="Could not list IP addresses: {error}",
    )

    details = await run_dependency_walks(client, [spec], DeclaredState({}), {})
    dependencies = details.get("dependencies", [])
    assert [dep.get("label") for dep in dependencies] == ["203.0.113.10"]

    broken = AsyncMock()
    broken.route_raw.side_effect = RuntimeError("boom")
    bare = await run_dependency_walks(broken, [spec], DeclaredState({}), {})
    assert bare.get("warnings") == ["Could not list IP addresses: boom"]
    assert "dependencies" not in bare


async def test_billing_words_all_three_branches() -> None:
    spec = BillingSpec(
        price_tool="linode_type_get",
        type_field="type",
        amount="-{monthly:.2f}",
        sentence="Instance billing stops. Attached volume billing continues.",
        unknown_sentence="Could not fetch type pricing for the estimate.",
    )

    client = AsyncMock()
    client.route_raw.return_value = {"id": "g6-standard-2", "price": {"monthly": 24.0}}
    priced: DryRunDetails = {}
    await run_billing_delta(
        client, spec, DeclaredState({"type": "g6-standard-2"}), priced
    )
    assert priced.get("billing_delta") == {
        "monthly_change_usd": "-24.00",
        "note": "Instance billing stops. Attached volume billing continues.",
    }

    broken = AsyncMock()
    broken.route_raw.side_effect = RuntimeError("boom")
    unknown: DryRunDetails = {}
    await run_billing_delta(
        broken, spec, DeclaredState({"type": "g6-standard-2"}), unknown
    )
    assert unknown.get("billing_delta") == {
        "monthly_change_usd": "unknown",
        "note": "Could not fetch type pricing for the estimate.",
    }

    absent: DryRunDetails = {}
    await run_billing_delta(AsyncMock(), spec, DeclaredState({}), absent)
    assert absent.get("billing_delta") == {"monthly_change_usd": "unknown", "note": ""}

    priceless = AsyncMock()
    priceless.route_raw.return_value = {"id": "g6-standard-2", "price": {}}
    unpriced: DryRunDetails = {}
    await run_billing_delta(
        priceless, spec, DeclaredState({"type": "g6-standard-2"}), unpriced
    )
    assert unpriced.get("billing_delta") == {
        "monthly_change_usd": "unknown",
        "note": "Could not fetch type pricing for the estimate.",
    }
