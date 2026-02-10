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
	MaxOpenConns       int           // Maximum number of open connections to the database
	MaxIdleConns       int           // Maximum number of idle connections in the pool
	ConnMaxLifetime    time.Duration // Maximum lifetime of a connection
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
	cache  *holdingsCache
}

// Open initializes a Core using the provided database path.
// For new code, prefer OpenWithOptions which provides more configuration options.
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
	// Configure connection pool. For SQLite, we typically use single connection
	// but allow configuration for flexibility.
	db.SetMaxOpenConns(defaultInt(opts.MaxOpenConns, 1))
	db.SetMaxIdleConns(defaultInt(opts.MaxIdleConns, 1))
	if opts.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		logger.Warn("pragma busy_timeout failed", "err", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		logger.Warn("pragma foreign_keys failed", "err", err)
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

	c := &Core{
		db:     db,
		logger: logger,
		price:  pf,
		dbPath: cleanPath,
		cache:  newHoldingsCache(),
	}

	// Inject rate resolver so priceFetcher can look up FX rates (e.g. HKD→CNY)
	// from the database at runtime.
	pf.rateResolver = func(fromCurrency string) (float64, error) {
		return c.GetRateToCNY(fromCurrency)
	}

	return c, nil
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

// Logger returns the Core logger, falling back to the default logger.
func (c *Core) Logger() *slog.Logger {
	if c == nil || c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

func (c *Core) invalidateHoldingsCache() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.invalidate()
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
