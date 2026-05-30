package registry

import (
	"context"
	"fmt"
	"log/slog"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/util"

	"github.com/samber/lo"
)

type Registry struct {
	ctx      context.Context
	adapters map[string]manager.Adapter
}

var Reg *Registry

func InitRegistry(ctx context.Context) {
	Reg = newRegistry(ctx)
}

func newRegistry(ctx context.Context) *Registry {
	reg := Registry{
		adapters: make(map[string]manager.Adapter, 0),
		ctx:      ctx,
	}

	return &reg
}

func (reg *Registry) register(adp manager.Adapter) {
	if err := adp.Preflight(); err != nil {
		slog.ErrorContext(
			reg.ctx,
			fmt.Sprintf("Manager '%s' is not available: %v", adp.Info().Name, err),
		)
		adp.Info().Enabled = false
	} else {
		slog.InfoContext(
			reg.ctx,
			fmt.Sprintf("Manager '%s' is available and registered", adp.Info().Name),
		)
		adp.Info().Enabled = true
	}
	reg.adapters[adp.Info().ID] = adp
}

// GetManagers returns the list of registered managers
func (reg *Registry) GetManagers() []manager.Info {
	return lo.Map(lo.Values(reg.adapters), func(adp manager.Adapter, _ int) manager.Info {
		return *adp.Info()
	})
}

func (reg *Registry) getAdapter(id string) (manager.Adapter, error) {
	adp, ok := reg.adapters[id]
	if !ok {
		return nil, fmt.Errorf("manager not found: %s", id)
	}
	return adp, nil
}

// GetInstalledPackages returns the list of all installed packages across all registered managers
func (reg *Registry) GetInstalledPackages() ([]manager.Package, error) {
	collectTasks := make([]util.CollectTask[manager.Package], 0, len(reg.adapters))
	for _, adp := range reg.adapters {
		if !adp.Info().Enabled {
			continue
		}
		collectTasks = append(collectTasks, func() ([]manager.Package, error) {
			pkgs, err := adp.ListInstalled()
			if err != nil {
				slog.ErrorContext(reg.ctx, "Failed to list installed packages", "manager", adp.Info().Name, "error", err)
			}
			return pkgs, nil
		})
	}
	return util.RunParallel(reg.ctx, collectTasks...)
}
