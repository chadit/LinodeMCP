#!/bin/bash
set -euo pipefail

# Detect exported symbols in internal/ packages that are never referenced
# from outside their own package directory (in non-test .go files).
# Runs as part of `make lint` to catch test-only exports.

violations=0

for pkg_dir in internal/*/; do
	# Extract exported symbol names from non-test .go files.
	# Matches: func/type/const/var declarations starting with an uppercase letter.
	symbols=$(grep -hE '^\s*(func|type|const|var)\s+\(?[A-Z]' "$pkg_dir"*.go 2>/dev/null |
		grep -v '_test.go' |
		sed -E 's/.*\s(func|type|const|var)\s+\(?\s*//' |
		sed -E 's/[^A-Za-z0-9_].*//' |
		sort -u || true)

	for sym in $symbols; do
		# Skip empty symbols
		[ -z "$sym" ] && continue

		# Count references to this symbol outside its own package dir,
		# in non-test .go files only.
		ref_count=$(grep -rl --include='*.go' --exclude='*_test.go' "$sym" . |
			grep -vc "^\./${pkg_dir}" || echo "0")

		if [ "$ref_count" -eq 0 ]; then
			echo "WARN: ${pkg_dir}${sym} is exported but has no external references"
			violations=$((violations + 1))
		fi
	done
done

if [ "$violations" -gt 0 ]; then
	echo ""
	echo "Found $violations exported symbol(s) in internal/ with no external references."
	echo "Consider unexporting (lowercasing) these symbols."
	exit 1
fi
