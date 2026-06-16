package server_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

func TestVolumeCloneToolRegisteredAsWrite(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	for _, info := range infos {
		if info.Name != "linode_volume_clone" {
			continue
		}

		if info.Capability != profiles.CapWrite {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapWrite)
		}

		if _, ok := info.InputSchema.Properties["volume_id"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "volume_id")
		}

		if _, ok := info.InputSchema.Properties["label"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "label")
		}

		if _, ok := info.InputSchema.Properties["confirm"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "confirm")
		}

		if _, ok := info.InputSchema.Properties["dry_run"]; !ok {
			t.Errorf("info.InputSchema.Properties missing key %v", "dry_run")
		}

		return
	}

	t.Fatal("linode_volume_clone should be registered")
}
