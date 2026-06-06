package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRegionGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	requireNotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_region_get" {
			continue
		}

		assertEqual(t, profiles.CapRead, info.Capability, "region get is a read-only route")
		assertContains(t, info.InputSchema.Properties, "region_id", "region_id parameter should be exported")
		assertContains(t, info.InputSchema.Required, "region_id", "region_id should be required")
		assertNotContains(t, info.InputSchema.Properties, "confirm", "read-only route should not require confirm")

		return
	}

	t.Fatalf("linode_region_get should be registered")
}
