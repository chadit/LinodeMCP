"""Two-stage error codes.

Mirrors ``go/internal/twostage/errors.go``. These strings are the stable
``error`` field a refused apply returns, plus the refusal codes the
precedence resolver emits.
"""

from __future__ import annotations

# Apply-path error codes.
ERR_PLAN_NOT_FOUND = "PLAN_NOT_FOUND"
ERR_PLAN_EXPIRED = "PLAN_EXPIRED"
ERR_PLAN_DRIFT = "PLAN_DRIFT_DETECTED"
ERR_PLAN_ARGS_MISMATCH = "PLAN_ARGS_MISMATCH"

# Precedence-helper refusal codes.
ERR_MISSING_CONFIRM = "MISSING_CONFIRM"
ERR_BYPASS_FLAGS_CONFLICT = "BYPASS_FLAGS_CONFLICT"
ERR_YOLO_NOT_PERMITTED = "YOLO_NOT_PERMITTED"
