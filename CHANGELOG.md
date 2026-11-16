# Changelog

One section per released version, and nothing else: a version's section is its
GitHub release's notes, which the update window shows, and `mygo build` reads it
when it publishes an update, so a version without a section cannot be released.
It is written when the version is cut, and it lists what that version ships.

## 0.4.2 — 2026-11-16

- The tasks page's console is a dock along the bottom edge, so the table has the
  window's whole width and a log line no longer wraps in a narrow column. The
  dock folds to its header, and copies the command a task ran or the output it
  holds.
- Small UI adjustments: dialogs sit in the middle of the window, a package's page
  can be selected and copied, the bulk actions stay visible without a selection,
  and an update's two versions get a line each.

## 0.4.1 — 2026-11-13

- Settings › About carries the app's own updater, beside the version it changes:
  **Check now**, which opens the update window without waiting for the background
  check, and the two switches that decide whether UniPac checks in the background
  and whether it installs what it finds without asking.
- Both switches were previously reachable only from the update window itself, so
  the choice to install without asking could be turned on and not off.

## 0.4.0 — 2026-11-13

- UniPac updates itself from signed releases. Ten seconds after launch, then once
  a day, it asks the release feed for a newer version, checks the archive against
  the app's own signing key, and offers to install it; **Check for Updates…** in
  the app menu asks at once. An update downloads only what changed since the
  version you run when the release has a delta, and the whole app otherwise.
- One line installs UniPac, and checks the archive before it does:
  `curl -fsSL …/releases/latest/download/install.sh | sh`. `curl` sets no
  quarantine flag, so Gatekeeper does not ask about an app installed this way.
- A page of its own for every package, opened from any of the three tables: its
  description, its links, and the versions its manager lists.
- Descriptions in the tables, with a placeholder while one is read, and links in
  place.
- One proxy setting, reaching every manager's tool through the variable names
  each one reads, and offering the desktop's own proxy when System Settings has
  one.
- Settings became one Onevra surface in a window of its own, and a write queues
  instead of the page disabling itself while one runs.
- Notifications say which operation ended and how, and ask the system for
  permission rather than offering switches it cannot honor.
- The macOS 26 dark and tinted icon.
- bun: an empty global directory is an empty inventory, not a failure.

## 0.3.0 — 2026-11-06

- Installs a package at a version its manager lists, from the search page's
  version dialog.
- Searches are capped per manager and say when a manager had more than it showed;
  Homebrew's details are read in batches, and an installed or outdated result is
  marked as such.
- Every install, update and uninstall is a task with its command, outcome and
  console: cancellable, retriable, filterable by status, and reopened from its
  notification.
- A page for the package managers themselves: each one's version, what it holds,
  what it cannot do, and the executable it runs.
- Settings: appearance, language, log level, scan on launch, hold to confirm, and
  an About section with the folders the app keeps and the history to clear.
- Commands run with the PATH of the user's shell, so opening UniPac from Finder
  no longer makes every manager look missing.
- Everything the app writes lives under its identifier's directory, and what an
  older build left behind is moved there.
- Quitting with unfinished tasks asks first, in the window's own dialog.

## 0.2.0 — 2026-11-03

- Rewritten as a native Go application. The WebView front end of 0.1.0 — Tauri,
  then a Wails rewrite that never shipped — is gone: MyGo draws the windows and
  Onevra supplies the controls, so the app is one binary with no JavaScript and
  no IPC boundary between the interface and the backend.
- Managers are configurable: each one can be turned off, or pointed at another
  executable, and the settings page is built around them.
- Every install, update and uninstall is recorded as a task that outlives the
  call.
- CI builds and publishes a macOS arm64 disk image for each tag.

## 0.1.0 — 2026-03-24 (prerelease)

- The first release: installed and outdated packages from the package managers it
  discovered, in one window, with a search across them.
- The window was a WebView with a Vue 3 front end.
