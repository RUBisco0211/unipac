# Package Manager Adapter Implementation Guide

This document defines the backend capability boundaries and CLI implementation plan for package-manager adapters in `unipac`: `npm`, `pip`, `pipx`, `cargo`, and `uv tool`.

Use this guide together with `.agents/ARCHITECTURE.md`. The architecture document remains the source of truth for module boundaries and dependency direction.

## Scope

UniPac is a desktop package inventory and action tool. These adapters should manage user/global package-manager state, not project-local dependency graphs.

- `npm` manages global npm packages installed with `npm -g`.
- `pip` manages packages in the Python environment addressed by the configured `pip` executable.
- `pipx` manages applications installed and isolated by `pipx`.
- `cargo` manages binaries installed with `cargo install`.
- `uv-tool` manages Python command-line tools installed with `uv tool`.

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
10. Before implementing CLI details, verify current command syntax against the manager's own `--help` output or documentation rather than relying on memory.

## Backend API Shape

Expose user workflow methods on the `App` service in `app.go`, backed by registry routing methods. Keep adapter internals hidden from the views.

Recommended registry and `App` workflows:

- `SearchPackages(managerID string, keyword string)`
- `GetPackageInfo(managerID string, name string)`
- `ListPackageVersions(managerID string, name string)`
- `InstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`
- `UninstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`
- `UpdatePackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions)`

Batch operations should be routed by manager. If the frontend sends mixed-manager targets, group them in the backend or require the frontend API wrapper to call the backend once per manager. The registry should not call an adapter with packages owned by another manager.

## Shared Implementation Notes

A search returns at most `manager.MaxSearchResults` (200) packages from each
manager, and the registry cuts a longer answer and reports it as truncated. Each
adapter asks its own tool for that many, so the cap means the same thing
everywhere; `brew search` has no such flag, and the details of a large result are
asked for in batches of 100 instead.

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

### Batch Actions and Concurrency

Some package-manager CLIs support handling multiple packages in a single command, while others only accept one package per invocation for a given action. Adapters should choose the safest native shape:

- Use one command when the CLI explicitly supports multiple targets, such as `uv tool uninstall <name>...`, `uv tool upgrade <name>...`, or `brew upgrade <name...>`.
- Use one command per package when the CLI only accepts a single package spec, such as `uv tool install <package>` or `pipx install <package>`.
- For one-command-per-package actions, controlled concurrency is allowed. Prefer a small worker limit such as `2` or `3`, preserve context cancellation, and stop returning success if any package action fails.
- Keep command argument construction isolated in helper functions so sequential and concurrent execution paths share the same package-spec behavior.
- Aggregate per-package failures into a concise `manager.ErrorResult` message instead of failing silently after the first package when parallel work has already started.
- Do not use shell interpolation for concurrency. Start each command with the shared command runner and direct argv.

For first-pass implementations, sequential execution is acceptable. If the action may be slow and does not mutate shared state in an order-sensitive way, concurrency can be added behind a small helper such as `runPackageActionsConcurrently`.

## Capability Matrix

Use conservative capabilities. Only advertise a capability when the implementation is reliable enough for UI exposure.

| Manager | ListInstalled | ListOutdated | Detail | Search | Install | Uninstall | Update | ListVersions |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| brew | yes | yes | yes | yes | yes | yes | yes | no |
| bun | yes | yes | yes | no | yes | yes | yes | no |
| npm | yes | yes | yes | yes | yes | yes | yes | yes |
| pip | yes | yes | yes | limited | yes | yes | yes | limited |
| pipx | yes | limited | yes | no | yes | yes | yes | no |
| cargo | yes | no | yes | yes | yes | yes | no | no |
| uv-tool | yes | yes | yes | no | yes | yes | yes | no |

`Detail` is what a package's own page shows, read when the page asks rather than
while scanning. Two `no` columns remain below:

bun has no search command, and its packages are npm's, so a second search would
list every result twice; it also cannot answer `bun info` without a
`package.json` in the working directory, which the app does not have — its
package detail therefore comes from the npm registry, over HTTP, which is where
npm's comes from too. cargo cannot list outdated packages or upgrade an
installed binary; its detail comes from crates.io for the same reason it cannot
answer locally: `cargo install --list` prints a name and a version and nothing
else.

### Package detail

`Detail(pkg)` fills `manager.Detail`: name, installed and latest version,
summary, homepage, publisher, license, and where the copy lives. Every field is
optional, and a manager leaves empty what its own source does not say. The
sources, verified against each tool on macOS:

| Manager | Text facts | Latest version | Path |
| --- | --- | --- | --- |
| brew formula | `brew info --json=v2` (local) | `versions.stable` | `brew --prefix` |
| brew cask | `brew info --json=v2` (local); no license in a cask | `version` | `brew --caskroom` |
| npm | npm registry (HTTP) | `dist-tags.latest` | `npm root -g` |
| bun | npm registry (HTTP) | `dist-tags.latest` | `~/.bun/install/global/node_modules` |
| cargo | crates.io (HTTP), owners call for the publisher | `crate.max_version` | `$CARGO_HOME/bin/<binary>` |
| pip | `pip show` (local) | PyPI when the scan did not know it | `Location` from `pip show` |
| pipx | the environment's `dist-info/METADATA` (local) | PyPI when the scan did not know it | `pipx environment --value PIPX_LOCAL_VENVS` |
| uv-tool | the tool environment's `dist-info/METADATA` (local) | PyPI when the scan did not know it | `uv tool dir` |

What no manager answers: brew and cargo give no upstream release date (a
formula's history is its tap's git log), and brew's publisher is the tap that
ships the formula — Homebrew itself for its own taps — because a formula has no
author field.

Detail is also asked for packages that are not installed: the search page opens
the same page for a result. A local source answers when there is one — brew's
`info` reads its tap data for a formula that is not installed — and the python
managers fall back to PyPI when there is no environment or site-packages to read,
so a result shows the project's own words without a copy on the machine.

Homebrew's `ListVersions` is `no` and stays there: `brew info --json=v2` reports
one stable version and a head, and the versions a formula published before live
only in the tap's git history, so there is no list to offer.

`limited` means the operation can be implemented later if the CLI behavior is verified and parsing is stable. First implementations should usually set limited capabilities to `false`.

## bun Adapter

Manage the packages installed with bun's global install. Treat this as a separate
manager from `npm`: bun keeps its own global directory, its own lockfile and its
own package.json, so the same package can be installed through both and the two
are two rows with two versions.

Metadata:

- `ID`: `bun`
- `Name`: `Bun`
- `ExecName`: `bun`
- `Website`: `https://bun.sh`

Capabilities:

- Enable `ListInstalled`, `ListOutdated`, `Install`, `Uninstall`, and `Update`.
- Disable `Search`, `GetPackageInfo`, and `ListVersions`.

CLI plan:

- List installed: `bun pm ls -g`
- List outdated: `bun outdated -g`
- Install: `bun add -g <name>@<version>`
- Uninstall: `bun remove -g <name>...`
- Update: `bun update -g <name>@<version>`

Do not assume the global directory. It is normally `~/.bun/install/global`, but a
run on this project's machine installed into `$HOME` instead; the first line of
`bun pm ls -g` names the directory in use, and the adapter reads the packages from
the lines under it rather than building a path.

### bun Parsing

`bun pm ls -g` prints a directory line, then one tree line per top-level
dependency:

```
/Users/me/.bun/install/global node_modules (61 installed)
├── @quasar/cli@5.0.9
└── is-odd@3.0.1
```

The count in the directory line includes the transitive tree, and `--all` would
print that tree; without it only the top-level dependencies appear, which is the
global package set. Split a name from its version at the **last** `@`: a scoped
name starts with one. Skip nested lines, which are dependencies indented under a
`│` or under spaces.

`bun outdated -g` prints a pipe-delimited table whose separator rows repeat
between entries:

```
bun outdated v1.4.2 (744846f84)
|--------------------------------------|
| Package  | Current | Update | Latest |
|----------|---------|--------|--------|
| is-odd   | 2.0.0   | 2.0.0  | 3.0.1  |
|--------------------------------------|
```

Only top-level globals are listed. `Latest` is the newest version overall and
becomes the row's newer version; `Update` is the newest within the range the
global `package.json` declares, and a global install pins an exact version, so it
never differs from `Current`.

Both listings fail with exit code 1 when bun has never installed a package
globally, and say `missing lockfile`. That is an empty inventory, not a failure:
allow exit code 1, return no packages when the output carries that phrase, and
report anything else as the error. Colour is stripped before parsing, because the
app inherits the user's environment and `FORCE_COLOR` makes bun colourize a pipe.

Naming the version is part of the command. `bun add -g <name>` writes a caret
range into the global package.json, where `name@version` writes the exact version
the user asked for; and `bun update -g <name>` alone answers "no changes" for an
exactly pinned package, so an update always names the version, falling back to
`<name>@latest`.

Bun has no search command, and `bun info` needs a project directory, which is why
those capabilities are off. Turning them on means giving the app a place for a
scratch `package.json` and `util.Run` a working-directory argument. The open
question of whether installing a package found by npm should be possible through
bun at all is deliberately undecided.

## npm Adapter

Only manage globally installed npm packages.

Metadata:

- `ID`: `npm`
- `Name`: `NPM`
- `ExecName`: `npm`
- `Website`: `https://www.npmjs.com`

Capabilities: enable all current adapter capabilities.

CLI plan:

- List installed: `npm ls -g --depth=0 --json`
- List outdated: `npm outdated -g --json`
- Info: `npm view <name> --json`
- Search: `npm search <keyword> --json --searchlimit 200`
- Install: `npm install -g <name>` or `npm install -g <name>@<version>`
- Uninstall: `npm uninstall -g <name>`
- Update: `npm install -g <name>@<version>...`
- Versions: `npm view <name> versions --json`

Normalize installed packages from top-level `dependencies` entries. `npm outdated` may return a non-zero exit code when outdated packages exist, so the command helper must still parse stdout when present.

## pip Adapter

Manage packages for the Python environment addressed by `ExecName`. The current skeleton uses `pip3`; keep that unless a future config feature lets users choose the pip executable.

Metadata:

- `ID`: `pip`
- `Name`: `Pip`
- `ExecName`: `pip3`
- `Website`: `https://pip.pypa.io`

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
- `Website`: `https://pipx.pypa.io`

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
- `Website`: `https://doc.rust-lang.org/cargo`

First-pass capabilities:

- Enable `ListInstalled`, `Search`, `Install`, and `Uninstall`.
- Disable `ListOutdated`, `GetPackageInfo`, `Update`, and `ListVersions` initially.

CLI plan:

- List installed: `cargo install --list`
- Search: `cargo search <keyword> --limit 200`
- Install: `cargo install <name>` or `cargo install <name> --version <version>`
- Uninstall: `cargo uninstall <name>`

Parse installed package header lines like `crate-name v1.2.3:`. Ignore following binary lines in the first implementation. Cargo search output is line-based, so parse cautiously and skip malformed lines rather than failing the whole search.

Cargo does not have a built-in universal update command for all `cargo install` packages. A future implementation may integrate an optional external tool such as `cargo-install-update`, but that must be treated as a separate dependency and preflighted explicitly.

## UV Tool Adapter

Manage Python command-line tools installed by `uv tool`. Treat this as a separate manager from `pip` and `pipx`; it owns uv's tool environments, not arbitrary Python packages in the active interpreter.

Metadata:

- `ID`: `uv-tool`
- `Name`: `UV Tool`
- `ExecName`: `uv`
- `Website`: `https://docs.astral.sh/uv`

First-pass capabilities:

- Enable `ListInstalled`, `ListOutdated`, `GetPackageInfo`, `Install`, `Uninstall`, and `Update`.
- Disable `Search` and `ListVersions`.

CLI plan:

- List installed: `uv tool list --show-version-specifiers --show-paths --show-python`
- List outdated: `uv tool list --outdated`
- Info: use the parsed local tool record from `uv tool list --show-version-specifiers --show-paths --show-python`
- Install: `uv tool install <name>` or `uv tool install <name>==<version>`
- Uninstall: `uv tool uninstall <name>...`
- Update: `uv tool upgrade <name>...`
- Tool directory: `uv tool dir`
- Tool executable directory: `uv tool dir --bin`

Do not expose search. `uv tool` installs packages from package indexes but does not provide a search subcommand.

Do not expose version lookup in the first implementation. `uv tool` does not provide a stable version-enumeration subcommand. A future implementation may add PyPI-backed version lookup, but that should be modeled as package-index metadata and verified separately.

### uv tool Parsing

`uv tool list` currently produces text output rather than a documented JSON format, so keep parsing isolated and well tested.

Recommended parser behavior:

- Return an empty package slice for `No tools installed`.
- Parse the tool package name and installed version from the top-level tool line.
- Parse optional detail lines for version specifier, Python version, tool environment path, and executable paths.
- Skip malformed detail lines where possible instead of failing the whole list.
- Keep richer local details in a small adapter-specific struct, then normalize to `manager.Package`.

Recommended normalized package fields:

- `Name`: uv tool package name.
- `Version`: installed package version when parseable.
- `Manager`: `uv-tool`.
- `Installed`: `true` for installed list results.
- `Outdated`: `true` for outdated list results.
- `LatestVersion`: latest package version from `uv tool list --outdated`, when parseable.
- `Fullname`: original install spec when available.

`GetPackageInfo` should return a JSON string synthesized from the local parsed record, for example:

```json
{
  "name": "ruff",
  "version": "0.8.0",
  "version_specifier": "ruff==0.8.0",
  "python": "CPython 3.12.7",
  "tool_path": "...",
  "executables": ["ruff"]
}
```

### uv tool Actions

Install accepts exactly one package spec per command. Implement either sequential execution or controlled concurrency:

- Build specs as `<name>`, `<name>==<opt.version>`, or `<name>==<pkg.version>` when installing a search result with a specific version.
- Run `uv tool install <spec>` once per package.
- If concurrency is used, cap workers to a small value and aggregate failures.

Uninstall and update support multiple names in one command:

- `uv tool uninstall <name>...`
- `uv tool upgrade <name>...`

Use direct argv and avoid shell interpolation. Do not set `UV_TOOL_DIR` in production code, because that would redirect the user's real uv tool inventory. Tests may set `UV_TOOL_DIR` and `UV_CACHE_DIR` to temporary directories.

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
8. Expose them as `App` methods in `app.go`.
9. Call them from the views through guarded background jobs, without adding manager-specific UI logic.

## Testing Guidance

Prefer parser tests over tests that require the host machine to have every package manager installed.

Recommended test shape:

- Put command-output parsing into small pure helper functions per adapter.
- Test normal output, empty output, and malformed-but-skippable lines.
- Test action argument construction separately from actual command execution if command helpers become injectable.
- Keep live CLI tests optional or behind environment flags.

The cache refresh path depends on `ListInstalled`; parser bugs there can make startup look broken. Treat installed-list parsing as the highest-priority test surface.
