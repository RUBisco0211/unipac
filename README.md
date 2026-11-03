# UniPac

> Status: Work in Progress. Expect incomplete features, rough edges and
> changing APIs.

UniPac is a desktop package manager: one place to inspect and act on packages
installed by Homebrew, npm, pip, pipx, Cargo and uv tool, without learning each
manager's command line.

The window is native Go, drawn by [MyGo](https://github.com/egoist/mygo) with
controls from [Onevra](https://github.com/oneo-me/onevra). One binary, no
webview, no JavaScript.

![screenshot](./assets/screenshot.png)

## Features

- Discovers the package managers available on the host.
- Lists installed and outdated packages in a native UI, cached in SQLite.
- Reports each manager's capabilities so unsupported actions stay disabled.
- Adds search, install, uninstall, update and version lookup incrementally.

Support is uneven while the project is under active development: some managers
list reliably while other actions remain disabled or incomplete.

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

## Demo Mode

Run on simulated packages and tasks without touching any real package manager:

```sh
go run ./source -demo
# or
UNIPAC_DEMO=1 go run ./source
```

The demo exercises update, uninstall, install, bulk and concurrent tasks, cancel
and retry, task filtering, and the quit prompt. Its data lives in
`source/demo.go` and is not persisted. On macOS, system notifications need a
packaged app, so use `UNIPAC_DEMO=1 go tool mygo dev` to see them.

## Releases

Pushing a `v*` tag runs `.github/workflows/release.yml`, which checks the tag
against `version` in `mygo.json`, runs the checks above, and publishes a macOS
arm64 disk image with checksums to a GitHub release:

```sh
git tag v0.2.0
git push origin v0.2.0
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
- Some capabilities stay disabled until their CLI behavior is reliable.
- Native layout and platform behavior need manual review; headless tests do not
  cover them.

## License

MIT
