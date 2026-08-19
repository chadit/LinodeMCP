"""Offline tests for the API surface census gate.

The gate holds three readings of "which surface does this tool answer on" to
one answer: the checked-in census, the proto contract, and the description each
language advertises. These cover each way they can disagree, in both
directions, plus the reading of the census file itself.

The contamination case is the one worth naming: a marker on a tool the census
does not list would tell a caller, or a model choosing from a tool listing,
that a v4 tool reaches another API surface.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

if TYPE_CHECKING:
    from types import ModuleType

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPTS_DIR = REPO_ROOT / "scripts"

_BETA = "v4beta"
_LOCK_LIST = "linode_lock_list"
_TAG_LIST = "linode_tag_list"


def _load_script(name: str) -> ModuleType:
    if str(SCRIPTS_DIR) not in sys.path:
        sys.path.insert(0, str(SCRIPTS_DIR))
    spec = importlib.util.spec_from_file_location(name, SCRIPTS_DIR / f"{name}.py")
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gate = _load_script("verify_api_surfaces")


def test_a_census_matching_the_contract_reports_nothing() -> None:
    """The agreeing case: one annotated tool, listed once, on one surface."""
    assert gate.census_problems({_LOCK_LIST: _BETA}, {_LOCK_LIST: _BETA}) == []


def test_a_censused_tool_the_contract_does_not_declare_fails() -> None:
    """A stale census line would claim a surface no tool answers on."""
    problems = gate.census_problems({_LOCK_LIST: _BETA}, {})

    assert len(problems) == 1
    assert "the contract does not declare" in problems[0]


def test_an_annotated_tool_the_census_does_not_list_fails() -> None:
    """Annotating without listing is how the beta set would drift unreviewed."""
    problems = gate.census_problems({}, {_LOCK_LIST: _BETA})

    assert len(problems) == 1
    assert "the census does not list" in problems[0]


def test_a_surface_the_two_disagree_on_fails() -> None:
    """Same tool, two answers, so neither can be trusted."""
    problems = gate.census_problems({_LOCK_LIST: "v4alpha"}, {_LOCK_LIST: _BETA})

    assert len(problems) == 1
    assert "the census says v4alpha" in problems[0]


def test_the_census_may_not_list_the_default_surface() -> None:
    """An unannotated tool already answers on v4, so listing it says nothing."""
    problems = gate.census_problems({_TAG_LIST: "v4"}, {})

    assert any("lists the default surface" in entry for entry in problems)


def test_a_censused_tool_with_no_marker_fails() -> None:
    """A beta tool whose description does not say so is invisible in discovery.

    This is the half that matters to a model reading a tool listing: without the
    marker it has no way to tell the beta tool from its v4 siblings.
    """
    problems = gate.marker_problems({_LOCK_LIST: _BETA}, "go", {})

    assert len(problems) == 1
    assert "carries no marker" in problems[0]


def test_a_marker_on_an_unlisted_tool_fails() -> None:
    """The contamination case, and the reason the check runs both directions.

    A v4 tool advertising a beta marker would read, to a caller or a model, as
    reaching an API surface it never touches.
    """
    problems = gate.marker_problems({}, "python", {_TAG_LIST: _BETA})

    assert len(problems) == 1
    assert "the census does not" in problems[0]


def test_a_marker_naming_another_surface_fails() -> None:
    """The marker a caller reads has to be the surface the client picks."""
    problems = gate.marker_problems({_LOCK_LIST: _BETA}, "go", {_LOCK_LIST: "v4alpha"})

    assert len(problems) == 1
    assert "advertises v4alpha" in problems[0]


def test_a_census_line_that_is_not_a_pair_fails(tmp_path: Path) -> None:
    """A malformed line would otherwise read as a tool with no surface."""
    path = tmp_path / "api-surfaces.txt"
    path.write_text("# comment\nlinode_lock_list\n", encoding="utf-8")

    with pytest.raises(SystemExit, match="is not"):
        gate.read_census(path)


def test_a_tool_listed_twice_fails(tmp_path: Path) -> None:
    """Two lines for one tool leaves no answer to which surface it is on."""
    path = tmp_path / "api-surfaces.txt"
    path.write_text(
        "linode_lock_list v4beta\nlinode_lock_list v4alpha\n", encoding="utf-8"
    )

    with pytest.raises(SystemExit, match="listed twice"):
        gate.read_census(path)


def test_comments_and_blanks_are_skipped(tmp_path: Path) -> None:
    """The shipped census is mostly prose explaining when to edit it."""
    path = tmp_path / "api-surfaces.txt"
    path.write_text("# why this file exists\n\n  \n", encoding="utf-8")

    assert gate.read_census(path) == {}


def test_the_checked_in_census_agrees_with_the_contract() -> None:
    """The real census, the real descriptors, and both real generated trees."""
    census = gate.read_census()
    assert gate.census_problems(census, gate.declared_surfaces()) == []

    for language in gate.registered_languages():
        marked = gate.marked_tools(language)
        assert gate.marker_problems(census, language, marked) == []


def test_every_registered_language_has_a_tree_to_scan() -> None:
    """A language registered with no arm here would go unchecked in silence."""
    for language in gate.registered_languages():
        assert language in gate._TREES
