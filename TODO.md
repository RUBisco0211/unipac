# TODO

Migration from Wails + Vue to MyGo native UI + Onevra.

Decisions, boundaries and the target layout live in
[.agents/ARCHITECTURE.md](.agents/ARCHITECTURE.md). This file tracks progress
only, so it must not restate architecture rules.

**Rule: tick an item in the same change that finishes it.** A step counts as
done only when its `Verify:` line passes. Anything discovered mid-work that
needs a decision goes under [Open](#open) instead of being silently absorbed.

## Current

Slices 1–7 are done. What remains is the unreviewed list under [Open](#open): a
human look at the window, a real package action, and packaging.

## Slice 1 — read-only package list

- [x] **1.1 Module rename and toolchain**
      Module `github.com/RUBisco0211/unipac`, `go 1.27.3`, MyGo v0.7.4, Onevra
      v0.2026.16, and the MyGo CLI as a Go tool.
      Also swapped `mattn/go-sqlite3` for `modernc.org/sqlite`: the cgo driver
      compiles under `CGO_ENABLED=0` but registers a stub that fails at runtime,
      which would have hidden the breakage until first launch.
      Verify: `go build ./...`, `gofmt -l .` empty, and
      `CGO_ENABLED=0 go test ./backend/cache/` round-trips through the driver.

- [x] **1.2 Extract `App` into an importable package**
      `app/` holds the service. `Startup(ctx) error` returns the failing stage
      instead of showing a dialog, so the entry point decides how to report it.
      Verify: `go build ./...`, `go vet ./...`.

- [x] **1.3 MyGo entry point and `mygo.json`**
      `main.go` with `mygo.App.WhenReady`; `mygo.json` carries name,
      identifier, icon, output directory and the i18n catalogs.
      Verify: the built binary opens a window, initialises config, logging,
      cache and the registry, and exits cleanly.

- [x] **1.4 App icon**
      `assets/icon.png`, 1024 × 1024.

- [x] **1.5 Shell**
      `sidebar.NewShell`, four navigation entries in two sections,
      `StateKey: "main"`, `Theme: theme.Onevra`.
      Verify: `TestSidebarSwitchesPage` clicks a navigation item headlessly and
      observes the page change.

- [x] **1.6 Packages view**
      `TableVirtualized` over name, installed version, latest version and
      manager, with header sorting.
      Verify: `TestListViewShowsRows`, plus a real launch that filled the cache
      with 218–229 packages the same builder renders.

- [x] **1.7 Catalogs**
      `locales/en.json`, `locales/zh-CN.json`, and the committed
      `locales/catalog_gen.go`.
      Verify: `go test ./...` passes without the CLI, and `views/` contains no
      user-visible string literals.

- [x] **1.8 Slice check**
      Verify: `gofmt -l .` empty, `CGO_ENABLED=0 go test ./...` passes,
      `go tool mygo vet .` clean, and a launch on a fresh home directory created
      its logs, its database and 229 cache rows.

## Slice 2 — background refresh

- [x] Startup refresh moved into a guarded job (`UI.Refresh`); a request while
      one is in flight is dropped.
- [x] Rows are published through `Window.Update`; builders never touch the
      backend after `views.New`.
- [x] The window stays responsive during a slow scan.
      Verify: a fresh-home launch rendered before the scan finished and was
      still usable while Homebrew listed packages.

## Slice 3 — remaining pages

- [x] Outdated page: the same `listView` with `outdatedOnly`, its own manager
      filter, sorting and update action.
- [x] Settings page: manager availability, executable path and capabilities.
      Verify: `TestOutdatedViewFiltersRows`, `TestOutdatedEmptyState`,
      `TestSettingsShowsManagers`; the Settings page lists the disabled Cargo
      manager on this machine.

## Slice 4 — search

- [x] Search page wired to the real `App.SearchPackages`, not a placeholder: a
      keyword field, Enter or the button to submit, results in the shared table,
      and an install action for the selected result.
      Verify: `TestSearchViewHintBeforeSearch`, `TestSearchViewRendersResults`.

## Slice 5 — actions

- [x] `App` methods for install, uninstall, update and version lookup already
      existed and route through the registry's capability checks.
- [x] Disabled actions stay visible, with a tooltip naming the reason: the
      manager does not support it, the package is already current, or another
      action is running. The reason hangs on a wrapper, because MyGo shows no
      tooltip on a disabled element.
- [x] Uninstall opens a dialog per action whose confirm is `HoldToConfirm`; batch
      uninstall opens one dialog for every target.
      Verify: `TestUpdateActionRespectsCapability` proves a button without the
      capability does nothing; `TestUpdateActionFires` proves a supported one
      reaches the selected package. A real uninstall was not run, because it
      would remove software from this machine.

## Slice 6 — cutover

- [x] `main.go` at the repository root; `cmd/unipac` removed.
- [x] `frontend/`, `frontend/README.md`, `wails.json` and `build/` deleted.
- [x] Wails dropped from `go.mod`, and `backend/util/dialog.go` deleted with it.
- [x] Refresh `assets/screenshot.png`.
      Rendered from the real builders by `TestScreenshot`, because this session
      cannot capture the screen. It shows the content and sidebar without the
      native title bar.
      Verify: `go tool mygo build` produced a signed `UniPac.app` and a
      `unipac-0.1.0-arm64.dmg`, and the packaged app launched and refreshed its
      cache.

## Slice 7 — interface parity

The first pass reproduced behaviour but invented its own layout. This slice
rebuilds the pages against the old Vue interface, which is the specification,
and then against the review notes on the native window.

- [x] Multi-select: row checkboxes, select-all, selected-row highlighting, and a
      selection that drops rows hidden by a filter.
- [x] Bulk bar: `Update selected` and `Remove selected`, shown only with a
      selection.
- [x] Operations column: per-row update, greyed out unless the package is
      outdated, and a per-row remove.
- [x] Type column (`CLI` / `GUI`) and manager badges tinted by a hash of the
      manager ID, reusing the old hue hash.
- [x] Cell hierarchy: package icon, name, muted description; tag icon for the
      installed version; branch icon for the latest, accented when outdated.
- [x] Title bar: back and forward over a page history, with the page title
      left-aligned beside them. They live in the title bar rather than the
      sidebar header because the shell hides that header when the sidebar
      collapses, and the arrows must stay.
- [x] Sidebar: starts below the title bar, keeps a 220–320 DIP width range, and
      blurs what is under it with `glass.Blur` plus a surface tint.
      Known limit: the backdrop behind the sidebar is opaque on pages with a
      table and translucent elsewhere, measured as alpha `ff` against `8c` in
      `CapturePage` output, so the blur only reads on the pages without a table.
      The tint is at 0.5, favouring the blur over a perfectly even surface.
- [x] Clicking a row does nothing: selection lives in its own set and the
      table's `OnChange` restores it, so only the checkbox and select-all
      change it.
- [x] The selection count reads `selected / total` and sits in the Package
      column's header, after the column name, only while rows are selected.
      The header only takes text, so it is drawn over the header cell, placed by
      measuring the column title, and centred on the header line with
      `AlignSelf` because a Badge pins itself to the cross-start.
- [x] A thin indeterminate bar sweeps above the table header while the
      inventory loads or reloads. It is drawn absolutely over a slot that is
      always reserved, so nothing moves when a refresh starts, and it shows on
      the empty state too. `TestLoadingBarPaints` covers the drawing.
- [x] A refresh keeps the cached rows on screen; only the bar says a scan is
      running. Skeleton rows were tried and dropped: replacing the table on every
      refresh hid the data the window had just opened on.
- [x] The empty state is a centred package icon over its message.
      `TestEmptyStateIsCentredWithText` and `TestRefreshKeepsCachedRows` cover
      both.
- [x] The Outdated navigation entry uses `lucide.CircleFadingArrowUp`.
- [x] Table spans the main area edge to edge while the toolbar and chips keep
      their inset; rows are square so selected rows meet without notches.
- [x] Column headers no longer drag or resize.
- [x] Select-all lives in the table header's first column, drawn over the header
      cell. Both it and the row checkboxes sit 6 DIPs past the cell padding so
      they line up with the toolbar's inset; the table keeps its default
      padding.
- [x] Package names and versions show a text tooltip when truncated.
- [x] Toolbar actions are icon buttons with tooltips, matching the title bar.
- [x] Uninstall opens a dialog and the dialog's confirm is the hold gesture; the
      row itself only offers an icon button.
- [x] The outdated page offers update only, in the row and in the bulk bar.
- [x] Settings lists managers as cards with availability, executable path and
      capability badges.
- [x] Window minimum is 1024 × 620 so the toolbar never clips.
      Verify: `TestListViewShowsRows`, `TestListViewQueryFilter`,
      `TestSelectionTracksVisibleRowsOnly`, `TestBulkBarAppearsWithSelection`,
      `TestOutdatedPageHidesUninstall`, `TestUpdateActionRespectsOutdatedFlag`,
      plus captures of the running window on every page.

Deviations from the old interface, kept on purpose:

- Onevra has no destructive button variant, so `Remove selected` is a bordered
  button in the danger color rather than a filled red one.
- The table header cannot host a widget, so select-all and the selection count
  are drawn over the header cell rather than being part of it.
- The row radius is flattened by building the table with a square theme and
  restoring the real theme inside each cell; Onevra paints the radius on the row
  box it creates, so there is no per-row option.

## Open

- **Visual review.** Both themes were reviewed through renders from the real
  builders, but nobody has looked at the actual window: header insets, the
  floating title bar, narrow windows, keyboard navigation and the Chinese
  catalog still need a desktop pass.
- **A real package action.** Update and uninstall are wired but never executed
  against a live manager.
- **Packaging.** `mygo build`, code signing, notarization and auto-update are
  untouched; `go tool mygo dev` is the only run mode exercised.
- **Identifier.** `com.unipac.app` is a placeholder. Confirm before the first
  release, because systems key preferences, permissions and registrations on it.
- **Version selection.** No page offers `ListVersions`, even for managers that
  advertise it.
