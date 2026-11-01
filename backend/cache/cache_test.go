package cache

import (
	"context"
	"testing"

	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/RUBisco0211/unipac/backend/util"
)

// TestRoundTrip exercises the real driver: the app builds with CGO_ENABLED=0,
// so this fails loudly if the SQLite driver ever regresses to a cgo-only one.
func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	util.AppRoot = t.TempDir()

	if err := Init(ctx, DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = Default.Close() })

	var mode string
	if err := Default.db.GetContext(ctx, &mode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	pkgs := []manager.Package{
		{Name: "ripgrep", Fullname: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true},
		{Name: "typescript", Version: "5.9.3", Manager: "npm", Installed: true, Outdated: true, LatestVersion: "6.0.0"},
	}
	if err := Default.UpdateCache(pkgs); err != nil {
		t.Fatalf("UpdateCache: %v", err)
	}

	got, err := Default.List()
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
	if err := Default.UpdateCache(nil); err != nil {
		t.Fatalf("UpdateCache(nil): %v", err)
	}
	if again, err := Default.List(); err != nil || len(again) != 2 {
		t.Fatalf("cache changed after empty update: %d rows, err %v", len(again), err)
	}
}
