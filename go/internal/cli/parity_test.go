package cli_test

import (
	"sort"
	"strings"
	"testing"
)

// TestCallableSurfaceMatchesRegistry is the CLI-to-registry parity gate,
// mirroring the tool-manifest idea: the set of tools `call` will accept
// must equal the server's registry, so the CLI cannot silently drift from
// the tool surface.
//
// RunCallCommand validates a requested tool against AllToolInfos before
// dispatch (rejectUnknownTool), so AllToolInfos IS the callable surface.
// ToolCatalog is the authoritative registry. Asserting the two name sets
// are equal proves every registered tool is reachable from `call` and the
// CLI exposes nothing the registry doesn't.
func TestCallableSurfaceMatchesRegistry(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	allInfos := srv.AllToolInfos()
	callable := make(map[string]bool, len(allInfos))

	for _, info := range allInfos {
		if callable[info.Name] {
			t.Errorf("tool %q appears twice in the callable surface", info.Name)
		}

		callable[info.Name] = true
	}

	catalog := srv.ToolCatalog()
	registry := make(map[string]bool, len(catalog))

	for _, descriptor := range catalog {
		registry[descriptor.Name] = true
	}

	missing := diffKeys(registry, callable)
	extra := diffKeys(callable, registry)

	if len(missing) > 0 {
		t.Errorf("registry tools the CLI cannot call: %s", strings.Join(missing, ", "))
	}

	if len(extra) > 0 {
		t.Errorf("CLI-callable tools missing from the registry: %s", strings.Join(extra, ", "))
	}
}

// diffKeys returns the sorted keys present in want but absent from have.
func diffKeys(want, have map[string]bool) []string {
	var out []string

	for name := range want {
		if !have[name] {
			out = append(out, name)
		}
	}

	sort.Strings(out)

	return out
}
