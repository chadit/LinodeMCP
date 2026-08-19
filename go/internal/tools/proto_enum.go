package tools

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// MemberChoiceArgument reads one raw text argument and holds it to the member
// names its contract declares, for the fields whose vocabulary a rule cannot
// see: an enum argument naming no member decodes to the enum's zero, the same
// as an absent one. The generated handlers pass the names, sentinel excluded,
// in enum-number order.
//
// An absent, non-string, or empty argument answers "<name> is required" when
// the field is required and is accepted when it is optional. Any other string
// outside the members answers "<name> must be one of: a, b", or
// "<name> must be <a>" for a vocabulary of one.
func MemberChoiceArgument(
	request *mcp.CallToolRequest, name string, required bool, members ...string,
) (string, string) {
	return DeclaredMemberChoice(request, name, required, "", "", members...)
}

// DeclaredMemberChoice is the membership reader answering the sentences one
// declaration words. A vocabulary of one reads as "must be <a>" unless the
// declaration says otherwise, which is how the allocate tools keep the
// "must be one of: ipv4" a caller has always been answered.
func DeclaredMemberChoice(
	request *mcp.CallToolRequest, name string, required bool, absent, refused string, members ...string,
) (string, string) {
	value, _ := request.GetArguments()[name].(string)
	if value == "" {
		if required {
			return "", declaredOr(absent, name+" is required")
		}

		return "", ""
	}

	if slices.Contains(members, value) {
		return value, ""
	}

	if refused != "" {
		return "", refused
	}

	if len(members) == 1 {
		return "", fmt.Sprintf("%s must be %s", name, members[0])
	}

	return "", fmt.Sprintf("%s must be one of: %s", name, strings.Join(members, ", "))
}
