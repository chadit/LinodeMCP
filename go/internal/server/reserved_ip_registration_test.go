package server_test

import (
	"slices"
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

func TestReservedIPDeleteToolRegisteredAsDestroy(t *testing.T) {
	t.Parallel()

	for _, info := range newCapabilityTestServer(t).AllToolInfos() {
		if info.Name != "linode_networking_reserved_ip_delete" {
			continue
		}

		if info.Capability != profiles.CapDestroy {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapDestroy)
		}

		properties := toolSchemaProps(t, &info)
		for _, key := range []string{"environment", keyAddress, keyConfirm, "dry_run", "mode", "plan_id"} {
			if _, ok := properties[key]; !ok {
				t.Errorf("info.InputSchema.Properties missing key %v", key)
			}
		}

		required := toolSchemaRequired(t, &info)
		for _, key := range []string{keyAddress, keyConfirm} {
			if !slices.Contains(required, key) {
				t.Errorf("required fields %v missing %v", required, key)
			}
		}

		return
	}

	t.Fatal("linode_networking_reserved_ip_delete should be registered")
}
