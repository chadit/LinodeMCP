#!/bin/bash
#
# Generate markdown release notes from commits since a given tag.
#
# Walks first-parent commits between <previous_tag>..HEAD, groups them by
# conventional commit type (feat/fix/docs/...), and emits a markdown
# changelog. PR titles via `gh` take precedence over commit subjects when
# a commit references a PR; falls back gracefully if `gh` isn't available.
#
# Run from the repo root (the release workflow does this automatically).
#
# Usage:
#   .github/scripts/generate-release-notes.sh <previous_tag>
#
# Optional env (provided automatically inside GitHub Actions):
#   GH_TOKEN   Auth for `gh pr view` lookups.
#   GH_REPO    Repo slug (e.g. chadit/LinodeMCP). Without it, PR title
#              lookup is skipped and commit subjects are used.

set -euo pipefail

# Resolve a PR title via gh, empty on miss. Stays best-effort: any failure
# (no gh, no auth, deleted PR) just falls through to the commit subject.
get_pr_title() {
	local pr_num="${1}"
	local repo="${2}"
	if command -v gh >/dev/null 2>&1 && [[ -n "${repo}" ]]; then
		gh pr view "${pr_num}" --json title -q '.title' -R "${repo}" 2>/dev/null || echo ""
	fi
}

main() {
	if [[ $# -gt 1 ]]; then
		echo "Usage: $0 [previous_tag]" >&2
		exit 1
	fi

	local previous_tag="${1:-}"
	local repo="${GH_REPO:-}"
	local git_range

	if [[ -z "${previous_tag}" ]]; then
		# No prior tag, so walk every commit reachable from HEAD so the
		# first release still gets a real changelog.
		git_range="HEAD"
	else
		if ! git rev-parse "${previous_tag}" >/dev/null 2>&1; then
			echo "Error: tag '${previous_tag}' not found" >&2
			exit 1
		fi
		git_range="${previous_tag}..HEAD"
	fi

	declare -A groups
	declare -A labels=(
		[feat]="Features"
		[fix]="Bug Fixes"
		[perf]="Performance"
		[refactor]="Refactoring"
		[test]="Tests"
		[docs]="Documentation"
		[ci]="CI/CD"
		[build]="Build"
		[chore]="Chores"
		[style]="Style"
	)

	local hash subject pr_num msg pr_title body_title type desc line
	while IFS= read -r line; do
		hash="${line%% *}"
		subject="${line#* }"
		pr_num=""

		# Squash merges end with `(#123)`; merge commits start with
		# `Merge pull request #123`. Either form yields a PR number.
		if [[ "${subject}" =~ ^Merge\ pull\ request\ \#([0-9]+) ]]; then
			pr_num="${BASH_REMATCH[1]}"
		elif [[ "${subject}" =~ \(#([0-9]+)\)$ ]]; then
			pr_num="${BASH_REMATCH[1]}"
		fi

		if [[ -n "${pr_num}" ]]; then
			pr_title=$(get_pr_title "${pr_num}" "${repo}")
			if [[ -n "${pr_title}" ]]; then
				msg="${pr_title} (#${pr_num})"
			elif [[ "${subject}" =~ ^Merge\ pull\ request ]]; then
				# Merge-commit subjects are boilerplate; pull the real
				# title from the body's first line. Parameter expansion
				# rather than `head -1` to avoid SIGPIPE under pipefail.
				body_title=$(git log "${hash}" -1 --format="%b")
				body_title="${body_title%%$'\n'*}"
				msg="${body_title:-${subject}} (#${pr_num})"
			else
				msg="${subject}"
			fi
		else
			msg="${subject}"
		fi

		# Trailing `!?` allows the conventional-commits breaking-change
		# marker (`feat!:` or `feat(scope)!:`) without losing the type.
		if [[ "${msg}" =~ ^([a-z]+)(\(.+\))?!?:\ (.+)$ ]]; then
			type="${BASH_REMATCH[1]}"
			desc="${BASH_REMATCH[3]}"
			if [[ -n "${labels[${type}]+exists}" ]]; then
				groups["${type}"]+="- ${desc}"$'\n'
				continue
			fi
		fi
		groups["other"]+="- ${msg}"$'\n'
	done < <(git log "${git_range}" --first-parent --format="%H %s")

	local order=(feat fix perf refactor test docs ci build chore style other)
	local printed=false key

	for key in "${order[@]}"; do
		if [[ -z "${groups[${key}]+exists}" ]]; then
			continue
		fi
		if [[ "${printed}" == "true" ]]; then
			echo ""
		fi
		if [[ "${key}" == "other" ]]; then
			echo "## Other"
		else
			echo "## ${labels[${key}]}"
		fi
		printf '%s' "${groups[${key}]}"
		printed=true
	done

	if [[ "${printed}" == "false" ]]; then
		echo "No notable changes."
	fi
}

main "$@"
