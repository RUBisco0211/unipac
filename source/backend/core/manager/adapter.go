package manager

import (
	"context"
	"fmt"

	"github.com/RUBisco0211/unipac/source/backend/util"
)

type ActionResult struct {
	Success bool   `json:"success,omitempty"`
	Message string `json:"message,omitempty"`
}

func SuccessResult(msg string) ActionResult {
	return ActionResult{
		Success: true,
		Message: msg,
	}
}

// ErrorResult creates an unsuccessful ActionResult with the given message
func ErrorResult(msg string) ActionResult {
	return ActionResult{
		Success: false,
		Message: msg,
	}
}

// ActionOptions contains optional package action flags.
type ActionOptions struct {
	Version string `json:"version,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
	Verbose bool   `json:"verbose,omitempty"`
}

type Adapter interface {
	Info() *Info

	Preflight() error
	Setup() error

	GetPackageInfo(pkg Package) (string, error)
	ListInstalled() ([]Package, error)
	ListOutdated() ([]Package, error)
	Search(keyword string) ([]Package, error)
	Install(names []Package, opt ActionOptions) (ActionResult, error)
	Uninstall(names []Package, opt ActionOptions) (ActionResult, error)
	Update(names []Package, opt ActionOptions) (ActionResult, error)
	ListVersions(pkg Package) ([]string, error)
}

// ContextualAdapter is implemented by an adapter that can run one action with
// its own context. The registry binds the action's context before calling it,
// so what the action's commands print reaches that action's console and not a
// later one's.
type ContextualAdapter interface {
	WithContext(ctx context.Context) Adapter
}

// AdapterConstructor builds one adapter. It must stay cheap and free of I/O:
// the registry calls it for every manager on each settings change, and runs
// Preflight separately for the managers that actually need checking.
type AdapterConstructor func(ctx context.Context) Adapter

func Preflight(a Adapter, ctx context.Context) error {
	info := a.Info()
	// A configured executable is the user's choice: it is checked as given and
	// never replaced by one found on PATH, so a typo shows up as unavailable
	// instead of silently running a different binary.
	if info.ExecPath != "" {
		if err := util.CheckCommandVersion(ctx, info.ExecPath, []string{"--version"}); err != nil {
			return fmt.Errorf("%s is not usable at %s: %w", info.ID, info.ExecPath, err)
		}
		return nil
	}

	path, err := util.GetExecPath(info.ExecName)
	if err != nil {
		return fmt.Errorf("%s not found: %w", info.ID, err)
	}
	info.ExecPath = path
	if err := util.CheckCommandVersion(ctx, info.ExecPath, []string{"--version"}); err != nil {
		return fmt.Errorf("%s not available: %w", info.ID, err)
	}
	return nil
}
