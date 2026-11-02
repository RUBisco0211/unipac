package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteJson writes data to path as indented JSON. It writes a temporary file
// beside the target and renames it into place, so a crash mid-write cannot
// leave a truncated file that the next load would replace with defaults.
func WriteJson(path string, data interface{}) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path is not absolute: %s", path)
	}

	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Errorf("fail to marshal json data: %v", err)
	}

	dir, name := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return fmt.Errorf("fail to create a temporary file for %s: %v", path, err)
	}
	tmpPath := tmp.Name()
	// No-op once the rename succeeded; otherwise it keeps a failed write from
	// leaving a stray file behind.
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(jsonData); err != nil {
		tmp.Close()
		return fmt.Errorf("fail to write to file %s: %v", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fail to write to file %s: %v", path, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("fail to set permissions on %s: %v", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fail to write to file %s: %v", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("fail to replace %s: %v", path, err)
	}
	return nil
}

func ReadJson[T interface{}](path string) (T, error) {
	var data T
	if !filepath.IsAbs(path) {
		return data, fmt.Errorf("path is not absolute: %s", path)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}
	if err = json.Unmarshal(bytes, &data); err != nil {
		return data, err
	}
	return data, nil
}
