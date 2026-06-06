package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestObjectStorageQuotaGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	requireNotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_object_storage_quota_get" {
			continue
		}

		assertEqual(t, profiles.CapRead, info.Capability, "quota get is a read-only route")
		assertContains(t, info.InputSchema.Properties, "obj_quota_id", "quota ID parameter should be exported")
		assertNotContains(t, info.InputSchema.Properties, "confirm", "read-only route should not require confirm")

		return
	}

	t.Fatalf("linode_object_storage_quota_get should be registered")
}
