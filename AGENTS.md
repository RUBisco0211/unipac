# UniPac

UniPac is a desktop package manager: one place to inspect and act on packages
installed by Homebrew, npm, pip, pipx, Cargo and uv tool, without learning each
manager's command line.

The window is native Go, drawn by [MyGo](https://github.com/egoist/mygo) with
controls from [Onevra](https://github.com/oneo-me/onevra). One binary, no
webview, no JavaScript.

Read [.agents/ARCHITECTURE.md](.agents/ARCHITECTURE.md) before changing code.
[.agents/PACKAGE_MANAGER_ADAPTERS.md](.agents/PACKAGE_MANAGER_ADAPTERS.md) holds
adapter capability boundaries and per-manager CLI details.

The app was rebuilt from Wails + Vue into MyGo native UI; the architecture
document records what that settled. Open questions live in the "Suggested
Near-Term Cleanup" section there.

## Skills

- `.agents/skills/mygo-maintenance/` — MyGo lifetime, identity, threading and
  testing rules.
- `.agents/skills/onevra/` — component choice, hidden UI tests, and `mygo.json`
  packaging.

Load both before writing or reviewing UI code.

## Rules

- `source/` is the Go module root: `go.mod`, `mygo.json`, `main.go`, `app/`,
  `backend/`, `views/` and `locales/` sit there. Run Go and MyGo commands from
  `source/`, never from the repository root. Documentation, `assets/screenshot.png`
  and `dist/` stay at the root.
- Build with `CGO_ENABLED=0`; every platform builds from any machine.
- The backend registry is the source of truth for managers. Never hardcode a
  manager list, label, icon or command in the UI.
- Views call `App` methods. Never call adapters, the registry or the cache
  directly.
- Builders run on the UI thread every frame: no I/O, no parsing, no blocking,
  no background work inside them.
- Elements expire at the end of a build pass. Keep application state and
  `ui.Handle`, never an element.
- Prefer Onevra controls. Use MyGo's styled widgets only where Onevra has no
  equivalent.
- All user-visible text goes through the catalogs in `locales/`.

## Verification

```sh
cd source
gofmt -l .
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

`mygo vet` runs `go vet ./...` and adds the native UI lifetime checks. `gofmt -l .`
must print nothing. Layout and platform behavior need review in a desktop session
on each claimed platform; headless tests do not prove native behavior.
