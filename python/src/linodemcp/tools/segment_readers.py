"""The path-segment readers, answering the sentences one declaration words.

Two contract members read a text argument that is spliced into one URL path
segment. They live here rather than beside the account tools that first wrote
them because the emitter names them for every family that declares a segment,
and a family-named file reads as per-tool code that could be retired with it.
"""

from typing import Any

from linodemcp.tools.helpers import declared_or


def required_pathsafe_segment(arguments: dict[str, Any], name: str) -> tuple[str, str]:
    """Read a text segment that has to survive being spliced into a URL.

    Mirrors Go's RequiredPathSafeArgument end to end. The separator check below
    was written out by each caller; folding it in beside the two-sentence read
    is what lets one reader answer all three of Go's sentences, so the contract
    can name it instead of every hook repeating it.
    """
    return declared_pathsafe_segment(arguments, name, "", "", "")


def declared_pathsafe_segment(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[str, str]:
    """The path-safe reader answering the sentences one declaration words.

    Mirrors Go's DeclaredPathSafeArgument. The segment tools name the characters
    they refuse in five different ways, which is why the words are the
    declaration's and only the accepted set is the member's.
    """
    return _segment_argument(arguments, name, "/?", absent, unusable, refused)


def declared_fragment_safe_segment(
    arguments: dict[str, Any], name: str, absent: str, unusable: str, refused: str
) -> tuple[str, str]:
    """The same reader over the narrower set that also refuses the fragment marker.

    Mirrors Go's DeclaredFragmentSafeArgument. An OAuth client id may carry one
    and the ids these routes address may not, which is a difference in what is
    accepted rather than in what is said.
    """
    return _segment_argument(arguments, name, "/?#", absent, unusable, refused)


def _segment_argument(
    arguments: dict[str, Any],
    name: str,
    guarded: str,
    absent: str,
    unusable: str,
    refused: str,
) -> tuple[str, str]:
    """Read a text argument that has to survive being spliced into one segment.

    Go spells the same body segmentArgument. Both sit in their language's
    hand-validator plumbing set: this is the shared body two contract members
    read through, not a per-tool check.
    """
    if name not in arguments:
        return "", declared_or(absent, f"{name} is required")

    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        return "", declared_or(unusable, f"{name} must be a non-empty string")

    if (
        value != value.strip()
        or any(char in value for char in guarded)
        or ".." in value
    ):
        return "", declared_or(
            refused,
            f"{name} must not contain path separators, "
            "query separators, or traversal segments",
        )

    return value, ""
