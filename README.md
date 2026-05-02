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
