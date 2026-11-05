# Version selection: what was built

Status: done, for installing. The behavior is documented in
[ARCHITECTURE.md](ARCHITECTURE.md) under "Version Selection"; this note keeps the
plan's decisions and the two places the implementation settled differently, so
nobody re-opens them.

## What shipped

- `inventory.Service` carries `ListPackageVersions(managerID, name)`, which
  `*App` already had, so the pages can call it.
- The search page's install button opens a dialog for a manager that advertises
  `Capabilities.ListVersions`: the versions newest first, a field filtering them
  in the page alone, and a confirm that installs the chosen one through
  `OnInstallVersion` → `InstallPackage(pkg, manager.ActionOptions{Version: v})`.
  The task records that version, and the tasks page shows `→ <version>`.
- The fetch is `UI.Versions`, a job like `Search`: off the UI thread, answered
  through `Window.Update` with the sequence number the page registered. The
  page keeps one lookup per package for its lifetime, drops an answer a newer
  lookup superseded, and keeps the builders free of I/O.
- A failed lookup says so in the dialog and leaves the package installable: the
  confirm becomes `Install latest`.
- npm's list is sorted in the page rather than trusted from the adapter, and
  capped at 100 with a line saying so. Filters apply to the whole answer, so a
  search still reaches a version the cap hid.
- Tests: `views/search/versions_test.go` (the control appears only with the
  capability, the order, the filter, the cap, a stale answer, a failed lookup,
  reuse of an answer in hand), `views/shell_test.go` (the page is wired to the
  shell), `backend/adapters/npm/npm_test.go` (`parseVersions` reads both shapes
  npm prints, in npm's order).

## Where it settled differently from the plan

- **A dialog, not a caret and a `components.Select`.** The request was an install
  button that opens a list, chooses a version, and confirms. A `Select` keeps its
  open state inside MyGo, so the page could not tell when to start the fetch
  without reading a control's internals; a `Dialog` keeps the open state, the
  query and the choice in the page, where the sequence numbers already are.
- **Installing only, not the installed and outdated pages.** Neither list has an
  install action — their row action is update — so a version choice there would
  mean choosing the version an *update* goes to, which is the downgrade question
  this plan left out of scope. A manager without `ListVersions` (every manager
  but npm) keeps the single install button it had.
- **A confirm step.** The dialog chooses and then confirms, instead of a click
  on a version installing it, because an accidental click in a long list of
  similar-looking versions starts a real install.

## Still open

- An update could offer a version too: the adapters accept `opt.Version` for
  updates, but a downgrade is a different conversation, and Homebrew cannot.
- The batch bar stays on "latest": a version means one package, not a selection.
- npm's prereleases sit in the list with the releases, in their semver order.
  Hiding them behind a toggle was not asked for, and a user looking for one would
  find it by typing it.
