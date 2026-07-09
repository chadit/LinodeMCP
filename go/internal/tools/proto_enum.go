package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// enumSentinel is the proto3 zero-value name every MCP enum defines (proto3
// requires a zero value). It is not a real Linode API value: the generated JSON
// Schema strips it (scripts/strip_enum_sentinel.py) and enum validation rejects
// it.
const enumSentinel = "unspecified"

// enumValueNames returns a proto enum's API value names, excluding the zero
// sentinel, ordered by enum number. The input is a generated <Enum>_value map
// (name -> number), so the allowed set and its order come from the proto enum
// and stay in sync with the generated schema without a hand-maintained list.
func enumValueNames(valueMap map[string]int32) []string {
	names := make([]string, 0, len(valueMap))
	for name := range valueMap {
		if name == enumSentinel {
			continue
		}

		names = append(names, name)
	}

	sort.Slice(names, func(i, j int) bool { return valueMap[names[i]] < valueMap[names[j]] })

	return names
}

// enumChoiceError validates an already-read enum value against a proto enum's
// value map. Empty is treated as absent and allowed (callers enforce
// required-ness separately, before this). The zero sentinel is rejected like
// any other invalid value. Returns "" when valid or empty, else the
// "<key> must be one of: ..." message. The text and value order match the
// Python side so the message-parity gate stays green.
func enumChoiceError(value, key string, valueMap map[string]int32) string {
	if value == "" {
		return ""
	}

	if _, ok := valueMap[value]; ok && value != enumSentinel {
		return ""
	}

	return fmt.Sprintf("%s must be one of: %s", key, strings.Join(enumValueNames(valueMap), ", "))
}

// optionalEnumChoice reads an optional string argument and validates it against
// a proto enum's value map. Empty or absent is allowed (the field is optional).
// The valueMap is a generated <Enum>_value map.
func optionalEnumChoice(request *mcp.CallToolRequest, key string, valueMap map[string]int32) (string, string) {
	value := request.GetString(key, "")

	return value, enumChoiceError(value, key, valueMap)
}

// requiredEnumChoice reads a required string argument and validates it against a
// proto enum's value map. Unlike optionalEnumChoice, empty or absent is rejected
// (the field is required), producing the same "<key> must be one of: ..."
// message as an invalid value. Returns "" when valid.
func requiredEnumChoice(request *mcp.CallToolRequest, key string, valueMap map[string]int32) string {
	return requiredEnumChoiceValue(request.GetString(key, ""), key, valueMap)
}

// requiredEnumChoiceValue validates an already-read enum value against a proto
// enum's value map with the same required semantics as requiredEnumChoice
// (empty, absent, sentinel, or unknown all rejected). Callers use it when the
// raw argument must be transformed first, such as upper-casing a
// case-insensitive value, which the request-reading requiredEnumChoice cannot do.
func requiredEnumChoiceValue(value, key string, valueMap map[string]int32) string {
	if _, ok := valueMap[value]; ok && value != enumSentinel {
		return ""
	}

	return fmt.Sprintf("%s must be one of: %s", key, strings.Join(enumValueNames(valueMap), ", "))
}
