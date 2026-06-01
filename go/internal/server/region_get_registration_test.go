package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestRegionGetToolRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	require.NotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_region_get" {
			continue
		}

		assert.Equal(t, profiles.CapRead, info.Capability, "region get is a read-only route")
		assert.Contains(t, info.InputSchema.Properties, "region_id", "region_id parameter should be exported")
		assert.Contains(t, info.InputSchema.Required, "region_id", "region_id should be required")
		assert.NotContains(t, info.InputSchema.Properties, "confirm", "read-only route should not require confirm")

		return
	}

	t.Fatalf("linode_region_get should be registered")
}
