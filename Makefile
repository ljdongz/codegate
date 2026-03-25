BINARY := codegate
BINDIR := bin

.PHONY: build test lint clean install

build:
	go build -o $(BINDIR)/$(BINARY) ./cmd/codegate

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -rf $(BINDIR) dist

install: build
	cp $(BINDIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp $(BINDIR)/$(BINARY) /usr/local/bin/$(BINARY)
