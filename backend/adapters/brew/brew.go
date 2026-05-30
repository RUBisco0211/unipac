package brew

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/util"
)

type Adapter struct {
	ctx  context.Context
	info *manager.Info
}

func NewAdapter(ctx context.Context) manager.Adapter {
	return &Adapter{
		ctx: ctx,
		info: &manager.Info{
			ID:       "brew",
			Name:     "Homebrew",
			ExecName: "brew",
			Capabilities: *manager.
				DefaultCapabilities().
				WithSearch(true).
				WithListVersions(false),
		},
	}
}

func (a *Adapter) lineParserFunc(cask bool) util.LineHandler[manager.Package] {
	return func(line string) (manager.Package, error) {
		var p manager.Package
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			slog.ErrorContext(a.ctx,
				"unexpected line format",
				"line", line,
			)
			return manager.Package{}, fmt.Errorf("unexpected line format: %s", line)
		}
		p.Name = strings.TrimSpace(parts[0])
		p.Version = strings.TrimSpace(parts[1])
		p.Installed = true
		p.IsGUI = cask
		p.Manager = a.Info().ID
		return p, nil
	}
}

// Info returns the manager info
func (a *Adapter) Info() *manager.Info {
	return a.info
}

// Preflight checks if brew is available in the system
func (a *Adapter) Preflight() error {
	return manager.Preflight(a, a.ctx)
}

// Setup performs any necessary setup for the manager (no-op for brew)
func (a *Adapter) Setup() error {
	return nil
}

// ListInstalled returns the list of installed packages for brew
func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	caskCollector := func() ([]manager.Package, error) {
		casks, err := util.ExecAndCollect(a.ctx,
			a.info.ExecPath,
			[]string{"list", "--cask", "--versions"},
			nil,
			a.lineParserFunc(true),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list casks: %w", err)
		}
		return casks, nil
	}

	formulaCollector := func() ([]manager.Package, error) {
		formulas, err := util.ExecAndCollect(a.ctx,
			a.info.ExecPath,
			[]string{"list", "--formula", "--versions"},
			nil,
			a.lineParserFunc(false),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list formulas: %w", err)
		}
		return formulas, nil
	}

	return util.CollectParallel(a.ctx, caskCollector, formulaCollector)
}

// ListOutdated returns the list of outdated packages for brew
func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	output, err := util.Exec(a.ctx,
		a.info.ExecPath,
		[]string{"outdated", "--json=v2"},
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"})
	if err != nil {
		return nil, fmt.Errorf("failed to list outdated packages: %w", err)
	}

	var obj jsonOutput
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse brew outdated output: %w", err)
	}

	// Process the JSON output to extract outdated packages
	var outdated []manager.Package
	for _, formula := range obj.Formulae {
		outdated = append(outdated, manager.Package{
			Name:          formula.Name,
			Version:       formula.InstalledVersions[0],
			LatestVersion: formula.CurrentVersion,
			Manager:       a.Info().ID,
			IsGUI:         false,
		})
	}

	for _, cask := range obj.Casks {
		outdated = append(outdated, manager.Package{
			Name:          cask.Name,
			Version:       cask.InstalledVersions[0],
			LatestVersion: cask.CurrentVersion,
			Manager:       a.Info().ID,
			IsGUI:         true,
			Outdated:      true,
		})
	}
	return outdated, nil
}

// GetPackageInfo returns the package info for a given package using brew
func (a *Adapter) GetPackageInfo(p manager.Package) (string, error) {
	output, err := util.Exec(a.ctx,
		a.info.ExecPath,
		[]string{"info", p.Name, "--json=v2"},
		nil)
	if err != nil || strings.Contains(output, "Error") {
		return "", fmt.Errorf("failed to get package info: %w", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		return "", fmt.Errorf("failed to parse brew info output: %w", err)
	}

	var key string
	if p.IsGUI {
		key = "casks"
	} else {
		key = "formulae"
	}

	bytes, err := json.Marshal(obj[key].([]any)[0])
	if err != nil {
		return "", fmt.Errorf("failed to marshal package info: %w", err)
	}
	return string(bytes), nil
}

// Search returns the list of packages matching the keyword for brew
func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	res, err := util.RunAllowExitCodes(a.ctx, a.info.ExecPath,
		[]string{"search", keyword},
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"},
		0, 1,
	)
	if err != nil {
		return nil, fmt.Errorf("brew search failed: %w", err)
	}
	if res.Stdout == "" {
		return []manager.Package{}, nil
	}

	entries := parseBrewSearch(res.Stdout)
	if len(entries) == 0 {
		return []manager.Package{}, nil
	}

	// collect all package names into a single brew info call
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	infoRes, err := util.Run(a.ctx, a.info.ExecPath,
		append([]string{"info", "--json=v2"}, names...),
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"},
	)
	if err != nil {
		// info 失败时降级，返回无版本/描述的结果
		slog.WarnContext(a.ctx,
			"brew info failed, returning search results without details",
			"keyword", keyword,
			"error", err,
		)
		return buildSearchResults(entries, nil), nil
	}

	info := parseBrewInfo(infoRes.Stdout)
	return buildSearchResults(entries, info), nil
}

// searchEntry is an intermediate result from parsing brew search output
type searchEntry struct {
	Name   string
	IsCask bool
}

// parseBrewSearch parses the flat list output from `brew search <keyword>`.
// An empty line separates formulae (first group) from casks (second group).
func parseBrewSearch(output string) []searchEntry {
	lines := strings.Split(output, "\n")
	entries := make([]searchEntry, 0, len(lines))
	isCask := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			isCask = true
			continue
		}
		entries = append(entries, searchEntry{Name: line, IsCask: isCask})
	}
	return entries
}

// pkgDetail holds version and description looked up from brew info
type pkgDetail struct {
	version string
	desc    string
}

// pkgLookup maps a search-result name to its detail from brew info
type pkgLookup map[string]pkgDetail

// parseBrewInfo parses the JSON output from `brew info --json=v2 [names...]`
// and builds a lookup map keyed by formula name / cask token.
func parseBrewInfo(output string) pkgLookup {
	var obj brewInfoOutput
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		return nil
	}
	m := make(pkgLookup, len(obj.Formulae)+len(obj.Casks))
	for _, f := range obj.Formulae {
		m[f.Name] = pkgDetail{version: f.Versions.Stable, desc: f.Desc}
	}
	for _, c := range obj.Casks {
		m[c.Token] = pkgDetail{version: c.Version, desc: c.Desc}
	}
	return m
}

// buildSearchResults merges search entries with info details into manager.Package slice.
func buildSearchResults(entries []searchEntry, info pkgLookup) []manager.Package {
	pkgs := make([]manager.Package, 0, len(entries))
	for _, e := range entries {
		pkg := manager.Package{
			Name:    e.Name,
			Manager: "brew",
			IsGUI:   e.IsCask,
		}
		if info != nil {
			if d, ok := info[e.Name]; ok {
				pkg.Version = d.version
				pkg.Description = d.desc
			}
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// Install installs the specified packages using brew
func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return manager.ErrorResult("Install not implemented for brew"), nil
}

// Uninstall uninstalls the specified packages using brew
func (a *Adapter) Uninstall(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	formulas := make([]string, 0, len(pkgs))
	casks := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.IsGUI {
			casks = append(casks, pkg.Name)
		} else {
			formulas = append(formulas, pkg.Name)
		}
	}

	if len(formulas) > 0 {
		args := append([]string{"uninstall"}, formulas...)
		if result, err := a.runAction(args, "Uninstalled Homebrew formula(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	if len(casks) > 0 {
		args := append([]string{"uninstall", "--cask"}, casks...)
		if result, err := a.runAction(args, "Uninstalled Homebrew cask(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Uninstalled Homebrew package(s)"), nil
}

// Update updates the specified packages using brew
func (a *Adapter) Update(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return manager.ErrorResult("Update not implemented for brew"), nil
}

// ListVersions NOT available for Homebrew
func (a *Adapter) ListVersions(_ manager.Package) ([]string, error) {
	return nil, fmt.Errorf("ListVersions not available for Homebrew")
}

func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, args, nil)
	if err != nil {
		msg := res.Output()
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}
