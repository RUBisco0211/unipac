package pipx

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/adapters/pymeta"
	"github.com/RUBisco0211/unipac/source/backend/adapters/pypi"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads what pipx installed: the environment it keeps for the package says
// what the project is, and where that environment is says where the copy lives.
// All of it is local — pipx keeps the metadata pipx installs with.
func (a Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	venvsDir := a.venvsDir()
	detail, err := pymeta.FromVenv(venvsDir+"/"+pkg.Name, pkg.Name)
	if err != nil {
		return manager.Detail{Name: pkg.Name, Version: pkg.Version}, err
	}
	detail.Path = venvsDir + "/" + pkg.Name
	if detail.Version == "" {
		detail.Version = pkg.Version
	}
	if latest, err := pypi.Latest(a.ctx, pkg.Name); err == nil {
		detail.Latest = latest
	}
	return detail, nil
}

// venvsDir is where pipx keeps the environments, asked of pipx itself so a user
// who moved PIPX_HOME is answered correctly, and the default when the command
// cannot say.
func (a Adapter) venvsDir() string {
	if out, err := util.Exec(a.ctx, a.info.ExecPath,
		[]string{"environment", "--value", "PIPX_LOCAL_VENVS"}, nil); err == nil {
		if dir := strings.TrimSpace(out); dir != "" {
			return dir
		}
	}
	return util.HomePath(".local", "pipx", "venvs")
}
