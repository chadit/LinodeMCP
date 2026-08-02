package observability_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// healthProbePath is the prefix the health endpoints are mounted under in
// these tests. The instance appends /live, /ready, and /healthz to it.
const (
	healthProbePath = "/health"

	// Log settings the health instances share; quiet logs keep the test
	// output readable.
	logLevelError  = "error"
	logFormatJSON  = "json"
	probeDialLimit = 10000
)

// errDependencyDown is the failure a registered check reports so the readiness
// endpoint has something concrete to turn into a 503.
var errDependencyDown = errors.New("dependency is unreachable")

// newHealthServer starts an instance with only the health endpoints enabled
// and returns the base URL its probes are reachable at.
func newHealthServer(t *testing.T) (*observability.Observability, string) {
	t.Helper()

	baseCtx := t.Context()
	port := freePort(t)

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Health: config.HealthConfig{
			Enabled: true,
			Host:    "127.0.0.1",
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

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	waitForListener(t, address)

	return obs, "http://" + address
}

// waitForListener blocks until the health server has bound its port. New
// starts ListenAndServe on its own goroutine, so the first probe can arrive
// before the socket exists; yielding to the scheduler rather than sleeping
// keeps the wait off the wall clock.
func waitForListener(t *testing.T, address string) {
	t.Helper()

	var dialer net.Dialer

	for range probeDialLimit {
		conn, err := dialer.DialContext(t.Context(), "tcp", address)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				t.Fatalf("close probe connection: %v", closeErr)
			}

			return
		}

		runtime.Gosched()
	}

	t.Fatalf("health server never bound %s", address)
}

// probeClient is the HTTP client the probes use. An explicit timeout keeps a
// wedged handler from hanging the whole package run.
func probeClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// probe issues a GET against one health endpoint and returns its status code
// and body.
func probe(t *testing.T, url string) (int, []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request for %s: %v", url, err)
	}

	resp, err := probeClient().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	body, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close response body: %v", closeErr)
	}

	if readErr != nil {
		t.Fatalf("read response body: %v", readErr)
	}

	return resp.StatusCode, body
}

// TestLiveEndpointAnswersWithoutConsultingChecks pins the liveness contract:
// /live reports on the process itself, so a failing dependency check must not
// pull it down. Only readiness is allowed to fail on a sick dependency.
func TestLiveEndpointAnswersWithoutConsultingChecks(t *testing.T) {
	t.Parallel()

	obs, base := newHealthServer(t)

	obs.RegisterHealthCheck("upstream", func(context.Context) error {
		return errDependencyDown
	})

	status, body := probe(t, base+healthProbePath+"/live")

	if status != http.StatusOK {
		t.Errorf("GET /live status = %d, want %d even with a failing check registered",
			status, http.StatusOK)
	}

	var decoded observability.HealthResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode /live body %q: %v", body, err)
	}

	if decoded.Status != "alive" {
		t.Errorf("/live reported status %q, want %q", decoded.Status, "alive")
	}

	if decoded.Timestamp.IsZero() {
		t.Error("/live reported a zero timestamp, want the time the probe was served")
	}
}

// TestReadyEndpointReflectsRegisteredChecks drives the readiness path through
// its three states: the built-in check alone, a failing dependency, and that
// same dependency unregistered again.
func TestReadyEndpointReflectsRegisteredChecks(t *testing.T) {
	t.Parallel()

	obs, base := newHealthServer(t)
	readyURL := base + healthProbePath + "/ready"

	status, body := probe(t, readyURL)
	if status != http.StatusOK {
		t.Fatalf("GET /ready status = %d, want %d with only the built-in check", status, http.StatusOK)
	}

	var healthy observability.HealthResponse
	if err := json.Unmarshal(body, &healthy); err != nil {
		t.Fatalf("decode /ready body %q: %v", body, err)
	}

	if healthy.Status != "healthy" {
		t.Errorf("/ready reported status %q, want %q", healthy.Status, "healthy")
	}

	if _, ok := healthy.Checks["config"]; !ok {
		t.Errorf("/ready checks = %v, want the built-in config check present", healthy.Checks)
	}

	obs.RegisterHealthCheck("upstream", func(context.Context) error {
		return errDependencyDown
	})

	if status, _ = probe(t, readyURL); status != http.StatusServiceUnavailable {
		t.Errorf("GET /ready status = %d, want %d while a check is failing",
			status, http.StatusServiceUnavailable)
	}

	obs.UnregisterHealthCheck("upstream")

	if status, _ = probe(t, readyURL); status != http.StatusOK {
		t.Errorf("GET /ready status = %d, want %d after the failing check was unregistered",
			status, http.StatusOK)
	}
}

// TestHealthzMirrorsReady locks the alias in place: /healthz is the same
// readiness handler under the name orchestrators expect, so the two endpoints
// must not drift apart.
func TestHealthzMirrorsReady(t *testing.T) {
	t.Parallel()

	obs, base := newHealthServer(t)

	obs.RegisterHealthCheck("upstream", func(context.Context) error {
		return fmt.Errorf("probe: %w", errDependencyDown)
	})

	readyStatus, _ := probe(t, base+healthProbePath+"/ready")
	healthzStatus, _ := probe(t, base+healthProbePath+"/healthz")

	if readyStatus != healthzStatus {
		t.Errorf("/healthz status = %d, want the same %d /ready reported", healthzStatus, readyStatus)
	}

	if healthzStatus != http.StatusServiceUnavailable {
		t.Errorf("/healthz status = %d, want %d while a check is failing",
			healthzStatus, http.StatusServiceUnavailable)
	}
}

// TestRegisterHealthCheckReplacesByName covers the last-writer-wins contract
// on the check map: registering twice under one name must swap the check
// rather than accumulate both, otherwise a repaired dependency could never
// clear its own failure.
func TestRegisterHealthCheckReplacesByName(t *testing.T) {
	t.Parallel()

	obs, base := newHealthServer(t)
	readyURL := base + healthProbePath + "/ready"

	obs.RegisterHealthCheck("upstream", func(context.Context) error {
		return errDependencyDown
	})

	if status, _ := probe(t, readyURL); status != http.StatusServiceUnavailable {
		t.Fatalf("GET /ready status = %d, want %d after the failing check registered",
			status, http.StatusServiceUnavailable)
	}

	obs.RegisterHealthCheck("upstream", func(context.Context) error { return nil })

	if status, _ := probe(t, readyURL); status != http.StatusOK {
		t.Errorf("GET /ready status = %d, want %d after the same name re-registered healthy",
			status, http.StatusOK)
	}
}

// TestHealthDisabledOpensNoListener is the negative half of the config switch:
// with health off, nothing should be listening on the port at all.
func TestHealthDisabledOpensNoListener(t *testing.T) {
	t.Parallel()

	port := freePort(t)

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Health: config.HealthConfig{
			Enabled: false,
			Host:    "127.0.0.1",
			Port:    port,
			Path:    healthProbePath,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		if shutdownErr := obs.Shutdown(t.Context()); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		"http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(port))+healthProbePath+"/live", http.NoBody,
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := probeClient().Do(req)
	if err == nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}

		t.Fatalf("GET /live returned %d, want a connection failure with health disabled", resp.StatusCode)
	}
}
