package brew

type packageItem struct {
	Name              string   `json:"name,omitempty"`
	InstalledVersions []string `json:"installed_versions,omitempty"`
	CurrentVersion    string   `json:"current_version,omitempty"`
}

// jsonOutput represents the JSON output structure for `brew ** --json=v2` command (used for outdated)
type jsonOutput struct {
	Formulae []packageItem `json:"formulae,omitempty"`
	Casks    []packageItem `json:"casks,omitempty"`
}

// brewFormulaInfo represents a formula entry in `brew info --json=v2` output
type brewFormulaInfo struct {
	Name     string `json:"name"`
	Desc     string `json:"description,omitempty"`
	Versions struct {
		Stable string `json:"stable,omitempty"`
	} `json:"versions,omitempty"`
}

// brewCaskInfo represents a cask entry in `brew info --json=v2` output
type brewCaskInfo struct {
	Token   string `json:"token"`
	Version string `json:"version,omitempty"`
	Desc    string `json:"desc,omitempty"`
}

// brewInfoOutput represents the top-level output of `brew info --json=v2`
type brewInfoOutput struct {
	Formulae []brewFormulaInfo `json:"formulae"`
	Casks    []brewCaskInfo    `json:"casks"`
}
