package audit_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestResolveMaxRecordsAppliesTheDefaultAndTheCeiling covers the cap an
// unbounded range would otherwise pull into memory.
func TestResolveMaxRecordsAppliesTheDefaultAndTheCeiling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		requested int
		want      int
	}{
		{name: "unasked for", requested: 0, want: audit.DefaultExportMaxRecords},
		{name: "negative", requested: -1, want: audit.DefaultExportMaxRecords},
		{name: "under the ceiling", requested: 5, want: 5},
		{
			name:      "over the ceiling",
			requested: audit.MaxExportRecords + 1,
			want:      audit.MaxExportRecords,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := audit.ResolveMaxRecords(testCase.requested); got != testCase.want {
				t.Errorf("cap = %d, want %d", got, testCase.want)
			}
		})
	}
}

// TestExportToFileLandsUnderTheFormatExtension covers the name the answer
// reports and the content behind it.
func TestExportToFileLandsUnderTheFormatExtension(t *testing.T) {
	t.Parallel()

	event := makeTestEvent(tcLinodeInstanceList, audit.CapabilityRead, audit.StatusSuccess, day(20, 8))

	path, err := audit.ExportToFile([]*audit.Event{event}, audit.ExportFormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(path) })

	if filepath.Ext(path) != ".json" {
		t.Errorf("path = %q, want the format's own extension", path)
	}

	if !strings.HasPrefix(filepath.Base(path), "linode-audit-export-") {
		t.Errorf("path = %q, want the export's own prefix", path)
	}

	body, err := os.ReadFile(path) // #nosec G304 -- the path this call just wrote
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []map[string]any
	if unmarshalErr := json.Unmarshal(body, &decoded); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if len(decoded) != 1 || decoded[0][colTool] != tcLinodeInstanceList {
		t.Errorf("the file carries %v, want the one exported event", decoded)
	}
}

// TestExportToFileReportsATempDirectoryItCannotWriteIn covers the other way an
// export fails: the file is never created, so there is nothing to encode into
// and nothing to remove.
//
// Reached the way an operator reaches it, through the directory the OS hands
// out: a TMPDIR naming nothing is what a stale environment or an unmounted
// volume leaves behind.
func TestExportToFileReportsATempDirectoryItCannotWriteIn(t *testing.T) {
	// No t.Parallel: the directory an export lands in is an environment value.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-such-directory"))

	path, err := audit.ExportToFile(nil, audit.ExportFormatJSON)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v (path %q), want the missing directory reported", err, path)
	}

	if path != "" {
		t.Errorf("path = %q, want no file named beside a reported failure", path)
	}
}

// TestExportToFileLeavesNothingBehindWhenTheEncoderRefuses covers the one thing
// a failed export must not do: leave a file under a name nobody was told about.
func TestExportToFileLeavesNothingBehindWhenTheEncoderRefuses(t *testing.T) {
	tempDir := t.TempDir()
	// No t.Parallel: the temp directory the export lands in is an environment
	// value, and this case reads what is left in it.
	t.Setenv("TMPDIR", tempDir)

	path, err := audit.ExportToFile(nil, "xml")
	if !errors.Is(err, audit.ErrUnknownExportFormat) {
		t.Fatalf("err = %v (path %q), want %v", err, path, audit.ErrUnknownExportFormat)
	}

	left, globErr := filepath.Glob(filepath.Join(tempDir, "linode-audit-export-*"))
	if globErr != nil {
		t.Fatalf("unexpected error: %v", globErr)
	}

	if len(left) != 0 {
		t.Errorf("export left %v behind, want the half-written file removed", left)
	}
}
