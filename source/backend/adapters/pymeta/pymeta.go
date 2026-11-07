// Package pymeta reads the metadata an installed Python distribution carries:
// the METADATA file pip writes into its .dist-info directory, which pip, pipx and
// uv all leave behind. It is what the copy on this machine says about itself,
// with no command run and no network.
package pymeta

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// FromVenv reads the metadata of the distribution named project inside a virtual
// environment, as pipx and uv tool keep them: site-packages sits under lib/
// python<version>/, and the distribution's directory is named after its
// normalized name. A project that is not there is an error, so the caller can
// fall back to the registry.
func FromVenv(venvDir, project string) (manager.Detail, error) {
	sitePackages, err := filepath.Glob(filepath.Join(venvDir, "lib", "python*", "site-packages"))
	if err != nil || len(sitePackages) == 0 {
		return manager.Detail{Name: project}, os.ErrNotExist
	}
	return FromSitePackages(sitePackages, project)
}

// FromSitePackages reads the metadata of project in one of the directories its
// distributions are installed into, which is where pip keeps them for the
// interpreter it runs under.
func FromSitePackages(sitePackages []string, project string) (manager.Detail, error) {
	want := normalize(project)
	for _, dir := range sitePackages {
		matches, err := filepath.Glob(filepath.Join(dir, "*.dist-info"))
		if err != nil {
			continue
		}
		for _, distInfo := range matches {
			// The directory name is "<name>-<version>.dist-info" with the name
			// normalized, which cannot be split apart again when the name has
			// dashes of its own: the metadata says what it is.
			detail, err := readMetadata(filepath.Join(distInfo, "METADATA"))
			if err != nil {
				continue
			}
			if normalize(detail.Name) == want {
				return detail, nil
			}
		}
	}
	return manager.Detail{Name: project}, os.ErrNotExist
}

// readMetadata parses a METADATA file: RFC 822 headers, one "Name: value" per
// line, a header repeated when a project has several of it, and the README after
// the first blank line.
func readMetadata(path string) (manager.Detail, error) {
	file, err := os.Open(path)
	if err != nil {
		return manager.Detail{}, err
	}
	defer file.Close()

	detail := manager.Detail{Source: "installed metadata"}
	var projectURLs []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break // the long description starts here
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "name":
			detail.Name = value
		case "version":
			detail.Version = value
		case "summary":
			detail.Summary = value
		case "home-page":
			detail.Homepage = value
		case "author":
			detail.Publisher = value
		case "author-email":
			if detail.Publisher == "" {
				detail.Publisher = emailName(value)
			}
		case "license":
			detail.License = value
		case "license-expression":
			detail.License = value
		case "project-url":
			projectURLs = append(projectURLs, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return manager.Detail{}, err
	}
	if detail.Homepage == "" {
		detail.Homepage = homeFromProjectURLs(projectURLs)
	}
	return detail, nil
}

// normalize is the name form the metadata directories use: lower case, with runs
// of -_. written as one -.
func normalize(name string) string {
	var b strings.Builder
	dashed := false
	for _, r := range strings.ToLower(name) {
		switch r {
		case '-', '_', '.':
			if !dashed {
				b.WriteByte('-')
				dashed = true
			}
		default:
			b.WriteRune(r)
			dashed = false
		}
	}
	return b.String()
}

// emailName is the name in a "Name <address>" header, or the address alone.
func emailName(value string) string {
	if before, _, ok := strings.Cut(value, "<"); ok {
		if name := strings.TrimSpace(before); name != "" {
			return name
		}
	}
	return strings.TrimSpace(value)
}

// homeFromProjectURLs picks the project's page among the addresses it declares,
// which is where modern projects put it instead of the old Home-page header.
func homeFromProjectURLs(values []string) string {
	byLabel := map[string]string{}
	for _, value := range values {
		label, url, ok := strings.Cut(value, ",")
		if !ok {
			continue
		}
		byLabel[strings.ToLower(strings.TrimSpace(label))] = strings.TrimSpace(url)
	}
	for _, label := range []string{"homepage", "home", "website", "repository", "source"} {
		if url := byLabel[label]; url != "" {
			return url
		}
	}
	return ""
}
