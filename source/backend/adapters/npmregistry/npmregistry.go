package npmregistry

import (
	"cmp"
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Base is where npm packages are read from. A test points it at a server of its
// own.
var Base = "https://registry.npmjs.org"

// registryPackage is the part of a registry document this app reads. The
// registry answers one object per package with everything published under it,
// including the publish time of every version, which is where the release date
// comes from.
type registryPackage struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Homepage    string            `json:"homepage"`
	License     string            `json:"license"`
	Author      registryPerson    `json:"author"`
	Maintainers []registryPerson  `json:"maintainers"`
	DistTags    map[string]string `json:"dist-tags"`
	Versions    map[string]any    `json:"versions"`
	Time        map[string]string `json:"time"`
}

// registryPerson is an author or a maintainer. The registry writes it as an
// object or, in older documents, as a single "name <email> (url)" string.
type registryPerson struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (p *registryPerson) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := jsonUnmarshal(data, &text); err != nil {
			return err
		}
		p.Name = strings.TrimSpace(strings.SplitN(text, "<", 2)[0])
		return nil
	}
	type plain registryPerson
	return jsonUnmarshal(data, (*plain)(p))
}

// Detail reads one package from the npm registry: npm and bun both install npm
// packages, and the registry answers what the CLIs do without needing another
// process. The release date is the installed version's, and the publisher the
// first maintainer, which is the account the package belongs to.
func Detail(ctx context.Context, name, version string) (manager.Detail, error) {
	var doc registryPackage
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/"+registryPath(name), &doc); err != nil {
		return manager.Detail{Name: name, Version: version}, err
	}
	detail := manager.Detail{
		Name:      cmp.Or(doc.Name, name),
		Version:   version,
		Latest:    doc.DistTags["latest"],
		Summary:   doc.Description,
		Homepage:  doc.Homepage,
		License:   doc.License,
		Publisher: publisher(doc),
		Source:    "registry.npmjs.org",
	}
	if detail.Version == "" {
		detail.Version = detail.Latest
	}
	return detail, nil
}

// registryPath is a package name's path in the registry: a scoped name keeps its
// slash, everything else is one segment.
func registryPath(name string) string {
	return strings.TrimPrefix(name, "/")
}

// publisher is who the package belongs to: the first maintainer, which is how
// the registry lists the account that publishes it, or the author when a package
// has no maintainers.
func publisher(doc registryPackage) string {
	for _, person := range doc.Maintainers {
		if person.Name != "" {
			return person.Name
		}
	}
	return doc.Author.Name
}

// Versions is every published version of a package, newest first by publish time,
// which is what a page offering a version to install lists. It is one request, the
// same document Detail reads.
func Versions(ctx context.Context, name string) ([]string, error) {
	var doc registryPackage
	if err := util.GetJSON(ctx, util.HTTPClient(), Base+"/"+registryPath(name), &doc); err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(doc.Versions))
	for version := range doc.Versions {
		versions = append(versions, version)
	}
	sort.SliceStable(versions, func(i, j int) bool {
		return doc.Time[versions[i]] > doc.Time[versions[j]]
	})
	return versions, nil
}

// jsonUnmarshal is encoding/json's, named here so the person decoder reads as
// one shape of the same answer.
func jsonUnmarshal(data []byte, out any) error { return json.Unmarshal(data, out) }
