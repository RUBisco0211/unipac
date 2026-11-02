package util

import "os"

func IsDev() bool {
	return os.Getenv("WAILS_DEV") == "true"
}
