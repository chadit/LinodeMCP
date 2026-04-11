package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
)

// healthState holds health check state.
type healthState struct {
	mu          sync.RWMutex
	checks      map[string]HealthCheck
	server      *http.Server
	initialized bool
}

//nolint:gochecknoglobals // Singleton pattern required for health check infrastructure
var health healthState

// HealthCheck is a function that checks the health of a dependency.
type HealthCheck func(context.Context) error

// HealthStatus represents the status of a health check.
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents the response from the health endpoint.
type HealthResponse struct {
	Status    string                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]HealthStatus `json:"checks"`
}

const (
	healthCheckTimeout = 5 * time.Second
)

// initHealth sets up health check endpoints.
func initHealth(cfg config.HealthConfig) {
	if !cfg.Enabled {
		return
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	if health.initialized {
		return
	}

	health.checks = make(map[string]HealthCheck)

	// Register built-in health checks
	RegisterHealthCheck("config", func(ctx context.Context) error {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("health check context error: %w", err)
		}

		// Config is loaded at startup, so if we're here, it's loaded
		return nil
	})

	// Start health check server
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Path+"/live", handleLive)
	mux.HandleFunc(cfg.Path+"/ready", handleReady)
	mux.HandleFunc(cfg.Path+"/healthz", handleHealthz)

	addr := fmt.Sprintf(":%d", cfg.Port)
	health.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		Logger().Info("starting health server", "address", addr)

		if err := health.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			Logger().Error("health server failed", "error", err)
		}
	}()

	registerShutdown(func(shutdownCtx context.Context) error {
		if health.server != nil {
			return health.server.Shutdown(shutdownCtx)
		}

		return nil
	})

	health.initialized = true
}

// RegisterHealthCheck registers a new health check.
func RegisterHealthCheck(name string, check HealthCheck) {
	health.mu.Lock()
	defer health.mu.Unlock()

	if health.checks == nil {
		health.checks = make(map[string]HealthCheck)
	}

	health.checks[name] = check
}

// UnregisterHealthCheck removes a health check.
func UnregisterHealthCheck(name string) {
	health.mu.Lock()
	defer health.mu.Unlock()

	delete(health.checks, name)
}

// handleLive handles the /live endpoint (is the process alive?).
func handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(HealthResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC(),
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// handleReady handles the /ready endpoint (are all dependencies healthy?).
func handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	response := checkAllHealth(ctx)

	w.Header().Set("Content-Type", "application/json")

	if response.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// handleHealthz handles the /healthz endpoint (combined health check).
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	handleReady(w, r)
}

// checkAllHealth runs all health checks and returns the status.
func checkAllHealth(ctx context.Context) HealthResponse {
	health.mu.RLock()
	defer health.mu.RUnlock()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]HealthStatus),
	}

	for name, check := range health.checks {
		status := HealthStatus{Status: "healthy"}

		if err := check(ctx); err != nil {
			status.Status = "unhealthy"
			status.Message = err.Error()
			response.Status = "unhealthy"
		}

		response.Checks[name] = status
	}

	return response
}
