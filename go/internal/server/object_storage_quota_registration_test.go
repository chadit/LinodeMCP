package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestObjectStorageQuotaGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	require.NotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_object_storage_quota_get" {
			continue
		}

		assert.Equal(t, profiles.CapRead, info.Capability, "quota get is a read-only route")
		assert.Contains(t, info.InputSchema.Properties, "obj_quota_id", "quota ID parameter should be exported")
		assert.NotContains(t, info.InputSchema.Properties, "confirm", "read-only route should not require confirm")

		return
	}

	t.Fatalf("linode_object_storage_quota_get should be registered")
}
