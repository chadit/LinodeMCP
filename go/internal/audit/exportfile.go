package audit

import (
	"errors"
	"fmt"
	"os"
)

// Where an export lands and how large it is allowed to be. Both belong to the
// audit subsystem rather than to a caller: the cap keeps an unbounded range out
// of memory, and the temp file is the only thing an export leaves behind.

// ResolveMaxRecords applies the default and the hard ceiling to a requested
// cap. Zero or negative means the caller asked for the default.
func ResolveMaxRecords(requested int) int {
	if requested <= 0 {
		return DefaultExportMaxRecords
	}

	return min(requested, MaxExportRecords)
}

// ExportToFile encodes the events into a temp file named for the format and
// answers its path. The file is left for the caller to read; the OS reclaims
// the temp directory on its own schedule.
//
// The format doubles as the extension because the three the contract allows are
// spelled the way their files are. A format the encoder does not know reaches
// here only from a caller the contract's own rule does not stand in front of.
func ExportToFile(events []*Event, format string) (string, error) {
	file, err := os.CreateTemp("", "linode-audit-export-*."+format)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	// The close travels with the encode because a buffered write reports its
	// failure there, and a half-written export is removed rather than named.
	if err := errors.Join(EncodeEvents(file, events, format), file.Close()); err != nil {
		_ = os.Remove(file.Name())

		return "", fmt.Errorf("write export: %w", err)
	}

	return file.Name(), nil
}
