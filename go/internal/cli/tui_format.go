package cli

import (
	"fmt"
	"strings"
)

// dash is the placeholder shown for an empty optional column (mode,
// plan_id) so a blank cell reads as "not set" rather than looking like a
// rendering gap.
const dash = "-"

// fixedWidth pads or truncates s to exactly width runes, so fixed-column
// tables stay aligned regardless of value length. A value longer than the
// column is cut to fit; a shorter one is right-padded with spaces.
func fixedWidth(value string, width int) string {
	runes := []rune(value)
	if len(runes) > width {
		return string(runes[:width])
	}

	return value + strings.Repeat(" ", width-len(runes))
}

// shortTimestamp trims an RFC 3339 timestamp to its "YYYY-MM-DD HH:MM:SS"
// prefix for compact display, replacing the 'T' separator with a space. A
// value too short to trim is returned as-is.
func shortTimestamp(timestamp string) string {
	// dateTimeLen is the rune length of an RFC 3339 date-time without the
	// fractional seconds or zone: 2006-01-02T15:04:05.
	const dateTimeLen = 19
	if len(timestamp) < dateTimeLen {
		return timestamp
	}

	return strings.Replace(timestamp[:dateTimeLen], "T", " ", 1)
}

// defaultDash returns dash for an empty value, otherwise the value, so an
// unset optional field renders as a placeholder.
func defaultDash(value string) string {
	if value == "" {
		return dash
	}

	return value
}

// planIDOrDash renders an optional plan id: the id when present, else the
// dash placeholder.
func planIDOrDash(planID *string) string {
	if planID == nil || *planID == "" {
		return dash
	}

	return *planID
}

// wrapDecode wraps a JSON decode error with the context of what was being
// parsed, so a render fallback can report which payload failed.
func wrapDecode(what string, err error) error {
	return fmt.Errorf("decode %s payload: %w", what, err)
}
