.PHONY: build test vet check clean

build:
	go build -trimpath -ldflags="-s -w" -o dist/shim ./cmd/shim

test:
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -rf dist
