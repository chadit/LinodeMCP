package server_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRegionAvailabilityToolsRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

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

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		if _, ok := info.InputSchema.Properties["environment"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "environment")
		}

		if _, ok := info.InputSchema.Properties["confirm"]; ok {
			t.Errorf("info.InputSchema.Properties has unexpected key %v", "confirm")
		}

		if want.wantRegionID {
			if _, ok := info.InputSchema.Properties["region_id"]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", "region_id")
			}

			if !slices.Contains(info.InputSchema.Required, "region_id") {
				t.Errorf("info.InputSchema.Required does not contain %v", "region_id")
			}
		}

		found[info.Name] = true
	}

	for name := range wantTools {
		if !found[name] {
			t.Error("found[name] = false, want true")
		}
	}
}
