.DEFAULT_GOAL := build

build:
	go build ./cmd/box

fmt:
	./fmt.sh

test:
	go test ./...
