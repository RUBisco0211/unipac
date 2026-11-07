package npmregistry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/util"
)

// latest is one version's registry entry, the shape <name>/latest answers with.
const latest = `{
  "name": "typescript",
  "version": "7.0.2",
  "description": "TypeScript is a language for application scale JavaScript development",
  "homepage": "https://www.typescriptlang.org/",
  "license": "Apache-2.0",
  "author": "Microsoft Corp.",
  "maintainers": [{"name": "typescript-bot", "email": "typescript@microsoft.com"}]
}`

// older is the same package at an older version, which keeps its own terms.
const older = `{
  "name": "typescript",
  "version": "6.0.0",
  "license": "Apache-2.0",
  "author": {"name": "Someone Else"},
  "maintainers": [{"name": "someone"}]
}`

// serve answers /latest with latest and /<version> with older, recording the paths
// it was asked for.
func serve(t *testing.T, asked *[]string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if asked != nil {
			*asked = append(*asked, r.URL.Path)
		}
		if strings.HasSuffix(r.URL.Path, "/latest") {
			w.Write([]byte(latest))
			return
		}
		w.Write([]byte(older))
	}))
	t.Cleanup(server.Close)
	old := Base
	Base = server.URL
	t.Cleanup(func() { Base = old })
}

// failure answers every request with 404.
func failure(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	old := Base
	Base = server.URL
	t.Cleanup(func() { Base = old })
}

// One version's entry becomes the page's fields; the installed version adds its
// own terms, and the newest version is what the page offers.
func TestDetailFromTheRegistry(t *testing.T) {
	var asked []string
	serve(t, &asked)

	detail, err := Detail(context.Background(), "typescript", "6.0.0")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Name != "typescript" || detail.Version != "6.0.0" || detail.Latest != "7.0.2" {
		t.Errorf("versions = %+v", detail)
	}
	if detail.Summary == "" || detail.Homepage != "https://www.typescriptlang.org/" || detail.License != "Apache-2.0" {
		t.Errorf("facts = %+v", detail)
	}
	// The installed copy's own entry names who publishes that version.
	if detail.Publisher != "someone" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
	// Two small documents, never the whole package's history.
	if len(asked) != 2 || asked[0] != "/typescript/latest" || asked[1] != "/typescript/6.0.0" {
		t.Errorf("asked for %v", asked)
	}
}

// With the newest version installed, one request is enough, and the publisher is
// the newest version's.
func TestDetailAtTheNewestVersionAsksOnce(t *testing.T) {
	var asked []string
	serve(t, &asked)

	detail, err := Detail(context.Background(), "typescript", "7.0.2")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Publisher != "typescript-bot" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
	if len(asked) != 1 {
		t.Errorf("asked for %v, want one request", asked)
	}
}

// A scoped name keeps its slash, and a package the registry does not have is an
// error rather than an empty answer with a made-up name.
func TestDetailRequestsTheScopedPath(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	old := Base
	Base = server.URL
	defer func() { Base = old }()

	if _, err := Detail(context.Background(), "@scope/tool", "1.0.0"); err == nil {
		t.Fatal("a missing package read as an answer")
	}
	if want := "/@scope/tool/latest"; len(asked) == 0 || asked[0] != want {
		t.Errorf("asked %v, want %q first", asked, want)
	}
}

// A package whose page it does not declare still has one: its repository, in git
// or npm's shorthand.
func TestPageFromRepository(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"git+https://github.com/a/b.git", "https://github.com/a/b"},
		{"git://github.com/a/b.git", "https://github.com/a/b"},
		{"github:a/b", "https://github.com/a/b"},
		{"https://github.com/a/b", "https://github.com/a/b"},
		{"", ""},
	} {
		if got := pageFromRepository(tc.in); got != tc.want {
			t.Errorf("pageFromRepository(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A registry that is not reachable leaves the caller with the name and version it
// asked about, so a page shows what it knew.
func TestDetailWhenTheRegistryIsUnreachable(t *testing.T) {
	failure(t)
	util.SetProxy("")
	t.Cleanup(func() { util.SetProxy("") })

	detail, err := Detail(context.Background(), "typescript", "6.0.0")
	if err == nil {
		t.Fatal("an unreachable registry read as an answer")
	}
	if detail.Name != "typescript" || detail.Version != "6.0.0" {
		t.Errorf("detail = %+v, want the name and version it was asked about", detail)
	}
}
