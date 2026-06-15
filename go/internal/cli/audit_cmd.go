package cli

import (
	"context"
	"flag"
	"io"

	"github.com/chadit/LinodeMCP/internal/server"
)

// Audit query tool names the subcommands wrap. They are the same
// CapMeta tools an MCP client would call, so the CLI drives them through
// the identical dispatch and prints their result.
const (
	toolAuditRecent  = "linode_audit_recent"
	toolAuditSummary = "linode_audit_summary"
	toolAuditHealth  = "linode_audit_health"
	toolAuditExport  = "linode_audit_export"
)

// Audit argument field names. These match the audit tools' input schema,
// so the mapped flags land where the tool reads them.
const (
	auditArgTool        = "tool"
	auditArgSince       = "since"
	auditArgLimit       = "limit"
	auditArgFormat      = "format"
	auditArgIncludeMeta = "include_meta"
)

const auditUsage = `Usage: linodemcp audit <subcommand> [flags]

Read the audit log through the same query tools the MCP server exposes.

Subcommands:
  recent [--tool glob] [--since ts] [--limit n] [--include-meta]
                  Most recent events, newest first.
  summary [--since ts]
                  Counts grouped by tool, status, capability, and more.
  health          Audit subsystem state: paths, disk bytes, dropped count.
  export --format json|csv|ndjson [--tool glob] [--since ts]
                  Dump a filtered range to a temp file and print its path.`

// RunAuditCommand dispatches `linodemcp audit <subcommand>` and returns
// the exit code. Each subcommand maps its flags into the matching
// linode_audit_* tool's arguments and drives the shared call path, so the
// output is the tool's own JSON. Output streams are parameters for tests.
func RunAuditCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeln(stderr, auditUsage)

		return ExitUsageError
	}

	switch args[0] {
	case "recent":
		return runAuditRecent(args[1:], stdout, stderr)
	case "summary":
		return runAuditSummary(args[1:], stdout, stderr)
	case "health":
		return runAuditHealth(args[1:], stdout, stderr)
	case "export":
		return runAuditExport(args[1:], stdout, stderr)
	default:
		writef(stderr, "unknown audit subcommand: %s\n\n%s\n", args[0], auditUsage)

		return ExitUsageError
	}
}

// runAuditRecent maps --tool/--since/--limit/--include-meta into
// linode_audit_recent and prints the events.
func runAuditRecent(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("audit recent", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		tool        string
		since       string
		limit       int
		includeMeta bool
	)

	flags.StringVar(&tool, "tool", "", "only events whose tool matches this glob")
	flags.StringVar(&since, "since", "", "only events at or after this RFC 3339 timestamp")
	flags.IntVar(&limit, "limit", 0, "max events to return")
	flags.BoolVar(&includeMeta, "include-meta", false, "include audit/profile meta events")

	if err := flags.Parse(args); err != nil {
		return ExitUsageError
	}

	arguments := map[string]any{}
	putString(arguments, auditArgTool, tool)
	putString(arguments, auditArgSince, since)
	putInt(arguments, auditArgLimit, limit)

	if includeMeta {
		arguments[auditArgIncludeMeta] = true
	}

	return runAuditTool(toolAuditRecent, arguments, stdout, stderr)
}

// runAuditSummary maps --since into linode_audit_summary and prints the
// grouped counts.
func runAuditSummary(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("audit summary", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var since string

	flags.StringVar(&since, "since", "", "only events at or after this RFC 3339 timestamp")

	if err := flags.Parse(args); err != nil {
		return ExitUsageError
	}

	arguments := map[string]any{}
	putString(arguments, auditArgSince, since)

	return runAuditTool(toolAuditSummary, arguments, stdout, stderr)
}

// runAuditHealth drives linode_audit_health, which takes no arguments,
// and prints the subsystem state.
func runAuditHealth(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		writef(stderr, "audit health takes no arguments, got: %v\n", args)

		return ExitUsageError
	}

	return runAuditTool(toolAuditHealth, map[string]any{}, stdout, stderr)
}

// runAuditExport maps --format/--tool/--since into linode_audit_export.
// format is required by the tool; an empty value is left off so the
// tool's own validation produces the error message.
func runAuditExport(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("audit export", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		format string
		tool   string
		since  string
	)

	flags.StringVar(&format, "format", "", "output format: json, csv, or ndjson")
	flags.StringVar(&tool, "tool", "", "only events whose tool matches this glob")
	flags.StringVar(&since, "since", "", "only events at or after this RFC 3339 timestamp")

	if err := flags.Parse(args); err != nil {
		return ExitUsageError
	}

	arguments := map[string]any{}
	putString(arguments, auditArgFormat, format)
	putString(arguments, auditArgTool, tool)
	putString(arguments, auditArgSince, since)

	return runAuditTool(toolAuditExport, arguments, stdout, stderr)
}

// runAuditTool builds a one-shot runtime, dispatches one audit tool call
// through the shared path, prints the result payload, and maps it to an
// exit code (0 success, 1 on a tool error or transport fault). The audit
// tools return JSON, so the payload prints verbatim.
func runAuditTool(tool string, arguments map[string]any, stdout, stderr io.Writer) int {
	ctx := context.Background()

	runtime, err := newRuntime(ctx, stderr)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}
	defer runtime.Close()

	return dispatchAndPrint(ctx, runtime.Server, tool, arguments, stdout, stderr)
}

// dispatchAndPrint runs one tools/call and prints the payload, returning
// the exit code. Shared by the audit subcommands; kept separate from
// runAuditTool so the runtime lifecycle and the dispatch stay readable.
func dispatchAndPrint(
	ctx context.Context,
	srv *server.Server,
	tool string,
	arguments map[string]any,
	stdout, stderr io.Writer,
) int {
	result, err := dispatchCall(ctx, srv, tool, arguments)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}

	writeln(stdout, result.Text)

	if result.IsError {
		writeln(stderr, "audit query returned an error result")

		return 1
	}

	return 0
}

// putString sets key only when value is non-empty, so an unset flag stays
// off the request and the tool's default applies.
func putString(args map[string]any, key, value string) {
	if value != "" {
		args[key] = value
	}
}

// putInt sets key only when value is non-zero, matching the audit tools'
// "0 means unset" convention for limit-style fields.
func putInt(args map[string]any, key string, value int) {
	if value != 0 {
		args[key] = value
	}
}
