// Package database opens the PostgreSQL connection and runs the embedded,
// versioned SQL migrations.
package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Connect opens a GORM connection pool.
func Connect(url string, maxOpen int, log *slog.Logger) (*gorm.DB, error) {
	level := logger.Warn
	if log != nil && log.Enabled(nil, slog.LevelDebug) {
		level = logger.Info
	}
	gcfg := &gorm.Config{
		Logger: logger.New(
			stdLogWriter{log: log},
			logger.Config{SlowThreshold: 500 * time.Millisecond, LogLevel: level},
		),
		// Prevent accidental mass assignment / table drops in production paths.
		AllowGlobalUpdate: false,
		PrepareStmt:       true,
	}
	db, err := gorm.Open(postgres.Open(url), gcfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("underlying sql db: %w", err)
	}
	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
	}
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

// stdLogWriter adapts slog to gorm's logger interface.
type stdLogWriter struct{ log *slog.Logger }

func (w stdLogWriter) Write(p []byte) (int, error) {
	if w.log != nil {
		w.log.Debug("gorm", "msg", string(p))
	}
	return len(p), nil
}

// Printf satisfies gorm's logger.Writer (format strings are logged verbatim
// at debug level; they never contain parameter values — gorm passes SQL
// separately in Write).
func (w stdLogWriter) Printf(format string, args ...any) {
	if w.log != nil {
		w.log.Debug("gorm", "msg", fmt.Sprintf(format, args...))
	}
}

type migration struct {
	version string
	sql     string
}

// upMigrations returns the sorted ".up.sql" file names from the given
// directory entries. The suffix check uses strings.HasSuffix; an earlier
// version sliced the last 11 characters and compared them to the 8-character
// ".up.sql", so nothing ever matched and migrations were silently skipped.
func upMigrations(entries []fs.DirEntry) []string {
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// Migrate applies pending embedded migrations in filename order. Each
// migration runs inside a transaction and is recorded in schema_migrations.
func Migrate(db *gorm.DB, log *slog.Logger) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(128) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	files := upMigrations(entries)

	for _, name := range files {
		version := name[:len(name)-len(".up.sql")]
		var count int64
		if err := db.Raw(`SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count).Error; err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if count > 0 {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		// Migration bodies are multi-statement SQL (e.g. BEGIN; ... COMMIT;),
		// which the pgx extended-protocol prepared statement rejects with
		// `cannot insert multiple commands into a prepared statement`
		// (SQLSTATE 42601). execMigration runs each body over a dedicated
		// simple-protocol connection; version tracking stays identical.
		if err := execMigration(db, name, version, string(body)); err != nil {
			return err
		}
		if log != nil {
			log.Info("migration applied", "version", version)
		}
	}
	return nil
}

// execMigration applies one migration file and records its version.
//
// It opens a short-lived dedicated connection whose DefaultQueryExecMode is
// pgx.QueryExecModeSimpleProtocol, so the whole (possibly multi-statement)
// body is sent as a single simple-query message. PostgreSQL allows multiple
// commands in a simple query and runs them as one implicit transaction, which
// preserves atomicity for files that do not open their own BEGIN/COMMIT; files
// that do open their own transaction keep their explicit control. Statements
// are never split client-side, and the rest of the application keeps using the
// normal extended-protocol pool (prepared statements unaffected).
func execMigration(db *gorm.DB, name, version, body string) error {
	dia, ok := db.Dialector.(*postgres.Dialector)
	if !ok || dia.Config == nil || dia.Config.DSN == "" {
		return fmt.Errorf("apply %s: gorm dialector does not expose the DSN needed for a simple-protocol connection", name)
	}
	cfg, err := pgx.ParseConfig(dia.Config.DSN)
	if err != nil {
		return fmt.Errorf("apply %s: parse dsn: %w", name, err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("apply %s: connect: %w", name, err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, body); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	return nil
}

// ErrNotFound is a small sentinel for callers checking gorm.ErrRecordNotFound.
var ErrNotFound = errors.New("record not found")
