package cache

import (
	"context"
	"fmt"

	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// sqliteDriver is the pure-Go driver, so the app builds and cross-compiles with
// CGO_ENABLED=0.
const sqliteDriver = "sqlite"

// sqliteDSN opens path with WAL and a busy timeout. modernc.org/sqlite reads
// pragmas from the DSN, unlike the cgo driver's mattn-style parameters.
func sqliteDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
}

// Config represents the configuration for the cache manager
type Config struct {
	Path string `json:"path,omitempty"`
}

// DefaultConfig returns the default configuration for the cache manager
func DefaultConfig() Config {
	return Config{
		Path: "cache/package.db",
	}
}

// Cache is responsible for managing the cache database connection and operations
type Cache struct {
	ctx context.Context
	db  *sqlx.DB
}

// Default is the default cache manager instance
var Default *Cache

// Init initializes the cache manager with the provided configuration and sets up the database connection
func Init(ctx context.Context, cfg Config) error {
	c, err := newCache(ctx, cfg)
	if err != nil {
		return err
	}
	Default = c

	if err := Default.createPackageCacheTable(); err != nil {
		return err
	}
	return nil
}

func newCache(ctx context.Context, cfg Config) (*Cache, error) {
	dbPath, err := util.SafeJoin(util.AppRoot, cfg.Path)
	if err != nil {
		return nil, err
	}

	db, err := sqlx.Open(sqliteDriver, sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Cache{
		ctx: ctx,
		db:  db,
	}, nil
}

// Close the database connection
func (m *Cache) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
