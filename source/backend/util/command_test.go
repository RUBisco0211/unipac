package util

import (
	"context"
	"os/exec"
	"path/filepath"
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
