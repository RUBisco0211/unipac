package pypi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, body string, asked *string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if asked != nil {
			*asked = r.URL.Path
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	old := Base
	Base = server.URL
	t.Cleanup(func() { Base = old })
}

// The current version is the registry's answer, and a name with capitals and
// separators is asked for the way PyPI normalizes it.
func TestLatestNormalizesTheName(t *testing.T) {
	var asked string
	serve(t, `{"info": {"version": "6.1"}}`, &asked)

	got, err := Latest(context.Background(), "VaasuDevanS.Cowsay__Py")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got != "6.1" {
		t.Errorf("latest = %q", got)
	}
	if want := "/pypi/vaasudevans-cowsay-py/json"; asked != want {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// A project PyPI does not have is an error rather than an empty version.
func TestLatestWhenTheProjectIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	old := Base
	Base = server.URL
	defer func() { Base = old }()

	if _, err := Latest(context.Background(), "private-tool"); err == nil {
		t.Fatal("a missing project read as an answer")
	}
}
