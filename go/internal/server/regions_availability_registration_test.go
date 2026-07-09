package server_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestRegionAvailabilityToolsRegisteredAsRead(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	wantTools := map[string]struct {
		wantRegionID bool
	}{
		"linode_region_availability_list": {},
		"linode_region_availability_get":  {wantRegionID: true},
	}

	found := make(map[string]bool, len(wantTools))

	for _, info := range infos {
		want, ok := wantTools[info.Name]
		if !ok {
			continue
		}

		if info.Capability != profiles.CapRead {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapRead)
		}

		raw := string(info.RawInputSchema)
		if !strings.Contains(raw, "environment") {
			t.Errorf("info.RawInputSchema missing key %v", "environment")
		}

		if strings.Contains(raw, "confirm") {
			t.Errorf("info.RawInputSchema has unexpected key %v", "confirm")
		}

		if want.wantRegionID {
			if !strings.Contains(raw, "region_id") {
				t.Errorf("info.RawInputSchema missing key %v", "region_id")
			}

			var parsed struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(info.RawInputSchema, &parsed); err != nil {
				t.Fatalf("unmarshal RawInputSchema: %v", err)
			}

			if !slices.Contains(parsed.Required, "region_id") {
				t.Errorf("info.RawInputSchema required does not contain %v", "region_id")
			}
		}

		found[info.Name] = true
	}

	for name := range wantTools {
		if !found[name] {
			t.Error("found[name] = false, want true")
		}
	}
}
