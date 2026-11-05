package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/lo"
)

var AppRoot string

// AppDirName is the directory this app keeps its files in, under the user's
// config directory. It is the bundle identifier from mygo.json: one directory
// for every build of the app, and a name that says whose it is.
const AppDirName = "com.rubisco.unipac"

// MigrateAppRoot moves what an earlier version kept under fromName to the
// directory this version uses, and reports whether it moved anything. It moves
// only when the old directory is there and the new one is not, so it runs once
// and never overwrites what is already in place.
func MigrateAppRoot(fromName string) (moved bool, err error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return false, err
	}
	oldDir := filepath.Join(base, fromName)
	newDir := filepath.Join(base, AppDirName)
	if _, err := os.Stat(newDir); err == nil {
		return false, nil
	}
	if _, err := os.Stat(oldDir); err != nil {
		// Nothing to move, which is a first run.
		return false, nil
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return false, err
	}
	return true, nil
}

func InitAppRoot(name string) error {
	dir, err := appDir(name)
	if err != nil {
		return err
	}
	AppRoot = dir
	return nil
}

func appDir(name string) (string, error) {
	userDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(userDir, name)
	return appDir, os.MkdirAll(appDir, 0755)
}

// SafeJoin joins the base path and sub paths, ensuring that the resulting path is within the base directory and that all necessary directories exist
func SafeJoin(basePath string, subPaths ...string) (string, error) {
	if !filepath.IsAbs(basePath) {
		return "", fmt.Errorf("base path must be absolute: %s", basePath)
	}
	subPaths = lo.Map(subPaths, func(p string, _ int) string {
		return filepath.Clean(p)
	})

	path := filepath.Join(append([]string{basePath}, subPaths...)...)
	dir := filepath.Dir(path)
	return path, os.MkdirAll(dir, 0755)
}
