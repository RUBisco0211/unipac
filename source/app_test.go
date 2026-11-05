package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// An install records the version it goes to. A search result is not installed,
// so it carries that version in Version and has no latest of its own.
func TestTaskVersions(t *testing.T) {
	cases := []struct {
		name      string
		kind      tasks.Kind
		pkg       manager.Package
		opt       manager.ActionOptions
		old, next string
	}{
		{"install from search", tasks.KindInstall, manager.Package{Version: "1.2.3"}, manager.ActionOptions{}, "", "1.2.3"},
		{"install of a chosen version", tasks.KindInstall, manager.Package{Version: "1.2.3"}, manager.ActionOptions{Version: "2.0.0"}, "", "2.0.0"},
		{"install with a latest", tasks.KindInstall, manager.Package{Version: "1.2.3", LatestVersion: "2.0.0"}, manager.ActionOptions{}, "", "2.0.0"},
		{"update", tasks.KindUpdate, manager.Package{Version: "1.2.3", LatestVersion: "2.0.0"}, manager.ActionOptions{}, "1.2.3", "2.0.0"},
		{"update without a latest", tasks.KindUpdate, manager.Package{Version: "1.2.3"}, manager.ActionOptions{}, "1.2.3", "1.2.3"},
		{"uninstall", tasks.KindUninstall, manager.Package{Version: "1.2.3", LatestVersion: "2.0.0"}, manager.ActionOptions{}, "1.2.3", ""},
	}
	for _, tc := range cases {
		old, next := taskVersions(tc.kind, tc.pkg, tc.opt)
		if old != tc.old || next != tc.next {
			t.Errorf("%s: versions = %q -> %q, want %q -> %q", tc.name, old, next, tc.old, tc.next)
		}
	}
}

// The directory the app keeps its files in is the bundle identifier, and the
// two must not drift apart: the identifier names the app to the system, and
// this names it on disk.
func TestAppDirNameIsTheBundleIdentifier(t *testing.T) {
	raw, err := os.ReadFile("../mygo.json")
	if err != nil {
		t.Skipf("mygo.json is not beside the package: %v", err)
	}
	var manifest struct {
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if util.AppDirName != manifest.Identifier {
		t.Errorf("app dir = %q, bundle identifier = %q", util.AppDirName, manifest.Identifier)
	}
}
