package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// UserAgent identifies the app to the registries it reads. crates.io refuses a
// request without one.
const UserAgent = "UniPac (+https://github.com/RUBisco0211/unipac)"

// httpTimeout bounds one registry request: a page that shows what it finds is
// better with a missing line than with a spinner that never stops.
const httpTimeout = 10 * time.Second

// maxBody caps what a registry answer may read, so a wrong address cannot stream
// the app out of memory. It is generous because some registries answer with a
// whole version history; a reader that only needs one version asks for it
// (npmregistry does) rather than raising this.
const maxBody = 8 << 20

// HTTPClient is the client the app reads registries with: it goes through the
// configured proxy, or the environment's when none is set, and gives up on a
// request rather than holding a page.
func HTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if value := Proxy(); value != "" {
		if u, err := url.Parse(value); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: transport, Timeout: httpTimeout}
}

// GetJSON reads url into out as JSON. A non-200 answer is an error that names
// the status, so a caller can tell "this package is not there" from "the network
// is not".
func GetJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("bad request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", url, res.Status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing %s: %w", url, err)
	}
	return nil
}
