package util

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A command that never started has no exit code. Treating the zero value as an
// allowed exit code made a missing executable look like a working one, which is
// how a typo in a configured path used to pass preflight.
func TestRunDoesNotAcceptAFailedStart(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-binary")
	if _, err := Run(context.Background(), missing, []string{"--version"}, nil); err == nil {
		t.Fatal("a command that never started reported success")
	}
}

func TestRunAllowsListedExitCode(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}
	res, err := RunAllowExitCodes(context.Background(), sh, []string{"-c", "exit 3"}, nil, 0, 3)
	if err != nil {
		t.Fatalf("allowed exit code reported an error: %v", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit code = %d, want 3", res.ExitCode)
	}
}

// A task's console sees the command and what it prints while it runs, and the
// result still holds the full text.
func TestRunReportsEventsToContext(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}
	var (
		mu     sync.Mutex
		events []Event
	)
	ctx := WithReporter(context.Background(), func(ev Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	res, err := Run(ctx, sh, []string{"-c", "printf 'out one\\nout two\\n'; printf 'bad\\n' >&2"}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	mu.Lock()
	got := append([]Event(nil), events...)
	mu.Unlock()

	if len(got) == 0 || got[0].Kind != EventCommand {
		t.Fatalf("first event = %+v, want the command", got)
	}
	if got[0].Path != sh || !strings.Contains(got[0].Command, "out one") {
		t.Errorf("command event = %+v, want %s with its arguments", got[0], sh)
	}

	// stdout and stderr are read concurrently, so their relative order is not
	// fixed; the lines themselves are.
	lines := map[string]bool{}
	for _, ev := range got[1:] {
		if ev.Kind != EventOutput {
			t.Errorf("event = %+v, want output", ev)
			continue
		}
		lines[ev.Text] = true
	}
	for _, want := range []string{"out one", "out two", "bad"} {
		if !lines[want] {
			t.Errorf("line %q was not reported: %+v", want, got)
		}
	}
	if res.Stdout != "out one\nout two" {
		t.Errorf("collected stdout = %q", res.Stdout)
	}
	if res.Stderr != "bad" {
		t.Errorf("collected stderr = %q", res.Stderr)
	}
}

// A probe an adapter runs to decide what to do is not the command the user
// asked for: WithoutReporting keeps it, and its output, out of the task console.
func TestWithoutReportingDropsTheCommands(t *testing.T) {
	var events []Event
	ctx := WithReporter(context.Background(), func(ev Event) { events = append(events, ev) })

	if _, err := Exec(ctx, "printf", []string{"reported"}, nil); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("the command did not report")
	}

	events = nil
	if _, err := Exec(WithoutReporting(ctx), "printf", []string{"quiet"}, nil); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("a quiet command reported %v", events)
	}
}

// The managers disagree about how to print a version; these are the outputs the
// seven of them answered on one machine, and each one's version is the run of
// dot-separated numbers in it.
func TestVersionToken(t *testing.T) {
	cases := []struct {
		manager string
		output  string
		want    string
	}{
		{"brew", "Homebrew 7.0.12-105-g3cea970\n", "7.0.12"},
		{"npm", "11.17.0\n", "11.17.0"},
		{"bun", "1.4.2\n", "1.4.2"},
		{"uv", "uv 0.12.17 (635500036 2026-09-18 aarch64-apple-darwin)\n", "0.12.17"},
		{"cargo", "cargo 1.98.0 (797e8a9bc 2026-08-05)\n", "1.98.0"},
		{"pip", "pip 26.2.1 from /opt/homebrew/lib/python3.14/site-packages/pip (python 3.14)\n", "26.2.1"},
		{"pipx", "1.12.0\n", "1.12.0"},
		{"a tool with no version", "unknown\n", ""},
	}
	for _, tc := range cases {
		if got := VersionToken(tc.output); got != tc.want {
			t.Errorf("%s: %q -> %q, want %q", tc.manager, strings.TrimSpace(tc.output), got, tc.want)
		}
	}
}

// A version command that cannot run is how an unavailable manager is told apart,
// and the version it prints is what comes back.
func TestCommandVersion(t *testing.T) {
	if _, err := CommandVersion(context.Background(), "/nonexistent/tool", []string{"--version"}); err == nil {
		t.Fatal("a missing executable reported a version")
	}
	version, err := CommandVersion(context.Background(), "/bin/sh", []string{"-c", `echo "fake-tool 3.2.1 (abc)"`})
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != "3.2.1" {
		t.Fatalf("version = %q, want 3.2.1", version)
	}
}
