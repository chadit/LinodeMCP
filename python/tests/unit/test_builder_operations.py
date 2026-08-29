"""The builder state's own operations, at the surface the generated arm calls.

The handler cases beside these read the sentence a caller sees, which the tool
declares. What they cannot see is the condition the operation raises, and that
is the whole of what the emitted except ladder catches: a method raising the
wrong one answers the wrong sentence with nothing failing. These read it
directly.
"""

from __future__ import annotations

import errno
from typing import TYPE_CHECKING

import pytest

from linodemcp.config import Config, UserProfileConfig
from linodemcp.genlocal import (
    LocalAlreadyExistsError,
    LocalBuiltinProfileError,
    LocalDraftMissingError,
    LocalNotFoundError,
    LocalReadFailedError,
    LocalWriteFailedError,
)
from linodemcp.profiles import Capability, Profile
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.builderstate import BuilderState

if TYPE_CHECKING:
    from pathlib import Path

_DRAFT = "dns-readall"
_SOURCE = "compute-admin"
_MISSING_DRAFT = "no-such-draft"
_BOOT_TOOL = "linode_instance_boot"
_LIST_TOOL = "linode_instance_list"
_CONFIG_PATH_ENV = "LINODEMCP_CONFIG_PATH"
_BUILTIN_NAME = "default"

# The smallest configuration write_atomic round-trips, which the save reads,
# merges into and writes back.
_MINIMAL_YAML = """\
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


def _catalog() -> list[ToolDescriptor]:
    return [
        ToolDescriptor(name=_BOOT_TOOL, capability=Capability.Write),
        ToolDescriptor(name=_LIST_TOOL, capability=Capability.Read),
    ]


def _config() -> Config:
    """A configuration carrying the one profile a clone source can name."""
    cfg = Config()
    cfg.profiles[_SOURCE] = UserProfileConfig(
        description="Compute admin clone source",
        allowed_tools=(_BOOT_TOOL, _LIST_TOOL),
        allowed_environments=("prod",),
        required_token_scopes=("linodes:read_write",),
    )
    return cfg


def _no_profile() -> Profile:
    """The active-profile reader for the operations that never read one."""
    return Profile(name="test", description="", allowed_tools=())


def _state() -> BuilderState:
    return BuilderState(
        drafts=Registry(),
        catalog=_catalog,
        active_profile=_no_profile,
        config=_config(),
    )


def test_draft_create_reports_the_conditions_it_declares() -> None:
    """Both conditions LOCAL_CALL_DRAFT_CREATE declares, at their own values."""
    state = _state()

    with pytest.raises(LocalNotFoundError):
        state.draft_create(_DRAFT, "no-such-profile")

    state.draft_create(_DRAFT, "")

    with pytest.raises(LocalAlreadyExistsError):
        state.draft_create(_DRAFT, "")


def test_draft_create_seeds_from_the_configuration_the_state_carries() -> None:
    """The path the shipped answer capture never reaches: a seed that resolves.

    It is what says the configuration reaches the operation at all. The refusal
    beside it answers the same way for a name nothing carries and for an empty
    configuration, so only a hit tells the two apart.
    """
    answer = _state().draft_create(_DRAFT, _SOURCE)

    assert answer.description == "Compute admin clone source"
    assert answer.allowed_tools == [_BOOT_TOOL, _LIST_TOOL]


@pytest.mark.parametrize(
    ("environments", "scopes", "yolo"),
    [(["prod"], None, None), (None, ["linodes:read_write"], None), (None, None, True)],
)
def test_draft_set_reports_the_condition_it_declares(
    environments: list[str] | None, scopes: list[str] | None, yolo: bool | None
) -> None:
    """Every setting that can report the one condition LOCAL_CALL_DRAFT_SET has."""
    with pytest.raises(LocalDraftMissingError):
        _state().draft_set(_MISSING_DRAFT, environments, scopes, yolo)


def test_draft_set_leaves_a_setting_the_call_did_not_send_alone() -> None:
    """An absent setting is not an instruction, so it reaches no answer."""
    state = _state()
    state.draft_create(_DRAFT, "")

    answer = state.draft_set(_DRAFT, None, None, True)

    assert answer.changes == {"allow_yolo": True}


def test_the_two_sides_of_draft_tools_answer_differently() -> None:
    """The direction proof: each side is its own method now.

    The two differ in more than sign. Adding expands its patterns against the
    live catalog, so a wildcard picks up a tool the draft never named, while
    removing matches the draft's own list. A side serving the other one answers
    the wrong member with the wrong contents.
    """
    state = _state()
    state.draft_create(_DRAFT, "")

    added = state.draft_tools_add(_DRAFT, ["linode_*"])
    assert added.added, (
        "the adding side matched nothing, so the catalog never reached it"
    )

    removed = state.draft_tools_remove(_DRAFT, [added.added[0]])
    assert removed.removed == [added.added[0]]

    # The catalog is what the two sides disagree about: a pattern the draft does
    # not carry matches on the adding side and nothing on the removing one.
    assert state.draft_tools_remove(_DRAFT, [added.added[0]]).removed == []


def test_both_sides_of_draft_tools_report_the_condition_they_declare() -> None:
    """Each side raises the one condition LOCAL_CALL_DRAFT_TOOLS declares."""
    state = _state()

    with pytest.raises(LocalDraftMissingError):
        state.draft_tools_add(_MISSING_DRAFT, [])

    with pytest.raises(LocalDraftMissingError):
        state.draft_tools_remove(_MISSING_DRAFT, [])


@pytest.fixture
def staged_config(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Aim the save at a config file this case owns, and answer its path.

    The operation reads the live path on every call, so the environment
    override is what keeps a real write off the operator's own config.
    """
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL_YAML)
    monkeypatch.setenv(_CONFIG_PATH_ENV, str(path))
    return path


def test_draft_save_reports_the_conditions_it_declares(
    staged_config: Path,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The four conditions LOCAL_CALL_DRAFT_SAVE declares, at their own values.

    The two store conditions carry a cause as well as a condition, which is the
    half a handler case cannot see: the sentence it words names the file, so a
    condition raised bare would read "failed to load config from read failed".
    """
    state = _state()
    state.draft_create(_DRAFT, "")

    with pytest.raises(LocalBuiltinProfileError):
        state.draft_save(_BUILTIN_NAME)

    with pytest.raises(LocalDraftMissingError):
        state.draft_save(_MISSING_DRAFT)

    missing = tmp_path / "absent.yml"
    monkeypatch.setenv(_CONFIG_PATH_ENV, str(missing))

    with pytest.raises(LocalReadFailedError) as read_failure:
        state.draft_save(_DRAFT)

    assert str(missing) in str(read_failure.value)

    monkeypatch.setenv(_CONFIG_PATH_ENV, str(staged_config))

    def refuse_write(*_args: object, **_kwargs: object) -> None:
        raise OSError(errno.EACCES, "permission denied")

    monkeypatch.setattr("linodemcp.tools.builderstate.write_atomic", refuse_write)

    with pytest.raises(LocalWriteFailedError) as write_failure:
        state.draft_save(_DRAFT)

    assert str(staged_config) in str(write_failure.value)


def test_draft_save_answers_what_the_write_changed(staged_config: Path) -> None:
    """Both saves fill the answer: the first files a profile, the second moves it."""
    state = _state()
    state.draft_create(_DRAFT, "")
    state.draft_tools_add(_DRAFT, [_LIST_TOOL])

    first = state.draft_save(_DRAFT)

    assert first.is_new
    assert first.added_tools == [_LIST_TOOL]

    state.draft_tools_remove(_DRAFT, [_LIST_TOOL])
    state.draft_tools_add(_DRAFT, [_BOOT_TOOL])
    state.draft_set(_DRAFT, None, None, True)

    second = state.draft_save(_DRAFT)

    assert not second.is_new
    assert second.added_tools == [_BOOT_TOOL]
    assert second.removed_tools == [_LIST_TOOL]
    assert "allow_yolo" in second.changed_fields

    # The write is the point of the operation, so the answer alone is not proof.
    assert _BOOT_TOOL in staged_config.read_text()
