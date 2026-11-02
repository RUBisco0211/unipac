package main

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
)

func newFastDemo() *demoService {
	s := newDemoService()
	s.lineDelay = time.Millisecond
	return s
}

func TestDemoUpdateCompletesAndMovesThePackage(t *testing.T) {
	s := newFastDemo()
	result, err := s.UpdatePackage(manager.Package{Manager: "brew", Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0"}, manager.ActionOptions{})
	if err != nil || !result.Success {
		t.Fatalf("update = %+v, %v", result, err)
	}

	list := s.ListTasks()
	if len(list) != 1 {
		t.Fatalf("tasks = %d, want 1", len(list))
	}
	task := list[0]
	if task.State != tasks.StateSucceeded || !task.OutputKept {
		t.Fatalf("task = %+v", task)
	}
	if task.Command == "" || task.ExecPath == "" || len(task.Output) == 0 {
		t.Fatalf("task lacks its console data: %+v", task)
	}
	if s.UnfinishedTasks() != 0 {
		t.Fatalf("unfinished tasks = %d", s.UnfinishedTasks())
	}

	packages, err := s.GetCachedPackages()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		if pkg.Manager == "brew" && pkg.Name == "ripgrep" {
			if pkg.Outdated || pkg.Version != "15.0.0" {
				t.Fatalf("ripgrep after update = %+v", pkg)
			}
			return
		}
	}
	t.Fatal("ripgrep left the inventory")
}

// The first update of libpng fails; that is what the Retry button is demoed on.
func TestDemoLibpngFailsOnceThenRetrySucceeds(t *testing.T) {
	s := newFastDemo()
	first, err := s.UpdatePackage(manager.Package{Manager: "brew", Name: "libpng", Version: "1.6.43", LatestVersion: "1.6.44"}, manager.ActionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Success {
		t.Fatal("the first libpng update should fail")
	}
	list := s.ListTasks()
	if len(list) != 1 || list[0].State != tasks.StateFailed {
		t.Fatalf("first outcome = %+v", list)
	}

	second, err := s.UpdatePackage(manager.Package{Manager: "brew", Name: "libpng", Version: "1.6.43", LatestVersion: "1.6.44"}, manager.ActionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Success {
		t.Fatalf("the retry failed: %+v", second)
	}
	list = s.ListTasks()
	if len(list) != 2 {
		t.Fatalf("tasks after retry = %d, want 2 (the failed one is kept)", len(list))
	}
	states := map[tasks.State]int{}
	for _, task := range list {
		states[task.State]++
	}
	if states[tasks.StateFailed] != 1 || states[tasks.StateSucceeded] != 1 {
		t.Fatalf("task states = %v", states)
	}
}

func TestDemoBatchCreatesOneTaskPerPackage(t *testing.T) {
	s := newFastDemo()
	result, err := s.BatchUpdatePackages([]manager.Package{
		{Name: "ripgrep", Manager: "brew"},
		{Name: "typescript", Manager: "npm"},
		{Name: "requests", Manager: "pip"},
	}, manager.ActionOptions{})
	if err != nil || !result.Success {
		t.Fatalf("batch = %+v, %v", result, err)
	}
	list := s.ListTasks()
	if len(list) != 3 {
		t.Fatalf("tasks = %d, want 3", len(list))
	}
	for _, task := range list {
		if task.State != tasks.StateSucceeded {
			t.Fatalf("task = %+v", task)
		}
	}
}

func TestDemoStopTasksFailsRunningWork(t *testing.T) {
	s := newDemoService()
	s.lineDelay = 300 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.UpdatePackage(manager.Package{Manager: "brew", Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0"}, manager.ActionOptions{})
	}()

	deadline := time.Now().Add(time.Second)
	for {
		if list := s.ListTasks(); len(list) == 1 && list[0].State == tasks.StateRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the simulated task never started")
		}
		time.Sleep(5 * time.Millisecond)
	}

	s.StopTasks()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stopped action did not return")
	}

	list := s.ListTasks()
	if len(list) != 1 || list[0].State != tasks.StateCancelled {
		t.Fatalf("stopped task = %+v", list)
	}
	if list[0].Message != tasks.InterruptedMessage {
		t.Fatalf("message = %q", list[0].Message)
	}
	if s.UnfinishedTasks() != 0 {
		t.Fatalf("unfinished tasks = %d", s.UnfinishedTasks())
	}
}

func TestDemoInstallAndUninstallMoveThePackage(t *testing.T) {
	s := newFastDemo()

	results, err := s.SearchPackages("fzf", nil)
	if err != nil || len(results) != 1 {
		t.Fatalf("search = %+v, %v", results, err)
	}
	result, err := s.InstallPackage(results[0], manager.ActionOptions{})
	if err != nil || !result.Success {
		t.Fatalf("install = %+v, %v", result, err)
	}
	packages, _ := s.GetCachedPackages()
	found := false
	for _, pkg := range packages {
		if pkg.Manager == "brew" && pkg.Name == "fzf" && pkg.Installed {
			found = true
		}
	}
	if !found {
		t.Fatal("fzf did not enter the inventory")
	}
	if results, _ := s.SearchPackages("fzf", nil); len(results) != 0 {
		t.Fatalf("an installed package is still installable: %+v", results)
	}

	pkg := manager.Package{Manager: "brew", Name: "fzf", Version: "0.55.0"}
	if result, err := s.UninstallPackage(pkg, manager.ActionOptions{}); err != nil || !result.Success {
		t.Fatalf("uninstall = %+v, %v", result, err)
	}
	packages, _ = s.GetCachedPackages()
	for _, pkg := range packages {
		if pkg.Manager == "brew" && pkg.Name == "fzf" {
			t.Fatal("fzf stayed after uninstall")
		}
	}
}

func TestDemoCancelTask(t *testing.T) {
	s := newDemoService()
	s.lineDelay = 300 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.UpdatePackage(manager.Package{Manager: "brew", Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0"}, manager.ActionOptions{})
	}()

	deadline := time.Now().Add(time.Second)
	var id string
	for {
		list := s.ListTasks()
		if len(list) == 1 && list[0].State == tasks.StateRunning {
			id = list[0].ID
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the simulated task never started")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := s.CancelTask(id); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the cancelled action did not return")
	}
	list := s.ListTasks()
	if len(list) != 1 || list[0].State != tasks.StateCancelled || list[0].Message != tasks.CancelledMessage {
		t.Fatalf("cancelled task = %+v", list)
	}
	if err := s.CancelTask(id); err == nil {
		t.Fatal("cancelling a finished task succeeded")
	}
}

func TestDemoSearchFiltersManagersAtTheSource(t *testing.T) {
	s := newFastDemo()
	all, _ := s.SearchPackages("fzf", nil)
	if len(all) != 1 {
		t.Fatalf("all managers search = %+v", all)
	}
	only, _ := s.SearchPackages("fzf", []string{"brew"})
	if len(only) != 1 {
		t.Fatalf("brew search = %+v", only)
	}
	none, _ := s.SearchPackages("fzf", []string{"npm"})
	if len(none) != 0 {
		t.Fatalf("npm search = %+v", none)
	}
	disabled, _ := s.SearchPackages("fzf", []string{})
	if len(disabled) != 0 {
		t.Fatalf("empty manager set = %+v", disabled)
	}
}
