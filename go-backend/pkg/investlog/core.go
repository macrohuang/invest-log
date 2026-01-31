package investlog

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Options controls Core initialization.
type Options struct {
	DBPath             string
	Logger             *slog.Logger
	PriceCacheTTL      time.Duration
	PriceFailThreshold int
	PriceFailWindow    time.Duration
	PriceCooldown      time.Duration
	HTTPTimeout        time.Duration
}

// Core provides access to Invest Log business logic and storage.
type Core struct {
	db     *sql.DB
	logger *slog.Logger
	price  *priceFetcher
	dbPath string
}

// Open initializes a Core using the provided database path.
func Open(dbPath string) (*Core, error) {
	return OpenWithOptions(Options{DBPath: dbPath})
}

// OpenWithOptions initializes a Core using the provided options.
func OpenWithOptions(opts Options) (*Core, error) {
	if opts.DBPath == "" {
		return nil, errors.New("db path is required")
	}
	cleanPath := filepath.Clean(opts.DBPath)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite performs best with a single writer.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		logger.Warn("pragma busy_timeout failed", "err", err)
	}

	if err := initDatabase(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init database: %w", err)
	}

	pf := newPriceFetcher(priceFetcherOptions{
		Logger:        logger,
		CacheTTL:      defaultDuration(opts.PriceCacheTTL, 30*time.Second),
		FailThreshold: defaultInt(opts.PriceFailThreshold, 3),
		FailWindow:    defaultDuration(opts.PriceFailWindow, 60*time.Second),
		Cooldown:      defaultDuration(opts.PriceCooldown, 120*time.Second),
		HTTPTimeout:   defaultDuration(opts.HTTPTimeout, 10*time.Second),
	})

	return &Core{
		db:     db,
		logger: logger,
		price:  pf,
		dbPath: cleanPath,
	}, nil
}

// Close releases database resources.
func (c *Core) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// DBPath returns the underlying database path.
func (c *Core) DBPath() string {
	return c.dbPath
}

func defaultDuration(v time.Duration, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return v
}

func defaultInt(v int, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
