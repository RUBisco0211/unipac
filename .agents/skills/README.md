# Agent skills

Project-local agent skills. Each directory is a verbatim copy of an upstream
skill, so refreshing one is a plain re-copy rather than a merge.

| Skill | Upstream | Revision | Installed |
| --- | --- | --- | --- |
| `mygo-maintenance/` | `egoist/mygo` at `cmd/mygo/template/shared/.agents/skills/mygo-maintenance` | `c971c903` (2026-11-01) | 2026-11-01 |
| `onevra/` | `oneo-me/onevra` at `skills/onevra` | `18faf834` (2026-10-28) | 2026-11-01 |

`mygo-maintenance` covers MyGo lifetime, identity, threading and testing rules
for `github.com/egoist/mygo`. `onevra` covers component choice, hidden UI
tests, and `mygo.json` packaging for `github.com/oneo-me/onevra/source`.

## Updating

`mygo-maintenance` ships with the MyGo CLI. From the Go module root:

```sh
go tool mygo install-skills .
```

It replaces the bundled files and leaves unrelated skills and extra files
alone.

`onevra` has no installer. Re-copy `skills/onevra` from the Onevra repository
at the revision that matches the pinned `github.com/oneo-me/onevra/source`
version, then update the table above.

## Relative links

Four rows of `onevra/SKILL.md` link chart, graph, treemap and flow usage
examples as `../../source/components/...`. Those paths resolve inside the
Onevra repository, not here. Resolve them against the module directory printed
by:

```sh
go list -m -f '{{.Dir}}' github.com/oneo-me/onevra/source
```
