# Handoff: managing bun's global packages

Status: built on `feature/bun`. Every output quoted below was captured from this
machine's `bun 1.4.2`, and the adapter was then run against the same CLI before
the branch was committed.

## What was added

A `bun` adapter beside `brew`, `npm`, `pip`, `pipx`, `cargo` and `uv-tool`, so the
packages a user installed with `bun add -g` appear in the installed and outdated
lists and can be updated and removed like any other manager's. The views needed no
change: the registry is the source of truth and `App.ListManagers()` is how a page
learns a manager exists.

Three decisions were made while planning, and they bound the adapter:

- **Search is off.** bun has no search command, and bun's packages are npm's
  packages, so a second search over the same registry would show every result
  twice and ask the user which of two identical rows to install with.
- **No probe project.** `bun info` needs a `package.json` in the working
  directory, which the app does not have. A scratch project in the app root was
  rejected, so `GetPackageInfo` and `ListVersions` stay off with it.
- **No way to install a new package yet.** With search off there is no bun row to
  install from. An exact-name lookup and a manager choice on an install row are
  both possible later; neither is built.

What is left for a user: a bun package can be seen, updated and removed, but a
package found in Discover is installed with the manager that found it.

## What bun answers

### Listing the globals: `bun pm ls -g`

```
$ bun pm ls -g
/Users/rubisco/.bun/install/global node_modules (61 installed)
├── @quasar/cli@5.0.9
└── is-odd@3.0.1
```

- The first line names the directory bun considers global. **Do not assume
  `~/.bun/install/global`**: on this machine one run installed into `$HOME`
  instead (`bun pm ls -g` then said `/Users/rubisco node_modules`), so the adapter
  reads the packages from the lines under that header and never builds the path.
- Without `--all` the tree holds the top-level dependencies only, which is exactly
  the global package set. `--all` adds every transitive dependency (61 lines for
  two packages) and is not used.
- `(61 installed)` counts the transitive tree, so it is not a row count.
- The tree glyphs are UTF-8 box drawing; a name is `name@version`, and a scoped
  name needs the **last** `@` as the split, not the first.
- With nothing installed globally the command **fails**:

```
$ bun pm ls -g
error: missing lockfile, nothing to list
note: run 'bun install' first
$ echo $?
1
```

That is a normal state, not an error to report: the adapter allows exit code 1 and
returns an empty list when the output says `missing lockfile`.

### Outdated: `bun outdated -g`

```
$ bun outdated -g
bun outdated v1.4.2 (744846f84)
|--------------------------------------|
| Package  | Current | Update | Latest |
|----------|---------|--------|--------|
| is-odd   | 2.0.0   | 2.0.0  | 3.0.1  |
|--------------------------------------|
```

- A pipe-delimited table with a separator row before and after each entry, so
  parsing skips rows whose cells are dashes.
- Only top-level globals are listed. `is-number@3.0.0` (a dependency of
  `is-odd@1.0.0`, with 7.0.0 published) never appeared.
- `Update` is the newest within the declared range, `Latest` the newest overall. A
  global install pins an exact version, so `Update` equals `Current`; the
  adapter's `LatestVersion` comes from `Latest`.
- Nothing outdated is also an exit-1 failure with its own message:

```
$ bun outdated -g
bun outdated v1.4.2 (744846f84)
error: missing lockfile, nothing outdated
note: run 'bun install' first
$ echo $?
1
```

### Installing: `bun add -g <spec>...`

```
$ bun add -g is-odd@2.0.0
installed is-odd@2.0.0

2 packages installed [548.00ms]
```

- `name@version` writes an **exact pin** (`"is-odd": "2.0.0"`) into
  `~/.bun/install/global/package.json`; `name` alone writes a caret range
  (`"^3.0.1"`), and so does `name@latest`. The adapter always names the version it
  wants, so a global install means one version, and `name@latest` is the fallback
  when none is known.
- Several specs in one command work (`bun add -g is-odd@2.0.0 is-number@4.0.0`),
  but a batch aborts on the first spec that cannot resolve. The app fans a batch
  out into one task per package, so the adapter is handed one package per call
  today; multi-spec support matches npm's adapter and is not relied on.
- Failure is a non-zero exit with an `error:` line (`404`, "No version matching …
  found for specifier"), which is what the adapter puts in the task's result.
- `-g` targets the global directory from any working directory: verified from `/`,
  from `/tmp` (no project) and from inside a project.

### Updating: `bun update -g <spec>...`

```
$ bun update -g is-odd@3.0.1        # from 2.0.0
Saved lockfile

^ is-odd 2.0.0 -> 3.0.1

2 packages installed [363.00ms]
```

- Without a version (`bun update -g is-odd`) an exactly pinned global is left
  alone: `Checked 4 installs across 5 packages (no changes)`. A version must be
  named, which the row already knows (its `LatestVersion`).
- The package must be in `bun.lock`: `bun update -g lodash` answers
  `error: "lodash" is not in bun.lock`, exit 1. That only happens for a package the
  row says is installed.

### Removing: `bun remove -g <name>...`

```
$ bun remove -g @quasar/cli
- @quasar/cli
1 package removed [4.00ms]
```

Removing the last global leaves `package.json` as `{}`; removing anything else
then answers `error: package.json is empty {}, so there's nothing to remove!`,
exit 1.

### What bun cannot do

- **Search: none.** (`bun search` answers `error: Script not found "search"`.)
- **Registry lookups need a project.** `bun info <name> --json`, `bun pm view
  <name>` and even the versions list fail without a `package.json` in the working
  directory:

```
$ cd /tmp && bun info is-odd --json
error: Bun could not find a package.json file to install from
```

  `--cwd` is not a flag of `info` (`bun info is-odd --cwd /tmp/bunprobe --json`
  answers `{"error": "Property not found", …}`), and `util.Run` runs commands in
  the app's own working directory.

## Capability matrix for the bun adapter

| Capability | bun | Why |
| --- | --- | --- |
| ListInstalled | yes | `bun pm ls -g`, top-level tree |
| ListOutdated | yes | `bun outdated -g`, `Latest` column |
| Install | yes | `bun add -g name@version` |
| Update | yes | `bun update -g name@version` |
| Uninstall | yes | `bun remove -g name` |
| Search | no | no such command; and it would duplicate every npm result |
| GetPackageInfo | no | `bun info` needs a project directory; no page asks for it today |
| ListVersions | no | same project requirement; and the picker only opens on install rows |

The three actions are advertised even though no page reaches `Install` today,
because they describe what the adapter can do — and a retried task or a future
entry point must not find them missing.

## What exists

| File | Change |
| --- | --- |
| `source/backend/adapters/bun/bun.go` | the adapter |
| `source/backend/adapters/bun/bun_test.go` | the parsers, the spec builders, the capability pins |
| `source/backend/adapters/index.go` | `bun.NewAdapter` registered |
| `.agents/PACKAGE_MANAGER_ADAPTERS.md` | capability matrix row, a bun section, the limits |
| `README.md`, `AGENTS.md` | bun in the manager lists |

Shape is npm's and uvtool's: `ctx`/`info` fields, `NewAdapter`, `Info()`,
`WithContext`, `Preflight`, `Setup`, one `runAction` for the three actions. The
scan commands run without color and strip ANSI anyway; the three actions pass
`FORCE_COLOR=1`, because their output is the console the user reads and the app
renders SGR.

The pieces a test can hold are pure functions: `parseGlobalTree`, `parseTreeLine`,
`parseOutdatedTable`, `globalsState` (empty inventory versus failure),
`packageSpecs`, `updateSpecs`, `packageNames`.

## Verified against the real CLI

Beyond `go test`, the adapter itself was run against this machine's bun:

- `ListInstalled` returned `@quasar/cli 5.0.9` and `is-odd 2.0.0`.
- `ListOutdated` returned `is-odd 2.0.0 → 3.0.1`.
- `Update` with `LatestVersion: 3.0.1` reported success, and `ListInstalled`
  afterwards showed `is-odd 3.0.1` — the version the adapter named is the version
  that landed.
- `Uninstall` on the listed packages reported success, after which both
  `ListInstalled` and `ListOutdated` returned no packages and no error, which is
  the `missing lockfile` path.
- `~/.bun/install/global` was left as it was found (empty), and the test packages
  are gone from the machine.

## Still open

- **Installing a new package with bun.** Two shapes were considered and are not
  built: an exact-name lookup (search asks `bun info <keyword>` for the typed name,
  so a bun row appears for an exact match) and a manager choice on an install row,
  so a package found by npm can be installed with bun. Both need a place for
  `bun info` to run: a `package.json` under `util.AppRoot`, plus a working-directory
  argument for `util.Run`. The first would also turn on `GetPackageInfo` and
  `ListVersions`.
- **`missing lockfile` is matched as a phrase.** It is how bun reports an empty
  global install in two commands, and a bun release that rewords it would turn an
  empty inventory into an error in the log. One constant, two tests, and the
  comment says why.
- **`bun link` registration.** Not tested: the adapter reports what the global
  lockfile says, which is what bun itself reports.
