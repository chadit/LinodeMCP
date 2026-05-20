// Package main provides the LinodeMCP server application entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/observability"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	shutdownTimeout = 10 * time.Second
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "profile" {
		os.Exit(runProfileCommand(os.Args[2:]))
	}

	exitCode := run()
	os.Exit(exitCode)
}

// openAuditSink resolves the audit directory, opens a rolling
// JSONL sink under it, and returns the open sink ready to attach to
// the server. Returns nil on any failure after logging the cause;
// the caller falls back to the server's default NoopSink so audit
// never blocks startup.
func openAuditSink(log interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
},
) *audit.JSONLSink {
	auditDir := audit.ResolveDefaultAuditDir()

	sink, err := audit.NewJSONLSink(auditDir)
	if err != nil {
		log.Warn("audit JSONL sink unavailable; continuing without audit", "dir", auditDir, "error", err)

		return nil
	}

	log.Info("audit JSONL sink open", "path", sink.Path())

	return sink
}

// runScopeValidation enforces the Phase 6.4 token-scope policy at
// startup. Returns 0 to continue startup, non-zero to abort.
//
// Policy:
//   - Missing required scopes: always fail (the AI would hit auth
//     errors mid-call; better to fail at load).
//   - Excess scopes: warn (least-privilege signal) but continue.
//   - API failure or no-token configured: fail for elevated profiles
//     (any tool needs write access), warn for read-only profiles
//     (tool discovery works without credentials).
func runScopeValidation(
	ctx context.Context,
	srv *server.Server,
	active *profiles.Profile,
	log interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	},
) int {
	result, err := srv.ValidateScopes(ctx)
	if err != nil {
		elevated := profiles.ProfileIsElevated(active)

		switch {
		case errors.Is(err, profiles.ErrTokenNotConfigured):
			if elevated {
				log.Error(
					"profile requires a Linode token; configure environments.<env>.linode.token",
					"profile", active.Name,
				)

				return 1
			}

			log.Warn(
				"no Linode token configured; read-only profile starts but API calls will fail",
				"profile", active.Name,
			)

			return 0
		default:
			if elevated {
				log.Error(
					"token-scope validation failed; profile requires write access so refusing to start",
					"profile", active.Name,
					"error", err,
				)

				return 1
			}

			log.Warn(
				"token-scope validation failed; read-only profile continues without verified token",
				"profile", active.Name,
				"error", err,
			)

			return 0
		}
	}

	if result.Comparison.HasMissing() {
		log.Error(
			"active token is missing scopes the profile requires; refusing to start",
			"profile", active.Name,
			"token_kind", result.Kind.String(),
			"missing", result.Comparison.Missing,
		)

		return 1
	}

	if result.Comparison.HasExcess() {
		log.Warn(
			"active token carries more scopes than the profile requires (least-privilege violated)",
			"profile", active.Name,
			"token_kind", result.Kind.String(),
			"excess", result.Comparison.Excess,
		)
	}

	log.Info(
		"token-scope validation passed",
		"profile", active.Name,
		"token_kind", result.Kind.String(),
		"username", result.Profile.Username,
	)

	return 0
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

	// Phase 2a: replace the default NoopSink with a rolling JSONL
	// writer so every tool call (success, error, refusal) lands on
	// disk. If the sink fails to open, log and continue with the
	// noop default; audit is a best-effort observation channel, not
	// a hard prerequisite for the server.
	if auditSink := openAuditSink(log); auditSink != nil {
		srv.SetAuditSink(auditSink)

		defer func() {
			if closeErr := auditSink.Close(); closeErr != nil {
				log.Warn("audit JSONL sink close error", "error", closeErr)
			}
		}()
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

	// Phase 6.4c: validate the active token's scopes against the active
	// profile's required scopes. Missing scopes always fail load; an
	// API failure or missing token fails for elevated profiles and
	// warns-and-continues for read-only ones. Excess scopes warn only.
	active := srv.ActiveProfile()
	if exitCode := runScopeValidation(ctx, srv, &active, log); exitCode != 0 {
		return exitCode
	}

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
