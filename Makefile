VERSION ?= 0.1.0-dev
MODULE  := github.com/pedrobelmino/tui-sdd-open-api
BINARY  := bin/sdd-open-api
LDFLAGS := -ldflags "-X $(MODULE)/internal/cmd.version=$(VERSION)"

# User-writable install dir (no sudo). Override: make build INSTALL_DIR=$$HOME/.local/bin
INSTALL_DIR ?= $(HOME)/go/bin

.PHONY: build install test clean

build:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) ./cmd/sdd-open-api
	@mkdir -p "$(INSTALL_DIR)"
	@cp -f $(BINARY) "$(INSTALL_DIR)/sdd-open-api"
	@echo "→ $(INSTALL_DIR)/sdd-open-api"

install: build

test:
	go test ./...

clean:
	rm -rf bin/
