package manager

// Info describes a package manager to the backend and the views.
type Info struct {
	ID       string `json:"id,omitempty"`
	ExecName string `json:"exec_name,omitempty"`
	Name     string `json:"name,omitempty"`
	// Enabled reports whether UniPac uses the manager: its executable answered
	// and the user has not turned it off.
	Enabled bool `json:"enabled,omitempty"`
	// Available reports whether the executable was found and answered. It stays
	// true for a manager the user turned off, so the settings page can tell
	// "off" from "missing".
	Available bool `json:"available,omitempty"`
	// ExecPath is the executable in use: the saved path when there is one,
	// otherwise the one found on PATH.
	ExecPath string `json:"exec_path,omitempty"`
	// Version is what that executable reports for --version, read once during
	// preflight. It is empty for a manager that could not be probed.
	Version string `json:"version,omitempty"`
	// Website is the manager's own documentation, declared by its adapter: the
	// settings page shows the address and the window's way to open it. It does
	// not depend on the executable being there.
	Website      string       `json:"website,omitempty"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

// Settings is the user's saved configuration for one manager. The zero value
// means "on, detected automatically", so a config without an entry behaves as
// the default.
type Settings struct {
	// Disabled turns the manager off without changing its executable.
	Disabled bool `json:"disabled,omitempty"`
	// ExecPath overrides the executable found on PATH. Empty detects it.
	ExecPath string `json:"exec_path,omitempty"`
}
