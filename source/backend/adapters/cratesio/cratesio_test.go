package cratesio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sample = `{
  "crate": {
    "name": "ripgrep",
    "description": "line-oriented search tool",
    "homepage": "",
    "repository": "https://github.com/BurntSushi/ripgrep",
    "max_version": "15.2.0"
  },
  "versions": [
    {"num": "15.2.0", "created_at": "2026-07-15T16:28:46.152381Z", "license": "MIT OR Apache-2.0"},
    {"num": "14.1.1", "created_at": "2024-09-01T00:00:00.000000Z", "license": "MIT OR Apache-2.0"}
  ]
}`

func serve(t *testing.T, body string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	old := Base
	Base = server.URL
	t.Cleanup(func() { Base = old })
}

// The crate's page, description and the release date of the version installed
// here, not of the newest one.
func TestDetailFromCratesIO(t *testing.T) {
	serve(t, sample)

	detail, err := Detail(context.Background(), "ripgrep", "14.1.1")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Name != "ripgrep" || detail.Version != "14.1.1" || detail.Latest != "15.2.0" {
		t.Errorf("versions = %+v", detail)
	}
	if detail.Summary != "line-oriented search tool" {
		t.Errorf("summary = %q", detail.Summary)
	}
	// The crate declares no homepage; its repository is the page then.
	if detail.Homepage != "https://github.com/BurntSushi/ripgrep" {
		t.Errorf("homepage = %q", detail.Homepage)
	}
	if detail.License != "MIT OR Apache-2.0" {
		t.Errorf("license = %q", detail.License)
	}
	// crates.io has no author on a crate, so no publisher is claimed.
	if detail.Publisher != "" {
		t.Errorf("publisher = %q, want none", detail.Publisher)
	}
}

// A crate the registry does not have is an error, with the name and version it
// was asked about.
func TestDetailWhenTheCrateIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	old := Base
	Base = server.URL
	defer func() { Base = old }()

	detail, err := Detail(context.Background(), "nope", "1.0.0")
	if err == nil {
		t.Fatal("a missing crate read as an answer")
	}
	if detail.Name != "nope" || detail.Version != "1.0.0" {
		t.Errorf("detail = %+v", detail)
	}
}
