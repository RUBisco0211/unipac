package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A registry answer is read as JSON, with the user agent crates.io insists on.
func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != UserAgent {
			t.Errorf("user agent = %q, want %q", got, UserAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"ripgrep","version":"14.1.1"}`))
	}))
	defer server.Close()

	var out struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := GetJSON(context.Background(), HTTPClient(), server.URL, &out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Name != "ripgrep" || out.Version != "14.1.1" {
		t.Fatalf("decoded %+v", out)
	}
}

// A missing package is an error that says so, not a silent zero value.
func TestGetJSONNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	var out any
	err := GetJSON(context.Background(), HTTPClient(), server.URL, &out)
	if err == nil {
		t.Fatal("a 404 was read as an answer")
	}
}

// The client a registry call uses goes through the configured proxy.
func TestHTTPClientFollowsTheProxy(t *testing.T) {
	SetProxy("http://127.0.0.1:7890")
	t.Cleanup(func() { SetProxy("") })

	transport, ok := HTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatal("the client has no transport to inspect")
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := transport.Proxy(req)
	if err != nil || u == nil {
		t.Fatalf("proxy = %v, %v", u, err)
	}
	if got, want := u.String(), "http://127.0.0.1:7890"; got != want {
		t.Errorf("proxy = %q, want %q", got, want)
	}
}
