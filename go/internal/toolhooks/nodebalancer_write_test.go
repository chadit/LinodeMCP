package toolhooks_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	nbID          = "nodebalancer_id"
	nbConfigID    = "config_id"
	nbNodeID      = "node_id"
	nbThrottle    = "client_conn_throttle"
	nbUpdatePath  = "/nodebalancers/789"
	nbConfigsPath = "/nodebalancers/5/configs"
	nbConfigPath  = "/nodebalancers/5/configs/7"
	nbRebuildPath = "/nodebalancers/5/configs/7/rebuild"
	nbNodePath    = "/nodebalancers/5/configs/7/nodes/9"
)

// nbConfigState is the config every config preview reads before reporting.
const nbConfigState = `{"id":7,"port":80,"protocol":"http","nodebalancer_id":5}`

// nbPreview is the part of a dry-run envelope these tests read.
type nbPreview struct {
	WouldExecute struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"would_execute"`
	CurrentState json.RawMessage `json:"current_state"`
	SideEffects  []string        `json:"side_effects"`
	Warnings     []string        `json:"warnings"`
}

// nbDecodePreview reads the envelope a preview hook answered with.
func nbDecodePreview(t *testing.T, text string) nbPreview {
	t.Helper()

	var preview nbPreview
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// nbStubAPI answers one GET with body and records the path it was asked for.
func nbStubAPI(t *testing.T, body string) (*httptest.Server, *string) {
	t.Helper()

	fetched := new(string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*fetched = r.URL.Path

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write stub response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, fetched
}

// TestNodebalancerConfigCreatePreviewReadsTheExistingConfigs covers the list
// fetch, which is what tells a caller which ports are already taken.
func TestNodebalancerConfigCreatePreviewReadsTheExistingConfigs(t *testing.T) {
	t.Parallel()

	server, fetched := nbStubAPI(t, `{"data":[`+nbConfigState+`],"page":1,"pages":1,"results":1}`)

	request := requestWith(map[string]any{nbID: float64(5), keyDryRun: true})

	result, err := toolhooks.LinodeNodebalancerConfigCreatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPost, nbConfigsPath, map[string]any{"port": 443})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if *fetched != nbConfigsPath {
		t.Errorf("fetched = %q, want the NodeBalancer's config list", *fetched)
	}

	preview := nbDecodePreview(t, resultText(t, result))

	var configs []map[string]any
	if err := json.Unmarshal(preview.CurrentState, &configs); err != nil {
		t.Fatalf("current_state is not a config list: %v", err)
	}

	if len(configs) != 1 || configs[0]["port"] != float64(80) {
		t.Errorf("current_state = %v, want the one config the NodeBalancer already has", configs)
	}
}

// TestNodebalancerConfigCreatePreviewReportsAnEmptyPageAsAList pins the shape
// an empty page reaches a client as: a nil slice would print as null where
// Python prints [].
func TestNodebalancerConfigCreatePreviewReportsAnEmptyPageAsAList(t *testing.T) {
	t.Parallel()

	server, _ := nbStubAPI(t, `{"data":[],"page":1,"pages":1,"results":0}`)

	request := requestWith(map[string]any{nbID: float64(5), keyDryRun: true})

	result, err := toolhooks.LinodeNodebalancerConfigCreatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPost, nbConfigsPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	preview := nbDecodePreview(t, resultText(t, result))
	if string(preview.CurrentState) != "[]" {
		t.Errorf("current_state = %s, want []", preview.CurrentState)
	}
}

// TestNodebalancerConfigCreatePreviewReportsAnUnreadableList pins that a failed
// read reaches the caller rather than a preview naming no starting point.
func TestNodebalancerConfigCreatePreviewReportsAnUnreadableList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{nbID: float64(5), keyDryRun: true})

	result, err := toolhooks.LinodeNodebalancerConfigCreatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPost, nbConfigsPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true for an unreadable config list")
	}
}

// TestNodebalancerUpdatePreviewNamesTheChange walks each sentence the update
// walk answers against the NodeBalancer it reads first.
func TestNodebalancerUpdatePreviewNamesTheChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want []string
	}{
		{
			name: "a new label reports the one it replaces",
			args: map[string]any{argLabel: "after"},
			want: []string{`Label changes from "lb-1" to "after".`},
		},
		{
			name: "a label matching the balancer reports a set",
			args: map[string]any{argLabel: "lb-1"},
			want: []string{`Label is set to "lb-1".`},
		},
		{
			name: "a throttle reports the rate it sets",
			args: map[string]any{nbThrottle: float64(5)},
			want: []string{"Connection throttle is set to 5 connections per second per client IP."},
		},
		{
			name: "a zero throttle is a value, not an absence",
			args: map[string]any{nbThrottle: float64(0)},
			want: []string{"Connection throttle is set to 0 connections per second per client IP."},
		},
		{
			name: "a tags-only edit reports nothing",
			args: map[string]any{argTags: []any{tagLabelProd}},
			want: nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server, fetched := nbStubAPI(t, `{"id":789,"label":"lb-1","region":"us-east"}`)

			args := map[string]any{nbID: float64(789), keyDryRun: true}
			maps.Copy(args, testCase.args)

			request := requestWith(args)

			result, err := toolhooks.LinodeNodebalancerUpdatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPut, nbUpdatePath, nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if *fetched != nbUpdatePath {
				t.Errorf("fetched = %q, want the NodeBalancer the change starts from", *fetched)
			}

			preview := nbDecodePreview(t, resultText(t, result))
			if len(preview.SideEffects) != len(testCase.want) {
				t.Fatalf("side_effects = %v, want %v", preview.SideEffects, testCase.want)
			}

			for i, sentence := range testCase.want {
				if preview.SideEffects[i] != sentence {
					t.Errorf("side_effects[%d] = %q, want %q", i, preview.SideEffects[i], sentence)
				}
			}
		})
	}
}

// TestNodebalancerUpdatePreviewReportsAnUnreadableBalancer pins that a failed
// read reaches the caller rather than a preview diffing against nothing.
func TestNodebalancerUpdatePreviewReportsAnUnreadableBalancer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{nbID: float64(789), argLabel: "after", keyDryRun: true})

	result, err := toolhooks.LinodeNodebalancerUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, nbUpdatePath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true for an unreadable NodeBalancer")
	}
}
