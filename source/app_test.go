package main

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
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
