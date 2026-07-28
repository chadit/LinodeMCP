package tools_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	reservedIPTagFixture     = envProd
	reservedIPGateway        = "192.0.2.1"
	reservedIPAssignedEntity = "assigned_entity"
	keyReservedIPTags        = "tags"
	keyReservedIPPrefix      = "prefix"
	keyReservedIPVPCNAT      = "vpc_nat_1_1"
	caseTagsNotAList         = "tags not a list"
	caseEmptyTag             = "empty tag"
	caseNoConfirmation       = "no confirmation"
	caseIPv6Address          = "ipv6 address"
	// The malformed-body half of both write tools' error text, shared with the
	// Python twin through testdata/behavior so neither side can drift.
	reservedIPNotObjectText = "reserved IP response must be an object"
)

// reservedIPBodyFixture is the documented single-address response, carrying the
// explicit nulls the API returns for an unassigned address.
func reservedIPBodyFixture() map[string]any {
	return map[string]any{
		keyAddress:               reservedIPAddressFixture,
		reservedIPAssignedEntity: nil,
		"gateway":                reservedIPGateway,
		keyInterfaceID:           nil,
		keySupportTicketLinodeID: nil,
		keyReservedIPPrefix:      24,
		"public":                 true,
		keyRDNS:                  nil,
		keyRegion:                regionUSEast,
		"reserved":               true,
		"subnet_mask":            "255.255.255.0",
		keyReservedIPTags:        []string{},
		keyType:                  keyIPv4,
		keyReservedIPVPCNAT:      nil,
	}
}

// reservedIPServer serves one JSON body and records the request the tool made.
func reservedIPServer(t *testing.T, body any) (*config.Config, *http.Request, *[]byte) {
	t.Helper()

	var (
		seen     http.Request
		seenBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = *r

		read, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		seenBody = read

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	return cfg, &seen, &seenBody
}

func reservedIPResultBody(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	if result == nil || result.IsError {
		t.Fatalf("result = %+v, want successful result", result)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("result.Content[0] is not mcp.TextContent")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}

	return body
}

func TestLinodeReservedIPGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPGetTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_get")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyAddress) {
		t.Errorf("tool.RawInputSchema missing key %v", keyAddress)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestLinodeReservedIPGetToolPreservesExplicitNulls pins the reason the get path
// carries the raw body alongside the proto: protojson drops the nulls the API
// documents, and dropping them would report an assigned address as unassigned.
func TestLinodeReservedIPGetToolPreservesExplicitNulls(t *testing.T) {
	t.Parallel()

	cfg, seen, _ := reservedIPServer(t, reservedIPBodyFixture())
	_, _, handler := tools.NewLinodeReservedIPGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: reservedIPAddressFixture,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := reservedIPResultBody(t, result)

	if seen.Method != http.MethodGet {
		t.Errorf("method = %v, want %v", seen.Method, http.MethodGet)
	}

	if seen.URL.Path != "/networking/reserved/ips/"+reservedIPAddressFixture {
		t.Errorf("path = %v, want the address route", seen.URL.Path)
	}

	for _, field := range []string{reservedIPAssignedEntity, keyInterfaceID, keySupportTicketLinodeID, keyRDNS, keyReservedIPVPCNAT} {
		value, present := body[field]
		if !present || value != nil {
			t.Errorf("body[%q] = %v (present %v), want an explicit null", field, value, present)
		}
	}
}

func TestLinodeReservedIPGetToolRejectsInvalidAddressBeforeClient(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPGetTool(&config.Config{})

	for name, args := range map[string]map[string]any{
		caseMissing: {},
		"ipv6":      {keyAddress: "2001:db8::10"},
		"garbage":   {keyAddress: "not-an-ip"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil || !result.IsError {
				t.Fatalf("result = %+v, want a validation error", result)
			}
		})
	}
}

func TestLinodeReservedIPTypeListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPTypeListTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_type_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_type_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestLinodeReservedIPTypeListToolKeepsNullPrices proves the pricing route needs
// no raw copy: its nullable price fields survive protojson because they are
// declared optional in the proto.
func TestLinodeReservedIPTypeListToolKeepsNullPrices(t *testing.T) {
	t.Parallel()

	cfg, seen, _ := reservedIPServer(t, map[string]any{
		"data": []map[string]any{{
			keySupportTicketID: "ipv4_reserved",
			"label":            "IPv4 Reserved",
			"price":            map[string]any{"hourly": nil, "monthly": nil},
			"region_prices":    []any{},
			"transfer":         0,
		}},
		keyPage: 1, keyPages: 1, keyResults: 1,
	})
	_, _, handler := tools.NewLinodeReservedIPTypeListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := reservedIPResultBody(t, result)

	if seen.URL.Path != "/networking/reserved/ips/types" {
		t.Errorf("path = %v, want the types route", seen.URL.Path)
	}

	types, isList := body["reserved_ip_types"].([]any)
	if !isList || len(types) != 1 {
		t.Fatalf("reserved_ip_types = %v, want one entry", body["reserved_ip_types"])
	}

	entry, isObject := types[0].(map[string]any)
	if !isObject {
		t.Fatalf("entry = %v, want an object", types[0])
	}

	price, hasPrice := entry["price"].(map[string]any)
	if !hasPrice {
		t.Fatalf("price = %v, want an object", entry["price"])
	}

	for _, field := range []string{"hourly", "monthly"} {
		value, present := price[field]
		if !present || value != nil {
			t.Errorf("price[%q] = %v (present %v), want an explicit null", field, value, present)
		}
	}
}

func TestLinodeReservedIPCreateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPCreateTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_create")
	}

	for _, key := range []string{keyRegion, keyConfirm, keyDryRun} {
		if !strings.Contains(string(tool.RawInputSchema), key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestLinodeReservedIPCreateToolOmitsAbsentTags pins the difference between an
// absent tag list and an empty one: absent leaves the field off the request
// entirely rather than sending an empty array the API would read as a value.
func TestLinodeReservedIPCreateToolOmitsAbsentTags(t *testing.T) {
	t.Parallel()

	cfg, seen, seenBody := reservedIPServer(t, reservedIPBodyFixture())
	_, _, handler := tools.NewLinodeReservedIPCreateTool(cfg)

	_, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast, keyConfirm: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seen.Method != http.MethodPost {
		t.Errorf("method = %v, want %v", seen.Method, http.MethodPost)
	}

	var sent map[string]any
	if err := json.Unmarshal(*seenBody, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	if sent["region"] != regionUSEast {
		t.Errorf("body region = %v, want %v", sent["region"], regionUSEast)
	}

	if _, present := sent[keyReservedIPTags]; present {
		t.Errorf("body = %v, want no tags field when the caller supplied none", sent)
	}
}

func TestLinodeReservedIPCreateToolSendsSuppliedTags(t *testing.T) {
	t.Parallel()

	cfg, _, seenBody := reservedIPServer(t, reservedIPBodyFixture())
	_, _, handler := tools.NewLinodeReservedIPCreateTool(cfg)

	_, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:         regionUSEast,
		keyReservedIPTags: []any{reservedIPTagFixture},
		keyConfirm:        true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(*seenBody, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	tags, ok := sent[keyReservedIPTags].([]any)
	if !ok || len(tags) != 1 || tags[0] != reservedIPTagFixture {
		t.Errorf("body tags = %v, want the supplied list", sent[keyReservedIPTags])
	}
}

// TestLinodeReservedIPCreateToolRejectsBadInputBeforeClient covers the whole
// validation surface with no server behind it, so a reachable client call would
// fail the test rather than silently reserve an address.
func TestLinodeReservedIPCreateToolRejectsBadInputBeforeClient(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPCreateTool(&config.Config{})

	for name, args := range map[string]map[string]any{
		caseMissingRegion:  {keyConfirm: true},
		"uppercase region": {keyRegion: "US-East", keyConfirm: true},
		"trailing dash":    {keyRegion: "us-east-", keyConfirm: true},
		caseTagsNotAList:   {keyRegion: regionUSEast, keyReservedIPTags: reservedIPTagFixture, keyConfirm: true},
		caseEmptyTag:       {keyRegion: regionUSEast, keyReservedIPTags: []any{""}, keyConfirm: true},
		caseNoConfirmation: {keyRegion: regionUSEast},
		caseFalseConfirm:   {keyRegion: regionUSEast, keyConfirm: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil || !result.IsError {
				t.Fatalf("result = %+v, want a validation error", result)
			}
		})
	}
}

// TestLinodeReservedIPCreateToolDryRunReportsUnknownBilling pins the billing
// contract: the preview cannot price a reservation without a region price, and
// "unknown" is what the dry-run shape uses for that rather than an empty string
// that reads like a free resource.
func TestLinodeReservedIPCreateToolDryRunReportsUnknownBilling(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast, keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := reservedIPResultBody(t, result)

	delta, ok := body["billing_delta"].(map[string]any)
	if !ok {
		t.Fatalf("billing_delta = %v, want an object", body["billing_delta"])
	}

	if delta["monthly_change_usd"] != "unknown" {
		t.Errorf("monthly_change_usd = %v, want %q", delta["monthly_change_usd"], "unknown")
	}

	if body["current_state"] != nil {
		t.Errorf("current_state = %v, want null for a create preview", body["current_state"])
	}
}

func TestLinodeReservedIPUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPUpdateTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_update")
	}

	for _, key := range []string{keyAddress, keyReservedIPTags, keyConfirm, keyDryRun} {
		if !strings.Contains(string(tool.RawInputSchema), key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestLinodeReservedIPUpdateToolSendsEmptyTagsToClear proves an empty list is a
// meaningful request here rather than an omission: the update replaces the whole
// tag set, so sending nothing would leave the existing tags in place.
func TestLinodeReservedIPUpdateToolSendsEmptyTagsToClear(t *testing.T) {
	t.Parallel()

	cfg, seen, seenBody := reservedIPServer(t, reservedIPBodyFixture())
	_, _, handler := tools.NewLinodeReservedIPUpdateTool(cfg)

	_, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress:        reservedIPAddressFixture,
		keyReservedIPTags: []any{},
		keyConfirm:        true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seen.Method != http.MethodPut {
		t.Errorf("method = %v, want %v", seen.Method, http.MethodPut)
	}

	var sent map[string]any
	if err := json.Unmarshal(*seenBody, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	tags, ok := sent[keyReservedIPTags].([]any)
	if !ok || len(tags) != 0 {
		t.Errorf("body tags = %v, want an empty list", sent[keyReservedIPTags])
	}
}

func TestLinodeReservedIPUpdateToolRejectsBadInputBeforeClient(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPUpdateTool(&config.Config{})

	for name, args := range map[string]map[string]any{
		"missing address":  {keyReservedIPTags: []any{reservedIPTagFixture}, keyConfirm: true},
		caseIPv6Address:    {keyAddress: "2001:db8::10", keyReservedIPTags: []any{reservedIPTagFixture}, keyConfirm: true},
		"missing tags":     {keyAddress: reservedIPAddressFixture, keyConfirm: true},
		caseTagsNotAList:   {keyAddress: reservedIPAddressFixture, keyReservedIPTags: reservedIPTagFixture, keyConfirm: true},
		caseEmptyTag:       {keyAddress: reservedIPAddressFixture, keyReservedIPTags: []any{""}, keyConfirm: true},
		caseNoConfirmation: {keyAddress: reservedIPAddressFixture, keyReservedIPTags: []any{reservedIPTagFixture}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil || !result.IsError {
				t.Fatalf("result = %+v, want a validation error", result)
			}
		})
	}
}

// TestLinodeReservedIPUpdateToolDryRunFetchesStateWithoutUpdating proves the
// preview reads the address it would change and never issues the PUT.
func TestLinodeReservedIPUpdateToolDryRunFetchesStateWithoutUpdating(t *testing.T) {
	t.Parallel()

	cfg, seen, _ := reservedIPServer(t, reservedIPBodyFixture())
	_, _, handler := tools.NewLinodeReservedIPUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress:        reservedIPAddressFixture,
		keyReservedIPTags: []any{reservedIPTagFixture},
		keyDryRun:         true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seen.Method != http.MethodGet {
		t.Errorf("method = %v, want the state fetch to be a %v", seen.Method, http.MethodGet)
	}

	body := reservedIPResultBody(t, result)

	if body[keyDryRun] != true {
		t.Errorf("dry_run = %v, want true", body[keyDryRun])
	}

	state, ok := body["current_state"].(map[string]any)
	if !ok || state["address"] != reservedIPAddressFixture {
		t.Errorf("current_state = %v, want the fetched address", body["current_state"])
	}
}

// reservedIPErrorServer answers every request with an API error, which is what
// each tool's failure branch has to surface rather than swallow.
func reservedIPErrorServer(t *testing.T) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// reservedIPToolFactory is one of the family's constructors, named so a failure
// says which tool left an error branch unhandled. shapeError is the whole tool
// error a non-object response body has to produce, set for the three tools that
// decode one address; type_list reaches the shared list decoder instead and
// fails with its wording, not this one.
type reservedIPToolFactory struct {
	name       string
	tool       func(*config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error))
	args       map[string]any
	shapeError string
}

func reservedIPToolFactories() []reservedIPToolFactory {
	return []reservedIPToolFactory{
		{
			name:       "get",
			tool:       tools.NewLinodeReservedIPGetTool,
			args:       map[string]any{keyAddress: reservedIPAddressFixture},
			shapeError: "Failed to get reserved IPv4 address: " + reservedIPNotObjectText,
		},
		{name: "type_list", tool: tools.NewLinodeReservedIPTypeListTool, args: map[string]any{}},
		{
			name:       "create",
			tool:       tools.NewLinodeReservedIPCreateTool,
			args:       map[string]any{keyRegion: regionUSEast, keyConfirm: true},
			shapeError: "Failed to reserve public IPv4 address: " + reservedIPNotObjectText,
		},
		{
			name:       "update",
			tool:       tools.NewLinodeReservedIPUpdateTool,
			args:       map[string]any{keyAddress: reservedIPAddressFixture, keyReservedIPTags: []any{reservedIPTagFixture}, keyConfirm: true},
			shapeError: "Failed to replace reserved IPv4 tags: " + reservedIPNotObjectText,
		},
	}
}

// reservedIPAddressToolFactories narrows the family to the tools that decode a
// single-address response body, which are the ones the malformed-body case
// attacks.
func reservedIPAddressToolFactories() []reservedIPToolFactory {
	decoders := make([]reservedIPToolFactory, 0, 3)

	for _, factory := range reservedIPToolFactories() {
		if factory.shapeError != "" {
			decoders = append(decoders, factory)
		}
	}

	return decoders
}

// TestReservedIPToolsRejectNonObjectBodies proves a get, create or update
// response that is not a JSON object fails as a tool error carrying the shared
// sentence, rather than rendering as an address with every field empty. The
// exact text is pinned because the Python twin has to emit the same one.
func TestReservedIPToolsRejectNonObjectBodies(t *testing.T) {
	t.Parallel()

	bodies := map[string]any{
		"bare array":  []any{},
		"json null":   nil,
		"bare string": "reserved",
		"bare number": 42,
		"boolean":     true,
	}

	for _, factory := range reservedIPAddressToolFactories() {
		for bodyName, body := range bodies {
			t.Run(factory.name+"/"+bodyName, func(t *testing.T) {
				t.Parallel()

				cfg, _, _ := reservedIPServer(t, body)
				_, _, handler := factory.tool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, factory.args))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				textContent, isText := result.Content[0].(mcp.TextContent)
				if !result.IsError || !isText {
					t.Fatalf("result = %+v, want a tool error", result)
				}

				if textContent.Text != factory.shapeError {
					t.Errorf("text = %q, want %q", textContent.Text, factory.shapeError)
				}
			})
		}
	}
}

// TestReservedIPToolsSurfaceAPIErrors proves no tool in the family reports a
// rejected call as success.
func TestReservedIPToolsSurfaceAPIErrors(t *testing.T) {
	t.Parallel()

	for _, factory := range reservedIPToolFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := factory.tool(reservedIPErrorServer(t))

			result, err := handler(t.Context(), createRequestWithArgs(t, factory.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			textContent, isText := result.Content[0].(mcp.TextContent)
			if !result.IsError || !isText || textContent.Text == "" {
				t.Errorf("result = %+v, want a non-empty tool error", result)
			}
		})
	}
}

// TestReservedIPToolsSurfaceConfigErrors proves an unusable environment fails
// before any request rather than panicking on a nil client.
func TestReservedIPToolsSurfaceConfigErrors(t *testing.T) {
	t.Parallel()

	for _, factory := range reservedIPToolFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := factory.tool(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, factory.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			textContent, isText := result.Content[0].(mcp.TextContent)
			if !result.IsError || !isText || textContent.Text == "" {
				t.Errorf("result = %+v, want a non-empty configuration error", result)
			}
		})
	}
}

// TestReservedIPGetToolRejectsAMalformedBody covers the half of the malformed
// surface the shape guard deliberately leaves alone: an object whose fields do
// not fit the proto is still a hard decode error, because only the wrong
// top-level JSON type has a sentence the Python twin can match.
func TestReservedIPGetToolRejectsAMalformedBody(t *testing.T) {
	t.Parallel()

	cfg, _, _ := reservedIPServer(t, map[string]any{keyReservedIPPrefix: "not-an-integer"})
	_, _, handler := tools.NewLinodeReservedIPGetTool(cfg)

	_, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: reservedIPAddressFixture,
	}))
	if err == nil {
		t.Fatal("expected a decode error for a body the proto cannot accept")
	}
}
