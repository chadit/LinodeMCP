package tools

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// The declared argument rewrites. A tool names its transforms through
// normalize_fields and the generated handler calls the one below that renders
// it, before anything reads an argument, so every later read sees one value.

// TrimArguments drops surrounding whitespace from the named text arguments in
// place. A value that arrived as anything else is left alone, so the body
// builder still refuses it by type rather than reading one this rewrote.
// Mirrors the Python trim_arguments helper.
func TrimArguments(request *mcp.CallToolRequest, names ...string) {
	arguments := request.GetArguments()

	for _, name := range names {
		if value, isText := arguments[name].(string); isText {
			arguments[name] = strings.TrimSpace(value)
		}
	}
}

// TrimListDropBlank trims every entry of the named list arguments and drops the
// ones left blank, so an entry that was only padding does not reach the rules as
// a value nothing can match. Mirrors the Python trim_list_drop_blank helper.
func TrimListDropBlank(request *mcp.CallToolRequest, names ...string) {
	arguments := request.GetArguments()

	for _, name := range names {
		entries, isList := arguments[name].([]any)
		if !isList {
			continue
		}

		if trimmed, allText := trimmedEntries(entries); allText {
			arguments[name] = trimmed
		}
	}
}

// trimmedEntries is one list with its entries trimmed and the blanks dropped.
// A list holding anything but text answers false and is left whole: the body
// builder words that refusal for the whole surface alike.
func trimmedEntries(entries []any) ([]any, bool) {
	trimmed := make([]any, 0, len(entries))

	for _, entry := range entries {
		text, isText := entry.(string)
		if !isText {
			return nil, false
		}

		if value := strings.TrimSpace(text); value != "" {
			trimmed = append(trimmed, value)
		}
	}

	return trimmed, true
}
