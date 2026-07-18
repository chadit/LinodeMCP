#!/usr/bin/env python3
"""Per-language parity to-do report, aggregated from the ratchet baselines.

The baselines under docs/ ARE the remaining-work lists; this script only
makes them legible per language. For every language registered in
docs/contracts/languages.txt it reports the tools whose absence is accepted (with the
tracking annotation), the proto-surface conversions still owed, and the
shared cross-language debts (uncovered behavior fixtures, missing dry-run
preview cases, confirm-message divergences). A freshly registered language
starts with every manifest tool in its missing list, so this report doubles
as the onboarding checklist that docs/adding-a-language.md points at.

Read-only, stdlib plus scripts/_baselines.py; needs no venv. Run directly or
via `make parity-todo`.
"""

from __future__ import annotations

from pathlib import Path

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_CONTRACTS = _REPO_ROOT / "docs" / "contracts"

# The proto-surface classifiers compare exactly two columns today; their
# lines read "<tool>\t<go status>\t<python status>". A third language means
# extending those scripts and this mapping (docs/adding-a-language.md tracks
# that as part of onboarding).
_PAIRWISE_COLUMNS = ("go", "python")

# gate file -> (label, status a column must reach to stop being a straggler)
_PROTO_GATES = {
    "write-proto-baseline.txt": ("write-proto", "proto"),
    "read-proto-baseline.txt": ("read-proto", "proto"),
    "meta-proto-baseline.txt": ("meta-proto", "proto"),
    "input-proto-baseline.txt": ("input-proto", "generated"),
}


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


def _proto_stragglers() -> dict[str, list[str]]:
    """Return per-language proto-conversion debts across the four gates."""
    owed: dict[str, list[str]] = {}

    for filename, (label, done_status) in sorted(_PROTO_GATES.items()):
        for entry in sorted(_baselines.read_entries(_CONTRACTS / filename)):
            columns = entry.split("\t")
            tool = columns[0]
            statuses = columns[1:]
            for language, status in zip(_PAIRWISE_COLUMNS, statuses, strict=False):
                if status != done_status:
                    owed.setdefault(language, []).append(f"{tool} ({label}: {status})")

    return owed


def _shared_counts() -> list[str]:
    """Summarize the language-neutral debts every implementation shares."""
    uncovered = _baselines.read_baseline(_CONTRACTS / "behavior-baseline.txt")
    dryrun = _baselines.read_entries(_CONTRACTS / "behavior-dryrun-baseline.txt")
    messages = _baselines.read_entries(_CONTRACTS / "message-parity-baseline.txt")

    lines = [f"behavior fixtures uncovered: {len(uncovered)}"]
    lines.extend(
        f"  {tool}  ({annotation or 'no annotation'})"
        for tool, annotation in sorted(uncovered.items())
    )
    lines.append(f"Destroy tools without a dry-run preview case: {len(dryrun)}")
    lines.append(f"confirm-message divergences: {len(messages)}")
    return lines


def main() -> int:
    languages = _languages()
    absences, contract = _tool_absences()
    proto_owed = _proto_stragglers()

    print(f"languages: {', '.join(languages)} (first is the contract reference)")

    for language in languages:
        print(f"\n== {language}")
        missing = absences.get(language, [])
        print(f"tools missing (accepted, tracked): {len(missing)}")
        for tool, annotation in missing:
            print(f"  {tool}  ({annotation or 'no annotation'})")

        owed = proto_owed.get(language, [])
        print(f"proto-surface conversions owed: {len(owed)}")
        for line in owed:
            print(f"  {line}")

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
