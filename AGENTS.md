# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go status page service for vpsFree.cz. The main
package lives at the repository root, with `main.go` wiring configuration,
templates, routes, metrics, and background checks. Supporting packages are in
`config/` for JSON configuration parsing and `json/` for response
serialization. HTML templates are in `templates/`, static assets are in
`public/`, and sample runtime files are `config-sample.json` and
`notice-sample.html`.

## Build, Test, and Development Commands

- `make`: formats all Go packages, then builds the `vpsf-status` binary.
- `make hooks`: installs Lefthook-managed Git hooks.
- `go fmt ./...`: formats all Go packages; run this before submitting changes.
- `go test ./...`: runs all package tests.
- `./vpsf-status config-sample.json`: runs the service locally using the sample
  configuration. The default sample listens on `:8080`.
- `nix develop`: opens a flake dev shell with Go, GNU Make, and Lefthook, when
  using Nix.

## Coding Style & Naming Conventions

Follow `.editorconfig`: UTF-8, LF line endings, trimmed trailing whitespace,
tabs for Go files, and tab indentation with width 2 for templates. Use standard
Go formatting. Keep package names short and lowercase, exported identifiers in
`PascalCase`, and unexported identifiers in `camelCase`. Prefer small functions
matching current boundaries such as status collection and HTTP handling.

## Testing Guidelines

Place Go tests next to the code under test using the `*_test.go` suffix. Use
table-driven tests for parsing, view construction, and JSON output. For HTTP
handlers, cover response headers and body shape with `httptest` where practical.
Always run `go test ./...` before opening a pull request.

JSON responses and Prometheus metrics are public compatibility contracts. Do not
change JSON field names, metric names, label names, status strings, or numeric
metric meanings without explicit maintainer approval and intentional test
updates.

## Configuration & Runtime Notes

Do not commit local `config.json` or built `vpsf-status` binaries; both are
ignored. Keep sample configuration changes synchronized with `config/config.go`.
When changing Go dependencies in `go.mod` or `go.sum`, update
`nix/package.nix` so its `vendorHash` matches the new module graph, and verify
the package with `nix build .#vpsf-status`.
Runtime paths are resolved from `data_dir`, so templates and assets must work
with both `.` and deployment-specific data directories.

## Localization

English source strings live in `internal/i18n/catalog/messages.go`. Editable
translations live in `i18n/<lang>.toml`; generated `i18n/*.active.toml` files
are embedded into the Go binary and must be refreshed with `make i18n-update`.
Use `make i18n-health` to check that generated files are fresh and translations
are complete.
Follow the Czech terminology guidelines in `i18n/README.md` when editing Czech
translations.

## Commit & Pull Request Guidelines

Git history uses short, imperative subjects, for example `Fix notice presence
metric` or `Show both active and recent outages`. Each commit message must say
what is changing and why, describe the problem, and summarize the solution.
Wrap subject and body lines at 80 characters. Pre-commit hooks must run for
every commit; do not use `--no-verify`, `SKIP=...`, `LEFTHOOK=0`, or any other
method to bypass them. Write commit messages through a temporary file and commit
with `git commit -F`, for example:

```bash
msg=$(mktemp)
$EDITOR "$msg"
git commit -F "$msg"
rm "$msg"
```

Pull requests should include a concise description, linked issue if applicable,
test results such as `go test ./...`, and screenshots when changing rendered
templates or static assets.
