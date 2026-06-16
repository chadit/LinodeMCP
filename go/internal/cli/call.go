package cli

import (
	"context"
	"flag"
	"io"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/server"
)

// stringList is a flag.Value that accumulates repeated --arg occurrences
// into a slice, so `--arg a=1 --arg b=2` yields two entries.
type stringList []string

// String renders the accumulated values for flag's usage output.
func (*stringList) String() string {
	return ""
}

// Set appends one occurrence's value, called once per flag instance.
func (s *stringList) Set(value string) error {
	*s = append(*s, value)

	return nil
}

const callUsage = `Usage: linodemcp call <tool> [flags]

Run one tool through the same dispatch the MCP server uses: the call is
validated, audited, and subject to profile, dry-run, and two-stage rules
exactly as an MCP client's call would be.

Arguments (mutually exclusive):
  --arg key=value     Set one argument; repeatable. Values are typed from
                      the tool's schema (number, integer, boolean, string).
  --json '{...}'      Supply all arguments as one JSON object.

Safety flags (folded into the request like the MCP fields):
  --dry-run           Preview without executing (sets dry_run).
  --confirm           Confirm a write/destroy (sets confirm).
  --mode plan|apply   Two-stage destroy mode.
  --plan-id <id>      Plan id from a prior --mode plan, for --mode apply.
  --confirmed-dry-run Acknowledge a dry-run preview before a real run.
  --yolo              Bypass confirm where the profile allows it.
  --environment <name> Target a named environment.

Output:
  --output json       Print the tool's result payload (default).
  --output table      Render a list/object result as a text table.

Discover tools with "linodemcp tools" and "linodemcp tools show <tool>".`

// RunCallCommand runs `linodemcp call <tool> [flags]` and returns the
// process exit code: 0 on success, 1 when the tool returns an error
// result, 2 for any usage problem (no tool, unknown tool, bad flags,
// bad JSON, mutually-exclusive args). Output streams are parameters so
// tests capture stdout and stderr.
//
// The command builds a tools/call request and drives it through the
// server's HandleMessage. It reimplements no tool logic: validation,
// the confirm gate, dry-run, two-stage, audit, and profile filtering all
// happen inside the dispatch.
func RunCallCommand(args []string, stdout, stderr io.Writer) int {
	parsed, code, done := parseCallArgs(args, stderr)
	if done {
		return code
	}

	ctx := context.Background()

	runtime, err := newRuntime(ctx, stderr)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}
	defer runtime.Close()

	if exit, handled := rejectUnknownTool(runtime.Server, parsed.tool, stderr); handled {
		return exit
	}

	schema := schemaForTool(runtime.Server, parsed.tool)

	arguments, err := BuildArguments(schema, parsed.jsonArg, parsed.kvArgs)
	if err != nil {
		writef(stderr, "%v\n", err)

		return ExitUsageError
	}

	ApplySafetyFlags(arguments, &parsed.safety)

	return executeCall(ctx, runtime.Server, &parsed, arguments, stdout, stderr)
}

// parsedCall holds the flag-parse output for a call invocation.
type parsedCall struct {
	tool    string
	jsonArg string
	kvArgs  []string
	output  string
	safety  SafetyFlags
}

// parseCallArgs parses the call flags. The third return reports whether
// the caller should stop (true) and use the returned exit code, which
// covers both the no-tool usage error and a -h/--help request.
func parseCallArgs(args []string, stderr io.Writer) (parsedCall, int, bool) {
	if len(args) == 0 {
		writeln(stderr, callUsage)

		return parsedCall{}, ExitUsageError, true
	}

	tool := args[0]

	flags := flag.NewFlagSet("call", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { writeln(stderr, callUsage) }

	var (
		kvArgs stringList
		parsed parsedCall
	)

	flags.StringVar(&parsed.jsonArg, "json", "", "all arguments as one JSON object")
	flags.Var(&kvArgs, "arg", "one argument as key=value (repeatable)")
	flags.StringVar(&parsed.output, "output", outputJSON, "output format: json or table")
	bindSafetyFlags(flags, &parsed.safety)

	if err := flags.Parse(args[1:]); err != nil {
		// flag already printed the error and usage; classify as usage.
		return parsedCall{}, ExitUsageError, true
	}

	parsed.tool = tool
	parsed.kvArgs = kvArgs

	if !validOutput(parsed.output) {
		writef(stderr, "unknown --output %q: want json or table\n", parsed.output)

		return parsedCall{}, ExitUsageError, true
	}

	return parsed, 0, false
}

// bindSafetyFlags registers the safety flags on a flag set, wiring the
// bool ones through tri-state pointers so an unset flag stays absent from
// the request. flag's BoolVar can't express "unset", so each bool is
// captured via a bool pointer that flag's Func sets only when present.
func bindSafetyFlags(flags *flag.FlagSet, safety *SafetyFlags) {
	bindOptionalBool(flags, "dry-run", &safety.DryRun)
	bindOptionalBool(flags, "confirm", &safety.Confirm)
	bindOptionalBool(flags, "confirmed-dry-run", &safety.ConfirmedDry)
	bindOptionalBool(flags, "yolo", &safety.Yolo)

	flags.StringVar(&safety.Mode, "mode", "", "two-stage mode: plan or apply")
	flags.StringVar(&safety.PlanID, "plan-id", "", "plan id for --mode apply")
	flags.StringVar(&safety.Environment, "environment", "", "target environment name")
}

// bindOptionalBool registers a boolean flag whose presence is tracked.
// When the flag appears the target pointer is set so ApplySafetyFlags
// writes the field; when it is absent the pointer stays nil and the field
// is left off the request, so a tool's own default applies.
//
// flag.BoolFunc passes "true" for the bare --flag form and the literal
// value for --flag=<value>, so parseBoolFlag handles both and rejects a
// non-boolean value as a usage error.
func bindOptionalBool(flags *flag.FlagSet, name string, target **bool) {
	flags.BoolFunc(name, "", func(raw string) error {
		value, err := parseBoolFlag(raw)
		if err != nil {
			return err
		}

		*target = &value

		return nil
	})
}

// executeCall dispatches the request, prints the payload, and maps the
// result to an exit code. A transport/RPC error is exit 1 with the error
// on stderr; a tool IsError result is exit 1 with the message on stderr
// and the payload still printed to stdout; success is exit 0.
func executeCall(
	ctx context.Context,
	srv *server.Server,
	parsed *parsedCall,
	arguments map[string]any,
	stdout, stderr io.Writer,
) int {
	result, err := dispatchCall(ctx, srv, parsed.tool, arguments)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}

	renderOutput(stdout, result.Text, parsed.output)

	if result.IsError {
		writeln(stderr, "tool returned an error result")

		return 1
	}

	return 0
}

// rejectUnknownTool checks the requested tool against the full catalog
// and, when it is not a registered tool, prints a helpful message and
// reports handled=true with exit code 2. A tool that exists but is hidden
// by the active profile is reported distinctly so the user knows to
// switch profiles rather than think the tool doesn't exist.
func rejectUnknownTool(srv *server.Server, tool string, stderr io.Writer) (int, bool) {
	for _, info := range srv.AllToolInfos() {
		if info.Name == tool {
			return 0, false
		}
	}

	writef(stderr, "unknown tool %q.\n", tool)
	writeln(stderr, `Run "linodemcp tools --all" to list every tool.`)

	return ExitUsageError, true
}

// schemaForTool returns the input schema for tool from the active-profile
// view, falling back to the full catalog. The active view is preferred so
// coercion reflects what this profile exposes, but a tool present only in
// the full catalog (allowed by rejectUnknownTool's broader check) still
// gets its schema for argument typing.
func schemaForTool(srv *server.Server, tool string) mcp.ToolInputSchema {
	for _, info := range srv.ToolInfos() {
		if info.Name == tool {
			return info.InputSchema
		}
	}

	for _, info := range srv.AllToolInfos() {
		if info.Name == tool {
			return info.InputSchema
		}
	}

	return mcp.ToolInputSchema{}
}

// validOutput reports whether the --output value is one the renderer
// understands.
func validOutput(format string) bool {
	return format == outputJSON || format == outputTable
}

// parseBoolFlag parses an explicit boolean flag value, returning a usage
// error for anything that isn't a recognized bool literal.
func parseBoolFlag(raw string) (bool, error) {
	switch raw {
	case "true", "1", "t", "T", "TRUE", "True":
		return true, nil
	case "false", "0", "f", "F", "FALSE", "False":
		return false, nil
	default:
		return false, errInvalidBool
	}
}
