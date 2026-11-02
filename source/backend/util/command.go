package util

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (r ExecResult) Output() string {
	if r.Stderr == "" {
		return r.Stdout
	}
	if r.Stdout == "" {
		return r.Stderr
	}
	return r.Stdout + "\n" + r.Stderr
}

// Exec executes a command with the given arguments and environment variables, returns the combined output and error if any
func Exec(ctx context.Context, bin string, args []string, envs []string) (string, error) {
	res, err := Run(ctx, bin, args, envs)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

func Run(ctx context.Context, bin string, args []string, envs []string) (ExecResult, error) {
	return RunAllowExitCodes(ctx, bin, args, envs, 0)
}

func RunAllowExitCodes(ctx context.Context, bin string, args []string, envs []string, allowedExitCodes ...int) (ExecResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if len(envs) > 0 {
		cmd.Env = append(cmd.Environ(), envs...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := ExecResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: 0,
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		for _, code := range allowedExitCodes {
			if res.ExitCode == code {
				return res, nil
			}
		}
		if res.Stderr != "" {
			return res, fmt.Errorf("%s failed with exit code %d: %s", bin, res.ExitCode, res.Stderr)
		}
		return res, err
	}
	return res, nil
}

// LineHandler processes a line string and returns (data, continue).
// if data is nil and continue is true, the line will be skipped
// if data is nil and continue is false, the line will be processed but the command will be killed immediately after
type LineHandler[T any] func(line string) (T, error)

// ExecAndCollect execute a command with a line handler func, returns (result, error)
func ExecAndCollect[T any](ctx context.Context, bin string, args []string, envs []string, handler LineHandler[T]) ([]T, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if len(envs) > 0 {
		cmd.Env = append(cmd.Environ(), envs...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		collector []T
		readErr   error
		once      sync.Once
	)

	// once an error occurred, kill the cmd process
	setErr := func(e error) {
		once.Do(func() {
			readErr = e
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
	}

	read := func(r io.Reader, collect bool) {
		// stdout collects but stderr does not
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				setErr(ctx.Err())
				return
			default:
			}
			if !collect {
				continue
			}

			line := scanner.Text()
			data, err := handler(line)
			if err != nil {
				// once parse error occurred, end the routine
				setErr(err)
				return
			}
			_ = WithMutex(&mu, func() error {
				collector = append(collector, data)
				return nil
			})
		}
		// scanner error
		if err := scanner.Err(); err != nil {
			setErr(err)
		}
	}
	wg.Go(func() {
		read(stdout, true)
	})
	wg.Go(func() {
		read(stderr, false)
	})

	wg.Wait()

	waitErr := cmd.Wait()

	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		return nil, waitErr
	}
	return collector, nil
}

func GetExecPath(bin string) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", bin)
	}
	return path, nil
}

// CheckCommandVersion executes the command with version args and checks if it runs successfully, returns error if command is not found or returns an error
func CheckCommandVersion(ctx context.Context, bin string, versionArgs []string) error {
	output, err := Exec(ctx, bin, versionArgs, nil)
	if err != nil {
		return err
	}
	if strings.Contains(output, "not found") {
		return fmt.Errorf("%s not available", bin)
	}
	return nil
}
