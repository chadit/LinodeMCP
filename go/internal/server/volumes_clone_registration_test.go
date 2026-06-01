package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

func TestVolumeCloneToolRegisteredAsWrite(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	require.NoError(t, err)

	infos := srv.ToolInfos()
	require.NotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_volume_clone" {
			continue
		}

		assert.Equal(t, profiles.CapWrite, info.Capability)
		assert.Contains(t, info.InputSchema.Properties, "volume_id")
		assert.Contains(t, info.InputSchema.Properties, "label")
		assert.Contains(t, info.InputSchema.Properties, "confirm")
		assert.Contains(t, info.InputSchema.Properties, "dry_run")

		return
	}

	t.Fatal("linode_volume_clone should be registered")
}
