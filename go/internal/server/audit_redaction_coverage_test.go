package server_test

import (
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
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
	//   - token_uuid: an image share group token resource identifier,
	//     not token material. Safe to log.
	//   - token_id: numeric personal access token resource identifier,
	//     not token material. Safe to log.
	knownSafe := map[string]struct{}{
		"check_passive":         {}, // Health-check mode, not credential material.
		"key_id":                {},
		"sshkey_id":             {},
		"required_token_scopes": {},
		"token_uuid":            {},
		"token_id":              {},
	}

	srv := newCapabilityTestServer(t)

	infos := srv.AllToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

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
			if !redacted {
				t.Error("redacted = false, want true")
			}
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

// TestRedactionCoversSensitivePIIArgNames is the Phase 4c PII tier
// catch-net. Parallel structure to TestRedactionCoversSensitiveArgNames
// but for PII substrings against the PII redaction list. The PII tier
// is operator-controllable (audit.redact_pii), but the catch-net runs
// regardless: a tool that ships a new PII arg without updating either
// the PII redaction list or this allowlist is a bug worth surfacing,
// even if the running operator has PII redaction off today.
//
// A PII-substring hit must be either in the PII redaction list
// (RedactionFieldsPII), in the credential list (a PII name with
// secret-like content also gets always-on redaction), or explicitly
// allowlisted as a name that contains a PII substring but is not
// personally identifying (e.g. firewall IP addresses, network
// address args).
func TestRedactionCoversSensitivePIIArgNames(t *testing.T) {
	t.Parallel()

	// Case-insensitive substrings that flag an arg name as potentially
	// carrying PII. Conservative scope per the spec.
	piiSubstrings := []string{"tax", "address", "phone", "dob", "card", "cvv"}

	// Arg names that match a PII substring but are NOT personally
	// identifying. Each entry needs a justification because the
	// heuristic exists to catch real PII leaks; a careless addition
	// here silently disables the catch-net.
	//
	//   - address: an IPv4 or IPv6 network address (linode_instance_ips,
	//     linode_networking, linode_nodebalancers). Network addresses
	//     are operational, not PII.
	//   - addresses: a list of network addresses on firewall rules
	//     (linode_firewalls). Same reasoning as `address`.
	knownSafePII := map[string]struct{}{
		"address":   {},
		"addresses": {},
	}

	srv := newCapabilityTestServer(t)

	infos := srv.AllToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	piiSet := audit.RedactionFieldSetPII()
	credSet := audit.RedactionFieldSet()

	for _, info := range infos {
		for argName := range info.InputSchema.Properties {
			if !matchesAnySubstring(argName, piiSubstrings) {
				continue
			}

			if _, safe := knownSafePII[argName]; safe {
				continue
			}

			_, inPII := piiSet[argName]

			_, inCred := credSet[argName]
			if !inPII && !inCred {
				t.Error("expected condition to be true")
			}
		}
	}
}
