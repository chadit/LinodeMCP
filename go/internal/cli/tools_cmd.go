package cli

import (
	"context"
	"io"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/server"
)

const toolsUsage = `Usage: linodemcp tools [--all]
       linodemcp tools show <tool>

List the tools the CLI can call, or show one tool's argument schema.

  (no args)     List tools the active profile exposes, with capability.
  --all         List every tool in the catalog, ignoring the profile.
  show <tool>   Print a tool's description, capability, and arguments.`

// Column widths for the `tools` listing. Extracted so the rows and any
// future header stay in sync and the numbers aren't bare literals.
const (
	toolsColName       = 40
	toolsColCapability = 10
)

// RunToolsCommand dispatches `linodemcp tools [...]` and returns the exit
// code. With no args (or --all) it lists tools; with `show <tool>` it
// prints one tool's schema. Output streams are parameters for tests.
func RunToolsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "show" {
		return runToolsShow(args[1:], stdout, stderr)
	}

	return runToolsList(args, stdout, stderr)
}

// runToolsList prints one line per tool: name then capability. The
// default view is the active-profile surface (what `call` can actually
// run right now); --all prints the full catalog so a user can see tools
// gated behind another profile.
func runToolsList(args []string, stdout, stderr io.Writer) int {
	var all bool

	for _, arg := range args {
		switch arg {
		case "--all", "-all":
			all = true
		default:
			writef(stderr, "unknown argument %q\n\n%s\n", arg, toolsUsage)

			return ExitUsageError
		}
	}

	runtime, err := newRuntime(context.Background(), stderr)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}
	defer runtime.Close()

	infos := runtime.Server.ToolInfos()
	if all {
		infos = runtime.Server.AllToolInfos()
	}

	printToolList(stdout, infos)

	return 0
}

// printToolList writes the sorted name+capability table. Sorting by name
// keeps the output stable across runs (the catalog order is not).
func printToolList(stdout io.Writer, infos []server.ToolInfo) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	for i := range infos {
		writef(
			stdout,
			"%-*s %-*s\n",
			toolsColName, infos[i].Name,
			toolsColCapability, infos[i].Capability.String(),
		)
	}
}

// runToolsShow prints one tool's description, capability, and argument
// schema (each property's name, type, and whether it is required). The
// tool is matched against the full catalog so a user can inspect a tool
// even when the active profile hides it.
func runToolsShow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeln(stderr, "Usage: linodemcp tools show <tool>")

		return ExitUsageError
	}

	tool := args[0]

	runtime, err := newRuntime(context.Background(), stderr)
	if err != nil {
		writef(stderr, "%v\n", err)

		return 1
	}
	defer runtime.Close()

	info, found := findToolInfo(runtime.Server, tool)
	if !found {
		writef(stderr, "unknown tool %q.\n", tool)
		writeln(stderr, `Run "linodemcp tools --all" to list every tool.`)

		return ExitUsageError
	}

	printToolDetail(stdout, runtime.Server, &info)

	return 0
}

// findToolInfo locates a tool by name in the full catalog, returning the
// ToolInfo and whether it was found.
func findToolInfo(srv *server.Server, tool string) (server.ToolInfo, bool) {
	for _, info := range srv.AllToolInfos() {
		if info.Name == tool {
			return info, true
		}
	}

	return server.ToolInfo{}, false
}

// printToolDetail writes a tool's name, capability, description, and the
// argument schema. The description is fetched from the live tool
// definition; the schema's properties are printed sorted with their type
// and a required marker. info is taken by pointer to dodge gocritic's
// hugeParam.
func printToolDetail(stdout io.Writer, srv *server.Server, info *server.ToolInfo) {
	writef(stdout, "Tool: %s\n", info.Name)
	writef(stdout, "Capability: %s\n", info.Capability.String())

	if description := toolDescription(srv, info.Name); description != "" {
		writef(stdout, "Description: %s\n", description)
	}

	writef(stdout, "Arguments (%d):\n", len(info.InputSchema.Properties))

	required := requiredSet(info.InputSchema.Required)

	for _, name := range sortedPropertyNames(info.InputSchema.Properties) {
		var marker string
		if _, ok := required[name]; ok {
			marker = " (required)"
		}

		writef(stdout, "  %-28s %s%s\n", name, propertyType(info.InputSchema, name), marker)
	}
}

// toolDescription returns the live description for tool from the server's
// registered tool definitions. ToolInfo carries only capability and
// schema, so the description comes from the contracts.Tool set. That set
// is profile-filtered, so a tool the active profile hides returns "";
// the caller skips an empty description but still prints capability and
// the argument schema (both available for the full catalog).
func toolDescription(srv *server.Server, tool string) string {
	for _, registered := range srv.Tools() {
		if registered.Name() == tool {
			return registered.Description()
		}
	}

	return ""
}

// requiredSet turns the schema's required-name slice into a set for O(1)
// membership checks while printing each property.
func requiredSet(required []string) map[string]struct{} {
	set := make(map[string]struct{}, len(required))
	for _, name := range required {
		set[name] = struct{}{}
	}

	return set
}

// sortedPropertyNames returns the schema property names in sorted order
// so `tools show` output is stable.
func sortedPropertyNames(properties map[string]any) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// propertyType returns the JSON-schema type for a property, defaulting
// to "string" when the schema leaves it unspecified (the same permissive
// default the coercion uses).
func propertyType(schema mcp.ToolInputSchema, name string) string {
	if declared := schemaPropType(schema, name); declared != "" {
		return declared
	}

	return "string"
}
