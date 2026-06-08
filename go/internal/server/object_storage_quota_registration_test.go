package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestObjectStorageQuotaGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	for _, info := range infos {
		if info.Name != "linode_object_storage_quota_get" {
			continue
		}

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		if _, ok := info.InputSchema.Properties["obj_quota_id"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "obj_quota_id")
		}

		if _, ok := info.InputSchema.Properties["confirm"]; ok {
			t.Errorf("info.InputSchema.Properties has unexpected key %v", "confirm")
		}

		return
	}

	t.Fatalf("linode_object_storage_quota_get should be registered")
}
