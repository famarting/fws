BIN ?= $(HOME)/bin/fws
PKG := ./cmd/fws

.PHONY: build install clean tidy test fmt vet

build:
	go build -o ./bin/fws $(PKG)

install:
	go build -o $(BIN) $(PKG)
	@echo "installed -> $(BIN)"

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf ./bin
