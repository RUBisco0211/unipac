package pip

import (
	"cmp"
	"context"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/adapters/pypi"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads what the installed copy says about itself with `pip show`, which
// prints name, version, summary, home page, author, license and location as plain
// lines. It is all local: pip writes the metadata beside the package, and the
// location it names is where the copy lives.
func (a *Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	output, err := util.Exec(a.ctx, a.info.ExecPath, []string{"show", pkg.Name}, nil)
	if err != nil {
		return manager.Detail{Name: pkg.Name, Version: pkg.Version}, err
	}
	detail := parseShow(output, pkg)
	// The registry knows the current version; the installed copy does not. A
	// scan that already found it saves the call.
	detail.Latest = cmp.Or(pkg.LatestVersion, latestFromPyPI(a.ctx, pkg))
	return detail, nil
}

// latestFromPyPI is the version PyPI calls current, or "" when the registry does
// not answer: a page shows what it has rather than failing over one field.
func latestFromPyPI(ctx context.Context, pkg manager.Package) string {
	latest, err := pypi.Latest(ctx, pkg.Name)
	if err != nil {
		return ""
	}
	return latest
}

// parseShow reads `pip show`'s output: "Name: value" lines, with continuation
// lines indented under a multi-line value such as License.
func parseShow(output string, pkg manager.Package) manager.Detail {
	detail := manager.Detail{Name: pkg.Name, Version: pkg.Version, Source: "pip show"}
	values := map[string]string{}
	var last string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if last != "" {
				values[last] = strings.TrimSpace(values[last] + " " + strings.TrimSpace(line))
			}
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		last = strings.ToLower(strings.TrimSpace(name))
		values[last] = strings.TrimSpace(value)
	}
	if name := values["name"]; name != "" {
		detail.Name = name
	}
	if version := values["version"]; version != "" {
		detail.Version = version
	}
	detail.Summary = values["summary"]
	detail.Homepage = values["home-page"]
	if detail.Homepage == "" {
		detail.Homepage = values["homepage"]
	}
	detail.Publisher = values["author"]
	if detail.Publisher == "" {
		detail.Publisher = emailName(values["author-email"])
	}
	detail.License = values["license"]
	if detail.License == "" {
		detail.License = values["license-expression"]
	}
	if location := values["location"]; location != "" {
		detail.Path = location + "/" + pkg.Name
	}
	return detail
}

// emailName is the name in a "Name <address>" value, or the address alone.
func emailName(value string) string {
	if before, _, ok := strings.Cut(value, "<"); ok {
		if name := strings.TrimSpace(before); name != "" {
			return name
		}
	}
	return strings.TrimSpace(value)
}
