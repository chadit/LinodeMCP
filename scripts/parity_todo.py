#!/usr/bin/env python3
"""Per-language parity to-do report, aggregated from the ratchet baselines.

The baselines under docs/contracts/ ARE the remaining-work lists; this script
only makes them legible per language. For every language registered in
docs/contracts/languages.txt it reports the tools whose absence is accepted
(with the tracking annotation) and the shared cross-language debt of mutating
tools that still have no pinned dry-run preview case. A freshly registered
language starts with every manifest tool in its missing list, so this report
doubles as the onboarding checklist that docs/adding-a-language.md points at.

What is NOT here is as much of the answer as what is. The proto surfaces
(input, read, write, meta), behavior-fixture coverage, malformed-response
cases, confirm-message parity, pagination, fixture response shapes, the
list-envelope collapse and route evidence are hard gates with no baseline
file: their debt is always zero because a finding fails the build. Those gates
are named below so an empty report cannot be read as an unmeasured one.

Read-only, stdlib plus scripts/_baselines.py; needs no venv. Run directly or
via `make parity-todo`.
"""

from __future__ import annotations

from pathlib import Path

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"

# The ratchets this report reads. A missing one is a failure rather than an
# empty section: the whole point of the report is to say what is owed, and a
# file that stopped existing subtracts silently.
_RATCHETS = ("tool-parity-baseline.txt", "behavior-dryrun-baseline.txt")

# The gates that hold their class at zero with no baseline file, named with
# what each one refuses. Listed here so the report says where that work went
# rather than leaving it looking unchecked.
_HARD_GATES = (
    ("generated-form", "a hand-written tool surface"),
    ("behavior", "a tool with no shared behavior fixture"),
    ("messages", "confirm text that differs between languages"),
    ("pagination", "a list tool that cannot reach past page one"),
    ("response-shapes", "a fixture body the spec contradicts"),
    ("list-envelope", "a list response built from a falsey-collapsed member"),
    ("route-evidence", "a declared route no client can build"),
)


def _languages() -> list[str]:
    names: list[str] = []
    for raw in (_CONTRACTS / "languages.txt").read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            names.append(stripped.split("\t")[0])
    return names


def _tool_absences() -> tuple[dict[str, list[tuple[str, str]]], list[str]]:
    """Split the tool-parity baseline into per-language absences and the rest."""
    absences: dict[str, list[tuple[str, str]]] = {}
    contract: list[str] = []

    baseline = _baselines.read_baseline(_CONTRACTS / "tool-parity-baseline.txt")
    for entry, annotation in sorted(baseline.items()):
        tool, marker, language = entry.partition(": missing in ")
        if marker:
            absences.setdefault(language, []).append((tool, annotation or ""))
        else:
            contract.append(entry)

    return absences, contract


def _require_ratchets() -> None:
    """Fail when a ratchet this report reads is gone.

    A missing file reads as zero owed work, which is the one wrong answer this
    report can give. Deleting a ratchet is a real move (it is how a gate goes
    hard), so it has to come with the line here that stopped reading it.
    """
    missing = [name for name in _RATCHETS if not (_CONTRACTS / name).exists()]
    if missing:
        msg = (
            f"parity-todo reads baselines that are gone: {', '.join(missing)}."
            " Update _RATCHETS in scripts/parity_todo.py to match what the"
            " gates still keep."
        )
        raise SystemExit(msg)


def _shared_counts() -> list[str]:
    """Summarize the language-neutral debts every implementation shares."""
    dryrun = _baselines.read_entries(_CONTRACTS / "behavior-dryrun-baseline.txt")

    lines = [f"mutating tools without a dry-run preview case: {len(dryrun)}"]
    lines.append("hard gates (no baseline; any finding fails the build):")
    lines.extend(f"  {gate}: {refuses}" for gate, refuses in _HARD_GATES)
    return lines


def main() -> int:
    _require_ratchets()

    languages = _languages()
    absences, contract = _tool_absences()

    print(f"languages: {', '.join(languages)} (first is the contract reference)")

    for language in languages:
        print(f"\n== {language}")
        missing = absences.get(language, [])
        print(f"tools missing (accepted, tracked): {len(missing)}")
        for tool, annotation in missing:
            print(f"  {tool}  ({annotation or 'no annotation'})")

    print("\n== shared (every language)")
    if contract:
        print(f"tool-contract divergences: {len(contract)}")
        for line in contract:
            print(f"  {line}")
    for line in _shared_counts():
        print(line)

    unknown_languages = sorted(set(absences) - set(languages))
    if unknown_languages:
        print(
            "\nWARNING: baseline names languages not in docs/contracts/languages.txt: "
            + ", ".join(unknown_languages)
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
