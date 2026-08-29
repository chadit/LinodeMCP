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

// TrimList trims every text entry of the named list arguments in place and
// keeps the blanks, so an entry that was only padding is refused by the rule
// that names it rather than silently dropped. Entries that are not text are
// left for the type refusal. Mirrors the Python trim_list helper.
func TrimList(request *mcp.CallToolRequest, names ...string) {
	arguments := request.GetArguments()

	for _, name := range names {
		entries, isList := arguments[name].([]any)
		if !isList {
			continue
		}

		for index, entry := range entries {
			if text, isText := entry.(string); isText {
				entries[index] = strings.TrimSpace(text)
			}
		}
	}
}

// UppercaseArguments folds the named text arguments to upper case in place, so
// a value the API reads as an upper-case vocabulary reaches the rules that way
// however the caller spelled it. Mirrors the Python uppercase_arguments helper.
func UppercaseArguments(request *mcp.CallToolRequest, names ...string) {
	arguments := request.GetArguments()

	for _, name := range names {
		if value, isText := arguments[name].(string); isText {
			arguments[name] = strings.ToUpper(value)
		}
	}
}

// FoldIntList folds a convenience integer-list argument into a member of an
// object argument, in place. A caller-supplied non-empty target wins and the
// source is dropped, so the wire never carries both spellings of one fact. A
// target or source no reader can parse is left alone so the rules and the body
// builder refuse it by name. Mirrors the Python fold_int_list helper.
func FoldIntList(request *mcp.CallToolRequest, source, target, key string) {
	arguments := request.GetArguments()

	targetValue, message := ObjectMapArgument(arguments[target], target)
	if message != "" {
		return
	}

	if len(targetValue) > 0 {
		delete(arguments, source)

		return
	}

	raw, supplied := arguments[source]
	if !supplied {
		return
	}

	values, message := IntSliceArgument(raw, source)
	if message != "" {
		return
	}

	arguments[target] = map[string]any{key: values}

	delete(arguments, source)
}
