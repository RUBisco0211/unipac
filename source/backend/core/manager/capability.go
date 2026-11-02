package manager

type Capabilities struct {
	ListInstalled  bool `json:"list_installed,omitempty"`
	ListOutdated   bool `json:"list_outdated,omitempty"`
	GetPackageInfo bool `json:"get_package_info,omitempty"`
	Search         bool `json:"search,omitempty"`
	Install        bool `json:"install,omitempty"`
	Uninstall      bool `json:"uninstall,omitempty"`
	Update         bool `json:"update,omitempty"`
	ListVersions   bool `json:"list_versions,omitempty"`
}

// DefaultCapabilities returns the default capabilities for a manager adapter (can be overridden by specific adapters)
func DefaultCapabilities() *Capabilities {
	return &Capabilities{
		ListInstalled:  true,
		ListOutdated:   false,
		GetPackageInfo: true,
		Search:         false,
		Install:        true,
		Uninstall:      true,
		Update:         true,
		ListVersions:   false,
	}
}

// WithSearch enables or disables the Search capability
func (cap *Capabilities) WithSearch(val bool) *Capabilities {
	cap.Search = val
	return cap
}

// WithListVersions enables or disables the ListVersions capability
func (cap *Capabilities) WithListVersions(val bool) *Capabilities {
	cap.ListVersions = val
	return cap
}

// WithListOutdated enables or disables the ListOutdated capability
func (cap *Capabilities) WithListOutdated(val bool) *Capabilities {
	cap.ListOutdated = val
	return cap
}

func (cap *Capabilities) WithGetPackageInfo(val bool) *Capabilities {
	cap.GetPackageInfo = val
	return cap
}

func (cap *Capabilities) WithInstall(val bool) *Capabilities {
	cap.Install = val
	return cap
}

func (cap *Capabilities) WithUninstall(val bool) *Capabilities {
	cap.Uninstall = val
	return cap
}

func (cap *Capabilities) WithUpdate(val bool) *Capabilities {
	cap.Update = val
	return cap
}
