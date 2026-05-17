// Package main provides the LinodeMCP server application entry point.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/observability"
	"github.com/chadit/LinodeMCP/internal/server"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	shutdownTimeout = 10 * time.Second
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	configPath := config.GetConfigPath()

	watcher, err := config.NewWatcher(configPath, config.DefaultWatchInterval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)

		return 1
	}
	defer watcher.Close()

	cfg := watcher.Get()

	obs, err := observability.New(&cfg.Observability)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize observability: %v\n", err)
		// Continue with whatever obs returned (always non-nil on err==nil; on
		// err!=nil we still want a logger). New only returns err on hard
		// failures, but defend against that path anyway.
		obs, _ = observability.New(nil)
	}

	log := obs.Logger()
	versionInfo := appinfo.Get()

	log.Info("starting LinodeMCP Server")
	log.Info("version info", "version", versionInfo.Version, "platform", versionInfo.Platform)
	log.Info("server config", "name", cfg.Server.Name)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("shutdown signal received")
		cancel()
	}()

	// Bridge the watcher to tool handlers. Subsequent tool calls read the
	// latest cfg through this hook so reloaded resilience and environment
	// settings take effect without restart.
	tools.SetLiveConfigSource(watcher.Get)
	defer tools.SetLiveConfigSource(nil)

	// Drain reload errors in the background. Advisory; failed reloads keep
	// the previous Config in place.
	go func() {
		for reloadErr := range watcher.Errors() {
			log.Warn("config reload failed", "error", reloadErr)
		}
	}()

	srv, err := server.New(cfg)
	if err != nil {
		log.Error("failed to create server", "error", err)

		return 1
	}

	// Phase 5: a config reload that changes active_profile or the active
	// profile's contents must re-resolve the running tool surface. The
	// callback runs in the watcher's polling goroutine; ReloadProfile is
	// fast (diff + mcp-go DeleteTools/AddTool) so synchronous is fine.
	// A failed reload is logged and the previous profile stays active.
	watcher.SetOnChange(func(newCfg *config.Config) {
		if reloadErr := srv.ReloadProfile(newCfg); reloadErr != nil {
			log.Warn("profile reload failed", "error", reloadErr)
		}
	})

	watcher.Start(ctx)

	if err := srv.Start(ctx); err != nil {
		log.Error("server error", "error", err)

		return 1
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Drain in-flight tool handlers before tearing down observability so
	// active write operations get a chance to log their final state.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown drain error", "error", err)
	}

	if err := obs.Shutdown(shutdownCtx); err != nil {
		log.Error("observability shutdown error", "error", err)
	}

	log.Info("server shutdown complete")

	return 0
}
