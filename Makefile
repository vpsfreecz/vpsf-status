build:
	go fmt
	go fmt github.com/vpsfreecz/vpsf-status/config
	go fmt github.com/vpsfreecz/vpsf-status/json
	go build

.PHONY: build
