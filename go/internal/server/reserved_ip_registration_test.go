package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestReservedIPListToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	for _, info := range newCapabilityTestServer(t).ToolInfos() {
		if info.Name != "linode_networking_reserved_ip_list" {
			continue
		}

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		properties := toolSchemaProps(t, &info)
		for _, key := range []string{"environment", "page", "page_size"} {
			if _, ok := properties[key]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", key)
			}
		}

		return
	}

	t.Fatal("linode_networking_reserved_ip_list should be registered")
}
