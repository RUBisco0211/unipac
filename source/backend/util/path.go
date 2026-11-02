package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/lo"
)

var AppRoot string

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
