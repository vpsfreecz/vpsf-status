build:
	go fmt ./...
	go build

hooks:
	lefthook install

test-integration:
	./test-runner.sh test -t ci

.PHONY: build hooks test-integration
