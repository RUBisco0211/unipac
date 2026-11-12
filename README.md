
<p align="center">
    <img src="./assets/icon.png" width="128" height="128" alt="UniPac">
</p>

# UniPac

[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/apps/unipac)

Inspired by [Devolutions/UniGetUI](https://github.com/Devolutions/UniGetUI).

UniPac is a desktop package manager: one place to inspect and act on packages
installed through different package managers, without learning each manager's
command line interfaces.

The window is native Go, drawn by [MyGo](https://github.com/egoist/mygo) with
controls from [Onevra](https://github.com/oneo-me/onevra). One binary, no
webview, no JavaScript.

![screenshot](./assets/screenshot.png)

## Features

- Discovers the package managers available on the host, and reports each one's
  capabilities so unsupported actions stay disabled with a reason.
- Lists installed and outdated packages in a native UI, cached in SQLite.
- Searches across managers, marking what is already installed and what has a
  newer version.
- Installs a package at a version its manager lists: the search page opens a
  version dialog, newest first, filtered as you type.
- Records every install, update and uninstall as a task with its console —
  cancellable, retriable, reopened from a notification, and clearable.
- Updates itself from signed releases, downloading only what changed since the
  version it runs.

## Supported Package Managers

UniPac's backend is adapter-based: each adapter reports its own capabilities,
and the UI treats manager IDs as opaque values.

- Homebrew
- Bun
- npm
- pip
- pipx
- Cargo
- UV Tool

Support is uneven: bun, Homebrew and npm list and act reliably; the Python and
Cargo adapters are narrower. A manager that cannot answer an action says so on
its page rather than leaving a disabled button unexplained.

## Install

On macOS, one line installs the latest version in `/Applications`, or in
`~/Applications` when you cannot write there:

```sh
curl -fsSL https://github.com/RUBisco0211/unipac/releases/latest/download/install.sh | sh
```

The script checks the archive's size and Ed25519 signature against the key in
`mygo.json` before it installs anything, and
`curl -fsSL …/install.sh | sh -s -- --uninstall` removes the app, keeping its
settings and data. `curl` sets no quarantine flag, so Gatekeeper does not ask
about an app installed this way. Releases before 0.4.0 carry no update manifest
for the script to read: install those from the disk image.

From the disk image, Gatekeeper asks once, because the build is ad hoc signed and
not notarized: allow it in System Settings › Privacy & Security, or run

```sh
xattr -dr com.apple.quarantine /Applications/UniPac.app
```

Only that first launch needs it: the updates the app installs itself carry no
quarantine flag.

## Requirements

Go 1.27.3 or newer. Build with `CGO_ENABLED=0`.

## Development

From the repository root:

```sh
go get -tool github.com/egoist/mygo/cmd/mygo
go tool mygo dev    # run with live reload
go tool mygo build  # build for this machine
```

Cross-compile to `dist/<os>-<arch>/`; every platform builds from any machine:

```sh
go tool mygo build -platform darwin/arm64,windows/amd64
```

Before submitting a change, run:

```sh
gofmt -l .
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

If Go cannot write to its default build cache, point `GOCACHE` at a writable
path.

## Releases

Pushing a `v*` tag runs `.github/workflows/release.yml`, which checks the tag
against `version` in `mygo.json`, runs the checks above, and publishes a macOS
arm64 disk image and the signed update to a GitHub release:

```sh
git tag v0.4.0
git push origin v0.4.0
```

The release notes are that version's section of [CHANGELOG.md](CHANGELOG.md), so
a version without one cannot be released. The workflow can also be run from the
Actions tab to build without releasing.

Only macOS arm64 is packaged, and builds are ad hoc signed, so Gatekeeper asks
users on other Macs to allow the app once.

### Updates

An installed copy updates itself: ten seconds after launch, then once a day, it
asks the release feed for a version newer than its own, verifies the manifest and
the files against the Ed25519 key in `mygo.json`, and offers to install it.
**Check for Updates…** in the app menu checks at once. Installing downloads a
delta from the running version when the release has one, and the whole archive
otherwise; the new version runs after a restart, or at the next launch.

The key is made once, with `go tool mygo keygen`, and its secret half must be the
repository secret `MYGO_UPDATER_PRIVATE_KEY`. It is never regenerated: installed
copies accept only updates signed with it, so losing it strands them, and anyone
who has it can ship code to them.

Notarization is not part of this: the app verifies updates with its own key, and
the files it writes carry no quarantine. Adding `macos.signingIdentity` and
`macos.notarize` removes the first-launch question of a build installed from the
disk image.

## Documentation

- [.agents/ARCHITECTURE.md](.agents/ARCHITECTURE.md) — architecture and backend
  boundaries.
- [.agents/PACKAGE_MANAGER_ADAPTERS.md](.agents/PACKAGE_MANAGER_ADAPTERS.md) —
  adapter capabilities and per-manager CLI details.
- [AGENTS.md](AGENTS.md) — repository rules and verification.
- [.agents/skills/](.agents/skills/) — MyGo and Onevra skills for UI work.

## Known Limitations

- Not production-ready; package actions may require host permissions.
- Startup and refresh depend on manager command health.
- Some capabilities stay off until their CLI behavior is verified — notably,
  no manager but npm lists a package's versions, and none of the Python or
  Cargo adapters search.
- Native layout and platform behavior need manual review; headless tests do not
  cover them.

## License

MIT
