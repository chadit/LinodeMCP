package tools

import (
	"context"
	"errors"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// The audit subsystem's own local operations, as the declared operations reach
// them.
//
// Each takes the readings and inputs its declaration names and answers the
// shape it declares, so none of them can tell which tool called it. The
// generated arm beside the handlers is what turns a reported condition into the
// sentence a caller reads, which is why nothing here carries prose.

// AuditHealth answers the audit stores' own health.
//
// It reports genlocal.ErrLocalReadFailed only where nothing readable answers:
// a configured database that will not open leaves the JSONL half standing
// behind a warning, which is the store's own degrade rule.
func AuditHealth(ctx context.Context, cfg *config.Config) (*genlocal.AuditHealthResponse, error) {
	answer, err := audit.Health(ctx, audit.ResolveSQLitePath(cfg), audit.ResolveDefaultAuditDir())
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed, err.Error())
	}

	return answer, nil
}

// AuditRecent answers the events the JSONL log holds, newest first, through
// whichever filters the call named.
//
// No store and no cancellation: this operation reads the log alone, and the
// reader behind it takes neither.
func AuditRecent(
	limit int, since, until time.Time, tool, capability, status string, includeMeta bool,
) (*genlocal.AuditRecentResponse, error) {
	events, err := audit.ReadRecent(audit.ResolveDefaultAuditDir(), &audit.RecentQuery{
		Since:       since,
		Until:       until,
		Limit:       limit,
		Tool:        tool,
		Capability:  audit.Capability(capability),
		Status:      audit.Status(status),
		IncludeMeta: includeMeta,
	})
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed, err.Error())
	}

	return genlocal.NewAuditRecentResponse(len(events), events), nil
}

// AuditSummary counts the events in a window into the buckets the call named.
//
// It reports genlocal.ErrLocalInputRejected for a column outside the
// vocabulary, which is answered before any store is opened, and
// genlocal.ErrLocalReadFailed where nothing readable answers.
func AuditSummary(
	ctx context.Context, cfg *config.Config, since time.Time, groupBy []string, includeMeta bool,
) (*genlocal.AuditSummaryResponse, error) {
	columns, err := audit.ValidateGroupBy(groupBy)
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalInputRejected, err.Error())
	}

	answer, err := audit.SummaryOver(ctx, audit.ResolveSQLitePath(cfg),
		audit.ResolveDefaultAuditDir(), since, columns, includeMeta)
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed, err.Error())
	}

	return answer, nil
}

// AuditReport runs one named report from the audit configuration over the same
// stores the other query operations read.
//
// The definition is resolved at call time rather than captured, so editing the
// report file takes effect on the next call. It reports
// genlocal.ErrLocalNotFound for a name the configuration does not declare,
// genlocal.ErrLocalInputRejected for a definition the vocabulary will not take,
// and genlocal.ErrLocalReadFailed where nothing readable answers.
func AuditReport(
	ctx context.Context, cfg *config.Config, clock time.Time, report string,
) (*genlocal.AuditReportResponse, error) {
	definition, declared := cfg.Audit.Reports[report]
	if !declared {
		// No cause: the tool's own sentence names the report it was asked for,
		// which is the only thing there is to say about a name nothing declares.
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalNotFound, "")
	}

	answer, err := audit.Report(ctx, audit.ResolveSQLitePath(cfg),
		audit.ResolveDefaultAuditDir(), report, &definition, clock)

	switch {
	case errors.Is(err, audit.ErrReportDefinition):
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalInputRejected, err.Error())
	case err != nil:
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed, err.Error())
	}

	return answer, nil
}

// AuditExport writes a window of events to a file in the named format and
// answers where it landed.
//
// It reports genlocal.ErrLocalReadFailed where no store answers and
// genlocal.ErrLocalWriteFailed where the file cannot be written. The record cap
// is applied by the subsystem so an unbounded range never reaches memory.
func AuditExport(
	ctx context.Context, cfg *config.Config, format string, since, until time.Time,
	tool string, maxRecords int, includeMeta bool,
) (*genlocal.AuditExportResponse, error) {
	events, warnings, err := audit.ReadWithFallback(audit.ResolveSQLitePath(cfg),
		func(store string) ([]*audit.Event, error) {
			return audit.ExportEvents(ctx, store, audit.ResolveDefaultAuditDir(), &audit.RecentQuery{
				Since:       since,
				Until:       until,
				Limit:       audit.ResolveMaxRecords(maxRecords),
				Tool:        tool,
				IncludeMeta: includeMeta,
			})
		})
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed, err.Error())
	}

	written, err := audit.ExportToFile(events, format)
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalWriteFailed, err.Error())
	}

	return genlocal.NewAuditExportResponse(written, format, len(events), warnings), nil
}
