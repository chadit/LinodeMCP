"""Tests for the two-stage precedence resolver, registry, and id generator.

The precedence layer is exhaustively enumerated (all 36,864 Request
combinations) the same way the Go ``precedence_test.go`` does, which is
complete rather than sampled.
"""

from __future__ import annotations

import itertools
from dataclasses import replace
from typing import TYPE_CHECKING, cast

from linodemcp.profiles import Capability
from linodemcp.twostage import (
    DEFAULT_PLAN_TTL,
    ERR_BYPASS_FLAGS_CONFLICT,
    ERR_MISSING_CONFIRM,
    ERR_PLAN_DRIFT,
    ERR_YOLO_NOT_PERMITTED,
    MODE_APPLY,
    MODE_PLAN,
    PLAN_ID_PREFIX,
    Branch,
    PlanLookup,
    Request,
    new_plan_id,
    opted_in,
    plan_ttl,
    resolve,
)

if TYPE_CHECKING:
    from collections.abc import Iterator

_BOOL = (False, True)

# itertools.product erases the per-field types, so cast each combo back to its
# known shape before unpacking.
type _Combo = tuple[
    Capability, bool, bool, bool, str, str, PlanLookup, bool, bool, bool, bool
]


def _all_requests() -> Iterator[Request]:
    combos = itertools.product(
        list(Capability),
        _BOOL,
        _BOOL,
        _BOOL,
        ("", MODE_PLAN, MODE_APPLY, "bogus"),
        ("", "plan_x"),
        list(PlanLookup),
        _BOOL,
        _BOOL,
        _BOOL,
        _BOOL,
    )
    for raw_combo in combos:
        (
            cap,
            opted,
            allow_yolo,
            dry,
            mode,
            pid,
            lookup,
            confirm,
            bypass,
            confirmed,
            yolo,
        ) = cast("_Combo", raw_combo)
        yield Request(
            capability=cap,
            two_stage_opted_in=opted,
            profile_allow_yolo=allow_yolo,
            dry_run=dry,
            mode=mode,
            plan_id=pid,
            plan_lookup=lookup,
            confirm=confirm,
            bypass_dry_run=bypass,
            confirmed_dry_run=confirmed,
            yolo=yolo,
        )


def _enters_machine(capability: Capability) -> bool:
    return capability not in (Capability.Read, Capability.Meta)


def test_resolve_table() -> None:
    cases: list[tuple[Request, Branch, str]] = [
        (Request(capability=Capability.Read), Branch.SINGLE_STEP, ""),
        (
            Request(capability=Capability.Meta, yolo=True, profile_allow_yolo=True),
            Branch.SINGLE_STEP,
            "",
        ),
        (
            Request(capability=Capability.Destroy, yolo=True, profile_allow_yolo=True),
            Branch.YOLO,
            "",
        ),
        (
            Request(capability=Capability.Destroy, yolo=True),
            Branch.REFUSE,
            ERR_YOLO_NOT_PERMITTED,
        ),
        (
            Request(
                capability=Capability.Destroy,
                confirm=True,
                bypass_dry_run=True,
                confirmed_dry_run=True,
            ),
            Branch.REFUSE,
            ERR_BYPASS_FLAGS_CONFLICT,
        ),
        (
            Request(
                capability=Capability.Destroy,
                two_stage_opted_in=True,
                mode=MODE_APPLY,
                plan_id="plan_x",
                plan_lookup=PlanLookup.VALID,
            ),
            Branch.APPLY,
            "",
        ),
        (
            Request(
                capability=Capability.Destroy,
                two_stage_opted_in=True,
                mode=MODE_APPLY,
                plan_id="plan_x",
                plan_lookup=PlanLookup.DRIFTED,
            ),
            Branch.REFUSE,
            ERR_PLAN_DRIFT,
        ),
        (
            Request(
                capability=Capability.Destroy,
                two_stage_opted_in=True,
                mode=MODE_PLAN,
            ),
            Branch.PLAN,
            "",
        ),
        (
            Request(capability=Capability.Write, dry_run=True),
            Branch.DRY_RUN,
            "",
        ),
        (
            Request(capability=Capability.Write, confirm=True),
            Branch.SINGLE_STEP,
            "",
        ),
        (
            Request(capability=Capability.Destroy, confirm=True),
            Branch.REFUSE,
            ERR_MISSING_CONFIRM,
        ),
        (
            Request(
                capability=Capability.Destroy, confirm=True, confirmed_dry_run=True
            ),
            Branch.SINGLE_STEP,
            "",
        ),
        (
            Request(capability=Capability.Destroy),
            Branch.REFUSE,
            ERR_MISSING_CONFIRM,
        ),
    ]
    for req, want_branch, want_err in cases:
        decision = resolve(req)
        assert decision.branch == want_branch, req
        assert decision.err_code == want_err, req


def test_yolo_dominates_for_mutators() -> None:
    for base in _all_requests():
        if not _enters_machine(base.capability):
            continue

        req = replace(base, yolo=True, profile_allow_yolo=True)
        assert resolve(req).branch == Branch.YOLO


def test_refusal_carries_code() -> None:
    for req in _all_requests():
        decision = resolve(req)
        is_refuse = decision.branch == Branch.REFUSE
        has_code = bool(decision.err_code)
        assert is_refuse == has_code, (req, decision)


def test_plan_apply_preconditions() -> None:
    for req in _all_requests():
        branch = resolve(req).branch
        if branch == Branch.APPLY:
            assert req.two_stage_opted_in
            assert req.mode == MODE_APPLY
            assert req.plan_id != ""
            assert req.plan_lookup == PlanLookup.VALID
        elif branch == Branch.PLAN:
            assert req.two_stage_opted_in
            assert req.mode == MODE_PLAN


def test_capability_boundary() -> None:
    for req in _all_requests():
        if _enters_machine(req.capability):
            continue
        assert resolve(req).branch == Branch.SINGLE_STEP


def test_resolve_is_reproducible() -> None:
    for req in _all_requests():
        assert resolve(req) == resolve(req)


def test_opted_in_capability_defaults() -> None:
    assert opted_in("linode_x", Capability.Destroy) is True
    # Admin opts out by default: no admin tool is wired for two-stage, so
    # claiming it would advertise a flow it cannot run.
    assert opted_in("linode_x", Capability.Admin) is False
    assert opted_in("linode_x", Capability.Write) is False
    assert opted_in("linode_x", Capability.Read) is False
    assert opted_in("linode_x", Capability.Meta) is False
    assert opted_in("linode_x", Capability.Unknown) is False


def test_plan_ttl_default() -> None:
    assert plan_ttl("linode_unknown") == DEFAULT_PLAN_TTL


def test_new_plan_id_prefix_and_uniqueness() -> None:
    seen: set[str] = set()
    for _ in range(1000):
        plan_id = new_plan_id()
        assert plan_id.startswith(PLAN_ID_PREFIX)
        assert plan_id not in seen
        seen.add(plan_id)
