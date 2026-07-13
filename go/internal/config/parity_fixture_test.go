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
	// Blank the env overrides both loaders honor so a developer shell with
	// LINODEMCP_* set cannot change what the fixture parses to. t.Setenv also
	// restores prior values on cleanup, and its presence (as direct calls, not
	// in a loop, which paralleltest cannot see through) is why this test runs
	// serial: t.Setenv forbids t.Parallel.
	t.Setenv("LINODEMCP_SERVER_NAME", "")
	t.Setenv("LINODEMCP_LOG_LEVEL", "")
	t.Setenv("LINODEMCP_LINODE_API_URL", "")
	t.Setenv("LINODEMCP_LINODE_TOKEN", "")
	t.Setenv("LINODEMCP_METRICS_ENABLED", "")
	t.Setenv("LINODEMCP_METRICS_PORT", "")
	t.Setenv("LINODEMCP_TRACING_ENABLED", "")
	t.Setenv("LINODEMCP_TRACING_ENDPOINT", "")
	t.Setenv("LINODEMCP_TRACING_SAMPLE_RATE", "")
	t.Setenv("LINODEMCP_HEALTH_ENABLED", "")

	cfg, err := config.Load(filepath.Join("..", "..", "..", "testdata", "config", "parity.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env, ok := cfg.Environments["default"]
	if !ok {
		t.Fatal("Environments missing key \"default\"")
	}

	checks := []struct {
		field string
		got   any
		want  any
	}{
		{"server.name", cfg.Server.Name, "ParityCheck"},
		{"server.logLevel", cfg.Server.LogLevel, "debug"},
		{"server.transport", cfg.Server.Transport, "stdio"},
		{"server.host", cfg.Server.Host, "127.0.0.2"},
		{"server.port", cfg.Server.Port, 8180},
		{"metrics.enabled", cfg.Observability.Metrics.Enabled, true},
		{"metrics.runtime", cfg.Observability.Metrics.Runtime, false},
		{"metrics.host", cfg.Observability.Metrics.Host, true},
		{"metrics.prometheus.enabled", cfg.Observability.Metrics.Prometheus.Enabled, true},
		{"metrics.prometheus.host", cfg.Observability.Metrics.Prometheus.Host, "192.0.2.7"},
		{"metrics.prometheus.port", cfg.Observability.Metrics.Prometheus.Port, 9101},
		{"metrics.prometheus.path", cfg.Observability.Metrics.Prometheus.Path, "/parity-metrics"},
		{"tracing.enabled", cfg.Observability.Tracing.Enabled, true},
		{"tracing.endpoint", cfg.Observability.Tracing.Endpoint, "collector.example.internal:4317"},
		{"tracing.protocol", cfg.Observability.Tracing.Protocol, "http"},
		{"tracing.insecure", cfg.Observability.Tracing.Insecure, true},
		{"tracing.sampleRate", cfg.Observability.Tracing.SampleRate, 0.25},
		{"tracing.headers length", len(cfg.Observability.Tracing.Headers), 1},
		{"tracing.headers[x-parity]", cfg.Observability.Tracing.Headers["x-parity"], "check"},
		{"logging.level", cfg.Observability.Logging.Level, "warn"},
		{"logging.format", cfg.Observability.Logging.Format, "text"},
		{"health.enabled", cfg.Observability.Health.Enabled, true},
		{"health.host", cfg.Observability.Health.Host, "192.0.2.8"},
		{"health.port", cfg.Observability.Health.Port, 9102},
		{"health.path", cfg.Observability.Health.Path, "/parity-health"},
		{"resilience.rateLimitPerMinute", cfg.Resilience.RateLimitPerMinute, 500},
		{"resilience.circuitBreakerThreshold", cfg.Resilience.CircuitBreakerThreshold, 7},
		{"resilience.circuitBreakerTimeout", cfg.Resilience.CircuitBreakerTimeout, 45 * time.Second},
		{"resilience.maxRetries", cfg.Resilience.MaxRetries, 2},
		{"resilience.baseRetryDelay", cfg.Resilience.BaseRetryDelay, 250 * time.Millisecond},
		{"resilience.maxRetryDelay", cfg.Resilience.MaxRetryDelay, 90 * time.Second},
		{"environment.label", env.Label, "Parity"},
		{"environment.linode.apiUrl", env.Linode.APIURL, "https://api.linode.com/v4"},
		{"environment.linode.token", env.Linode.Token, "parity-test-token"},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v, want %v", check.field, check.got, check.want)
		}
	}
}
