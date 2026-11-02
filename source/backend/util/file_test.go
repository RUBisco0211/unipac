package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// WriteJson replaces the file through a temporary one, so a successful write
// must leave exactly the target behind, with the usual permissions.
func TestWriteJsonLeavesOnlyTheTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unipac.json")

	if err := WriteJson(path, map[string]string{"level": "debug"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	assertOnlyFile(t, dir, "unipac.json")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("mode = %v, want 0644", perm)
	}
	if got := readJSON(t, path)["level"]; got != "debug" {
		t.Fatalf("level = %q, want debug", got)
	}

	// A second write replaces the file in place, without adding another one.
	if err := WriteJson(path, map[string]string{"level": "warn"}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	assertOnlyFile(t, dir, "unipac.json")
	if got := readJSON(t, path)["level"]; got != "warn" {
		t.Fatalf("level = %q, want warn", got)
	}
}

func assertOnlyFile(t *testing.T, dir, want string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != want {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory holds %v, want only %s", names, want)
	}
}

func readJSON(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
