"""Two-stage writes: plan/apply with drift detection.

Mirrors ``go/internal/twostage``. A destructive call first produces a plan
with a state hash and an id; the user reviews it, then applies by reference.
The server refuses the apply when the underlying resource drifted between
plan and apply.
"""

from __future__ import annotations

from linodemcp.twostage.context import (
    plan_store_from_context,
    reset_plan_store,
    set_plan_store,
)
from linodemcp.twostage.errors import (
    ERR_BYPASS_FLAGS_CONFLICT,
    ERR_MISSING_CONFIRM,
    ERR_PLAN_ARGS_MISMATCH,
    ERR_PLAN_DRIFT,
    ERR_PLAN_EXPIRED,
    ERR_PLAN_NOT_FOUND,
    ERR_YOLO_NOT_PERMITTED,
)
from linodemcp.twostage.ids import PLAN_ID_PREFIX, new_plan_id
from linodemcp.twostage.precedence import (
    MODE_APPLY,
    MODE_PLAN,
    Branch,
    Decision,
    PlanLookup,
    Request,
    resolve,
)
from linodemcp.twostage.registry import (
    DEFAULT_PLAN_TTL,
    Settings,
    opted_in,
    plan_ttl,
)

__all__ = [
    "DEFAULT_PLAN_TTL",
    "ERR_BYPASS_FLAGS_CONFLICT",
    "ERR_MISSING_CONFIRM",
    "ERR_PLAN_ARGS_MISMATCH",
    "ERR_PLAN_DRIFT",
    "ERR_PLAN_EXPIRED",
    "ERR_PLAN_NOT_FOUND",
    "ERR_YOLO_NOT_PERMITTED",
    "MODE_APPLY",
    "MODE_PLAN",
    "PLAN_ID_PREFIX",
    "Branch",
    "Decision",
    "PlanLookup",
    "Request",
    "Settings",
    "new_plan_id",
    "opted_in",
    "plan_store_from_context",
    "plan_ttl",
    "reset_plan_store",
    "resolve",
    "set_plan_store",
]
