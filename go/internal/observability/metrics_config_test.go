package observability_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

const (
	metricsProbePath = "/metrics"

	// Series names the optional collectors publish, spelled once so each test
	// asserts the same wire name.
	runtimeSeries = "go_goroutine_count{"
	hostSeries    = "system_memory_usage_bytes{"
)

// newMetricsInstance starts an instance with the Prometheus endpoint on a free
// port and the optional collectors set as the caller's cfg says. It returns the
// instance and the port the endpoint answers on.
func newMetricsInstance(t *testing.T, cfg config.MetricsConfig) (*observability.Observability, int) {
	t.Helper()

	baseCtx := t.Context()
	port := freePort(t)

	cfg.Enabled = true
	cfg.Prometheus = config.PrometheusConfig{Enabled: true, Host: config.DefaultBindHost, Port: port, Path: metricsProbePath}

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Metrics: cfg,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(baseCtx), 5*time.Second)
		defer cancel()

		if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	return obs, port
}

// TestRuntimeMetricsAppearOnTheScrapeEndpoint pins what the runtime switch
// buys: the Go runtime instruments are bridged to the same endpoint as the
// application series, so a scrape carries them under the contrib scope.
func TestRuntimeMetricsAppearOnTheScrapeEndpoint(t *testing.T) {
	t.Parallel()

	_, port := newMetricsInstance(t, config.MetricsConfig{Runtime: true})

	body := scrapeMetrics(t, port)

	if !strings.Contains(body, runtimeSeries) {
		t.Errorf("scraped /metrics with runtime enabled lacks %q\nbody:\n%s", runtimeSeries, body)
	}

	if !strings.Contains(body, `otel_scope_name="go.opentelemetry.io/contrib/instrumentation/runtime"`) {
		t.Error("runtime series are not attributed to the contrib runtime scope")
	}
}

// TestHostMetricsAppearOnTheScrapeEndpoint is the host-switch counterpart:
// system memory and CPU series show up only because host.Start was handed
// this instance's meter provider.
func TestHostMetricsAppearOnTheScrapeEndpoint(t *testing.T) {
	t.Parallel()

	_, port := newMetricsInstance(t, config.MetricsConfig{Host: true})

	body := scrapeMetrics(t, port)

	if !strings.Contains(body, hostSeries) {
		t.Errorf("scraped /metrics with host enabled lacks %q\nbody:\n%s", hostSeries, body)
	}

	if !strings.Contains(body, `otel_scope_name="go.opentelemetry.io/contrib/instrumentation/host"`) {
		t.Error("host series are not attributed to the contrib host scope")
	}
}

// TestOptionalCollectorsStayOffUnlessEnabled is the negative half of the two
// switches: with both off, neither collector's series leak onto the endpoint,
// so an operator who left them out of config does not pay for them.
func TestOptionalCollectorsStayOffUnlessEnabled(t *testing.T) {
	t.Parallel()

	_, port := newMetricsInstance(t, config.MetricsConfig{})

	body := scrapeMetrics(t, port)

	if strings.Contains(body, runtimeSeries) {
		t.Errorf("scraped /metrics carries %q with runtime disabled", runtimeSeries)
	}

	if strings.Contains(body, hostSeries) {
		t.Errorf("scraped /metrics carries %q with host disabled", hostSeries)
	}
}

// TestRecordErrorAppearsOnTheScrapeEndpoint drives the error counter through
// the real export path: once metrics are on, a recorded error must surface as
// a linodemcp_errors_total series labeled with the tool and error type.
func TestRecordErrorAppearsOnTheScrapeEndpoint(t *testing.T) {
	t.Parallel()

	obs, port := newMetricsInstance(t, config.MetricsConfig{})

	obs.RecordError(t.Context(), "probe_tool", "timeout")

	body := scrapeMetrics(t, port)

	for _, want := range []string{"linodemcp_errors_total{", `error_type="timeout"`, `tool="probe_tool"`} {
		if !strings.Contains(body, want) {
			t.Errorf("scraped /metrics missing %q after RecordError\nbody:\n%s", want, body)
		}
	}
}

// TestMetricsPortAlreadyInUseLeavesNewUsable covers the bind failure an
// operator hits when two instances share a port. Metrics are best-effort, so
// New must still hand back a working instance, must not evict whoever holds
// the port, and must shut down cleanly with nothing to tear down.
func TestMetricsPortAlreadyInUseLeavesNewUsable(t *testing.T) {
	t.Parallel()

	occupant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("occupant")); err != nil {
			t.Errorf("occupant write: %v", err)
		}
	}))
	defer occupant.Close()

	tcpAddr, ok := occupant.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("occupant addr is %T, want *net.TCPAddr", occupant.Listener.Addr())
	}

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Metrics: config.MetricsConfig{
			Enabled:    true,
			Prometheus: config.PrometheusConfig{Enabled: true, Host: config.DefaultBindHost, Port: tcpAddr.Port, Path: metricsProbePath},
		},
	})
	if err != nil {
		t.Fatalf("New with the metrics port in use returned %v, want the instance without metrics", err)
	}

	// The instruments never came up, so recording must be a no-op rather than
	// a nil dereference.
	obs.RecordRequest(t.Context(), "probe_tool", "execute", "success", 0.01)
	obs.RecordError(t.Context(), "probe_tool", "timeout")

	if got := scrapeMetrics(t, tcpAddr.Port); got != "occupant" {
		t.Errorf("port %d answers %q, want the occupant's body: the failed bind must not displace it", tcpAddr.Port, got)
	}

	if err := obs.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown after a failed metrics bind returned %v, want nil", err)
	}
}

// TestServiceVersionComesFromTheEnvironment pins how the build version reaches
// the exported resource: LINODEMCP_VERSION lands on target_info's
// service_version label, which is what dashboards key rollouts on. Setenv
// rules out t.Parallel, so the test runs in the sequential phase.
func TestServiceVersionComesFromTheEnvironment(t *testing.T) {
	t.Setenv("LINODEMCP_VERSION", "9.9.9-probe")

	_, port := newMetricsInstance(t, config.MetricsConfig{})

	body := scrapeMetrics(t, port)

	if !strings.Contains(body, `service_version="9.9.9-probe"`) {
		t.Errorf("scraped /metrics target_info lacks the version from LINODEMCP_VERSION\nbody:\n%s", body)
	}
}
