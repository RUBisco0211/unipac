// Package pypi reads a package's page on PyPI, the registry pip, pipx and uv all
// install from. What those tools keep locally describes an installed copy; a
// package that is only a search result has none, and a project's latest version is
// the registry's either way.
package pypi

import (
	"cmp"
	"context"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Base is where PyPI answers. A test points it at a server of its own.
var Base = "https://pypi.org"

// document is the part of PyPI's JSON for one project this app reads.
type document struct {
	Info struct {
		Name        string            `json:"name"`
		Version     string            `json:"version"`
		Summary     string            `json:"summary"`
		Homepage    string            `json:"home_page"`
		ProjectURLs map[string]string `json:"project_urls"`
		Author      string            `json:"author"`
		AuthorEmail string            `json:"author_email"`
		License     string            `json:"license"`
	} `json:"info"`
}

// Detail reads one project from PyPI: what it says about itself, who publishes it,
// its terms, and the version it calls current. A copy installed here has the same
// facts in its own metadata; a package that is only a search result has nothing
// else.
func Detail(ctx context.Context, name string) (manager.Detail, error) {
	var doc document
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/pypi/"+ProjectPath(name)+"/json", &doc); err != nil {
		return manager.Detail{Name: name}, err
	}
	return manager.Detail{
		Name:      cmp.Or(doc.Info.Name, name),
		Latest:    doc.Info.Version,
		Summary:   doc.Info.Summary,
		Homepage:  cmp.Or(doc.Info.Homepage, homeFromProjectURLs(doc.Info.ProjectURLs)),
		Publisher: person(doc.Info.Author, doc.Info.AuthorEmail),
		License:   doc.Info.License,
		Source:    "pypi.org",
	}, nil
}

// Latest is the version PyPI calls current, for a package whose own manager did
// not report one.
func Latest(ctx context.Context, name string) (string, error) {
	detail, err := Detail(ctx, name)
	if err != nil {
		return "", err
	}
	return detail.Latest, nil
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

// homeFromProjectURLs finds the project's page among the addresses it declares,
// which is where modern projects put it instead of the old home_page field.
func homeFromProjectURLs(urls map[string]string) string {
	for _, key := range []string{"Homepage", "homepage", "Home", "Website", "Source", "Repository"} {
		if value := urls[key]; value != "" {
			return value
		}
	}
	for _, value := range urls {
		if value != "" {
			return value
		}
	}
	return ""
}

// person names whoever publishes a project: the author, with the address when the
// author is only an email.
func person(author, email string) string {
	author, email = strings.TrimSpace(author), strings.TrimSpace(email)
	switch {
	case author != "":
		return author
	case email != "":
		return strings.SplitN(email, " ", 2)[0]
	}
	return ""
}
