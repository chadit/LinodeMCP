package server_test

import (
	"strings"
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

		raw := string(info.RawInputSchema)
		for _, key := range []string{"volume_id", "label", "confirm", "dry_run"} {
			if !strings.Contains(raw, key) {
				t.Errorf("info.RawInputSchema missing key %v", key)
			}
		}

		return
	}

	t.Fatal("linode_volume_clone should be registered")
}
