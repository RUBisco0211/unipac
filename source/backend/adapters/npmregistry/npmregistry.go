// Package npmregistry reads a package from the npm registry. npm and bun both
// install npm packages, so one reader answers for both, over HTTP rather than
// through `npm view`, which returns the same facts only when npm is on the machine
// and pays for a process.
package npmregistry

import (
	"cmp"
	"context"
	"encoding/json"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Base is where npm packages are read from. A test points it at a server of its
// own.
var Base = "https://registry.npmjs.org"

// manifest is one version's entry: everything the page shows, and nothing else.
// The registry answers it at <name>/latest and at <name>/<version>.
//
// The whole package document — every version ever published, with the README of
// each — is what this deliberately avoids: a package with a long history reaches
// tens of megabytes, and the reader caps an answer's size.
type manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	License     string `json:"license"`
	Author      person `json:"author"`
	// Maintainers is who the package belongs to, which is the account that
	// publishes it rather than the one person named as its author.
	Maintainers []person `json:"maintainers"`
	Repository  struct {
		URL string `json:"url"`
	} `json:"repository"`
	// Bugs is the issue tracker, which a package without a page still has.
	Bugs struct {
		URL string `json:"url"`
	} `json:"bugs"`
}

// person is an author or a maintainer. The registry writes it as an object, or in
// older documents as a single "name <email> (url)" string.
type person struct {
	Name string `json:"name"`
}

func (p *person) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		p.Name = strings.TrimSpace(strings.SplitN(text, "<", 2)[0])
		return nil
	}
	var object struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	p.Name = object.Name
	return nil
}

// Detail reads one package: what it says about itself and the version the registry
// calls current. When a copy is installed at another version, that version's own
// entry is read too, since a license can change between releases.
func Detail(ctx context.Context, name, version string) (manager.Detail, error) {
	client := util.HTTPClient()
	var latest manifest
	if err := util.GetJSON(ctx, client, Base+"/"+registryPath(name)+"/latest", &latest); err != nil {
		return manager.Detail{Name: name, Version: version}, err
	}
	detail := detailOf(latest, name)

	// The installed version's own entry, when it is not the newest: its license
	// and author are what this copy is under.
	if version != "" && version != latest.Version {
		var installed manifest
		if err := util.GetJSON(ctx, client, Base+"/"+registryPath(name)+"/"+version, &installed); err == nil {
			detail.Version = installed.Version
			detail.License = cmp.Or(installed.License, detail.License)
			detail.Publisher = cmp.Or(publisher(installed), detail.Publisher)
		}
	}
	if detail.Version == "" {
		detail.Version = version
	}
	return detail, nil
}

// detailOf turns one version's entry into the page's fields.
func detailOf(m manifest, name string) manager.Detail {
	return manager.Detail{
		Name:      cmp.Or(m.Name, name),
		Version:   m.Version,
		Latest:    m.Version,
		Summary:   m.Description,
		Homepage:  cmp.Or(m.Homepage, pageFromRepository(m.Repository.URL), m.Bugs.URL),
		License:   m.License,
		Publisher: publisher(m),
		Source:    "registry.npmjs.org",
	}
}

// publisher is who the package belongs to: the first maintainer, which is the
// account that publishes it, or the author when a package has no maintainers.
func publisher(m manifest) string {
	for _, person := range m.Maintainers {
		if person.Name != "" {
			return person.Name
		}
	}
	return m.Author.Name
}

// registryPath is a package name's path in the registry: a scoped name keeps its
// slash, everything else is one segment.
func registryPath(name string) string {
	return strings.TrimPrefix(name, "/")
}

// pageFromRepository turns a repository URL into the page a reader can open: a
// git address, or the shorthand npm documents such as "github:user/repo".
func pageFromRepository(url string) string {
	url = strings.TrimSpace(url)
	switch {
	case url == "":
		return ""
	case strings.HasPrefix(url, "git+"):
		url = strings.TrimPrefix(url, "git+")
	case strings.HasPrefix(url, "git://"):
		url = "https://" + strings.TrimPrefix(url, "git://")
	case strings.HasPrefix(url, "github:"):
		return "https://github.com/" + strings.TrimPrefix(url, "github:")
	}
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	return ""
}
