build:
	go fmt ./...
	go build

hooks:
	lefthook install

.PHONY: build hooks
