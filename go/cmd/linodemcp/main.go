// Package main provides the LinodeMCP server application entry point.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/server"
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)

		return 1
	}

	versionInfo := appinfo.Get()

	log.Printf("Starting LinodeMCP Server")
	log.Printf("Version: %s", versionInfo.Version)
	log.Printf("Server: %s", cfg.Server.Name)
	log.Printf("Platform: %s", versionInfo.Platform)
	log.Printf("Git Commit: %s", versionInfo.GitCommit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("Shutdown signal received")
		cancel()
	}()

	srv, err := server.New(cfg)
	if err != nil {
		log.Printf("Failed to create server: %v", err)

		return 1
	}

	if err := srv.Start(ctx); err != nil {
		log.Printf("Server error: %v", err)

		return 1
	}

	log.Printf("Server shutdown complete")

	return 0
}
