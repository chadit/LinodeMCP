package config_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

const allInterfaces = "0.0.0.0"

// TestBindHostDefaults locks the secure-by-default bind host for the metrics
// and health servers: loopback unless an operator overrides it. The Python
// side asserts the same default (test_config.py test_bind_host_defaults), so
// the two languages stay at parity.
func TestBindHostDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(""))

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.Observability.Metrics.Prometheus.Host; got != config.DefaultBindHost {
		t.Errorf("Prometheus.Host = %q, want %q", got, config.DefaultBindHost)
	}

	if got := cfg.Observability.Health.Host; got != config.DefaultBindHost {
		t.Errorf("Health.Host = %q, want %q", got, config.DefaultBindHost)
	}
}

// TestBindHostOverride confirms an explicit host is honored, so an operator
// can expose the endpoints on all interfaces for remote scraping.
func TestBindHostOverride(t *testing.T) {
	t.Parallel()

	const block = `
observability:
  metrics:
    enabled: true
    prometheus:
      host: "0.0.0.0"
  health:
    enabled: true
    host: "0.0.0.0"
`

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(block))

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.Observability.Metrics.Prometheus.Host; got != allInterfaces {
		t.Errorf("Prometheus.Host = %q, want %q", got, allInterfaces)
	}

	if got := cfg.Observability.Health.Host; got != allInterfaces {
		t.Errorf("Health.Host = %q, want %q", got, allInterfaces)
	}
}
