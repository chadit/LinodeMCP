package server_test

import (
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
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

		raw := string(info.RawInputSchema)
		if !strings.Contains(raw, "obj_quota_id") {
			t.Errorf("info.RawInputSchema missing key %v", "obj_quota_id")
		}

		if strings.Contains(raw, "confirm") {
			t.Errorf("info.RawInputSchema has unexpected key %v", "confirm")
		}

		return
	}

	t.Fatalf("linode_object_storage_quota_get should be registered")
}
