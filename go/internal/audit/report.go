package audit

import (
	"context"
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// One named report, run over the same stores the other query operations read.
//
// The engine sits here rather than beside the tool because every decision it
// makes belongs to the audit subsystem: which store answers, what a window
// means, which events a filter admits, and what a definition the vocabulary
// will not take does. The tool layer is left with the two conditions those
// decisions report.

// definitionError marks one failure as the definition's own while leaving its
// sentence alone, because the sentence is what a caller reads back and a wrap
// would double the condition into it.
type definitionError struct{ cause error }

func (d definitionError) Error() string { return d.cause.Error() }

// Unwrap answers the condition beside the cause, so errors.Is finds
// ErrReportDefinition without the sentence carrying its words.
func (d definitionError) Unwrap() []error { return []error{ErrReportDefinition, d.cause} }

// Report runs one named report over the configured database where one answers
// and over the JSONL log otherwise.
//
// It reports ErrReportDefinition for a definition the vocabulary will not take
// and answers whatever the read failed with otherwise. A configured database
// that will not open leaves the log standing behind a warning, which is the
// store's own degrade rule.
//
// now is the reference instant a relative window is measured back from. It is
// handed in rather than read here so a pinned clock reaches the window, which
// is what lets a report fixture replay.
func Report(
	ctx context.Context, sqlitePath, jsonlDir, name string,
	definition *config.ReportConfig, now time.Time,
) (*genlocal.AuditReportResponse, error) {
	query, err := reportQuery(&definition.Filter, now)
	if err != nil {
		return nil, definitionError{cause: err}
	}

	events, warnings, err := ReadWithFallback(sqlitePath,
		func(store string) ([]*Event, error) {
			return ExportEvents(ctx, store, jsonlDir, query)
		})
	if err != nil {
		return nil, err
	}

	return reportAnswer(name, definition, matchingReportEvents(events, &definition.Filter), warnings)
}

// reportAnswer is the report's own answer, built once with its warnings in
// whichever output the definition names.
//
// Both outputs fill both list members because the answer declares both and the
// one the output did not build is answered empty rather than left out.
func reportAnswer(
	name string, definition *config.ReportConfig, matched []*Event, warnings []string,
) (*genlocal.AuditReportResponse, error) {
	if definition.Output == config.ReportOutputSummary {
		columns, err := ValidateGroupBy(definition.GroupBy)
		if err != nil {
			return nil, definitionError{
				cause: fmt.Errorf("validate report group_by: %w", err),
			}
		}

		return genlocal.NewAuditReportResponse(name, config.ReportOutputSummary,
			len(matched), Summarize(matched, columns), []*Event{}, warnings), nil
	}

	if definition.Limit > 0 && len(matched) > definition.Limit {
		matched = matched[:definition.Limit]
	}

	return genlocal.NewAuditReportResponse(name, definition.Output, len(matched),
		[]*genlocal.AuditSummaryRow{}, matched, warnings), nil
}

// reportQuery translates the filter members a store query carries into one
// query.
//
// IncludeMeta is true because the report grammar decides meta inclusion through
// its own capability filter rather than through the tool-layer default, and
// since_offset wins over the absolute since when both are set.
func reportQuery(filter *config.ReportFilter, now time.Time) (*RecentQuery, error) {
	since, err := reportSince(filter, now)
	if err != nil {
		return nil, err
	}

	until, err := reportBound(filter.Until, "until")
	if err != nil {
		return nil, err
	}

	return &RecentQuery{
		Limit:       MaxExportRecords,
		Since:       since,
		Until:       until,
		Tool:        filter.Tool,
		Capability:  Capability(filter.Capability),
		Status:      Status(filter.Status),
		IncludeMeta: true,
	}, nil
}

// reportSince is the load-time lower bound: the offset measured back from the
// reference instant where the filter names one, else the absolute timestamp,
// else no bound.
func reportSince(filter *config.ReportFilter, now time.Time) (time.Time, error) {
	if filter.SinceOffset == "" {
		return reportBound(filter.Since, "since")
	}

	span, err := time.ParseDuration(filter.SinceOffset)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse since_offset %q: %w", filter.SinceOffset, err)
	}

	return now.Add(-span), nil
}

// reportBound parses one absolute bound, answering the zero time for an absent
// one. The configuration validator reads these first, so a value that fails
// here reached the operation from a file nothing checked.
func reportBound(value, member string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s %q: %w", member, value, err)
	}

	return parsed, nil
}

// matchingReportEvents keeps the events the filter members a store query does
// not carry admit: the two membership lists, and the environment and profile
// globs.
func matchingReportEvents(events []*Event, filter *config.ReportFilter) []*Event {
	matched := make([]*Event, 0, len(events))

	for _, event := range events {
		if reportAdmits(event, filter) {
			matched = append(matched, event)
		}
	}

	return matched
}

// reportAdmits is whether one event clears every post-load filter member.
func reportAdmits(event *Event, filter *config.ReportFilter) bool {
	if len(filter.CapabilityIn) > 0 && !slices.Contains(filter.CapabilityIn, event.ToolCapability) {
		return false
	}

	if len(filter.StatusIn) > 0 && !slices.Contains(filter.StatusIn, event.Status) {
		return false
	}

	return globAdmits(filter.Environment, event.Environment) &&
		globAdmits(filter.Profile, event.Profile)
}

// globAdmits is whether one glob admits a value, with an unset glob admitting
// everything. A malformed pattern matches nothing, which is what path.Match
// answers beside its error.
func globAdmits(pattern, value string) bool {
	if pattern == "" {
		return true
	}

	matched, _ := path.Match(pattern, value)

	return matched
}
