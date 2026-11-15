ifeq ($(OS),Windows_NT)
EXE_EXT := .exe
endif
BINARY := envdoctor$(EXE_EXT)
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: bootstrap build test race lint fmt vet docs check-docs run docker clean

bootstrap:
	go mod download
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

build: export CGO_ENABLED := 0
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/envdoctor


test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

lint: fmt vet
	golangci-lint run ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

docs:
	go run ./cmd/rulegen

check-docs:
	go run ./cmd/rulegen --check

run: build
	./bin/$(BINARY)

docker:
	docker build -t envdoctor:$(VERSION) .

clean:
	go clean
	rm -rf bin
