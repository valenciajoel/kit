BINARY  := kit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build build-linux build-windows test vet fmt clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/kit

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux-amd64 ./cmd/kit

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-windows-amd64.exe ./cmd/kit

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin
