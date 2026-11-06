# TODO: what the settings page should still offer

The batch on `feature/settings` is done and merged: the managers page (tabs,
marks, key-value panels, capability badges), the version capture in preflight,
the appearance and notification settings, the About section with its GitHub row,
and the history clear behind a dialog. This file keeps what was left out on
purpose.

## Phase A — frontend only

- [ ] **Install roots.** Where each manager puts its packages, as a panel row: a
      new optional adapter method, because the answers are uneven — `brew
      --prefix`, `npm prefix -g`, `uv tool dir` are exact, `pipx` and `cargo` only
      have a default path, and `pip` writes into a site-packages directory that
      depends on the interpreter. `bun pm bin -g` gives the bin directory, not the
      install root. Confirm the shape before building it.

## Verification

```sh
gofmt -l .
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

Layout and the two new selects need a desktop session to judge; headless tests
prove the wiring, not how it looks.
