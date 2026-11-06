
<p align="center">
    <img src="./assets/icon.png" width="128" height="128" alt="UniPac">
</p>

# UniPac

[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/apps/unipac)

UniPac is a desktop package manager: one place to inspect and act on packages
installed through different package managers, without learning each manager's
command line.

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
- Configures appearance (light, dark or system), notifications, language and
  each manager's executable, and shows every manager's version, state and
  capabilities on a page of its own.

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
arm64 disk image with checksums to a GitHub release:

```sh
git tag v0.3.0
git push origin v0.3.0
```

The workflow can also be run from the Actions tab to build the disk image as an
artifact. Only macOS arm64 is packaged; builds are ad hoc signed, so Gatekeeper
asks users on other Macs to allow the app. Add `macos.signingIdentity` and
`macos.notarize` and import the certificate in the workflow to ship a signed,
notarized build.

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
