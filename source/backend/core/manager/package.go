package manager

type Package struct {
	Name          string `json:"name,omitempty" db:"name"`
	Fullname      string `json:"fullname,omitempty" db:"fullname"`
	Version       string `json:"version,omitempty" db:"version"`
	LatestVersion string `json:"latest_version,omitempty" db:"latest_version"`
	Manager       string `json:"manager,omitempty" db:"manager"`
	Installed     bool   `json:"installed" db:"installed"`
	Outdated      bool   `json:"outdated" db:"outdated"`
	Description   string `json:"description,omitempty" db:"description"`
}

// Detail is what a package's own entry says about it, gathered from the manager's
// tool for what it keeps locally and from the registry where the project
// publishes the rest. Every field is optional: a manager that cannot answer one
// leaves it empty, and a page that shows the same rows for every manager shows a
// dash.
type Detail struct {
	Name string `json:"name,omitempty"`
	// Version is the installed version and Latest what the manager offers, which
	// differ for an outdated package.
	Version string `json:"version,omitempty"`
	Latest  string `json:"latest,omitempty"`
	// Summary is the one line the project describes itself with.
	Summary string `json:"summary,omitempty"`
	// Homepage, Publisher and License are what the project declares about itself:
	// the address of its page, who publishes it, and the terms.
	Homepage  string `json:"homepage,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	License   string `json:"license,omitempty"`
	// Path is where the installed copy lives: a package directory, a cask's
	// room, or the executable a crate installed.
	Path string `json:"path,omitempty"`
	// Source says where the facts came from, so a reader can tell a local answer
	// from a registry's: the tool's name, or its registry's.
	Source string `json:"source,omitempty"`
}

// Complete reports whether a detail has the facts a page would show, which is
// what a cache keeps and what a registry answer adds.
func (d Detail) Complete() bool {
	return d.Summary != "" || d.Homepage != "" || d.Publisher != "" || d.License != ""
}
