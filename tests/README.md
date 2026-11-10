# Integration tests

The integration suite uses the vpsAdminOS test runner to boot a local vpsAdmin
cluster and a VM running the packaged `vpsf-status` service.

Useful commands:

```sh
./test-runner.sh ls
./test-runner.sh test status-page
./test-runner.sh test -t ci
```

Use `./test-runner.sh test -t ci --verbose` to expose periodic per-test build
and execution output. CI captures both output streams in `test.log` and attempts
to upload that log with the per-test logs on failure or cancellation. This
helps distinguish package/image preparation from VM startup and test assertions.

The test platform follows the nested `vpsadmin/vpsadminos` input. Refresh it with
`nix flake update vpsadmin/vpsadminos`. Before a full run, verify that the evaluated
kernel outputs match those published by the selected OS's kernel workflow; the
same kernel version alone does not establish cache availability. Changes to this
input still need full integration testing against the pinned vpsAdmin version.
