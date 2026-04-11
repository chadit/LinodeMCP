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
)

const (
	shutdownTimeout = 10 * time.Second
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)

		return 1
	}

	// Initialize observability (tracing, metrics, logging, health)
	if err := observability.Init(&cfg.Observability); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize observability: %v\n", err)
		// Continue without observability
	}

	versionInfo := appinfo.Get()

	observability.Logger().Info("starting LinodeMCP Server")
	observability.Logger().Info("version info", "version", versionInfo.Version, "platform", versionInfo.Platform)
	observability.Logger().Info("server config", "name", cfg.Server.Name)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		observability.Logger().Info("shutdown signal received")
		cancel()
	}()

	srv, err := server.New(cfg)
	if err != nil {
		observability.Logger().Error("failed to create server", "error", err)

		return 1
	}

	if err := srv.Start(ctx); err != nil {
		observability.Logger().Error("server error", "error", err)

		return 1
	}

	// Shutdown observability
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := observability.Shutdown(shutdownCtx); err != nil {
		observability.Logger().Error("observability shutdown error", "error", err)
	}

	observability.Logger().Info("server shutdown complete")

	return 0
}
