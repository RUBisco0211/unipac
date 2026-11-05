# Handoff: choosing the version to install

Status: not started. The backend route exists; the port and the interface do
not. Read `ARCHITECTURE.md` first — this note assumes its vocabulary (`App`, the
`inventory.Service` port, capabilities, tasks, jobs).

## Where it stands

| Layer | State |
| --- | --- |
| `manager.Capabilities.ListVersions` | exists, default off |
| `manager.Adapter.ListVersions(pkg) ([]string, error)` | on the adapter interface; every adapter implements it |
| `Registry.ListPackageVersions(managerID, pkg)` | exists, gated on `cap.ListVersions` |
| `App.ListPackageVersions(managerID, name)` | exists, `app.go` |
| `inventory.Service` | **does not declare it**, so no page can call it |
| Pages | no picker anywhere; every install passes `manager.ActionOptions{}` |

Exactly one manager advertises the capability now:

| Manager | Advertises | What it answers |
| --- | --- | --- |
| npm | yes | `npm view <name> versions --json`: every published version, oldest first |
| brew | **no** (see below) | — |
| pip, pipx, cargo, uv-tool | no | `list versions not available for …` |

### Why Homebrew's capability is off

`brew info --json=v2 <formula>` reports one version, not a list:

```
$ brew info --json=v2 mole | jq '.formulae[0].versions'
{ "stable": "1.69.0", "head": "HEAD", "bottle": true }
```

`versioned_formulae` is a different thing — `python@3.12` is a package of its own,
not an older version of `python`. Homebrew keeps no list of a formula's published
versions; that lives in the tap's git history, and installing one means fetching a
tarball by URL. A picker with one option is worse than none, so the adapter
returns `list versions not available for brew` and does not advertise the
capability (`ListVersions` in `backend/adapters/brew/brew.go`, with a test in
`brew_test.go`).

**If Homebrew ever grows a real one** (`brew info --json=v2 --all-versions`, or a
stable way to list a cask's versions), re-enabling is two changes: drop the error
for a real query, and add `WithListVersions(true)` to the adapter's capabilities.

## Goal

A user can install a version other than the newest, for a manager that
advertises `ListVersions`, without leaving the row they are looking at.

## Acceptance criteria

1. On the search page and the installed list, a row whose manager advertises
   `Capabilities.ListVersions` offers a version choice beside its install action;
   a row whose manager does not (every manager but npm today) looks exactly as it
   does now — the control is absent, not disabled, because there is nothing to
   choose and the reason is the manager, not the package.
2. Choosing a version installs that version: the task records it as the version
   it goes to, and the tasks page shows `→ <version>`.
3. The fetch happens off the UI thread, and a result that arrives after its row
   closed, after another version was chosen, or after a newer fetch for the same
   row started, is dropped.
4. A failed fetch says so in the picker and leaves the row usable; it never
   blocks the plain install button.
5. The builders stay pure: no `exec`, no parsing, no waiting.

## Design

### The port

`views/inventory/inventory.go`, next to `SearchPackages`:

```go
ListPackageVersions(managerID string, name string) ([]string, error)
```

`*App` already satisfies it; nothing in package main changes. A test asserts the
port still matches (`app_test.go` has the shape for this).

### Where the picker lives

The row actions, in `list.View.rowActions` and `search.View.rowAction`. Both
already gate on `s.Data.Capabilities(pkg.Manager)`; the version choice is
`caps.ListVersions`.

Recommended shape: the install button stays, and beside it a caret that opens a
`components.Select[string]` (it is searchable via `SearchField`, which npm needs:
`npm view react versions --json` answers 2,999 entries). Do not reach for
`NativeSelect`: the list can be thousands long and needs filtering.

```go
choice := components.Select(c, &page.version)
choice.Trigger.Children(func() { /* label or caret */ })
choice.Popup(func(panel ui.Element) {
    query := choice.SearchField(&page.versionQuery)
    for _, v := range page.versionsOf(pkg, query) {
        item := choice.Item(v)
        item.Children(func() { components.Text(c, v).SingleLine() })
    }
})
```

### State, and where it lives

Page-local, because it is one row's transient UI:

```go
// versionState is the version picker of one package: what was fetched, what
// the fetch is doing, and the sequence that drops a stale answer.
type versionState struct {
    versions []string // newest first, as shown
    loading  bool
    err      error
    seq      int  // bumped per fetch; an answer for an older seq is dropped
    query    string
}
// key: inventory.Key(pkg)
versionOf map[string]*versionState
```

`npm` answers oldest first; reverse once when the answer arrives, so the newest
is at the top of the popup. Cap what is shown (`maxVersions = 100`) and say when
it was cut, the way search says it: npm's list is long and prereleases dominate
the tail.

### Fetching

The shell owns the job, like `Search` and `loadTaskOutput`:

```go
// shell.go
func (u *UI) Versions(pkg manager.Package) {
    win := u.shell.Window()
    go func() {
        versions, err := u.svc.ListPackageVersions(pkg.Manager, pkg.Name)
        win.Update(func() { u.installed.SetVersions(pkg, versions, err) })
    }()
}
```

The page bumps a sequence when it asks, and `SetVersions` drops an answer whose
sequence is not the current one. One fetch per package while it is open; a second
open reuses what arrived unless it failed.

### Installing a version

Wire a second install callback through the page, so the shell can build the
options:

```go
// list.View / search.View
OnInstallVersion func(manager.Package, string, flight.Origin)
```

`UI.InstallVersion` calls `runTask("install", …, u.svc.InstallPackage(pkg,
manager.ActionOptions{Version: v}))`. `taskVersions` in `app.go` already prefers
`opt.Version` for an install, so the task's target is right without further work,
and the adapters already build `name@version` (`npmPackageSpecs`) or
`name==version` (`pipPackageSpecs`, `uvToolPackageSpecs`) from it.

The success toast stays the same label as a plain install; the task page shows
which version it went to.

### Copy

New catalog keys, in both `en.json` and `zh-CN.json`:

| Key | English | Notes |
| --- | --- | --- |
| `versionsTitle` | "Install a version" | the popover's label |
| `versionsSearch` | "Search versions" | the `SearchField` placeholder |
| `versionsLoading` | "Loading versions…" | |
| `versionsFailed` | "Could not list versions" | the error line; the error itself goes to the log |
| `versionsTruncated` | "Showing the newest {count}" | when the list is cut |
| `versionsEmpty` | "No versions to choose" | |
| `actionsVersion` | "Versions" | the caret's tooltip and accessible name |

## Steps

1. `views/inventory/inventory.go`: add `ListPackageVersions` to `Service`.
   `go build ./...` must still pass — `*App` has it.
2. `views/list`: `versionState` and its `map[string]*versionState`, the fetch
   request (`RequestVersions(pkg, seq int)`), the setter
   (`SetVersions(pkg, versions []string, err error)`), the popup in
   `rowActions`, the `OnInstallVersion` wiring, and the popup's labels.
3. `views/search`: the same, in `rowAction`. Share the state machine if it is
   cheap to: it is a page-local map either way, so a copy is honest.
4. `views/shell.go`: `OnInstallVersion` on each page, `UI.Versions` (the job) and
   `UI.InstallVersion`.
5. `locales`: the keys above, then `go tool mygo generate`.
6. Tests (below).
7. Docs: `ARCHITECTURE.md`'s "Suggested Near-Term Cleanup" loses the version
   selection item and "Resolved" gains what was built and Homebrew's decision;
   `PACKAGE_MANAGER_ADAPTERS.md`'s capability matrix already says npm `yes`, brew
   absent — add Homebrew to the matrix with `no` for ListVersions and the reason.

## Tests

- `views/inventory`: no test needed; the interface is checked by the build and by
  `app_test.go`'s port assertion.
- `views/list`: a row of a manager with `ListVersions` shows the version control;
  a row without it does not; choosing a version calls `OnInstallVersion` with it;
  a stale answer (older sequence) is dropped; a failed fetch shows the error and
  keeps the install button working.
- `views/shell`: `InstallVersion` starts a task whose `NewVersion` is the chosen
  version — assert through a stub service, as `TestToastActionOpensItsTask` does.
- `backend/adapters/npm`: `ListVersions` reverses nothing (that is the page's
  job) and parses both the array and the single-version shapes it already
  handles; assert the shape, not a network answer.
- `backend/adapters/brew`: already covered by `TestVersionLookupIsNotOffered`.

Use `ui.NewTester` for the popup: opening it, typing in the search field and
clicking an item are all reachable headlessly.

## Open questions

- Should the picker also appear for the batch bar? A batch install is per row in
  the UI, but the bulk bar acts on many packages with one action. Recommendation:
  no — a version means one package. Leave the bulk bar on latest.
- Should an installed row's update action offer a version too? The adapters take
  `opt.Version` for updates as well, but a downgrade is a different conversation
  (Homebrew cannot, npm can). Out of scope.
- Should the fetched list be cached after the popup closes? Recommendation: keep
  it for the life of the page; dropping it means a second open pays for the same
  fetch, and the answer changes only as often as the registry.

## Out of scope

- Version selection for managers that do not advertise the capability.
- Downgrades through the update action.
- Making Homebrew able to list versions (that is a Homebrew question, not ours).
