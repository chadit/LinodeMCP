package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// TestSharedConfigParityFixture loads testdata/config/parity.yml, the fixture
// the Python suite also loads (tests/unit/test_config_parity_fixture.py), and
// asserts every field. The two tests pin the same values, so a loader that
// reads a shared key differently from the other implementation fails here
// instead of surfacing later as a config-file incompatibility.
func TestSharedConfigParityFixture(t *testing.T) {
	// Blank the env overrides both loaders honor (the docs/contracts/env-vars.txt
	// surface; observability has none by design) so a developer shell with
	// LINODEMCP_* set cannot change what the fixture parses to. t.Setenv also
	// restores prior values on cleanup, and its presence (as direct calls, not
	// in a loop, which paralleltest cannot see through) is why this test runs
	// serial: t.Setenv forbids t.Parallel.
	t.Setenv("LINODEMCP_SERVER_NAME", "")
	t.Setenv("LINODEMCP_LOG_LEVEL", "")
	t.Setenv("LINODEMCP_LINODE_API_URL", "")
	t.Setenv("LINODEMCP_LINODE_TOKEN", "")

	cfg, err := config.Load(filepath.Join("..", "..", "..", "testdata", "config", "parity.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env, ok := cfg.Environments["default"]
	if !ok {
		t.Fatal("Environments missing key \"default\"")
	}

	checks := []struct {
		got   any
		want  any
		field string
	}{
		{cfg.Server.Name, "ParityCheck", "server.name"},
		{cfg.Server.LogLevel, "debug", "server.logLevel"},
		{cfg.Server.Transport, "stdio", "server.transport"},
		{cfg.Server.Host, "127.0.0.2", "server.host"},
		{cfg.Server.Port, 8180, "server.port"},
		{cfg.Observability.Metrics.Enabled, true, "metrics.enabled"},
		{cfg.Observability.Metrics.Runtime, false, "metrics.runtime"},
		{cfg.Observability.Metrics.Host, true, "metrics.host"},
		{cfg.Observability.Metrics.Prometheus.Enabled, true, "metrics.prometheus.enabled"},
		{cfg.Observability.Metrics.Prometheus.Host, "192.0.2.7", "metrics.prometheus.host"},
		{cfg.Observability.Metrics.Prometheus.Port, 9101, "metrics.prometheus.port"},
		{cfg.Observability.Metrics.Prometheus.Path, "/parity-metrics", "metrics.prometheus.path"},
		{cfg.Observability.Tracing.Enabled, true, "tracing.enabled"},
		{cfg.Observability.Tracing.Endpoint, "collector.example.internal:4317", "tracing.endpoint"},
		{cfg.Observability.Tracing.Protocol, "http", "tracing.protocol"},
		{cfg.Observability.Tracing.Insecure, true, "tracing.insecure"},
		{cfg.Observability.Tracing.SampleRate, 0.25, "tracing.sampleRate"},
		{len(cfg.Observability.Tracing.Headers), 1, "tracing.headers length"},
		{cfg.Observability.Tracing.Headers["x-parity"], "check", "tracing.headers[x-parity]"},
		{cfg.Observability.Logging.Level, "warn", "logging.level"},
		{cfg.Observability.Logging.Format, "text", "logging.format"},
		{cfg.Observability.Health.Enabled, true, "health.enabled"},
		{cfg.Observability.Health.Host, "192.0.2.8", "health.host"},
		{cfg.Observability.Health.Port, 9102, "health.port"},
		{cfg.Observability.Health.Path, "/parity-health", "health.path"},
		{cfg.Resilience.RateLimitPerMinute, 500, "resilience.rateLimitPerMinute"},
		{cfg.Resilience.CircuitBreakerThreshold, 7, "resilience.circuitBreakerThreshold"},
		{cfg.Resilience.CircuitBreakerTimeout, 45 * time.Second, "resilience.circuitBreakerTimeout"},
		{cfg.Resilience.MaxRetries, 2, "resilience.maxRetries"},
		{cfg.Resilience.BaseRetryDelay, 250 * time.Millisecond, "resilience.baseRetryDelay"},
		{cfg.Resilience.MaxRetryDelay, 90 * time.Second, "resilience.maxRetryDelay"},
		{env.Label, "Parity", "environment.label"},
		{env.Linode.APIURL, "https://api.linode.com/v4", "environment.linode.apiUrl"},
		{env.Linode.Token, "parity-test-token", "environment.linode.token"},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v, want %v", check.field, check.got, check.want)
		}
	}
}
