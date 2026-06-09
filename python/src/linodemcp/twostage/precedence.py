"""Two-stage flag precedence resolver.

Mirrors ``go/internal/twostage/precedence.go``. ``resolve`` is a pure
function: it maps a :class:`Request` to a :class:`Decision` using the spec's
precedence order (yolo, apply, plan, dry_run, single-step confirm, refuse).
The caller performs the I/O (plan lookup, args compare, drift fetch) and
passes a :class:`PlanLookup` classification, so this stays free of side
effects and is exhaustively testable.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from linodemcp.profiles import Capability
from linodemcp.twostage.errors import (
    ERR_BYPASS_FLAGS_CONFLICT,
    ERR_MISSING_CONFIRM,
    ERR_PLAN_ARGS_MISMATCH,
    ERR_PLAN_DRIFT,
    ERR_PLAN_EXPIRED,
    ERR_PLAN_NOT_FOUND,
    ERR_YOLO_NOT_PERMITTED,
)

# MCP ``mode`` parameter values that select the two-stage branches.
MODE_PLAN = "plan"
MODE_APPLY = "apply"


class Branch(StrEnum):
    """The execution path the resolver selects for a call."""

    YOLO = "yolo"
    APPLY = "apply"
    PLAN = "plan"
    DRY_RUN = "dry_run"
    SINGLE_STEP = "single_step"
    REFUSE = "refuse"


class PlanLookup(StrEnum):
    """Caller-computed classification of the plan store state for an apply."""

    NOT_APPLICABLE = "not_applicable"
    VALID = "valid"
    EXPIRED = "expired"
    UNKNOWN = "unknown"
    ARGS_MISMATCH = "args_mismatch"
    DRIFTED = "drifted"


@dataclass(frozen=True)
class Request:
    """The resolved control flags plus the static facts about the tool."""

    capability: Capability
    two_stage_opted_in: bool = False
    profile_allow_yolo: bool = False
    dry_run: bool = False
    mode: str = ""
    plan_id: str = ""
    confirm: bool = False
    bypass_dry_run: bool = False
    confirmed_dry_run: bool = False
    yolo: bool = False
    plan_lookup: PlanLookup = PlanLookup.NOT_APPLICABLE


@dataclass(frozen=True)
class Decision:
    """The resolved branch plus a refusal code (empty unless refused)."""

    branch: Branch
    err_code: str = ""


def resolve(req: Request) -> Decision:
    """Map a request to a decision using the spec's precedence order."""
    # Reads and meta tools never enter the two-stage machine.
    if req.capability in (Capability.Read, Capability.Meta):
        return Decision(Branch.SINGLE_STEP)

    # Yolo dominates every other flag for a mutator.
    if req.yolo:
        if req.profile_allow_yolo:
            return Decision(Branch.YOLO)
        return Decision(Branch.REFUSE, ERR_YOLO_NOT_PERMITTED)

    # Contradictory bypass flags are malformed input on any path.
    if req.bypass_dry_run and req.confirmed_dry_run:
        return Decision(Branch.REFUSE, ERR_BYPASS_FLAGS_CONFLICT)

    return _resolve_intent(req)


def _resolve_intent(req: Request) -> Decision:
    # Plan and apply are available only to opted-in tools.
    if req.two_stage_opted_in:
        if req.mode == MODE_APPLY:
            return _resolve_apply(req)
        if req.mode == MODE_PLAN:
            return Decision(Branch.PLAN)

    # A preview with no plan_id and no state change.
    if req.dry_run:
        return Decision(Branch.DRY_RUN)

    # A one-shot write. CapDestroy still requires a dry-run assertion.
    if req.confirm:
        return _resolve_confirm(req)

    # A mutator called with no execution intent at all.
    return Decision(Branch.REFUSE, ERR_MISSING_CONFIRM)


def _resolve_apply(req: Request) -> Decision:
    if not req.plan_id:
        return Decision(Branch.REFUSE, ERR_PLAN_NOT_FOUND)

    mapping = {
        PlanLookup.VALID: Decision(Branch.APPLY),
        PlanLookup.EXPIRED: Decision(Branch.REFUSE, ERR_PLAN_EXPIRED),
        PlanLookup.ARGS_MISMATCH: Decision(Branch.REFUSE, ERR_PLAN_ARGS_MISMATCH),
        PlanLookup.DRIFTED: Decision(Branch.REFUSE, ERR_PLAN_DRIFT),
    }
    return mapping.get(req.plan_lookup, Decision(Branch.REFUSE, ERR_PLAN_NOT_FOUND))


def _resolve_confirm(req: Request) -> Decision:
    if (
        req.capability == Capability.Destroy
        and not req.confirmed_dry_run
        and not req.bypass_dry_run
    ):
        return Decision(Branch.REFUSE, ERR_MISSING_CONFIRM)
    return Decision(Branch.SINGLE_STEP)
