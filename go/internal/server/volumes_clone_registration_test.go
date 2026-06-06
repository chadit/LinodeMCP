package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

func TestVolumeCloneToolRegisteredAsWrite(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	requireNoError(t, err)

	infos := srv.ToolInfos()
	requireNotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		if info.Name != "linode_volume_clone" {
			continue
		}

		assertEqual(t, profiles.CapWrite, info.Capability)
		assertContains(t, info.InputSchema.Properties, "volume_id")
		assertContains(t, info.InputSchema.Properties, "label")
		assertContains(t, info.InputSchema.Properties, "confirm")
		assertContains(t, info.InputSchema.Properties, "dry_run")

		return
	}

	t.Fatal("linode_volume_clone should be registered")
}
