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

// goRoutePolicyFile is where a Go run acts the one option this arm emits
// nothing for: the retry policy is read off the descriptor when the call is
// made, so a factory carrying it would say the same thing twice.
const goRoutePolicyFile = "go/internal/linoderoute/policy.go"

// acts is this arm's answer for every option the contract declares. Everything
// the Go factory renders is emitted here, which is nearly all of it, since this
// arm writes the prose and the checks into the tool rather than looking them up
// at call time the way the Python driver does.
func (goRenderer) acts() []optionClaim {
	emitted := []protoreflect.Name{
		"argument_reader", "body_comma_list", "body_constant", "body_fold",
		"body_name", "body_nullable", "body_root", "confirm_message",
		"echo_argument", "error_message", "explicit_null_fields",
		"field_location", "list_envelope", "list_filter", "normalize_fields",
		"object_walk", "preview_omits_body", "preview_redact",
		"preview_sentence", "reader_message", "reader_values",
		"refuse_arguments", "refuse_unknown_arguments", "require_any_of",
		"resource_type", "response_body_fields", "state_route",
		"success_message", "tool_api_surface", "tool_capability",
		"tool_description", "tool_hooks", "tool_meta", "tool_response",
		"tool_route", "warning_message",
	}

	claims := make([]optionClaim, 0, len(emitted)+1)
	for _, option := range emitted {
		claims = append(claims, optionClaim{option: option, emitted: true, home: ""})
	}

	return append(claims, optionClaim{
		option: "retry_disabled", emitted: false, home: goRoutePolicyFile,
	})
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
