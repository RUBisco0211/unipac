// Package cratesio reads a package's page on crates.io, the registry cargo
// installs from. The cargo tool itself only lists what is installed, so the
// project's description, its page and when its version was published are the
// registry's to answer.
package cratesio

import (
	"context"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Base is where crates.io answers. A test points it at a server of its own.
var Base = "https://crates.io"

// document is crates.io's answer for one crate: the crate itself, with the
// version it calls newest, and the versions it published with their dates.
type document struct {
	Crate struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Homepage    string `json:"homepage"`
		Repository  string `json:"repository"`
		MaxVersion  string `json:"max_version"`
	} `json:"crate"`
	Versions []struct {
		Num     string `json:"num"`
		License string `json:"license"`
	} `json:"versions"`
}

// owners is crates.io's answer for who owns a crate: its GitHub accounts, the
// first of which is the account that publishes it.
type owners struct {
	Users []struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"users"`
}

// Detail reads one crate: its description, where its page is, its terms, and who
// publishes it. The crate has no author field of its own — its owners are a second
// call, which is what the publisher is — and the repository stands in for a page
// the crate does not declare.
func Detail(ctx context.Context, name, version string) (manager.Detail, error) {
	var doc document
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/api/v1/crates/"+name, &doc); err != nil {
		return manager.Detail{Name: name, Version: version}, err
	}
	detail := manager.Detail{
		Name:     strings.TrimSpace(doc.Crate.Name),
		Version:  version,
		Latest:   doc.Crate.MaxVersion,
		Summary:  strings.TrimSpace(doc.Crate.Description),
		Homepage: doc.Crate.Homepage,
		Source:   "crates.io",
	}
	if detail.Name == "" {
		detail.Name = name
	}
	if detail.Homepage == "" {
		detail.Homepage = doc.Crate.Repository
	}
	if detail.Version == "" {
		detail.Version = detail.Latest
	}
	for _, published := range doc.Versions {
		if published.Num == detail.Version {
			detail.License = published.License
			break
		}
	}
	// The owners are a second answer; a registry that does not give it costs the
	// publisher and nothing else.
	var owned owners
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/api/v1/crates/"+name+"/owners", &owned); err == nil {
		detail.Publisher = ownerName(owned)
	}
	return detail, nil
}

// ownerName is the first owner of a crate, named as a person when crates.io
// knows one and by login otherwise.
func ownerName(doc owners) string {
	for _, user := range doc.Users {
		if user.Name != "" {
			return user.Name
		}
		if user.Login != "" {
			return user.Login
		}
	}
	return ""
}
