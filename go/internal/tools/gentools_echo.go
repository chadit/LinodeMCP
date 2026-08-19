package tools

import "github.com/mark3labs/mcp-go/mcp"

// This file reads a repeated argument back for an answer that is assembled
// rather than decoded: the API returns an opaque body, so the tool reports the
// list it was asked to apply.
//
// Every reader here is lenient because the body builder has already refused a
// malformed call by the time an answer is assembled: the handler reports the
// body's message and never reaches the echo.

// EchoStrings reads a repeated string argument back.
func EchoStrings(request *mcp.CallToolRequest, name string) []string {
	values, _ := bodyStringList(name, request.GetArguments()[name])

	return values
}

// EchoInt32s reads a repeated integer argument back, narrowing each entry the
// way IDToInt32 narrows every other echoed id.
func EchoInt32s(request *mcp.CallToolRequest, name string) []int32 {
	values, _ := bodyIntList(name, request.GetArguments()[name])

	found := make([]int32, 0, len(values))

	for _, value := range values {
		found = append(found, IDToInt32(value))
	}

	return found
}

// EchoItem is one entry of a repeated named-message argument, read member by
// member because the response item is a message of its own rather than the one
// the caller sent.
type EchoItem map[string]any

// Text reads one member as the string it arrived as.
func (e EchoItem) Text(member string) string {
	value, _ := e[member].(string)

	return value
}

// Number reads one member as the id it arrived as.
func (e EchoItem) Number(member string) int32 {
	value, _ := bodyIntValue(member, e[member])

	return IDToInt32(value)
}

// EchoItems reads a repeated named-message argument back, one entry per item.
func EchoItems(request *mcp.CallToolRequest, name string) []EchoItem {
	items, _ := bodyObjectList(name, request.GetArguments()[name])

	found := make([]EchoItem, 0, len(items))

	for _, item := range items {
		found = append(found, EchoItem(item))
	}

	return found
}
