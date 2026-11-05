# TODO: what the settings page should show and offer

Scope agreed for this batch: everything in the settings audit except the
console's width memory (P2) and the P3 list. Frontend-only items first, then the
ones that need the backend. The audit and its reasoning are in the branch's
conversation; this file is the worklist and goes away when the batch lands.

Branch: `feature/settings`.

## Phase A — frontend only

- [x] **Manager row: what the manager holds.** A muted `· 27 packages` beside the
      name, counted from `Data.Rows` and rebuilt only when `Data.Version` changes
      (the pattern `list.managerCounts` already uses).
- [x] **Manager row: what it cannot do.** A muted line naming the capabilities the
      manager does not advertise, in the capability matrix's order, and nothing at
      all when it advertises every one: the row should say what is missing rather
      than list what is there.
- [x] **Manager row layout.** Name, stats, state and switch on the first line, the
      executable path and its Apply on a second line of its own, the unsupported
      capabilities under them. The path field was 150 DIPs narrower than this and
      now has the row's full width.
- [x] Tests in `views/settings`: the count follows the inventory, the unsupported
      line names exactly the missing ones and is absent when nothing is missing.

## Phase B — backend, in this order

- [ ] **Manager version.** `manager.Info.Version`, captured where preflight
      already runs `--version`: `util.CheckCommandVersion` throws the output away
      today. A `util.CommandVersion` returns the first line and a generic reader
      takes the first version-looking token, because the seven outputs differ
      (`Homebrew 7.0.12-105-g3cea970`, `11.17.0`, `uv 0.12.17 (…)`,
      `pip 26.2.1 from …`). Shown on the row's stats line; a reused preflight
      copies it too.
- [x] **Appearance.** A new `theme` config value (`system`, `light`, `dark`),
      applied with `mygo.Theme.SetSource` before the window opens and on change.
      Onevra's palette already follows the frame's dark state, so the page needs
      no other work.
- [ ] **Notifications.** A `notifications` config value (nil = on) read before
      announcing a finished task, with a switch in General.
- [ ] **About.** App version (a constant in package main, pinned by a test against
      `mygo.json` the way `TestAppDirNameIsTheBundleIdentifier` pins the
      identifier), the MyGo and Onevra versions from the build's module info, and
      the data and log directories.
- [ ] **Open the log and the data directory.** `mygo.Shell.OpenPath` on paths the
      backend hands the page: `logging.AppLogDir` and `util.AppRoot`, through `App`
      methods since a page may not import backend packages.
- [ ] **Clear the task history.** The store keeps the newest 500 tasks and every
      console line with them; a maintenance action deletes them behind the same
      hold-to-confirm gesture as uninstalling, and the tasks page reloads.

## Phase C — not scheduled

- [ ] **Install roots.** Where each manager puts its packages, as a row line: a new
      optional adapter method, because the answers are uneven — `brew --prefix`,
      `npm prefix -g`, `uv tool dir` are exact, `pipx` and `cargo` only have a
      default path, and `pip` writes into a site-packages directory that depends on
      the interpreter. `bun pm bin -g` gives the bin directory, not the install
      root. Confirm the shape before building it.

## Verification

```sh
gofmt -l .
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

Layout and the two new selects need a desktop session to judge; headless tests
prove the wiring, not how it looks.
