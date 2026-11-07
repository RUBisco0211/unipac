package npm

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/adapters/npmregistry"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads a package from the npm registry, and the directory npm installed it
// into from npm itself. The copy here carries only what `npm ls` prints, so the
// project's page, author and terms are the registry's to say; the registry is read
// over HTTP rather than through `npm view`, which answers the same document only
// when npm is on the machine and pays for a process.
func (a *Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	detail, err := npmregistry.Detail(a.ctx, pkg.Name, pkg.Version)
	detail.Path = a.globalDir(pkg.Name)
	return detail, err
}

// globalDir is where npm keeps a global package: the global root it reports, and
// the package's name under it.
func (a *Adapter) globalDir(name string) string {
	out, err := util.Exec(a.ctx, a.info.ExecPath, []string{"root", "-g"}, nil)
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return ""
	}
	return root + "/" + name
}
