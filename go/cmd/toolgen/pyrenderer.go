package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// pyRenderer emits the Python tool factories, handlers, and registry.
//
// outDir is where the tree lands, which the formatter needs to resolve the same
// configuration the linter will read the written files under; ruff is the same
// formatter the repo lints with, so the emitted tree passes the format gate
// hand-written code does.
type pyRenderer struct {
	outDir string
	ruff   string
}

func (pyRenderer) owns(name string) bool {
	return strings.HasSuffix(name, pySuffix)
}

func (pyRenderer) language() string {
	return languagePython
}

// Where a Python run acts the options its emitted tree carries nothing for.
// This arm hands the driver the tool name and lets it read the declaration off
// the descriptor when the call is made, so the prose and the policies below are
// looked up rather than written into each factory.
const (
	pyDriverFile      = "python/src/linodemcp/tools/drivers.py"
	pyRoutePolicyFile = "python/src/linodemcp/linode/routes.py"
)

// acts is this arm's answer for every option the contract declares. The split
// between the two lists is the real difference between the languages: Go writes
// these sentences and policies into the factory, and Python resolves them per
// call, so the same contract reaches the caller either way.
func (pyRenderer) acts() []optionClaim {
	emitted := []protoreflect.Name{
		"argument_reader", "body_comma_list", "body_constant", "body_fold",
		"body_name", "body_nullable", "body_root", "echo_argument",
		"field_location", "list_envelope", "list_filter", "normalize_fields",
		"object_walk", "preview_omits_body", "preview_redact",
		"preview_sentence", "reader_message", "reader_values",
		"refuse_arguments", "refuse_unknown_arguments", "require_any_of",
		"response_body_fields", "state_route", "tool_api_surface",
		"tool_capability", "tool_description", "tool_hooks", "tool_meta",
		"tool_response", "tool_route",
	}

	served := map[protoreflect.Name]string{
		"confirm_message":      pyDriverFile,
		"error_message":        pyDriverFile,
		"explicit_null_fields": pyDriverFile,
		"resource_type":        pyDriverFile,
		"success_message":      pyDriverFile,
		"warning_message":      pyDriverFile,
		"retry_disabled":       pyRoutePolicyFile,
	}

	claims := make([]optionClaim, 0, len(emitted)+len(served))
	for _, option := range emitted {
		claims = append(claims, optionClaim{option: option, emitted: true, home: ""})
	}

	for option, home := range served {
		claims = append(claims, optionClaim{option: option, emitted: false, home: home})
	}

	return claims
}

func (p pyRenderer) renderGroup(group string, tools []*contract) (emittedFile, error) {
	ordered := slices.Clone(tools)
	slices.SortFunc(ordered, func(left, right *contract) int {
		return strings.Compare(left.Name, right.Name)
	})

	text, err := pyRenderModule(ordered)
	if err != nil {
		return emittedFile{}, err
	}

	name := group + pySuffix

	finished, err := p.finish(text, name)
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: name, Text: finished}, nil
}

func (p pyRenderer) renderRegistry(contracts []contract) (emittedFile, error) {
	text := pyRegistrySource(contracts)

	finished, err := p.finish(text, "__init__"+pySuffix)
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: "__init__" + pySuffix, Text: finished}, nil
}

// registrySource is the package module the server reads its generated tools
// from. A generated tool nothing registers would pass every gate that reads the
// contract and still be missing from the running server, so the registration is
// emitted from the same cohort as the factories.
func pyRegistrySource(contracts []contract) string {
	byModule := make(map[string][]string, len(contracts))
	exported := make([]string, 0, len(contracts)*2)

	ordered := make([]*contract, 0, len(contracts))
	for i := range contracts {
		ordered = append(ordered, &contracts[i])
	}

	slices.SortFunc(ordered, func(left, right *contract) int {
		return strings.Compare(left.Name, right.Name)
	})

	for _, built := range ordered {
		module := groupName(built.SourceFile)
		names := []string{pyFactoryName(built.Name), pyHandlerName(built.Name)}
		byModule[module] = append(byModule[module], names...)
		exported = append(exported, names...)
	}

	lines := make([]string, 0, len(byModule)+12)
	lines = append(lines,
		pyHeader,
		`"""The tool factories go/cmd/toolgen emits from the proto contract.`,
		"",
		"Nothing here is hand-written and nothing here survives a `make proto`:",
		"change the contract, not this tree.",
		`"""`,
		"",
		"from __future__ import annotations",
		"",
	)

	modules := make([]string, 0, len(byModule))
	for module := range byModule {
		modules = append(modules, module)
	}

	sort.Strings(modules)

	for _, module := range modules {
		names := slices.Clone(byModule[module])
		sort.Strings(names)

		indented := make([]string, 0, len(names))
		for _, name := range names {
			indented = append(indented, "    "+name)
		}

		lines = append(lines, fmt.Sprintf("from %s.%s import (\n%s,\n)",
			pyPackage, module, strings.Join(indented, ",\n")))
	}

	sort.Strings(exported)

	listed := make([]string, 0, len(exported))
	for _, name := range exported {
		listed = append(listed, "    "+pyQuote(name))
	}

	lines = append(lines, "", "__all__ = [\n"+strings.Join(listed, ",\n")+",\n]", "")

	return strings.Join(lines, "\n")
}

// renderModule is one emitted module: the factories and handlers of one proto
// file.
func pyRenderModule(contracts []*contract) (string, error) {
	tools := make([]*pyTool, 0, len(contracts))

	for _, built := range contracts {
		tool, err := newPyTool(built)
		if err != nil {
			return "", err
		}

		tools = append(tools, tool)
	}

	body, err := pyModuleBody(tools)
	if err != nil {
		return "", err
	}

	sources := make([]string, 0, len(contracts))

	for _, built := range contracts {
		if !slices.Contains(sources, built.SourceFile) {
			sources = append(sources, built.SourceFile)
		}
	}

	sort.Strings(sources)

	imports, err := pyModuleImports(tools)
	if err != nil {
		return "", err
	}

	head := []string{
		pyHeader,
		`"""Generated tool factories for ` + strings.Join(sources, ", ") + `."""`,
		"",
		"from __future__ import annotations",
		"",
		"from typing import TYPE_CHECKING, Any",
		"",
		"from mcp.types import TextContent, Tool",
		"",
	}
	head = append(head, imports...)
	head = append(head, "", "if TYPE_CHECKING:")
	head = append(head, pyTypeChecking(tools)...)
	head = append(head, "", "")

	return strings.Join(append(append(head, body[:len(body)-2]...), ""), "\n"), nil
}

// pyModuleBody is the factories, body builders, and handlers of one module, in
// tool order.
func pyModuleBody(tools []*pyTool) ([]string, error) {
	body := make([]string, 0, len(tools)*32)

	for _, tool := range tools {
		factory, err := pyFactory(tool)
		if err != nil {
			return nil, err
		}

		body = append(body, factory...)
		body = append(body, "", "")

		if tool.buildsBody() {
			builder, bodyErr := pyBody(tool)
			if bodyErr != nil {
				return nil, bodyErr
			}

			body = append(body, builder...)
			body = append(body, "", "")
		}

		handler, err := pyHandler(tool)
		if err != nil {
			return nil, err
		}

		body = append(body, handler...)
		body = append(body, "", "")
	}

	return body, nil
}

// pyTypeChecking names the types the emitted annotations reference and nothing
// else imports. A destroy hands the driver closures over the client and the
// preview details, and both are annotated; a staged mutation hands over the
// same two, since a plan reads state and walks it before it is stored, and so
// does any tool whose declared preview reads its own state.
func pyTypeChecking(tools []*pyTool) []string {
	lines := []string{"    from linodemcp.config import Config"}

	staged := make([]*pyTool, 0, len(tools))

	for _, tool := range tools {
		if tool.c.Tier == tierDestroy || tool.c.Mode {
			staged = append(staged, tool)
		}
	}

	if len(staged) > 0 || slices.ContainsFunc(tools, func(tool *pyTool) bool {
		return tool.c.previewReadsState()
	}) {
		lines = append(lines, "    from linodemcp.linode import RetryableClient")
	}

	if slices.ContainsFunc(staged, func(tool *pyTool) bool {
		return tool.hook(hookKindDependencyWalk) != ""
	}) {
		return append(lines, "    from linodemcp.tools.helpers import DryRunDetails")
	}

	return lines
}

// pyFactory is the factory: the tool a client sees, and the tier it registers
// at.
func pyFactory(tool *pyTool) ([]string, error) {
	capability, err := tool.capabilityMember()
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, 12)
	lines = append(lines,
		"def "+pyFactoryName(tool.c.Name)+"() -> tuple[Tool, Capability]:",
		`    """Build `+tool.c.Name+".",
		"",
	)
	lines = append(lines, pyRouteDoc(tool, pyDocIndent)...)
	lines = append(lines,
		`    """`,
		"    return Tool(",
		"        name="+pyQuote(tool.c.Name)+",",
		"        description=(",
	)
	lines = append(lines, pyQuoteWrapped(tool.c.Description, pyProseIndent)...)
	lines = append(lines,
		"        ),",
		"        input_schema=schema("+pyQuote(tool.c.InputMessage)+"),",
		"    ), Capability."+capability,
	)

	return lines, nil
}

// pyRouteDoc is the "Calls ..." sentence, moved onto a second line when it does
// not fit. A route nested three deep runs past the line budget with the method
// on the same line and ruff cannot rewrap prose, so the split happens here
// rather than failing the run.
func pyRouteDoc(tool *pyTool, indent int) []string {
	pad := strings.Repeat(" ", indent)

	if tool.c.Tier == tierMeta {
		return []string{pad + "Answers from local state and reaches no Linode route."}
	}

	single := pad + "Calls " + tool.routeText() + "."
	if len(single) <= pyLineBudget {
		return []string{single}
	}

	return []string{pad + "Calls " + tool.c.Method, pad + tool.c.PathTemplate + "."}
}

// finish formats the rendered text, then holds it to the line budget. ruff
// decides where the code breaks, the same way the Go arm leans on go/format.
// Prose is what ruff cannot rewrap, so a line still over the budget after
// formatting is a sentence too long to render and fails by name.
func (p pyRenderer) finish(text, name string) (string, error) {
	path := filepath.Join(p.outDir, name)

	formatted, err := p.formatted(text, path)
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(formatted, "\n") {
		if len(line) > pyLineBudget {
			return "", fmt.Errorf("%w: emitted line over %d columns in %s: %q",
				errPyLineBudget, pyLineBudget, path, line)
		}
	}

	return formatted, nil
}

// formatted is rendered text as the repo's formatter would write it. The file
// name goes in so ruff resolves the same configuration it would for a file at
// that path, rather than its defaults.
func (p pyRenderer) formatted(text, path string) (string, error) {
	// Fixed argv, no shell; the formatter and the path are both repo-owned.
	command := exec.CommandContext(context.Background(), p.ruff, "format", "--stdin-filename", path, "-") // #nosec G204 -- repo-owned formatter at the path the build passes
	command.Stdin = strings.NewReader(text)

	var stderr strings.Builder

	command.Stderr = &stderr

	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%w: formatting %s failed: %s",
			errPyFormat, path, strings.TrimSpace(stderr.String()))
	}

	return string(out), nil
}

// pyDriver is the driver one tier runs through, spelled out so a new tier fails
// here rather than reaching a plausible-looking wrong one. The meta tier has
// none because it opens no client: its answer is local state.
func pyDriver(runs tier) (string, bool) {
	switch runs {
	case tierGet:
		return pyGetDriver, true
	case tierList, tierSubresourceList, tierMarkerList:
		return pyListDriver, true
	case tierBodyRead:
		return "run_body_read_tool", true
	case tierWrite:
		return "run_write_tool", true
	case tierAcknowledge:
		return "run_acknowledge_tool", true
	case tierDestroy:
		return "run_destructive_tool", true
	case tierMeta, tierUnknown:
	}

	return "", false
}

// The two drivers a read runs through, named here because the tier table and
// the call site both spell them.
const (
	pyGetDriver  = "run_get_tool"
	pyListDriver = "run_list_tool"
)

// pyModuleImports is the first-party imports the rendered module takes: only
// what the tiers present actually use, so an unused import cannot reach the
// emitted tree. Sorted before returning because ruff format leaves import order
// alone and linodemcp.toolhooks sorts ahead of linodemcp.tools.
func pyModuleImports(tools []*pyTool) ([]string, error) {
	drivers, err := pyDriverImports(tools)
	if err != nil {
		return nil, err
	}

	lines := []string{"from linodemcp.profiles import Capability"}

	// Only where a declared fetch feeds a walk: the reader refuses any other
	// state, so a module whose walks all pair with hand-written fetches never
	// names it.
	if slices.ContainsFunc(tools, func(tool *pyTool) bool {
		return tool.c.StateRead.declared() && tool.hook(hookKindDependencyWalk) != ""
	}) {
		lines = append(lines, "from linodemcp.tools.declared_state import declared_state_of")
	}

	if slices.ContainsFunc(tools, (*pyTool).buildsBody) {
		names, bodyErr := pyBodyImports(tools)
		if bodyErr != nil {
			return nil, bodyErr
		}

		lines = append(lines, "from linodemcp.tools.body import "+strings.Join(names, ", "))
	}

	if len(drivers) > 0 {
		lines = append(lines, "from linodemcp.tools.drivers import "+strings.Join(drivers, ", "))
	}

	helpers, err := pyHelperImports(tools)
	if err != nil {
		return nil, err
	}

	if helpers != "" {
		lines = append(lines, "from linodemcp.tools.helpers import "+helpers)
	}

	readers, err := pySlotReaders(tools)
	if err != nil {
		return nil, err
	}

	if account := pyNamedIn(readers, pyAccountModuleReaders); len(account) > 0 {
		lines = append(lines, "from linodemcp.tools.linode_account import "+strings.Join(account, ", "))
	}

	lines = append(lines, "from linodemcp.tools.constraints import check as check_constraints")

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return tool.c.Walks }) {
		lines = append(lines, "from linodemcp.tools.objectwalk import walk as walk_objects")
	}

	if names := pyPreviewImports(tools); names != "" {
		lines = append(lines, "from linodemcp.tools.preview import "+names)
	}

	lines = append(lines, "from linodemcp.tools.toolschemas import schema")

	if hooks := pyHookImports(tools); hooks != "" {
		lines = append(lines, "from "+pyHooksModule+" import "+hooks)
	}

	sort.Strings(lines)

	return lines, nil
}

// pyDriverImports is what a module takes from the driver layer: one name per
// tier present, plus the pieces a list or a synthesized state fetch spells.
func pyDriverImports(tools []*pyTool) ([]string, error) {
	named := make(map[string]bool, len(tools))

	for _, tool := range tools {
		if tool.c.Tier == tierMeta {
			continue
		}

		if tool.assembledRead() {
			named["run_assembled_read_tool"] = true

			continue
		}

		driver, known := pyDriver(tool.c.Tier)
		if !known {
			return nil, fmt.Errorf("%w: %s", errUnsupportedTier, tool.c.Name)
		}

		named[driver] = true
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return len(tool.c.Filters) > 0 }) {
		named["ListFilter"] = true
	}

	// MatchMode names the equality mode only. A module whose filters all match
	// by substring never spells it, since that is the ListFilter default.
	if slices.ContainsFunc(tools, func(tool *pyTool) bool {
		return slices.ContainsFunc(tool.c.Filters, func(entry listFilter) bool {
			return !pyFilterContains(entry)
		})
	}) {
		named["MatchMode"] = true
	}

	if slices.ContainsFunc(tools, (*pyTool).forwardedQuery) {
		named["with_query_arguments"] = true
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool {
		return tool.c.StateRead.declared() && !tool.c.StateRead.collection()
	}) {
		named["read_route_state"] = true
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return tool.c.StateRead.collection() }) {
		named["read_collection_state"] = true
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return len(tool.c.StateRead.Query) > 0 }) {
		named["state_read_query"] = true
	}

	return pySortedKeys(named), nil
}

// pyHookImports is the hook functions a rendered module calls, named directly
// so a missing one fails at import rather than on a tool call.
func pyHookImports(tools []*pyTool) string {
	named := make(map[string]bool, len(tools))

	for _, tool := range tools {
		for _, name := range tool.hookNames() {
			named[name] = true
		}
	}

	return strings.Join(pySortedKeys(named), ", ")
}

// pySortedKeys is a name set in sorted order.
func pySortedKeys(named map[string]bool) []string {
	found := make([]string, 0, len(named))
	for name := range named {
		found = append(found, name)
	}

	sort.Strings(found)

	return found
}

// pyNamedIn is the sorted names a set shares with a space-bounded group.
func pyNamedIn(named map[string]bool, group string) []string {
	found := make([]string, 0, len(named))

	for name := range named {
		if strings.Contains(group, " "+name+" ") {
			found = append(found, name)
		}
	}

	sort.Strings(found)

	return found
}

// pyFilterContains reports whether one filter matches by substring, which is
// the ListFilter default and the one mode a literal leaves unnamed.
func pyFilterContains(entry listFilter) bool {
	return entry.Match == linodev1.ListFilterSpec_MATCH_CONTAINS
}

// pyPreviewImports is what a module takes from the preview support layer: the
// sentence chooser for every tool declaring prose, and the argument reader only
// for the ones whose wordings read a value.
func pyPreviewImports(tools []*pyTool) string {
	named := make([]string, 0, 2)

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return tool.c.previewFillsWording() }) {
		named = append(named, "preview_sentence")
	}

	named = append(named, pyPreviewChoiceImports(tools)...)
	named = append(named, pyPreviewReaders(tools)...)

	sort.Strings(named)

	return strings.Join(named, ", ")
}

// pyPreviewChoiceImports is what a module takes for the lines a flag selects:
// the chooser, and the flag read each of those lines spells.
func pyPreviewChoiceImports(tools []*pyTool) []string {
	named := make([]string, 0, 3)

	for _, tool := range tools {
		for _, choice := range tool.c.previewChoices() {
			read := "preview_flag"
			if choice.Member != "" {
				read = "preview_member_flag"
			}

			for _, name := range []string{"preview_chosen", read} {
				if !slices.Contains(named, name) {
					named = append(named, name)
				}
			}
		}
	}

	return named
}

// pyPreviewReaders is the argument readers a module's declared wordings use,
// sorted so the import line reads the way ruff leaves it.
func pyPreviewReaders(tools []*pyTool) []string {
	named := make([]string, 0, 2)

	for _, tool := range tools {
		for _, name := range tool.c.previewValueArguments() {
			reader := pyPreviewReader(tool.c.previewArgument(name))
			if !slices.Contains(named, reader) {
				named = append(named, reader)
			}
		}
	}

	sort.Strings(named)

	return named
}
