package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

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
	// healthStatusHealthy is the status value returned when all checks pass.
	healthStatusHealthy = "healthy"
)

// initHealth sets up health check endpoints if enabled.
func (o *Observability) initHealth(cfg config.HealthConfig) {
	if !cfg.Enabled {
		return
	}

	o.healthMu.Lock()
	defer o.healthMu.Unlock()

	if o.healthServer != nil {
		return
	}

	if o.healthChecks == nil {
		o.healthChecks = make(map[string]HealthCheck)
	}

	// Built-in config check: if we got this far, config loaded successfully.
	o.healthChecks["config"] = func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("health check context error: %w", err)
		}

		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Path+"/live", o.handleLive)
	mux.HandleFunc(cfg.Path+"/ready", o.handleReady)
	mux.HandleFunc(cfg.Path+"/healthz", o.handleReady)

	bindHost := cfg.Host
	if bindHost == "" {
		bindHost = config.DefaultBindHost
	}

	addr := net.JoinHostPort(bindHost, strconv.Itoa(cfg.Port))
	o.healthServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		o.logger.Info("starting health server", "address", addr)

		if err := o.healthServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			o.logger.Error("health server failed", "error", err)
		}
	}()

	o.registerShutdown(func(shutdownCtx context.Context) error {
		if o.healthServer != nil {
			return o.healthServer.Shutdown(shutdownCtx)
		}

		return nil
	})
}

// RegisterHealthCheck registers a new health check by name.
func (o *Observability) RegisterHealthCheck(name string, check HealthCheck) {
	o.healthMu.Lock()
	defer o.healthMu.Unlock()

	if o.healthChecks == nil {
		o.healthChecks = make(map[string]HealthCheck)
	}

	o.healthChecks[name] = check
}

// UnregisterHealthCheck removes a health check by name.
func (o *Observability) UnregisterHealthCheck(name string) {
	o.healthMu.Lock()
	defer o.healthMu.Unlock()

	delete(o.healthChecks, name)
}

// handleLive serves /live: is the process alive?
// Receiver-less (no instance state needed) so the linter is satisfied; kept on
// the type as a method so route wiring stays consistent with handleReady.
func (*Observability) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(HealthResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC(),
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// handleReady serves /ready and /healthz: are all dependencies healthy?
func (o *Observability) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	response := o.checkAllHealth(ctx)

	w.Header().Set("Content-Type", "application/json")

	if response.Status != healthStatusHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// checkAllHealth runs every registered check under one read lock.
func (o *Observability) checkAllHealth(ctx context.Context) HealthResponse {
	o.healthMu.RLock()
	defer o.healthMu.RUnlock()

	response := HealthResponse{
		Status:    healthStatusHealthy,
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]HealthStatus),
	}

	for name, check := range o.healthChecks {
		status := HealthStatus{Status: healthStatusHealthy}

		if err := check(ctx); err != nil {
			status.Status = "unhealthy"
			status.Message = err.Error()
			response.Status = "unhealthy"
		}

		response.Checks[name] = status
	}

	return response
}
