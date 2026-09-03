package tools

import (
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The whole-map refusals every call answers before its rules run. Both read the
// raw argument map because the values they answer for never reach the input
// message: a name the message does not declare is dropped when the message is
// built, so a caller's typo is only visible here.

// refusedFieldsPlaceholder is what a declared refuse_arguments sentence names
// the arguments it refused with.
const refusedFieldsPlaceholder = "{fields}"

// engineControlArgument reports whether a name is one a call carries for the
// server rather than for the route. No input message declares these, so the
// refusal below has to let them through by name: destroy.go reads
// confirmed_dry_run and confirm_bypass_dry_run, and server/server.go reads yolo
// and confirm_bypass_dry_run. Declaring them as system param fields, which
// would retire this function, is booked as a follow-up.
func engineControlArgument(name string) bool {
	switch name {
	case "confirm_bypass_dry_run", "confirmed_dry_run", "yolo":
		return true
	}

	return false
}

// CheckArgumentRefusals answers the whole-map refusals for one call: the
// arguments the input message declares it refuses, then any argument it does
// not declare. Refused runs first so a declaration's own wording wins.
//
// Both read the descriptor because the list tiers reach their checks through a
// shared driver holding only the message name, so one derivation serves both
// tiers.
func CheckArgumentRefusals(inputMessage, toolName string, arguments map[string]any) string {
	descriptor := inputMessageDescriptor(inputMessage)
	if descriptor == nil {
		return ""
	}

	if message := refusedByDeclaration(descriptor, arguments); message != "" {
		return message
	}

	return refusedAsUndeclared(descriptor, toolName, arguments)
}

// inputMessageDescriptor resolves a tool's input message, or nil when the
// registry does not carry it under that name. A nil descriptor refuses
// nothing: there is no allowlist to hold the call to, and refusing every
// argument would be worse than refusing none.
func inputMessageDescriptor(inputMessage string) protoreflect.MessageDescriptor {
	found, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(inputMessage))
	if err != nil {
		return nil
	}

	descriptor, isMessage := found.(protoreflect.MessageDescriptor)
	if !isMessage {
		return nil
	}

	return descriptor
}

// refusedByDeclaration answers the tool's own sentence for any argument its
// refuse_arguments declaration names. Sorted, so a payload carrying several
// reads the same way every time.
func refusedByDeclaration(descriptor protoreflect.MessageDescriptor, arguments map[string]any) string {
	declared, _ := proto.GetExtension(
		descriptor.Options(), linodev1.E_RefuseArguments,
	).(*linodev1.RefuseArguments)
	if declared == nil {
		return ""
	}

	var supplied []string

	for _, name := range declared.GetFields() {
		if _, set := arguments[name]; set {
			supplied = append(supplied, name)
		}
	}

	if len(supplied) == 0 {
		return ""
	}

	sort.Strings(supplied)

	return strings.ReplaceAll(
		declared.GetMessage(), refusedFieldsPlaceholder, strings.Join(supplied, ", "),
	)
}

// refusedAsUndeclared answers for any argument outside the message's own fields
// and the engine's control names.
//
// The sentence is the engine's rather than a declaration's because every tool
// answers it and the list drivers cannot take an emitted literal. The behavior
// fixtures are what hold the Go and Python copies to the same words.
func refusedAsUndeclared(
	descriptor protoreflect.MessageDescriptor, toolName string, arguments map[string]any,
) string {
	fields := descriptor.Fields()

	var unknown []string

	for name := range arguments {
		declared := fields.ByName(protoreflect.Name(name)) != nil || engineControlArgument(name)
		if !declared {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) == 0 {
		return ""
	}

	sort.Strings(unknown)

	return "Unsupported argument(s) for " + toolName + ": " + strings.Join(unknown, ", ")
}

// PresentStringArgument holds one argument to having been sent as a string,
// blank included, read off the argument map because a body field that is not a
// string stops the message being built and no rule runs. required is false for
// an `optional` field, where an absent argument is accepted. Mirrors the Python
// present_string helper.
func PresentStringArgument(request *mcp.CallToolRequest, name string, required bool) string {
	return DeclaredPresentString(request, name, required, "", "")
}

// DeclaredPresentString is the string reader answering the sentences one
// declaration words.
func DeclaredPresentString(request *mcp.CallToolRequest, name string, required bool, absent, unusable string) string {
	raw, present := request.GetArguments()[name]
	if !present {
		if required {
			return declaredOr(absent, name+" must be a string")
		}

		return ""
	}

	if _, isText := raw.(string); !isText {
		return declaredOr(unusable, name+" must be a string")
	}

	return ""
}

// IDListArgument holds one argument to being a list of distinct positive ids.
func IDListArgument(request *mcp.CallToolRequest, name string) string {
	return DeclaredIDList(request, name, "", "", "")
}

// DeclaredIDList is the id-list reader answering the sentences one declaration
// words.
//
// It reads the argument map because proto3 reads an absent list and an empty
// one as the same empty list, and the routes declaring this answer those two
// differently.
func DeclaredIDList(request *mcp.CallToolRequest, name, absent, unusable, refused string) string {
	raw, present := request.GetArguments()[name]
	if !present {
		return declaredOr(absent, name+" is required")
	}

	entries, isList := raw.([]any)
	if !isList {
		return declaredOr(unusable, name+" must be a JSON array of positive integers")
	}

	shape := declaredOr(refused,
		name+" must be a non-empty array of distinct positive integers")

	if len(entries) == 0 {
		return shape
	}

	seen := make(map[int]struct{}, len(entries))

	for _, entry := range entries {
		value, whole := numberArgToInt(entry)
		if !whole || value < 1 {
			return shape
		}

		if _, repeated := seen[value]; repeated {
			return shape
		}

		seen[value] = struct{}{}
	}

	return ""
}
