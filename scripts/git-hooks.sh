#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat >&2 <<'USAGE'
Usage: scripts/git-hooks.sh install|check

install  Install both pre-commit and pre-push hooks from .pre-commit-config.yaml.
check    Fail if either generated hook is missing.
USAGE
}

repo_root() {
	git rev-parse --show-toplevel 2>/dev/null || pwd
}

hook_path() {
	local root="$1"
	local hook_type="$2"
	git -C "$root" rev-parse --git-path "hooks/${hook_type}"
}

require_generated_hook() {
	local root="$1"
	local hook_type="$2"
	local hook
	hook="$(hook_path "$root" "$hook_type")"
	if [ ! -f "$hook" ]; then
		echo "[error] missing ${hook_type} hook at ${hook}" >&2
		echo "        run: make install-hooks" >&2
		return 1
	fi
	if ! grep -q 'pre-commit' "$hook" 2>/dev/null; then
		echo "[error] ${hook_type} hook at ${hook} is not managed by pre-commit" >&2
		echo "        run: make install-hooks" >&2
		return 1
	fi
}

install_hooks() {
	local root="$1"
	if check_hooks "$root" >/dev/null 2>&1; then
		return 0
	fi

	if ! command -v pre-commit >/dev/null 2>&1; then
		echo "[warn] pre-commit not installed; skipping hook installation" >&2
		return 0
	fi

	echo "Installing pre-commit and pre-push hooks..."
	(cd "$root" && pre-commit install && pre-commit install --hook-type pre-push)
	check_hooks "$root"
}

check_hooks() {
	local root="$1"
	require_generated_hook "$root" pre-commit
	require_generated_hook "$root" pre-push
}

main() {
	if [ "$#" -ne 1 ]; then
		usage
		exit 2
	fi

	local root
	root="$(repo_root)"

	case "$1" in
	install)
		install_hooks "$root"
		;;
	check)
		check_hooks "$root"
		;;
	*)
		usage
		exit 2
		;;
	esac
}

main "$@"
