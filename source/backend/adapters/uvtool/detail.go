package uvtool

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/adapters/pymeta"
	"github.com/RUBisco0211/unipac/source/backend/adapters/pypi"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads what uv installed as a tool: its environment's metadata says what
// the project is, and where uv keeps that environment says where the copy lives.
// All of it is local.
func (a *Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	dir := a.toolsDir()
	detail, err := pymeta.FromVenv(dir+"/"+pkg.Name, pkg.Name)
	if err != nil {
		// Not installed here: a search result has no environment to read.
		return pypi.Detail(a.ctx, pkg.Name)
	}
	detail.Path = dir + "/" + pkg.Name
	if detail.Version == "" {
		detail.Version = pkg.Version
	}
	if latest, err := pypi.Latest(a.ctx, pkg.Name); err == nil {
		detail.Latest = latest
	}
	return detail, nil
}

// toolsDir is where uv keeps its tool environments, asked of uv itself so a user
// who set UV_TOOL_DIR is answered correctly, and the default when the command
// cannot say.
func (a *Adapter) toolsDir() string {
	if out, err := util.Exec(a.ctx, a.info.ExecPath, []string{"tool", "dir"}, nil); err == nil {
		if dir := strings.TrimSpace(out); dir != "" {
			return dir
		}
	}
	return util.HomePath(".local", "share", "uv", "tools")
}
