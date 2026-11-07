package cache_test

import (
	"context"
	"os"
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

	// An empty snapshot clears the cache. Callers decide whether an empty scan
	// is trustworthy; the cache just stores what it is given.
	if err := cache.Default.UpdateCache(nil); err != nil {
		t.Fatalf("UpdateCache(nil): %v", err)
	}
	if again, err := cache.Default.List(); err != nil || len(again) != 0 {
		t.Fatalf("cache kept rows after an empty update: %d rows, err %v", len(again), err)
	}
}

// A scan updates the rows it finds and drops the ones it does not, and a
// package's page survives while its version does: the detail is what a page shows
// and what a rescan must not throw away.
func TestUpdateCacheKeepsDetailWhileTheVersionDoes(t *testing.T) {
	ctx := context.Background()
	util.AppRoot = t.TempDir()
	if err := cache.Init(ctx, cache.DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = cache.Default.Close() })

	pkgs := []manager.Package{
		{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true},
		{Name: "typescript", Version: "5.9.3", Manager: "npm", Installed: true},
	}
	if err := cache.Default.UpdateCache(pkgs); err != nil {
		t.Fatalf("UpdateCache: %v", err)
	}
	detail := manager.Detail{Name: "ripgrep", Version: "14.1.1", Summary: "line-oriented search tool", Source: "brew"}
	if err := cache.Default.SavePackageDetail("brew", "ripgrep", "14.1.1", detail); err != nil {
		t.Fatalf("SavePackageDetail: %v", err)
	}

	// A scan that finds the same versions keeps the page.
	if err := cache.Default.UpdateCache(pkgs); err != nil {
		t.Fatalf("UpdateCache again: %v", err)
	}
	got, fresh, err := cache.Default.PackageDetail("brew", "ripgrep", "14.1.1")
	if err != nil {
		t.Fatalf("PackageDetail: %v", err)
	}
	if !fresh || got.Summary != "line-oriented search tool" {
		t.Fatalf("detail = %+v fresh=%v, want the page kept", got, fresh)
	}

	// An upgrade of that package takes the old page with it, and one that is
	// gone loses its row.
	upgraded := []manager.Package{
		{Name: "ripgrep", Version: "14.1.2", Manager: "brew", Installed: true},
	}
	if err := cache.Default.UpdateCache(upgraded); err != nil {
		t.Fatalf("UpdateCache upgrade: %v", err)
	}
	if _, fresh, err := cache.Default.PackageDetail("brew", "ripgrep", "14.1.1"); err != nil || fresh {
		t.Errorf("the old version's page survived: fresh=%v err=%v", fresh, err)
	}
	rows, err := cache.Default.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "ripgrep" || rows[0].Version != "14.1.2" {
		t.Fatalf("rows = %+v, want ripgrep 14.1.2 alone", rows)
	}
}

// A database written before the package page existed gains its columns, and the
// rows it already holds are read as pages that were never fetched.
func TestOldDatabaseGainsTheDetailColumns(t *testing.T) {
	ctx := context.Background()
	util.AppRoot = t.TempDir()
	path := filepath.Join(util.AppRoot, cache.DefaultConfig().Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	db, err := sqlx.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE installed_package_cache (
		name TEXT NOT NULL, fullname TEXT, version TEXT NOT NULL, latest_version TEXT,
		manager TEXT NOT NULL, installed INTEGER NOT NULL DEFAULT 1,
		outdated INTEGER NOT NULL DEFAULT 0, description TEXT, PRIMARY KEY(name, manager))`)
	if err != nil {
		t.Fatalf("create old table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO installed_package_cache (name, version, manager, installed)
		VALUES ('cowsay', '6.1', 'pipx', 1)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = db.Close()

	if err := cache.Init(ctx, cache.DefaultConfig()); err != nil {
		t.Fatalf("Init on an old database: %v", err)
	}
	t.Cleanup(func() { _ = cache.Default.Close() })

	if _, fresh, err := cache.Default.PackageDetail("pipx", "cowsay", "6.1"); err != nil || fresh {
		t.Errorf("old row read as a fetched page: fresh=%v err=%v", fresh, err)
	}
	if err := cache.Default.SavePackageDetail("pipx", "cowsay", "6.1",
		manager.Detail{Name: "cowsay", Version: "6.1", Summary: "a cow says things"}); err != nil {
		t.Fatalf("SavePackageDetail on a migrated table: %v", err)
	}
	if detail, fresh, err := cache.Default.PackageDetail("pipx", "cowsay", "6.1"); err != nil || !fresh || detail.Summary == "" {
		t.Errorf("detail = %+v fresh=%v err=%v", detail, fresh, err)
	}
}

// A page's summary becomes the row's description when the listing had none, and a
// row that already says what it is keeps its own words.
func TestSavePackageDetailDescribesTheRow(t *testing.T) {
	ctx := context.Background()
	util.AppRoot = t.TempDir()
	if err := cache.Init(ctx, cache.DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = cache.Default.Close() })

	pkgs := []manager.Package{
		{Name: "typescript", Version: "5.9.3", Manager: "npm", Installed: true},
		{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true, Description: "from the scan"},
	}
	if err := cache.Default.UpdateCache(pkgs); err != nil {
		t.Fatalf("UpdateCache: %v", err)
	}

	if err := cache.Default.SavePackageDetail("npm", "typescript", "5.9.3",
		manager.Detail{Name: "typescript", Version: "5.9.3", Summary: "a language"}); err != nil {
		t.Fatalf("SavePackageDetail: %v", err)
	}
	if err := cache.Default.SavePackageDetail("brew", "ripgrep", "14.1.1",
		manager.Detail{Name: "ripgrep", Version: "14.1.1", Summary: "a search tool"}); err != nil {
		t.Fatalf("SavePackageDetail: %v", err)
	}

	rows, err := cache.Default.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, row := range rows {
		switch row.Name {
		case "typescript":
			if row.Description != "a language" {
				t.Errorf("typescript description = %q, want the page's", row.Description)
			}
		case "ripgrep":
			if row.Description != "from the scan" {
				t.Errorf("ripgrep description = %q, want the scan's own", row.Description)
			}
		}
	}
}
