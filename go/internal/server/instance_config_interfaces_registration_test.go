package server_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestInstanceConfigInterfacesListToolRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	for _, info := range srv.AllToolInfos() {
		if info.Name != "linode_instance_config_interface_list" {
			continue
		}

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		properties := toolSchemaProps(t, &info)
		if len(properties) != 3 {
			t.Errorf("schema properties = %v, want only environment, linode_id, and config_id", properties)
		}

		for _, key := range []string{"linode_id", "config_id"} {
			if _, ok := properties[key]; !ok {
				t.Errorf("schema properties missing key %v", key)
			}

			if !slices.Contains(toolSchemaRequired(t, &info), key) {
				t.Errorf("schema required list missing key %v", key)
			}
		}

		return
	}

	t.Fatal("linode_instance_config_interface_list is not registered")
}
