package manager

type Info struct {
	ID           string       `json:"id,omitempty"`
	ExecName     string       `json:"exec_name,omitempty"`
	Name         string       `json:"name,omitempty"`
	Enabled      bool         `json:"enabled,omitempty"`
	ExecPath     string       `json:"exec_path,omitempty"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}
