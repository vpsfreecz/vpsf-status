# Integration tests

The integration suite uses the vpsAdminOS test runner to boot a local vpsAdmin
cluster and a VM running the packaged `vpsf-status` service.

Useful commands:

```sh
./test-runner.sh ls
./test-runner.sh test status-page
./test-runner.sh test -t ci
```
