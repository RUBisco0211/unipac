package brew

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// brewInfo is the part of `brew info --json=v2` a page reads. The command answers
// from what is installed here, so this is the local half of a package's facts: a
// formula and a cask are different shapes, and which one a name is decides where
// the values sit.
type brewInfo struct {
	Formulae []formula `json:"formulae"`
	Casks    []cask    `json:"casks"`
}

type formula struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Desc     string `json:"desc"`
	Homepage string `json:"homepage"`
	License  any    `json:"license"`
	Tap      string `json:"tap"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Installed []struct {
		Version string `json:"version"`
	} `json:"installed"`
}

type cask struct {
	Token     string `json:"token"`
	Name      any    `json:"name"`
	Desc      string `json:"desc"`
	Homepage  string `json:"homepage"`
	Version   string `json:"version"`
	Installed string `json:"installed"`
	Tap       string `json:"tap"`
}

// Detail reads what Homebrew knows about a package without a network call: the
// description, the project's page, the version offered and when the copy here was
// installed. Homebrew does not say when a version was published upstream — a
// formula's history lives in its tap's git log — so the release date is left
// empty. The tap that ships a formula is where it comes from rather than who
// wrote it, so it is the closest thing to a publisher Homebrew has.
func (a *Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	empty := manager.Detail{Name: pkg.Name, Version: pkg.Version}
	output, err := util.Exec(a.ctx, a.info.ExecPath,
		[]string{"info", pkg.Name, "--json=v2"}, nil)
	if err != nil {
		return empty, fmt.Errorf("brew info %s: %w", pkg.Name, err)
	}
	var info brewInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return empty, fmt.Errorf("parsing brew info: %w", err)
	}
	switch {
	case len(info.Casks) > 0:
		detail := caskDetail(info.Casks[0], pkg)
		detail.Path = a.caskRoom(info.Casks[0].Token)
		return detail, nil
	case len(info.Formulae) > 0:
		detail := formulaDetail(info.Formulae[0], pkg)
		detail.Path = a.prefixOf(pkg.Name)
		return detail, nil
	default:
		return empty, fmt.Errorf("brew info %s returned nothing", pkg.Name)
	}
}

// prefixOf is where Homebrew installed a formula: its own `--prefix`, which
// answers without a network for anything it has.
func (a *Adapter) prefixOf(name string) string {
	out, err := util.Exec(a.ctx, a.info.ExecPath, []string{"--prefix", name}, nil)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// caskRoom is where a cask was unpacked, under Homebrew's cask room.
func (a *Adapter) caskRoom(token string) string {
	out, err := util.Exec(a.ctx, a.info.ExecPath, []string{"--caskroom"}, nil)
	if err != nil {
		return ""
	}
	room := strings.TrimSpace(out)
	if room == "" || token == "" {
		return ""
	}
	return room + "/" + token
}

// formulaDetail reads one formula: the versions sit with the formula, and the
// installed copy carries the time it was written.
func formulaDetail(f formula, pkg manager.Package) manager.Detail {
	detail := manager.Detail{
		Name:      f.FullName,
		Version:   pkg.Version,
		Latest:    f.Versions.Stable,
		Summary:   f.Desc,
		Homepage:  f.Homepage,
		License:   licenseText(f.License),
		Publisher: tapOwner(f.Tap),
		Source:    "brew",
	}
	if detail.Name == "" {
		detail.Name = f.Name
	}
	if len(f.Installed) > 0 && detail.Version == "" {
		detail.Version = f.Installed[0].Version
	}
	return detail
}

// caskDetail reads one cask, which keeps the version offered as a string and the
// installed one beside it, and records no install time.
func caskDetail(c cask, pkg manager.Package) manager.Detail {
	detail := manager.Detail{
		Name:      pkg.Name,
		Version:   c.Installed,
		Latest:    c.Version,
		Summary:   c.Desc,
		Homepage:  c.Homepage,
		Publisher: tapOwner(c.Tap),
		Source:    "brew",
	}
	if names, ok := c.Name.([]any); ok && len(names) > 0 {
		if name, ok := names[0].(string); ok && name != "" {
			detail.Name = name
		}
	}
	if detail.Version == "" {
		detail.Version = pkg.Version
	}
	return detail
}

// tapOwner names where a formula or cask comes from: Homebrew's own taps are the
// Homebrew project, and another tap is named by its owner, which is who maintains
// what it ships.
func tapOwner(tap string) string {
	switch {
	case tap == "":
		return "Homebrew"
	case strings.HasPrefix(tap, "homebrew/"):
		return "Homebrew"
	default:
		owner, _, _ := strings.Cut(tap, "/")
		return owner
	}
}

// licenseText is Homebrew's license field, which is a string for most formulae
// and an object for those that list several.
func licenseText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
