"""Phase 1 invariant for the Profiles capability tagging plumbing.

Read tools must not declare a ``confirm`` parameter; write/destroy/admin
tools must declare it. ``Meta`` and ``Unknown`` are intentionally exempt
(per the spec). During the Phase 1 rollout most tools are ``Unknown`` so
this check is effectively vacuous for them; it ratchets up as category PRs
assign real capability tags. The Phase 1 cleanup PR adds a separate
"no Unknown remaining" assertion once every tool has been tagged.
"""

from __future__ import annotations

from typing import Any, cast

from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry


def _schema_has_boolean_prop(schema: dict[str, Any], name: str) -> bool:
    """Return True if ``schema`` declares ``name`` as a boolean property."""
    properties_value = schema.get("properties")
    if not isinstance(properties_value, dict):
        return False
    # ``properties_value`` is narrowed to ``dict[Unknown, Unknown]`` by
    # isinstance; cast to ``dict[str, object]`` so pyright knows what we
    # expect each value to be (we then re-narrow with isinstance below).
    properties = cast("dict[str, object]", properties_value)
    prop = properties.get(name)
    if not isinstance(prop, dict):
        return False
    prop_typed = cast("dict[str, object]", prop)
    return prop_typed.get("type") == "boolean"


def _schema_requires(schema: dict[str, Any], name: str) -> bool:
    """Return True iff ``name`` is declared as a boolean property AND listed
    in ``required[]``. A mutator that lists confirm in properties but omits
    it from required can be invoked without confirm at runtime, so the safety
    gate is not actually enforced. This check rejects that case.
    """
    if not _schema_has_boolean_prop(schema, name):
        return False
    required_value = schema.get("required")
    if not isinstance(required_value, list):
        return False
    return name in cast("list[object]", required_value)


def test_capability_and_confirm_invariants() -> None:
    """Confirm parameter matches the tool's declared capability.

    - ``Read`` tools must not declare ``confirm`` (they don't mutate state).
    - ``Write``, ``Destroy``, ``Admin`` tools must require ``confirm``
      (declared as a boolean property AND listed in ``required[]``). Just
      declaring it in properties is not enough: the safety gate has to be
      enforceable, which means the client must be required to send it.
    - ``Meta`` and ``Unknown`` are exempt (either shape is permitted).
    """
    registry = get_tool_registry()
    mutators = {Capability.Write, Capability.Destroy, Capability.Admin}

    read_violations: list[str] = [
        entry.name
        for entry in registry
        if entry.capability == Capability.Read
        and _schema_has_boolean_prop(entry.tool.inputSchema, "confirm")
    ]
    mutator_violations: list[str] = [
        entry.name
        for entry in registry
        if entry.capability in mutators
        and not _schema_requires(entry.tool.inputSchema, "confirm")
    ]

    assert not read_violations, "Read tools must not declare confirm: " + ", ".join(
        sorted(read_violations)
    )
    assert not mutator_violations, (
        "Write/Destroy/Admin tools must require confirm in required[]: "
        + ", ".join(sorted(mutator_violations))
    )
