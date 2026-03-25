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
	@mkdir -p $(HOME)/go/bin
	cp $(BINDIR)/$(BINARY) $(HOME)/go/bin/$(BINARY).tmp
	mv $(HOME)/go/bin/$(BINARY).tmp $(HOME)/go/bin/$(BINARY)
