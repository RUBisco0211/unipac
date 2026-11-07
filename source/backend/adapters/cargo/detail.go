package cargo

import (
	"os"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/adapters/cratesio"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Detail reads a crate from crates.io: `cargo install --list` prints a name and a
// version and nothing else, so the project's page, its description and its terms
// come from the registry. The path is the binary cargo installed, which is what is
// left of a crate on this machine — cargo keeps no source tree for it.
func (a Adapter) Detail(pkg manager.Package) (manager.Detail, error) {
	detail, err := cratesio.Detail(a.ctx, pkg.Name, pkg.Version)
	detail.Path = a.installedBinary(pkg.Name)
	return detail, err
}

// installedBinary is where cargo put the crate's first executable: its install
// root, asked of cargo, and the binary `cargo install --list` names under the
// crate.
func (a Adapter) installedBinary(name string) string {
	out, err := util.Exec(a.ctx, a.info.ExecPath, []string{"install", "--list"}, nil)
	if err != nil {
		return ""
	}
	binary := firstBinaryOf(out, name)
	if binary == "" {
		return ""
	}
	root := cargoHome()
	if root == "" {
		root = util.HomePath(".cargo")
	}
	return root + "/bin/" + binary
}

// firstBinaryOf is the first executable listed under a crate in `cargo install
// --list`, whose output is one "name vX.Y.Z:" line per crate followed by its
// binaries, indented.
func firstBinaryOf(list, name string) string {
	lines := strings.Split(list, "\n")
	for i, line := range lines {
		crateName, _, ok := strings.Cut(strings.TrimSpace(line), " v")
		if !ok || crateName != name {
			continue
		}
		for _, next := range lines[i+1:] {
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break
			}
			if binary := strings.TrimSpace(next); binary != "" {
				return binary
			}
		}
	}
	return ""
}

// cargoHome is CARGO_HOME, which moves where cargo installs its binaries.
func cargoHome() string {
	return strings.TrimSpace(os.Getenv("CARGO_HOME"))
}
