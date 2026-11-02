package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJson(path string, data interface{}) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path is not absolute: %s", path)
	}

	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Errorf("fail to marshal json data: %v", err)
	}
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("fail to write to file %s: %v", path, err)
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
