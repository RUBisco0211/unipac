package cache

import (
	"context"
	"fmt"
	"unipac-wails/backend/util"

	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3"
)

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

	db, err := sqlx.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL", dbPath))
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
