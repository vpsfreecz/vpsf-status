build:
	go fmt ./...
	go build

i18n-update:
	go run ./cmd/i18n

i18n-health:
	go run ./cmd/i18n -check

hooks:
	lefthook install

test-integration:
	./test-runner.sh test -t ci

.PHONY: build i18n-update i18n-health hooks test-integration
