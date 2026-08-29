"""The state a declared fetch reports, and the reader a dependency walk uses.

Mirrors Go's internal/tools/declared_state.go: the same projection rule, the
same three accessors, and the same refusal sentence when a walk is handed a
state some other fetch produced.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

if TYPE_CHECKING:
    from collections.abc import Mapping, Sequence

# The well-known messages holding the API's own object rather than a modeled
# shape, so a projection passes their subtree whole.
_FREE_FORM_MESSAGES = frozenset(
    {
        "google.protobuf.Struct",
        "google.protobuf.Value",
        "google.protobuf.ListValue",
    }
)

# The sentence both languages report when a walk written for a declared fetch is
# handed some other state.
STATE_NOT_DECLARED = "dependency walk received state that is not a declared fetch"


class DeclaredState:
    """The resource a declared fetch read, as the API reported it.

    It is a class rather than the bare dict so a dependency walk written for a
    declared fetch cannot be handed a hand-written fetch's own answer, which is
    the pairing that used to read as a resource carrying nothing and report an
    empty walk.
    """

    __slots__ = ("fields",)

    def __init__(self, fields: dict[str, Any]) -> None:
        self.fields = fields

    def number(self, name: str) -> int | None:
        """The whole number the named member carries, None when it carries none.

        A bool is refused rather than read as 0 or 1, which is the reading Go's
        json.Number gives it.
        """
        value = self.fields.get(name)
        if isinstance(value, bool) or not isinstance(value, int):
            return None
        return value

    def text(self, name: str) -> str:
        """The string the named member carries, empty when it carries none."""
        value = self.fields.get(name)
        return value if isinstance(value, str) else ""

    def object(self, name: str) -> DeclaredState:
        """The object the named singular member carries, empty when none."""
        value = self.fields.get(name)
        if isinstance(value, DeclaredState):
            return value
        if isinstance(value, dict):
            return DeclaredState(cast("dict[str, Any]", value))
        return DeclaredState({})

    def objects(self, name: str) -> list[DeclaredState]:
        """The objects the named repeated member carries, each read this way."""
        value = self.fields.get(name)
        if not isinstance(value, list):
            return []
        items = list(cast("Sequence[Any]", value))
        # An envelope state's elements arrive already projected, so both the
        # raw dict and the projected shape read as one kind of object.
        objects: list[DeclaredState] = []
        for item in items:
            if isinstance(item, DeclaredState):
                objects.append(item)
            elif isinstance(item, dict):
                objects.append(DeclaredState(cast("dict[str, Any]", item)))
        return objects


def declared_state_of(state: Any) -> DeclaredState:
    """Read the state a declared fetch produced, refusing every other shape.

    A walk paired with a hand-written fetch fails here, where the report names
    it, rather than reporting no dependencies and looking like a resource that
    has none.
    """
    if not isinstance(state, DeclaredState):
        raise TypeError(STATE_NOT_DECLARED)
    return state


def project_declared_state(raw: Mapping[str, Any], descriptor: Any) -> DeclaredState:
    """The state a declared fetch reports: the body the API sent, carrying the
    members the read's message models and no others, so a key the API sent as
    null survives and a key it never sent is not invented.

    descriptor is typed Any for the reason restore_explicit_nulls types it that
    way: the runtime descriptor comes from the C (upb) backend.
    """
    return DeclaredState(_project_fields(raw, descriptor))


def _project_fields(body: Mapping[str, Any], descriptor: Any) -> dict[str, Any]:
    """Keep the members the message models, in declared order."""
    projected: dict[str, Any] = {}
    for field in descriptor.fields:
        value, sent = _state_member(body, field)
        if sent:
            projected[str(field.name)] = _project_member(value, field)
    return projected


def _state_member(body: Mapping[str, Any], field: Any) -> tuple[Any, bool]:
    """Read a member under the contract's spelling or the camel-case one the
    decoder also accepts."""
    if field.name in body:
        return body[field.name], True
    if field.json_name in body:
        return body[field.json_name], True
    return None, False


def _project_member(value: Any, field: Any) -> Any:
    """Keep a value as the API sent it, descending only where the contract
    models the shape underneath."""
    if value is None or field.message_type is None or _passes_whole(field.message_type):
        return value
    if field.is_repeated:
        return _project_list(value, field.message_type)
    if not isinstance(value, dict):
        return value
    return _project_fields(cast("Mapping[str, Any]", value), field.message_type)


def _passes_whole(message_type: Any) -> bool:
    """A map's keys are the API's rather than the contract's, so its subtree
    passes whole the way a free-form one does."""
    if message_type.GetOptions().map_entry:
        return True
    return str(message_type.full_name) in _FREE_FORM_MESSAGES


def _project_list(value: Any, message_type: Any) -> Any:
    """Project each element the API sent, keeping anything that is not an
    object as it arrived."""
    if not isinstance(value, list):
        return value
    items = list(cast("Sequence[Any]", value))
    return [
        _project_fields(cast("Mapping[str, Any]", item), message_type)
        if isinstance(item, dict)
        else item
        for item in items
    ]
