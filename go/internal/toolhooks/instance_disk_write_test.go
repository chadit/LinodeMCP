package toolhooks_test

import (
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	diskWriteCreatePath = "/linode/instances/123/disks"
	diskWriteNewLabel   = "boot"
	diskWriteTargetSize = 2048
	argDiskSize         = "size"
)

// The resize preview reads the disk so the size it starts from is reported
// beside the size it is headed for. A read that answered with nothing leaves
// the starting size out rather than naming a zero, which is the only place the
// two wordings differ.
func TestLinodeInstanceDiskResizePreviewNamesTheSizeItStartsFrom(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		state map[string]any
		want  string
	}{
		"a disk that answered with its size": {
			state: map[string]any{argSimpleLabel: simpleWriteDiskLabel, argDiskSize: 1024},
			want:  "Disk resizes from 1024 MB to 2048 MB.",
		},
		"a disk that answered with none": {
			state: map[string]any{argSimpleLabel: simpleWriteDiskLabel},
			want:  "Disk resizes to 2048 MB.",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := simpleWriteStateServer(t, test.state)
			request := requestWith(map[string]any{
				argLinodeID: float64(123), argDiskID: float64(5),
				argDiskSize: float64(diskWriteTargetSize), keyDryRun: true,
			})

			result, err := toolhooks.LinodeInstanceDiskResizePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPost,
				simpleWriteDiskPath+"/resize", map[string]any{argDiskSize: diskWriteTargetSize},
			)
			if err != nil {
				t.Fatalf("LinodeInstanceDiskResizePreview: %v", err)
			}

			envelope := simpleWritePreview(t, result)
			if effects := simpleWriteEffects(t, envelope); len(effects) != 1 || effects[0] != test.want {
				t.Errorf("side_effects = %v, want [%q]", effects, test.want)
			}

			warnings, isList := envelope["warnings"].([]any)
			want := "The instance must be powered off to resize a disk."

			if !isList || len(warnings) != 1 || warnings[0] != want {
				t.Errorf("warnings = %v, want [%q]", envelope["warnings"], want)
			}
		})
	}
}
