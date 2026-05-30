# Package Manager Adapter Implementation Guide

This document defines the backend capability boundaries and CLI implementation plan for incomplete package-manager adapters in `unipac-wails`: `npm`, `pip`, `pipx`, and `cargo`.

Use this guide together with `.agents/ARCHITECTURE.md`. The architecture document remains the source of truth for module boundaries and dependency direction.

## Scope

UniPac is a desktop package inventory and action tool. These adapters should manage user/global package-manager state, not project-local dependency graphs.

- `npm` manages global npm packages installed with `npm -g`.
- `pip` manages packages in the Python environment addressed by the configured `pip` executable.
- `pipx` manages applications installed and isolated by `pipx`.
- `cargo` manages binaries installed with `cargo install`.

Do not scan project-local dependency files such as `package.json`, `package-lock.json`, `requirements.txt`, `pyproject.toml`, `Cargo.toml`, or `Cargo.lock` as installed system packages.

## Architecture Rules

1. Implement behavior behind backend adapters only.
2. Register adapters through `backend/adapters/index.go`.
3. Do not add frontend hardcoded manager lists, labels, command strings, icons, or special cases.
4. The backend registry must remain the source of truth for manager availability and capabilities.
5. Frontend UI should enable or hide package actions only from backend-provided capabilities.
6. Registry methods must validate manager existence, enabled state, and capability support before calling an adapter.
7. Adapter methods should never `panic`; unsupported operations should return a clear error or an unsuccessful `manager.ActionResult`.
8. Normalize all package-manager outputs into `manager.Package`.
9. Prefer structured CLI output such as JSON whenever the package manager supports it.
10. Before implementing CLI details, use `ctx7` per the repository instructions to verify current command syntax.

## Backend API Shape

Expose user workflow methods at the Wails `App` boundary, backed by registry routing methods. Keep adapter internals hidden from the frontend.

Recommended registry and `App` workflows:

- `SearchPackages(managerID string, keyword string)`
- `GetPackageInfo(managerID string, name string)`
- `ListPackageVersions(managerID string, name string)`
- `InstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`
- `UninstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`
- `UpdatePackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`

Batch operations should be routed by manager. If the frontend sends mixed-manager targets, group them in the backend or require the frontend API wrapper to call the backend once per manager. The registry should not call an adapter with packages owned by another manager.

## Shared Implementation Notes

### Command Execution

The current `util.Exec` uses `cmd.Output()`, which discards stderr and fails on non-zero exits. Some package managers use non-zero exits for useful states, especially outdated checks. Add or extend command helpers before implementing all adapters:

- Return stdout, stderr, and exit code.
- Allow selected non-zero exit codes when stdout is still parseable.
- Preserve context cancellation.
- Avoid shell interpolation; pass executable path and args directly.

### Action Results

Fix `manager.ErrorResult` before wiring action methods. It currently returns `Success: true`; it should return `Success: false`.

Action methods should return:

- `SuccessResult` when the command succeeds.
- `ErrorResult` with stderr or a concise synthesized message when the command fails but the application remains healthy.
- A Go `error` for unexpected execution, parsing, or adapter failures.

### Version Options

Use `manager.ActionOptions` for optional versions and flags. Recommended keys:

- `version`: desired version string.
- `dry_run`: boolean if supported by the manager.
- `verbose`: boolean if supported by the manager.

Adapters should ignore unsupported optional flags rather than leaking frontend conditionals.

## Capability Matrix

Use conservative capabilities. Only advertise a capability when the implementation is reliable enough for UI exposure.

| Manager | ListInstalled | ListOutdated | GetPackageInfo | Search | Install | Uninstall | Update | ListVersions |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| npm | yes | yes | yes | yes | yes | yes | yes | yes |
| pip | yes | yes | yes | limited | yes | yes | yes | limited |
| pipx | yes | limited | yes | no | yes | yes | yes | no |
| cargo | yes | no | limited | yes | yes | yes | no | no |

`limited` means the operation can be implemented later if the CLI behavior is verified and parsing is stable. First implementations should usually set limited capabilities to `false`.

## npm Adapter

Only manage globally installed npm packages.

Metadata:

- `ID`: `npm`
- `Name`: `NPM`
- `ExecName`: `npm`

Capabilities: enable all current adapter capabilities.

CLI plan:

- List installed: `npm ls -g --depth=0 --json`
- List outdated: `npm outdated -g --json`
- Info: `npm view <name> --json`
- Search: `npm search <keyword> --json`
- Install: `npm install -g <name>` or `npm install -g <name>@<version>`
- Uninstall: `npm uninstall -g <name>`
- Update: `npm update -g <name...>`
- Versions: `npm view <name> versions --json`

Normalize installed packages from top-level `dependencies` entries. `npm outdated` may return a non-zero exit code when outdated packages exist, so the command helper must still parse stdout when present.

## pip Adapter

Manage packages for the Python environment addressed by `ExecName`. The current skeleton uses `pip3`; keep that unless a future config feature lets users choose the pip executable.

Metadata:

- `ID`: `pip`
- `Name`: `Pip`
- `ExecName`: `pip3`

First-pass capabilities:

- Enable `ListInstalled`, `ListOutdated`, `GetPackageInfo`, `Install`, `Uninstall`, and `Update`.
- Disable `Search` and `ListVersions` initially.

CLI plan:

- List installed: `pip3 list --format=json`
- List outdated: `pip3 list --outdated --format=json`
- Info: `pip3 show <name>`
- Install: `pip3 install <name>` or `pip3 install <name>==<version>`
- Uninstall: `pip3 uninstall -y <name>`
- Update: `pip3 install --upgrade <name>`

Do not expose search in the first implementation. `pip search` is not reliable for modern PyPI usage. A future version lookup may use `pip3 index versions <name>`, but keep `ListVersions` disabled until output parsing is verified.

## pipx Adapter

Manage pipx-installed applications only. Do not treat all Python packages as pipx packages.

Metadata:

- `ID`: `pipx`
- `Name`: `Pipx`
- `ExecName`: `pipx`

First-pass capabilities:

- Enable `ListInstalled`, `GetPackageInfo`, `Install`, `Uninstall`, and `Update`.
- Disable `ListOutdated`, `Search`, and `ListVersions` initially.

CLI plan:

- List installed: `pipx list --json`
- Info: use the matched package object from `pipx list --json`
- Install: `pipx install <name>` or `pipx install <name>==<version>`
- Uninstall: `pipx uninstall <name>`
- Update: `pipx upgrade <name>`
- Future bulk update: `pipx upgrade-all`

Do not expose search. pipx installs applications from package indexes but is not itself a package search tool.

## cargo Adapter

Manage binaries installed by `cargo install`. Do not inspect project `Cargo.toml` dependencies.

Metadata:

- `ID`: `cargo`
- `Name`: `Cargo`
- `ExecName`: `cargo`

First-pass capabilities:

- Enable `ListInstalled`, `Search`, `Install`, and `Uninstall`.
- Disable `ListOutdated`, `GetPackageInfo`, `Update`, and `ListVersions` initially.

CLI plan:

- List installed: `cargo install --list`
- Search: `cargo search <keyword> --limit 20`
- Install: `cargo install <name>` or `cargo install <name> --version <version>`
- Uninstall: `cargo uninstall <name>`

Parse installed package header lines like `crate-name v1.2.3:`. Ignore following binary lines in the first implementation. Cargo search output is line-based, so parse cautiously and skip malformed lines rather than failing the whole search.

Cargo does not have a built-in universal update command for all `cargo install` packages. A future implementation may integrate an optional external tool such as `cargo-install-update`, but that must be treated as a separate dependency and preflighted explicitly.

## Suggested Implementation Order

1. Fix shared manager correctness issues:
   - `manager.ErrorResult` should return `Success: false`.
   - Registry loop-variable capture in `GetInstalledPackages()` should pin the adapter per iteration.
   - Skeleton adapter methods should return unsupported errors instead of panicking.
2. Add safer command execution helpers that expose stdout, stderr, and exit status.
3. Implement `ListInstalled` for `npm`, `pip`, `pipx`, and `cargo`.
4. Add focused parser tests for each `ListInstalled` output shape.
5. Implement reliable action methods:
   - npm install/uninstall/update
   - pip install/uninstall/update
   - pipx install/uninstall/update
   - cargo install/uninstall
6. Implement search/version/info where capabilities are marked true.
7. Add registry routing methods with capability checks.
8. Add Wails-facing `App` methods.
9. Regenerate Wails bindings.
10. Wire `frontend/src/lib/api.ts` to the new bindings without adding manager-specific frontend logic.

## Testing Guidance

Prefer parser tests over tests that require the host machine to have every package manager installed.

Recommended test shape:

- Put command-output parsing into small pure helper functions per adapter.
- Test normal output, empty output, and malformed-but-skippable lines.
- Test action argument construction separately from actual command execution if command helpers become injectable.
- Keep live CLI tests optional or behind environment flags.

The cache refresh path depends on `ListInstalled`; parser bugs there can make startup look broken. Treat installed-list parsing as the highest-priority test surface.
