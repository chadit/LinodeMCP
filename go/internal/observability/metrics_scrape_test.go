package observability_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// TestPrometheusEndpointExposesApplicationMetrics is the regression guard for
// the metrics-export bug: the instruments used to record into a reader that
// was never bridged to the scrape endpoint, so /metrics carried only the
// runtime collectors. This drives the real New -> record -> scrape path and
// asserts the application series are actually exposed. New binds the listener
// synchronously, so a scrape right after New does not race startup.
func TestPrometheusEndpointExposesApplicationMetrics(t *testing.T) {
	t.Parallel()

	baseCtx := t.Context()
	port := freePort(t)

	cfg := &config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: "error", Format: "json"},
		Metrics: config.MetricsConfig{
			Enabled: true,
			Prometheus: config.PrometheusConfig{
				Enabled: true,
				Port:    port,
				Path:    "/metrics",
			},
		},
	}

	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		// Detach from the test context (already canceled by cleanup time) so
		// Shutdown gets a live deadline of its own.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(baseCtx), 5*time.Second)
		defer cancel()

		if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	// A pull exporter only emits a series after it has been recorded, so seed
	// one tool call and one API call before scraping.
	obs.RecordRequest(baseCtx, "probe_tool", "execute", "success", 0.01)
	obs.RecordAPIRequest(baseCtx, "/regions", "GET", 200, 0.02)

	body := scrapeMetrics(t, port)

	wantSubstrings := []string{
		`linodemcp_requests_total{`,
		`tool="probe_tool"`,
		`status="success"`,
		`linodemcp_request_duration_seconds_count{`,
		`linodemcp_api_requests_total{`,
		`method="GET"`,
		// The Go runtime collector must survive the move to a per-instance
		// registry, so the endpoint still carries process-level metrics.
		"go_goroutines",
	}

	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("scraped /metrics missing %q\nbody:\n%s", want, body)
		}
	}
}

// freePort reserves an ephemeral TCP port and releases it so the metrics
// server can bind it. The brief reuse window is acceptable for a test.
func freePort(t *testing.T) int {
	t.Helper()

	var listenCfg net.ListenConfig

	listener, err := listenCfg.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr is %T, want *net.TCPAddr", listener.Addr())
	}

	port := addr.Port

	if closeErr := listener.Close(); closeErr != nil {
		t.Fatalf("release port: %v", closeErr)
	}

	return port
}

// scrapeMetrics fetches the metrics endpoint once and returns the body.
func scrapeMetrics(t *testing.T, port int) string {
	t.Helper()

	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("scrape %s: %v", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return string(data)
}
