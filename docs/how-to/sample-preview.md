# Preview pages with sample data

Run from the repository root in its Nix development shell:

```sh
nix develop -c go run -tags preview .
```

The preview listens on `127.0.0.1:8080`. Visit `/?lang=en` or `/?lang=cs` and
follow the entity, group and about links. Every HTML page has a bilingual
banner identifying the sample data. All service addresses use reserved example
names and IP addresses. Links to those example services do not lead to live systems.

The preview reuses the application's routes, templates, translations and view
builders. It never starts monitoring or reads `config-sample.json`, production
configuration or production history. Each run creates a new temporary directory
with SQLite history and a sample notice. It removes that directory on normal
exit, including Ctrl+C or SIGTERM. A forced termination may leave a directory
whose path is printed at startup.

No vpsAdmin API, credentials, database server, ICMP privileges or development
cluster is required. The Go module dependencies and repository templates/assets
are still needed. Bootstrap, its icons and its JavaScript remain served by the
existing CDN, so the browser needs network access for those assets.

## Choose a scenario

The default `mixed` scenario includes healthy, maintenance, degraded and down
services, active and resolved incidents, a notice, an advisory and long labels.
The first node has more than 50 synthetic probe events, with history bars and
popovers. Its detail page and the Praha group offer event pagination. For example:

```text
/entity?kind=node&id=node01.preview.example.invalid&lang=en
/entity?kind=node&id=node01.preview.example.invalid&probe_page=2&lang=cs
/group?kind=location&id=1&lang=cs
```

Select another scenario at startup:

```sh
nix develop -c go run -tags preview . -scenario healthy
nix develop -c go run -tags preview . -scenario loading
nix develop -c go run -tags preview . -scenario fetch-failure
```

`healthy` makes every service operational. `loading` keeps the overview in its
initial loading state. `fetch-failure` shows failed outage/advisory retrieval
alongside healthy service checks. These are fixed examples; no fetch is attempted.
All scenarios keep the notice and the synthetic-data banner visible.

The application clock and notice timestamp are fixed at 2026-11-10 12:00 UTC.
Displayed times use the host's time zone; set `TZ=UTC` when consistent capture
times across hosts are needed.

## Listen address and assets

Override the address or the directory containing `templates/` and `public/`:

```sh
nix develop -c go run -tags preview . -listen 127.0.0.1:8081 -data-dir .
```

Binding to a non-loopback address exposes the synthetic preview to other hosts.
There is no authentication. In a managed development workspace, launch previews
through that workspace's registered temporary-service runner.

The `preview` build tag selects a separate entry point. Ordinary `go build`,
`go test ./...` and the production configuration-file CLI are unchanged. Run the
preview-specific tests with `nix develop -c go test -tags preview ./...`.
