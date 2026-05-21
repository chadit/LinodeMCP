// Package main provides the LinodeMCP server application entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
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

// auditLogger is the minimal logging surface the audit setup needs.
// Narrowed to an interface so the helpers can be tested with a stub
// and stay decoupled from the concrete *slog.Logger.
type auditLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

// setupAudit opens the JSONL sink, optionally adds the SQLite sink
// (when audit.sqlite.enabled), attaches the combined sink to the
// server, and starts the retention sweeper bound to ctx. Returns a
// cleanup func that closes whatever sinks opened; the cleanup is a
// no-op when the JSONL sink could not open (audit never blocks
// startup).
func setupAudit(ctx context.Context, srv *server.Server, cfg *config.Config, log *slog.Logger) func() {
	jsonlSink := openAuditSink(log)
	if jsonlSink == nil {
		return func() {}
	}

	closers := []func(){func() {
		if err := jsonlSink.Close(); err != nil {
			log.Warn("audit JSONL sink close error", "error", err)
		}
	}}

	var sink audit.Sink = jsonlSink

	if cfg.Audit.SQLite.Enabled {
		if sqliteSink := openSQLiteSink(ctx, cfg, jsonlSink, log); sqliteSink != nil {
			sink = audit.NewMultiSink(jsonlSink, sqliteSink)

			closers = append(closers, func() {
				if err := sqliteSink.Close(); err != nil {
					log.Warn("audit SQLite sink close error", "error", err)
				}
			})

			// Phase 3c: hourly retention sweep over the SQLite rows,
			// tied to ctx so it stops on shutdown.
			go sqliteSink.RunRetention(ctx, *cfg.Audit.RetentionDays, audit.DefaultRetentionSweepInterval, log)
		}
	}

	srv.SetAuditSink(sink)

	sweeper := audit.NewRetentionSweeper(
		filepath.Dir(jsonlSink.Path()),
		*cfg.Audit.RetentionDays,
		audit.WithSweepLogger(log),
	)

	go sweeper.Run(ctx)

	return func() {
		for _, closeSink := range closers {
			closeSink()
		}
	}
}

// openSQLiteSink resolves the SQLite database path (config value, or
// audit.db alongside the JSONL log) and opens the sink. Returns nil
// on failure after logging; the caller keeps the JSONL sink as the
// durable record.
func openSQLiteSink(ctx context.Context, cfg *config.Config, jsonlSink *audit.JSONLSink, log auditLogger) *audit.SQLiteSink {
	dbPath := cfg.Audit.SQLite.Path
	if dbPath == "" {
		dbPath = filepath.Join(filepath.Dir(jsonlSink.Path()), "audit.db")
	}

	sink, err := audit.NewSQLiteSink(ctx, dbPath, cfg.Audit.SQLite.BusyTimeoutMS)
	if err != nil {
		log.Warn("audit SQLite sink unavailable; continuing with JSONL only", "path", dbPath, "error", err)

		return nil
	}

	log.Info("audit SQLite sink open", "path", dbPath)

	return sink
}

// openAuditSink resolves the audit directory, opens a rolling
// JSONL sink under it, and returns the open sink ready to attach to
// the server. Returns nil on any failure after logging the cause;
// the caller falls back to the server's default NoopSink so audit
// never blocks startup.
func openAuditSink(log auditLogger) *audit.JSONLSink {
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

	// Phase 2a/2b/3b: open the JSONL sink (always on), add the SQLite
	// sink when audit.sqlite.enabled, attach the combined sink, and
	// start the retention sweeper. setupAudit returns a cleanup that
	// closes the sinks at shutdown, after the handler drain below.
	defer setupAudit(ctx, srv, cfg, log)()

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
