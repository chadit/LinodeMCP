package server_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestRegionGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	for _, info := range infos {
		if info.Name != "linode_region_get" {
			continue
		}

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		if _, ok := toolSchemaProps(t, &info)["region_id"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "region_id")
		}

		if !slices.Contains(toolSchemaRequired(t, &info), "region_id") {
			t.Errorf("info.InputSchema.Required does not contain %v", "region_id")
		}

		if _, ok := toolSchemaProps(t, &info)["confirm"]; ok {
			t.Errorf("info.InputSchema.Properties has unexpected key %v", "confirm")
		}

		return
	}

	t.Fatalf("linode_region_get should be registered")
}
