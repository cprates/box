.DEFAULT_GOAL := build

build:
	go build

fmt:
	./fmt.sh

test:
	go test ./...
