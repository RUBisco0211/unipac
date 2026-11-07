package npmregistry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/util"
)

// document is a registry answer for one package, trimmed to what the app reads.
const document = `{
  "name": "typescript",
  "description": "TypeScript is a language for application scale JavaScript development",
  "homepage": "https://www.typescriptlang.org/",
  "license": "Apache-2.0",
  "author": "Microsoft Corp.",
  "maintainers": [{"name": "typescript-bot", "email": "typescript@microsoft.com"}],
  "dist-tags": {"latest": "7.0.2"},
  "versions": {"6.0.0": {}, "7.0.2": {}},
  "time": {
    "created": "2022-01-26T13:10:59.042Z",
    "modified": "2026-11-06T09:46:44.664Z",
    "6.0.0": "2025-01-02T00:00:00.000Z",
    "7.0.2": "2026-10-01T00:00:00.000Z"
  }
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

// The registry's document becomes the page's fields, with the release date of the
// version installed rather than of the newest one.
func TestDetailFromTheRegistry(t *testing.T) {
	serve(t, document)

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
	// The first maintainer is the account the package belongs to.
	if detail.Publisher != "typescript-bot" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
}

// A scoped name keeps its slash, and a package the registry does not have is an
// error rather than an empty answer with a made-up name.
func TestDetailRequestsTheScopedPath(t *testing.T) {
	var asked string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	old := Base
	Base = server.URL
	defer func() { Base = old }()

	if _, err := Detail(context.Background(), "@scope/tool", "1.0.0"); err == nil {
		t.Fatal("a missing package read as an answer")
	}
	if want := "/@scope/tool"; asked != want {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// The version list is newest first, and the document's own bookkeeping is not a
// version.
func TestVersionsNewestFirst(t *testing.T) {
	serve(t, document)

	versions, err := Versions(context.Background(), "typescript")
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(versions) != 2 || versions[0] != "7.0.2" || versions[1] != "6.0.0" {
		t.Fatalf("versions = %v, want [7.0.2 6.0.0]", versions)
	}
}

// A registry that is not reachable leaves the caller with the name and version it
// asked about, so a page shows what it knew.
func TestDetailWhenTheRegistryIsUnreachable(t *testing.T) {
	old := Base
	Base = "http://127.0.0.1:1"
	t.Cleanup(func() { Base = old })
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
