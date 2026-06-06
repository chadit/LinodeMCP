package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRegionAvailabilityToolsRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	requireNotEmpty(t, infos, "server must expose registered tools")

	wantTools := map[string]struct {
		wantRegionID bool
	}{
		"linode_region_availability_list": {},
		"linode_region_availability_get":  {wantRegionID: true},
	}

	found := make(map[string]bool, len(wantTools))

	for _, info := range infos {
		want, ok := wantTools[info.Name]
		if !ok {
			continue
		}

		assertEqual(t, profiles.CapRead, info.Capability, "region availability tools are read-only routes")
		assertContains(t, info.InputSchema.Properties, "environment", "environment parameter should be exported")
		assertNotContains(t, info.InputSchema.Properties, "confirm", "read-only route should not require confirm")

		if want.wantRegionID {
			assertContains(t, info.InputSchema.Properties, "region_id", "region_id parameter should be exported")
			assertContains(t, info.InputSchema.Required, "region_id", "region_id should be required")
		}

		found[info.Name] = true
	}

	for name := range wantTools {
		assertTruef(t, found[name], "%s should be registered", name)
	}
}
