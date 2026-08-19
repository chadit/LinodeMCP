package tools

import (
	"slices"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// The two argument-map refusals a tool declares rather than derives. Both read
// the raw arguments because the values they answer for never reach the input
// message: a name the message does not declare is dropped before any rule runs,
// so a caller's typo is only visible here.

// refusedFieldsPlaceholder and refusedFieldPlaceholder are what a declared
// sentence names its refused arguments with.
const (
	refusedFieldsPlaceholder = "{fields}"
	refusedFieldPlaceholder  = "{field}"
)

// RefusedArguments answers for any of the named arguments the caller set,
// filling the declared sentence with the ones they actually sent.
func RefusedArguments(request *mcp.CallToolRequest, sentence string, names ...string) string {
	arguments := request.GetArguments()

	var supplied []string

	for _, name := range names {
		if _, set := arguments[name]; set {
			supplied = append(supplied, name)
		}
	}

	if len(supplied) == 0 {
		return ""
	}

	sort.Strings(supplied)

	return fillRefusedNames(sentence, supplied)
}

// UnknownArguments answers for any argument the tool's input message does not
// declare, filling the declared sentence with what the caller sent.
func UnknownArguments(request *mcp.CallToolRequest, sentence string, declared ...string) string {
	var unknown []string

	for name := range request.GetArguments() {
		if !slices.Contains(declared, name) {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) == 0 {
		return ""
	}

	sort.Strings(unknown)

	return fillRefusedNames(sentence, unknown)
}

// fillRefusedNames writes the refused names into a declared sentence. Sorted
// input, so a payload carrying several reads the same way every time.
func fillRefusedNames(sentence string, names []string) string {
	filled := strings.ReplaceAll(sentence, refusedFieldsPlaceholder, strings.Join(names, ", "))

	return strings.ReplaceAll(filled, refusedFieldPlaceholder, names[0])
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
