package tools

import (
	"encoding/base64"
	"math"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	accountUserUsernameParam        = "username"
	errAccountUserUsernamePathParam = "username must not contain '/', '?', '#', or '..'"
	oauthClientThumbnailPNGParam    = "thumbnail_png_base64"
	errThumbnailPNGRequired         = "thumbnail_png_base64 is required"
)

// MaxJSONSafeID is the largest integer a JSON number carries exactly (2^53-1).
// An id above it has already been rounded by the time a handler sees it, so a
// lookup would address a resource the caller never named. Exported because the
// generated read tools reach it through their validate hooks.
const MaxJSONSafeID = 9007199254740991

// OAuthClientThumbnailPNG decodes the base64 image the thumbnail update carries
// and answers the sentence a caller gets for each way the argument can be
// unusable. Exported because the generated tool reaches it through its validate
// hook; the transport decodes the same text again when it sends the bytes.
func OAuthClientThumbnailPNG(request *mcp.CallToolRequest) ([]byte, string) {
	raw, exists := request.GetArguments()[oauthClientThumbnailPNGParam]
	if !exists {
		return nil, errThumbnailPNGRequired
	}

	encoded, ok := raw.(string)
	if !ok || strings.TrimSpace(encoded) == "" {
		return nil, "thumbnail_png_base64 must be a non-empty string"
	}

	thumbnailPNG, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "thumbnail_png_base64 must be valid standard base64"
	}

	return thumbnailPNG, ""
}

func requiredAccountUserString(args map[string]any, name string) (string, string) {
	raw, found := args[name]
	if !found {
		return "", name + " is required"
	}

	value, isString := raw.(string)
	if !isString || strings.TrimSpace(value) == "" {
		return "", name + " must be a non-empty string"
	}

	return value, ""
}

// AccountUsernamePathArgument reads the username the account-user routes are
// addressed by, rejecting the characters that would let one escape its path
// segment. Exported because the generated read tools reach it through their
// validate hooks while the hand-written write tools still call it directly, and
// two copies of a path-safety rule is one copy too many.
func AccountUsernamePathArgument(request *mcp.CallToolRequest) (string, string) {
	username, validationMessage := requiredAccountUserString(request.GetArguments(), accountUserUsernameParam)
	if validationMessage != "" {
		return "", validationMessage
	}

	if strings.ContainsAny(username, "/?#") || strings.Contains(username, "..") {
		return "", errAccountUserUsernamePathParam
	}

	return username, ""
}

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

// RequiredPathSafeArgument reads a string that goes into a path segment,
// rejecting the separators and traversal segments that would let a value
// address a route it was not given. The three arguments spelled this way
// (euuid, client_id, token) share one implementation so their wording cannot
// drift apart, and it is exported because the generated read tools reach it
// through their validate hooks.
func RequiredPathSafeArgument(request *mcp.CallToolRequest, name string) (string, string) {
	return DeclaredPathSafeArgument(request, name, "", "", "")
}

// DeclaredPathSafeArgument is the path-safe reader answering the sentences one
// declaration words. The segment tools name the characters they refuse in five
// different ways, which is why the words are the declaration's and only the
// accepted set is the member's.
func DeclaredPathSafeArgument(request *mcp.CallToolRequest, name, absent, unusable, refused string) (string, string) {
	return segmentArgument(request, name, "/?", absent, unusable, refused)
}

// DeclaredFragmentSafeArgument is the same reader over the narrower set that
// also refuses the fragment marker. An OAuth client id may carry one and the
// ids these routes address may not, which is a difference in what is accepted
// rather than in what is said, so it is a member of its own.
func DeclaredFragmentSafeArgument(request *mcp.CallToolRequest, name, absent, unusable, refused string) (string, string) {
	return segmentArgument(request, name, "/?#", absent, unusable, refused)
}

// segmentArgument reads a text argument that has to survive being spliced into
// one path segment, refusing the characters in guarded along with a traversal
// segment or untrimmed padding.
func segmentArgument(request *mcp.CallToolRequest, name, guarded, absent, unusable, refused string) (string, string) {
	raw, exists := request.GetArguments()[name]
	if !exists {
		return "", declaredOr(absent, name+" is required")
	}

	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", declaredOr(unusable, name+" must be a non-empty string")
	}

	if value != strings.TrimSpace(value) || strings.ContainsAny(value, guarded) || strings.Contains(value, "..") {
		return "", declaredOr(refused,
			name+" must not contain path separators, query separators, or traversal segments")
	}

	return value, ""
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
