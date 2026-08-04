package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

const (
	filterDomainAlpha = "alpha.example.com"
	filterDomainBeta  = "beta.example.net"
	filterTypeMaster  = "master"
	filterTypeSlave   = "slave"
	argDomainContains = "domain_contains"
	argType           = "type"
	// filterSharedSubstring appears in both domain names, so a contains match on
	// it narrows only once a second filter joins in.
	filterSharedSubstring = "example"
)

// domainFilterPage is the page every case in this file filters, served whole so
// the filtering under test is the client-side pass rather than anything the
// stub decided.
const domainFilterPage = `{"data":[
	{"id":1,"domain":"alpha.example.com","type":"master"},
	{"id":2,"domain":"beta.example.net","type":"slave"}
],"page":1,"pages":1,"results":2}`

// TestGeneratedListToolAppliesDerivedFilters covers the filters the emitter
// derives from a collection's query arguments rather than from a hand-written
// closure per family. The derivation is only right if a "_contains" argument
// matches by substring and a plain one matches the whole field, so both are
// exercised against a page holding one of each.
func TestGeneratedListToolAppliesDerivedFilters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want []string
	}{
		{
			name: "no filter returns the whole page",
			args: map[string]any{},
			want: []string{filterDomainAlpha, filterDomainBeta},
		},
		{
			name: "contains filter matches a substring of the name",
			args: map[string]any{argDomainContains: "beta"},
			want: []string{filterDomainBeta},
		},
		{
			name: "contains filter ignores case",
			args: map[string]any{argDomainContains: "ALPHA"},
			want: []string{filterDomainAlpha},
		},
		{
			name: "field filter matches the whole value",
			args: map[string]any{argType: filterTypeSlave},
			want: []string{filterDomainBeta},
		},
		{
			name: "field filter does not match a substring",
			args: map[string]any{argType: "mas"},
			want: nil,
		},
		{
			name: "both filters narrow together",
			args: map[string]any{argDomainContains: filterSharedSubstring, argType: filterTypeMaster},
			want: []string{filterDomainAlpha},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := domainFilterServer(t)
			defer srv.Close()

			_, _, handler := gentools.NewLinodeDomainListTool(newTestConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := listedDomains(t, resultText(t, result))
			if !equalStrings(got, testCase.want) {
				t.Errorf("domains = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestGeneratedListToolEchoesTheAppliedFilters pins the envelope's filter echo,
// which is how a caller tells a page narrowed by its arguments from one the
// account really holds.
func TestGeneratedListToolEchoesTheAppliedFilters(t *testing.T) {
	t.Parallel()

	srv := domainFilterServer(t)
	defer srv.Close()

	_, _, handler := gentools.NewLinodeDomainListTool(newTestConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{argType: filterTypeMaster}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Filter *string `json:"filter"`
		Count  int     `json:"count"`
	}

	if unmarshalErr := json.Unmarshal([]byte(resultText(t, result)), &envelope); unmarshalErr != nil {
		t.Fatalf("decode envelope: %v", unmarshalErr)
	}

	if envelope.Count != 1 {
		t.Errorf("count = %d, want 1", envelope.Count)
	}

	if envelope.Filter == nil || *envelope.Filter != argType+"="+filterTypeMaster {
		t.Errorf("filter echo = %v, want %q", envelope.Filter, argType+"="+filterTypeMaster)
	}
}

// resultText reads the single text payload a list tool answers with.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if result == nil || len(result.Content) != 1 {
		t.Fatalf("result = %v, want one content entry", result)
	}

	content, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatalf("content = %T, want text", result.Content[0])
	}

	return content.Text
}

// domainFilterServer serves the two-domain page every case here filters.
func domainFilterServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(domainFilterPage)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

// listedDomains reads the domain names out of a list tool's envelope.
func listedDomains(t *testing.T, text string) []string {
	t.Helper()

	var envelope struct {
		Domains []struct {
			Domain string `json:"domain"`
		} `json:"domains"`
	}

	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	names := make([]string, 0, len(envelope.Domains))
	for _, entry := range envelope.Domains {
		names = append(names, entry.Domain)
	}

	if len(names) == 0 {
		return nil
	}

	return names
}

// equalStrings reports whether two name lists hold the same values in order.
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
