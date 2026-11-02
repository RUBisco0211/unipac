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
