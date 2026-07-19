package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// auditDBName is the SQLite filename used when audit.sqlite.path is
// empty: the database sits beside the JSONL log, matching the serve path.
const auditDBName = "audit.db"

// Runtime is a server instance wired for a CLI or TUI session. It holds
// the constructed server with its audit sink attached, the config it was
// built from, and a close func that flushes and shuts the sinks. Unlike
// the long-running serve path it starts no retention sweeper, no config
// watcher, and no observability HTTP listener, so a `call` never binds a
// port or leaves background goroutines running after the command returns.
//
// A one-shot `call` builds it, dispatches once, and closes. The TUI holds
// one open for the whole session: build once, dispatch many, close on
// exit. The exported Config lets the TUI read the configured environments
// for its environment picker without a second config load.
type Runtime struct {
	Server *server.Server
	Config *config.Config
	close  func()
}

// Close flushes and shuts the audit sinks opened for this invocation.
// Safe to call once; intended for a defer at the call site.
func (r *Runtime) Close() {
	if r.close != nil {
		r.close()
	}
}

// quietStartupLogging points the process-global slog logger at stderr but
// raises its level to Warn, so server.New's per-tool "profile filtered
// out tool at registration" INFO chatter doesn't flood a one-shot CLI
// command. Genuine warnings and errors still surface. This is a global
// side effect, which is fine for a one-shot process that exits after the
// command; the long-running serve path configures its own logger via
// observability before constructing the server.
func quietStartupLogging(stderr io.Writer) {
	handler := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(handler))
}

// newRuntime loads the config from the standard path, builds the server,
// and attaches the audit sink so a CLI call is recorded exactly like an
// MCP call. ctx bounds the SQLite sink's open; warn goes to stderr.
//
// A missing config file is not an error: the runtime falls back to an
// in-memory default (read-only default profile active, no environments)
// so tool discovery and meta-tool calls work offline, the same way the
// Python CLI lists its catalog without a config. Linode-API tools then
// fail at call time with a clear "no environment configured" message
// rather than the whole command dying at config load. A malformed or
// unreadable config (any error other than file-not-found) still fails.
//
// Audit setup never fails the command: if the JSONL sink can't open, the
// server keeps its default no-op sink and the call still runs. Only a
// real config error or a server construction error returns an error.
func newRuntime(ctx context.Context, stderr io.Writer) (*Runtime, error) {
	quietStartupLogging(stderr)

	cfg, err := loadConfigOrDefault()
	if err != nil {
		return nil, err
	}

	srv, err := server.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	closeAudit := attachAuditSink(ctx, srv, cfg, stderr)

	return &Runtime{Server: srv, Config: cfg, close: closeAudit}, nil
}

// loadConfigOrDefault loads the config from the standard path, returning
// an in-memory default when the file is absent so the CLI works offline.
// A file-not-found is the only error swallowed; a malformed or unreadable
// config still propagates so the user sees the real problem.
func loadConfigOrDefault() (*config.Config, error) {
	path := config.Path()

	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}

	if errors.Is(err, config.ErrConfigFileNotFound) {
		return defaultCLIConfig(), nil
	}

	return nil, fmt.Errorf("load config from %s: %w", path, err)
}

// defaultCLIConfig builds the in-memory config the CLI uses when no config
// file exists. It mirrors what config.Load produces for an empty file:
// the server fields take their package defaults, audit keeps its defaults
// (PII redaction on, the standard SQLite busy timeout), the active profile
// is left empty so the resolver selects the read-only default, and there
// are no environments. The RedactPII pointer must be non-nil because the
// audit middleware dereferences it.
func defaultCLIConfig() *config.Config {
	redactPII := config.DefaultAuditRedactPII

	return &config.Config{
		Server: config.ServerConfig{
			Name:      config.DefaultServerName,
			LogLevel:  config.DefaultLogLevel,
			Transport: config.DefaultTransport,
			Host:      config.DefaultHost,
			Port:      config.DefaultServerPort,
		},
		Audit: config.AuditConfig{
			RedactPII: &redactPII,
			SQLite:    config.AuditSQLiteConfig{BusyTimeoutMS: config.DefaultAuditSQLiteBusyTimeoutMS},
		},
	}
}

// attachAuditSink opens the JSONL audit sink (and the SQLite sink when
// audit.sqlite.enabled), attaches the combined sink to srv, and selects
// the PII redaction tier from config. Returns a close func that shuts
// whatever opened. A sink that fails to open is logged to stderr and
// skipped; the returned close is still safe to call.
//
// This is the read/write-meet contract: the sink writes under
// ResolveDefaultAuditDir, the same directory the linode_audit_* tools
// read, so a `call` and a later `audit recent` see the same log.
func attachAuditSink(ctx context.Context, srv *server.Server, cfg *config.Config, stderr io.Writer) func() {
	jsonlSink, err := audit.NewJSONLSink(audit.ResolveDefaultAuditDir())
	if err != nil {
		// Same wording as the server path (cmd/linodemcp/main.go) and the
		// Python twin, so operators grep one phrase across languages and
		// entry points; docs/audit-operations.md documents it verbatim.
		writef(stderr, "audit JSONL sink unavailable; continuing without audit: %v\n", err)

		return func() {}
	}

	closers := []func(){closeSinkFunc(stderr, "audit log", jsonlSink)}

	var sink audit.Sink = jsonlSink

	if cfg.Audit.SQLite.Enabled {
		if sqliteSink := openCLISQLiteSink(ctx, cfg, jsonlSink, stderr); sqliteSink != nil {
			sink = audit.NewMultiSink(jsonlSink, sqliteSink)

			closers = append(closers, closeSinkFunc(stderr, "audit database", sqliteSink))
		}
	}

	srv.SetAuditSink(sink)
	srv.SetAuditRedactPII(*cfg.Audit.RedactPII)

	return func() {
		for _, closeSink := range closers {
			closeSink()
		}
	}
}

// openCLISQLiteSink resolves the SQLite path (config value, or audit.db
// beside the JSONL log) and opens the sink. Returns nil after logging on
// failure; the JSONL sink remains the durable record so the call still
// runs and is audited.
func openCLISQLiteSink(
	ctx context.Context,
	cfg *config.Config,
	jsonlSink *audit.JSONLSink,
	stderr io.Writer,
) *audit.SQLiteSink {
	dbPath := cfg.Audit.SQLite.Path
	if dbPath == "" {
		dbPath = filepath.Join(filepath.Dir(jsonlSink.Path()), auditDBName)
	}

	sink, err := audit.NewSQLiteSink(ctx, dbPath, cfg.Audit.SQLite.BusyTimeoutMS)
	if err != nil {
		writef(stderr, "audit SQLite sink unavailable; continuing with JSONL only: %v\n", err)

		return nil
	}

	return sink
}

// sinkCloser is the close surface shared by the JSONL and SQLite sinks,
// narrowed so closeSinkFunc can wrap either without a type switch.
type sinkCloser interface {
	Close() error
}

// closeSinkFunc builds a close func that shuts one sink and reports a
// close error to stderr. label names the sink in any error message.
func closeSinkFunc(stderr io.Writer, label string, sink sinkCloser) func() {
	return func() {
		if err := sink.Close(); err != nil {
			writef(stderr, "%s close error: %v\n", label, err)
		}
	}
}
