package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ToolDirs are where the package managers UniPac drives install their
// executables: the Homebrew prefixes, the per-user bins pipx, uv and cargo
// write into, and bun's. They follow the system ones, so a PATH from a shell
// still decides first.
var ToolDirs = []string{
	"/opt/homebrew/bin",
	"/opt/homebrew/sbin",
	"/usr/local/bin",
	"/usr/local/sbin",
	"~/.local/bin",
	"~/.cargo/bin",
	"~/.bun/bin",
}

// EnsureToolPath appends the tool directories that exist to PATH, and returns
// the PATH it set.
//
// An app opened from Finder, the Dock or Spotlight is launched by launchd with
// a minimal PATH (`/usr/bin:/bin:/usr/sbin:/sbin`), not with the one a shell
// had: every manager then looks missing and a scan replaces the cache with the
// few packages that PATH can find. Adding the directories the tools live in
// makes a launch from outside a shell behave like one from inside it.
func EnsureToolPath() string {
	path := extendPath(os.Getenv("PATH"), os.Getenv("HOME"), ToolDirs...)
	if path != os.Getenv("PATH") {
		_ = os.Setenv("PATH", path)
	}
	return path
}

// extendPath returns path with the directories of dirs that exist, and that
// path does not already hold, appended in order. "~" means home.
func extendPath(path, home string, dirs ...string) string {
	seen := make(map[string]bool, len(dirs))
	for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
		if dir != "" {
			seen[dir] = true
		}
	}
	out := path
	for _, dir := range dirs {
		if strings.HasPrefix(dir, "~/") {
			if home == "" {
				continue
			}
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
		}
		if seen[dir] {
			continue
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		if out != "" {
			out += string(os.PathListSeparator)
		}
		out += dir
		seen[dir] = true
	}
	return out
}
