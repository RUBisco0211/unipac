package manager

type Package struct {
	Name          string `json:"name,omitempty" db:"name"`
	Fullname      string `json:"fullname,omitempty" db:"fullname"`
	Version       string `json:"version,omitempty" db:"version"`
	LatestVersion string `json:"latest_version,omitempty" db:"latest_version"`
	Manager       string `json:"manager,omitempty" db:"manager"`
	Installed     bool   `json:"installed" db:"installed"`
	Outdated      bool   `json:"outdated" db:"outdated"`
	IsGUI         bool   `json:"is_gui" db:"is_gui"`
	Description   string `json:"description,omitempty" db:"description"`
}
