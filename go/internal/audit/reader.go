package audit

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"time"
)

// DefaultRecentLimit is the default number of events returned by a
// recent-events query when the caller does not specify a limit.
const DefaultRecentLimit = 20

// MaxRecentLimit caps the recent-events query so a single call can't
// pull an unbounded slice into memory. Callers asking for more are
// clamped to this value.
const MaxRecentLimit = 200

// RecentQuery filters a recent-events scan. Zero-valued fields mean
// "no constraint" for that dimension. Tool is a glob (path.Match
// syntax); Capability and Status are exact matches; Since and Until
// bound the event timestamp (inclusive lower, inclusive upper).
type RecentQuery struct {
	Limit       int
	Since       time.Time
	Until       time.Time
	Tool        string
	Capability  Capability
	Status      Status
	IncludeMeta bool
}

// ReadRecent returns up to Limit events from the audit directory,
// newest first, matching the query filters. It scans the active
// audit.log first (today's events), then rotated audit-YYYY-MM-DD.log
// and .log.gz files in date-descending order, stopping once Limit
// matches are collected.
//
// A missing directory (no audit ever written) returns an empty slice,
// not an error: querying before the first event is a normal state.
// Undecodable lines are skipped so a single corrupt record doesn't
// abort the scan. File-open failures mid-scan are skipped too; the
// returned error is reserved for a directory that exists but can't be
// listed.
func ReadRecent(dir string, query *RecentQuery) ([]Event, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultRecentLimit
	}

	if limit > MaxRecentLimit {
		limit = MaxRecentLimit
	}

	root, err := openReadRoot(dir)
	if err != nil {
		if errors.Is(err, errAuditDirMissing) {
			return []Event{}, nil
		}

		return nil, err
	}

	defer func() { _ = root.Close() }()

	files, err := orderedAuditFiles(root)
	if err != nil {
		return nil, fmt.Errorf("audit: list audit dir %s: %w", dir, err)
	}

	results := make([]Event, 0, limit)

	for _, name := range files {
		events := readEventsFromFile(root, name)

		// Within a file lines are append-order (oldest first); walk
		// backwards so the accumulator stays newest-first across the
		// date-descending file order.
		for i := range slices.Backward(events) {
			if !query.matches(&events[i]) {
				continue
			}

			results = append(results, events[i])
			if len(results) >= limit {
				return results, nil
			}
		}
	}

	return results, nil
}

// matches reports whether an event satisfies every set filter in the
// query. Meta events are excluded unless IncludeMeta is set.
func (q *RecentQuery) matches(event *Event) bool {
	if !q.IncludeMeta && event.ToolCapability == CapabilityMeta {
		return false
	}

	if q.Tool != "" {
		ok, err := path.Match(q.Tool, event.Tool)
		if err != nil || !ok {
			return false
		}
	}

	if q.Capability != "" && event.ToolCapability != q.Capability {
		return false
	}

	if q.Status != "" && event.Status != q.Status {
		return false
	}

	if !q.Since.IsZero() && event.TS.Before(q.Since) {
		return false
	}

	if !q.Until.IsZero() && event.TS.After(q.Until) {
		return false
	}

	return true
}

// openReadRoot opens an os.Root on dir for reading. A missing
// directory returns errAuditDirMissing so callers can treat "queried
// before the first event" as an empty result rather than a hard error
// (and without a (nil, nil) return). Other open failures are wrapped.
func openReadRoot(dir string) (*os.Root, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errAuditDirMissing
		}

		return nil, fmt.Errorf("audit: open read root %s: %w", dir, err)
	}

	return root, nil
}

// orderedAuditFiles returns the audit file names to scan, newest
// first: the active audit.log (if present) followed by rotated files
// in date-descending order. Caller must hold the root open.
func orderedAuditFiles(root *os.Root) ([]string, error) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var (
		hasActive bool
		rotated   []rotatedFile
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == ActiveLogFileName {
			hasActive = true

			continue
		}

		if day, ok := parseRotatedFileDay(name); ok {
			rotated = append(rotated, rotatedFile{name: name, day: day})
		}
	}

	// Newest rotated date first.
	sort.Slice(rotated, func(i, j int) bool {
		return rotated[i].day.After(rotated[j].day)
	})

	names := make([]string, 0, len(rotated)+1)
	if hasActive {
		names = append(names, ActiveLogFileName)
	}

	for _, file := range rotated {
		names = append(names, file.name)
	}

	return names, nil
}

// rotatedFile pairs a rotated file name with its parsed day so the
// scan can sort by date without re-parsing.
type rotatedFile struct {
	name string
	day  time.Time
}

// readEventsFromFile decodes every JSON line in name into an Event,
// in file order (oldest first). Gzipped rotated files are
// decompressed transparently. Returns whatever decoded successfully;
// open failures yield an empty slice (the scan skips the file), and
// undecodable lines are skipped individually.
func readEventsFromFile(root *os.Root, name string) []Event {
	file, err := root.Open(name)
	if err != nil {
		return nil
	}

	defer func() { _ = file.Close() }()

	var reader io.Reader = file

	if isGzipName(name) {
		gzReader, gzErr := gzip.NewReader(file)
		if gzErr != nil {
			return nil
		}

		defer func() { _ = gzReader.Close() }()

		reader = gzReader
	}

	var events []Event

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, bufScanInitial), bufScanMax)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		events = append(events, event)
	}

	return events
}

// Scanner buffer sizes. Audit lines are small (a few hundred bytes
// typically), but a deeply nested args map could push past the
// default 64KB token cap, so the max is raised to keep large events
// readable rather than silently truncated.
const (
	bufScanInitial = 64 * 1024
	bufScanMax     = 1024 * 1024
)

// isGzipName reports whether name is a gzipped rotated log.
func isGzipName(name string) bool {
	const gzSuffix = ".gz"

	return len(name) >= len(gzSuffix) && name[len(name)-len(gzSuffix):] == gzSuffix
}
