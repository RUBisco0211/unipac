package bun

import (
	"github.com/RUBisco0211/unipac/source/backend/adapters/npmregistry"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads a package from the npm registry: bun installs npm packages, so the
// facts are npm's, and `bun info` cannot answer them — it needs a package.json in
// the working directory, which the app does not have. The directory is bun's own
// global tree, which is where `bun add -g` puts it.
func (a *Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	detail, err := npmregistry.Detail(a.ctx, pkg.Name, pkg.Version)
	detail.Path = util.HomePath(".bun", "install", "global", "node_modules", pkg.Name)
	return detail, err
}
