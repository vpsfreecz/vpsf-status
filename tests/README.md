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

On the first failed example, the status suite captures bounded status and API
journals, API/WebUI/console HTTP results, and the WebUI container's nginx/PHP-FPM
journal through the existing machine objects. The capture takes at most six
commands with eight-second deadlines, runs only once, and preserves the original
failure. Diagnostic errors do not prevent the remaining test cleanup.

The status VM uses a 15-second HTTP/API probe budget because healthy WebUI
metadata requests can exceed two seconds in this fixture. The polling interval
stays at two seconds, and the existing detection/recovery assertions retain
their 90/120-second deadlines. This fixture budget does not change production
configuration or the application's default timeout.
