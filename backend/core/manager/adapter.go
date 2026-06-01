package manager

import (
	"context"
	"fmt"
	"unipac-wails/backend/util"
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

type AdapterConstructor func(ctx context.Context) Adapter

func Preflight(a Adapter, ctx context.Context) error {
	if a.Info().ExecPath != "" {
		if err := util.CheckCommandVersion(
			ctx,
			a.Info().ExecPath,
			[]string{"--version"},
		); err == nil {
			return nil
		}
	}

	path, err := util.GetExecPath(a.Info().ExecName)
	if err != nil {
		return fmt.Errorf("%s not found: %w", a.Info().ID, err)
	}
	a.Info().ExecPath = path
	if err := util.CheckCommandVersion(
		ctx,
		a.Info().ExecPath,
		[]string{"--version"},
	); err != nil {
		return fmt.Errorf("%s not available: %w", a.Info().ID, err)
	}
	return nil
}
