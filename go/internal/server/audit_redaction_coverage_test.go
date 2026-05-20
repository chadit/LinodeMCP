package server_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestRedactionCoversSensitiveArgNames is the heuristic catch-net from
// the audit-log spec's Phase 1a. It scans every tool the server could
// register (across all profiles, via AllToolInfos) for arg names
// containing a sensitive substring, then asserts each hit is either in
// the redaction list or explicitly allowlisted as a known-safe name.
//
// The runtime redaction rule is exact-match; this heuristic is the
// substring-based safety net that flags a new tool shipping a
// sensitive arg the implementer forgot to add to the redaction list.
// It would have caught the SSL-upload tool's private_key arg.
func TestRedactionCoversSensitiveArgNames(t *testing.T) {
	t.Parallel()

	// Case-insensitive substrings that flag an arg name as potentially
	// carrying a secret. Mirrors the spec's Risks-section heuristic.
	sensitiveSubstrings := []string{"pass", "token", "key", "secret"}

	// Arg names that match a substring but are NOT secrets, so they're
	// intentionally absent from the redaction list. Each entry needs a
	// justification because the heuristic exists to catch real leaks; a
	// careless addition here silently disables the catch-net.
	//
	//   - key_id: an Object Storage access-key identifier, not the
	//     secret key material. The ID is safe to log.
	//   - sshkey_id: the numeric ID of an SSH key resource, not the
	//     key material. Safe to log.
	//   - required_token_scopes: a profile-builder arg holding a list
	//     of OAuth scope names (e.g. "linodes:read_write"), not a
	//     token value.
	knownSafe := map[string]struct{}{
		"key_id":                {},
		"sshkey_id":             {},
		"required_token_scopes": {},
	}

	srv := newCapabilityTestServer(t)
	infos := srv.AllToolInfos()
	require.NotEmpty(t, infos, "server must register at least one tool")

	redactionSet := audit.RedactionFieldSet()

	for _, info := range infos {
		for argName := range info.InputSchema.Properties {
			if !matchesAnySubstring(argName, sensitiveSubstrings) {
				continue
			}

			if _, safe := knownSafe[argName]; safe {
				continue
			}

			_, redacted := redactionSet[argName]
			assert.True(
				t, redacted,
				"tool %q declares arg %q which looks sensitive (matches one of %v) "+
					"but is not in the audit redaction list; add it to "+
					"RedactionFields() in both Go and Python, or allowlist it in "+
					"knownSafe with a justification",
				info.Name, argName, sensitiveSubstrings,
			)
		}
	}
}

// matchesAnySubstring reports whether the lowercased arg name contains
// any of the provided substrings.
func matchesAnySubstring(argName string, substrings []string) bool {
	lower := strings.ToLower(argName)

	for _, sub := range substrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}

	return false
}
