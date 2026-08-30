package tools

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// The named format readers: one body per member, called from generated code
// through the argument_reader each field declares. They live here rather than
// beside the family that first wrote them because a member is shared by
// definition, and a second copy is what the declaration exists to prevent.

// ServiceTypeSlugArgument reads a monitoring service type, which the routes
// carry as a bare path segment.
func ServiceTypeSlugArgument(request *mcp.CallToolRequest, name string) (string, string) {
	return DeclaredServiceTypeSlug(request, name, "", "", "")
}

// DeclaredServiceTypeSlug is the service-type reader answering the sentences
// one declaration words.
//
// Surrounding space is refused rather than trimmed: a trimmed value addresses a
// different route than the caller spelled.
func DeclaredServiceTypeSlug(request *mcp.CallToolRequest, name, absent, unusable, refused string) (string, string) {
	raw, exists := request.GetArguments()[name]
	if !exists {
		return "", declaredOr(absent, name+" is required")
	}

	value, isText := raw.(string)
	if !isText {
		return "", declaredOr(unusable, name+" must be a string")
	}

	if value == "" || !hyphenatedLowerSlug(value) {
		return "", declaredOr(refused, name+" must be a single non-empty service type slug")
	}

	return value, ""
}

// hyphenatedLowerSlug reports whether value is lowercase letters and digits
// with hyphens only between them.
func hyphenatedLowerSlug(value string) bool {
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			continue
		}

		if char == '-' && index != 0 && index != len(value)-1 {
			continue
		}

		return false
	}

	return true
}

// RegionSlugArgument reads a region id, which Linode spells as a lowercase
// slug.
func RegionSlugArgument(request *mcp.CallToolRequest, name string) (string, string) {
	return DeclaredRegionSlug(request, name, "", "", "")
}

// DeclaredRegionSlug is the region reader answering the sentences one
// declaration words.
//
// The accepted set is exactly what the refusal names, so the stricter spellings
// these routes grew apart into are gone: no sentence ever told a caller that a
// region may not begin with a hyphen.
func DeclaredRegionSlug(request *mcp.CallToolRequest, name, absent, unusable, refused string) (string, string) {
	value, message := nonBlankText(request, name, absent, unusable)
	if message != "" {
		return "", message
	}

	if !lowerSlugCharset(value) {
		return "", declaredOr(refused,
			name+" must be a lowercase region slug containing only letters, numbers, and hyphens")
	}

	return value, ""
}

// lowerSlugCharset reports whether every character is a lowercase letter, a
// digit, or a hyphen, wherever it sits.
func lowerSlugCharset(value string) bool {
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' {
			continue
		}

		return false
	}

	return true
}

// BetaSlugArgument reads a beta program id, which the API spells as a slug
// rather than a number.
func BetaSlugArgument(request *mcp.CallToolRequest, name string) (string, string) {
	return DeclaredBetaSlug(request, name, "", "", "")
}

// DeclaredBetaSlug is the beta-program reader answering the sentences one
// declaration words. Wider than the region member by case and the underscore,
// which is the whole difference between the two.
func DeclaredBetaSlug(request *mcp.CallToolRequest, name, absent, unusable, refused string) (string, string) {
	value, message := nonBlankText(request, name, absent, unusable)
	if message != "" {
		return "", message
	}

	if value != strings.TrimSpace(value) || !wordSlugCharset(value) {
		return "", declaredOr(refused, name+" must contain only letters, numbers, underscores, and hyphens")
	}

	return value, ""
}

// wordSlugCharset reports whether every character is a letter, a digit, an
// underscore, or a hyphen.
func wordSlugCharset(value string) bool {
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
			continue
		case char >= '0' && char <= '9', char == '_', char == '-':
			continue
		default:
			return false
		}
	}

	return true
}

// FreeTextArgument reads a path segment whose shape the message's own rules
// judge.
func FreeTextArgument(request *mcp.CallToolRequest, name string) (string, string) {
	return DeclaredFreeText(request, name, "", "")
}

// DeclaredFreeText is the free-text reader answering the sentences one
// declaration words.
//
// It refuses nothing a rule can see. What it adds is the answer for a caller
// who sent a non-string, which no rule reaches because a message that cannot be
// built evaluates none.
func DeclaredFreeText(request *mcp.CallToolRequest, name, absent, unusable string) (string, string) {
	return nonBlankText(request, name, absent, unusable)
}

// nonBlankText answers the two arms the slug readers share before they judge a
// charset: an argument nobody sent, and one sent as anything but text with
// something in it.
func nonBlankText(request *mcp.CallToolRequest, name, absent, unusable string) (string, string) {
	raw, exists := request.GetArguments()[name]
	if !exists {
		return "", declaredOr(absent, name+" is required")
	}

	value, isText := raw.(string)
	if !isText || strings.TrimSpace(value) == "" {
		return "", declaredOr(unusable, name+" must be a non-empty string")
	}

	return value, ""
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
