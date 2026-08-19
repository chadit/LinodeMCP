package tools

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// boolTrue is used for boolean string comparison in filter functions.
const boolTrue = "true"

// RequiredIDArgument parses a required positive-integer id path argument,
// returning the Option-B pair used repo-wide: "<name> is required" when the
// argument is absent, "<name> must be a positive integer" when present but not
// a positive integer (wrong type, bool, zero, or negative). Mirrors the Python
// required_int_id helper so both languages reject identical inputs with
// identical text.
func RequiredIDArgument(request *mcp.CallToolRequest, name string) (int, string) {
	return RequiredBoundedIDArgument(request, name, 0)
}

// RequiredBoundedIDArgument is RequiredIDArgument plus an upper bound: ids above
// maxValue are rejected with the same "must be a positive integer" text. Used by
// the parsers that guard against oversized ids (float64 precision loss on very
// large JSON numbers). A maxValue of 0 disables the upper bound.
func RequiredBoundedIDArgument(request *mcp.CallToolRequest, name string, maxValue int) (int, string) {
	return DeclaredIDArgument(request, name, maxValue, "", "")
}

// DeclaredIDArgument is the id reader answering the sentences one declaration
// words, for the tools whose contract carries a `reader_message`. An empty arm
// falls back to the pair above, so the tools that word nothing and the tools
// that word one arm read the same values as the same ids.
func DeclaredIDArgument(request *mcp.CallToolRequest, name string, maxValue int, absent, refused string) (int, string) {
	raw, exists := request.GetArguments()[name]
	if !exists {
		return 0, declaredOr(absent, name+" is required")
	}

	value, ok := numberArgToInt(raw)
	if !ok || value < 1 || (maxValue > 0 && value > maxValue) {
		return 0, declaredOr(refused, name+" must be a positive integer")
	}

	return value, ""
}

// declaredOr is the sentence a declaration words for one refusal arm, or the
// reader's own when the declaration words nothing there.
func declaredOr(declared, own string) string {
	if declared != "" {
		return declared
	}

	return own
}

// RequiredPresentArgument answers whether the caller sent an argument at all,
// which is a different question from whether its value is usable.
//
// A repeated body field cannot be asked this through the message: proto3 reads
// an absent list and an empty one as the same empty list, and an empty list is
// a legal value on the routes that use this, meaning "remove every one". So the
// raw arguments are what the presence is read from.
func RequiredPresentArgument(request *mcp.CallToolRequest, name string) string {
	if _, present := request.GetArguments()[name]; !present {
		return name + " is required"
	}

	return ""
}

// PresentTextArgument reads one text argument off the argument map, answering
// "<name> is required" for every unusable shape: the hooks this replaces never
// told a missing value from one sent as a number or as spaces. required is
// false for an `optional` field, where only a present-but-unusable value is
// refused. Mirrors the Python present_text helper.
func PresentTextArgument(request *mcp.CallToolRequest, name string, required bool) string {
	return DeclaredPresentText(request, name, required, "", "")
}

// DeclaredPresentText is the text reader answering the sentences one
// declaration words. Wording the two arms apart is what lets a tool that has
// always answered "<name> must be a non-empty string" to a blank value keep
// saying it while an absent one still reads as missing.
func DeclaredPresentText(request *mcp.CallToolRequest, name string, required bool, absent, unusable string) string {
	raw, present := request.GetArguments()[name]
	if !present {
		if required {
			return declaredOr(absent, name+" is required")
		}

		return ""
	}

	value, isText := raw.(string)
	if !isText || strings.TrimSpace(value) == "" {
		return declaredOr(unusable, name+" is required")
	}

	return ""
}

// PresentBoolArgument holds one argument to having been sent as a boolean,
// read off the argument map because an implicit-presence bool reads absent and
// false alike. required is false for an `optional` field, where an absent flag
// is accepted and only a present non-boolean is refused. Mirrors the Python
// present_bool helper.
func PresentBoolArgument(request *mcp.CallToolRequest, name string, required bool) string {
	return DeclaredPresentBool(request, name, required, "", "")
}

// DeclaredPresentBool is the boolean reader answering the sentences one
// declaration words, for the tools that tell a caller who sent no flag from one
// who sent something that is not a flag.
func DeclaredPresentBool(request *mcp.CallToolRequest, name string, required bool, absent, unusable string) string {
	raw, present := request.GetArguments()[name]
	if !present {
		if required {
			return declaredOr(absent, name+" must be a boolean")
		}

		return ""
	}

	if _, isBool := raw.(bool); !isBool {
		return declaredOr(unusable, name+" must be a boolean")
	}

	return ""
}

// RequireAnyArgument answers sentence when the caller sent none of the named
// arguments, reading the map because the updates that ask this accept a
// field's zero as a real change (an empty tags list clears the tags), so
// absent and empty conflate anywhere later. Mirrors the Python
// require_any_argument helper.
func RequireAnyArgument(request *mcp.CallToolRequest, sentence string, names ...string) string {
	arguments := request.GetArguments()

	for _, name := range names {
		if _, present := arguments[name]; present {
			return ""
		}
	}

	return sentence
}

func selectEnvironment(cfg *config.Config, environment string) (*config.EnvironmentConfig, error) {
	if environment != "" {
		if env, exists := cfg.Environments[environment]; exists {
			return &env, nil
		}

		return nil, fmt.Errorf("%w: %s", ErrEnvironmentNotFound, environment)
	}

	selectedEnv, err := cfg.SelectEnvironment("default")
	if err != nil {
		return nil, fmt.Errorf("failed to select default environment: %w", err)
	}

	return selectedEnv, nil
}

// linodeConfigComplete reports whether an environment carries what a client
// needs. Named apart from the argument readers on purpose: this judges the
// deployment's own configuration rather than anything a caller sent, and the
// hand-validator scan counts by that naming convention.
func linodeConfigComplete(env *config.EnvironmentConfig) error {
	if env.Linode.APIURL == "" || env.Linode.Token == "" {
		return ErrLinodeConfigIncomplete
	}

	return nil
}
