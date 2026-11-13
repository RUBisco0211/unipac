# Changelog

The section of a version is the release notes of its GitHub release, which the
update window shows: `mygo build` reads it when it publishes an update, so a
version without a section cannot be released.

## 0.4.0

- UniPac updates itself: ten seconds after launch, then once a day, it asks the
  release feed for a newer version, checks the archive against the app's own
  signing key, and offers to install it. **Check for Updates…** in the app menu
  checks at once, and the release notes you read there are this section.
- Installing downloads only what changed since the version you run when the
  release has a delta, and the whole app otherwise; the new version runs after a
  restart, or at the next launch.
- One line installs UniPac, and checks the archive before it does:
  `curl -fsSL https://github.com/RUBisco0211/unipac/releases/latest/download/install.sh | sh`

## 0.3.0

- One page per package, opened from any of the three tables, with its own
  description, links and version history.
- A proxy setting that reaches every manager's tool, including the desktop's own
  proxy when System Settings has one.
- The macOS 26 dark and tinted icon.
- The window opens on cached rows and refreshes in the background, so a slow
  manager no longer holds up the first frame.

## 0.2.0

- Searches across managers, and installs a package at a version its manager
  lists.
- Every install, update and uninstall is a task with its console, which the
  notification reopens.

## 0.1.0

- The first release: installed and outdated packages from Homebrew, Bun, npm,
  pip, pipx, Cargo and UV Tool in one window.
