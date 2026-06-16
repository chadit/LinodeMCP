package builder

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// wildcardChar is the only glob metacharacter the builder honors in
// pattern arguments. Matches the spec used by the active-profile
// resolver so user-facing wildcard semantics stay consistent across
// the builder and config-driven profiles.
const wildcardChar = "*"

// MatchPatterns expands the given list of literal-or-wildcard
// patterns against a tool catalog and returns the deduplicated,
// sorted list of matching tool names. Used by the Phase 8.4
// `_add_tools` and `_remove_tools` handlers.
//
//   - Literal entries (no "*") must equal a registered tool name to
//     contribute. Unknown literals contribute nothing rather than
//     erroring out; the handler reports a hit count so the caller
//     can detect typos by absence.
//   - Wildcard entries use filepath.Match (shell-glob). A malformed
//     pattern matches nothing.
//   - The same name produced by multiple patterns appears once.
//
// Returns an empty slice when patterns is nil or empty; never nil.
func MatchPatterns(patterns []string, catalog []profiles.ToolDescriptor) []string {
	seen := make(map[string]struct{}, len(catalog))
	out := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		matches := matchOne(pattern, catalog)
		for _, name := range matches {
			if _, dup := seen[name]; dup {
				continue
			}

			seen[name] = struct{}{}

			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

// matchOne expands a single pattern against the catalog. Split from
// MatchPatterns to keep the loop body readable; behavior is equivalent
// to the active-profile resolver's matchPattern helper.
func matchOne(pattern string, catalog []profiles.ToolDescriptor) []string {
	if !strings.Contains(pattern, wildcardChar) {
		for i := range catalog {
			if catalog[i].Name == pattern {
				return []string{pattern}
			}
		}

		return nil
	}

	matches := make([]string, 0)

	for idx := range catalog {
		ok, err := filepath.Match(pattern, catalog[idx].Name)
		if err != nil || !ok {
			continue
		}

		matches = append(matches, catalog[idx].Name)
	}

	return matches
}
