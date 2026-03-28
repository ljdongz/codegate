BINARY := codegate
BINDIR := bin

.PHONY: build test lint clean dev setup uninstall

build:
	go build -o $(BINDIR)/$(BINARY) ./cmd/codegate

setup: build
	CODEGATE_CONFIG_DIR=$(HOME)/.codegate-dev ./$(BINDIR)/$(BINARY) setup

dev: build
	CODEGATE_CONFIG_DIR=$(HOME)/.codegate-dev ./$(BINDIR)/$(BINARY) run

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -rf $(BINDIR) dist

uninstall:
	rm -rf $(HOME)/.codegate-dev $(BINDIR)

