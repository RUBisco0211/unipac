# UniPac Architecture

This document describes the architecture of `unipac` and the direction future agents should follow. `unipac` is the primary project. The old Tauri project is no longer a maintained target and should not be modified.

UniPac is a single Go program. MyGo owns the runtime: windows, menus, dialogs, notifications, i18n, and the GPU-drawn `ui` toolkit. Onevra supplies the themed controls and the window shell. There is no webview, no JavaScript, and no IPC boundary between the UI and the backend.

## History

The app was rebuilt from Wails v2 + Vue 3 into MyGo native UI + Onevra. Nothing
of the old stack remains, and these are the decisions worth not revisiting:

| Area | Now |
| --- | --- |
| Shell | `mygo.json`, `mygo.App`, `sidebar.NewShell`; no `wails.json` and no embedded frontend |
| UI | Go views in `views/`; no Vue, Vite, vue-router, vue-i18n or shadcn-vue |
| Boundary | Views call `App` methods directly; there is no generated binding layer |
| Dialogs | `mygo.Dialog`; the Wails dialog helper is gone |
| Go directive | `go 1.27.3` or newer |
| Module path | `github.com/RUBisco0211/unipac` |
| SQLite driver | `modernc.org/sqlite`, pure Go, so `CGO_ENABLED=0` builds actually run |

## High-Level Shape

- `main.go` configures the app: `mygo.App.WhenReady`, the main window, menus, theme, and shutdown hooks.
- `app/` is the application service. Its exported methods are plain Go calls that views make directly; there is no binding generation step.
- `backend/` owns package-manager discovery, command execution, cache persistence, config, and logging.
- `views/` owns presentation: the shell, one file per page, shared table rendering, and the retained state each page shows.
- `locales/` holds the ARB catalogs and the Go package MyGo generates from them.

`source/` is the Go module root and holds every Go package, `mygo.json` and the
app icon; documentation, the screenshot and the build output stay at the
repository root. Paths in this document are relative to `source/`.

The intended dependency direction is:

`views` → `App` methods → `registry` / `cache` / `adapters` / `util`

Views must not import `backend/adapters`, `backend/cache`, or `backend/core/registry` directly. This replaces the old `src/lib/api.ts` boundary: the rule is unchanged, the mechanism is now a Go package boundary.

## Backend Modules

### App Boundary

`app/app.go` is the public backend API exposed to the views.

Exported methods:

- `Startup(ctx) error`, `Shutdown()`, `Cleanup()`
- `ListManagers() []manager.Info`
- `ListEnabledManagers() []manager.Info`
- `GetCachedPackages() ([]manager.Package, error)`
- `UpdateCache() error`
- `SearchPackages(keyword) ([]manager.Package, error)`
- `GetPackageInfo(managerID, name) (string, error)`
- `ListPackageVersions(managerID, name) ([]string, error)`
- `InstallPackage(managerID, name, opt)`, `UpdatePackage(managerID, name, opt)`
- `UninstallPackage(pkg, opt)`
- `BatchUninstallPackages(pkgs, opt)`, `BatchUpdatePackages(pkgs, opt)`

`Startup` returns the stage that failed instead of reporting it itself, so each
entry point chooses how to tell the user. `Startup` does not scan packages; the
window opens on cached rows and the first refresh runs as a background job.

These methods are the coarse UI-facing operations. New package actions should be
added here first, and the views call them.

`App` methods run on whatever goroutine calls them. Slow operations such as
package scans and installs must be started from a job, never from a builder; see
Background Work.

### Manager Core

`backend/core/manager/` defines shared backend domain types:

- `Info`: manager metadata such as `id`, executable name, display name, enabled state, executable path, and capabilities.
- `Capabilities`: feature flags reported by an adapter.
- `Package`: normalized package data used by the registry, cache, views, and table.
- `Adapter`: interface implemented by package-manager integrations.
- `ActionResult` and `ActionOptions`: common command result and options types.

The backend manager registry is the source of truth for which package managers exist. The views must not maintain a parallel list of managers such as `brew`, `npm`, `pip`, or `cargo`.

### Registry

`backend/core/registry/registry.go` owns adapter registration and orchestration.

Current behavior:

- Builds adapter instances from `backend/adapters.GetAdapterConstructors()`.
- Runs adapter `Preflight()` and marks managers enabled or disabled.
- Returns all manager metadata through `ListManagers()`.
- Collects installed packages from enabled adapters with `ListInstalled` capability.

Important current issue:

- The closure inside `GetInstalledPackages()` captures the loop variable `adp`. The source comment notes this risk. Future work should pin the per-iteration adapter before appending the collector.

The registry should be the only backend component that knows about all adapters as a group. UI-level operations like search/install/update/uninstall should be added as registry methods that route by `manager.ID` and check `Capabilities`.

### Adapters

`backend/adapters/` contains concrete package-manager integrations.

- `index.go` statically registers adapter constructors.
- `brew/` has the most complete implementation for listing installed packages and outdated data.
- `npm`, `pip`, `pipx`, `cargo`, and `uvtool` are partial implementations or stubs.

Adding a package manager should require backend-only changes:

1. Implement `manager.Adapter`.
2. Add the constructor to `backend/adapters/index.go`.
3. Ensure `Info.ID`, `Info.Name`, `ExecName`, and `Capabilities` are correct.
4. Let the views discover it through `App.ListManagers()`.

No hardcoded manager list is required anywhere in the UI.

### Cache

`backend/cache/` stores installed package snapshots in SQLite.

- Default path: `cache/package.db` under `util.AppRoot`.
- Schema table: `installed_package_cache`.
- `List()` returns cached packages.
- `UpdateCache()` currently deletes all rows and inserts the latest scan result.

Current behavior:

- If `UpdateCache()` receives an empty package list, it returns without clearing old cache data, so a failed scan never wipes a good snapshot.
- Startup initializes the cache but does not scan. The window opens on cached rows and the first scan runs as a background job.

### Config and Logging

`backend/config/config.go` loads or creates `unipac.json` under the app root. It writes default cache/log config and snapshots current manager metadata into config.

`backend/logging/logging.go` initializes `slog` to stderr and app log file. It
creates its log directory and returns the real `OpenFile` error.

### Util

`backend/util/` contains shared helpers for:

- App root and safe path creation.
- JSON file read/write.
- Command execution and line collection.
- Parallel collection.
- Dialogs, currently through Wails; these move to `mygo.Dialog`.

Command helpers are backend-only. Views should request operations through `App` methods rather than constructing package-manager commands.

## UI Modules

### Shell, Window, and Theme

`sidebar.NewShell` from `github.com/oneo-me/onevra/source/shells/sidebar` provides the main window: a `Left` sidebar, a `Content` area, a `ContentTitle` header area, and toasts.

- Four sidebar entries map to the four pages: Packages, Outdated, Search, Settings. Use `components.Sidebar` with `components.SidebarItem` inside `Left`.
- Give the window a 1024 × 620 minimum; `sidebar.NewShell` defaults to 820 × 740,
  which is too narrow for the table. Keep the sidebar between 220 and 320 DIPs so
  the title bar controls fit and the table columns never overflow. Set `StateKey`
  so MyGo remembers bounds and maximized state.
- Pass `Theme: theme.Onevra`. Choose light/dark/system with `mygo.Theme.SetSource` before `App.Run` for a saved preference. Components never call `Context.SetTheme`: it repaints the root with the theme's opaque background, which covers the window material behind the sidebar and costs the blur.
- The app defines no native menus yet. The old `open_settings` Wails event and its
  `useWailsEvents.ts` listener are gone; settings is a page in the sidebar.
- Dialogs use `mygo.Dialog` with the window as `Parent`, so macOS shows them as sheets.

### Views

One package `views`, split by responsibility rather than by page:

- `shell.go`: the shell, sidebar navigation, the page switch, the background
  jobs (`Refresh`, `Search`, `Install`, `Update`, `Remove`), and the settings page.
- `data.go`: the inventory shared by the pages, the refresh and action state, and
  capability lookup.
- `list.go`: the installed and outdated tables: query and manager filters,
  sorting, row checkboxes with select-all, the bulk bar, and the per-row
  actions. Uninstall opens a dialog whose confirm is a hold gesture; the
  outdated page omits uninstall entirely.
- `search.go`: the search field, its results table, and the install action.
- `table.go`: the columns and cell rendering shared by all three tables.

The installed and outdated pages are the same `listView` with `outdatedOnly`
set, so a change to the table applies to both.

Rules that replace the Vue ones:

- Each page is a struct holding retained state; its `view(c *ui.Context)` builds the interface from that state on every frame.
- Builders run on the UI thread. No I/O, no command execution, no parsing, no blocking.
- Elements expire at the end of a build pass. Store application state and `ui.Handle`, never an element or its parts.
- Give stateful controls stable identity with `c.Key(...)` before construction.
- Bind directly to persistent model fields. A value recomputed each pass loses the edit before its notice; write derived values back in `OnClick`.
- Keep slow work out of the builder and publish results through `Window.Update`.

Actions are never hidden. A button whose manager lacks the capability stays
visible and disabled, and a tooltip names the reason: the manager does not
support it, the package is already current, or another action is running. MyGo
shows no tooltip on a disabled element, so the reason hangs on the wrapper row
that `actionSlot` builds, not on the button.

### Components

Onevra supplies the controls the pages need: `Sidebar` and `SidebarItem`, `Button`, `IconButton`, `HoldToConfirm`, `Text`, `TextBox`, `Checkbox`, `Switch`, `Select`, `Table`, `TableVirtualized`, `Tabs`, `Dialog`, `AlertDialog`, `Tooltip`, `Toasts`, `Progress`, `Badge`, `Avatar`, and `Icon` with `icons/lucide`.

Use MyGo's `ui` package for layout (`Box`, `Column`, `Row`, `Spacer`, `Grid`, `Overlay`), for text formatting (`ui.Textf`), and for anything Onevra has no equivalent for. Onevra intentionally omits `Divider`, `Meter`, `Form`, `Field`, `Toolbar` and `OutlineTable`; compose dividers from `Box` borders rather than adding a replacement widget.

### Manager Discovery

Views load manager metadata once through `App.ListManagers()` and keep it in retained state:

- `managers`
- `enabledManagers`
- `managerNameMap`
- `hasManager`

This replaces `composables/useManagers.ts`. Manager IDs stay opaque: display the backend-provided `name`, color a badge from the ID hash, and gate actions on capabilities.

The old `frontend/src/lib/format.ts` `managerLabelMap` disappears with the frontend and must not be reintroduced in Go.

### Table and Actions

`views/table.go` replaces `PackagesDataTable.vue` and `packages-columns.ts`. Use `TableVirtualized` so a few thousand rows scroll smoothly; keep rows a fixed height, which rules out variable-height cells. Keep row identity stable across refreshes so selection and scroll position survive an update.

`VersionSelectButton.vue` assumed version lookup exists. The Go equivalent must appear only when the selected package's manager reports `listVersions` and the backend exposes version lookup.

Action availability is capability-driven, and the result is never a disappearing button: an unsupported action stays visible and disabled, with a tooltip naming the missing capability. A user must be able to tell "this manager cannot search" from "the app is broken".

Uninstall opens a dialog, and the dialog's confirm button is a `HoldToConfirm` gesture, so removing a package takes a deliberate press-and-hold after the dialog is open. Batch uninstall opens one dialog for every target and confirms them with the same gesture.

### Localization

Catalogs live in `locales/` as ARB JSON, enabled by `i18n` in `mygo.json`. `mygo dev` and `mygo build` write a typed Go function per message beside the catalogs; commit the generated files so `go build` and `go test` work without the CLI.

- Keys are flat and named by area: `nav_packages`, `packages_empty`, `settings_capabilities`.
- This replaces `frontend/src/i18n/` and vue-i18n. Locale resolution is MyGo's; `go tool mygo dev -lang zh-Hans` tries a language without changing the saved preference.
- Component-owned strings follow the window language from Onevra's catalog and are overridden with `onevra.<key>` messages.
- Keep keys and stored values independent of translated text.

### Background Work

The cache refresh is the main long operation. In Wails it blocked a startup goroutine; in native UI a blocking builder freezes the window.

Target behavior:

1. `App.startup` initializes the app root, registry, config, logging, and cache.
2. The window opens immediately and renders cached packages.
3. A guarded job refreshes the cache off the UI thread and publishes rows through `Window.Update`.
4. A refresh asked for while one is running sets a pending flag instead of being
   dropped, and the job runs once more when it finishes. Nothing is cancelled:
   a scan holds the cache for the whole run, and an action that lands mid-scan
   triggers that follow-up scan so the rows never describe the pre-action state.

Views may rebuild several times for one event. Trigger side effects through handled actions or explicit guarded jobs, not from rendering code.

## Data Flow

Startup:

1. `mygo.App.Run()` starts the runtime and calls the `WhenReady` callback.
2. `App.Startup` initializes the app root.
3. Config is loaded or created.
4. Logging is initialized.
5. Cache is initialized.
6. The registry constructs and preflights all adapters.
7. `sidebar.NewShell` opens the main window on the Packages view.
8. Cached packages render immediately; a background job refreshes the cache.

Installed packages UI:

1. View renders rows from retained state loaded out of the cache.
2. Refresh action starts a job that calls `App.UpdateCache()`.
3. The job publishes the new rows through `Window.Update`.
4. The table filters and renders.

Manager UI:

1. `App.ListManagers()` returns adapter metadata from the registry.
2. Views filter by `enabled` and `capabilities`.

## Design Rules Going Forward

1. `unipac` is the maintained project. Do not modify the old Tauri project unless the user explicitly asks.
2. The views must not know the concrete package-manager set. No hardcoded manager lists, labels, icons, install commands, or special cases in UI code.
3. The backend registry is the package-manager source of truth.
4. Capabilities should drive UI availability. If a manager cannot search, install, update, uninstall, or list versions, the UI should reflect that generically.
5. Views call `App` methods, never adapters, the registry, or the cache.
6. There is no generated binding layer. `App` methods are called directly and `mygo.json` is the only packaging manifest.
7. Adapters should normalize package data into `manager.Package` before returning it.
8. Registry methods should validate manager existence, enabled state, and capabilities before calling adapter methods.
9. Cache should remain an implementation detail behind backend methods; views should not know the database schema.
10. Prefer small `App` methods that map to user workflows, not leaking adapter internals to the UI.
11. Builders stay pure: no I/O, no blocking, no background work.
12. Elements expire at the end of a build pass; application state is retained and passed by pointer.
13. Prefer Onevra controls. Reach for MyGo's styled widgets only where Onevra has no equivalent.
14. All user-visible text goes through `locales/`; no literals in builders.
15. Keep `CGO_ENABLED=0` builds working on every platform.

## Suggested Near-Term Cleanup

- Version selection is not implemented: no page offers `ListVersions` yet, even
  for managers that advertise it.
- `InstallPackage`, `UpdatePackage` and `UninstallPackage` handle one package at
  a time; the batch `App` methods exist but no page calls them.
- Actions report their outcome as a status line. A toast would fit the shell's
  `Toasts` option better once the wording settles.
- `assets/screenshot.png` still shows the old interface.

## Resolved

Kept so nobody re-opens them:

- `manager.ErrorResult` returns `Success: false`.
- `GetInstalledPackages` pins the loop adapter before appending its collector.
- `logging.Init` creates its log directory and returns the real `OpenFile`
  error instead of calling `Close` on a nil `*os.File`.
- An empty scan keeps the previous cache; `backend/cache` covers it.
- `App` methods for search, info, versions and the actions exist and route
  through the registry's capability checks.
- The module path is `github.com/RUBisco0211/unipac` on Go 1.27.3.
