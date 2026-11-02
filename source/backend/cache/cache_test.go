package cache_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/cache"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// TestRoundTrip exercises the real driver: the app builds with CGO_ENABLED=0,
// so this fails loudly if the SQLite driver ever regresses to a cgo-only one.
func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	util.AppRoot = t.TempDir()

	cfg := cache.DefaultConfig()
	if err := cache.Init(ctx, cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = cache.Default.Close() })

	// The DSN turns WAL on; a second connection to the same file reads back what
	// the first one set.
	db, err := sqlx.Open("sqlite", "file:"+filepath.Join(util.AppRoot, cfg.Path))
	if err != nil {
		t.Fatalf("open for pragma check: %v", err)
	}
	var mode string
	if err := db.GetContext(ctx, &mode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	_ = db.Close()
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	pkgs := []manager.Package{
		{Name: "ripgrep", Fullname: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true},
		{Name: "typescript", Version: "5.9.3", Manager: "npm", Installed: true, Outdated: true, LatestVersion: "6.0.0"},
	}
	if err := cache.Default.UpdateCache(pkgs); err != nil {
		t.Fatalf("UpdateCache: %v", err)
	}

	got, err := cache.Default.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d rows, want 2", len(got))
	}
	// Rows come back ordered by name.
	if got[0].Name != "ripgrep" || got[1].Name != "typescript" {
		t.Fatalf("unexpected order: %q, %q", got[0].Name, got[1].Name)
	}
	if got[1].LatestVersion != "6.0.0" || !got[1].Outdated {
		t.Errorf("outdated fields lost: %+v", got[1])
	}

	// An empty scan must not wipe the cache.
	if err := cache.Default.UpdateCache(nil); err != nil {
		t.Fatalf("UpdateCache(nil): %v", err)
	}
	if again, err := cache.Default.List(); err != nil || len(again) != 2 {
		t.Fatalf("cache changed after empty update: %d rows, err %v", len(again), err)
	}
}
