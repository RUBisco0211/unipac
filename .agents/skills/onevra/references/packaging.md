# Project layout and packaging

Package applications with MyGo's own CLI and `mygo.json`; Onevra has no packager of its own. MyGo's [configuration](https://mygo.egoist.dev/docs/configuration) and [distribution](https://mygo.egoist.dev/docs/distribution) guides are authoritative; this page records only the Onevra project conventions.

## Requirements

- Pin the CLI as a Go tool in the application module: `go get -tool github.com/egoist/mygo/cmd/mygo`, at the MyGo version the module already requires.
- `mygo.json` sits at the Go module root, beside `go.mod`. `main` selects the main package (default `.`). Builds disable cgo, so the application must be pure-Go-compatible for cross-builds.
- `resources/` holds only files the application reads at run time. Everything in it ships as payload, including platform directories such as `resources/darwin-arm64/` for compiled libraries and helpers of one target. Locate it with `mygo.App.Path(mygo.PathResources)`, which resolves to the bundle when packaged and to `source/resources` under `go run`; tests that read resources set that path with `mygo.App.SetPath`. Embed small fixed data with `//go:embed` instead.
- Build-time inputs go in `assets/` and are named explicitly in `mygo.json`. MyGo's default `resources/icon.png` would ship the icon a second time, and an `.icon` inside `resources/` would ship as raw files.
- The application icon's source is the Icon Composer document `assets/icon.icon`, set as `macos.icon` and compiled with Xcode 26 or later. Any file name works because MyGo renames it to `AppIcon.icon` for `actool`. Export `assets/icon.png` from it for `icon` and commit both. MyGo derives the `.icns`, the Windows icon resource and the Linux icon from that PNG, so do not commit those formats:

  ```sh
  "/Applications/Xcode.app/Contents/Applications/Icon Composer.app/Contents/Executables/ictool" \
    assets/icon.icon --export-image --output-file assets/icon.png \
    --platform macOS --rendition Default --width 1024 --height 1024 --scale 1
  ```

  Re-export the PNG whenever the `.icon` changes. Builds without Xcode skip the `.icon` and use the PNG alone.
- Plugin native libraries come from imported packages' `mygo-plugin.json` manifests; do not list them in configuration.

## Layout

Follow Onevra's repository shape: documentation at the repository root, and everything Go, including packaging configuration, in `source/`.

```text
<app>/
├── .gitignore            # /dist/
├── AGENTS.md
├── ARCHITECTURE.md
├── README.md
├── docs/
└── source/               # Go module root; run Go and MyGo here
    ├── go.mod            # module <repository path>/source; tool github.com/egoist/mygo/cmd/mygo
    ├── mygo.json
    ├── main.go           # or a main package selected by `main`
    ├── assets/           # build-time inputs, not shipped
    │   ├── icon.icon/    # icon source (Icon Composer)
    │   └── icon.png      # 1024 × 1024 export of icon.icon
    ├── locales/          # embedded translation catalogs
    ├── resources/        # runtime payload, loaded through MyGo's resource path
    └── internal/…        # application packages
```

When the module is published, its path includes `/source` and version tags use the `source/` prefix, such as `source/v0.1.0`.

## Configuration

```json
{
  "name": "My App",
  "identifier": "com.example.myapp",
  "version": "0.1.0",
  "icon": "assets/icon.png",
  "out": "../dist",
  "macos": {
    "icon": "assets/icon.icon",
    "minimumSystemVersion": "13.0"
  }
}
```

Declare macOS usage descriptions and other application-owned metadata in `macos.infoPlist`. Applications still own runtime authorization; adding a usage description does not grant permission.

## Commands

```sh
cd source
go tool mygo dev                                          # "<name> Dev" with live reload
go tool mygo build -platform darwin/arm64,windows/amd64   # outputs in <out>/<os>-<arch>/
```

Keep the application's displayed version aligned with `version` in `mygo.json`.
