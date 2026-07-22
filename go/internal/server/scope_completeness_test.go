package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// scopelessTools mirrors the documented scopeless list in
// profiles.RequiredScopes: public catalog routes (kernels, database
// engines and types, network transfer prices) plus token-only routes
// documented with an empty scope list (betas, maintenance policies).
// Keep the two lists in step; the test below fails in both directions
// when they drift.
func scopelessTools() map[string]bool {
	return map[string]bool{
		"linode_kernel_get":                  true,
		"linode_kernel_list":                 true,
		"linode_database_engine_get":         true,
		"linode_database_engine_list":        true,
		"linode_database_type_get":           true,
		"linode_database_type_list":          true,
		"linode_network_transfer_price_list": true,
		"linode_beta_get":                    true,
		"linode_beta_list":                   true,
		"linode_maintenance_policy_list":     true,
	}
}

// TestScopeCompletenessEveryToolResolvesScopes closes the silent gap the
// scope parity gate cannot see: when BOTH languages forget to map a tool
// family, the dumps agree on empty and parity passes. RequiredScopes
// returns nil for unknown names by design (the loader degrades to a
// warning), so without this test a forgotten mapping ships as a tool no
// profile scope-check ever restricts. Every registered non-meta tool
// must resolve to at least one scope or sit on the documented scopeless
// list, and every scopeless entry must both exist in the registry and
// still resolve empty so the list cannot go stale.
func TestScopeCompletenessEveryToolResolvesScopes(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scopeless := scopelessTools()
	seen := make(map[string]bool, len(scopeless))

	for _, info := range srv.AllToolInfos() {
		if info.Capability == profiles.CapMeta {
			continue
		}

		scopes := profiles.RequiredScopes(info.Name, info.Capability)

		if scopeless[info.Name] {
			seen[info.Name] = true

			if len(scopes) != 0 {
				t.Errorf("%s is on the scopeless list but resolves %v; remove the stale entry", info.Name, scopes)
			}

			continue
		}

		if len(scopes) == 0 {
			t.Errorf("%s (capability %s) resolves no scope; map it in scope.go or add it to the documented scopeless list", info.Name, info.Capability)
		}
	}

	for name := range scopeless {
		if !seen[name] {
			t.Errorf("scopeless entry %s is not a registered tool; fix the name or drop it", name)
		}
	}
}
