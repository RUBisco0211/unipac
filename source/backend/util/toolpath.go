package util

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// shellPathTimeout bounds how long the login shell may take to report its PATH.
// A shell that sources a heavy rc file can take a moment; the app does not wait
// longer than this for it.
var shellPathTimeout = 3 * time.Second

// EnsureToolPath gives the process the PATH a terminal has, and returns it.
//
// An app opened from Finder, the Dock or Spotlight is launched by launchd with
// `PATH=/usr/bin:/bin:/usr/sbin:/sbin`, not with the one the user's shell
// builds: every manager then looks missing, and the scan that follows replaces
// the package cache with the few packages that PATH can find. Asking the login
// shell for its PATH is what makes a launch from outside a shell behave like
// one from inside it, with the user's own `nvm`, `volta` or `asdf` directories
// included rather than a list of guesses.
//
// The shell's PATH comes first and the process's own entries follow, so a
// directory this process was given still resolves. When the shell cannot be
// asked, PATH is left as it is and the failure is logged: probing managers then
// reports what it finds, which is what the settings page can fix with a path.
func EnsureToolPath() string {
	current := os.Getenv("PATH")
	shellPath, err := loginShellPath()
	if err != nil {
		slog.Warn("util: could not read the login shell's PATH; using the one this process has",
			"shell", os.Getenv("SHELL"), "error", err)
		return current
	}
	path := mergePaths(shellPath, current)
	if path != current {
		_ = os.Setenv("PATH", path)
	}
	slog.Debug("util: PATH taken from the login shell", "path", path)
	return path
}

// loginShellPath asks the user's shell for the PATH it would give a command. It
// tries an interactive login shell first, whose rc files set up tools like nvm,
// then a plain login shell.
func loginShellPath() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	var err error
	for _, args := range [][]string{
		{"-ilc", `printf %s "$PATH"`},
		{"-lc", `printf %s "$PATH"`},
	} {
		var out []byte
		out, err = runShell(shell, args...)
		if err != nil {
			continue
		}
		if path := lastLine(string(out)); path != "" {
			return path, nil
		}
	}
	if err == nil {
		err = errors.New("the shell printed no PATH")
	}
	return "", err
}

// runShell runs the shell with args, giving up after shellPathTimeout.
func runShell(shell string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), shellPathTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	// A login shell may print banners and warnings; only its last line is read.
	return cmd.Output()
}

// lastLine returns the last non-empty line of out, trimmed: rc files print
// their own messages before the PATH.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && strings.Contains(line, "/") {
			return line
		}
	}
	return ""
}

// mergePaths returns first's directories followed by those of rest that it lacks,
// without duplicates.
func mergePaths(first, rest string) string {
	seen := make(map[string]bool)
	var out []string
	for _, path := range []string{first, rest} {
		for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			out = append(out, dir)
		}
	}
	return strings.Join(out, string(os.PathListSeparator))
}
