// Package pypi reads a package's page on PyPI, the registry pip, pipx and uv all
// install from. What those tools keep locally is the installed copy's metadata;
// which version the project offers is only the registry's to say.
package pypi

import (
	"context"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Base is where PyPI answers. A test points it at a server of its own.
var Base = "https://pypi.org"

// document is the part of PyPI's JSON for one project this app reads.
type document struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

// Latest is the version PyPI calls current, for a package whose own manager did
// not report one: pip, pipx and uv install from PyPI, so the registry answers
// what the installed copy cannot.
func Latest(ctx context.Context, name string) (string, error) {
	var doc document
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/pypi/"+ProjectPath(name)+"/json", &doc); err != nil {
		return "", err
	}
	return doc.Info.Version, nil
}

// ProjectPath is the URL segment of a project. PyPI normalizes names: runs of
// -_. become one -, and lower case.
func ProjectPath(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch r {
		case '-', '_', '.':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			b.WriteRune(r)
			lastDash = false
		}
	}
	return b.String()
}
