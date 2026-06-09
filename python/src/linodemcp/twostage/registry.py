"""Two-stage opt-in registry and per-tool TTLs.

Mirrors ``go/internal/twostage/registry.go``. Only CapDestroy tools default
into the two-stage flow, because every CapDestroy tool that reaches the flow
routes through the shared destroy path and so can honor plan/apply. Other
capabilities (a CapWrite tool like instance_resize, or any CapAdmin tool) opt
in only via an explicit per-tool config entry, so none claims a flow it cannot
run.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import timedelta

from linodemcp.profiles import Capability

# How long a plan stays valid before it expires.
DEFAULT_PLAN_TTL = timedelta(minutes=5)

_ZERO = timedelta(0)


@dataclass(frozen=True)
class Settings:
    """Operator-tunable two-stage parameters resolved from config.

    The default instance behaves exactly like the built-in defaults
    (DEFAULT_PLAN_TTL and the capability-default opt-in), so a caller without
    config can use ``Settings()``. Mirrors ``twostage.Settings`` in Go.
    """

    default_ttl: timedelta | None = None
    tool_ttl: dict[str, timedelta] = field(default_factory=dict[str, timedelta])
    opt_in: dict[str, bool] = field(default_factory=dict[str, bool])

    def opted_in(self, tool: str, capability: Capability) -> bool:
        """Report whether a tool participates in the two-stage flow.

        An explicit ``opt_in`` entry wins; otherwise the capability default
        applies: only Destroy opts in. CapAdmin tools do not route through the
        two-stage flow, so opting them in by default would advertise a flow
        they cannot run; they (and a CapWrite tool like instance_resize) opt in
        only through an explicit entry.
        """
        override = self.opt_in.get(tool)
        if override is not None:
            return override
        return capability is Capability.Destroy

    def plan_ttl(self, tool: str) -> timedelta:
        """Return the plan lifetime for a tool.

        A per-tool override wins, then ``default_ttl``, then the built-in
        DEFAULT_PLAN_TTL. Non-positive overrides fall back to the next level.
        """
        tool_override = self.tool_ttl.get(tool)
        if tool_override is not None and tool_override > _ZERO:
            return tool_override
        if self.default_ttl is not None and self.default_ttl > _ZERO:
            return self.default_ttl
        return DEFAULT_PLAN_TTL


def opted_in(tool: str, capability: Capability) -> bool:
    """Report whether a tool participates in the flow under the built-in defaults."""
    return Settings().opted_in(tool, capability)


def plan_ttl(tool: str) -> timedelta:
    """Return the built-in plan lifetime for a tool, ignoring config overrides."""
    return Settings().plan_ttl(tool)
