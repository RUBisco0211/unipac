package uvtool

type toolRecord struct {
	Name             string   `json:"name,omitempty"`
	Version          string   `json:"version,omitempty"`
	VersionSpecifier string   `json:"version_specifier,omitempty"`
	Python           string   `json:"python,omitempty"`
	ToolPath         string   `json:"tool_path,omitempty"`
	Executables      []string `json:"executables,omitempty"`
	LatestVersion    string   `json:"latest_version,omitempty"`
}
