package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// The argument readers: the id, presence, id-list, pagination and tags bodies
// that generated code and the hand-written plumbing read tool arguments
// through. They live here rather than beside the family that first wrote them
// because each is shared by definition, and a family-named file reads as
// per-tool code that could be retired with the family.

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

// MaxJSONSafeID is the largest integer a JSON number carries exactly (2^53-1).
// An id above it has already been rounded by the time a handler sees it, so a
// lookup would address a resource the caller never named. Exported because the
// generated read tools reach it through their validate hooks.
const MaxJSONSafeID = 9007199254740991

// IntSliceArgument reads an ID-array tool argument. Exported because the
// generated write tools reach it through their hooks.
func IntSliceArgument(raw any, name string) ([]int, string) {
	switch values := raw.(type) {
	case []int:
		if len(values) == 0 {
			return nil, name + " must include at least one ID"
		}

		ids := make([]int, 0, len(values))
		for _, value := range values {
			if value <= 0 {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, value)
		}

		return ids, ""
	case []any:
		return intSliceFromAnySlice(values, name)
	default:
		return nil, name + " must be an array of positive integers"
	}
}

func intSliceFromAnySlice(values []any, name string) ([]int, string) {
	if len(values) == 0 {
		return nil, name + " must include at least one ID"
	}

	ids := make([]int, 0, len(values))

	for _, value := range values {
		switch number := value.(type) {
		case float64:
			// math.MaxInt64 has no exact float64 representation: float64(math.MaxInt64)
			// rounds up to 2^63 (math.MaxInt64+1). Use >= against that float so any
			// value at or above the representable boundary is rejected. math.Trunc
			// rejects fractional values without going through an int conversion that
			// overflows for out-of-range floats.
			if number <= 0 || number >= float64(math.MaxInt64) || math.Trunc(number) != number {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, int(number))
		case int:
			if number <= 0 {
				return nil, name + " must be an array of positive integers"
			}

			ids = append(ids, number)
		default:
			return nil, name + " must be an array of positive integers"
		}
	}

	return ids, ""
}

func optionalPaginationInt(args map[string]any, name string, minValue, maxValue int) (int, string) {
	raw, exists := args[name]
	if !exists {
		return 0, ""
	}

	var value int

	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		value = int(typed)
		if typed != float64(value) {
			return 0, name + " must be an integer"
		}
	default:
		return 0, name + " must be an integer"
	}

	if value < minValue || (maxValue > 0 && value > maxValue) {
		if maxValue > 0 {
			return 0, name + " must be an integer from " + strconv.Itoa(minValue) + " through " + strconv.Itoa(maxValue)
		}

		return 0, name + " must be an integer greater than or equal to 1"
	}

	return value, ""
}

func tagsValueFromToolArg(rawTags any) ([]string, string) {
	switch tags := rawTags.(type) {
	case string:
		tagsText := strings.TrimSpace(tags)

		var values []string
		if err := json.Unmarshal([]byte(tagsText), &values); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTagsMustBeJSONStringArray, err).Error()
		}

		if values == nil {
			return nil, ErrTagsMustBeJSONStringArray.Error()
		}

		return normalizeTags(values)
	case []string:
		return normalizeTags(tags)
	case []any:
		values := make([]string, 0, len(tags))
		for _, tag := range tags {
			tagText, ok := tag.(string)
			if !ok {
				return nil, ErrTagsMustBeJSONStringArray.Error()
			}

			values = append(values, tagText)
		}

		return normalizeTags(values)
	default:
		return nil, ErrTagsMustBeJSONStringArray.Error()
	}
}

func normalizeTags(values []string) ([]string, string) {
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = strings.TrimSpace(value)
		if normalized[index] == "" {
			return nil, ErrTagsEntriesNonEmpty.Error()
		}
	}

	return normalized, ""
}

func numberArgToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}

		return int(typed), true
	default:
		return 0, false
	}
}
