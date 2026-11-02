package uvtool

// ToolRecord is one tool as `uv tool list` prints it, before it becomes a
// manager.Package. Exported so the parser can be tested from outside the
// package.
type ToolRecord struct {
	Name             string   `json:"name,omitempty"`
	Version          string   `json:"version,omitempty"`
	VersionSpecifier string   `json:"version_specifier,omitempty"`
	Python           string   `json:"python,omitempty"`
	ToolPath         string   `json:"tool_path,omitempty"`
	Executables      []string `json:"executables,omitempty"`
	LatestVersion    string   `json:"latest_version,omitempty"`
}
