package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chadit/LinodeMCP/internal/config"
)

// dryRunGetStateServer serves state on GET and fails any non-GET request,
// so a dry-run preview that only reads current state passes while a leaked
// mutation fails the test. Returns a cfg pointed at the server and a pointer
// to the slice of HTTP methods the server observed (assert it equals
// [GET] to prove the preview never mutated).
func dryRunGetStateServer(t *testing.T, wantGetPath string, state any) (*config.Config, *[]string) {
	t.Helper()

	methods := &[]string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*methods = append(*methods, r.Method)

		if r.Method == http.MethodGet {
			assert.Equal(t, wantGetPath, r.URL.Path, "dry_run should read state from the resource path")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(state))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	return cfg, methods
}

// dryRunRouteServer serves a JSON body per GET path (exact match on
// r.URL.Path, ignoring query) and fails any non-GET request. It backs
// dependency-walk previews that read several resources in one dry-run.
// An unmatched GET path returns 404, which a best-effort walk treats as a
// missing dependency category rather than a hard error. Returns a cfg
// pointed at the server plus the observed HTTP methods (assert no DELETE
// or other mutation leaked).
func dryRunRouteServer(t *testing.T, routes map[string]any) (*config.Config, *[]string) {
	t.Helper()

	methods := &[]string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*methods = append(*methods, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	return cfg, methods
}
