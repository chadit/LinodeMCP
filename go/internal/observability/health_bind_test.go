package observability_test

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// TestHealthWithoutAHostBindsTheLoopbackDefault covers the config most
// operators write: a port and no host. The server must come up on the
// loopback default rather than refusing to start or binding somewhere a
// local probe cannot reach.
func TestHealthWithoutAHostBindsTheLoopbackDefault(t *testing.T) {
	t.Parallel()

	baseCtx := t.Context()
	port := freePort(t)

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Health: config.HealthConfig{
			Enabled: true,
			Host:    "",
			Port:    port,
			Path:    healthProbePath,
		},
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

	address := net.JoinHostPort(config.DefaultBindHost, strconv.Itoa(port))
	waitForListener(t, address)

	status, _ := probe(t, "http://"+address+healthProbePath+"/live")
	if status != http.StatusOK {
		t.Errorf("GET /live on %s status = %d, want %d with no host configured", address, status, http.StatusOK)
	}
}
