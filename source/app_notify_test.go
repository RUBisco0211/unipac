package main

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/config"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/RUBisco0211/unipac/source/views/settings"
)

// Turning the master switch off must not clear the switches under it: the page
// reads its state back from the service after every write, so a service that
// gates the answer on the master switch would wipe them — the boxes would go
// empty and stay empty across restarts. This is the regression test for exactly
// that: App.Notifications once reported NotifiesEnabled && Notifies, and a
// master turned off erased every switch on the page.
func TestNotificationsReportTheSwitchesWhateverTheMasterSays(t *testing.T) {
	util.AppRoot = t.TempDir()
	if err := config.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	a := &App{}

	// The state the user builds: everything on, then the master off.
	if err := a.SetNotifyUpdates(true); err != nil {
		t.Fatalf("updates: %v", err)
	}
	for _, kind := range config.NotifyKinds() {
		for _, succeeded := range []bool{true, false} {
			if err := a.SetNotifications(kind, succeeded, true); err != nil {
				t.Fatalf("%s succeeded=%v: %v", kind, succeeded, err)
			}
		}
	}
	if err := a.SetNotificationsEnabled(false); err != nil {
		t.Fatalf("master: %v", err)
	}

	// What a write answers with, and what the page shows after it: the saved
	// switches, not the master's gate.
	notify := settings.NotifyFrom(a)
	if notify.Enabled {
		t.Error("the master switch reported on")
	}
	if !notify.Updates {
		t.Error("the updates switch was cleared by the master switch")
	}
	for _, kind := range config.NotifyKinds() {
		outcome := notify.ForKind(kind)
		if !outcome.Success || !outcome.Failure {
			t.Errorf("%s's switches were cleared by the master switch: %+v", kind, outcome)
		}
	}

	// The config file holds them too, so a restart shows the same.
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !loaded.Notifies(tasks.KindUninstall, false) {
		t.Error("the saved switch did not survive in the file")
	}

	// The page's own gate: with the master switch back on, the answers are
	// what announces.
	notify.Enabled = true
	if !notify.Updates || !notify.ForKind(tasks.KindUninstall).Failure {
		t.Fatal("the answers were not restored with the master switch")
	}
}
