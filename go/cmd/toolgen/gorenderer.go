package main

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const generatedPackage = "gentools"

// generatedSuffix marks the files this renderer owns. Everything under the
// output directory carrying it is removed before a run.
const generatedSuffix = ".gen.go"

// goRenderer emits the Go tool factories, handlers, and registry.
type goRenderer struct{}

func (goRenderer) owns(name string) bool {
	return strings.HasSuffix(name, generatedSuffix)
}

func (goRenderer) language() string {
	return languageGo
}

// Where a Go run acts the two options it emits nothing for. Both are read off
// the descriptor when the call is made: a factory carrying the retry policy
// would say the same thing twice, and the refusals have to reach the list
// drivers, which no emitted literal reaches.
const (
	goRoutePolicyFile = "go/internal/linoderoute/policy.go"
	goRefusalFile     = "go/internal/tools/argument_refusals.go"
)

// acts is this arm's answer for every option the contract declares. Nearly all
// of it is emitted, since this arm writes the prose and the checks into the
// tool rather than looking them up at call time the way the Python driver does.
func (goRenderer) acts() []optionClaim {
	emitted := []protoreflect.Name{
		"argument_reader", "body_comma_list", "body_constant", "body_fold",
		"body_name", "body_nullable", "body_root", "confirm_message",
		"echo_argument", "error_message", "explicit_null_fields",
		"field_location", "list_envelope", "list_filter", "local_answer", "local_operation", "local_record", "normalize_fields", "normalize_fold", "state_composite", "dependency_walk", "billing_delta",
		"object_walk", "preview_omits_body", "preview_redact",
		"preview_sentence", "preview_stand_in", "preview_unchanged", "reader_message", "reader_values",
		"require_any_of",
		"resource_type", "response_body_fields", "execute_transport", "state_route",
		"success_message", "tool_api_surface", "tool_capability",
		"tool_categories", "tool_description", "tool_meta",
		"tool_response", "tool_route", "tool_scopes", "warning_message",
	}

	served := map[protoreflect.Name]string{
		"refuse_arguments": goRefusalFile,
		"retry_disabled":   goRoutePolicyFile,
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

// renderAnswers writes the value type each declared answer shape fills, the
// projection that turns one into its plain body, and the surface every
// generated operation's subsystem is reached through.
func (goRenderer) renderAnswers(
	shapes []answerShape, operations []localOperation,
) (emittedFile, error) {
	return renderGoAnswers(shapes, operations)
}

// renderOperations writes the arm behind every generated operation.
func (goRenderer) renderOperations(operations []localOperation) ([]emittedFile, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	file, err := renderGoOperations(operations)
	if err != nil {
		return nil, err
	}

	return []emittedFile{file}, nil
}

func (goRenderer) renderGroup(group string, tools []*contract) (emittedFile, error) {
	out := newSource()

	for _, tool := range tools {
		if err := emitTool(out, tool); err != nil {
			return emittedFile{}, err
		}
	}

	formatted, err := formatSource(out.render(generatedPackage), group)
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: group + generatedSuffix, Text: formatted}, nil
}

// renderRegistry writes the slice the server appends to its own factory lists.
func (goRenderer) renderRegistry(contracts []contract) (emittedFile, error) {
	names := make([]string, 0, len(contracts))
	for i := range contracts {
		names = append(names, factoryName(contracts[i].Name))
	}

	sort.Strings(names)

	out := newSource()
	out.need(importConfig, importMCP, importProfiles, importTools)

	out.writef("// Package gentools holds the tool factories cmd/toolgen emits from the")
	out.writef("// linode.mcp.v1 proto contract. Nothing here is hand-written and nothing here")
	out.writef("// survives a `make proto`: change the contract, not this tree.")
	out.writef("package %s", generatedPackage)
	out.writef("")
	out.writef("// Factory is the shape the server registers a tool through.")
	out.writef("type Factory func(*config.Config) (mcp.Tool, profiles.Capability, tools.Handler)")
	out.writef("")
	out.writef("// Factories lists every generated tool factory, tool-name sorted.")
	out.writef("func Factories() []Factory {")
	out.writef("\treturn []Factory{")

	for _, name := range names {
		out.writef("\t\t%s,", name)
	}

	out.writef("\t}")
	out.writef("}")
	out.writef("")
	out.writef("// ScopesFor answers the OAuth scope strings the contract declares for one")
	out.writef("// tool, nil for a meta tool and for one documented scopeless.")
	out.writef("func ScopesFor(name string) []string {")
	out.writef("\treturn toolScopes[name]")
	out.writef("}")
	out.writef("")
	out.writef("// toolScopes is each declaring tool's scope list, declaration order.")
	out.writef("var toolScopes = map[string][]string{")

	for _, entry := range scopedContracts(contracts) {
		out.writef("\t%q: {%s},", entry.Name, quotedStrings(entry.Scopes))
	}

	out.writef("}")
	out.writef("")
	out.writef("// CategoriesFor answers the profile categories the contract declares for")
	out.writef("// one tool, first one its primary grouping, nil for a declared none.")
	out.writef("func CategoriesFor(name string) []string {")
	out.writef("\treturn toolCategories[name]")
	out.writef("}")
	out.writef("")
	out.writef("// toolCategories is each declaring tool's category list, declaration order.")
	out.writef("var toolCategories = map[string][]string{")

	for _, entry := range categorizedContracts(contracts) {
		out.writef("\t%q: {%s},", entry.Name, quotedStrings(entry.Categories))
	}

	out.writef("}")

	formatted, err := formatSource(registrySource(out), "registry")
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: "registry" + generatedSuffix, Text: formatted}, nil
}

// registrySource assembles the registry file, whose package clause and doc
// comment are written by the caller rather than by the shared renderer.
func registrySource(out *source) string {
	paths := make([]string, 0, len(out.imports))
	for path := range out.imports {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	body := out.body.String()
	packageLine := "package " + generatedPackage + "\n"

	head, rest, found := strings.Cut(body, packageLine)
	if !found {
		return generatedHeader + "\n\n" + body
	}

	var file strings.Builder

	file.Grow(len(body) + len(generatedHeader))
	file.WriteString(generatedHeader + "\n\n")
	file.WriteString(head)
	file.WriteString(packageLine)
	file.WriteString("\nimport (\n")
	writeImportBlock(&file, paths)
	file.WriteString(")\n")
	file.WriteString(rest)

	return file.String()
}

// formatSource runs emitted text through go/format, naming the group in the
// error so a template defect points at where it came from.
func formatSource(text, group string) (string, error) {
	formatted, err := format.Source([]byte(text))
	if err != nil {
		return "", fmt.Errorf("format %s: %w", group, err)
	}

	return string(formatted), nil
}
