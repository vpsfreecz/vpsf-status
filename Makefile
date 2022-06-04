build:
	go fmt
	go fmt github.com/vpsfreecz/vpsf-status/config
	go build

.PHONY: build
