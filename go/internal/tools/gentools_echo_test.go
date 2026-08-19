package tools_test

import (
	"math"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The list shapes every echo reader is held to, named because each of the
// three below runs the same set.
const (
	caseDecodedJSON = "decoded JSON"
	caseNativeSlice = "native slice"
	caseNotAList    = "not a list"
	caseWrongMember = "wrong member"
)

// echoRequest wraps one call's arguments the way the MCP layer hands them to a
// handler.
func echoRequest(arguments map[string]any) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: arguments}}
}

// A list argument reaches a handler either as decoded JSON or as the native
// slice a Go caller built, so both shapes have to read back the same. Anything
// else answers empty rather than failing, because the body builder has already
// refused the call by the time an answer is assembled.
func TestEchoStringsReadsBothListShapes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		argument any
		want     []string
	}{
		caseDecodedJSON: {argument: []any{reservedIPGateway, phase2MasterIP}, want: []string{reservedIPGateway, phase2MasterIP}},
		caseNativeSlice: {argument: []string{reservedIPGateway}, want: []string{reservedIPGateway}},
		caseEmpty:       {argument: []any{}, want: []string{}},
		caseNotAList:    {argument: reservedIPGateway, want: nil},
		caseWrongMember: {argument: []any{1}, want: nil},
		caseOmitted:     {argument: nil, want: nil},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tools.EchoStrings(echoRequest(echoArguments("ips", testCase.argument)), "ips")
			if len(got) != len(testCase.want) {
				t.Fatalf("EchoStrings = %#v, want %#v", got, testCase.want)
			}

			for index, entry := range testCase.want {
				if got[index] != entry {
					t.Errorf("entry %d = %q, want %q", index, got[index], entry)
				}
			}
		})
	}
}

// An id arrives as a JSON number and reaches the response as int32, narrowed
// the way every other echoed id is: a value the field cannot hold answers zero
// rather than wrapping around.
func TestEchoInt32sNarrowsEveryEntry(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		argument any
		want     []int32
	}{
		caseDecodedJSON: {argument: []any{float64(101), float64(102)}, want: []int32{101, 102}},
		caseNativeSlice: {argument: []int{101}, want: []int32{101}},
		"out of range":  {argument: []any{float64(math.MaxInt32) + 1}, want: []int32{0}},
		caseEmpty:       {argument: []any{}, want: []int32{}},
		caseNotAList:    {argument: float64(101), want: []int32{}},
		caseWrongMember: {argument: []any{"101"}, want: []int32{}},
		caseOmitted:     {argument: nil, want: []int32{}},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tools.EchoInt32s(echoRequest(echoArguments("ids", testCase.argument)), "ids")
			if len(got) != len(testCase.want) {
				t.Fatalf("EchoInt32s = %#v, want %#v", got, testCase.want)
			}

			for index, entry := range testCase.want {
				if got[index] != entry {
					t.Errorf("entry %d = %d, want %d", index, got[index], entry)
				}
			}
		})
	}
}

// The item list is what a named-message echo is rebuilt from, so both the
// decoded and the native object shapes have to reach the mapping.
func TestEchoItemsReadsBothObjectListShapes(t *testing.T) {
	t.Parallel()

	decoded := []any{map[string]any{managedServiceAddressParam: reservedIPGateway}}
	native := []map[string]any{{managedServiceAddressParam: reservedIPGateway}}

	cases := map[string]struct {
		argument any
		want     int
	}{
		caseDecodedJSON: {argument: decoded, want: 1},
		caseNativeSlice: {argument: native, want: 1},
		caseEmpty:       {argument: []any{}, want: 0},
		caseNotAList:    {argument: reservedIPGateway, want: 0},
		caseWrongMember: {argument: []any{reservedIPGateway}, want: 0},
		caseOmitted:     {argument: nil, want: 0},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			arguments := echoArguments("assignments", testCase.argument)

			got := tools.EchoItems(echoRequest(arguments), "assignments")
			if len(got) != testCase.want {
				t.Fatalf("EchoItems returned %d item(s), want %d", len(got), testCase.want)
			}
		})
	}
}

// One item member reaches the response through the kind the response declares,
// and a member the caller left out reads as the zero that field holds.
func TestEchoItemReadsEachMemberKind(t *testing.T) {
	t.Parallel()

	item := tools.EchoItem{
		managedServiceAddressParam: reservedIPGateway,
		keySupportTicketLinodeID:   float64(123),
		"native_id":                456,
		"wrong":                    true,
		"oversized":                float64(math.MaxInt32) + 1,
	}

	if got := item.Text(managedServiceAddressParam); got != reservedIPGateway {
		t.Errorf("Text(address) = %q, want %q", got, reservedIPGateway)
	}

	if got := item.Text("missing"); got != "" {
		t.Errorf("Text(missing) = %q, want empty", got)
	}

	if got := item.Text("wrong"); got != "" {
		t.Errorf("Text over a non-string = %q, want empty", got)
	}

	if got := item.Number(keySupportTicketLinodeID); got != 123 {
		t.Errorf("Number(linode_id) = %d, want 123", got)
	}

	if got := item.Number("native_id"); got != 456 {
		t.Errorf("Number over a native int = %d, want 456", got)
	}

	if got := item.Number("missing"); got != 0 {
		t.Errorf("Number(missing) = %d, want 0", got)
	}

	if got := item.Number("wrong"); got != 0 {
		t.Errorf("Number over a non-number = %d, want 0", got)
	}

	if got := item.Number("oversized"); got != 0 {
		t.Errorf("Number over an out-of-range id = %d, want 0", got)
	}
}

// echoArguments builds one call's arguments, leaving the name out entirely for
// the omitted case rather than sending it as a null.
func echoArguments(name string, argument any) map[string]any {
	if argument == nil {
		return map[string]any{}
	}

	return map[string]any{name: argument}
}
