# vpsFree.cz Status
Status page for vpsFree.cz infrastructure.

## Building
Build with make:
```bash
make
```

## Git hooks
Inside `nix develop`, install the pre-commit hook with:
```bash
make hooks
```

The hook runs `gofmt` on staged Go files and stages formatting changes. If you
do not use Nix, install `lefthook` first.

## Run
`vpsf-status` needs a config file, see [config-sample.json](./config-sample.json):

```bash
./vpsf-status config-sample.json
```

## Sample preview

Preview the real pages with synthetic data, without API credentials, a database
server or monitoring privileges:

```bash
nix develop -c go run -tags preview .
```

Open <http://127.0.0.1:8080/?lang=en> or <http://127.0.0.1:8080/?lang=cs>.
The preview uses disposable SQLite history and a frozen clock. Existing CDN
styles, scripts and icons need browser network access.
See [preview scenarios and options](docs/how-to/sample-preview.md).
See [themes and responsive layout](docs/reference/appearance.md) for appearance
controls and supported browser behavior.

## Automated flake updates

The `update-dependencies.yml` workflow updates the resolved nixpkgs input using
a separately pinned confctl CI tool. It requires ancestry preservation across the changed
lock graph and stops publication on rejection or unavailable evidence. It
checks the final lock against a saved baseline after hooks and checks, then
uses an ordinary non-force push. If the target branch advances, rerun from its
new baseline instead of rebasing the old lock update. The CI tool does not
become a runtime flake input.

The workflow saves confctl's Git mirrors in `~/.cache/confctl` even when an
update is rejected. Every update checks ancestry, whether the cache was
restored or freshly cloned.

See the [shared guard contract](https://github.com/vpsfreecz/confctl/blob/822c0e35dbe2549111912f539116b4225dfd3035/docs/flake-inputs.md#guarded-automated-updates) for supported sources and failure behavior.
