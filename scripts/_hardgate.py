"""Shared pieces of the gates that hold a debt class at zero.

A ratchet gate carries a baseline file: findings it already knows about are
accepted, and only new ones fail. A HARD gate has no such file. Every finding
fails by name, there is no ``--update-baseline`` to record one, and the fix is
the change itself.

That removes the acceptance path, and with it the one thing a baseline file was
still good for once it emptied: proof that the gate had a surface to look at. A
ratchet whose scan silently stopped covering anything would still have to
explain a baseline full of entries that suddenly read as fixed. A hard gate has
no such tell, because "no findings" and "nothing scanned" print the same. So a
hard gate says what it measured and fails when that count is zero, which is what
``measured`` is for.
"""

from __future__ import annotations

import sys
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Sequence


def say(line: str) -> None:
    """Emit one report line on stdout (gate output, not debug logging)."""
    sys.stdout.write(line + "\n")


def measured(what: str, count: int) -> None:
    """Fail when a scan covered nothing at all.

    ``what`` names the scan in the gate's own words, since the failure is read
    by whoever broke the scanner rather than by whoever wrote it: a renamed
    classifier entry point, a moved source tree, an emitter that stopped
    emitting.
    """
    if count > 0:
        return

    msg = (
        f"{what} covered nothing. A gate proves nothing about a surface it"
        " never scanned, so this fails rather than reporting a clean run."
    )
    raise SystemExit(msg)


def report(title: str, findings: Sequence[str], fix: str) -> int:
    """Print the findings and return the gate's exit code.

    Every finding is named. There is no accepted subset to diff against, so the
    only two outcomes are a clean surface and a list of things to fix.
    """
    if not findings:
        return 0

    say(f"{title} ({len(findings)}):")
    for line in findings:
        say(f"  {line}")
    say(f"\n{fix}")

    return 1
